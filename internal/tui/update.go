package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/olekgolus11/SliceDiff/internal/agent"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.synced(spinnerCmd)
	case loadPRMsg:
		if msg.err != nil {
			m.stage = stageFatal
			m.setAppError(msg.err)
			m.status = m.errorSummary("Could not load PR.")
			return m.synced(spinnerCmd)
		}
		m.pr = msg.pr
		m.clearError()
		m.status = "PR loaded."
		next, cmd := m.maybeStartAI()
		return next.synced(spinnerCmd, cmd)
	case cacheMsg:
		runner := m.selectedRunner()
		if msg.hit && msg.err == nil {
			if err := agent.ValidateSliceSet(msg.slices, runner, m.pr.HeadSHA, m.pr.Files); err == nil {
				m.slices = msg.slices
				m.mode = modeGrouped
				m.stage = stageReady
				m.aiBusy = false
				m.status = "Loaded cached semantic slices."
				return m.synced(spinnerCmd)
			}
		}
		next, cmd := m.startAgent(runner)
		return next.synced(spinnerCmd, cmd)
	case agentMsg:
		m.aiBusy = false
		if msg.err != nil {
			m.mode = modeRaw
			m.stage = stageReady
			m.setAppError(msg.err)
			m.status = m.errorSummary("AI grouping failed. Showing raw diff.")
			return m.synced(spinnerCmd)
		}
		m.slices = msg.slices
		m.mode = modeGrouped
		m.stage = stageReady
		m.selectedSlice = 0
		m.selectedHunk = 0
		m.resetScrolls()
		m.clearError()
		m.status = "Semantic slices ready."
		key := m.cacheKey(agent.RunnerName(msg.slices.Runner))
		return m.synced(spinnerCmd, writeCacheCmd(m.opts.Config, key, msg.slices))
	case tea.KeyPressMsg:
		next, cmd := m.handleKey(msg)
		return next.synced(spinnerCmd, cmd)
	case tea.MouseClickMsg:
		m.handleMouseClick(msg.Mouse())
		return m.synced(spinnerCmd)
	case tea.MouseWheelMsg:
		m.handleMouseWheel(msg.Mouse())
		return m.synced(spinnerCmd)
	case spinner.TickMsg:
		return m.synced(spinnerCmd)
	}
	return m.synced(spinnerCmd)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch m.stage {
	case stageConsent:
		return m.handleConsentKey(msg)
	case stageRunner:
		return m.handleRunnerKey(msg)
	case stageReady:
		return m.handleReadyKey(msg)
	case stageFatal:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleConsentKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleRunnerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up, m.keys.Down):
		if m.selectedSetup == 0 {
			m.selectedSetup = 1
		} else {
			m.selectedSetup = 0
		}
	case key.Matches(msg, m.keys.Enter):
		runner := agent.RunnerCodex
		if m.selectedSetup == 1 {
			runner = agent.RunnerOpenCode
		}
		_ = m.opts.Config.SetRunner(string(runner))
		return m.startAgent(runner)
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleReadyKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Tab):
		m.focus = (m.focus + 1) % 3
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
	case key.Matches(msg, m.keys.View):
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
	case key.Matches(msg, m.keys.Regen):
		if !m.opts.NoAI && m.pr != nil {
			runner := m.selectedRunner()
			if runner != "" {
				return m.startAgent(runner)
			}
			m.stage = stageRunner
			m.status = "Choose an AI runner."
		}
	case key.Matches(msg, m.keys.Up):
		m.moveSelection(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveSelection(1)
	case key.Matches(msg, m.keys.PageUp):
		m.pageFocused(-1)
	case key.Matches(msg, m.keys.PageDown):
		m.pageFocused(1)
	case key.Matches(msg, m.keys.Home):
		m.homeFocused()
	case key.Matches(msg, m.keys.End):
		m.endFocused()
	case key.Matches(msg, m.keys.Left):
		if m.focus > panelLeft {
			m.focus--
		}
	case key.Matches(msg, m.keys.Right):
		if m.focus < panelRight {
			m.focus++
		}
	case key.Matches(msg, m.keys.Enter):
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
				m.centerViewport.GotoTop()
				m.rightViewport.GotoTop()
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
			before := m.selectedFile
			m.selectedFile = clamp(m.selectedFile+delta, 0, len(m.pr.Files)-1)
			if m.selectedFile != before {
				m.selectedHunk = 0
				m.centerViewport.GotoTop()
				m.rightViewport.GotoTop()
			}
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
		if direction > 0 {
			m.centerViewport.PageDown()
		} else {
			m.centerViewport.PageUp()
		}
	case panelRight:
		if direction > 0 {
			m.rightViewport.PageDown()
		} else {
			m.rightViewport.PageUp()
		}
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
		m.leftList.GoToStart()
	case panelCenter:
		m.centerViewport.GotoTop()
		m.selectedHunk = 0
	case panelRight:
		m.rightViewport.GotoTop()
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
		m.rightViewport.GotoBottom()
	}
}

func (m *Model) resetScrolls() {
	m.leftList.GoToStart()
	m.centerViewport.GotoTop()
	m.rightViewport.GotoTop()
}

func (m *Model) ensureSelectedVisible(p panel) {
	switch p {
	case panelLeft:
		m.leftList.Select(m.currentLeftIndex())
	case panelCenter:
		m.syncViewportContent()
		_, selectedLine := m.centerScrollStyledLines()
		if selectedLine >= 0 {
			m.centerViewport.EnsureVisible(selectedLine, 0, 0)
		}
	case panelRight:
		m.rightViewport.EnsureVisible(m.selectedHunk, 0, 0)
	}
}

func (m *Model) handleMouseClick(mouse tea.Mouse) {
	if m.stage != stageReady {
		return
	}
	headerHeight := lipglossHeight(m.renderHeader())
	if mouse.Y < headerHeight {
		return
	}
	bodyHeight := max(1, m.height-headerHeight-lipglossHeight(m.renderFooter()))
	relY := mouse.Y - headerHeight
	if relY >= bodyHeight {
		return
	}
	if shouldStack(m.width, bodyHeight) {
		total := max(9, bodyHeight)
		top := total / 3
		mid := total / 3
		switch {
		case relY < top:
			m.focus = panelLeft
		case relY < top+mid:
			m.focus = panelCenter
		default:
			m.focus = panelRight
		}
		return
	}
	leftW, centerW, _ := weightedWidths(m.width, []int{1, 2, 2})
	switch {
	case mouse.X < leftW:
		m.focus = panelLeft
	case mouse.X < leftW+centerW:
		m.focus = panelCenter
	default:
		m.focus = panelRight
	}
}

func (m *Model) handleMouseWheel(mouse tea.Mouse) {
	if m.stage != stageReady {
		return
	}
	switch mouse.Button {
	case tea.MouseWheelUp:
		m.pageFocused(-1)
	case tea.MouseWheelDown:
		m.pageFocused(1)
	}
}

func (m Model) synced(cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	m.syncComponents()
	return m, tea.Batch(cmds...)
}

func (m *Model) syncComponents() {
	m.syncComponentSizes()
	m.syncLeftList()
	m.syncViewportContent()
	m.leftList.Select(m.currentLeftIndex())
}

func (m *Model) syncComponentSizes() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	contentHeight := max(1, m.height-lipglossHeight(m.renderHeader())-lipglossHeight(m.renderFooter()))
	if shouldStack(m.width, contentHeight) {
		total := max(9, contentHeight)
		top := total / 3
		mid := total / 3
		bottom := total - top - mid
		m.setComponentSizes(m.width, max(1, top-2), m.width, max(1, mid-2), m.width, max(1, bottom-2))
		return
	}
	leftW, centerW, rightW := weightedWidths(m.width, []int{1, 2, 2})
	innerHeight := max(1, contentHeight-2)
	m.setComponentSizes(leftW, innerHeight, centerW, innerHeight, rightW, innerHeight)
}

func (m *Model) setComponentSizes(leftW, leftInnerH, centerW, centerInnerH, rightW, rightInnerH int) {
	leftBodyH := max(0, leftInnerH-1)
	leftBodyW := max(1, leftW-2)
	m.leftList.SetSize(leftBodyW, leftBodyH)

	centerBodyH := max(0, centerInnerH-1)
	centerBodyW := max(1, centerW-2)
	overview := cropLines(strings.Join(m.centerOverviewStyledLines(centerBodyW), "\n"), min(centerBodyH, 8), centerBodyW)
	m.centerViewport.SetWidth(centerBodyW)
	m.centerViewport.SetHeight(max(0, centerBodyH-lipgloss.Height(overview)))

	m.rightViewport.SetWidth(max(1, rightW-2))
	m.rightViewport.SetHeight(max(0, rightInnerH-1))
}

func (m *Model) syncLeftList() {
	items := m.leftItems()
	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, item)
	}
	_ = m.leftList.SetItems(listItems)
}

func (m *Model) syncViewportContent() {
	centerLines, selectedLine := m.centerScrollStyledLines()
	m.centerViewport.SetContentLines(centerLines)
	if selectedLine >= 0 {
		m.centerViewport.EnsureVisible(selectedLine, 0, 0)
	}
	m.rightViewport.SetContentLines(m.rightStyledLines())
}

func (m Model) focusVisibleLines() int {
	_, height := m.panelSize(m.focus)
	return max(1, height-3)
}

func (m Model) panelSize(p panel) (int, int) {
	titleHeight := lipglossHeight(m.renderHeader())
	statusHeight := lipglossHeight(m.renderFooter())
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

func lipglossHeight(s string) int {
	return max(1, lipgloss.Height(s))
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
