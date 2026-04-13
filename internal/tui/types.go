package tui

import (
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/config"
	"github.com/olekgolus11/SliceDiff/internal/github"
)

type Options struct {
	Target         github.Target
	Config         *config.Store
	RunnerOverride string
	NoAI           bool
	RegenSlices    bool
	Version        string
}

type stage int

const (
	stageLoading stage = iota
	stageConsent
	stageRunner
	stageReady
	stageFatal
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

type styles struct {
	title     string
	subtle    string
	selected  string
	panel     string
	focused   string
	status    string
	errorText string
	success   string
}

type loadPRMsg struct {
	pr  *github.PullRequest
	err error
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
