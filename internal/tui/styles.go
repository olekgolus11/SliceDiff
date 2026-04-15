package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type styles struct {
	app          lipgloss.Style
	header       lipgloss.Style
	headerTitle  lipgloss.Style
	headerMeta   lipgloss.Style
	chip         lipgloss.Style
	chipHot      lipgloss.Style
	chipCool     lipgloss.Style
	panelTitle   lipgloss.Style
	panel        lipgloss.Style
	panelFocused lipgloss.Style
	footer       lipgloss.Style
	status       lipgloss.Style
	subtle       lipgloss.Style
	emphasis     lipgloss.Style
	section      lipgloss.Style
	callout      lipgloss.Style
	errorText    lipgloss.Style
	success      lipgloss.Style
	warning      lipgloss.Style
	navTitle     lipgloss.Style
	navDesc      lipgloss.Style
	navSelected  lipgloss.Style
	navSelectedD lipgloss.Style
	diffGutter   lipgloss.Style
	diffHeader   lipgloss.Style
	diffAdded    lipgloss.Style
	diffDeleted  lipgloss.Style
	diffContext  lipgloss.Style
	diffSelected lipgloss.Style
	detailText   lipgloss.Style
	detailMeta   lipgloss.Style
	detailRail   lipgloss.Style
	detailHunk   lipgloss.Style
}

func defaultStyles() styles {
	ink := lipgloss.Color("#070B12")
	panel := lipgloss.Color("#0D1726")
	panelEdge := lipgloss.Color("#21324B")
	ember := lipgloss.Color("#FF7A1A")
	cyan := lipgloss.Color("#35D5FF")
	amber := lipgloss.Color("#FFC857")
	muted := lipgloss.Color("#7F8EA3")
	text := lipgloss.Color("#DCE8F5")
	green := lipgloss.Color("#7CFFB2")
	red := lipgloss.Color("#FF6B7A")

	return styles{
		app: lipgloss.NewStyle().
			Foreground(text).
			Background(ink),
		header: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#09111E")),
		headerTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ember),
		headerMeta: lipgloss.NewStyle().
			Foreground(muted),
		chip: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#16253A")).
			Padding(0, 1),
		chipHot: lipgloss.NewStyle().
			Bold(true).
			Foreground(ink).
			Background(ember).
			Padding(0, 1),
		chipCool: lipgloss.NewStyle().
			Bold(true).
			Foreground(ink).
			Background(cyan).
			Padding(0, 1),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(amber),
		panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(panelEdge).
			Background(panel).
			Foreground(text),
		panelFocused: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(ember).
			Background(panel).
			Foreground(text),
		footer: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#111A2A")),
		status: lipgloss.NewStyle().
			Bold(true).
			Foreground(amber),
		subtle: lipgloss.NewStyle().
			Foreground(muted),
		emphasis: lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan),
		section: lipgloss.NewStyle().
			Bold(true).
			Foreground(amber),
		callout: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#16253A")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(cyan).
			Padding(0, 1),
		errorText: lipgloss.NewStyle().
			Bold(true).
			Foreground(red),
		success: lipgloss.NewStyle().
			Bold(true).
			Foreground(green),
		warning: lipgloss.NewStyle().
			Bold(true).
			Foreground(amber),
		navTitle: lipgloss.NewStyle().
			Foreground(text),
		navDesc: lipgloss.NewStyle().
			Foreground(muted),
		navSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(ink).
			Background(ember),
		navSelectedD: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFE3C2")).
			Background(lipgloss.Color("#3B1F0B")),
		diffGutter: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#54657A")),
		diffHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(amber).
			Background(lipgloss.Color("#1F2635")),
		diffAdded: lipgloss.NewStyle().
			Foreground(green).
			Background(lipgloss.Color("#0C2A1B")),
		diffDeleted: lipgloss.NewStyle().
			Foreground(red).
			Background(lipgloss.Color("#2A1017")),
		diffContext: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAB8C8")),
		diffSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(ink).
			Background(cyan),
		detailText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F4F8FC")),
		detailMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B8C7D8")),
		detailRail: lipgloss.NewStyle().
			Background(cyan),
		detailHunk: lipgloss.NewStyle().
			Bold(true).
			Foreground(ink).
			Background(cyan),
	}
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "move up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "move down")),
		Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("left/h", "focus left")),
		Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("right/l", "focus right")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
		PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		Home:     key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		End:      key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		View:     key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "toggle view")),
		Focus:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "focus/all")),
		Regen:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "regenerate")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more help")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "drill in")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func newNavigationList(style styles) list.Model {
	delegate := navigationDelegate{style: style}

	model := list.New(nil, delegate, 0, 0)
	model.SetShowTitle(false)
	model.SetShowFilter(false)
	model.SetFilteringEnabled(false)
	model.SetShowStatusBar(false)
	model.SetShowPagination(false)
	model.SetShowHelp(false)
	model.DisableQuitKeybindings()
	return model
}

type navigationDelegate struct {
	style styles
}

type titledListItem interface {
	Title() string
	Description() string
}

func (d navigationDelegate) Height() int {
	return 2
}

func (d navigationDelegate) Spacing() int {
	return 0
}

func (d navigationDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (d navigationDelegate) Render(w io.Writer, model list.Model, index int, item list.Item) {
	nav, ok := item.(titledListItem)
	if !ok {
		return
	}

	width := max(1, model.Width())
	selected := index == model.Index()
	prefix := "  "
	titleStyle := d.style.navTitle
	descStyle := d.style.navDesc
	if selected {
		prefix = "> "
		titleStyle = d.style.navSelected
		descStyle = d.style.navSelectedD
	}

	titleWidth := max(1, width-lipgloss.Width(prefix))
	title := prefix + ansi.Truncate(nav.Title(), titleWidth, "...")
	desc := "  " + ansi.Truncate(nav.Description(), max(1, width-2), "...")
	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(title), descStyle.Render(desc))
}

func newViewport(style styles) viewport.Model {
	model := viewport.New()
	model.MouseWheelEnabled = true
	model.SoftWrap = false
	model.FillHeight = true
	model.Style = lipgloss.NewStyle().Background(lipgloss.Color("#0D1726"))
	model.HighlightStyle = style.diffSelected
	model.SelectedHighlightStyle = style.diffSelected
	return model
}

func newHelp(style styles) help.Model {
	model := help.New()
	model.ShortSeparator = " | "
	model.FullSeparator = "   "
	model.Styles.ShortKey = style.chipCool
	model.Styles.ShortDesc = style.subtle
	model.Styles.ShortSeparator = style.subtle
	model.Styles.FullKey = style.chipCool
	model.Styles.FullDesc = style.subtle
	model.Styles.FullSeparator = style.subtle
	model.Styles.Ellipsis = style.subtle
	return model
}

func newSpinner(style styles) spinner.Model {
	model := spinner.New(spinner.WithSpinner(spinner.Line))
	model.Style = style.chipHot
	return model
}

func panelStyle(style styles, focused bool, width int) lipgloss.Style {
	panel := style.panel
	if focused {
		panel = style.panelFocused
	}
	if width > 0 {
		panel = panel.Width(width)
	}
	return panel
}
