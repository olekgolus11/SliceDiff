package tui

import (
	"charm.land/bubbles/v2/key"
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/config"
	"github.com/olekgolus11/SliceDiff/internal/github"
)

type Options struct {
	Target         github.Target
	HasTarget      bool
	Config         *config.Store
	RunnerOverride string
	NoAI           bool
	RegenSlices    bool
	Version        string
}

type stage int

const (
	stageWelcome stage = iota
	stageLoading
	stageConsent
	stageRunner
	stageReady
	stageFatal
)

type welcomeSection int

const (
	welcomeRequested welcomeSection = iota
	welcomeManual
)

type manualStep int

const (
	manualRepos manualStep = iota
	manualPRs
)

type viewMode int

const (
	modeGrouped viewMode = iota
	modeRaw
)

type panel int

const (
	panelLeft panel = iota
	panelCenter
	panelRight
)

type reviewItem struct {
	ID           string
	Title        string
	Category     string
	Summary      string
	Rationale    string
	HunkRefs     []agent.HunkRef
	ReadingSteps []agent.ReadingStep
	IsUnassigned bool
	IsQuiet      bool
	IsAudit      bool
}

type navigationKind int

const (
	navigationSlice navigationKind = iota
	navigationFile
)

type navigationItem struct {
	kind        navigationKind
	index       int
	title       string
	description string
}

type pickerItem struct {
	index       int
	title       string
	description string
}

func (i pickerItem) Title() string {
	return i.title
}

func (i pickerItem) Description() string {
	return i.description
}

func (i pickerItem) FilterValue() string {
	return i.title + " " + i.description
}

func (i navigationItem) Title() string {
	return i.title
}

func (i navigationItem) Description() string {
	return i.description
}

func (i navigationItem) FilterValue() string {
	return i.title + " " + i.description
}

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Tab      key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	View     key.Binding
	Regen    key.Binding
	Help     key.Binding
	Enter    key.Binding
	Quit     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Up, k.Down, k.PageUp, k.PageDown, k.View, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Left, k.Right, k.Tab, k.Enter},
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.View, k.Regen, k.Help, k.Quit},
	}
}

type loadPRMsg struct {
	pr  *github.PullRequest
	err error
}

type reviewRequestsMsg struct {
	prs []github.PRSearchResult
	err error
}

type repoPoolMsg struct {
	repos []github.RepositorySearchResult
	err   error
}

type repoPRsMsg struct {
	repo string
	prs  []github.PRSearchResult
	err  error
}

type agentMsg struct {
	slices *agent.SliceSet
	err    error
}

type cacheMsg struct {
	slices *agent.SliceSet
	hit    bool
	err    error
}
