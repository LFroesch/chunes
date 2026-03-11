package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/playlist"
)

type playlistModel struct {
	playlists   []playlist.Playlist
	cursor      int
	scroll      int
	trackCursor int
	trackScroll int
	viewing     bool
	creating    bool
	renaming    bool
	nameInput   textinput.Model
	err         error
}

type playlistsLoadedMsg struct {
	playlists []playlist.Playlist
	err       error
}

func newPlaylistModel() playlistModel {
	ti := textinput.New()
	ti.Placeholder = "Playlist name..."
	ti.CharLimit = 100
	ti.Width = 40
	ti.PromptStyle = searchPromptStyle
	ti.Prompt = " 📁 "
	return playlistModel{nameInput: ti}
}

func loadPlaylists() tea.Cmd {
	return func() tea.Msg {
		pls, err := playlist.List()
		return playlistsLoadedMsg{playlists: pls, err: err}
	}
}

func (m playlistModel) Update(msg tea.Msg) (playlistModel, tea.Cmd) {
	switch msg := msg.(type) {
	case playlistsLoadedMsg:
		m.playlists = msg.playlists
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		if m.creating {
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.nameInput.Value())
				if name != "" {
					p := &playlist.Playlist{Name: name}
					p.Save()
					m.creating = false
					m.nameInput.Blur()
					m.nameInput.SetValue("")
					return m, loadPlaylists()
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

		if m.renaming {
			switch msg.String() {
			case "enter":
				newName := strings.TrimSpace(m.nameInput.Value())
				if newName != "" && m.cursor < len(m.playlists) {
					oldName := m.playlists[m.cursor].Name
					if newName != oldName {
						playlist.Rename(oldName, newName)
					}
					m.renaming = false
					m.nameInput.Blur()
					m.nameInput.SetValue("")
					return m, loadPlaylists()
				}
			case "esc":
				m.renaming = false
				m.nameInput.Blur()
				m.nameInput.SetValue("")
				return m, nil
			}
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}

		if m.viewing {
			switch msg.String() {
			case "esc":
				m.viewing = false
				m.trackCursor = 0
				m.trackScroll = 0
			case "up", "k":
				if m.trackCursor > 0 {
					m.trackCursor--
				}
			case "down", "j":
				pl := m.selectedPlaylist()
				if pl != nil && m.trackCursor < len(pl.Tracks)-1 {
					m.trackCursor++
				}
			case "delete", "backspace":
				pl := m.selectedPlaylist()
				if pl != nil && m.trackCursor < len(pl.Tracks) {
					pl.RemoveTrack(m.trackCursor)
					pl.Save()
					if m.trackCursor >= len(pl.Tracks) && m.trackCursor > 0 {
						m.trackCursor--
					}
					m.playlists[m.cursor] = *pl
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.playlists)-1 {
				m.cursor++
			}
		case "c":
			m.creating = true
			m.nameInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m playlistModel) View(width, maxHeight int, rf ratingFunc) string {
	var b strings.Builder

	if m.creating {
		b.WriteString(headerStyle.Render("  New Playlist"))
		b.WriteString("\n")
		boxStyle := searchBoxFocusedStyle.Width(min(width-4, 50))
		b.WriteString(boxStyle.Render(m.nameInput.View()))
		return b.String()
	}

	if m.renaming {
		b.WriteString(headerStyle.Render("  Rename Playlist"))
		b.WriteString("\n")
		boxStyle := searchBoxFocusedStyle.Width(min(width-4, 50))
		b.WriteString(boxStyle.Render(m.nameInput.View()))
		return b.String()
	}

	if m.viewing {
		pl := m.selectedPlaylist()
		if pl != nil {
			b.WriteString(headerStyle.Render(fmt.Sprintf("  %s (%d tracks)", pl.Name, len(pl.Tracks))))
			b.WriteString("\n")

			fixedCols := 28
			remaining := width - fixedCols - 4
			if remaining < 30 {
				remaining = 30
			}
			titleW := remaining * 3 / 5
			artistW := remaining - titleW

			header := fmt.Sprintf("  %s  %s  %-5s  %-5s  %5s", padRight("Title", titleW), padRight("Artist", artistW), "Dur", "Rate", "Plays")
			b.WriteString(colHeaderStyle.Render(header))
			b.WriteString("\n")

			visibleLines := maxHeight - 4
			if visibleLines < 1 {
				visibleLines = 1
			}

			end := m.trackScroll + visibleLines
			if end > len(pl.Tracks) {
				end = len(pl.Tracks)
			}

			for i := m.trackScroll; i < end; i++ {
				t := pl.Tracks[i]
				pointer := "  "
				if i == m.trackCursor {
					pointer = cursorStyle.Render("> ")
				}
				stars := renderRating(t.ID, rf)
				plays := renderPlays(t.ID, rf)
				line := fmt.Sprintf("%s  %s  %-5s  %s  %s",
					padRight(truncate(t.Title, titleW), titleW),
					padRight(truncate(t.Artist, artistW), artistW),
					t.Duration,
					stars,
					plays,
				)
				if i == m.trackCursor {
					b.WriteString(pointer + selectedStyle.Render(line))
				} else {
					b.WriteString(pointer + normalStyle.Render(line))
				}
				b.WriteString("\n")
			}
		}
		return b.String()
	}

	if len(m.playlists) == 0 {
		b.WriteString(statusStyle.Render("  No playlists — press 'c' to create one"))
		return b.String()
	}

	b.WriteString(headerStyle.Render("  Playlists"))
	b.WriteString("\n")

	nameW := width/2 - 4
	if nameW < 20 {
		nameW = 20
	}

	header := fmt.Sprintf("  %s  %s", padRight("Name", nameW), "Tracks")
	b.WriteString(colHeaderStyle.Render(header))
	b.WriteString("\n")

	visibleLines := maxHeight - 4
	hasUp := m.scroll > 0
	if hasUp {
		visibleLines--
	}
	if m.scroll+visibleLines < len(m.playlists) {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.scroll + visibleLines
	if end > len(m.playlists) {
		end = len(m.playlists)
	}

	if hasUp {
		b.WriteString(dimStyle("  ↑ more") + "\n")
	}

	for i := m.scroll; i < end; i++ {
		p := m.playlists[i]
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("> ")
		}
		line := fmt.Sprintf("%s  %d tracks", padRight(truncate(p.Name, nameW), nameW), len(p.Tracks))
		if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(m.playlists) {
		b.WriteString(dimStyle("  ↓ more"))
	}

	return b.String()
}

func (m *playlistModel) ensureVisible(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	if m.viewing {
		pl := m.selectedPlaylist()
		n := 0
		if pl != nil {
			n = len(pl.Tracks)
		}
		// Playlist track view has no ↑/↓ indicators, simple clamp
		if maxScroll := n - maxVisible; maxScroll > 0 {
			if m.trackScroll > maxScroll {
				m.trackScroll = maxScroll
			}
		} else {
			m.trackScroll = 0
		}
		if m.trackCursor < m.trackScroll {
			m.trackScroll = m.trackCursor
		}
		if m.trackCursor >= m.trackScroll+maxVisible {
			m.trackScroll = m.trackCursor - maxVisible + 1
		}
	} else {
		n := len(m.playlists)
		for range 2 {
			effective := maxVisible
			if m.scroll > 0 {
				effective--
			}
			if m.scroll+effective < n {
				effective--
			}
			if effective < 1 {
				effective = 1
			}
			if maxScroll := n - effective; maxScroll > 0 {
				if m.scroll > maxScroll {
					m.scroll = maxScroll
				}
			} else {
				m.scroll = 0
			}
			if m.cursor < m.scroll {
				m.scroll = m.cursor
			}
			if m.cursor >= m.scroll+effective {
				m.scroll = m.cursor - effective + 1
			}
		}
	}
}

func (m playlistModel) selectedPlaylist() *playlist.Playlist {
	if m.cursor >= 0 && m.cursor < len(m.playlists) {
		p := m.playlists[m.cursor]
		return &p
	}
	return nil
}

func (m playlistModel) selectedTrack() *player.Track {
	pl := m.selectedPlaylist()
	if pl != nil && m.trackCursor >= 0 && m.trackCursor < len(pl.Tracks) {
		t := pl.Tracks[m.trackCursor]
		return &t
	}
	return nil
}
