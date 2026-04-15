package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/diff"
)

func (m Model) View() tea.View {
	content := "SliceDiff loading..."
	if m.width > 0 {
		switch m.stage {
		case stageWelcome:
			content = m.renderWelcome()
		case stageLoading:
			content = m.renderFrame("Loading pull request...", []string{"Validating gh auth status", "Fetching PR metadata and diff"})
		case stageConsent:
			content = m.renderFrame("AI consent", []string{
				"SliceDiff can send PR metadata and structured diff hunks to your selected agent runner.",
				"Press y to allow AI grouping, or n to use raw diff navigation only.",
			})
		case stageRunner:
			content = m.renderRunnerPicker()
		case stageFatal:
			content = m.renderFrame("Could not start SliceDiff", append(m.errorLines(), "Press q to quit."))
		case stageReady:
			content = m.renderMain()
		}
	}

	view := tea.NewView(content)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.WindowTitle = "SliceDiff"
	return view
}

func (m Model) renderMain() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	contentHeight := max(1, m.height-lipgloss.Height(header)-1)
	body := m.renderPanels(m.width, contentHeight)
	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return m.style.app.Width(m.width).Render(fitLines(screen, m.height, m.width))
}

func (m Model) renderWelcome() string {
	footer := m.renderPickerFooter()
	bodyHeight := max(1, m.height-lipgloss.Height(footer))
	body := m.renderWelcomeBody(m.width, bodyHeight)
	screen := lipgloss.JoinVertical(lipgloss.Left, body, footer)
	return m.style.app.Width(m.width).Render(fitLines(screen, m.height, m.width))
}

func (m Model) renderWelcomeCluster(width int) string {
	requested := m.style.chipHot.Render("Requested review")
	manual := m.style.chip.Render("Manual")
	if m.welcomeSection == welcomeManual {
		requested = m.style.chip.Render("Requested review")
		manual = m.style.chipHot.Render("Manual")
	}
	lines := []string{}
	if width >= 24 {
		lines = append(lines, strings.Split(m.renderCakeLogo(width), "\n")...)
	}
	lines = append(lines,
		m.style.headerTitle.Render("SliceDiff"),
		m.style.diffContext.Render("Choose a pull request"),
		lipgloss.JoinHorizontal(lipgloss.Center,
			requested,
			" ",
			manual,
		),
		m.style.subtle.Render(m.welcomeSubtitle()),
	)
	for i, line := range lines {
		lines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Center, ansi.Truncate(line, width, "..."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCakeLogo(width int) string {
	lines := []string{
		"     i i i",
		"     | | |",
		"  .--'-----'--.",
		"  |  SLICE    |",
		"  |   DIFF    |",
		"  '-----------'",
	}
	for i, line := range lines {
		style := m.style.diffContext
		if i < 2 {
			style = m.style.warning
		}
		if i == 3 || i == 4 {
			style = m.style.emphasis
		}
		lines[i] = style.Render(lipgloss.PlaceHorizontal(width, lipgloss.Center, ansi.Truncate(line, width, "...")))
	}
	return strings.Join(lines, "\n")
}

func (m Model) welcomeSubtitle() string {
	if m.welcomeSection == welcomeRequested {
		return "Review requests assigned to you, sorted by recent activity."
	}
	if m.manualStep == manualPRs && m.selectedRepo != "" {
		return "Open pull requests in " + m.selectedRepo + "."
	}
	return "Search for a repository, then choose one of its open pull requests."
}

func (m Model) renderWelcomeBody(width, height int) string {
	bodyWidth := max(1, width-2)
	bodyHeight := max(0, height-2)
	cluster := m.renderWelcomeCluster(bodyWidth)
	clusterHeight := lipgloss.Height(cluster)
	lines := make([]string, 0, bodyHeight)
	promptLines := []string{}
	if m.welcomeSection == welcomeManual && m.manualStep == manualRepos {
		prompt := "/ " + m.manualQuery
		if m.manualQuery == "" {
			prompt = "/ type a repository name"
		}
		promptLines = append(promptLines, m.style.callout.Render(ansi.Truncate(prompt, bodyWidth, "...")), "")
	}
	minListHeight := 6
	if bodyHeight < 18 {
		minListHeight = 3
	}
	topPad := max(0, (bodyHeight-clusterHeight-len(promptLines)-minListHeight)/2)
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, cluster, "")
	lines = append(lines, promptLines...)
	if m.pickerBusy {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " ", "Loading..."))
	} else if m.pickerErr != "" {
		lines = append(lines, m.style.errorText.Render("Could not load choices."), m.style.diffContext.Render(ansi.Truncate(m.pickerErr, bodyWidth, "...")))
	} else {
		m.syncPickerList()
		listHeight := max(0, bodyHeight-lipgloss.Height(strings.Join(lines, "\n")))
		pickerList := m.pickerList
		pickerList.SetSize(bodyWidth, listHeight)
		pickerList.Select(clamp(m.selectedPicker, 0, max(0, m.pickerItemCount()-1)))
		lines = append(lines, fitLines(pickerList.View(), listHeight, bodyWidth))
	}
	body := fitLines(strings.Join(lines, "\n"), bodyHeight, bodyWidth)
	return m.renderPanel(m.welcomePanelTitle(), body, width, max(1, height-2), true)
}

func (m Model) renderPickerFooter() string {
	help := "up/down move | enter selects | tab switches | q quits"
	if m.welcomeSection == welcomeManual {
		help = "type to search | enter selects | esc/backspace goes back | tab switches | q quits"
	}
	statusText := m.style.status.Render(truncate(m.status, max(1, m.width/3)))
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		statusText,
		m.style.subtle.Render(" | "),
		m.style.subtle.Render(truncate(help, max(1, m.width-lipgloss.Width(statusText)-3))),
	)
	return m.style.footer.Width(m.width).Render(fitLines(line, 1, m.width))
}

func (m Model) welcomePanelTitle() string {
	switch m.welcomeSection {
	case welcomeRequested:
		total := len(m.reviewPRs)
		if total == 0 {
			return "Requested review 0/0"
		}
		return fmt.Sprintf("Requested review %d/%d", clamp(m.selectedPicker+1, 1, total), total)
	case welcomeManual:
		if m.manualStep == manualPRs {
			total := len(m.repoPRs)
			if total == 0 {
				return "Manual pull requests 0/0"
			}
			return fmt.Sprintf("Manual pull requests %d/%d", clamp(m.selectedPicker+1, 1, total), total)
		}
		total := len(m.repoResults)
		if total == 0 {
			return "Manual repositories 0/0"
		}
		return fmt.Sprintf("Manual repositories %d/%d", clamp(m.selectedPicker+1, 1, total), total)
	default:
		return "Choose pull request"
	}
}

func (m Model) renderHeader() string {
	name := "SliceDiff"
	meta := "waiting for PR metadata"
	if m.pr != nil {
		name = fmt.Sprintf("%s/%s#%d", m.pr.Owner, m.pr.Repo, m.pr.Number)
		meta = m.prTitle()
	}

	mode := "raw"
	if m.mode == modeGrouped {
		mode = "grouped"
	}

	counts := m.headerCounts()
	line1 := lipgloss.JoinHorizontal(lipgloss.Center,
		m.style.headerTitle.Render("SliceDiff"),
		"  ",
		m.style.emphasis.Render(truncate(name, max(10, m.width/3))),
		"  ",
		m.style.chipHot.Render(mode),
		" ",
		m.style.chipCool.Render("runner "+m.runnerLabel()),
		" ",
		m.style.chip.Render(counts),
	)
	line1 = ansi.Truncate(line1, m.width, "...")
	line2 := m.style.headerMeta.Render(truncate(meta, max(1, m.width-2)))
	line2 = ansi.Truncate(line2, m.width, "...")
	return m.style.header.Width(m.width).Render(lipgloss.JoinVertical(lipgloss.Left, line1, line2))
}

func (m Model) headerCounts() string {
	if m.pr == nil {
		return "loading"
	}
	focus := 0
	quiet := 0
	audit := 0
	for _, file := range m.pr.Files {
		for _, hunk := range file.Hunks {
			switch hunkSignal(hunk) {
			case diff.HunkSignalQuiet:
				quiet++
			case diff.HunkSignalAudit:
				audit++
			default:
				focus++
			}
		}
	}
	return fmt.Sprintf("%d focus / %d quiet / %d audit", focus, quiet, audit)
}

func (m Model) renderFooter() string {
	status := m.status
	if m.appErr != nil {
		status = m.appErr.Summary
	}
	statusText := m.style.status.Render(truncate(status, max(1, m.width/3)))
	helpModel := m.help
	helpModel.ShowAll = m.showHelp
	helpModel.SetWidth(max(1, m.width-lipgloss.Width(statusText)-3))
	helpText := helpModel.View(m.keys)
	helpText = firstLine(helpText)
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		statusText,
		m.style.subtle.Render(" | "),
		helpText,
	)
	return m.style.footer.Width(m.width).Render(fitLines(line, 1, m.width))
}

func (m Model) renderPanels(width, height int) string {
	if shouldStack(width, height) {
		return m.renderStacked(width, height)
	}
	leftW, centerW, rightW := horizontalPanelWidths(width)
	innerHeight := max(1, height-2)
	left := m.renderListPanel(m.leftTitle(), leftW, innerHeight, m.focus == panelLeft)
	center := m.renderCenterPanel(m.centerTitle(), centerW, innerHeight, m.focus == panelCenter)
	right := m.renderViewportPanel(m.rightTitle(), panelRight, rightW, innerHeight, m.focus == panelRight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (m Model) renderStacked(width, height int) string {
	total := max(9, height)
	top := total / 3
	mid := total / 3
	bottom := total - top - mid
	left := m.renderListPanel(m.leftTitle(), width, max(1, top-2), m.focus == panelLeft)
	center := m.renderCenterPanel(m.centerTitle(), width, max(1, mid-2), m.focus == panelCenter)
	right := m.renderViewportPanel(m.rightTitle(), panelRight, width, max(1, bottom-2), m.focus == panelRight)
	return lipgloss.JoinVertical(lipgloss.Left, left, center, right)
}

func (m Model) renderListPanel(title string, width, innerHeight int, focused bool) string {
	bodyHeight := max(0, innerHeight-1)
	bodyWidth := max(1, width-2)
	listModel := m.leftList
	listModel.SetSize(bodyWidth, bodyHeight)
	listModel.Select(m.currentLeftIndex())
	body := fitLines(listModel.View(), bodyHeight, bodyWidth)
	return m.renderPanel(title, body, width, innerHeight, focused)
}

func (m Model) renderViewportPanel(title string, p panel, width, innerHeight int, focused bool) string {
	bodyHeight := max(0, innerHeight-1)
	bodyWidth := max(1, width-2)
	viewportModel := m.centerViewport
	if p == panelRight {
		viewportModel = m.rightViewport
	}
	viewportModel.SetWidth(bodyWidth)
	viewportModel.SetHeight(bodyHeight)
	body := fitLines(viewportModel.View(), bodyHeight, bodyWidth)
	return m.renderPanel(title, body, width, innerHeight, focused)
}

func (m Model) renderCenterPanel(title string, width, innerHeight int, focused bool) string {
	bodyHeight := max(0, innerHeight-1)
	bodyWidth := max(1, width-2)
	overview := cropLines(strings.Join(m.centerOverviewStyledLines(bodyWidth), "\n"), m.centerOverviewHeight(bodyHeight), bodyWidth)
	overviewHeight := lipgloss.Height(overview)
	remainingHeight := max(0, bodyHeight-overviewHeight)

	viewportModel := m.centerViewport
	viewportModel.SetWidth(bodyWidth)
	viewportModel.SetHeight(remainingHeight)
	scrolling := fitLines(viewportModel.View(), remainingHeight, bodyWidth)
	body := overview
	if remainingHeight > 0 {
		body = lipgloss.JoinVertical(lipgloss.Left, overview, scrolling)
	}
	return m.renderPanel(title, fitLines(body, bodyHeight, bodyWidth), width, innerHeight, focused)
}

func (m Model) centerOverviewHeight(bodyHeight int) int {
	if m.mode == modeGrouped && m.slices != nil {
		return min(bodyHeight, 12)
	}
	return min(bodyHeight, 8)
}

func (m Model) renderPanel(title, body string, width, innerHeight int, focused bool) string {
	contentWidth := max(1, width-2)
	titleLine := m.style.panelTitle.Render(ansi.Truncate(title, contentWidth, "..."))
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, fitLines(body, max(0, innerHeight-1), contentWidth))
	return panelStyle(m.style, focused, width).Render(content)
}

func (m Model) leftItems() []navigationItem {
	if m.mode == modeGrouped && m.slices != nil {
		items := m.reviewItems()
		if len(items) == 0 {
			return []navigationItem{{kind: navigationSlice, title: "No semantic slices", description: "Press v for raw diff or r to regenerate."}}
		}
		nav := make([]navigationItem, 0, len(items))
		for i, item := range items {
			desc := fmt.Sprintf("%s / %d hunks", item.Category, len(item.HunkRefs))
			if item.IsUnassigned {
				desc = fmt.Sprintf("uncertain / needs review / %d hunks", len(item.HunkRefs))
			}
			if item.IsQuiet {
				desc = fmt.Sprintf("format/import whitespace, still reviewable / %d hunks", len(item.HunkRefs))
			}
			if item.IsAudit {
				desc = fmt.Sprintf("generated/vendor/lockfile verification / %d hunks", len(item.HunkRefs))
			}
			nav = append(nav, navigationItem{
				kind:        navigationSlice,
				index:       i,
				title:       fmt.Sprintf("%02d  %s", i+1, item.Title),
				description: desc,
			})
		}
		return nav
	}
	if m.pr == nil || len(m.pr.Files) == 0 {
		return []navigationItem{{kind: navigationFile, title: "No files changed", description: "This PR has no parsed diff files."}}
	}
	nav := make([]navigationItem, 0, len(m.pr.Files))
	for i, file := range m.pr.Files {
		flags := []string{file.Status, fmt.Sprintf("%d hunks", len(file.Hunks))}
		if file.IsGenerated {
			flags = append(flags, "generated")
		}
		if file.IsBinary {
			flags = append(flags, "binary")
		}
		nav = append(nav, navigationItem{
			kind:        navigationFile,
			index:       i,
			title:       file.Path,
			description: strings.Join(flags, " / "),
		})
	}
	return nav
}

func (m Model) pickerItems() []pickerItem {
	if m.pickerBusy {
		return []pickerItem{{title: "Loading choices", description: "SliceDiff is reading from gh."}}
	}
	if m.pickerErr != "" {
		return []pickerItem{{title: "Could not load choices", description: m.pickerErr}}
	}
	switch m.welcomeSection {
	case welcomeRequested:
		if len(m.reviewPRs) == 0 {
			return []pickerItem{{title: "No requested reviews", description: "Switch to Manual to choose a repository."}}
		}
		items := make([]pickerItem, 0, len(m.reviewPRs))
		for i, pr := range m.reviewPRs {
			desc := fmt.Sprintf("%s#%d", pr.RepoName(), pr.Number)
			if pr.Author != "" {
				desc += " / " + pr.Author
			}
			if pr.IsDraft {
				desc += " / draft"
			}
			if !pr.UpdatedAt.IsZero() {
				desc += " / updated " + pr.UpdatedAt.Format("2006-01-02")
			}
			items = append(items, pickerItem{index: i, title: pr.Title, description: desc})
		}
		return items
	case welcomeManual:
		if m.manualStep == manualPRs {
			if len(m.repoPRs) == 0 {
				return []pickerItem{{title: "No open pull requests", description: "Press esc or backspace to choose another repository."}}
			}
			items := make([]pickerItem, 0, len(m.repoPRs))
			for i, pr := range m.repoPRs {
				desc := fmt.Sprintf("%s#%d", m.selectedRepo, pr.Number)
				if pr.Author != "" {
					desc += " / " + pr.Author
				}
				if pr.IsDraft {
					desc += " / draft"
				}
				if !pr.UpdatedAt.IsZero() {
					desc += " / updated " + pr.UpdatedAt.Format("2006-01-02")
				}
				items = append(items, pickerItem{index: i, title: pr.Title, description: desc})
			}
			return items
		}
		if strings.TrimSpace(m.manualQuery) == "" {
			return []pickerItem{{title: "Search repositories", description: "Type an owner or repository name to begin."}}
		}
		if len(m.repoResults) == 0 {
			return []pickerItem{{title: "No repositories found", description: "Try a different owner or repository name."}}
		}
		items := make([]pickerItem, 0, len(m.repoResults))
		for i, repo := range m.repoResults {
			desc := repo.Description
			if desc == "" {
				desc = "No description"
			}
			if repo.IsPrivate {
				desc += " / private"
			}
			when := repo.PushedAt
			if when.IsZero() {
				when = repo.UpdatedAt
			}
			if !when.IsZero() {
				desc += " / updated " + when.Format("2006-01-02")
			}
			items = append(items, pickerItem{index: i, title: repo.FullName, description: desc})
		}
		return items
	default:
		return nil
	}
}

func (m Model) leftLines() []string {
	items := m.leftItems()
	lines := make([]string, 0, len(items))
	for i, item := range items {
		prefix := "  "
		if i == m.currentLeftIndex() {
			prefix = "> "
		}
		lines = append(lines, prefix+item.title)
	}
	return lines
}

func (m Model) centerLines() []string {
	lines, _ := m.centerPlainLines()
	return lines
}

func (m Model) centerPlainLines() ([]string, int) {
	prefix := m.errorLines()
	if m.mode == modeGrouped && m.slices != nil {
		item := m.currentReviewItem()
		if item == nil {
			return append(prefix, "No selected slice."), -1
		}
		lines := append(prefix, []string{
			"Title: " + item.Title,
			"Category: " + item.Category,
			"",
			"Summary:",
		}...)
		lines = append(lines, wrapWords(item.Summary, 80)...)
		lines = append(lines, "", "Reading order:")
		selectedLine := -1
		for i, step := range item.ReadingSteps {
			prefix := "  "
			if i == m.selectedHunk {
				prefix = "> "
				selectedLine = len(lines)
			}
			lines = append(lines, prefix+step.Body)
			lines = append(lines, "  > "+step.HunkRef.FilePath+"  "+step.HunkRef.HunkID)
		}
		return lines, selectedLine
	}
	file := m.currentFile()
	if file == nil {
		return append(prefix, "No selected file."), -1
	}
	if file.IsBinary {
		return append(prefix, []string{
			"Path: " + file.Path,
			"Status: " + file.Status,
			"Binary: true",
			"",
			"Binary files do not include line hunks in the unified diff.",
		}...), -1
	}
	lines := append(prefix, []string{
		"Path: " + file.Path,
		"Status: " + file.Status,
		fmt.Sprintf("Binary: %t", file.IsBinary),
		fmt.Sprintf("Generated/lockfile: %t", file.IsGenerated),
		"",
		"Hunks:",
	}...)
	if len(file.Hunks) == 0 {
		lines = append(lines, "No text hunks available for this file.")
		return lines, -1
	}
	selectedLine := -1
	for i, hunk := range file.Hunks {
		prefix := "  "
		if i == m.selectedHunk {
			prefix = "> "
			selectedLine = len(lines)
		}
		lines = append(lines, prefix+hunk.ID+" "+hunk.Header)
	}
	return lines, selectedLine
}

func (m Model) centerOverviewStyledLines(width int) []string {
	prefix := m.errorStyledLines()
	if m.mode == modeGrouped && m.slices != nil {
		item := m.currentReviewItem()
		if item == nil {
			return append(prefix, m.callout("No selected slice."))
		}
		lines := append(prefix,
			m.style.emphasis.Render(item.Title),
			m.renderBadges(item.Category),
			"",
			m.style.section.Render("Summary"),
		)
		lines = append(lines, styledWrap(item.Summary, max(24, width), m.style.detailText)...)
		return lines
	}

	file := m.currentFile()
	if file == nil {
		return append(prefix, m.callout("No selected file."))
	}
	if file.IsBinary {
		return append(prefix,
			m.style.emphasis.Render(file.Path),
			m.renderBadges(file.Status, "binary"),
			"",
			m.callout("Binary files do not include line hunks in the unified diff."),
		)
	}
	return append(prefix,
		m.style.emphasis.Render(file.Path),
		m.renderBadges(file.Status, generatedLabel(file)),
	)
}

func (m Model) centerScrollStyledLines(width int) ([]string, int) {
	if m.mode == modeGrouped && m.slices != nil {
		item := m.currentReviewItem()
		if item == nil {
			return []string{m.callout("No selected slice.")}, -1
		}
		lines := []string{"", m.style.section.Render("Reading order")}
		selectedLine := -1
		for i, step := range item.ReadingSteps {
			selected := i == m.selectedHunk
			lines = append(lines, m.renderReadingStepBody(step.Body, width, selected)...)
			if selected {
				selectedLine = len(lines)
			}
			lines = append(lines, m.renderReadingStepRef(step.HunkRef, width, selected), "")
		}
		return lines, selectedLine
	}

	file := m.currentFile()
	if file == nil {
		return []string{m.callout("No selected file.")}, -1
	}
	if file.IsBinary {
		return []string{m.callout("Binary files do not include line hunks in the unified diff.")}, -1
	}
	lines := []string{m.style.section.Render("Hunks")}
	if len(file.Hunks) == 0 {
		lines = append(lines, m.callout("No text hunks available for this file."))
		return lines, -1
	}
	selectedLine := -1
	for i, hunk := range file.Hunks {
		line := fmt.Sprintf("  %s  %s", hunk.ID, hunk.Header)
		if hunkSignal(hunk) != diff.HunkSignalFocus {
			line += "  " + string(hunkSignal(hunk)) + ":" + hunkQuietReason(hunk)
		}
		if i == m.selectedHunk {
			line = m.style.diffSelected.Render("> " + hunk.ID + "  " + hunk.Header)
			if hunkSignal(hunk) != diff.HunkSignalFocus {
				line += " " + m.style.subtle.Render(string(hunkSignal(hunk))+":"+hunkQuietReason(hunk))
			}
			selectedLine = len(lines)
		} else {
			line = m.style.diffContext.Render(line)
		}
		lines = append(lines, line)
	}
	return lines, selectedLine
}

func (m Model) renderReadingStepBody(body string, width int, selected bool) []string {
	prefix := m.readingStepPrefix(selected)
	lines := wrapWords(body, max(1, width-lipgloss.Width(prefix)))
	for i, line := range lines {
		lines[i] = prefix + m.style.detailText.Render(line)
	}
	return lines
}

func (m Model) renderReadingStepRef(ref agent.HunkRef, width int, selected bool) string {
	prefix := m.readingStepRefPrefix(selected)
	hunk := ref.HunkID
	separator := "  "
	if selected {
		hunk = m.style.detailHunk.Render(ref.HunkID)
	}
	stemWidth := lipgloss.Width(prefix) + lipgloss.Width(hunk) + ansi.StringWidth(separator)
	path := shortenPathAfterFirstSlash(ref.FilePath, max(1, width-stemWidth))
	return prefix + hunk + separator + m.style.detailMeta.Render(path)
}

func (m Model) readingStepPrefix(selected bool) string {
	if selected {
		return m.style.detailRail.Render(" ") + " "
	}
	return "  "
}

func (m Model) readingStepRefPrefix(selected bool) string {
	if selected {
		return m.style.detailRail.Render(" ") + "   "
	}
	return "    "
}

func (m Model) centerStyledLines() ([]string, int) {
	overview := m.centerOverviewStyledLines(82)
	scrolling, selectedLine := m.centerScrollStyledLines(82)
	if selectedLine >= 0 {
		selectedLine += len(overview)
	}
	return append(overview, scrolling...), selectedLine
}

func (m Model) errorLines() []string {
	if m.appErr == nil {
		return nil
	}
	lines := []string{
		"Last error:",
		"Kind: " + string(m.appErr.Kind),
		"Summary: " + m.appErr.Summary,
		"Recovery:",
	}
	lines = append(lines, wrapWords(m.appErr.Recovery, 80)...)
	lines = append(lines, "", "Details:")
	lines = append(lines, wrapWords(m.appErr.Detail, 80)...)
	lines = append(lines, "")
	return lines
}

func (m Model) errorStyledLines() []string {
	if m.appErr == nil {
		return nil
	}
	lines := []string{
		m.style.errorText.Render("Last error"),
		m.renderBadges(string(m.appErr.Kind), "recovery"),
		m.style.diffContext.Render(m.appErr.Summary),
		"",
		m.style.section.Render("Recovery"),
	}
	lines = append(lines, styledWrap(m.appErr.Recovery, 82, m.style.diffContext)...)
	lines = append(lines, "", m.style.section.Render("Details"))
	lines = append(lines, styledWrap(m.appErr.Detail, 82, m.style.subtle)...)
	lines = append(lines, "")
	return lines
}

func (m Model) rightLines() []string {
	hunk := m.selectedDiffHunk()
	if hunk == nil {
		if file := m.currentFile(); file != nil && file.IsBinary {
			return []string{file.Path, "", "Binary file. No text hunk preview is available."}
		}
		return []string{"No selected hunk.", "", "Use the left panel to select a file or slice with text hunks."}
	}
	lines := []string{hunk.FilePath, hunk.Header, ""}
	for _, line := range hunk.Lines {
		prefix := " "
		switch line.Type {
		case diff.LineAdded:
			prefix = "+"
		case diff.LineDeleted:
			prefix = "-"
		}
		lines = append(lines, prefix+line.Content)
	}
	return lines
}

func (m *Model) rightStyledLines() []string {
	hunk := m.selectedDiffHunk()
	if hunk == nil {
		if file := m.currentFile(); file != nil && file.IsBinary {
			return []string{
				m.style.emphasis.Render(file.Path),
				"",
				m.callout("Binary file. No text hunk preview is available."),
			}
		}
		return []string{
			m.callout("No selected hunk."),
			"",
			m.style.subtle.Render("Use the left panel to select a file or slice with text hunks."),
		}
	}
	lines := []string{
		m.style.emphasis.Render(hunk.FilePath),
		m.style.diffHeader.Render(hunk.Header),
	}
	if hunkSignal(*hunk) != diff.HunkSignalFocus {
		lines = append(lines, m.renderBadges(string(hunkSignal(*hunk)), hunkQuietReason(*hunk)))
	}
	lines = append(lines, "")
	for _, line := range hunk.Lines {
		lines = append(lines, m.renderDiffLine(hunk.FilePath, line))
	}
	return lines
}

func (m *Model) renderDiffLine(filePath string, line diff.DiffLine) string {
	cacheKey := diffLineCacheKey(filePath, line)
	if cached, ok := m.diffLineCache[cacheKey]; ok {
		return cached
	}

	oldNo := formatLineNumber(line.OldNumber)
	newNo := formatLineNumber(line.NewNumber)
	sign := " "
	style := m.style.diffContext
	switch line.Type {
	case diff.LineAdded:
		sign = "+"
		style = m.style.diffAdded
	case diff.LineDeleted:
		sign = "-"
		style = m.style.diffDeleted
	}
	gutter := m.style.diffGutter.Render(fmt.Sprintf("%4s %4s ", oldNo, newNo))
	code, ok := highlightDiffCode(filePath, line.Content, style)
	if !ok {
		code = style.Render(line.Content)
	}
	body := style.Render(sign+" ") + code
	rendered := gutter + body
	if m.diffLineCache == nil {
		m.diffLineCache = make(map[string]string)
	}
	m.diffLineCache[cacheKey] = rendered
	return rendered
}

func diffLineCacheKey(filePath string, line diff.DiffLine) string {
	return fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%s", filePath, line.Type, line.OldNumber, line.NewNumber, line.Content)
}

func (m Model) selectedDiffHunk() *diff.DiffHunk {
	if m.mode == modeGrouped && m.slices != nil {
		if item := m.currentReviewItem(); item != nil && len(item.HunkRefs) > 0 {
			idx := clamp(m.selectedHunk, 0, len(item.HunkRefs)-1)
			return m.findHunk(item.HunkRefs[idx])
		}
		return nil
	}
	return m.currentRawHunk()
}

func (m Model) findHunk(ref agent.HunkRef) *diff.DiffHunk {
	if m.pr == nil {
		return nil
	}
	for i := range m.pr.Files {
		for j := range m.pr.Files[i].Hunks {
			hunk := &m.pr.Files[i].Hunks[j]
			if hunk.ID == ref.HunkID && hunk.FilePath == ref.FilePath {
				return hunk
			}
			if hunk.ID == ref.HunkID {
				return hunk
			}
		}
	}
	return nil
}

func (m Model) renderFrame(title string, lines []string) string {
	width := max(40, m.width)
	height := max(8, m.height)
	body := []string{
		m.style.headerTitle.Render(title),
		"",
	}
	if m.stage == stageLoading || m.aiBusy {
		body[0] = lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " ", body[0])
	}
	for _, line := range lines {
		body = append(body, m.style.diffContext.Render(line))
	}
	body = append(body, "", m.style.subtle.Render("q quits"))
	panel := m.renderPanel("SliceDiff", fitLines(strings.Join(body, "\n"), min(height-4, 18), min(width-4, 92)), min(width, 96), min(height-2, 20), true)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func (m Model) renderRunnerPicker() string {
	options := []string{"codex", "opencode"}
	lines := []string{"Choose the AI runner SliceDiff should use:", ""}
	for i, option := range options {
		if i == m.selectedSetup {
			lines = append(lines, m.style.diffSelected.Render("> "+option))
		} else {
			lines = append(lines, m.style.diffContext.Render("  "+option))
		}
	}
	lines = append(lines, "", "enter selects | q quits")
	return m.renderFrame("AI runner", lines)
}

func (m Model) runnerLabel() string {
	if m.opts.NoAI {
		return "none"
	}
	if runner := m.selectedRunner(); runner != "" {
		return string(runner)
	}
	return "unset"
}

func (m Model) prTitle() string {
	if m.pr == nil {
		return ""
	}
	return m.pr.Title
}

func (m Model) leftTitle() string {
	focus := focusMark(m.focus == panelLeft)
	if m.mode == modeGrouped && m.slices != nil {
		total := len(m.reviewItems())
		if total == 0 {
			return focus + " Slices 0/0"
		}
		return fmt.Sprintf("%s Slices %d/%d", focus, clamp(m.selectedSlice+1, 1, total), total)
	}
	total := 0
	if m.pr != nil {
		total = len(m.pr.Files)
	}
	if total == 0 {
		return focus + " Files 0/0"
	}
	return fmt.Sprintf("%s Files %d/%d", focus, clamp(m.selectedFile+1, 1, total), total)
}

func (m Model) centerTitle() string {
	focus := focusMark(m.focus == panelCenter)
	total := m.currentHunkCount()
	if total == 0 {
		return focus + " Details hunk 0/0"
	}
	return fmt.Sprintf("%s Details hunk %d/%d", focus, clamp(m.selectedHunk+1, 1, total), total)
}

func (m Model) rightTitle() string {
	focus := focusMark(m.focus == panelRight)
	total := m.currentHunkCount()
	if total == 0 {
		return focus + " Diff 0/0"
	}
	return fmt.Sprintf("%s Diff %d/%d", focus, clamp(m.selectedHunk+1, 1, total), total)
}

func (m Model) currentHunkCount() int {
	if m.mode == modeGrouped && m.slices != nil {
		if item := m.currentReviewItem(); item != nil {
			return len(item.HunkRefs)
		}
		return 0
	}
	if file := m.currentFile(); file != nil {
		return len(file.Hunks)
	}
	return 0
}

func (m Model) currentLeftIndex() int {
	if m.mode == modeGrouped && m.slices != nil {
		return clamp(m.selectedSlice, 0, max(0, len(m.reviewItems())-1))
	}
	if m.pr != nil {
		return clamp(m.selectedFile, 0, max(0, len(m.pr.Files)-1))
	}
	return 0
}

func focusMark(active bool) string {
	if active {
		return ">>"
	}
	return "  "
}

func uniqueFiles(refs []agent.HunkRef) []string {
	seen := map[string]bool{}
	var files []string
	for _, ref := range refs {
		if seen[ref.FilePath] {
			continue
		}
		seen[ref.FilePath] = true
		files = append(files, ref.FilePath)
	}
	if len(files) == 0 {
		return []string{"No files referenced."}
	}
	return files
}

func shouldStack(width, height int) bool {
	return width < 100 || height < 30
}

func horizontalPanelWidths(total int) (int, int, int) {
	return weightedWidths(total, []int{2, 3, 5})
}

func weightedWidths(total int, weights []int) (int, int, int) {
	sum := 0
	for _, w := range weights {
		sum += w
	}
	left := total * weights[0] / sum
	center := total * weights[1] / sum
	right := total - left - center
	return left, center, right
}

func wrapWords(text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len([]rune(current))+1+len([]rune(word)) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	lines = append(lines, current)
	return lines
}

func styledWrap(text string, width int, style lipgloss.Style) []string {
	lines := wrapWords(text, width)
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return lines
}

func limitLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	out := append([]string{}, lines[:limit]...)
	out[limit-1] = ansi.Truncate(out[limit-1], max(1, lipgloss.Width(out[limit-1])-3), "...")
	return out
}

func firstLine(content string) string {
	line, _, _ := strings.Cut(content, "\n")
	return line
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func shortenPathAfterFirstSlash(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(path) <= width {
		return path
	}
	slash := strings.Index(path, "/")
	if slash < 0 {
		return ansi.Truncate(path, width, "...")
	}

	prefix := path[:slash+1] + "..."
	prefixWidth := ansi.StringWidth(prefix)
	if prefixWidth >= width {
		return ansi.Truncate(prefix, width, "")
	}

	remainder := path[slash+1:]
	tailWidth := width - prefixWidth
	removeWidth := max(0, ansi.StringWidth(remainder)-tailWidth)
	return prefix + ansi.TruncateLeft(remainder, removeWidth, "")
}

func fitLines(content string, height, width int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if width > 0 {
			lines[i] = ansi.Truncate(line, width, "...")
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func cropLines(content string, height, width int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if width > 0 {
			lines[i] = ansi.Truncate(line, width, "...")
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderBadges(values ...string) string {
	var badges []string
	for _, value := range values {
		if value == "" {
			continue
		}
		badges = append(badges, m.style.chip.Render(strings.ToUpper(value)))
	}
	return strings.Join(badges, " ")
}

func (m Model) callout(text string) string {
	return m.style.callout.Render(text)
}

func generatedLabel(file *diff.DiffFile) string {
	if file == nil {
		return ""
	}
	if file.IsGenerated {
		return "generated"
	}
	return "source"
}

func hunkQuietReason(hunk diff.DiffHunk) string {
	if hunk.Reason != "" {
		return hunk.Reason
	}
	return "quiet"
}

func formatLineNumber(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
