package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/config"
	"github.com/olekgolus11/SliceDiff/internal/diff"
	"github.com/olekgolus11/SliceDiff/internal/github"
)

func TestShouldStack(t *testing.T) {
	if shouldStack(120, 40) {
		t.Fatal("did not expect wide terminal to stack")
	}
	if !shouldStack(99, 40) {
		t.Fatal("expected narrow terminal to stack")
	}
	if !shouldStack(120, 29) {
		t.Fatal("expected short terminal to stack")
	}
}

func TestWeightedWidths(t *testing.T) {
	left, center, right := weightedWidths(100, []int{1, 2, 2})
	if left+center+right != 100 {
		t.Fatalf("widths do not sum to total: %d %d %d", left, center, right)
	}
	if left != 20 || center != 40 || right != 40 {
		t.Fatalf("unexpected widths: %d %d %d", left, center, right)
	}
}

func TestGroupedSliceChangeResetsHunkAndViewportOffsets(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.focus = panelLeft
	m.selectedSlice = 0
	m.selectedHunk = 2
	m.centerViewport.SetHeight(3)
	m.rightViewport.SetHeight(3)
	m.centerViewport.GotoBottom()
	m.rightViewport.GotoBottom()

	m.moveSelection(1)

	if m.selectedSlice != 1 {
		t.Fatalf("expected selected slice 1, got %d", m.selectedSlice)
	}
	if m.selectedHunk != 0 {
		t.Fatalf("expected hunk reset to 0, got %d", m.selectedHunk)
	}
	if m.centerViewport.YOffset() != 0 || m.rightViewport.YOffset() != 0 {
		t.Fatalf("expected viewport offsets reset, got %d/%d", m.centerViewport.YOffset(), m.rightViewport.YOffset())
	}
}

func TestToggleGroupedRawResetsHunkAndViewportOffsets(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.stage = stageReady
	m.selectedHunk = 3
	m.centerViewport.SetHeight(3)
	m.rightViewport.SetHeight(3)
	m.centerViewport.GotoBottom()
	m.rightViewport.GotoBottom()

	got, _ := m.handleReadyKey(keyPress("v"))

	if got.mode != modeRaw {
		t.Fatalf("expected raw mode, got %v", got.mode)
	}
	if got.selectedHunk != 0 || got.centerViewport.YOffset() != 0 || got.rightViewport.YOffset() != 0 {
		t.Fatalf("expected hunk and viewport offsets reset, got hunk=%d offsets=%d/%d", got.selectedHunk, got.centerViewport.YOffset(), got.rightViewport.YOffset())
	}
}

func TestUnassignedHunksRenderAsPseudoSlice(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	lines := strings.Join(m.leftLines(), "\n")
	if !strings.Contains(lines, "Unassigned or uncertain hunks") {
		t.Fatalf("expected unassigned pseudo-slice in left panel, got:\n%s", lines)
	}
	m.selectedSlice = 2
	center := strings.Join(m.centerLines(), "\n")
	if !strings.Contains(center, "SliceDiff keeps these hunks visible") {
		t.Fatalf("expected unassigned rationale in details, got:\n%s", center)
	}
}

func TestErrorStateRendersRecoveryInsteadOfOnlyCommandDump(t *testing.T) {
	m := testModel()
	m.setAppError(errorsForTest("codex CLI not found on PATH"))
	status := m.renderFooter()
	if !strings.Contains(status, "Codex CLI is not installed") {
		t.Fatalf("expected concise status summary, got %q", status)
	}
	center := strings.Join(m.centerLines(), "\n")
	if !strings.Contains(center, "Install Codex") {
		t.Fatalf("expected recovery text in center panel, got:\n%s", center)
	}
}

func TestStyledDiffLinesUseSemanticColors(t *testing.T) {
	m := testModel()
	m.mode = modeRaw
	m.selectedFile = 0
	m.selectedHunk = 0

	diffView := strings.Join(m.rightStyledLines(), "\n")
	if !strings.Contains(diffView, "\x1b[") {
		t.Fatalf("expected ANSI styling in diff view, got:\n%s", diffView)
	}
	if !strings.Contains(diffView, "+ a") {
		t.Fatalf("expected added line marker, got:\n%s", diffView)
	}
	if !strings.Contains(diffView, "- old") {
		t.Fatalf("expected deleted line marker, got:\n%s", diffView)
	}
}

func TestHelpExpandsInFooter(t *testing.T) {
	m := testModel()
	collapsed := m.renderFooter()
	m.showHelp = true
	expanded := m.renderFooter()

	if !strings.Contains(collapsed, "next panel") {
		t.Fatalf("expected short help in footer, got %q", collapsed)
	}
	if !strings.Contains(expanded, "focus left") {
		t.Fatalf("expected full help in expanded footer, got %q", expanded)
	}
}

func TestViewConfiguresAltScreenAndMouse(t *testing.T) {
	m := testModel()
	view := m.View()

	if !view.AltScreen {
		t.Fatal("expected SliceDiff view to request alt screen")
	}
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected cell-motion mouse mode, got %v", view.MouseMode)
	}
	if !strings.Contains(view.Content, "SliceDiff") {
		t.Fatalf("expected rendered view content, got %q", view.Content)
	}
}

func TestCenterPanelKeepsOverviewVisibleWithSelectedHunk(t *testing.T) {
	m := testModel()
	m.focus = panelCenter
	m.selectedHunk = 2
	m.syncComponents()

	panel := m.renderCenterPanel(m.centerTitle(), 80, 18, true)
	if !strings.Contains(panel, "Summary") {
		t.Fatalf("expected AI overview summary to stay visible, got:\n%s", panel)
	}
	if !strings.Contains(panel, "h3") {
		t.Fatalf("expected selected hunk list to remain visible, got:\n%s", panel)
	}
}

func testModel() Model {
	m := New(Options{Config: &config.Store{}})
	m.width = 120
	m.height = 40
	m.stage = stageReady
	m.mode = modeGrouped
	m.focus = panelLeft
	m.pr = &github.PullRequest{
		Owner:   "owner",
		Repo:    "repo",
		Number:  1,
		HeadSHA: "sha",
		Files: []diff.DiffFile{{
			Path:   "main.go",
			Status: "modified",
			Hunks: []diff.DiffHunk{
				{ID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@", Lines: []diff.DiffLine{{Type: diff.LineDeleted, OldNumber: 1, Content: "old"}, {Type: diff.LineAdded, NewNumber: 1, Content: "a"}}},
				{ID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@", Lines: []diff.DiffLine{{Type: diff.LineAdded, Content: "b"}}},
				{ID: "h3", FilePath: "main.go", Header: "@@ -3 +3 @@", Lines: []diff.DiffLine{{Type: diff.LineAdded, Content: "c"}}},
			},
		}},
	}
	m.slices = &agent.SliceSet{
		SchemaVersion: agent.SchemaVersion,
		Runner:        "codex",
		PromptVersion: agent.PromptVersion,
		PRHeadSHA:     "sha",
		Slices: []agent.Slice{
			{ID: "s1", Title: "First", Summary: "First summary.", Category: "feature", Confidence: "high", Rationale: "First rationale.", HunkRefs: []agent.HunkRef{{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}, {HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"}, {HunkID: "h3", FilePath: "main.go", Header: "@@ -3 +3 @@"}}},
			{ID: "s2", Title: "Second", Summary: "Second summary.", Category: "tests", Confidence: "medium", Rationale: "Second rationale.", HunkRefs: []agent.HunkRef{{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}}},
		},
		UnassignedHunks: []agent.HunkRef{{HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"}},
	}
	m.syncComponents()
	return m
}

func keyPress(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg(tea.Key{Text: s, Code: []rune(s)[0]})
	}
	return tea.KeyPressMsg(tea.Key{Code: keyCode(s)})
}

func keyCode(s string) rune {
	switch s {
	case "tab":
		return tea.KeyTab
	case "enter":
		return tea.KeyEnter
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	case "pgup":
		return tea.KeyPgUp
	case "pgdown":
		return tea.KeyPgDown
	case "home":
		return tea.KeyHome
	case "end":
		return tea.KeyEnd
	default:
		return 0
	}
}

func errorsForTest(message string) error {
	return testError(message)
}

type testError string

func (e testError) Error() string {
	return string(e)
}
