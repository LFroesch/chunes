package ui

import (
	"github.com/charmbracelet/lipgloss"
)

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmClearQueue
	confirmDeletePlaylist
	confirmDeleteDownload
	confirmDeleteHistory
)

type confirmModel struct {
	active  bool
	message string
	action  confirmAction
}

func (m *confirmModel) show(message string, action confirmAction) {
	m.active = true
	m.message = message
	m.action = action
}

func (m *confirmModel) close() {
	m.active = false
	m.message = ""
	m.action = confirmNone
}

func (m confirmModel) View() string {
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(warnColor).
		Padding(1, 3).
		Align(lipgloss.Center)

	title := lipgloss.NewStyle().
		Foreground(warnColor).
		Bold(true).
		Render("Confirm")

	msg := lipgloss.NewStyle().
		Foreground(textColor).
		Render(m.message)

	hints := lipgloss.NewStyle().
		Foreground(dimColor).
		Render("y confirm  n cancel")

	content := title + "\n\n" + msg + "\n\n" + hints
	return style.Render(content)
}
