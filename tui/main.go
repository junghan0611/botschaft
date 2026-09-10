// botschaft/tui — a thin terminal viewer over an existing ChatGPT web session.
//
// This program is the VERIFICATION HARNESS for the Emacs package that is the real
// deliverable. Both front ends call the same four verbs, so a server trap found
// here does not have to be found again there.
//
// It owns NO data. Every screen is a live read through chatgpt-web-adapter (CWA);
// nothing is cached to disk. Two process boundaries, deliberately different:
//
//	read   → `cwa messages <id> --json`   PUBLIC, schema-versioned CLI
//	list   → `cwaq list`                  private shim: upstream exposes no verb
//	search → `cwaq search <q>`            private shim: upstream exposes no verb
//
// The split is the point. When upstream grows `cwa list` / `cwa search`, delete
// cwaq and change two strings here.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── configuration ────────────────────────────────────────────────────────────

type config struct {
	cwaBin   string // public CWA CLI
	cwaqBin  string // private list/search shim
	pyBin    string // interpreter that can import chatgpt_web_adapter
	authFile string
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadConfig resolves the two binaries off PATH and the auth file to its
// documented home. Every value is overridable, because the interpreter that can
// import the adapter is usually a virtualenv rather than the system python.
func loadConfig() config {
	return config{
		cwaBin:   env("CWA_BIN", lookup("cwa")),
		pyBin:    env("CWA_PY", lookup("python3")),
		cwaqBin:  env("CWAQ_BIN", findShim()),
		authFile: env("CWA_AUTH", filepath.Join(stateDir(), "auth_data.json")),
	}
}

// stateDir is this program's XDG state directory, shared with the shim and the
// Emacs package so all three look in one place.
func stateDir() string {
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		return filepath.Join(state, "botschaft")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "botschaft")
}

// lookup returns the absolute path of name on PATH, or name itself so the
// startup preflight can report an honest miss.
func lookup(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return name
}

// findShim locates bin/cwaq without assuming where the binary was built or run
// from. The shim is shared with the Emacs front end, so it lives at the repo root
// rather than next to this program.
func findShim() string {
	var roots []string
	if self, err := os.Executable(); err == nil {
		dir := filepath.Dir(self)
		roots = append(roots, dir, filepath.Join(dir, "..", "bin"), filepath.Join(dir, "bin"))
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, filepath.Join(wd, "bin"), filepath.Join(wd, "..", "bin"), wd)
	}
	for _, r := range roots {
		candidate := filepath.Join(r, "cwaq")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "bin/cwaq" // report an honest miss through the startup preflight
}

// ── wire types ───────────────────────────────────────────────────────────────

type conv struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	UpdateTime string `json:"update_time"`
	Snippet    string `json:"snippet"`
	ProjectID  string `json:"project_id"`
}

type queryEnvelope struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Items []conv `json:"items"`
}

type turn struct {
	Role       string  `json:"role"`
	Recipient  string  `json:"recipient"`
	Text       string  `json:"text"`
	CreateTime float64 `json:"create_time"`
	Model      string  `json:"model"`
}

type messagesEnvelope struct {
	OK             bool   `json:"ok"`
	ConversationID string `json:"conversation_id"`
	Count          int    `json:"count"`
	Schema         int    `json:"schema"`
	Messages       []turn `json:"messages"`
}

// ── tea messages ─────────────────────────────────────────────────────────────

// Every async result carries the generation it was issued under. Tea gives no
// ordering guarantee across commands, so a late reply from an abandoned request
// would otherwise reopen a screen the user already left (measured by review:
// reload a conversation, press esc, and the late turnsMsg drags you back in).
type convsMsg struct {
	gen   int
	items []conv
	label string
	err   error
}

type projectsMsg struct {
	gen   int
	items []conv
	err   error
}

type turnsMsg struct {
	gen   int
	id    string
	title string
	turns []turn
	err   error
}

type tickMsg time.Time

// ── commands ─────────────────────────────────────────────────────────────────

func runJSON(name string, args []string, out any) error {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	stdout, err := cmd.Output()
	if err != nil {
		// cwaq reports failure as JSON on stdout with a non-zero exit; prefer it.
		if len(stdout) > 0 && json.Unmarshal(stdout, out) == nil {
			return nil
		}
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return fmt.Errorf("%s: %s", filepath.Base(name), strings.TrimSpace(string(ee.Stderr)))
		}
		return fmt.Errorf("%s: %w", filepath.Base(name), err)
	}
	return json.Unmarshal(stdout, out)
}

// scoped appends --project only when a project is selected. An empty --project is
// not the same as omitting it, and the plain /conversations route ignores gizmo_id
// entirely, so the shim (not this argv) is what routes a scoped list.
func scoped(args []string, project string) []string {
	if project != "" {
		return append(args, "--project", project)
	}
	return args
}

func (c config) fetchRecent(gen, limit int, project string) tea.Cmd {
	return func() tea.Msg {
		var env queryEnvelope
		args := scoped([]string{c.cwaqBin, "--auth-file", c.authFile,
			"list", "--limit", fmt.Sprint(limit)}, project)
		if err := runJSON(c.pyBin, args, &env); err != nil {
			return convsMsg{gen: gen, err: err}
		}
		if !env.OK {
			return convsMsg{gen: gen, err: fmt.Errorf("%s", env.Error)}
		}
		return convsMsg{gen: gen, items: env.Items, label: "recent"}
	}
}

func (c config) fetchSearch(gen int, query string, limit int, project string) tea.Cmd {
	return func() tea.Msg {
		var env queryEnvelope
		args := scoped([]string{c.cwaqBin, "--auth-file", c.authFile,
			"search", query, "--limit", fmt.Sprint(limit)}, project)
		if err := runJSON(c.pyBin, args, &env); err != nil {
			return convsMsg{gen: gen, err: err}
		}
		if !env.OK {
			return convsMsg{gen: gen, err: fmt.Errorf("%s", env.Error)}
		}
		return convsMsg{gen: gen, items: env.Items, label: "search: " + query}
	}
}

func (c config) fetchProjects(gen int) tea.Cmd {
	return func() tea.Msg {
		var env queryEnvelope
		if err := runJSON(c.pyBin, []string{c.cwaqBin, "--auth-file", c.authFile,
			"projects", "--limit", "50"}, &env); err != nil {
			return projectsMsg{gen: gen, err: err}
		}
		if !env.OK {
			return projectsMsg{gen: gen, err: fmt.Errorf("%s", env.Error)}
		}
		return projectsMsg{gen: gen, items: env.Items}
	}
}

func (c config) fetchTurns(gen int, id, title string) tea.Cmd {
	return func() tea.Msg {
		var env messagesEnvelope
		if err := runJSON(c.cwaBin, []string{"messages", id, "--json",
			"--auth-file", c.authFile, "--limit", "1000"}, &env); err != nil {
			return turnsMsg{gen: gen, id: id, err: err}
		}
		if !env.OK {
			return turnsMsg{gen: gen, id: id, err: fmt.Errorf("read failed (schema=%d)", env.Schema)}
		}
		return turnsMsg{gen: gen, id: id, title: title, turns: env.Messages}
	}
}

// ── conversation address ─────────────────────────────────────────────────────

func convURL(id string) string { return "https://chatgpt.com/c/" + id }

// shortTime trims the shim's offset-bearing ISO-8601 ("2026-09-10T14:27:04+09:00")
// down to what a list column can hold. The offset stays in the JSON contract for
// whoever parses it; only the display drops it.
func shortTime(s string) string {
	if len(s) >= 16 {
		return s[:16]
	}
	return s
}

// copyToClipboard picks the tool that matches the session. wl-copy exists on this
// host but silently fails under X11, so the display server decides, not $PATH.
func copyToClipboard(text string) error {
	var name string
	var args []string
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		name = "wl-copy"
	} else {
		name, args = "xclip", []string{"-selection", "clipboard"}
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func openInBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// ── styles ───────────────────────────────────────────────────────────────────

var (
	cTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	cDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cSel      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	cUser     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114"))
	cAsst     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	cTool     = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	cCode     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	cErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	cBar      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cHeadBar  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237"))
	spinChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// ── model ────────────────────────────────────────────────────────────────────

type screen int

const (
	screenList screen = iota
	screenSearch
	screenRead
	screenProjects
)

type model struct {
	cfg    config
	screen screen

	items  []conv
	label  string
	cursor int
	offset int

	// project scope. scopeID == "" means every conversation.
	projects  []conv
	pcursor   int
	poffset   int
	scopeID   string
	scopeName string

	input textinput.Model

	vp        viewport.Model
	vpReady   bool
	cur       conv
	turns     []turn
	showTools bool

	// reading quality (issue #2 §4.1). Derived from the live turns slice;
	// nothing is stored across reloads beyond the current view flags.
	turnCursor int   // index into the active (visible/overview) turn list
	turnStarts []int // line offset of each active turn in the viewport
	overview   bool
	expanded   map[int]bool // keyed by index into turns
	finding    bool
	findQ      string
	findHits   []int // indices into turns
	findAt     int

	gen     int // generation of the request currently owning the screen
	loading bool
	spin    int
	status  string
	err     error

	w, h int
}

func newModel(cfg config) model {
	in := textinput.New()
	in.Placeholder = "search — enter to run, esc to cancel"
	in.CharLimit = 200
	return model{cfg: cfg, screen: screenList, input: in, loading: true,
		expanded: map[int]bool{}, status: "loading recent conversations"}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.cfg.fetchRecent(m.gen, 200, m.scopeID), tick())
}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// minCols/minRows is the smallest frame that can hold header + one row + status.
// Below it the honest move is to say so, not to draw a layout that scrolls its own
// header away.
const (
	minCols = 24
	minRows = 5
)

func (m *model) listRows() int {
	r := m.h - 4 // header + status + help
	if r < 1 {
		r = 1
	}
	return r
}

func (m *model) clampScroll() {
	rows := m.listRows() - 1 // one line is reserved for the cursor's snippet
	if rows < 1 {
		rows = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m model) Update(raw tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := raw.(type) {

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		vh := msg.Height - 3
		if vh < 1 {
			vh = 1
		}
		if !m.vpReady {
			m.vp = viewport.New(msg.Width, vh)
			m.vpReady = true
		} else {
			m.vp.Width, m.vp.Height = msg.Width, vh
		}
		if m.screen == screenRead {
			m.relayout()
		}
		return m, nil

	case tickMsg:
		if m.loading {
			m.spin = (m.spin + 1) % len(spinChars)
			return m, tick()
		}
		return m, nil

	case convsMsg:
		if msg.gen != m.gen {
			return m, nil // a reply from a request the user has already abandoned
		}
		m.loading = false
		if msg.err != nil {
			m.err, m.status = msg.err, ""
			return m, nil
		}
		m.err = nil
		m.items, m.label = msg.items, msg.label
		m.cursor, m.offset = 0, 0
		scope := "all"
		if m.scopeName != "" {
			scope = "project " + m.scopeName
		}
		m.status = fmt.Sprintf("%s · %s · %d", scope, msg.label, len(msg.items))
		m.screen = screenList
		return m, nil

	case projectsMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err, m.status = msg.err, ""
			return m, nil
		}
		m.err = nil
		// "All conversations" is a synthetic first row, not a project the server knows about.
		m.projects = append([]conv{{ID: "", Title: "All conversations (outside projects)"}}, msg.items...)
		m.pcursor, m.poffset = 0, 0
		for i, it := range m.projects {
			if it.ID == m.scopeID {
				m.pcursor = i
			}
		}
		m.screen = screenProjects
		m.status = fmt.Sprintf("%d projects", len(msg.items))
		return m, nil

	case turnsMsg:
		if msg.gen != m.gen || msg.id != m.cur.ID {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err, m.status = msg.err, ""
			m.screen = screenList
			return m, nil
		}
		m.err = nil
		wasReading := m.screen == screenRead
		m.turns = msg.turns
		m.expanded = map[int]bool{}
		m.findHits, m.findAt, m.findQ = nil, 0, ""
		m.finding = false
		if !wasReading {
			m.overview = false
			m.turnCursor = 0
		}
		m.screen = screenRead
		m.clampTurnCursor()
		m.refreshRead()
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenSearch:
			switch msg.Type {
			case tea.KeyEsc:
				m.screen = screenList
				return m, nil
			case tea.KeyEnter:
				q := strings.TrimSpace(m.input.Value())
				if q == "" {
					m.screen = screenList
					return m, nil
				}
				m.loading, m.status, m.err = true, "searching: "+q, nil
				m.screen = screenList
				m.gen++
				return m, tea.Batch(m.cfg.fetchSearch(m.gen, q, 500, m.scopeID), tick())
			case tea.KeyCtrlC:
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		case screenRead:
			return m.updateRead(msg)

		case screenProjects:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "h", "left", "p":
				m.gen++
				m.loading = false
				m.screen = screenList
				return m, nil
			case "j", "down":
				if m.pcursor < len(m.projects)-1 {
					m.pcursor++
				}
			case "k", "up":
				if m.pcursor > 0 {
					m.pcursor--
				}
			case "enter", "l", "right":
				if len(m.projects) == 0 {
					return m, nil
				}
				sel := m.projects[m.pcursor]
				m.scopeID = sel.ID
				if sel.ID == "" {
					m.scopeName = ""
				} else {
					m.scopeName = sel.Title
				}
				m.loading, m.status, m.err = true, "loading: "+sel.Title, nil
				m.screen = screenList
				m.gen++
				return m, tea.Batch(m.cfg.fetchRecent(m.gen, 200, m.scopeID), tick())
			}
			rows := m.listRows()
			if m.pcursor < m.poffset {
				m.poffset = m.pcursor
			}
			if m.pcursor >= m.poffset+rows {
				m.poffset = m.pcursor - rows + 1
			}
			return m, nil

		default: // screenList
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "j", "down":
				if m.cursor < len(m.items)-1 {
					m.cursor++
					m.clampScroll()
				}
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					m.clampScroll()
				}
			case "g", "home":
				m.cursor, m.offset = 0, 0
			case "G", "end":
				m.cursor = len(m.items) - 1
				m.clampScroll()
			case "y":
				if len(m.items) > 0 {
					url := convURL(m.items[m.cursor].ID)
					if err := copyToClipboard(url); err != nil {
						m.err = err
					} else {
						m.status, m.err = "copied: "+url, nil
					}
				}
				return m, nil
			case "o":
				if len(m.items) > 0 {
					if err := openInBrowser(convURL(m.items[m.cursor].ID)); err != nil {
						m.err = err
					} else {
						m.status, m.err = "opened in browser", nil
					}
				}
				return m, nil
			case "p":
				m.loading, m.status, m.err = true, "loading projects", nil
				m.gen++
				return m, tea.Batch(m.cfg.fetchProjects(m.gen), tick())
			case "/":
				m.input.Placeholder = "search — enter to run, esc to cancel"
				m.input.SetValue("")
				m.input.Focus()
				m.screen = screenSearch
				return m, textinput.Blink
			case "r", "esc":
				m.loading, m.status, m.err = true, "loading recent conversations", nil
				m.gen++
				return m, tea.Batch(m.cfg.fetchRecent(m.gen, 200, m.scopeID), tick())
			case "enter", "l", "right":
				if len(m.items) == 0 || m.loading {
					return m, nil
				}
				m.cur = m.items[m.cursor]
				m.loading, m.status, m.err = true, "reading: "+m.cur.Title, nil
				m.gen++
				return m, tea.Batch(m.cfg.fetchTurns(m.gen, m.cur.ID, m.cur.Title), tick())
			}
			return m, nil
		}
	}
	return m, nil
}

// ── reading quality (issue #2 §4.1) ──────────────────────────────────────────

// defaultFoldLines is the threshold a single turn must cross before it is
// folded. Measured: the largest human turn was ~99 wrapped lines, twice a
// 50-row pane. 20 leaves the opening of the turn on screen and the rest a key
// away, without hiding short replies.
const defaultFoldLines = 20

func isToolTurn(t turn) bool {
	return t.Recipient != "all" || t.Role == "tool"
}

func activeIndices(turns []turn, showTools, overview bool) []int {
	var ids []int
	for i, t := range turns {
		tool := isToolTurn(t)
		if overview {
			if !tool {
				ids = append(ids, i)
			}
			continue
		}
		if tool && !showTools {
			continue
		}
		ids = append(ids, i)
	}
	return ids
}

func (m model) activeIndices() []int {
	return activeIndices(m.turns, m.showTools, m.overview)
}

func (m model) currentOrig() int {
	ids := m.activeIndices()
	if m.turnCursor < 0 || m.turnCursor >= len(ids) {
		return -1
	}
	return ids[m.turnCursor]
}

func (m *model) clampTurnCursor() {
	n := len(m.activeIndices())
	if n == 0 {
		m.turnCursor = 0
		return
	}
	if m.turnCursor >= n {
		m.turnCursor = n - 1
	}
	if m.turnCursor < 0 {
		m.turnCursor = 0
	}
}

func (m *model) relayout() {
	if !m.vpReady {
		return
	}
	content, starts := m.layoutConversation()
	m.turnStarts = starts
	m.vp.SetContent(content)
}

func (m *model) scrollToTurn() {
	if m.turnCursor >= 0 && m.turnCursor < len(m.turnStarts) {
		m.vp.SetYOffset(m.turnStarts[m.turnCursor])
	}
}

func (m *model) refreshRead() {
	m.relayout()
	m.scrollToTurn()
}

func (m *model) moveTurn(delta int) {
	ids := m.activeIndices()
	if len(ids) == 0 {
		return
	}
	m.turnCursor += delta
	m.clampTurnCursor()
	m.refreshRead()
}

func (m *model) jumpToOrig(orig int) {
	if orig < 0 || orig >= len(m.turns) {
		return
	}
	if isToolTurn(m.turns[orig]) && !m.showTools {
		m.showTools = true
	}
	m.overview = false
	if m.expanded == nil {
		m.expanded = map[int]bool{}
	}
	m.expanded[orig] = true
	ids := m.activeIndices()
	m.turnCursor = 0
	for i, id := range ids {
		if id == orig {
			m.turnCursor = i
			break
		}
	}
	m.refreshRead()
}

func (m *model) toggleOverview() {
	orig := m.currentOrig()
	m.overview = !m.overview
	m.finding = false
	ids := m.activeIndices()
	m.turnCursor = 0
	for i, id := range ids {
		if id == orig {
			m.turnCursor = i
			break
		}
	}
	m.clampTurnCursor()
	m.refreshRead()
}

func (m *model) toggleExpand() {
	orig := m.currentOrig()
	if orig < 0 {
		return
	}
	if m.expanded == nil {
		m.expanded = map[int]bool{}
	}
	m.expanded[orig] = !m.expanded[orig]
	m.refreshRead()
}

func findTurns(turns []turn, q string) []int {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var hits []int
	for i, t := range turns {
		if strings.Contains(strings.ToLower(t.Text), q) {
			hits = append(hits, i)
		}
	}
	return hits
}

func (m *model) runFind() {
	q := strings.TrimSpace(m.input.Value())
	m.finding = false
	m.input.Blur()
	m.findQ = q
	if q == "" {
		m.findHits, m.findAt = nil, 0
		m.status = "find cancelled"
		return
	}
	m.findHits = findTurns(m.turns, q)
	if len(m.findHits) == 0 {
		m.findAt = 0
		m.status = "no matches"
		return
	}
	m.findAt = 0
	m.jumpToOrig(m.findHits[0])
	m.status = fmt.Sprintf("hit 1/%d", len(m.findHits))
}

func (m *model) stepHit(delta int) {
	if len(m.findHits) == 0 {
		m.status = "no search hits — press / to find"
		return
	}
	n := len(m.findHits)
	m.findAt = (m.findAt + delta) % n
	if m.findAt < 0 {
		m.findAt += n
	}
	m.jumpToOrig(m.findHits[m.findAt])
	m.status = fmt.Sprintf("hit %d/%d", m.findAt+1, n)
}

func (m *model) copyCurrentTurn() {
	orig := m.currentOrig()
	if orig < 0 {
		m.status = "no turn to copy"
		return
	}
	if err := copyToClipboard(m.turns[orig].Text); err != nil {
		m.err = err
		return
	}
	m.status, m.err = fmt.Sprintf("copied turn %d", m.turnCursor+1), nil
}

func (m model) updateRead(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.finding {
		switch msg.Type {
		case tea.KeyEsc:
			m.finding = false
			m.input.Blur()
			return m, nil
		case tea.KeyEnter:
			m.runFind()
			return m, nil
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "backspace":
		m.gen++ // abandon any reload still in flight for this conversation
		m.loading, m.finding = false, false
		m.screen = screenList
		return m, nil
	case "h", "left":
		// h/left leave the conversation (viewport would otherwise steal them
		// for horizontal scroll). Same as esc.
		m.gen++
		m.loading, m.finding = false, false
		m.screen = screenList
		return m, nil
	case "y":
		url := convURL(m.cur.ID)
		if err := copyToClipboard(url); err != nil {
			m.err = err
		} else {
			m.status, m.err = "copied: "+url, nil
		}
		return m, nil
	case "Y":
		m.copyCurrentTurn()
		return m, nil
	case "o":
		if err := openInBrowser(convURL(m.cur.ID)); err != nil {
			m.err = err
		} else {
			m.status, m.err = "opened in browser", nil
		}
		return m, nil
	case "t":
		m.showTools = !m.showTools
		m.clampTurnCursor()
		m.refreshRead()
		return m, nil
	case "r":
		m.loading, m.status, m.err = true, "reloading", nil
		m.gen++
		return m, tea.Batch(m.cfg.fetchTurns(m.gen, m.cur.ID, m.cur.Title), tick())
	case "v":
		m.toggleOverview()
		return m, nil
	case "n":
		m.moveTurn(1)
		return m, nil
	case "p":
		m.moveTurn(-1)
		return m, nil
	case "g", "home":
		m.turnCursor = 0
		m.refreshRead()
		return m, nil
	case "G", "end":
		ids := m.activeIndices()
		if len(ids) > 0 {
			m.turnCursor = len(ids) - 1
		}
		m.refreshRead()
		return m, nil
	case "e", "enter":
		if m.overview {
			orig := m.currentOrig()
			m.overview = false
			if orig >= 0 {
				if m.expanded == nil {
					m.expanded = map[int]bool{}
				}
				m.expanded[orig] = true
			}
			m.refreshRead()
			return m, nil
		}
		m.toggleExpand()
		return m, nil
	case "/":
		m.finding = true
		m.input.Placeholder = "find in conversation — enter to jump, esc to cancel"
		m.input.SetValue(m.findQ)
		m.input.Focus()
		return m, textinput.Blink
	case "]":
		m.stepHit(1)
		return m, nil
	case "[":
		m.stepHit(-1)
		return m, nil
	case "j", "down", "k", "up":
		if m.overview {
			if msg.String() == "j" || msg.String() == "down" {
				m.moveTurn(1)
			} else {
				m.moveTurn(-1)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// ── rendering ────────────────────────────────────────────────────────────────

type readOpts struct {
	width     int
	showTools bool
	overview  bool
	foldAt    int
	expanded  map[int]bool
	cursor    int
}

func (m model) layoutConversation() (string, []int) {
	width := m.w - 2
	if width < 10 {
		width = 10
	}
	foldAt := defaultFoldLines
	if m.vpReady && m.vp.Height > 0 && m.vp.Height-2 < foldAt {
		foldAt = m.vp.Height - 2
		if foldAt < 8 {
			foldAt = 8
		}
	}
	return layoutConversation(m.turns, readOpts{
		width:     width,
		showTools: m.showTools,
		overview:  m.overview,
		foldAt:    foldAt,
		expanded:  m.expanded,
		cursor:    m.turnCursor,
	})
}

func layoutConversation(turns []turn, opts readOpts) (string, []int) {
	// Never widen past the real terminal: an 80-column fallback on a 20-column
	// pane wraps in the terminal instead of in lipgloss, and the layout tears.
	width := opts.width
	if width < 10 {
		width = 10
	}
	ids := activeIndices(turns, opts.showTools, opts.overview)
	if len(ids) == 0 {
		if opts.overview {
			return cDim.Render("\n  no human turns\n"), nil
		}
		return cDim.Render("\n  nothing to show — press t to include tool turns\n"), nil
	}
	var b strings.Builder
	starts := make([]int, len(ids))
	line := 0
	for vis, orig := range ids {
		starts[vis] = line
		block := renderTurn(turns[orig], vis, len(ids), orig, opts, width)
		if !strings.HasSuffix(block, "\n") {
			block += "\n"
		}
		b.WriteString(block)
		line += strings.Count(block, "\n")
	}
	return b.String(), starts
}

func renderTurn(t turn, vis, total, orig int, opts readOpts, width int) string {
	stamp := ""
	if t.CreateTime > 0 {
		stamp = time.Unix(int64(t.CreateTime), 0).Format("01-02 15:04")
	}
	marker := "  "
	if vis == opts.cursor {
		marker = cSel.Render("▸ ")
	}
	tool := isToolTurn(t)

	if opts.overview {
		who := "GPT"
		switch {
		case tool:
			who = "tool"
		case t.Role == "user":
			who = "You"
		}
		preview := oneLine.Replace(sane(strings.TrimSpace(t.Text)))
		label := fmt.Sprintf("%2d %-4s %s  ", vis+1, who, stamp)
		line := marker + cDim.Render(label) + truncate(preview, width-lipgloss.Width(label)-2)
		return line + "\n"
	}

	var head string
	switch {
	case tool:
		head = cTool.Render(fmt.Sprintf("── %s → %s", t.Role, t.Recipient))
	case t.Role == "user":
		head = cUser.Render("── You")
	default:
		label := "── ChatGPT"
		if t.Model != "" {
			label += " (" + t.Model + ")"
		}
		head = cAsst.Render(label)
	}
	idx := cDim.Render(fmt.Sprintf("%d/%d", vis+1, total))
	var b strings.Builder
	b.WriteString(marker + head + "  " + idx + "  " + cDim.Render(stamp) + "\n")
	body := renderTurnBody(t.Text, width)
	expanded := opts.expanded != nil && opts.expanded[orig]
	body = foldBody(body, opts.foldAt, expanded)
	if body != "" {
		b.WriteString(body)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func isFenceLine(s string) bool {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	rest := s[i:]
	return strings.HasPrefix(rest, "```") || strings.HasPrefix(rest, "~~~")
}

// renderTurnBody wraps prose to width and leaves fenced code as one visual
// line per source line. Word-wrapping a fence is what made 43 code blocks in
// one conversation unreadable: spaces inside the fence became wrap points.
func renderTurnBody(text string, width int) string {
	text = sane(strings.TrimRight(text, "\n"))
	if text == "" {
		return ""
	}
	if width < 10 {
		width = 10
	}
	wrap := lipgloss.NewStyle().Width(width)
	var b strings.Builder
	var prose strings.Builder
	flush := func() {
		if prose.Len() == 0 {
			return
		}
		rendered := wrap.Render(prose.String())
		b.WriteString(rendered)
		if !strings.HasSuffix(rendered, "\n") {
			b.WriteByte('\n')
		}
		prose.Reset()
	}
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if isFenceLine(line) {
			flush()
			inFence = !inFence
			b.WriteString(cCode.Render(cutLine(line, width)))
			b.WriteByte('\n')
			continue
		}
		if inFence {
			b.WriteString(cCode.Render(cutLine(line, width)))
			b.WriteByte('\n')
			continue
		}
		if prose.Len() > 0 {
			prose.WriteByte('\n')
		}
		prose.WriteString(line)
	}
	flush()
	return strings.TrimRight(b.String(), "\n")
}

func foldBody(rendered string, foldAt int, expanded bool) string {
	if foldAt <= 0 || expanded || rendered == "" {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) <= foldAt {
		return rendered
	}
	keep := lines[:foldAt]
	msg := cDim.Render(fmt.Sprintf("… %d more lines  [e] expand", len(lines)-foldAt))
	return strings.Join(keep, "\n") + "\n" + msg
}

// sane strips runes a terminal cannot measure. ChatGPT titles carry unassigned
// code points (measured 2026-09-10: a live title ended in U+7FFFF); lipgloss.Width
// reports 1 for them while the terminal draws 2, which pushed a whole row off screen.
func sane(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || unicode.IsGraphic(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// oneLine collapses the vertical whitespace a single-row cell cannot hold.
// Measured 2026-09-10: two live titles ended in U+5FFFF followed by a newline, and
// that trailing newline from the server pushed the whole list down a row, scrolling
// the header off screen. Server data is not guaranteed to be one line; the renderer
// guards.
var oneLine = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")

func fitWidth(s string, w int) string {
	if w <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	out := []rune{}
	acc := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if acc+rw > w-1 {
			break
		}
		out = append(out, r)
		acc += rw
	}
	return string(out) + "…"
}

func truncate(s string, w int) string {
	return fitWidth(oneLine.Replace(sane(s)), w)
}

// cutLine trims a single visual line to width without collapsing tabs or
// newlines (code lines keep their whitespace). oneLine is the list-cell guard;
// this is the fence guard.
func cutLine(s string, w int) string {
	return fitWidth(sane(s), w)
}

func (m model) View() string {
	if m.w == 0 {
		return "…"
	}
	if m.w < minCols || m.h < minRows {
		return truncate(fmt.Sprintf("terminal too small (%dx%d, need %dx%d)",
			m.w, m.h, minCols, minRows), m.w)
	}
	switch m.screen {
	case screenSearch:
		return "\n" + cTitle.Render("  search") + "\n\n  " + m.input.View() +
			"\n\n" + cDim.Render("  the server is canonical — nothing is written locally")
	case screenRead:
		head := truncate(fmt.Sprintf(" %s ", m.cur.Title), m.w)
		ids := m.activeIndices()
		idx, n := 0, len(ids)
		if n > 0 {
			idx = m.turnCursor + 1
		}
		mode := "read"
		if m.overview {
			mode = "overview"
		}
		tools := "t-"
		if m.showTools {
			tools = "t+"
		}
		hit := ""
		if len(m.findHits) > 0 {
			hit = fmt.Sprintf(" hit %d/%d", m.findAt+1, len(m.findHits))
		}
		bar := fmt.Sprintf(" %d/%d %s %s%s",
			idx, n, mode, tools, hit)
		if m.loading {
			bar = spinChars[m.spin] + " " + m.status
		} else if m.finding {
			bar = " find " + m.input.View()
		} else if m.err != nil {
			bar = fmt.Sprintf(" %d/%d error: %s", idx, n, m.err.Error())
		} else {
			if m.status != "" {
				bar += " · " + m.status
			}
			bar += " · n/p v / ][ Y e t y esc"
		}
		foot := cBar.Render(truncate(bar, m.w))
		if m.err != nil && !m.loading && !m.finding {
			foot = cErr.Render(truncate(bar, m.w))
		}
		return cHeadBar.Width(m.w).Render(head) + "\n" + m.vp.View() + "\n" + foot

	case screenProjects:
		var b strings.Builder
		b.WriteString(cHeadBar.Width(m.w).Render(" select project") + "\n")
		rows := m.listRows()
		end := m.poffset + rows
		if end > len(m.projects) {
			end = len(m.projects)
		}
		lines := make([]string, 0, rows)
		for i := m.poffset; i < end; i++ {
			it := m.projects[i]
			mark := "  "
			if it.ID == m.scopeID {
				mark = cSel.Render("● ")
			}
			title := truncate(it.Title, m.w-24)
			if i == m.pcursor {
				title = cSel.Render(title)
			}
			lines = append(lines, mark+cDim.Render(fmt.Sprintf("%-16s", shortTime(it.UpdateTime)))+" "+title)
		}
		for len(lines) < rows {
			lines = append(lines, "")
		}
		b.WriteString(strings.Join(lines[:rows], "\n") + "\n")
		b.WriteString(cBar.Render(truncate(" "+m.status, m.w)) + "\n")
		b.WriteString(cDim.Render(truncate(" [enter] select  [j/k] move  [esc] back", m.w)))
		return b.String()
	}

	// list
	var b strings.Builder
	head := " cwatui — ChatGPT web session · the server is canonical"
	if m.scopeName != "" {
		head = " cwatui — project: " + m.scopeName
	}
	b.WriteString(cHeadBar.Width(m.w).Render(truncate(head, m.w)) + "\n")

	rows := m.listRows()
	window := rows - 1
	if window < 1 {
		window = 1
	}
	if len(m.items) == 0 && !m.loading {
		b.WriteString(cDim.Render("\n  no conversations — press / to search or r to reload\n"))
	}
	end := m.offset + window
	if end > len(m.items) {
		end = len(m.items)
	}
	lines := make([]string, 0, rows)
	for i := m.offset; i < end; i++ {
		it := m.items[i]
		stamp := cDim.Render(shortTime(it.UpdateTime)) + "  "
		titleWidth := m.w - 24
		if m.w < 44 { // no room for both; the title is what identifies a conversation
			stamp = ""
			titleWidth = m.w - 4
		}
		prefix := "  "
		title := it.Title
		if i == m.cursor {
			prefix = cSel.Render("▸ ")
			title = cSel.Render(truncate(it.Title, titleWidth))
		} else {
			title = truncate(it.Title, titleWidth)
		}
		lines = append(lines, prefix+stamp+title)
		if it.Snippet != "" && i == m.cursor {
			lines = append(lines, "     "+cDim.Render(truncate(it.Snippet, m.w-6)))
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	b.WriteString(strings.Join(lines[:rows], "\n") + "\n")

	status := m.status
	if m.loading {
		status = spinChars[m.spin] + " " + status
	}
	if m.err != nil {
		status = cErr.Render("error: " + truncate(m.err.Error(), m.w-8))
	}
	b.WriteString(cBar.Render(truncate(" "+status, m.w)) + "\n")
	b.WriteString(cDim.Render(truncate(" [p] projects · [/] search · [enter] open · [y] url · [o] open · [r] reload · [q] quit", m.w)))
	return b.String()
}

func main() {
	cfg := loadConfig()
	for _, p := range []struct{ what, path string }{
		{"CWA CLI", cfg.cwaBin}, {"python", cfg.pyBin},
		{"cwaq shim", cfg.cwaqBin}, {"auth file", cfg.authFile},
	} {
		if _, err := os.Stat(p.path); err != nil {
			fmt.Fprintf(os.Stderr, "cannot find %s: %s\n", p.what, p.path)
			fmt.Fprintln(os.Stderr, "set CWA_BIN / CWA_PY / CWAQ_BIN / CWA_AUTH to override.")
			os.Exit(2)
		}
	}
	p := tea.NewProgram(newModel(cfg), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
