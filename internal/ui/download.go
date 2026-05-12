package ui

import (
	"fmt"
	"strings"

	"github.com/lucas/chunes/internal/download"
	"github.com/lucas/chunes/internal/player"
)

type downloadItem struct {
	track   player.Track
	percent float64
	done    bool
	err     error
	path    string
}

type downloadModel struct {
	items     []downloadItem
	cursor    int
	scroll    int
	outputDir string
	format    string
}

type downloadProgressMsg struct {
	progress download.Progress
	ch       <-chan download.Progress
}

func newDownloadModel(outputDir, format string) downloadModel {
	m := downloadModel{outputDir: outputDir, format: format}
	// Load completed downloads from library
	entries, _ := download.LoadLibrary()
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		m.items = append(m.items, downloadItem{
			track:   e.Track,
			percent: 100,
			done:    true,
			path:    e.Path,
		})
	}
	return m
}

func (m *downloadModel) prepend(item downloadItem) {
	m.items = append([]downloadItem{item}, m.items...)
	m.cursor = 0
	m.scroll = 0
}

func (m *downloadModel) moveToFront(index int) {
	if index <= 0 || index >= len(m.items) {
		return
	}
	item := m.items[index]
	copy(m.items[1:index+1], m.items[0:index])
	m.items[0] = item
	m.cursor = 0
	m.scroll = 0
}

func (m *downloadModel) add(t player.Track) {
	// Don't add if already exists
	for i, item := range m.items {
		if item.track.ID == t.ID {
			m.moveToFront(i)
			return
		}
	}
	m.prepend(downloadItem{track: t})
}

func (m *downloadModel) begin(t player.Track) {
	for i := range m.items {
		if m.items[i].track.ID == t.ID {
			m.items[i].percent = 0
			m.items[i].done = false
			m.items[i].err = nil
			m.items[i].path = ""
			m.moveToFront(i)
			return
		}
	}
	m.prepend(downloadItem{track: t})
}

func (m *downloadModel) updateProgress(p download.Progress) {
	for i := range m.items {
		if m.items[i].track.ID == p.Track.ID {
			m.items[i].percent = p.Percent
			m.items[i].done = p.Done
			m.items[i].err = p.Error
			if p.Path != "" {
				m.items[i].path = p.Path
			}
			if p.Done {
				path := m.items[i].path
				if path == "" {
					path = download.FindDownloadedPath(p.Track, m.outputDir, m.format)
					m.items[i].path = path
				}
				download.AddToLibrary(p.Track, path)
			}
			return
		}
	}
}

func (m *downloadModel) ensureVisible(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	n := len(m.items)
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

func (m downloadModel) View(width, maxHeight int, rf ratingFunc) string {
	var b strings.Builder

	if len(m.items) == 0 {
		b.WriteString(centerLine(width, statusStyle.Render("No downloads — press 'd' on a track to download")))
		b.WriteString("\n\n")
		b.WriteString(centerLine(width, statusStyle.Render(fmt.Sprintf("Download dir: %s", m.outputDir))))
		return b.String()
	}

	b.WriteString(centerLine(width, headerStyle.Render(fmt.Sprintf("Downloads (%d)  →  %s", len(m.items), m.outputDir))))
	b.WriteString("\n")

	// Fixed cols: Dur(5) + Rate(5) + Plays(5) + Progress(20) + Status(8) + spacing(17) = ~60
	fixedCols := 60
	remaining := width - fixedCols - 4
	if remaining < 30 {
		remaining = 30
	}
	titleW := remaining * 3 / 5
	artistW := remaining - titleW

	header := fmt.Sprintf("  %s  %s  %-5s  %-5s  %5s  %-20s  %s", padRight("Title", titleW), padRight("Artist", artistW), "Dur", "Rate", "Plays", "Progress", "Status")
	b.WriteString(colHeaderStyle.Render(header))
	b.WriteString("\n")

	visibleLines := maxHeight - 4
	hasUp := m.scroll > 0
	if hasUp {
		visibleLines--
	}
	if m.scroll+visibleLines < len(m.items) {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.scroll + visibleLines
	if end > len(m.items) {
		end = len(m.items)
	}

	if hasUp {
		b.WriteString(centerLine(width, dimStyle("↑ more")) + "\n")
	}

	for i := m.scroll; i < end; i++ {
		item := m.items[i]
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("> ")
		}

		status := fmt.Sprintf("%.0f%%", item.percent)
		if item.done {
			status = "✓ Done"
		}
		if item.err != nil {
			status = "✗ Error"
		}

		barWidth := 20
		filled := int(float64(barWidth) * item.percent / 100)
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		stars := renderRating(item.track.ID, rf)
		plays := renderPlays(item.track.ID, rf)
		line := fmt.Sprintf("%s  %s  %-5s  %s  %s  %s  %s",
			padRight(truncate(item.track.Title, titleW), titleW),
			padRight(truncate(item.track.Artist, artistW), artistW),
			item.track.Duration,
			stars,
			plays,
			bar, status,
		)
		if item.err != nil {
			b.WriteString(pointer + errorStyle.Render(line))
		} else if item.done {
			b.WriteString(pointer + progressStyle.Render(line))
		} else if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(m.items) {
		b.WriteString(centerLine(width, dimStyle("↓ more")))
	}

	return b.String()
}

func (m downloadModel) selectedTrack() *player.Track {
	if m.cursor >= 0 && m.cursor < len(m.items) {
		t := m.items[m.cursor].track
		return &t
	}
	return nil
}
