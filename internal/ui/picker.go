package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lucas/chunes/internal/playlist"
)

var pickerStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(primaryColor).
	Padding(1, 2).
	Width(40)

type pickerModel struct {
	active    bool
	lists     []playlist.Playlist
	cursor    int
	creating  bool
	nameInput textinput.Model
}

func newPickerModel() pickerModel {
	ti := textinput.New()
	ti.Placeholder = "New playlist name..."
	ti.CharLimit = 100
	ti.Width = 30
	ti.Prompt = " "
	return pickerModel{nameInput: ti}
}

func (m *pickerModel) open() tea.Cmd {
	m.active = true
	m.cursor = 0
	m.creating = false
	return func() tea.Msg {
		pls, _ := playlist.List()
		return pickerListMsg(pls)
	}
}

func (m *pickerModel) close() {
	m.active = false
	m.creating = false
	m.nameInput.Blur()
	m.nameInput.SetValue("")
}

type pickerListMsg []playlist.Playlist

func (m pickerModel) Update(msg tea.Msg) (pickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case pickerListMsg:
		m.lists = msg
		return m, nil

	case tea.KeyMsg:
		if m.creating {
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.nameInput.Value())
				if name != "" {
					m.creating = false
					m.nameInput.Blur()
					m.nameInput.SetValue("")
					// Add the new playlist and select it
					p := &playlist.Playlist{Name: name}
					p.Save()
					m.lists = append(m.lists, *p)
					m.cursor = len(m.lists) - 1
					// Auto-confirm: return the selected playlist name
					return m, nil
				}
			case "esc":
				m.creating = false
				m.nameInput.Blur()
				m.nameInput.SetValue("")
				return m, nil
			}
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.lists)-1 {
				m.cursor++
			}
		case "c":
			m.creating = true
			m.nameInput.Focus()
			return m, textinput.Blink
		case "esc":
			m.close()
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	if !m.active {
		return ""
	}

	var b strings.Builder
	b.WriteString(trackTitleStyle.Render("Save to playlist"))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", 34)))
	b.WriteString("\n")

	if len(m.lists) == 0 && !m.creating {
		b.WriteString(statusStyle.Render("No playlists yet"))
		b.WriteString("\n")
	}

	for i, p := range m.lists {
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("> ")
		}
		line := fmt.Sprintf("%-25s %dt", p.Name, len(p.Tracks))
		if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if m.creating {
		b.WriteString("\n")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")
	hints := []helpBinding{
		{"Enter", "save"},
		{"c", "new"},
		{"Esc", "cancel"},
	}
	b.WriteString(renderHints(hints, 34))

	return pickerStyle.Render(b.String())
}

func (m pickerModel) selectedPlaylist() *playlist.Playlist {
	if m.cursor >= 0 && m.cursor < len(m.lists) {
		p := m.lists[m.cursor]
		return &p
	}
	return nil
}
