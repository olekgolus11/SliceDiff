package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/olekgolus11/SliceDiff/internal/agent"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case loadPRMsg:
		if msg.err != nil {
			m.stage = stageFatal
			m.errMsg = msg.err.Error()
			m.status = "Could not load PR."
			return m, nil
		}
		m.pr = msg.pr
		m.status = "PR loaded."
		return m.maybeStartAI()
	case cacheMsg:
		runner := m.selectedRunner()
		if msg.hit && msg.err == nil {
			if err := agent.ValidateSliceSet(msg.slices, runner, m.pr.HeadSHA, m.pr.Files); err == nil {
				m.slices = msg.slices
				m.mode = modeGrouped
				m.stage = stageReady
				m.aiBusy = false
				m.status = "Loaded cached semantic slices."
				return m, nil
			}
		}
		return m.startAgent(runner)
	case agentMsg:
		m.aiBusy = false
		if msg.err != nil {
			m.mode = modeRaw
			m.stage = stageReady
			m.errMsg = msg.err.Error()
			m.status = "AI grouping failed. Showing raw diff."
			return m, nil
		}
		m.slices = msg.slices
		m.mode = modeGrouped
		m.stage = stageReady
		m.status = "Semantic slices ready."
		key := m.cacheKey(agent.RunnerName(msg.slices.Runner))
		return m, writeCacheCmd(m.opts.Config, key, msg.slices)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.stage {
	case stageConsent:
		return m.handleConsentKey(msg)
	case stageRunner:
		return m.handleRunnerKey(msg)
	case stageReady:
		return m.handleReadyKey(msg)
	case stageFatal:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleConsentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		_ = m.opts.Config.SetConsent(true)
		if m.selectedRunner() == "" {
			m.stage = stageRunner
			m.status = "Choose an AI runner."
			return m, nil
		}
		return m.maybeStartAI()
	case "n", "N":
		_ = m.opts.Config.SetConsent(false)
		m.stage = stageReady
		m.mode = modeRaw
		m.status = "AI consent declined. Showing raw diff."
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleRunnerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "down", "j":
		if m.selectedSetup == 0 {
			m.selectedSetup = 1
		} else {
			m.selectedSetup = 0
		}
	case "enter":
		runner := agent.RunnerCodex
		if m.selectedSetup == 1 {
			runner = agent.RunnerOpenCode
		}
		_ = m.opts.Config.SetRunner(string(runner))
		return m.startAgent(runner)
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleReadyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.focus = (m.focus + 1) % 3
	case "?":
		m.showHelp = !m.showHelp
	case "v":
		if m.mode == modeGrouped && m.slices != nil {
			m.mode = modeRaw
			m.status = "Raw diff view."
		} else if m.slices != nil {
			m.mode = modeGrouped
			m.status = "Grouped slice view."
		}
	case "r":
		if !m.opts.NoAI && m.pr != nil {
			runner := m.selectedRunner()
			if runner != "" {
				return m.startAgent(runner)
			}
			m.stage = stageRunner
			m.status = "Choose an AI runner."
		}
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	case "left", "h":
		if m.focus > panelLeft {
			m.focus--
		}
	case "right", "l":
		if m.focus < panelRight {
			m.focus++
		}
	case "enter":
		if m.focus < panelRight {
			m.focus++
		}
	}
	return m, nil
}

func (m *Model) moveSelection(delta int) {
	if m.mode == modeGrouped && m.slices != nil {
		switch m.focus {
		case panelLeft:
			m.selectedSlice = clamp(m.selectedSlice+delta, 0, len(m.slices.Slices)-1)
		default:
			slice := m.currentSlice()
			if slice != nil {
				m.selectedHunk = clamp(m.selectedHunk+delta, 0, len(slice.HunkRefs)-1)
			}
		}
		return
	}
	switch m.focus {
	case panelLeft:
		if m.pr != nil {
			m.selectedFile = clamp(m.selectedFile+delta, 0, len(m.pr.Files)-1)
			m.selectedHunk = 0
		}
	default:
		file := m.currentFile()
		if file != nil {
			m.selectedHunk = clamp(m.selectedHunk+delta, 0, len(file.Hunks)-1)
		}
	}
}

func clamp(v, min, max int) int {
	if max < min {
		return min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
