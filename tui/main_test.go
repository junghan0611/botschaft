package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Fixtures are synthetic. They do not reproduce any live conversation.

func human(role, text string) turn {
	return turn{Role: role, Recipient: "all", Text: text}
}

func tool(text string) turn {
	return turn{Role: "assistant", Recipient: "browser", Text: text}
}

func TestIsToolTurnSplitsOnRecipient(t *testing.T) {
	if isToolTurn(human("user", "x")) {
		t.Fatal("user with recipient all is not a tool turn")
	}
	if isToolTurn(human("assistant", "x")) {
		t.Fatal("assistant with recipient all is not a tool turn")
	}
	if !isToolTurn(tool("x")) {
		t.Fatal("recipient != all is a tool turn")
	}
	if !isToolTurn(turn{Role: "tool", Recipient: "all", Text: "x"}) {
		t.Fatal("role tool is a tool turn even with recipient all")
	}
}

func TestFenceDoesNotWordWrap(t *testing.T) {
	long := strings.Repeat("alpha beta ", 30) // spaces would wrap if treated as prose
	text := "```\n" + long + "\n```"
	got := renderTurnBody(text, 40)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("fenced block became %d lines; want 3 (open, body, close)\n%s", len(lines), got)
	}
	if lipgloss.Width(lines[1]) > 40 {
		t.Fatalf("code line wider than pane: %d", lipgloss.Width(lines[1]))
	}
	if strings.Contains(strings.TrimSuffix(lines[1], "…"), "\n") {
		t.Fatal("code line split internally")
	}
}

func TestProseStillWraps(t *testing.T) {
	got := renderTurnBody(strings.Repeat("alpha beta ", 30), 40)
	if strings.Count(got, "\n") < 5 {
		t.Fatalf("prose should wrap at width 40; got %d newlines\n%s", strings.Count(got, "\n"), got)
	}
}

func TestTildeFenceDoesNotWordWrap(t *testing.T) {
	long := strings.Repeat("alpha beta ", 30)
	got := renderTurnBody("~~~\n"+long+"\n~~~", 40)
	if gotLines := strings.Split(got, "\n"); len(gotLines) != 3 {
		t.Fatalf("tilde fence became %d lines; want 3", len(gotLines))
	}
}

func TestFoldLongTurn(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("line\n")
	}
	turns := []turn{human("user", b.String())}
	folded, _ := layoutConversation(turns, readOpts{width: 80, foldAt: 20, cursor: 0})
	if !strings.Contains(folded, "more lines") {
		t.Fatalf("expected fold marker, got:\n%s", folded)
	}
	if strings.Count(folded, "\nline") > 25 {
		t.Fatal("folded body still shows most lines")
	}
	open, _ := layoutConversation(turns, readOpts{
		width: 80, foldAt: 20, expanded: map[int]bool{0: true}, cursor: 0,
	})
	if strings.Contains(open, "more lines") {
		t.Fatal("expanded turn should not fold")
	}
	if strings.Count(open, "line") < 50 {
		t.Fatalf("expanded turn lost lines: %d", strings.Count(open, "line"))
	}
}

func TestOverviewIsOneLinePerHumanTurn(t *testing.T) {
	turns := []turn{
		human("user", "first human turn\nwith a second line"),
		tool(strings.Repeat("tool payload ", 40)),
		tool("another tool"),
		human("assistant", "visible reply"),
		tool("third tool"),
	}
	content, starts := layoutConversation(turns, readOpts{width: 80, overview: true, foldAt: 20})
	if len(starts) != 2 {
		t.Fatalf("overview should keep 2 human turns, got %d starts", len(starts))
	}
	lines := nonEmpty(strings.Split(strings.TrimRight(content, "\n"), "\n"))
	if len(lines) != 2 {
		t.Fatalf("overview should be 2 lines, got %d:\n%s", len(lines), content)
	}
	for i, ln := range lines {
		if strings.Count(ln, "\n") != 0 {
			t.Fatalf("overview line %d is not one line", i)
		}
	}
	if !strings.Contains(content, "first human turn") {
		t.Fatal("overview lost the first-line preview")
	}
	if strings.Contains(content, "tool payload") {
		t.Fatal("overview leaked a tool turn")
	}
}

func TestTurnStartsTrackJumpTargets(t *testing.T) {
	turns := []turn{
		human("user", "aaa"),
		human("assistant", "bbb\nccc"),
		human("user", "ddd"),
	}
	_, starts := layoutConversation(turns, readOpts{width: 80, foldAt: 20, cursor: 0})
	if len(starts) != 3 {
		t.Fatalf("want 3 starts, got %d", len(starts))
	}
	if starts[0] != 0 {
		t.Fatalf("first turn should start at 0, got %d", starts[0])
	}
	for i := 1; i < len(starts); i++ {
		if starts[i] <= starts[i-1] {
			t.Fatalf("starts not increasing: %v", starts)
		}
	}
}

func TestFindTurns(t *testing.T) {
	turns := []turn{
		human("user", "alpha"),
		tool("alpha hidden"),
		human("assistant", "beta"),
	}
	hits := findTurns(turns, "ALPHA")
	if len(hits) != 2 || hits[0] != 0 || hits[1] != 1 {
		t.Fatalf("find should be case-insensitive over all turns, got %v", hits)
	}
	if findTurns(turns, "zzz") != nil && len(findTurns(turns, "zzz")) != 0 {
		t.Fatal("expected no hits")
	}
}

func TestNAndPMoveTurnCursor(t *testing.T) {
	m := testReadModel([]turn{
		human("user", "one"),
		human("assistant", "two"),
		human("user", "three"),
	}, 80, 24)
	if m.turnCursor != 0 {
		t.Fatalf("start at 0, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "n")
	if m.turnCursor != 1 {
		t.Fatalf("n -> 1, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "n")
	if m.turnCursor != 2 {
		t.Fatalf("n -> 2, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "n")
	if m.turnCursor != 2 {
		t.Fatalf("n at end stays, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "p")
	if m.turnCursor != 1 {
		t.Fatalf("p -> 1, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "g")
	if m.turnCursor != 0 {
		t.Fatalf("g -> 0, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "G")
	if m.turnCursor != 2 {
		t.Fatalf("G -> last, got %d", m.turnCursor)
	}
}

func TestOverviewToggleAndEnter(t *testing.T) {
	m := testReadModel([]turn{
		human("user", "one"),
		tool("hidden"),
		human("assistant", "two"),
	}, 80, 24)
	m = sendKey(t, m, "v")
	if !m.overview {
		t.Fatal("v should enter overview")
	}
	if len(m.activeIndices()) != 2 {
		t.Fatalf("overview should list 2 human turns, got %d", len(m.activeIndices()))
	}
	content := m.vp.View()
	if strings.Contains(content, "hidden") {
		t.Fatal("overview showed a tool turn")
	}
	m = sendKey(t, m, "n")
	if m.turnCursor != 1 {
		t.Fatalf("n in overview -> 1, got %d", m.turnCursor)
	}
	m = sendKey(t, m, "enter")
	if m.overview {
		t.Fatal("enter should leave overview")
	}
	if m.turnCursor != 1 {
		t.Fatalf("enter should keep the current human turn, got %d", m.turnCursor)
	}
}

func TestFindJumpsAndRevealsToolTurn(t *testing.T) {
	m := testReadModel([]turn{
		human("user", "visible"),
		tool("secret-token"),
		human("assistant", "reply"),
	}, 80, 24)
	if m.showTools {
		t.Fatal("tools start hidden")
	}
	m.input.SetValue("secret-token")
	m.runFind()
	if !m.showTools {
		t.Fatal("a hit in a tool turn should reveal tools")
	}
	if m.currentOrig() != 1 {
		t.Fatalf("should land on the tool turn, orig=%d", m.currentOrig())
	}
	if len(m.findHits) != 1 {
		t.Fatalf("hits=%v", m.findHits)
	}
}

func TestSaneAndOneLineStillGuardOverview(t *testing.T) {
	// Server titles/bodies are not one line. Overview must not inherit a newline.
	turns := []turn{human("user", "hello\nworld\x00\uFFFD")}
	content, _ := layoutConversation(turns, readOpts{width: 80, overview: true, foldAt: 20})
	if strings.Count(strings.TrimRight(content, "\n"), "\n") != 0 {
		t.Fatalf("overview leaked a newline:\n%q", content)
	}
}

func testReadModel(turns []turn, w, h int) model {
	in := textinput.New()
	m := model{
		screen:   screenRead,
		turns:    turns,
		expanded: map[int]bool{},
		w:        w,
		h:        h,
		vpReady:  true,
		input:    in,
		cur:      conv{ID: "fixture", Title: "fixture"},
	}
	m.vp = viewport.New(w, h-3)
	m.refreshRead()
	return m
}

func sendKey(t *testing.T, m model, k string) model {
	t.Helper()
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	next, _ := m.Update(msg)
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T", next)
	}
	return got
}

func nonEmpty(lines []string) []string {
	var out []string
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}
