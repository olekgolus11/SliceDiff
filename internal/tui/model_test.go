package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestEnsureVisible(t *testing.T) {
	if got := ensureVisible(0, 10, 5); got != 6 {
		t.Fatalf("expected scroll 6, got %d", got)
	}
	if got := ensureVisible(6, 2, 5); got != 2 {
		t.Fatalf("expected scroll 2, got %d", got)
	}
	if got := ensureVisible(2, 4, 5); got != 2 {
		t.Fatalf("expected unchanged scroll 2, got %d", got)
	}
}

func TestGroupedSliceChangeResetsHunkAndScrolls(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.focus = panelLeft
	m.selectedSlice = 0
	m.selectedHunk = 2
	m.centerScroll = 3
	m.rightScroll = 4

	m.moveSelection(1)

	if m.selectedSlice != 1 {
		t.Fatalf("expected selected slice 1, got %d", m.selectedSlice)
	}
	if m.selectedHunk != 0 {
		t.Fatalf("expected hunk reset to 0, got %d", m.selectedHunk)
	}
	if m.centerScroll != 0 || m.rightScroll != 0 {
		t.Fatalf("expected detail/hunk scroll reset, got %d/%d", m.centerScroll, m.rightScroll)
	}
}

func TestToggleGroupedRawResetsHunkAndScrolls(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.stage = stageReady
	m.selectedHunk = 3
	m.leftScroll = 2
	m.centerScroll = 4
	m.rightScroll = 5

	next, _ := m.handleReadyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := next.(Model)

	if got.mode != modeRaw {
		t.Fatalf("expected raw mode, got %v", got.mode)
	}
	if got.selectedHunk != 0 || got.leftScroll != 0 || got.centerScroll != 0 || got.rightScroll != 0 {
		t.Fatalf("expected hunk and scroll reset, got hunk=%d scroll=%d/%d/%d", got.selectedHunk, got.leftScroll, got.centerScroll, got.rightScroll)
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
	status := m.renderStatus()
	if !strings.Contains(status, "Codex CLI is not installed") {
		t.Fatalf("expected concise status summary, got %q", status)
	}
	center := strings.Join(m.centerLines(), "\n")
	if !strings.Contains(center, "Install Codex") {
		t.Fatalf("expected recovery text in center panel, got:\n%s", center)
	}
}

func testModel() Model {
	return Model{
		opts:   Options{Config: &config.Store{}},
		width:  120,
		height: 40,
		stage:  stageReady,
		mode:   modeGrouped,
		focus:  panelLeft,
		pr: &github.PullRequest{
			Owner:   "owner",
			Repo:    "repo",
			Number:  1,
			HeadSHA: "sha",
			Files: []diff.DiffFile{{
				Path:   "main.go",
				Status: "modified",
				Hunks: []diff.DiffHunk{
					{ID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@", Lines: []diff.DiffLine{{Type: diff.LineAdded, Content: "a"}}},
					{ID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@", Lines: []diff.DiffLine{{Type: diff.LineAdded, Content: "b"}}},
					{ID: "h3", FilePath: "main.go", Header: "@@ -3 +3 @@", Lines: []diff.DiffLine{{Type: diff.LineAdded, Content: "c"}}},
				},
			}},
		},
		slices: &agent.SliceSet{
			SchemaVersion: agent.SchemaVersion,
			Runner:        "codex",
			PromptVersion: agent.PromptVersion,
			PRHeadSHA:     "sha",
			Slices: []agent.Slice{
				{ID: "s1", Title: "First", Summary: "First summary.", Category: "feature", Confidence: "high", Rationale: "First rationale.", HunkRefs: []agent.HunkRef{{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}, {HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"}, {HunkID: "h3", FilePath: "main.go", Header: "@@ -3 +3 @@"}}},
				{ID: "s2", Title: "Second", Summary: "Second summary.", Category: "tests", Confidence: "medium", Rationale: "Second rationale.", HunkRefs: []agent.HunkRef{{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}}},
			},
			UnassignedHunks: []agent.HunkRef{{HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"}},
		},
	}
}

func errorsForTest(message string) error {
	return testError(message)
}

type testError string

func (e testError) Error() string {
	return string(e)
}
