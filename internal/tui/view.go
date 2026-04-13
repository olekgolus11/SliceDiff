package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/olekgolus11/SliceDiff/internal/agent"
	"github.com/olekgolus11/SliceDiff/internal/diff"
)

func (m Model) View() string {
	if m.width == 0 {
		return "SliceDiff loading..."
	}
	switch m.stage {
	case stageLoading:
		return m.renderFrame("Loading pull request...", []string{"Validating gh auth status", "Fetching PR metadata and diff"})
	case stageConsent:
		return m.renderFrame("AI consent", []string{
			"SliceDiff can send PR metadata and structured diff hunks to your selected agent runner.",
			"Press y to allow AI grouping, or n to use raw diff navigation only.",
		})
	case stageRunner:
		return m.renderRunnerPicker()
	case stageFatal:
		return m.renderFrame("Could not start SliceDiff", append(m.errorLines(), "Press q to quit."))
	case stageReady:
		return m.renderMain()
	default:
		return ""
	}
}

func (m Model) renderMain() string {
	title := m.renderTitle()
	status := m.renderStatus()
	contentHeight := max(1, m.height-lipgloss.Height(title)-lipgloss.Height(status))
	body := m.renderPanels(m.width, contentHeight)
	return lipgloss.JoinVertical(lipgloss.Left, title, body, status)
}

func (m Model) renderTitle() string {
	name := "SliceDiff"
	if m.pr != nil {
		name = fmt.Sprintf("SliceDiff - %s/%s#%d", m.pr.Owner, m.pr.Repo, m.pr.Number)
	}
	mode := "raw"
	if m.mode == modeGrouped {
		mode = "grouped"
	}
	line1 := truncate(name, m.width)
	line2 := truncate(fmt.Sprintf("mode=%s runner=%s %s", mode, m.runnerLabel(), m.prTitle()), m.width)
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(line1),
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(line2),
	)
}

func (m Model) renderStatus() string {
	help := "tab focus | j/k move | pg scroll | home/end | v view | r regen | ? help | q quit"
	if m.showHelp {
		help = "Grouped: left selects slices, center/right select hunks. Raw: left selects files, center/right select hunks."
	}
	text := truncate(m.status+" | "+help, m.width)
	if m.appErr != nil {
		text = truncate(m.appErr.Summary+" | "+help, m.width)
	}
	return statusStyle(m.width).Render(text)
}

func (m Model) renderPanels(width, height int) string {
	if shouldStack(width, height) {
		return m.renderStacked(width, height)
	}
	leftW, centerW, rightW := weightedWidths(width, []int{1, 2, 2})
	innerHeight := max(1, height-2)
	left := m.renderPanel(m.leftTitle(), m.leftLines(), leftW, innerHeight, m.focus == panelLeft, m.leftScroll)
	center := m.renderPanel(m.centerTitle(), m.centerLines(), centerW, innerHeight, m.focus == panelCenter, m.centerScroll)
	right := m.renderPanel(m.rightTitle(), m.rightLines(), rightW, innerHeight, m.focus == panelRight, m.rightScroll)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (m Model) renderStacked(width, height int) string {
	total := max(9, height)
	top := total / 3
	mid := total / 3
	bottom := total - top - mid
	left := m.renderPanel(m.leftTitle(), m.leftLines(), width, max(1, top-2), m.focus == panelLeft, m.leftScroll)
	center := m.renderPanel(m.centerTitle(), m.centerLines(), width, max(1, mid-2), m.focus == panelCenter, m.centerScroll)
	right := m.renderPanel(m.rightTitle(), m.rightLines(), width, max(1, bottom-2), m.focus == panelRight, m.rightScroll)
	return lipgloss.JoinVertical(lipgloss.Left, left, center, right)
}

func (m Model) renderPanel(title string, lines []string, width, innerHeight int, focused bool, scroll int) string {
	innerWidth := max(1, width-4)
	out := make([]string, 0, innerHeight)
	out = append(out, truncate(title, innerWidth))
	visible := max(0, innerHeight-1)
	scroll = clamp(scroll, 0, max(0, len(lines)-visible))
	for _, line := range lines[scroll:] {
		if len(out) >= innerHeight {
			break
		}
		out = append(out, truncate(line, innerWidth))
	}
	for len(out) < innerHeight {
		out = append(out, "")
	}
	return panelStyle(focused, width).Render(strings.Join(out, "\n"))
}

func (m Model) leftLines() []string {
	if m.mode == modeGrouped && m.slices != nil {
		items := m.reviewItems()
		if len(items) == 0 {
			return []string{"No semantic slices returned.", "Press v for raw diff view.", "Press r to regenerate slices."}
		}
		lines := make([]string, 0, len(items))
		for i, item := range items {
			label := fmt.Sprintf("%d. %s", i+1, item.Title)
			if item.IsUnassigned {
				label += fmt.Sprintf(" (%d)", len(item.HunkRefs))
			}
			if i == m.selectedSlice {
				label = "> " + label
			} else {
				label = "  " + label
			}
			lines = append(lines, label)
		}
		return lines
	}
	if m.pr == nil || len(m.pr.Files) == 0 {
		return []string{"No files changed in this PR."}
	}
	lines := make([]string, 0, len(m.pr.Files))
	for i, file := range m.pr.Files {
		label := fmt.Sprintf("%s %s (%d)", file.Status, file.Path, len(file.Hunks))
		if i == m.selectedFile {
			label = "> " + label
		} else {
			label = "  " + label
		}
		lines = append(lines, label)
	}
	return lines
}

func (m Model) centerLines() []string {
	prefix := m.errorLines()
	if m.mode == modeGrouped && m.slices != nil {
		item := m.currentReviewItem()
		if item == nil {
			return append(prefix, "No selected slice.")
		}
		lines := append(prefix, []string{
			"Title: " + item.Title,
			"Category: " + item.Category,
			"Confidence: " + item.Confidence,
			"",
			"Summary:",
		}...)
		lines = append(lines, wrapWords(item.Summary, 80)...)
		lines = append(lines, "", "Rationale:")
		lines = append(lines, wrapWords(item.Rationale, 80)...)
		lines = append(lines, "", "Files:")
		for _, file := range uniqueFiles(item.HunkRefs) {
			lines = append(lines, "  "+file)
		}
		lines = append(lines, "", "Hunks:")
		for i, ref := range item.HunkRefs {
			prefix := "  "
			if i == m.selectedHunk {
				prefix = "> "
			}
			lines = append(lines, prefix+ref.HunkID+" "+ref.FilePath)
		}
		return lines
	}
	file := m.currentFile()
	if file == nil {
		return append(prefix, "No selected file.")
	}
	if file.IsBinary {
		return append(prefix, []string{
			"Path: " + file.Path,
			"Status: " + file.Status,
			"Binary: true",
			"",
			"Binary files do not include line hunks in the unified diff.",
		}...)
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
		return lines
	}
	for i, hunk := range file.Hunks {
		prefix := "  "
		if i == m.selectedHunk {
			prefix = "> "
		}
		lines = append(lines, prefix+hunk.ID+" "+hunk.Header)
	}
	return lines
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

func (m Model) rightLines() []string {
	var hunk *diff.DiffHunk
	if m.mode == modeGrouped && m.slices != nil {
		if item := m.currentReviewItem(); item != nil && len(item.HunkRefs) > 0 {
			idx := clamp(m.selectedHunk, 0, len(item.HunkRefs)-1)
			hunk = m.findHunk(item.HunkRefs[idx])
		}
	} else {
		hunk = m.currentRawHunk()
	}
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

func (m Model) findHunk(ref agent.HunkRef) *diff.DiffHunk {
	if m.pr == nil {
		return nil
	}
	for _, file := range m.pr.Files {
		for _, hunk := range file.Hunks {
			if hunk.ID == ref.HunkID {
				return &hunk
			}
		}
	}
	return nil
}

func (m Model) renderFrame(title string, lines []string) string {
	width := max(40, m.width)
	height := max(8, m.height)
	content := append([]string{title, ""}, lines...)
	panel := m.renderPanel("SliceDiff", content, min(width, 100), min(height-2, 20), true, 0)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func (m Model) renderRunnerPicker() string {
	options := []string{"codex", "opencode"}
	lines := []string{"Choose the AI runner SliceDiff should use:", ""}
	for i, option := range options {
		prefix := "  "
		if i == m.selectedSetup {
			prefix = "> "
		}
		lines = append(lines, prefix+option)
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
	if m.mode == modeGrouped && m.slices != nil {
		total := 0
		if item := m.currentReviewItem(); item != nil {
			total = len(item.HunkRefs)
		}
		if total == 0 {
			return focus + " Details hunk 0/0"
		}
		return fmt.Sprintf("%s Details hunk %d/%d", focus, clamp(m.selectedHunk+1, 1, total), total)
	}
	total := 0
	if file := m.currentFile(); file != nil {
		total = len(file.Hunks)
	}
	if total == 0 {
		return focus + " Details hunk 0/0"
	}
	return fmt.Sprintf("%s Details hunk %d/%d", focus, clamp(m.selectedHunk+1, 1, total), total)
}

func (m Model) rightTitle() string {
	focus := focusMark(m.focus == panelRight)
	if m.mode == modeGrouped && m.slices != nil {
		if item := m.currentReviewItem(); item != nil && len(item.HunkRefs) > 0 {
			return fmt.Sprintf("%s Hunk %d/%d", focus, clamp(m.selectedHunk+1, 1, len(item.HunkRefs)), len(item.HunkRefs))
		}
		return focus + " Hunk 0/0"
	}
	if file := m.currentFile(); file != nil && len(file.Hunks) > 0 {
		return fmt.Sprintf("%s Hunk %d/%d", focus, clamp(m.selectedHunk+1, 1, len(file.Hunks)), len(file.Hunks))
	}
	return focus + " Hunk 0/0"
}

func focusMark(active bool) string {
	if active {
		return "*"
	}
	return " "
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

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return string(runes[:1])
	}
	return string(runes[:maxLen-3]) + "..."
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
