package tui

import (
	"fmt"
	"image/color"
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
	view.BackgroundColor = color.RGBA{R: 0x07, G: 0x0B, B: 0x12, A: 0xFF}
	return view
}

func (m Model) renderMain() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	contentHeight := max(1, m.height-lipgloss.Height(header)-1)
	body := m.renderPanels(m.width, contentHeight)
	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return m.style.app.Render(fillBlock(screen, m.height, m.width, m.style.appFill))
}

func (m Model) renderWelcome() string {
	footer := m.renderPickerFooter()
	bodyHeight := max(1, m.height-lipgloss.Height(footer))
	body := m.renderWelcomeBody(m.width, bodyHeight)
	screen := lipgloss.JoinVertical(lipgloss.Left, body, footer)
	return m.style.app.Render(fillBlock(screen, m.height, m.width, m.style.appFill))
}

const welcomeSliceArt = `
  █████████  ████   ███
 ███░░░░░███░░███  ░░░
░███    ░░░  ░███  ████   ██████   ██████
░░█████████  ░███ ░░███  ███░░███ ███░░███
 ░░░░░░░░███ ░███  ░███ ░███ ░░░ ░███████
 ███    ░███ ░███  ░███ ░███  ███░███░░░
░░█████████  █████ █████░░██████ ░░██████
 ░░░░░░░░░  ░░░░░ ░░░░░  ░░░░░░   ░░░░░░
`

const welcomeCakeMarkArt = `
                ░▒▒▓███████
      ░▒▓▓█████▓▒░░    ▒██▒█░
 ███▓░              ░██▒ █ █░
 ██▓████░         ██▓ ▒███ █░
 █▓   ▓██████████▓░▒██████ █░
 █▓           ▓█ ██████▓░▒ █░
 ██           ▒█ ███▓░▒███ █░
 ██           ▒█ █░▒██████▓
  ██          ▒█ ██████▓
   ▒██▓░      ▒██████
      ░▒▓███▓▓████░
`

const welcomeDiffArt = `
██████████    ███     ██████     ██████
░░███░░░░███  ░░░     ███░░███   ███░░███
 ░███   ░░███ ████   ░███ ░░░   ░███ ░░░
 ░███    ░███░░███  ███████    ███████
 ░███    ░███ ░███ ░░░███░    ░░░███░
 ░███    ███  ░███   ░███       ░███
 ██████████   █████  █████      █████
░░░░░░░░░░   ░░░░░  ░░░░░      ░░░░░
`

func (m Model) renderWelcomeCluster(width, maxLogoHeight int) string {
	requested := m.style.chipHot.Render("Requested review")
	manual := m.style.chip.Render("Manual")
	if m.welcomeSection == welcomeManual {
		requested = m.style.chip.Render("Requested review")
		manual = m.style.chipHot.Render("Manual")
	}
	lines := []string{}
	if logo := m.renderWelcomeArt(width, maxLogoHeight); logo != "" {
		lines = append(lines, strings.Split(logo, "\n")...)
		lines = append(lines, "")
	}
	lines = append(lines,
		m.style.panelSection.Render("SliceDiff"),
		m.style.diffContextP.Render("Choose a pull request"),
		lipgloss.JoinHorizontal(lipgloss.Center,
			requested,
			m.style.panelFill.Render(fillCell),
			manual,
		),
		m.renderWelcomeStatus(width),
	)
	for i, line := range lines {
		if lipgloss.Width(line) > width && !strings.Contains(line, "\x1b[") {
			line = ansi.Truncate(line, width, "...")
		}
		lines[i] = centerStyledLine(line, width, m.style.panelFill)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderWelcomeSearchRows(width, pickerWidth int) []string {
	rows := []string{padStyledLine("", width, m.style.panelFill)}
	if m.welcomeSection != welcomeManual || m.manualStep != manualRepos {
		return append(rows, padStyledLine("", width, m.style.panelFill), padStyledLine("", width, m.style.panelFill))
	}
	prompt := "/ " + m.manualQuery
	if m.manualQuery == "" {
		prompt = "/ type a repository name"
	}
	searchWidth := max(1, pickerWidth-4)
	search := m.style.callout.Width(pickerWidth).Render(fitPlainLine(ansi.Truncate(prompt, searchWidth, "..."), searchWidth))
	return append(rows, m.centerWelcomeLine(search, width), padStyledLine("", width, m.style.panelFill))
}

func (m Model) renderWelcomeStatus(width int) string {
	count := m.welcomeCountLabel()
	countWidth := lipgloss.Width(count)
	separator := strings.Repeat(fillCell, 2) + "/" + strings.Repeat(fillCell, 2)
	subtitleWidth := max(1, width-countWidth-ansi.StringWidth(separator))
	subtitle := ansi.Truncate(m.welcomeSubtitle(), subtitleWidth, "...")
	line := lipgloss.JoinHorizontal(lipgloss.Center,
		m.style.panelSection.Render(count),
		m.style.panelSubtitle.Render(separator),
		m.style.panelSubtitle.Render(subtitle),
	)
	return line
}

func (m Model) renderWelcomeArt(width, maxHeight int) string {
	wordmarkLines := horizontalArt(
		trimBlankArtRows(strings.Split(strings.Trim(welcomeSliceArt, "\n"), "\n")),
		trimBlankArtRows(strings.Split(strings.Trim(welcomeCakeMarkArt, "\n"), "\n")),
		trimBlankArtRows(strings.Split(strings.Trim(welcomeDiffArt, "\n"), "\n")),
	)
	if artFits(wordmarkLines, width, maxHeight) {
		return strings.Join(wordmarkLines, "\n")
	}
	return ""
}

func artFits(lines []string, width, maxHeight int) bool {
	artWidth := 0
	for _, line := range lines {
		artWidth = max(artWidth, lipgloss.Width(line))
	}
	if width < artWidth || maxHeight < len(lines) {
		return false
	}
	return true
}

func horizontalArt(parts ...[]string) []string {
	height := 0
	widths := make([]int, len(parts))
	for i, part := range parts {
		height = max(height, len(part))
		for _, line := range part {
			widths[i] = max(widths[i], lipgloss.Width(line))
		}
	}
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		pieces := make([]string, len(parts))
		for i, part := range parts {
			offset := max(0, (height-len(part))/2)
			partRow := row - offset
			line := ""
			if partRow >= 0 && partRow < len(part) {
				line = part[partRow]
			}
			pieces[i] = padRight(line, widths[i])
		}
		lines[row] = strings.Join(pieces, strings.Repeat(fillCell, 3))
	}
	return lines
}

func padRight(line string, width int) string {
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += strings.Repeat(fillCell, gap)
	}
	return line
}

func trimBlankArtRows(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
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
	innerHeight := max(1, height-2)
	bodyHeight := max(0, innerHeight)
	pickerWidth := welcomePickerWidth(bodyWidth)
	lines := make([]string, 0, bodyHeight)
	minListHeight := 6
	if bodyHeight < 18 {
		minListHeight = 3
	}
	maxLogoHeight := max(0, bodyHeight-minListHeight-8)
	cluster := m.renderWelcomeCluster(bodyWidth, maxLogoHeight)
	lines = append(lines, cluster, "")
	lines = append(lines, m.renderWelcomeSearchRows(bodyWidth, pickerWidth)...)
	lines = m.fillWelcomeLines(lines, bodyWidth)
	availableHeight := max(0, bodyHeight-lipgloss.Height(strings.Join(lines, "\n")))
	listHeight := welcomePickerListHeight(availableHeight)
	if m.pickerBusy {
		loading := lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), fillCell, "Loading...")
		loading = centerStyledLine(loading, pickerWidth, m.style.panelFill)
		lines = append(lines, m.centerWelcomeBlock(fillBlock(loading, listHeight, pickerWidth, m.style.panelFill), bodyWidth)...)
	} else if m.pickerErr != "" {
		errBlock := strings.Join([]string{
			m.style.errorTextP.Render("Could not load choices."),
			m.style.diffContextP.Render(ansi.Truncate(m.pickerErr, pickerWidth, "...")),
		}, "\n")
		lines = append(lines, m.centerWelcomeBlock(fillBlock(errBlock, listHeight, pickerWidth, m.style.panelFill), bodyWidth)...)
	} else {
		m.syncPickerList()
		pickerList := m.pickerList
		pickerList.SetSize(pickerWidth, listHeight)
		pickerList.Select(clamp(m.selectedPicker, 0, max(0, m.pickerItemCount()-1)))
		lines = append(lines, m.centerWelcomeBlock(fillBlock(pickerList.View(), listHeight, pickerWidth, m.style.panelFill), bodyWidth)...)
	}
	contentHeight := lipgloss.Height(strings.Join(lines, "\n"))
	topPad := max(0, (bodyHeight-contentHeight)/2)
	if topPad > 0 {
		padded := make([]string, 0, len(lines)+topPad)
		for i := 0; i < topPad; i++ {
			padded = append(padded, padStyledLine("", bodyWidth, m.style.panelFill))
		}
		lines = append(padded, lines...)
	}
	body := fillBlock(strings.Join(lines, "\n"), bodyHeight, bodyWidth, m.style.panelFill)
	return m.renderPanel(m.welcomePanelTitle(), body, width, innerHeight, true)
}

func (m Model) renderPickerFooter() string {
	help := "up/down move | enter selects | tab switches | q quits"
	if m.welcomeSection == welcomeManual {
		help = "type to search | enter selects | esc/backspace goes back | tab switches | q quits"
	}
	statusText := m.style.status.Render(truncate(m.status, max(1, m.width/3)))
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		statusText,
		m.style.footerSubtle.Render(" | "),
		m.style.footerSubtle.Render(truncate(help, max(1, m.width-lipgloss.Width(statusText)-3))),
	)
	return m.style.footer.Render(fillBlock(line, 1, m.width, m.style.footerFill))
}

func (m Model) welcomePanelTitle() string {
	return ""
}

func (m Model) welcomeCountLabel() string {
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
		m.style.headerFill.Render(strings.Repeat(fillCell, 2)),
		m.style.emphasis.Render(truncate(name, max(10, m.width/3))),
		m.style.headerFill.Render(strings.Repeat(fillCell, 2)),
		m.style.chipHot.Render(mode),
		m.style.headerFill.Render(fillCell),
		m.style.chipCool.Render("runner "+m.runnerLabel()),
		m.style.headerFill.Render(fillCell),
		m.style.chip.Render(counts),
	)
	line1 = ansi.Truncate(line1, m.width, "...")
	line2 := m.style.headerMeta.Render(truncate(meta, max(1, m.width-2)))
	line2 = ansi.Truncate(line2, m.width, "...")
	return m.style.header.Render(fillBlock(lipgloss.JoinVertical(lipgloss.Left, line1, line2), 2, m.width, m.style.headerFill))
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
		m.style.footerSubtle.Render(" | "),
		helpText,
	)
	return m.style.footer.Render(fillBlock(line, 1, m.width, m.style.footerFill))
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
	body := fillBlock(listModel.View(), bodyHeight, bodyWidth, m.style.panelFill)
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
	body := fillBlock(viewportModel.View(), bodyHeight, bodyWidth, m.style.panelFill)
	return m.renderPanel(title, body, width, innerHeight, focused)
}

func (m Model) renderCenterPanel(title string, width, innerHeight int, focused bool) string {
	bodyHeight := max(0, innerHeight-1)
	bodyWidth := max(1, width-2)
	overview := cropLines(strings.Join(m.centerOverviewStyledLines(bodyWidth), "\n"), m.centerOverviewHeight(bodyHeight), bodyWidth, m.style.panelFill)
	overviewHeight := lipgloss.Height(overview)
	remainingHeight := max(0, bodyHeight-overviewHeight)

	viewportModel := m.centerViewport
	scrollingLines, selectedStart, selectedEnd := m.centerScrollStyledLineRange(bodyWidth)
	viewportModel.SetWidth(bodyWidth)
	viewportModel.SetHeight(remainingHeight)
	viewportModel.SetContentLines(scrollingLines)
	if selectedStart >= 0 {
		ensureViewportRangeVisible(&viewportModel, selectedStart, selectedEnd)
	}
	scrolling := fillBlock(viewportModel.View(), remainingHeight, bodyWidth, m.style.panelFill)
	body := overview
	if remainingHeight > 0 {
		body = lipgloss.JoinVertical(lipgloss.Left, overview, scrolling)
	}
	return m.renderPanel(title, fillBlock(body, bodyHeight, bodyWidth, m.style.panelFill), width, innerHeight, focused)
}

func (m Model) centerOverviewHeight(bodyHeight int) int {
	if m.mode == modeGrouped && m.slices != nil {
		return min(bodyHeight, 12)
	}
	return min(bodyHeight, 8)
}

func (m Model) renderPanel(title, body string, width, innerHeight int, focused bool) string {
	contentWidth := max(1, width-2)
	contentHeight := max(0, innerHeight)
	content := fillBlock(body, contentHeight, contentWidth, m.style.panelFill)
	if title != "" {
		titleLine := padStyledLine(m.style.panelTitle.Render(ansi.Truncate(title, contentWidth, "...")), contentWidth, m.style.panelFill)
		content = lipgloss.JoinVertical(lipgloss.Left, titleLine, fillBlock(body, max(0, innerHeight-1), contentWidth, m.style.panelFill))
	}
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
			m.style.panelEmphasis.Render(item.Title),
			m.renderBadges(item.Category),
			"",
			m.style.panelSection.Render("Summary"),
		)
		lines = append(lines, styledWrap(item.Summary, max(24, width), m.style.detailTextP)...)
		return lines
	}

	file := m.currentFile()
	if file == nil {
		return append(prefix, m.callout("No selected file."))
	}
	if file.IsBinary {
		return append(prefix,
			m.style.panelEmphasis.Render(file.Path),
			m.renderBadges(file.Status, "binary"),
			"",
			m.callout("Binary files do not include line hunks in the unified diff."),
		)
	}
	return append(prefix,
		m.style.panelEmphasis.Render(file.Path),
		m.renderBadges(file.Status, generatedLabel(file)),
	)
}

func (m Model) centerScrollStyledLines(width int) ([]string, int) {
	lines, selectedStart, _ := m.centerScrollStyledLineRange(width)
	return lines, selectedStart
}

func (m Model) centerScrollStyledLineRange(width int) ([]string, int, int) {
	if m.mode == modeGrouped && m.slices != nil {
		item := m.currentReviewItem()
		if item == nil {
			return []string{m.callout("No selected slice.")}, -1, -1
		}
		lines := []string{"", padStyledLine(m.style.panelSection.Render("Reading order"), width, m.style.panelFill)}
		selectedStart := -1
		selectedEnd := -1
		for i, step := range item.ReadingSteps {
			selected := i == m.selectedHunk
			bodyStart := len(lines)
			lines = append(lines, m.renderReadingStepBody(step.Body, width, selected)...)
			if selected {
				selectedStart = bodyStart
				selectedEnd = len(lines)
			}
			lines = append(lines, m.renderReadingStepRef(step.HunkRef, width, selected), "")
		}
		return lines, selectedStart, selectedEnd
	}

	file := m.currentFile()
	if file == nil {
		return []string{m.callout("No selected file.")}, -1, -1
	}
	if file.IsBinary {
		return []string{m.callout("Binary files do not include line hunks in the unified diff.")}, -1, -1
	}
	lines := []string{padStyledLine(m.style.panelSection.Render("Hunks"), width, m.style.panelFill)}
	if len(file.Hunks) == 0 {
		lines = append(lines, m.callout("No text hunks available for this file."))
		return lines, -1, -1
	}
	selectedLine := -1
	for i, hunk := range file.Hunks {
		line := fmt.Sprintf("  %s%s%s%s", hunk.ID, fillCell, hunk.Header, fillCell)
		if hunkSignal(hunk) != diff.HunkSignalFocus {
			line += strings.Repeat(fillCell, 2) + string(hunkSignal(hunk)) + ":" + hunkQuietReason(hunk)
		}
		if i == m.selectedHunk {
			line = m.style.diffSelected.Render("> " + hunk.ID + fillCell + hunk.Header)
			if hunkSignal(hunk) != diff.HunkSignalFocus {
				line += fillCell + m.style.diffSelected.Render(string(hunkSignal(hunk))+":"+hunkQuietReason(hunk))
			}
			line = padStyledLine(line, width, m.style.diffSelectedF)
			selectedLine = len(lines)
		} else {
			line = padStyledLine(m.style.diffContextP.Render(line), width, m.style.panelFill)
		}
		lines = append(lines, line)
	}
	return lines, selectedLine, selectedLine
}

func (m Model) renderReadingStepBody(body string, width int, selected bool) []string {
	prefix := m.readingStepPrefix(selected)
	lines := wrapWords(body, max(1, width-lipgloss.Width(prefix)))
	for i, line := range lines {
		lines[i] = padStyledLine(prefix+m.style.detailTextP.Render(line), width, m.style.panelFill)
	}
	return lines
}

func (m Model) renderReadingStepRef(ref agent.HunkRef, width int, selected bool) string {
	prefix := m.readingStepRefPrefix(selected)
	hunk := m.style.detailMetaP.Render(ref.HunkID)
	separator := strings.Repeat(fillCell, 2)
	if selected {
		hunk = m.style.detailHunk.Render(ref.HunkID)
	}
	stemWidth := lipgloss.Width(prefix) + lipgloss.Width(hunk) + ansi.StringWidth(separator)
	path := shortenPathAfterFirstSlash(ref.FilePath, max(1, width-stemWidth))
	return padStyledLine(prefix+hunk+m.style.detailMetaP.Render(separator)+m.style.detailMetaP.Render(path), width, m.style.panelFill)
}

func (m Model) readingStepPrefix(selected bool) string {
	if selected {
		return m.style.detailRail.Render(fillCell) + m.style.panelFill.Render(fillCell)
	}
	return m.style.panelFill.Render(strings.Repeat(fillCell, 2))
}

func (m Model) readingStepRefPrefix(selected bool) string {
	if selected {
		return m.style.detailRail.Render(fillCell) + m.style.panelFill.Render(strings.Repeat(fillCell, 3))
	}
	return m.style.panelFill.Render(strings.Repeat(fillCell, 4))
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
		m.style.errorTextP.Render("Last error"),
		m.renderBadges(string(m.appErr.Kind), "recovery"),
		m.style.diffContextP.Render(m.appErr.Summary),
		"",
		m.style.panelSection.Render("Recovery"),
	}
	lines = append(lines, styledWrap(m.appErr.Recovery, 82, m.style.diffContextP)...)
	lines = append(lines, "", m.style.panelSection.Render("Details"))
	lines = append(lines, styledWrap(m.appErr.Detail, 82, m.style.detailMetaP)...)
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
	width := max(1, m.rightViewport.Width())
	hunk := m.selectedDiffHunk()
	if hunk == nil {
		if file := m.currentFile(); file != nil && file.IsBinary {
			return []string{
				padStyledLine(m.style.panelEmphasis.Render(shortenPathAfterFirstSlash(file.Path, width)), width, m.style.panelFill),
				padStyledLine("", width, m.style.panelFill),
				padStyledLine(m.callout("Binary file. No text hunk preview is available."), width, m.style.panelFill),
			}
		}
		return []string{
			padStyledLine(m.callout("No selected hunk."), width, m.style.panelFill),
			padStyledLine("", width, m.style.panelFill),
			padStyledLine(m.style.detailMetaP.Render("Use the left panel to select a file or slice with text hunks."), width, m.style.panelFill),
		}
	}
	lines := []string{
		padStyledLine(m.style.panelEmphasis.Render(shortenPathAfterFirstSlash(hunk.FilePath, width)), width, m.style.panelFill),
		padStyledLine(m.style.diffHeader.Render(ansi.Truncate(hunk.Header, width, "...")), width, m.style.diffHeader),
	}
	if hunkSignal(*hunk) != diff.HunkSignalFocus {
		lines = append(lines, padStyledLine(m.renderBadges(string(hunkSignal(*hunk)), hunkQuietReason(*hunk)), width, m.style.panelFill))
	}
	lines = append(lines, padStyledLine("", width, m.style.panelFill))
	for _, line := range hunk.Lines {
		lines = append(lines, m.renderDiffLine(hunk.FilePath, line, width))
	}
	return lines
}

func (m *Model) renderDiffLine(filePath string, line diff.DiffLine, width int) string {
	cacheKey := diffLineCacheKey(filePath, line, width)
	if cached, ok := m.diffLineCache[cacheKey]; ok {
		return cached
	}

	oldNo := formatLineNumber(line.OldNumber)
	newNo := formatLineNumber(line.NewNumber)
	sign := " "
	style := m.style.diffContextP
	fill := m.style.panelFill
	switch line.Type {
	case diff.LineAdded:
		sign = "+"
		style = m.style.diffAdded
		fill = m.style.diffAdded
	case diff.LineDeleted:
		sign = "-"
		style = m.style.diffDeleted
		fill = m.style.diffDeleted
	}
	gutterText := fmt.Sprintf("%4s %4s ", oldNo, newNo)
	gutter := m.style.diffGutterP.Render(gutterText)
	code, ok := highlightDiffCode(filePath, line.Content, style)
	if !ok {
		code = style.Render(line.Content)
	}
	body := style.Render(sign+" ") + code
	if width > 0 {
		bodyWidth := max(0, width-lipgloss.Width(gutterText))
		body = padStyledLine(body, bodyWidth, fill)
	}
	rendered := gutter + body
	if m.diffLineCache == nil {
		m.diffLineCache = make(map[string]string)
	}
	m.diffLineCache[cacheKey] = rendered
	return rendered
}

func diffLineCacheKey(filePath string, line diff.DiffLine, width int) string {
	return fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d\x00%s", filePath, line.Type, line.OldNumber, line.NewNumber, width, line.Content)
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
		m.style.panelSection.Render(title),
		"",
	}
	if m.stage == stageLoading || m.aiBusy {
		body[0] = lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), fillCell, body[0])
	}
	for _, line := range lines {
		body = append(body, m.style.diffContextP.Render(line))
	}
	body = append(body, "", m.style.detailMetaP.Render("q quits"))
	panelBody := fillBlock(strings.Join(body, "\n"), min(height-4, 18), min(width-4, 92), m.style.panelFill)
	panel := m.renderPanel("SliceDiff", panelBody, min(width, 96), min(height-2, 20), true)
	return placeStyledBlock(panel, height, width, m.style.appFill)
}

func (m Model) renderRunnerPicker() string {
	options := []string{"codex", "opencode"}
	lines := []string{"Choose the AI runner SliceDiff should use:", ""}
	for i, option := range options {
		if i == m.selectedSetup {
			lines = append(lines, m.style.diffSelected.Render("> "+option))
		} else {
			lines = append(lines, m.style.diffContextP.Render(strings.Repeat(fillCell, 2)+option))
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
	return fillBlock(content, height, width, lipgloss.NewStyle())
}

func fillBlock(content string, height, width int, fill lipgloss.Style) string {
	if height <= 0 || width <= 0 {
		return ""
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, padStyledLine(line, width, fill))
	}
	for len(out) < height {
		out = append(out, padStyledLine("", width, fill))
	}
	return strings.Join(out, "\n")
}

func padStyledLine(line string, width int, fill lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "...")
	}
	line = replaceTrailingSpaces(line, fill)
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += fill.Render(strings.Repeat(fillCell, gap))
	}
	return line
}

func centerStyledLine(line string, width int, fill lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "...")
	}
	line = replaceTrailingSpaces(line, fill)
	gap := width - lipgloss.Width(line)
	if gap <= 0 {
		return line
	}
	left := gap / 2
	right := gap - left
	return fill.Render(strings.Repeat(fillCell, left)) + line + fill.Render(strings.Repeat(fillCell, right))
}

func placeStyledBlock(content string, height, width int, fill lipgloss.Style) string {
	if height <= 0 || width <= 0 {
		return ""
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	topPad := max(0, (height-len(lines))/2)
	out := make([]string, 0, height)
	for len(out) < topPad {
		out = append(out, padStyledLine("", width, fill))
	}
	for _, line := range lines {
		out = append(out, centerStyledLine(line, width, fill))
	}
	for len(out) < height {
		out = append(out, padStyledLine("", width, fill))
	}
	return strings.Join(out, "\n")
}

// fillCell is intentionally a non-breaking space. Bubble Tea v2's renderer can
// optimize trailing ASCII spaces into erase operations, which leaves some
// terminals showing the default background at the right edge.
const fillCell = "\u00a0"

func replaceTrailingSpaces(line string, fill lipgloss.Style) string {
	gap := 0
	for len(line) > gap && line[len(line)-1-gap] == ' ' {
		gap++
	}
	if gap == 0 {
		return line
	}
	return line[:len(line)-gap] + fill.Render(strings.Repeat(fillCell, gap))
}

func (m Model) fillWelcomeLines(lines []string, width int) []string {
	for i, line := range lines {
		lines[i] = padStyledLine(line, width, m.style.panelFill)
	}
	return lines
}

func (m Model) centerWelcomeBlock(content string, width int) []string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = m.centerWelcomeLine(line, width)
	}
	return lines
}

func (m Model) centerWelcomeLine(line string, width int) string {
	return centerStyledLine(line, width, m.style.panelFill)
}

func welcomePickerWidth(width int) int {
	if width <= 0 {
		return 0
	}
	if width < 68 {
		return max(1, width-4)
	}
	return min(width-8, 76)
}

func welcomePickerListHeight(availableHeight int) int {
	if availableHeight <= 0 {
		return 0
	}
	return min(availableHeight, 12)
}

func fillLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "...")
	}
	line = replaceTrailingSpacesPlain(line)
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += strings.Repeat(fillCell, gap)
	}
	return line
}

func fitPlainLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	line = ansi.Truncate(line, width, "...")
	line = replaceTrailingSpacesPlain(line)
	if gap := width - ansi.StringWidth(line); gap > 0 {
		line += strings.Repeat(fillCell, gap)
	}
	return line
}

func replaceTrailingSpacesPlain(line string) string {
	gap := 0
	for len(line) > gap && line[len(line)-1-gap] == ' ' {
		gap++
	}
	if gap == 0 {
		return line
	}
	return line[:len(line)-gap] + strings.Repeat(fillCell, gap)
}

func cropLines(content string, height, width int, fill lipgloss.Style) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if width > 0 {
			if lipgloss.Width(line) > width {
				lines[i] = ansi.Truncate(line, width, "...")
			}
		}
		lines[i] = padStyledLine(lines[i], width, fill)
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
	return strings.Join(badges, m.style.panelFill.Render(fillCell))
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
