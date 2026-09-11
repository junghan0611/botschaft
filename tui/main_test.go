package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
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
	compose := textarea.New()
	compose.ShowLineNumbers = false
	m := model{
		screen:   screenRead,
		turns:    turns,
		expanded: map[int]bool{},
		drafts:   map[string]string{},
		receipts: map[string]sendReceipt{},
		w:        w,
		h:        h,
		vpReady:  true,
		input:    in,
		compose:  compose,
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
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+d":
		msg = tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+e":
		msg = tea.KeyMsg{Type: tea.KeyCtrlE}
	case "ctrl+s":
		msg = tea.KeyMsg{Type: tea.KeyCtrlS}
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

// The turn index in the status bar, the highlighted turn and what Y copies must
// all name the same turn. Free scrolling with j/k moves the viewport without
// touching turnCursor, so before this was fixed the bar said "1/3" while turn 3
// filled the screen and Y copied turn 1.
func TestFreeScrollMovesTheTurnCursor(t *testing.T) {
	body := strings.Repeat("line\n", 30) // each turn is taller than the pane
	m := testReadModel([]turn{
		human("user", "first "+body),
		human("assistant", "second "+body),
		human("user", "third "+body),
	}, 80, 24)
	m.expanded = map[int]bool{0: true, 1: true, 2: true}
	m.refreshRead()

	if got := m.currentOrig(); got != 0 {
		t.Fatalf("starts on turn 0, got %d", got)
	}
	// Scroll far enough to land inside the last turn.
	for i := 0; i < 40; i++ {
		m = sendKey(t, m, "j")
	}
	// Computed inline, not with the helper under test, so this fails against a
	// build that has no such helper rather than failing to compile.
	want := 0
	for i, start := range m.turnStarts {
		if start > m.vp.YOffset {
			break
		}
		want = i
	}
	if want == 0 {
		t.Fatalf("the scroll did not leave the first turn; starts=%v offset=%d",
			m.turnStarts, m.vp.YOffset)
	}
	if m.turnCursor != want {
		t.Fatalf("after free scroll the cursor is on turn %d but the viewport shows turn %d",
			m.turnCursor, want)
	}
	if m.currentOrig() != want {
		t.Fatalf("Y would copy turn %d while the viewport shows turn %d", m.currentOrig(), want)
	}
}

func TestComposeDraftPersistsPerConversation(t *testing.T) {
	m := testReadModel([]turn{human("user", "existing")}, 80, 24)
	m = sendKey(t, m, "c")
	m.compose.SetValue("first line\nsecond line")
	m = sendKey(t, m, "esc")
	if got := m.drafts["fixture"]; got != "first line\nsecond line" {
		t.Fatalf("leaving compose lost draft: %q", got)
	}

	m.cur = conv{ID: "other", Title: "other"}
	m = sendKey(t, m, "c")
	m.compose.SetValue("other draft")
	m = sendKey(t, m, "esc")
	m.cur = conv{ID: "fixture", Title: "fixture"}
	m = sendKey(t, m, "c")
	if got := m.compose.Value(); got != "first line\nsecond line" {
		t.Fatalf("reopened conversation got draft %q", got)
	}
}

func TestDryRunDoesNotExecuteAndUsesExactArgv(t *testing.T) {
	m := testReadModel(nil, 80, 24)
	m.cfg = config{cwaBin: "fake-cwa", authFile: "fixture-auth"}
	m = sendKey(t, m, "c")
	m.compose.SetValue("line one\nit's line two")

	var command tea.Cmd
	next, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = next.(model)
	if command != nil {
		t.Fatal("dry-run returned a command; it must not execute anything")
	}
	wantArgv := []string{"send", "--conversation", "fixture",
		"--transport", "browserless-request", "--stream",
		"--auth-file", "fixture-auth", "--", "line one\nit's line two"}
	if got := m.cfg.sendArgv(m.compose.Value(), m.cur.ID); !reflect.DeepEqual(got, wantArgv) {
		t.Fatalf("send argv mismatch:\n got %#v\nwant %#v", got, wantArgv)
	}
	if want := shellCommand("fake-cwa", wantArgv); m.dryRun != want {
		t.Fatalf("dry-run mismatch:\n got %q\nwant %q", m.dryRun, want)
	}
}

func TestEditorDraftIsPrivateAndExact(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.Chmod(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	path, err := createEditorDraft("first line\nsecond line\n")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if got := filepath.Dir(path); got != runtimeDir {
		t.Fatalf("editor draft dir=%q, want private runtime dir %q", got, runtimeDir)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("editor draft mode=%#o, want 0600", got)
	}
	content, err := consumeEditorDraft(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(content); got != "first line\nsecond line\n" {
		t.Fatalf("editor draft changed content: %q", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("consumed editor draft still exists: %v", err)
	}
}

func TestSendArgvProtectsDashPrefixedDraft(t *testing.T) {
	cfg := config{cwaBin: "fake-cwa", authFile: "fixture-auth"}
	got := cfg.sendArgv("-hello", "fixture")
	want := []string{"send", "--conversation", "fixture",
		"--transport", "browserless-request", "--stream",
		"--auth-file", "fixture-auth", "--", "-hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dash-prefixed draft is not protected from option parsing:\n got %#v\nwant %#v", got, want)
	}
}

func processExitError(t *testing.T, command string) error {
	t.Helper()
	err := exec.Command("sh", "-c", command).Run()
	if err == nil {
		t.Fatalf("helper command unexpectedly succeeded: %s", command)
	}
	return err
}

func TestSendFailuresPreserveDraftAndNeverAutoRetry(t *testing.T) {
	cases := []struct {
		name      string
		err       func(*testing.T) error
		ambiguous bool
	}{
		{"usage exit 2", func(t *testing.T) error { return processExitError(t, "exit 2") }, false},
		{"operation exit 3", func(t *testing.T) error { return processExitError(t, "exit 3") }, false},
		{"reconciliation exit 4", func(t *testing.T) error { return processExitError(t, "exit 4") }, true},
		{"signal cancellation", func(t *testing.T) error {
			return processExitError(t, "kill -TERM $$")
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := testReadModel(nil, 80, 24)
			m.screen = screenCompose
			m.compose.SetValue("draft body")
			m.saveDraft()
			m.compose.Blur()
			m.sending, m.sendSeq = true, 7
			m.receipts["fixture"] = sendReceipt{
				kind: receiptInFlight, submitted: "draft body", boundary: 0, seq: 7,
			}
			next, command := m.Update(sendEventMsg{
				gen: 0, seq: 7, id: "fixture",
				event: sendEvent{done: true, err: tc.err(t)},
			})
			m = next.(model)
			if command != nil {
				t.Fatal("failed send scheduled another command")
			}
			if got := m.drafts["fixture"]; got != "draft body" {
				t.Fatalf("failed send changed draft: %q", got)
			}
			gotReceipt, hasReceipt := m.receipts["fixture"]
			if got := hasReceipt && gotReceipt.kind == receiptAmbiguous; got != tc.ambiguous {
				t.Fatalf("ambiguous receipt=%v, want %v", got, tc.ambiguous)
			}
			if !tc.ambiguous {
				before := m.compose.Value()
				next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
				m = next.(model)
				if m.compose.Value() == before {
					t.Fatal("definite send failure left the preserved draft uneditable")
				}
			}
		})
	}
}

func TestSendStartFailureLeavesDraftEditable(t *testing.T) {
	m := testReadModel(nil, 80, 24)
	m.screen = screenCompose
	m.compose.SetValue("draft body")
	m.saveDraft()
	m.compose.Blur()
	m.sending, m.sendSeq = true, 8
	m.receipts["fixture"] = sendReceipt{
		kind: receiptInFlight, submitted: "draft body", boundary: 0, seq: 8,
	}

	next, command := m.Update(sendStartedMsg{
		gen: 0, seq: 8, id: "fixture", err: os.ErrNotExist,
	})
	m = next.(model)
	if command != nil {
		t.Fatal("start failure scheduled another command")
	}
	before := m.compose.Value()
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = next.(model)
	if m.compose.Value() == before {
		t.Fatal("send start failure left the preserved draft uneditable")
	}
}

func TestSuccessfulSendReadsCanonicalAndOnlyThenClearsDraft(t *testing.T) {
	m := testReadModel(nil, 80, 24)
	m.cfg = config{cwaBin: "fake-cwa", authFile: "fixture-auth"}
	m.screen = screenCompose
	m.compose.Focus()
	m.compose.SetValue("reply body")
	m.saveDraft()
	m.sending, m.sendSeq = true, 11
	m.receipts["fixture"] = sendReceipt{
		kind: receiptInFlight, submitted: "reply body", boundary: 0, seq: 11,
	}

	events := make(chan sendEvent)
	next, waitMore := m.Update(sendEventMsg{
		gen: 0, seq: 11, id: "fixture", events: events,
		event: sendEvent{stdout: "transient"},
	})
	m = next.(model)
	if waitMore == nil || m.streamOut != "transient" {
		t.Fatalf("stream chunk was not exposed incrementally: %q", m.streamOut)
	}
	if view := m.View(); !strings.Contains(view, "transient") {
		t.Fatalf("compose view did not show stream chunk:\n%s", view)
	}
	next, readback := m.Update(sendEventMsg{
		gen: 0, seq: 11, id: "fixture", events: events,
		event: sendEvent{done: true},
	})
	m = next.(model)
	if readback == nil {
		t.Fatal("exit 0 did not schedule canonical messages readback")
	}
	if got := m.drafts["fixture"]; got != "reply body" {
		t.Fatalf("draft cleared before canonical readback: %q", got)
	}
	wantReadArgv := []string{"messages", "fixture", "--json", "--auth-file",
		"fixture-auth", "--limit", "1000"}
	if got := m.cfg.messagesArgv("fixture"); !reflect.DeepEqual(got, wantReadArgv) {
		t.Fatalf("canonical read argv mismatch:\n got %#v\nwant %#v", got, wantReadArgv)
	}

	next, _ = m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true,
		turns: []turn{human("user", "reply body"), human("assistant", "canonical")}})
	m = next.(model)
	if _, ok := m.drafts["fixture"]; ok || m.compose.Value() != "" {
		t.Fatal("successful canonical readback did not clear draft")
	}
	if m.screen != screenRead || len(m.turns) != 2 {
		t.Fatalf("did not return to canonical read view: screen=%v turns=%d", m.screen, len(m.turns))
	}
	if m.streamOut != "" {
		t.Fatalf("transient stream survived canonical replacement: %q", m.streamOut)
	}
}

func TestExitZeroReadbackMustConfirmSubmittedTurn(t *testing.T) {
	prior := []turn{human("user", "old prompt"), human("assistant", "old answer")}
	m := testReadModel(prior, 80, 24)
	m.screen = screenCompose
	m.compose.SetValue("reply body")
	m.saveDraft()
	m.receipts["fixture"] = sendReceipt{
		kind: receiptNeedsReadback, submitted: "reply body", boundary: len(prior), seq: 1,
	}
	m.readback = true

	next, _ := m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true, turns: prior})
	m = next.(model)
	if _, unresolved := m.receipts["fixture"]; !unresolved {
		t.Fatal("stale canonical readback cleared a completed-send receipt")
	}
	if got := m.drafts["fixture"]; got != "reply body" {
		t.Fatalf("stale canonical readback cleared the draft: %q", got)
	}
	if m.err == nil || !strings.Contains(m.err.Error(), "does not confirm") {
		t.Fatalf("stale canonical readback was not reported: %v", m.err)
	}
}

func TestReadbackFailureKeepsDraftAndDisablesResend(t *testing.T) {
	m := testReadModel(nil, 80, 24)
	m.screen = screenCompose
	m.compose.SetValue("reply body")
	m.saveDraft()
	m.receipts["fixture"] = sendReceipt{
		kind: receiptNeedsReadback, submitted: "reply body", boundary: 0, seq: 1,
	}
	m.readback = true

	next, command := m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true,
		err: processExitError(t, "exit 3")})
	m = next.(model)
	if command != nil {
		t.Fatal("readback failure scheduled an automatic retry")
	}
	if got := m.drafts["fixture"]; got != "reply body" {
		t.Fatalf("readback failure changed draft: %q", got)
	}
	next, command = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(model)
	if command != nil || m.sending {
		t.Fatal("send was re-enabled before explicit canonical resync")
	}

	beforeGen := m.gen
	next, command = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = next.(model)
	if command == nil || !m.readback || m.gen != beforeGen+1 {
		t.Fatal("compose screen could not retry canonical reconciliation")
	}
}

func TestCanonicalReadbackPreservesPostSubmitEdit(t *testing.T) {
	m := testReadModel(nil, 80, 24)
	m.screen = screenCompose
	m.compose.SetValue("edited after submit")
	m.drafts["fixture"] = "edited after submit"
	m.receipts["fixture"] = sendReceipt{
		kind: receiptNeedsReadback, submitted: "submitted text", boundary: 0, seq: 1,
	}

	m.sending = true
	before := m.compose.Value()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = next.(model)
	if got := m.compose.Value(); got != before {
		t.Fatalf("draft changed while send was in flight: %q", got)
	}
	m.sending = false

	next, _ = m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true,
		turns: []turn{human("user", "submitted text"), human("assistant", "answer")}})
	m = next.(model)
	if got := m.drafts["fixture"]; got != "edited after submit" {
		t.Fatalf("canonical readback erased post-submit edit: %q", got)
	}
	if _, unresolved := m.receipts["fixture"]; unresolved {
		t.Fatal("definitive successful send receipt was not resolved")
	}
}

func TestCanonicalConfirmationUsesMessagesWhitespaceNormalization(t *testing.T) {
	prior := []turn{human("assistant", "old answer")}
	m := testReadModel(prior, 80, 24)
	m.screen = screenCompose
	m.compose.SetValue("reply body\n")
	m.drafts["fixture"] = "reply body\n"
	m.receipts["fixture"] = sendReceipt{
		kind: receiptNeedsReadback, submitted: "reply body\n", boundary: len(prior), seq: 1,
	}

	confirmed := append(append([]turn{}, prior...),
		human("user", "reply body"), human("assistant", "new answer"))
	next, _ := m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true, turns: confirmed})
	m = next.(model)
	if _, unresolved := m.receipts["fixture"]; unresolved {
		t.Fatal("messages whitespace normalization prevented canonical confirmation")
	}
	if _, draft := m.drafts["fixture"]; draft {
		t.Fatal("confirmed exact submitted draft was not cleared")
	}
}

func TestAmbiguousReconcileRequiresNewUserAndAssistantAfterBoundary(t *testing.T) {
	prior := []turn{human("user", "repeated prompt"), human("assistant", "old answer")}
	m := testReadModel(prior, 80, 24)
	m.screen = screenCompose
	m.compose.SetValue("repeated prompt")
	m.drafts["fixture"] = "repeated prompt"
	m.receipts["fixture"] = sendReceipt{
		kind: receiptAmbiguous, submitted: "repeated prompt", boundary: len(prior), seq: 1,
	}

	next, _ := m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true, turns: prior})
	m = next.(model)
	if _, unresolved := m.receipts["fixture"]; !unresolved {
		t.Fatal("matching prompt before boundary incorrectly reconciled ambiguous send")
	}
	onlyUser := append(append([]turn{}, prior...), human("user", "repeated prompt"))
	next, _ = m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true, turns: onlyUser})
	m = next.(model)
	if _, unresolved := m.receipts["fixture"]; !unresolved {
		t.Fatal("new user turn without assistant reply incorrectly reconciled ambiguous send")
	}
	confirmed := append(onlyUser, human("assistant", "new answer"))
	next, _ = m.Update(turnsMsg{gen: 0, id: "fixture", afterSend: true, turns: confirmed})
	m = next.(model)
	if _, unresolved := m.receipts["fixture"]; unresolved {
		t.Fatal("new submitted user turn plus assistant reply did not reconcile")
	}
}

func TestComposeBlockedDuringReloadAndOrdinaryResultCannotCrossSend(t *testing.T) {
	m := testReadModel([]turn{human("user", "canonical before")}, 80, 24)
	m.loading = true
	m = sendKey(t, m, "c")
	if m.screen != screenRead {
		t.Fatal("compose opened while canonical reload was in flight")
	}

	m.loading = false
	m = sendKey(t, m, "c")
	m.compose.SetValue("reply")
	beforeGen := m.gen
	next, send := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(model)
	if send == nil || m.gen != beforeGen+1 {
		t.Fatal("send did not claim a fresh generation")
	}
	next, _ = m.Update(turnsMsg{gen: m.gen, id: "fixture",
		turns: []turn{human("user", "stale reload")}})
	m = next.(model)
	if len(m.turns) != 1 || m.turns[0].Text != "canonical before" {
		t.Fatal("ordinary reload result crossed an active send")
	}
}

func TestFakeSubprocessCancellationDrainsToAmbiguousTerminalEvent(t *testing.T) {
	path := t.TempDir() + "/fake-cwa"
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf transient\nwhile :; do :; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	m := testReadModel(nil, 80, 24)
	m.cfg = config{cwaBin: path, authFile: "fixture-auth"}
	m = sendKey(t, m, "c")
	m.compose.SetValue("synthetic draft")
	next, start := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(model)
	if start == nil {
		t.Fatal("send did not start fake subprocess")
	}
	started, ok := start().(sendStartedMsg)
	if !ok {
		t.Fatalf("start command returned unexpected message type")
	}
	next, waitEvent := m.Update(started)
	m = next.(model)
	next, quit := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(model)
	if quit != nil || !m.cancelRequested {
		t.Fatal("Ctrl+C quit instead of requesting cancellation")
	}
	for m.sending {
		event := waitEvent()
		next, waitEvent = m.Update(event)
		m = next.(model)
	}
	receipt, unresolved := m.receipts["fixture"]
	if !unresolved || receipt.kind != receiptAmbiguous {
		t.Fatalf("cancel ended without ambiguous receipt: %#v", receipt)
	}
	if got := m.drafts["fixture"]; got != "synthetic draft" {
		t.Fatalf("cancel changed draft: %q", got)
	}
}
