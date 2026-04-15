package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/github"
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
		m.diffLineCache = make(map[string]string)
		m.clearError()
		m.status = "PR loaded."
		next, cmd := m.maybeStartAI()
		return next.synced(spinnerCmd, cmd)
	case reviewRequestsMsg:
		if m.welcomeSection != welcomeRequested {
			if msg.err == nil {
				m.reviewPRs = msg.prs
			}
			return m.synced(spinnerCmd)
		}
		m.pickerBusy = false
		if msg.err != nil {
			m.pickerErr = msg.err.Error()
			m.status = "Could not load requested reviews."
		} else {
			m.pickerErr = ""
			m.reviewPRs = msg.prs
			m.selectedPicker = 0
			m.status = "Requested reviews loaded."
		}
		return m.synced(spinnerCmd)
	case repoPoolMsg:
		if m.welcomeSection != welcomeManual {
			if msg.err == nil {
				m.repoPool = msg.repos
				m.repoPoolLoaded = true
			}
			return m.synced(spinnerCmd)
		}
		m.pickerBusy = false
		if msg.err != nil {
			m.pickerErr = msg.err.Error()
			m.status = "Could not load your repositories."
		} else {
			m.pickerErr = ""
			m.repoPool = msg.repos
			m.repoPoolLoaded = true
			m.repoResults = github.FilterRepositories(m.repoPool, m.manualQuery)
			m.selectedPicker = 0
			m.status = "Repository list ready."
		}
		return m.synced(spinnerCmd)
	case repoPRsMsg:
		if msg.repo != m.selectedRepo {
			return m.synced(spinnerCmd)
		}
		if m.welcomeSection != welcomeManual || m.manualStep != manualPRs {
			if msg.err == nil {
				m.repoPRs = msg.prs
			}
			return m.synced(spinnerCmd)
		}
		m.pickerBusy = false
		if msg.err != nil {
			m.pickerErr = msg.err.Error()
			m.status = "Could not load repository pull requests."
		} else {
			m.pickerErr = ""
			m.repoPRs = msg.prs
			m.selectedPicker = 0
			m.status = "Open pull requests loaded."
		}
		return m.synced(spinnerCmd)
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
		keys := m.cacheKeys(agent.RunnerName(msg.slices.Runner))
		return m.synced(spinnerCmd, writeCacheCmd(m.opts.Config, keys, msg.slices))
	case tea.KeyPressMsg:
		next, cmd := m.handleKey(msg)
		return next.synced(spinnerCmd, cmd)
	case tea.MouseClickMsg:
		m.handleMouseClick(msg.Mouse())
		return m.synced(spinnerCmd)
	case tea.MouseWheelMsg:
		if m.handleMouseWheel(msg.Mouse()) {
			return m.synced(spinnerCmd)
		}
		return m, spinnerCmd
	case spinner.TickMsg:
		return m, spinnerCmd
	}
	return m.synced(spinnerCmd)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch m.stage {
	case stageWelcome:
		return m.handleWelcomeKey(msg)
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

func (m Model) handleWelcomeKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Tab):
		return m.switchWelcomeSection()
	case key.Matches(msg, m.keys.Left):
		if m.welcomeSection != welcomeRequested {
			return m.switchWelcomeSection()
		}
	case key.Matches(msg, m.keys.Right):
		if m.welcomeSection != welcomeManual {
			return m.switchWelcomeSection()
		}
	case key.Matches(msg, m.keys.Up):
		m.movePicker(-1)
	case key.Matches(msg, m.keys.Down):
		m.movePicker(1)
	case key.Matches(msg, m.keys.Home):
		m.selectedPicker = 0
	case key.Matches(msg, m.keys.End):
		m.selectedPicker = max(0, m.pickerItemCount()-1)
	case key.Matches(msg, m.keys.Enter):
		return m.selectPickerItem()
	}

	if m.welcomeSection == welcomeManual {
		return m.handleManualTextKey(msg)
	}
	return m, nil
}

func (m Model) switchWelcomeSection() (Model, tea.Cmd) {
	if m.welcomeSection == welcomeRequested {
		m.welcomeSection = welcomeManual
		m.manualStep = manualRepos
		m.selectedPicker = 0
		m.status = "Type a repository search."
		m.pickerErr = ""
		if !m.repoPoolLoaded {
			m.pickerBusy = true
			m.status = "Loading your repositories..."
			return m, loadRepoPoolCmd()
		}
		m.repoResults = github.FilterRepositories(m.repoPool, m.manualQuery)
		return m, nil
	}
	m.welcomeSection = welcomeRequested
	m.selectedPicker = 0
	m.status = "Requested reviews."
	m.pickerErr = ""
	if m.reviewPRs == nil {
		m.pickerBusy = true
		m.status = "Loading requested reviews..."
		return m, loadReviewRequestsCmd()
	}
	return m, nil
}

func (m Model) handleManualTextKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.manualStep == manualPRs {
		switch msg.String() {
		case "esc", "backspace":
			m.manualStep = manualRepos
			m.selectedPicker = 0
			m.status = "Repository search."
			m.pickerErr = ""
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if m.manualQuery != "" {
			m.manualQuery = ""
			m.repoResults = nil
			m.selectedPicker = 0
			m.pickerErr = ""
			m.status = "Type a repository search."
		}
		return m, nil
	case "backspace":
		if m.manualQuery == "" {
			return m, nil
		}
		runes := []rune(m.manualQuery)
		m.manualQuery = string(runes[:len(runes)-1])
		return m.searchManualRepos()
	}

	text := msg.String()
	if len(text) == 1 && text >= " " && text <= "~" {
		m.manualQuery += text
		return m.searchManualRepos()
	}
	return m, nil
}

func (m Model) searchManualRepos() (Model, tea.Cmd) {
	m.manualStep = manualRepos
	m.selectedPicker = 0
	m.repoPRs = nil
	m.selectedRepo = ""
	m.pickerErr = ""
	if strings.TrimSpace(m.manualQuery) == "" {
		m.repoResults = nil
		m.pickerBusy = false
		m.status = "Type a repository search."
		return m, nil
	}
	if !m.repoPoolLoaded {
		if m.pickerBusy {
			return m, nil
		}
		m.pickerBusy = true
		m.status = "Loading your repositories..."
		return m, loadRepoPoolCmd()
	}
	m.repoResults = github.FilterRepositories(m.repoPool, m.manualQuery)
	m.pickerBusy = false
	m.status = "Repository search complete."
	return m, nil
}

func (m *Model) movePicker(delta int) {
	count := m.pickerItemCount()
	if count <= 0 {
		m.selectedPicker = 0
		return
	}
	m.selectedPicker = clamp(m.selectedPicker+delta, 0, count-1)
	m.pickerList.Select(m.selectedPicker)
}

func (m Model) pickerItemCount() int {
	switch m.welcomeSection {
	case welcomeRequested:
		return len(m.reviewPRs)
	case welcomeManual:
		if m.manualStep == manualPRs {
			return len(m.repoPRs)
		}
		return len(m.repoResults)
	default:
		return 0
	}
}

func (m Model) selectPickerItem() (Model, tea.Cmd) {
	if m.pickerBusy {
		return m, nil
	}
	switch m.welcomeSection {
	case welcomeRequested:
		if m.selectedPicker >= 0 && m.selectedPicker < len(m.reviewPRs) {
			return m.startLoadingTarget(m.reviewPRs[m.selectedPicker].Target())
		}
	case welcomeManual:
		if m.manualStep == manualRepos {
			if m.selectedPicker >= 0 && m.selectedPicker < len(m.repoResults) {
				repo := m.repoResults[m.selectedPicker].FullName
				m.selectedRepo = repo
				m.manualStep = manualPRs
				m.selectedPicker = 0
				m.repoPRs = nil
				m.pickerBusy = true
				m.pickerErr = ""
				m.status = "Loading open pull requests..."
				return m, listRepoPRsCmd(repo)
			}
			return m, nil
		}
		if m.selectedPicker >= 0 && m.selectedPicker < len(m.repoPRs) {
			pr := m.repoPRs[m.selectedPicker]
			if pr.Owner == "" || pr.Repo == "" {
				owner, repo, ok := strings.Cut(m.selectedRepo, "/")
				if ok {
					pr.Owner = owner
					pr.Repo = repo
				}
			}
			return m.startLoadingTarget(pr.Target())
		}
	}
	return m, nil
}

func (m Model) startLoadingTarget(target github.Target) (Model, tea.Cmd) {
	m.opts.Target = target
	m.opts.HasTarget = true
	m.stage = stageLoading
	m.status = "Loading pull request..."
	m.clearError()
	m.pr = nil
	m.slices = nil
	m.mode = modeRaw
	m.selectedSlice = 0
	m.selectedFile = 0
	m.selectedHunk = 0
	m.resetScrolls()
	return m, loadPRCmd(target)
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
		m.moveOrScrollLine(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveOrScrollLine(1)
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

func (m *Model) moveOrScrollLine(delta int) {
	if m.focus != panelRight {
		m.moveSelection(delta)
		return
	}
	if delta > 0 {
		m.rightViewport.ScrollDown(delta)
	} else {
		m.rightViewport.ScrollUp(-delta)
	}
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
	case panelRight:
		m.rightViewport.EnsureVisible(m.selectedHunk, 0, 0)
	}
}

func (m *Model) handleMouseClick(mouse tea.Mouse) {
	if m.stage != stageReady {
		return
	}
	if p, ok := m.panelAtMouse(mouse); ok {
		m.focus = p
	}
}

func (m Model) panelAtMouse(mouse tea.Mouse) (panel, bool) {
	headerHeight := lipglossHeight(m.renderHeader())
	if mouse.Y < headerHeight {
		return panelLeft, false
	}
	bodyHeight := max(1, m.height-headerHeight-lipglossHeight(m.renderFooter()))
	relY := mouse.Y - headerHeight
	if relY >= bodyHeight {
		return panelLeft, false
	}
	if shouldStack(m.width, bodyHeight) {
		total := max(9, bodyHeight)
		top := total / 3
		mid := total / 3
		switch {
		case relY < top:
			return panelLeft, true
		case relY < top+mid:
			return panelCenter, true
		default:
			return panelRight, true
		}
	}
	leftW, centerW, _ := horizontalPanelWidths(m.width)
	switch {
	case mouse.X < leftW:
		return panelLeft, true
	case mouse.X < leftW+centerW:
		return panelCenter, true
	default:
		return panelRight, true
	}
}

func (m *Model) handleMouseWheel(mouse tea.Mouse) bool {
	if m.stage != stageReady {
		return false
	}
	target, ok := m.panelAtMouse(mouse)
	if !ok {
		return false
	}
	m.focus = target

	direction := 0
	switch mouse.Button {
	case tea.MouseWheelUp:
		direction = -1
	case tea.MouseWheelDown:
		direction = 1
	default:
		return false
	}

	delta := m.dampenedWheelDelta(target, direction)
	if delta == 0 {
		return false
	}
	return m.scrollPanelByMouse(target, delta)
}

func (m *Model) scrollPanelByMouse(target panel, delta int) bool {
	switch target {
	case panelLeft:
		m.moveSelection(delta)
		return true
	case panelCenter:
		m.moveSelection(delta)
		return true
	case panelRight:
		delta *= 2
		if delta > 0 {
			m.rightViewport.ScrollDown(delta)
		} else {
			m.rightViewport.ScrollUp(-delta)
		}
	}
	return false
}

func (m *Model) dampenedWheelDelta(target panel, direction int) int {
	if target != m.wheelTarget || direction != m.wheelDirection {
		m.wheelTarget = target
		m.wheelDirection = direction
		m.wheelRemainder = 0
	}

	threshold := wheelDampenThreshold(target)
	m.wheelRemainder += direction
	if abs(m.wheelRemainder) < threshold {
		return 0
	}

	delta := m.wheelRemainder / threshold
	m.wheelRemainder -= delta * threshold
	return delta
}

func wheelDampenThreshold(target panel) int {
	switch target {
	case panelRight:
		return 2
	default:
		return 4
	}
}

func (m Model) synced(cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	m.syncComponents()
	return m, tea.Batch(cmds...)
}

func (m *Model) syncComponents() {
	if m.stage == stageWelcome {
		m.syncPickerList()
		return
	}
	m.syncComponentSizes()
	m.syncLeftList()
	m.syncViewportContent()
	m.leftList.Select(m.currentLeftIndex())
}

func (m *Model) syncPickerList() {
	items := m.pickerItems()
	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, item)
	}
	_ = m.pickerList.SetItems(listItems)
	m.pickerList.Select(clamp(m.selectedPicker, 0, max(0, m.pickerItemCount()-1)))
}

func (m *Model) syncComponentSizes() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	contentHeight := max(1, m.height-lipglossHeight(m.renderHeader())-1)
	if shouldStack(m.width, contentHeight) {
		total := max(9, contentHeight)
		top := total / 3
		mid := total / 3
		bottom := total - top - mid
		m.setComponentSizes(m.width, max(1, top-2), m.width, max(1, mid-2), m.width, max(1, bottom-2))
		return
	}
	leftW, centerW, rightW := horizontalPanelWidths(m.width)
	innerHeight := max(1, contentHeight-2)
	m.setComponentSizes(leftW, innerHeight, centerW, innerHeight, rightW, innerHeight)
}

func (m *Model) setComponentSizes(leftW, leftInnerH, centerW, centerInnerH, rightW, rightInnerH int) {
	leftBodyH := max(0, leftInnerH-1)
	leftBodyW := max(1, leftW-2)
	m.leftList.SetSize(leftBodyW, leftBodyH)

	centerBodyH := max(0, centerInnerH-1)
	centerBodyW := max(1, centerW-2)
	overview := cropLines(strings.Join(m.centerOverviewStyledLines(centerBodyW), "\n"), m.centerOverviewHeight(centerBodyH), centerBodyW)
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
	centerLines, selectedStart, selectedEnd := m.centerScrollStyledLineRange(m.centerViewport.Width())
	m.centerViewport.SetContentLines(centerLines)
	if selectedStart >= 0 {
		ensureViewportRangeVisible(&m.centerViewport, selectedStart, selectedEnd)
	}
	m.rightViewport.SetContentLines(m.rightStyledLines())
}

func ensureViewportRangeVisible(v *viewport.Model, start, end int) {
	if start < 0 {
		return
	}
	if end < start {
		end = start
	}
	height := max(1, v.Height())
	top := v.YOffset()
	switch {
	case start < top:
		v.SetYOffset(start)
	case end-start+1 >= height:
		v.SetYOffset(start)
	default:
		v.SetYOffset(end - height + 1)
	}
}

func (m Model) focusVisibleLines() int {
	_, height := m.panelSize(m.focus)
	return max(1, height-3)
}

func (m Model) panelSize(p panel) (int, int) {
	titleHeight := lipglossHeight(m.renderHeader())
	contentHeight := max(1, m.height-titleHeight-1)
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
	left, center, right := horizontalPanelWidths(m.width)
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

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
