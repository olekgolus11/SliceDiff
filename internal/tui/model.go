package tui

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/config"
	"github.com/olekgolus11/SliceDiff/internal/diff"
	"github.com/olekgolus11/SliceDiff/internal/github"
)

type Model struct {
	opts Options

	width  int
	height int

	stage stage
	mode  viewMode
	focus panel

	pr     *github.PullRequest
	slices *agent.SliceSet

	selectedSlice  int
	selectedFile   int
	selectedHunk   int
	selectedSetup  int
	selectedPicker int
	showHelp       bool
	focusOnly      bool
	wheelTarget    panel
	wheelDirection int
	wheelRemainder int

	leftList       list.Model
	pickerList     list.Model
	centerViewport viewport.Model
	rightViewport  viewport.Model
	help           help.Model
	spinner        spinner.Model
	keys           keyMap

	status string
	errMsg string
	appErr *AppError
	aiBusy bool

	welcomeSection welcomeSection
	manualStep     manualStep
	manualQuery    string
	selectedRepo   string
	reviewPRs      []github.PRSearchResult
	repoPool       []github.RepositorySearchResult
	repoResults    []github.RepositorySearchResult
	repoPRs        []github.PRSearchResult
	repoPoolLoaded bool
	pickerBusy     bool
	pickerErr      string

	style styles
}

func New(opts Options) Model {
	style := defaultStyles()
	return Model{
		opts:           opts,
		stage:          initialStage(opts),
		mode:           modeRaw,
		focus:          panelLeft,
		status:         initialStatus(opts),
		leftList:       newNavigationList(style),
		pickerList:     newNavigationList(style),
		centerViewport: newViewport(style),
		rightViewport:  newViewport(style),
		help:           newHelp(style),
		spinner:        newSpinner(style),
		keys:           defaultKeyMap(),
		focusOnly:      true,
		pickerBusy:     !opts.HasTarget,
		style:          style,
	}
}

func (m Model) Init() tea.Cmd {
	if m.opts.HasTarget {
		return tea.Batch(loadPRCmd(m.opts.Target), m.spinner.Tick)
	}
	return tea.Batch(loadReviewRequestsCmd(), m.spinner.Tick)
}

func initialStage(opts Options) stage {
	if opts.HasTarget {
		return stageLoading
	}
	return stageWelcome
}

func initialStatus(opts Options) string {
	if opts.HasTarget {
		return "Loading pull request..."
	}
	return "Loading requested reviews..."
}

func loadPRCmd(target github.Target) tea.Cmd {
	return func() tea.Msg {
		pr, err := github.NewClient().Fetch(context.Background(), target)
		return loadPRMsg{pr: pr, err: err}
	}
}

func loadReviewRequestsCmd() tea.Cmd {
	return func() tea.Msg {
		prs, err := github.NewClient().SearchReviewRequests(context.Background())
		return reviewRequestsMsg{prs: prs, err: err}
	}
}

func loadRepoPoolCmd() tea.Cmd {
	return func() tea.Msg {
		repos, err := github.NewClient().LoadRepositoryPool(context.Background())
		return repoPoolMsg{repos: repos, err: err}
	}
}

func listRepoPRsCmd(repo string) tea.Cmd {
	return func() tea.Msg {
		prs, err := github.NewClient().ListOpenPRs(context.Background(), repo)
		return repoPRsMsg{repo: repo, prs: prs, err: err}
	}
}

func runAgentCmd(opts Options, pr github.PullRequest, runner agent.RunnerName) tea.Cmd {
	return func() tea.Msg {
		slices, err := agent.Run(context.Background(), agent.Options{
			Runner:  runner,
			Timeout: 3 * time.Minute,
			WorkDir: mustGetwd(),
		}, pr)
		return agentMsg{slices: slices, err: err}
	}
}

func readCacheCmd(store *config.Store, keys []string) tea.Cmd {
	return func() tea.Msg {
		if len(keys) == 0 {
			return cacheMsg{hit: false}
		}
		var slices agent.SliceSet
		for _, key := range keys {
			err := store.ReadJSON(key, &slices)
			if err == nil {
				return cacheMsg{slices: &slices, hit: true}
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return cacheMsg{err: err}
		}
		return cacheMsg{hit: false}
	}
}

func writeCacheCmd(store *config.Store, keys []string, slices *agent.SliceSet) tea.Cmd {
	return func() tea.Msg {
		if slices == nil {
			return nil
		}
		for _, key := range keys {
			_ = store.WriteJSON(key, slices)
		}
		return nil
	}
}

func (m Model) maybeStartAI() (Model, tea.Cmd) {
	if m.opts.NoAI {
		m.mode = modeRaw
		m.stage = stageReady
		m.status = "AI disabled. Showing raw diff."
		m.clearError()
		return m, nil
	}

	if m.opts.Config.Config.AIConsent == nil {
		m.stage = stageConsent
		m.status = "AI consent required before sending PR diffs to a runner."
		return m, nil
	}
	if !*m.opts.Config.Config.AIConsent {
		m.mode = modeRaw
		m.stage = stageReady
		m.status = "AI consent declined. Showing raw diff."
		m.clearError()
		return m, nil
	}

	runner := m.selectedRunner()
	if runner == "" {
		m.stage = stageRunner
		m.status = "Choose an AI runner."
		return m, nil
	}

	m.aiBusy = true
	m.status = "Checking cache..."
	m.clearError()
	keys := m.cacheKeys(runner)
	if !m.opts.RegenSlices {
		return m, readCacheCmd(m.opts.Config, keys)
	}
	return m.startAgent(runner)
}

func (m Model) startAgent(runner agent.RunnerName) (Model, tea.Cmd) {
	if m.pr == nil {
		m.stage = stageFatal
		m.errMsg = "Cannot run AI before the PR is loaded."
		return m, nil
	}
	m.aiBusy = true
	m.status = "Running " + string(runner) + " semantic grouping..."
	m.clearError()
	pr := *m.pr
	return m, runAgentCmd(m.opts, pr, runner)
}

func (m Model) selectedRunner() agent.RunnerName {
	if m.opts.RunnerOverride != "" {
		if agent.IsSupportedRunner(m.opts.RunnerOverride) {
			return agent.RunnerName(m.opts.RunnerOverride)
		}
		return ""
	}
	if agent.IsSupportedRunner(m.opts.Config.Config.AIRunner) {
		return agent.RunnerName(m.opts.Config.Config.AIRunner)
	}
	return ""
}

func (m Model) cacheKeys(runner agent.RunnerName) []string {
	if m.pr == nil {
		return nil
	}
	legacy := config.BuildSliceCacheKey(m.pr.Owner, m.pr.Repo, m.pr.Number, m.pr.HeadSHA, string(runner), agent.PromptVersion, m.opts.Version)
	normalized := config.BuildSliceCacheKey(strings.ToLower(m.pr.Owner), strings.ToLower(m.pr.Repo), m.pr.Number, m.pr.HeadSHA, string(runner), agent.PromptVersion, m.opts.Version)
	if legacy == normalized {
		return []string{legacy}
	}
	return []string{legacy, normalized}
}

func (m Model) currentFile() *diff.DiffFile {
	if m.pr == nil || len(m.pr.Files) == 0 {
		return nil
	}
	if m.selectedFile < 0 {
		m.selectedFile = 0
	}
	if m.selectedFile >= len(m.pr.Files) {
		m.selectedFile = len(m.pr.Files) - 1
	}
	return &m.pr.Files[m.selectedFile]
}

func (m Model) currentRawHunk() *diff.DiffHunk {
	file := m.currentFile()
	if file == nil || len(file.Hunks) == 0 {
		return nil
	}
	if m.selectedHunk < 0 {
		m.selectedHunk = 0
	}
	if m.selectedHunk >= len(file.Hunks) {
		m.selectedHunk = len(file.Hunks) - 1
	}
	return &file.Hunks[m.selectedHunk]
}

func (m Model) currentSlice() *agent.Slice {
	if m.slices == nil || len(m.slices.Slices) == 0 {
		return nil
	}
	if m.selectedSlice < 0 {
		m.selectedSlice = 0
	}
	if m.selectedSlice >= len(m.slices.Slices) {
		m.selectedSlice = len(m.slices.Slices) - 1
	}
	return &m.slices.Slices[m.selectedSlice]
}

func (m Model) reviewItems() []reviewItem {
	if m.slices == nil {
		return nil
	}
	items := make([]reviewItem, 0, len(m.slices.Slices)+1)
	for _, slice := range m.slices.Slices {
		item := reviewItem{
			ID:           slice.ID,
			Title:        slice.Title,
			Category:     slice.Category,
			Confidence:   slice.Confidence,
			Summary:      slice.Summary,
			Rationale:    slice.Rationale,
			HunkRefs:     slice.HunkRefs,
			ReadingSteps: slice.ReadingSteps,
		}
		if m.focusOnly {
			item = m.focusReviewItem(item)
			if len(item.HunkRefs) == 0 {
				continue
			}
		}
		items = append(items, item)
	}
	if len(m.slices.UnassignedHunks) > 0 {
		refs := m.slices.UnassignedHunks
		if m.focusOnly {
			refs = m.focusRefs(refs)
		}
		if len(refs) > 0 {
			items = append(items, reviewItem{
				ID:           "unassigned",
				Title:        "Unassigned or uncertain hunks",
				Category:     "uncertain",
				Confidence:   "low",
				Summary:      "The AI runner did not confidently assign these hunks to a semantic slice.",
				Rationale:    "SliceDiff keeps these hunks visible so no part of the PR is hidden.",
				HunkRefs:     refs,
				ReadingSteps: fallbackReadingSteps(refs),
				IsUnassigned: true,
			})
		}
	}
	if m.focusOnly {
		if quiet := m.signalReviewItem(diff.HunkSignalQuiet); quiet != nil {
			items = append(items, *quiet)
		}
		if audit := m.signalReviewItem(diff.HunkSignalAudit); audit != nil {
			items = append(items, *audit)
		}
	}
	return items
}

func (m Model) focusReviewItem(item reviewItem) reviewItem {
	stepsByHunk := map[string]agent.ReadingStep{}
	for _, step := range item.ReadingSteps {
		stepsByHunk[hunkKey(step.HunkRef)] = step
	}
	var refs []agent.HunkRef
	var steps []agent.ReadingStep
	for _, ref := range item.HunkRefs {
		if !m.isFocusRef(ref) {
			continue
		}
		refs = append(refs, ref)
		if step, ok := stepsByHunk[hunkKey(ref)]; ok {
			steps = append(steps, step)
		} else {
			steps = append(steps, agent.ReadingStep{
				HunkRef: ref,
				Body:    "Read this hunk directly against the diff.",
			})
		}
	}
	item.HunkRefs = refs
	item.ReadingSteps = steps
	return item
}

func (m Model) focusRefs(refs []agent.HunkRef) []agent.HunkRef {
	out := make([]agent.HunkRef, 0, len(refs))
	for _, ref := range refs {
		if m.isFocusRef(ref) {
			out = append(out, ref)
		}
	}
	return out
}

func (m Model) signalReviewItem(signal diff.HunkSignal) *reviewItem {
	refs := m.signalHunkRefs(signal)
	if len(refs) == 0 {
		return nil
	}
	item := reviewItem{
		HunkRefs:     refs,
		ReadingSteps: m.signalReadingSteps(refs),
	}
	switch signal {
	case diff.HunkSignalAudit:
		item.ID = "audit"
		item.Title = "Audit changes"
		item.Category = "audit"
		item.Confidence = "verify"
		item.Summary = "Generated, vendor, and lockfile changes are collapsed here for a quick verification pass."
		item.Rationale = "These changes may be mechanical, but they deserve audit instead of narrative review."
		item.IsAudit = true
	default:
		item.ID = "quiet"
		item.Title = "Quiet changes"
		item.Category = "quiet"
		item.Confidence = "skim"
		item.Summary = "Formatting, imports, and whitespace churn is collapsed here so behavior changes stay prominent."
		item.Rationale = "Nothing is hidden. These hunks are kept recoverable for quick skim."
		item.IsQuiet = true
	}
	return &item
}

func (m Model) signalHunkRefs(signal diff.HunkSignal) []agent.HunkRef {
	if m.pr == nil {
		return nil
	}
	var refs []agent.HunkRef
	for _, file := range m.pr.Files {
		for _, hunk := range file.Hunks {
			if hunkSignal(hunk) == signal {
				refs = append(refs, agent.RefForHunk(hunk))
			}
		}
	}
	return refs
}

func (m Model) signalReadingSteps(refs []agent.HunkRef) []agent.ReadingStep {
	steps := make([]agent.ReadingStep, 0, len(refs))
	for _, ref := range refs {
		steps = append(steps, agent.ReadingStep{
			HunkRef: ref,
			Body:    m.signalHunkSummary(ref),
		})
	}
	return steps
}

func (m Model) signalHunkSummary(ref agent.HunkRef) string {
	hunk := m.findHunk(ref)
	reason := "quiet"
	if hunk != nil && hunk.Reason != "" {
		reason = hunk.Reason
	}
	switch reason {
	case "imports":
		return "Imports: grouped or reordered import declarations."
	case "whitespace":
		return "Whitespace: content matches after whitespace is ignored."
	case "format":
		return "Format: token content matches after layout changes are collapsed."
	case "lockfile":
		return "Lockfile: dependency resolution changed; audit package identity or version."
	case "vendor":
		return "Vendor: third-party or vendored code changed; verify source and intent."
	case "generated":
		return "Generated: machine-produced output changed; verify it follows the source change."
	default:
		return "Quiet: low-signal diff kept here for review."
	}
}

func (m Model) isFocusRef(ref agent.HunkRef) bool {
	hunk := m.findHunk(ref)
	return hunk == nil || hunkSignal(*hunk) == diff.HunkSignalFocus
}

func hunkSignal(hunk diff.DiffHunk) diff.HunkSignal {
	switch hunk.Signal {
	case diff.HunkSignalQuiet, diff.HunkSignalAudit:
		return hunk.Signal
	default:
		return diff.HunkSignalFocus
	}
}

func hunkKey(ref agent.HunkRef) string {
	return ref.FilePath + "\x00" + ref.HunkID
}

func fallbackReadingSteps(refs []agent.HunkRef) []agent.ReadingStep {
	steps := make([]agent.ReadingStep, 0, len(refs))
	for _, ref := range refs {
		steps = append(steps, agent.ReadingStep{
			HunkRef: ref,
			Body:    "This hunk was not confidently grouped by the AI runner, so read it directly against the diff.",
		})
	}
	return steps
}

func (m Model) currentReviewItem() *reviewItem {
	items := m.reviewItems()
	if len(items) == 0 {
		return nil
	}
	idx := clamp(m.selectedSlice, 0, len(items)-1)
	return &items[idx]
}

func (m *Model) setAppError(err error) {
	m.appErr = classifyError(err)
	if m.appErr == nil {
		m.errMsg = ""
		return
	}
	m.errMsg = m.appErr.Detail
}

func (m *Model) clearError() {
	m.appErr = nil
	m.errMsg = ""
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
