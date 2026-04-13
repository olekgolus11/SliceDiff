package tui

import "github.com/charmbracelet/lipgloss"

func defaultStyles() styles {
	return styles{}
}

func panelStyle(focused bool, width int) lipgloss.Style {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	if focused {
		style = style.BorderForeground(lipgloss.Color("39"))
	} else {
		style = style.BorderForeground(lipgloss.Color("240"))
	}
	if width > 2 {
		style = style.Width(width - 2)
	}
	return style
}

func statusStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("238")).Width(width)
}
