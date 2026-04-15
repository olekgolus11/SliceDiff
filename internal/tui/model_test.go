package tui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

func TestHorizontalPanelWidthsPrioritizeDiff(t *testing.T) {
	left, center, right := horizontalPanelWidths(100)
	if left+center+right != 100 {
		t.Fatalf("widths do not sum to total: %d %d %d", left, center, right)
	}
	if left != 20 || center != 30 || right != 50 {
		t.Fatalf("unexpected horizontal panel widths: %d %d %d", left, center, right)
	}
}

func TestNewWithTargetStartsLoading(t *testing.T) {
	m := New(Options{Config: &config.Store{}, HasTarget: true, Target: github.Target{Owner: "owner", Repo: "repo", Number: 1}})

	if m.stage != stageLoading {
		t.Fatalf("expected loading stage, got %v", m.stage)
	}
	if m.status != "Loading pull request..." {
		t.Fatalf("unexpected status %q", m.status)
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected init command for direct target")
	}
}

func TestNewWithoutTargetStartsWelcomePicker(t *testing.T) {
	m := New(Options{Config: &config.Store{}})

	if m.stage != stageWelcome {
		t.Fatalf("expected welcome stage, got %v", m.stage)
	}
	if m.welcomeSection != welcomeRequested {
		t.Fatalf("expected requested review section, got %v", m.welcomeSection)
	}
	if !m.pickerBusy {
		t.Fatal("expected requested reviews to load on init")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected init command for requested reviews")
	}
}

func TestRequestedReviewSelectionStartsPRLoading(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.stage = stageWelcome
	m.pickerBusy = false
	m.reviewPRs = []github.PRSearchResult{{Owner: "owner", Repo: "repo", Number: 9, Title: "Pick me"}}

	got, cmd := m.handleWelcomeKey(keyPress("enter"))

	if got.stage != stageLoading {
		t.Fatalf("expected loading stage, got %v", got.stage)
	}
	if !got.opts.HasTarget || got.opts.Target.Raw != "owner/repo#9" {
		t.Fatalf("unexpected selected target: %+v", got.opts.Target)
	}
	if cmd == nil {
		t.Fatal("expected load PR command")
	}
}

func TestManualFlowRepoThenPRSelectionStartsPRLoading(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.stage = stageWelcome
	m.pickerBusy = false
	m.welcomeSection = welcomeManual
	m.manualStep = manualRepos
	m.repoResults = []github.RepositorySearchResult{{FullName: "owner/repo", Description: "Terminal app"}}

	got, cmd := m.handleWelcomeKey(keyPress("enter"))
	if got.manualStep != manualPRs || got.selectedRepo != "owner/repo" {
		t.Fatalf("expected repo PR step, got step=%v repo=%q", got.manualStep, got.selectedRepo)
	}
	if !got.pickerBusy || cmd == nil {
		t.Fatal("expected repo PR loading command")
	}

	got.pickerBusy = false
	got.repoPRs = []github.PRSearchResult{{Number: 10, Title: "Open PR"}}
	got, cmd = got.handleWelcomeKey(keyPress("enter"))
	if got.stage != stageLoading {
		t.Fatalf("expected loading stage, got %v", got.stage)
	}
	if got.opts.Target.Raw != "owner/repo#10" {
		t.Fatalf("unexpected target: %+v", got.opts.Target)
	}
	if cmd == nil {
		t.Fatal("expected load PR command")
	}
}

func TestWelcomePickerEmptyAndErrorStatesRender(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.width = 80
	m.height = 20
	m.stage = stageWelcome
	m.pickerBusy = false
	m.reviewPRs = nil
	m.syncComponents()

	content := m.renderWelcome()
	lines := strings.Split(content, "\n")
	if len(lines) > 1 && strings.Contains(lines[0]+lines[1], "Choose a pull request") {
		t.Fatalf("expected choose copy in centered body, got top lines:\n%s\n%s", lines[0], lines[1])
	}
	if !strings.Contains(content, "Choose a pull request") {
		t.Fatalf("expected centered choose copy, got:\n%s", content)
	}
	plain := ansi.Strip(content)
	if !strings.Contains(plain, "Requested review 0/0") {
		t.Fatalf("expected inline count, got:\n%s", plain)
	}
	if first := strings.Split(plain, "\n")[0]; strings.Contains(first, "Requested review") {
		t.Fatalf("expected count out of panel title, got first line %q", first)
	}
	if !strings.Contains(content, "No requested reviews") {
		t.Fatalf("expected empty state, got:\n%s", content)
	}

	m.pickerErr = "gh search failed"
	content = m.renderWelcome()
	if !strings.Contains(content, "Could not load choices") || !strings.Contains(content, "gh search failed") {
		t.Fatalf("expected error state, got:\n%s", content)
	}
}

func TestWelcomePickerLargeTerminalRendersSliceCakeDiffArt(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.width = 148
	m.height = 52
	m.stage = stageWelcome
	m.pickerBusy = false
	m.reviewPRs = nil
	m.syncComponents()

	content := m.renderWelcome()
	if !strings.Contains(content, "░░███░░░░███") || !strings.Contains(content, "░░▒▒▓▓██████████") || !strings.Contains(content, "██████████") {
		t.Fatalf("expected Slice cake Diff art, got:\n%s", content)
	}
	line := firstLineContaining(content, "░░▒▒▓▓██████████")
	if line == "" {
		t.Fatalf("expected cake mark in wordmark, got:\n%s", content)
	}
	sliceIndex := strings.Index(line, "█████████")
	cakeIndex := strings.Index(line, "░░▒▒▓▓██████████")
	diffIndex := strings.LastIndex(line, "██████████")
	if sliceIndex < 0 || cakeIndex < 0 || diffIndex < 0 || !(sliceIndex < cakeIndex && cakeIndex < diffIndex) {
		t.Fatalf("expected Slice, then cake, then Diff on one row, got %q", line)
	}
	if !strings.Contains(content, "Choose a pull request") {
		t.Fatalf("expected centered choose copy, got:\n%s", content)
	}
}

func TestWelcomeArtFallsBackToLargeCakeSliceWhenRebusDoesNotFit(t *testing.T) {
	m := New(Options{Config: &config.Store{}})

	art := m.renderWelcomeArt(80, 21)

	if strings.Contains(art, "░░███░░░░███") {
		t.Fatalf("expected Slice wordmark to be omitted when the rebus does not fit, got:\n%s", art)
	}
	if !strings.Contains(art, "░▒▓▓█████▓▒░░") || !strings.Contains(art, "░▒▓███▓▓████░") {
		t.Fatalf("expected large cake-slice fallback art, got:\n%s", art)
	}
}

func TestWelcomePickerListStartsOnStableRowAcrossSections(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.width = 96
	m.height = 28
	m.stage = stageWelcome
	m.pickerBusy = false
	m.reviewPRs = nil
	m.syncComponents()

	requested := ansi.Strip(m.renderWelcome())
	requestedRow := lineIndexContaining(requested, "No requested reviews")
	if requestedRow < 0 {
		t.Fatalf("expected requested empty item, got:\n%s", requested)
	}

	m.welcomeSection = welcomeManual
	m.manualStep = manualRepos
	m.manualQuery = ""
	m.repoResults = nil
	m.syncComponents()

	manual := ansi.Strip(m.renderWelcome())
	manualRow := lineIndexContaining(manual, "Search repositories")
	if manualRow < 0 {
		t.Fatalf("expected manual search item, got:\n%s", manual)
	}
	if requestedRow != manualRow {
		t.Fatalf("expected first picker item on same row, requested=%d manual=%d\nrequested:\n%s\nmanual:\n%s", requestedRow, manualRow, requested, manual)
	}
	if !strings.Contains(manual, "Manual repositories 0/0") {
		t.Fatalf("expected inline manual count, got:\n%s", manual)
	}
	searchRow := lineIndexContaining(manual, "/ type a repository name")
	if searchRow < 0 {
		t.Fatalf("expected dedicated search row, got:\n%s", manual)
	}
	if searchRow >= manualRow {
		t.Fatalf("expected search row above picker list, search=%d picker=%d\n%s", searchRow, manualRow, manual)
	}
	lines := strings.Split(manual, "\n")
	if searchRow == 0 || searchRow+1 >= len(lines) || !isBlankPanelRow(lines[searchRow-1]) || !isBlankPanelRow(lines[searchRow+1]) {
		t.Fatalf("expected blank spacing around search row, got:\n%s", manual)
	}
}

func TestNavigationDelegatePadsSelectedRows(t *testing.T) {
	style := defaultStyles()
	delegate := navigationDelegate{style: style}
	model := newNavigationList(style)
	model.SetSize(32, 4)

	var buf bytes.Buffer
	delegate.Render(&buf, model, 0, pickerItem{title: "Short", description: "Tiny"})
	lines := strings.Split(ansi.Strip(buf.String()), "\n")
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != 32 {
			t.Fatalf("line %d expected width 32, got %d: %q", i, got, line)
		}
	}
}

func lineIndexContaining(content, needle string) int {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

func firstLineContaining(content, needle string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func isBlankPanelRow(line string) bool {
	return strings.Trim(line, " ┃") == ""
}

func TestManualTypingFiltersScopedRepositoryPool(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.stage = stageWelcome
	m.pickerBusy = false
	m.welcomeSection = welcomeManual
	m.manualStep = manualRepos
	m.repoPoolLoaded = true
	m.repoPool = []github.RepositorySearchResult{
		{FullName: "team/useful-api", Description: "Work repository"},
		{FullName: "other/noise", Description: "Static"},
	}

	got, cmd := m.handleWelcomeKey(keyPress("u"))
	if cmd != nil {
		t.Fatal("expected local filtering without gh command")
	}
	if got.manualQuery != "u" {
		t.Fatalf("expected query to update, got %q", got.manualQuery)
	}
	if len(got.repoResults) != 1 || got.repoResults[0].FullName != "team/useful-api" {
		t.Fatalf("unexpected filtered repos: %+v", got.repoResults)
	}
}

func TestCacheReadFallsBackToNormalizedRepoName(t *testing.T) {
	store := &config.Store{CacheDir: t.TempDir()}
	m := New(Options{Config: store, Version: "0.1.0"})
	m.pr = &github.PullRequest{
		Owner:   "Owner",
		Repo:    "Repo",
		Number:  12,
		HeadSHA: "sha",
	}
	normalizedKey := config.BuildSliceCacheKey("owner", "repo", 12, "sha", "codex", agent.PromptVersion, "0.1.0")
	want := &agent.SliceSet{Runner: "codex", PRHeadSHA: "sha"}
	if err := store.WriteJSON(normalizedKey, want); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	msg := readCacheCmd(store, m.cacheKeys(agent.RunnerCodex))()
	got, ok := msg.(cacheMsg)
	if !ok {
		t.Fatalf("expected cacheMsg, got %T", msg)
	}
	if !got.hit || got.slices == nil || got.slices.Runner != "codex" {
		t.Fatalf("expected normalized cache hit, got %+v", got)
	}
}

func TestWelcomePickerFitsTerminal(t *testing.T) {
	m := New(Options{Config: &config.Store{}})
	m.width = 72
	m.height = 18
	m.stage = stageWelcome
	m.pickerBusy = false
	m.reviewPRs = []github.PRSearchResult{
		{Owner: "owner", Repo: "repo", Number: 1, Title: "A very long pull request title that should be truncated before wrapping through the terminal edge"},
		{Owner: "owner", Repo: "repo", Number: 2, Title: "Second"},
	}
	m.syncComponents()

	content := m.renderWelcome()
	if got := lipgloss.Height(content); got != m.height {
		t.Fatalf("expected height %d, got %d", m.height, got)
	}
	for i, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got != m.width {
			t.Fatalf("line %d expected width %d, got %d: %q", i, m.width, got, line)
		}
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
	if !strings.Contains(center, "not confidently grouped") {
		t.Fatalf("expected fallback reading step in details, got:\n%s", center)
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
	plain := ansi.Strip(diffView)
	if !strings.Contains(plain, "+ a") {
		t.Fatalf("expected added line marker, got:\n%s", diffView)
	}
	if !strings.Contains(plain, "- old") {
		t.Fatalf("expected deleted line marker, got:\n%s", diffView)
	}
}

func TestStyledDiffLinesUseLanguageColor(t *testing.T) {
	m := testModel()
	m.mode = modeRaw
	m.pr.Files[0].Hunks[0].Lines = []diff.DiffLine{{
		Type:      diff.LineAdded,
		NewNumber: 1,
		Content:   "func main() { return }",
	}}

	rendered := strings.Join(m.rightStyledLines(), "\n")
	if !strings.Contains(rendered, "\x1b[38;2;") {
		t.Fatalf("expected truecolor syntax highlighting in diff view, got:\n%s", rendered)
	}
	if plain := ansi.Strip(rendered); !strings.Contains(plain, "+ func main() { return }") {
		t.Fatalf("expected highlighted diff to preserve plain text, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "48;2;12;42;27") {
		t.Fatalf("expected highlighted added line to keep green background, got:\n%s", rendered)
	}
}

func TestStyledDiffLinesKeepDeletedBackgroundWithLanguageColor(t *testing.T) {
	m := testModel()
	m.mode = modeRaw
	m.pr.Files[0].Hunks[0].Lines = []diff.DiffLine{{
		Type:      diff.LineDeleted,
		OldNumber: 1,
		Content:   "func main() { return }",
	}}

	rendered := strings.Join(m.rightStyledLines(), "\n")
	if !strings.Contains(rendered, "\x1b[38;2;") {
		t.Fatalf("expected truecolor syntax highlighting in diff view, got:\n%s", rendered)
	}
	if plain := ansi.Strip(rendered); !strings.Contains(plain, "- func main() { return }") {
		t.Fatalf("expected highlighted diff to preserve plain text, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "48;2;42;16;23") {
		t.Fatalf("expected highlighted deleted line to keep red background, got:\n%s", rendered)
	}
}

func TestStyledDiffLinesCacheRenderedDiffLines(t *testing.T) {
	m := testModel()
	m.mode = modeRaw
	m.diffLineCache = make(map[string]string)
	m.pr.Files[0].Hunks[0].Lines = []diff.DiffLine{{
		Type:      diff.LineAdded,
		NewNumber: 1,
		Content:   "func main() { return }",
	}}

	_ = m.rightStyledLines()
	if len(m.diffLineCache) != 1 {
		t.Fatalf("expected one cached diff line, got %d", len(m.diffLineCache))
	}
	for key := range m.diffLineCache {
		m.diffLineCache[key] = "cached line"
	}

	rendered := strings.Join(m.rightStyledLines(), "\n")
	if !strings.Contains(rendered, "cached line") {
		t.Fatalf("expected cached rendered diff line, got:\n%s", rendered)
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
	if strings.Contains(expanded, "focus/all") || strings.Contains(expanded, " f ") {
		t.Fatalf("expected focus/all toggle to be absent from help, got %q", expanded)
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

func TestMainViewUsesFullTerminalAndKeepsFooterVisible(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 24
	m.syncComponents()

	content := m.renderMain()
	if got := lipgloss.Height(content); got != m.height {
		t.Fatalf("expected rendered height %d, got %d", m.height, got)
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || !strings.Contains(lines[len(lines)-1], "next panel") {
		t.Fatalf("expected footer on final line, got last line %q", lines[len(lines)-1])
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != m.width {
			t.Fatalf("line %d expected width %d, got %d: %q", i, m.width, got, line)
		}
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

func TestCenterPanelShowsFullLongSummary(t *testing.T) {
	m := testModel()
	m.slices.Slices[0].Summary = "The authentication flow now shares the callback validator with the session refresh path so expired tokens fail consistently. It also keeps the retry copy close to the underlying GitHub CLI error for clearer recovery. The final bit updates the cache guard so stale grouped hunks do not hide new diff content."
	m.syncComponents()

	panel := m.renderCenterPanel(m.centerTitle(), 80, 22, true)
	if !strings.Contains(panel, "The final bit") || !strings.Contains(panel, "updates the cache guard") {
		t.Fatalf("expected full summary to remain visible, got:\n%s", panel)
	}
	if strings.Contains(panel, "recovery...") {
		t.Fatalf("expected summary not to be capped with ellipsis, got:\n%s", panel)
	}
}

func TestShortenPathAfterFirstSlash(t *testing.T) {
	fits := "composeApp/src/file.kt"
	if got := shortenPathAfterFirstSlash(fits, 80); got != fits {
		t.Fatalf("expected fitting path unchanged, got %q", got)
	}

	longPath := "composeApp/src/commonMain/kotlin/com/farmermarket/kmp/products/presentation/seller_details.kt"
	got := shortenPathAfterFirstSlash(longPath, 52)
	if !strings.HasPrefix(got, "composeApp/...") {
		t.Fatalf("expected first segment and middle dots, got %q", got)
	}
	tail := strings.TrimPrefix(got, "composeApp/...")
	if !strings.HasSuffix(longPath, tail) {
		t.Fatalf("expected longest suffix preserved, got %q", got)
	}
	if width := lipgloss.Width(got); width > 52 {
		t.Fatalf("expected shortened path width <= 52, got %d: %q", width, got)
	}

	noSlash := shortenPathAfterFirstSlash("abcdefghijk", 8)
	if noSlash != "abcde..." {
		t.Fatalf("expected no-slash path to use end truncation, got %q", noSlash)
	}
}

func TestGroupedDetailsReadingStepsRenderNarrativeAndElideLongFilePaths(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.slices.Slices[0].HunkRefs[0].FilePath = "composeApp/src/commonMain/kotlin/com/farmermarket/kmp/products/presentation/seller_details.kt"
	m.slices.Slices[0].ReadingSteps[0].HunkRef.FilePath = m.slices.Slices[0].HunkRefs[0].FilePath

	lines, _ := m.centerScrollStyledLines(58)
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "Reading order") || !strings.Contains(rendered, "First hunk explains the state change.") {
		t.Fatalf("expected guided reading prose in details, got:\n%s", rendered)
	}
	if !strings.HasPrefix(rendered, "\n") {
		t.Fatalf("expected one blank line before reading order, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "composeApp/...") {
		t.Fatalf("expected elided path in reading step source, got:\n%s", rendered)
	}
	for _, line := range lines {
		if width := lipgloss.Width(line); width > 58 {
			t.Fatalf("expected line width <= 58, got %d: %q", width, line)
		}
	}
}

func TestGroupedDetailsSelectedLineAnchorsProse(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.selectedHunk = 0
	m.centerViewport.SetHeight(1)

	lines, selectedLine := m.centerScrollStyledLines(58)
	if selectedLine < 0 || selectedLine >= len(lines) {
		t.Fatalf("expected selected line inside details, got %d for %d lines", selectedLine, len(lines))
	}
	if !strings.Contains(lines[selectedLine], "First hunk explains the state change.") {
		t.Fatalf("expected selected line to anchor the hunk prose, got %q", lines[selectedLine])
	}
	if strings.Contains(lines[selectedLine], "main.go") {
		t.Fatalf("expected selected line to avoid anchoring the hunk reference, got %q", lines[selectedLine])
	}
}

func TestGroupedDetailsSelectionHighlightsHunkReferenceOnly(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.selectedHunk = 0

	lines, _ := m.centerScrollStyledLines(58)
	rendered := strings.Join(lines, "\n")
	selectedStyle := m.style.diffSelected.Render("First hunk explains the state change.")
	if strings.Contains(rendered, selectedStyle) {
		t.Fatalf("expected selected reading step prose to avoid full selection background, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, m.style.detailHunk.Render("h1")) {
		t.Fatalf("expected selected hunk badge in details, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, m.style.detailRail.Render(" ")) {
		t.Fatalf("expected selected rail marker in details, got:\n%s", rendered)
	}
	if lipgloss.Width(m.readingStepRefPrefix(true)) != lipgloss.Width(m.readingStepRefPrefix(false)) {
		t.Fatal("expected selected and unselected hunk references to share the same column")
	}
	if lipgloss.Width(m.style.detailHunk.Render("h1")) != lipgloss.Width("h1") {
		t.Fatal("expected selected hunk badge to keep the hunk id width stable")
	}
}

func TestGroupedReadingStepSelectionDrivesRightDiff(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.focus = panelCenter
	m.selectedHunk = 0

	first := strings.Join(m.rightLines(), "\n")
	if !strings.Contains(first, "+a") && !strings.Contains(first, "+ a") {
		t.Fatalf("expected first selected step to show first hunk diff, got:\n%s", first)
	}

	m.moveSelection(1)
	second := strings.Join(m.rightLines(), "\n")
	if !strings.Contains(second, "+b") && !strings.Contains(second, "+ b") {
		t.Fatalf("expected second selected step to show second hunk diff, got:\n%s", second)
	}
}

func TestGroupedModeAddsQuietChangesAndFiltersSlices(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.markHunkSignal("h2", diff.HunkSignalQuiet, "imports")

	items := m.reviewItems()
	if len(items) != 3 {
		t.Fatalf("expected two focus slices plus quiet changes, got %+v", items)
	}
	if !items[2].IsQuiet || items[2].Title != "Quiet changes" {
		t.Fatalf("expected quiet changes item at the end, got %+v", items[2])
	}
	if len(items[0].HunkRefs) != 2 {
		t.Fatalf("expected quiet hunk filtered from first slice, got %+v", items[0].HunkRefs)
	}
	for _, ref := range items[0].HunkRefs {
		if ref.HunkID == "h2" {
			t.Fatalf("expected h2 to be filtered from focus slice, got %+v", items[0].HunkRefs)
		}
	}
	left := strings.Join(m.leftLines(), "\n")
	if !strings.Contains(left, "Quiet changes") {
		t.Fatalf("expected quiet changes in left nav, got:\n%s", left)
	}
	if strings.Contains(strings.ToLower(left), "confidence") {
		t.Fatalf("expected left nav to omit confidence, got:\n%s", left)
	}
}

func TestGroupedModeAddsAuditChanges(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped
	m.markHunkSignal("h3", diff.HunkSignalAudit, "generated")

	items := m.reviewItems()
	if len(items) != 4 {
		t.Fatalf("expected focus slices, unassigned, and audit changes, got %+v", items)
	}
	if !items[3].IsAudit || items[3].Title != "Audit changes" {
		t.Fatalf("expected audit changes item at the end, got %+v", items[3])
	}
	if len(items[0].HunkRefs) != 2 {
		t.Fatalf("expected audit hunk filtered from first slice, got %+v", items[0].HunkRefs)
	}
	if got := m.headerCounts(); !strings.Contains(got, "1 audit") {
		t.Fatalf("expected audit count in header, got %q", got)
	}
}

func TestFocusKeyDoesNotRestoreQuietHunks(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.mode = modeGrouped
	m.markHunkSignal("h2", diff.HunkSignalQuiet, "imports")

	got, _ := m.handleReadyKey(keyPress("f"))
	items := got.reviewItems()
	if len(items) != 3 {
		t.Fatalf("expected quiet changes to stay separated after f, got %+v", items)
	}
	if len(items[0].HunkRefs) != 2 {
		t.Fatalf("expected quiet hunk to stay filtered from semantic slice, got %+v", items[0].HunkRefs)
	}
	for _, ref := range items[0].HunkRefs {
		if ref.HunkID == "h2" {
			t.Fatalf("expected h2 to remain separated from semantic slice, got %+v", items[0].HunkRefs)
		}
	}
	if !items[2].IsQuiet {
		t.Fatalf("expected quiet item to remain present after f, got %+v", items)
	}
}

func TestGroupedDetailsOmitConfidence(t *testing.T) {
	m := testModel()
	m.mode = modeGrouped

	plain, _ := m.centerPlainLines()
	styled := strings.Join(m.centerOverviewStyledLines(80), "\n")
	rendered := strings.Join(plain, "\n") + "\n" + styled
	if strings.Contains(strings.ToLower(rendered), "confidence") {
		t.Fatalf("expected grouped details to omit confidence, got:\n%s", rendered)
	}
}

func TestRawModeStillShowsQuietHunks(t *testing.T) {
	m := testModel()
	m.mode = modeRaw
	m.selectedFile = 0
	m.markHunkSignal("h2", diff.HunkSignalQuiet, "imports")

	lines, _ := m.centerScrollStyledLines(80)
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "h2") || !strings.Contains(rendered, "quiet:imports") {
		t.Fatalf("expected raw mode to expose quiet hunk with reason, got:\n%s", rendered)
	}
}

func TestRightPanelLineKeysScrollDiffWithoutChangingSelectedHunk(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.focus = panelRight
	m.selectedHunk = 1
	m.rightViewport.SetHeight(2)
	m.rightViewport.SetContentLines([]string{"one", "two", "three", "four", "five"})

	got, _ := m.handleReadyKey(keyPress("j"))
	if got.selectedHunk != 1 {
		t.Fatalf("expected selected hunk unchanged, got %d", got.selectedHunk)
	}
	if got.rightViewport.YOffset() != 1 {
		t.Fatalf("expected j to scroll diff viewport down by 1 line, got %d", got.rightViewport.YOffset())
	}

	got, _ = got.handleReadyKey(keyPress("k"))
	if got.rightViewport.YOffset() != 0 {
		t.Fatalf("expected k to scroll diff viewport up, got offset %d", got.rightViewport.YOffset())
	}

	got, _ = got.handleReadyKey(keyPress("down"))
	if got.rightViewport.YOffset() != 1 {
		t.Fatalf("expected down arrow to scroll diff viewport down by 1 line, got %d", got.rightViewport.YOffset())
	}
}

func TestRightPanelPageAndBoundaryKeysControlDiffViewport(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.focus = panelRight
	m.selectedHunk = 1
	m.rightViewport.SetHeight(2)
	m.rightViewport.SetContentLines([]string{"one", "two", "three", "four", "five", "six"})

	got, _ := m.handleReadyKey(keyPress("pgdown"))
	if got.selectedHunk != 1 {
		t.Fatalf("expected selected hunk unchanged after pgdown, got %d", got.selectedHunk)
	}
	if got.rightViewport.YOffset() == 0 {
		t.Fatal("expected pgdown to scroll diff viewport")
	}

	got, _ = got.handleReadyKey(keyPress("pgup"))
	if got.rightViewport.YOffset() != 0 {
		t.Fatalf("expected pgup to return viewport to top, got offset %d", got.rightViewport.YOffset())
	}

	got, _ = got.handleReadyKey(keyPress("end"))
	if got.rightViewport.YOffset() == 0 {
		t.Fatal("expected end to move diff viewport to bottom")
	}

	got, _ = got.handleReadyKey(keyPress("home"))
	if got.rightViewport.YOffset() != 0 {
		t.Fatalf("expected home to move diff viewport to top, got offset %d", got.rightViewport.YOffset())
	}
}

func TestCenterPanelLineKeysStillMoveSelectedHunk(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.mode = modeGrouped
	m.focus = panelCenter
	m.selectedHunk = 0

	got, _ := m.handleReadyKey(keyPress("j"))
	if got.selectedHunk != 1 {
		t.Fatalf("expected center j to move selected hunk, got %d", got.selectedHunk)
	}
}

func TestMouseWheelScrollsDiffByTwoLinesPerDampenedStep(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.focus = panelLeft
	m.selectedHunk = 1
	m.rightViewport.SetHeight(2)
	m.rightViewport.SetContentLines([]string{"one", "two", "three", "four", "five", "six"})

	for i := 0; i < 4; i++ {
		needsSync := m.handleMouseWheel(tea.Mouse{X: 90, Y: 3, Button: tea.MouseWheelDown})
		if needsSync {
			t.Fatal("expected diff wheel scroll to preserve viewport state without full sync")
		}
	}
	if m.focus != panelRight {
		t.Fatalf("expected wheel over diff to focus right panel, got %v", m.focus)
	}
	if got := m.rightViewport.YOffset(); got != 4 {
		t.Fatalf("expected wheel down to scroll diff by 2 lines per dampened step, got %d", got)
	}
	if m.selectedHunk != 1 {
		t.Fatalf("expected selected hunk unchanged, got %d", m.selectedHunk)
	}

	for i := 0; i < 4; i++ {
		m.handleMouseWheel(tea.Mouse{X: 90, Y: 3, Button: tea.MouseWheelUp})
	}
	if got := m.rightViewport.YOffset(); got != 0 {
		t.Fatalf("expected wheel up to scroll diff back by 2 lines per terminal notch, got %d", got)
	}
}

func TestMouseWheelScrollsSlicesPanelUnderCursor(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.focus = panelRight
	m.selectedSlice = 0
	m.selectedHunk = 1

	needsSync := false
	for i := 0; i < 4; i++ {
		needsSync = m.handleMouseWheel(tea.Mouse{X: 2, Y: 3, Button: tea.MouseWheelDown}) || needsSync
	}
	if !needsSync {
		t.Fatal("expected slices wheel movement to request full sync")
	}
	if m.focus != panelLeft {
		t.Fatalf("expected wheel over slices to focus left panel, got %v", m.focus)
	}
	if m.selectedSlice != 1 {
		t.Fatalf("expected wheel down to move slices by 1 row, got %d", m.selectedSlice)
	}
	if m.selectedHunk != 0 {
		t.Fatalf("expected slice wheel movement to reset selected hunk, got %d", m.selectedHunk)
	}
}

func TestMouseWheelMovesHunkSelectionUnderCursor(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.focus = panelRight
	m.selectedHunk = 0
	m.centerViewport.SetHeight(2)
	m.centerViewport.SetContentLines([]string{"Hunks", "h1", "h2", "h3", "h4", "h5"})

	needsSync := false
	for i := 0; i < 4; i++ {
		needsSync = m.handleMouseWheel(tea.Mouse{X: 30, Y: 3, Button: tea.MouseWheelDown}) || needsSync
	}
	if !needsSync {
		t.Fatal("expected hunks wheel movement to request full sync")
	}
	if m.focus != panelCenter {
		t.Fatalf("expected wheel over hunks to focus center panel, got %v", m.focus)
	}
	if m.selectedHunk != 1 {
		t.Fatalf("expected wheel down to move selected hunk by 1 row, got %d", m.selectedHunk)
	}
}

func TestMouseWheelUpdateMovesHunkSelectionAndSyncsDetails(t *testing.T) {
	m := testModel()
	m.stage = stageReady
	m.focus = panelRight
	m.selectedHunk = 0
	m.centerViewport.SetHeight(2)
	m.centerViewport.SetContentLines([]string{"Hunks", "h1", "h2", "h3", "h4", "h5"})

	got := m
	for i := 0; i < 4; i++ {
		got = updateModel(got, tea.MouseWheelMsg(tea.Mouse{X: 30, Y: 3, Button: tea.MouseWheelDown}))
	}
	got = updateModel(got, spinner.TickMsg{})
	if got.focus != panelCenter {
		t.Fatalf("expected wheel over hunks to focus center panel, got %v", got.focus)
	}
	if got.selectedHunk != 1 {
		t.Fatalf("expected wheel over hunks to move selected hunk by 1 row, got %d", got.selectedHunk)
	}
}

func updateModel(m Model, msg tea.Msg) Model {
	model, _ := m.Update(msg)
	return model.(Model)
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
			{
				ID:        "s1",
				Title:     "First",
				Summary:   "First summary.",
				Category:  "feature",
				Rationale: "First rationale.",
				HunkRefs: []agent.HunkRef{
					{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"},
					{HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"},
					{HunkID: "h3", FilePath: "main.go", Header: "@@ -3 +3 @@"},
				},
				ReadingSteps: []agent.ReadingStep{
					{HunkRef: agent.HunkRef{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}, Body: "First hunk explains the state change."},
					{HunkRef: agent.HunkRef{HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"}, Body: "Second hunk propagates that change to the next layer."},
					{HunkRef: agent.HunkRef{HunkID: "h3", FilePath: "main.go", Header: "@@ -3 +3 @@"}, Body: "Third hunk completes the flow for callers."},
				},
			},
			{
				ID:        "s2",
				Title:     "Second",
				Summary:   "Second summary.",
				Category:  "tests",
				Rationale: "Second rationale.",
				HunkRefs:  []agent.HunkRef{{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}},
				ReadingSteps: []agent.ReadingStep{
					{HunkRef: agent.HunkRef{HunkID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"}, Body: "The test hunk covers the updated behavior."},
				},
			},
		},
		UnassignedHunks: []agent.HunkRef{{HunkID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"}},
	}
	m.syncComponents()
	return m
}

func (m *Model) markHunkSignal(id string, signal diff.HunkSignal, reason string) {
	for i := range m.pr.Files {
		for j := range m.pr.Files[i].Hunks {
			if m.pr.Files[i].Hunks[j].ID == id {
				m.pr.Files[i].Hunks[j].Signal = signal
				m.pr.Files[i].Hunks[j].Reason = reason
				return
			}
		}
	}
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
	case "esc":
		return tea.KeyEsc
	case "backspace":
		return tea.KeyBackspace
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
