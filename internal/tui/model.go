package tui

import (
	"context"
	"errors"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	selectedSlice int
	selectedFile  int
	selectedHunk  int
	selectedSetup int
	showHelp      bool

	status string
	errMsg string
	aiBusy bool

	style styles
}

func New(opts Options) Model {
	return Model{
		opts:   opts,
		stage:  stageLoading,
		mode:   modeRaw,
		focus:  panelLeft,
		status: "Loading pull request...",
		style:  defaultStyles(),
	}
}

func (m Model) Init() tea.Cmd {
	return loadPRCmd(m.opts.Target)
}

func loadPRCmd(target github.Target) tea.Cmd {
	return func() tea.Msg {
		pr, err := github.NewClient().Fetch(context.Background(), target)
		return loadPRMsg{pr: pr, err: err}
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

func readCacheCmd(store *config.Store, key string) tea.Cmd {
	return func() tea.Msg {
		var slices agent.SliceSet
		err := store.ReadJSON(key, &slices)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return cacheMsg{hit: false}
			}
			return cacheMsg{err: err}
		}
		return cacheMsg{slices: &slices, hit: true}
	}
}

func writeCacheCmd(store *config.Store, key string, slices *agent.SliceSet) tea.Cmd {
	return func() tea.Msg {
		if slices == nil {
			return nil
		}
		_ = store.WriteJSON(key, slices)
		return nil
	}
}

func (m Model) maybeStartAI() (Model, tea.Cmd) {
	if m.opts.NoAI {
		m.mode = modeRaw
		m.stage = stageReady
		m.status = "AI disabled. Showing raw diff."
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
	key := m.cacheKey(runner)
	if !m.opts.RegenSlices {
		return m, readCacheCmd(m.opts.Config, key)
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

func (m Model) cacheKey(runner agent.RunnerName) string {
	if m.pr == nil {
		return ""
	}
	return config.BuildSliceCacheKey(m.pr.Owner, m.pr.Repo, m.pr.Number, m.pr.HeadSHA, string(runner), agent.PromptVersion, m.opts.Version)
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

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
