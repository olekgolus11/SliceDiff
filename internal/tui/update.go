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
			m.setAppError(msg.err)
			m.status = m.errorSummary("Could not load PR.")
			return m, nil
		}
		m.pr = msg.pr
		m.clearError()
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
			m.setAppError(msg.err)
			m.status = m.errorSummary("AI grouping failed. Showing raw diff.")
			return m, nil
		}
		m.slices = msg.slices
		m.mode = modeGrouped
		m.stage = stageReady
		m.selectedSlice = 0
		m.selectedHunk = 0
		m.leftScroll = 0
		m.centerScroll = 0
		m.rightScroll = 0
		m.clearError()
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
			m.selectedHunk = 0
			m.resetScrolls()
			m.status = "Raw diff view."
		} else if m.slices != nil {
			m.mode = modeGrouped
			m.selectedHunk = 0
			m.resetScrolls()
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
	case "pgup":
		m.pageFocused(-1)
	case "pgdown":
		m.pageFocused(1)
	case "home":
		m.homeFocused()
	case "end":
		m.endFocused()
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
			before := m.selectedSlice
			m.selectedSlice = clamp(m.selectedSlice+delta, 0, len(m.reviewItems())-1)
			if m.selectedSlice != before {
				m.selectedHunk = 0
				m.centerScroll = 0
				m.rightScroll = 0
			}
			m.ensureSelectedVisible(panelLeft)
		default:
			item := m.currentReviewItem()
			if item != nil {
				m.selectedHunk = clamp(m.selectedHunk+delta, 0, len(item.HunkRefs)-1)
				m.ensureSelectedVisible(panelCenter)
			}
		}
		return
	}
	switch m.focus {
	case panelLeft:
		if m.pr != nil {
			m.selectedFile = clamp(m.selectedFile+delta, 0, len(m.pr.Files)-1)
			m.selectedHunk = 0
			m.centerScroll = 0
			m.rightScroll = 0
			m.ensureSelectedVisible(panelLeft)
		}
	default:
		file := m.currentFile()
		if file != nil {
			m.selectedHunk = clamp(m.selectedHunk+delta, 0, len(file.Hunks)-1)
			m.ensureSelectedVisible(panelCenter)
		}
	}
}

func (m *Model) pageFocused(direction int) {
	page := max(1, m.focusVisibleLines()-1)
	switch m.focus {
	case panelLeft:
		m.moveSelection(direction * page)
	case panelCenter:
		m.centerScroll = clamp(m.centerScroll+(direction*page), 0, max(0, len(m.centerLines())-page))
	case panelRight:
		m.rightScroll = clamp(m.rightScroll+(direction*page), 0, max(0, len(m.rightLines())-page))
	}
}

func (m *Model) homeFocused() {
	switch m.focus {
	case panelLeft:
		if m.mode == modeGrouped {
			m.selectedSlice = 0
		} else {
			m.selectedFile = 0
		}
		m.selectedHunk = 0
		m.leftScroll = 0
	case panelCenter:
		m.centerScroll = 0
		m.selectedHunk = 0
	case panelRight:
		m.rightScroll = 0
	}
}

func (m *Model) endFocused() {
	switch m.focus {
	case panelLeft:
		if m.mode == modeGrouped {
			m.selectedSlice = max(0, len(m.reviewItems())-1)
		} else if m.pr != nil {
			m.selectedFile = max(0, len(m.pr.Files)-1)
		}
		m.selectedHunk = 0
		m.ensureSelectedVisible(panelLeft)
	case panelCenter:
		if m.mode == modeGrouped {
			if item := m.currentReviewItem(); item != nil {
				m.selectedHunk = max(0, len(item.HunkRefs)-1)
			}
		} else if file := m.currentFile(); file != nil {
			m.selectedHunk = max(0, len(file.Hunks)-1)
		}
		m.ensureSelectedVisible(panelCenter)
	case panelRight:
		lines := m.rightLines()
		m.rightScroll = max(0, len(lines)-m.focusVisibleLines())
	}
}

func (m *Model) resetScrolls() {
	m.leftScroll = 0
	m.centerScroll = 0
	m.rightScroll = 0
}

func (m *Model) ensureSelectedVisible(p panel) {
	visible := max(1, m.focusVisibleLines())
	switch p {
	case panelLeft:
		selected := m.selectedFile
		if m.mode == modeGrouped {
			selected = m.selectedSlice
		}
		m.leftScroll = ensureVisible(m.leftScroll, selected, visible)
	case panelCenter:
		m.centerScroll = ensureVisible(m.centerScroll, m.selectedHunk, visible)
	case panelRight:
		m.rightScroll = ensureVisible(m.rightScroll, m.selectedHunk, visible)
	}
}

func ensureVisible(scroll, selected, visible int) int {
	if selected < scroll {
		return selected
	}
	if selected >= scroll+visible {
		return selected - visible + 1
	}
	return scroll
}

func (m Model) focusVisibleLines() int {
	width, height := m.panelSize(m.focus)
	_ = width
	return max(1, height-3)
}

func (m Model) panelSize(p panel) (int, int) {
	titleHeight := 2
	statusHeight := 1
	contentHeight := max(1, m.height-titleHeight-statusHeight)
	if shouldStack(m.width, contentHeight) {
		total := max(9, contentHeight)
		top := total / 3
		mid := total / 3
		bottom := total - top - mid
		switch p {
		case panelLeft:
			return m.width, top
		case panelCenter:
			return m.width, mid
		default:
			return m.width, bottom
		}
	}
	left, center, right := weightedWidths(m.width, []int{1, 2, 2})
	switch p {
	case panelLeft:
		return left, contentHeight
	case panelCenter:
		return center, contentHeight
	default:
		return right, contentHeight
	}
}

func (m Model) errorSummary(fallback string) string {
	if m.appErr == nil || m.appErr.Summary == "" {
		return fallback
	}
	return m.appErr.Summary
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
