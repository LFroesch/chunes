package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/youtube"
)

var sourceNames = []string{"YouTube", "SoundCloud"}

type searchModel struct {
	input    textinput.Model
	results  []player.Track
	cursor   int
	scroll   int // top visible index
	loading  bool
	err      error
	focused  bool
	spinTick int
	source   int // 0=YouTube, 1=SoundCloud
}

type searchResultsMsg struct {
	tracks []player.Track
	err    error
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "Search YouTube Music..."
	ti.CharLimit = 200
	ti.Width = 60
	ti.PromptStyle = searchPromptStyle
	ti.Prompt = " 🔍 "
	return searchModel{input: ti}
}

func doSearch(query string, source int) tea.Cmd {
	return func() tea.Msg {
		src := "youtube"
		if source == 1 {
			src = "soundcloud"
		}
		tracks, err := youtube.Search(query, 10, src)
		return searchResultsMsg{tracks: tracks, err: err}
	}
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	if m.focused {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				query := strings.TrimSpace(m.input.Value())
				if query != "" {
					m.loading = true
					m.err = nil
					m.focused = false
					m.input.Blur()
					return m, doSearch(query, m.source)
				}
			case "esc":
				m.focused = false
				m.input.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case searchResultsMsg:
		m.loading = false
		m.results = msg.tracks
		m.err = msg.err
		m.cursor = 0
		m.scroll = 0
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.source = (m.source + 1) % 2
			if m.source == 1 {
				m.input.Placeholder = "Search SoundCloud..."
			} else {
				m.input.Placeholder = "Search YouTube Music..."
			}
			return m, nil
		case "/":
			m.focused = true
			m.input.Focus()
			return m, textinput.Blink
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// ensureVisible adjusts scroll so cursor is visible within maxVisible lines
func (m *searchModel) ensureVisible(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	n := len(m.results)
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

func (m searchModel) View(width, maxHeight int, rf ratingFunc) string {
	var b strings.Builder

	// Update prompt to show source
	sourceBadge := sourceNames[m.source]
	m.input.Prompt = fmt.Sprintf(" 🔍 %s: ", sourceBadge)

	// Search box
	boxStyle := searchBoxStyle.Width(min(width-4, 70))
	if m.focused {
		boxStyle = searchBoxFocusedStyle.Width(min(width-4, 70))
	}
	b.WriteString(boxStyle.Render(m.input.View()))
	b.WriteString("\n\n")

	if m.loading {
		spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := spinnerFrames[(m.spinTick/2)%len(spinnerFrames)]
		spinner := lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(frame)
		b.WriteString(fmt.Sprintf("  %s Searching...", spinner))
		return b.String()
	}
	if m.err != nil {
		b.WriteString(errorStyle.Render("  ✗ " + m.err.Error()))
		return b.String()
	}
	if len(m.results) == 0 {
		b.WriteString(statusStyle.Render(" 🎵  Press / to search for music"))
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(" 🌐  Press Tab to switch source"))
		b.WriteString("\n")
		return b.String()
	}

	// Column widths: fixed = Dur(5) + Rate(5) + Plays(5) + spacing(13) = ~28
	fixedCols := 28
	remaining := width - fixedCols - 4
	if remaining < 30 {
		remaining = 30
	}
	titleW := remaining * 3 / 5
	artistW := remaining - titleW

	// Column header
	header := fmt.Sprintf("  %s  %s  %-5s  %-5s  %5s", padRight("Title", titleW), padRight("Artist", artistW), "Dur", "Rate", "Plays")
	b.WriteString(colHeaderStyle.Render(header))
	b.WriteString("\n")

	// Scrollable results
	visibleLines := maxHeight - 5 // account for search box, header, padding
	hasUp := m.scroll > 0
	if hasUp {
		visibleLines--
	}
	if m.scroll+visibleLines < len(m.results) {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.scroll + visibleLines
	if end > len(m.results) {
		end = len(m.results)
	}

	// Scroll indicators
	if hasUp {
		b.WriteString(dimStyle("  ↑ more") + "\n")
	}

	for i := m.scroll; i < end; i++ {
		t := m.results[i]
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("> ")
		}

		title := truncate(t.Title, titleW)
		artist := truncate(t.Artist, artistW)
		dur := t.Duration
		stars := renderRating(t.ID, rf)
		plays := renderPlays(t.ID, rf)

		line := fmt.Sprintf("%s  %s  %-5s  %s  %s", padRight(title, titleW), padRight(artist, artistW), dur, stars, plays)
		if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(m.results) {
		b.WriteString(dimStyle("  ↓ more"))
	}

	// Result count
	b.WriteString("\n")
	b.WriteString(statusStyle.Render(fmt.Sprintf("  %d results", len(m.results))))

	return b.String()
}

func (m searchModel) Selected() *player.Track {
	if m.cursor >= 0 && m.cursor < len(m.results) {
		t := m.results[m.cursor]
		return &t
	}
	return nil
}

func truncate(s string, maxWidth int) string {
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	return runewidth.Truncate(s, maxWidth, "...")
}

func padRight(s string, width int) string {
	return runewidth.FillRight(s, width)
}

func dimStyle(s string) string {
	return statusStyle.Render(s)
}
