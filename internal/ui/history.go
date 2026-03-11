package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/lucas/chunes/internal/history"
	"github.com/lucas/chunes/internal/player"
)

var sortModeNames = []string{"Recent", "Most played", "Highest rated"}

// parseDurationSecs parses "m:ss" or "h:mm:ss" strings to total seconds.
func parseDurationSecs(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

func historyStats(entries []history.Entry) string {
	if len(entries) == 0 {
		return ""
	}
	totalPlays := 0
	totalSecs := 0
	rated := 0
	ratingSum := 0
	for _, e := range entries {
		totalPlays += e.PlayCount
		totalSecs += e.PlayCount * parseDurationSecs(e.Track.Duration)
		if e.Rating > 0 {
			rated++
			ratingSum += e.Rating
		}
	}
	hours := totalSecs / 3600
	mins := (totalSecs % 3600) / 60
	var timeStr string
	if hours > 0 {
		timeStr = fmt.Sprintf("%dh%02dm", hours, mins)
	} else {
		timeStr = fmt.Sprintf("%dm", mins)
	}
	var ratingStr string
	if rated > 0 {
		avg := float64(ratingSum) / float64(rated)
		ratingStr = fmt.Sprintf("  ·  Rated: %d  ·  Avg: %.1f★", rated, avg)
	}
	return fmt.Sprintf("  Plays: %d  ·  Time: %s%s", totalPlays, timeStr, ratingStr)
}

type historyModel struct {
	entries    []history.Entry
	allEntries []history.Entry // full history for stats
	cursor     int
	scroll     int
	sortMode   int // 0=recent, 1=plays, 2=rating
}

func newHistoryModel() historyModel {
	return historyModel{}
}

func (m historyModel) sortedEntries() []history.Entry {
	sorted := make([]history.Entry, len(m.entries))
	copy(sorted, m.entries)
	switch m.sortMode {
	case 0: // recent — newest first
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].PlayedAt.After(sorted[j].PlayedAt)
		})
	case 1: // most played
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].PlayCount > sorted[j].PlayCount
		})
	case 2: // highest rated
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].Rating > sorted[j].Rating
		})
	}
	return sorted
}

func (m *historyModel) ensureVisible(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	n := len(m.entries)
	// Two passes: indicators depend on scroll, scroll depends on effective visible lines
	for range 2 {
		effective := maxVisible
		if m.scroll > 0 {
			effective-- // ↑ more indicator
		}
		if m.scroll+effective < n {
			effective-- // ↓ more indicator
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

func (m historyModel) View(width, maxHeight int) string {
	var b strings.Builder

	if len(m.entries) == 0 {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render("     ♪ · · ·") + "\n\n")
		b.WriteString(statusStyle.Render("  No play history yet — search and play some music!"))
		return b.String()
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("  History (%d tracks)  ·  Sort: %s", len(m.allEntries), sortModeNames[m.sortMode])))
	b.WriteString("\n")
	if stats := historyStats(m.allEntries); stats != "" {
		b.WriteString(dimStyle(stats) + "\n")
	}

	// Apply sort
	entries := m.sortedEntries()

	// Fixed columns: Dur(5) + Rate(5) + Plays(5) + Played(12) + spacing(14) = ~41
	fixedCols := 41
	remaining := width - fixedCols - 4 // 4 for pointer + padding
	if remaining < 30 {
		remaining = 30
	}
	titleW := remaining * 3 / 5
	artistW := remaining - titleW

	header := fmt.Sprintf("  %s  %s  %-5s  %-5s  %5s  %s", padRight("Title", titleW), padRight("Artist", artistW), "Dur", "Rate", "Plays", "Played")
	b.WriteString(colHeaderStyle.Render(header))
	b.WriteString("\n")

	visibleLines := maxHeight - 4
	// Reserve lines for ↑/↓ more indicators so content stays within maxHeight
	hasUp := m.scroll > 0
	if hasUp {
		visibleLines--
	}
	if m.scroll+visibleLines < len(entries) {
		visibleLines-- // reserve for ↓ more
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.scroll + visibleLines
	if end > len(entries) {
		end = len(entries)
	}

	if hasUp {
		b.WriteString(dimStyle("  ↑ more") + "\n")
	}

	for i := m.scroll; i < end; i++ {
		e := entries[i]
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("> ")
		}
		timeStr := e.PlayedAt.Format("Jan 02 15:04")
		plays := fmt.Sprintf("%5d", e.PlayCount)
		stars := "  ─  "
		if e.Rating > 0 {
			stars = strings.Repeat("★", e.Rating) + strings.Repeat("☆", 5-e.Rating)
		}
		line := fmt.Sprintf("%s  %s  %-5s  %-5s  %s  %s",
			padRight(truncate(e.Track.Title, titleW), titleW),
			padRight(truncate(e.Track.Artist, artistW), artistW),
			e.Track.Duration,
			stars,
			plays,
			timeStr,
		)
		if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(entries) {
		b.WriteString(dimStyle("  ↓ more"))
	}

	return b.String()
}

func (m historyModel) selectedEntry() *history.Entry {
	entries := m.sortedEntries()
	if m.cursor >= 0 && m.cursor < len(entries) {
		e := entries[m.cursor]
		return &e
	}
	return nil
}

func (m historyModel) selectedTrack() *player.Track {
	if e := m.selectedEntry(); e != nil {
		t := e.Track
		return &t
	}
	return nil
}
