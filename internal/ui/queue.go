package ui

import (
	"fmt"
	"strings"

	"github.com/lucas/chunes/internal/player"
)

type queueModel struct {
	cursor int
	scroll int
}

func newQueueModel() queueModel {
	return queueModel{}
}

func (m *queueModel) ensureVisible(maxVisible int, listLen int) {
	if maxVisible <= 0 {
		return
	}
	for range 2 {
		effective := maxVisible
		if m.scroll > 0 {
			effective--
		}
		if m.scroll+effective < listLen {
			effective--
		}
		if effective < 1 {
			effective = 1
		}
		if maxScroll := listLen - effective; maxScroll > 0 {
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

func (m queueModel) View(q *player.Queue, width, maxHeight int, rf ratingFunc) string {
	var b strings.Builder
	tracks := q.Tracks()

	if len(tracks) == 0 {
		b.WriteString(statusStyle.Render("  Queue is empty — search and press 'a' to add tracks"))
		return b.String()
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("  Up Next (%d tracks)", len(tracks))))
	b.WriteString("\n")

	// Column widths: fixed = Dur(5) + Rate(5) + Plays(5) + spacing(14) = ~29
	fixedCols := 29
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
	hasUp := m.scroll > 0
	if hasUp {
		visibleLines--
	}
	if m.scroll+visibleLines < len(tracks) {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.scroll + visibleLines
	if end > len(tracks) {
		end = len(tracks)
	}

	if hasUp {
		b.WriteString(dimStyle("  ↑ more") + "\n")
	}

	for i := m.scroll; i < end; i++ {
		t := tracks[i]
		pointer := "  "
		if i == m.cursor {
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
		if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(tracks) {
		b.WriteString(dimStyle("  ↓ more"))
	}

	return b.String()
}

func (m queueModel) selectedTrack(q *player.Queue) *player.Track {
	tracks := q.Tracks()
	if m.cursor >= 0 && m.cursor < len(tracks) {
		t := tracks[m.cursor]
		return &t
	}
	return nil
}
