package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucas/chunes/internal/player"
)

func renderPlayerBar(track *player.Track, position, duration float64, volume int, paused bool, shuffle bool, repeat player.RepeatMode, rating int, width int) string {
	if width < 20 {
		width = 20
	}

	if track == nil {
		line1 := lipgloss.NewStyle().Foreground(dimColor).Render("  ♪  No track playing")
		return line1 + "\n"
	}

	// Status icon
	icon := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("▶")
	if paused {
		icon = lipgloss.NewStyle().Foreground(warnColor).Bold(true).Render("⏸")
	}

	// Track info
	titleMax := max(width/3, 15)
	artistMax := max(width/5, 10)

	title := trackTitleStyle.Render(truncate(track.Title, titleMax))
	artist := trackArtistStyle.Render(truncate(track.Artist, artistMax))

	// Time
	posStr := formatSeconds(position)
	durStr := formatSeconds(duration)
	timeStr := lipgloss.NewStyle().Foreground(dimColor).Render(posStr) +
		lipgloss.NewStyle().Foreground(dimColor).Render("/") +
		lipgloss.NewStyle().Foreground(dimColor).Render(durStr)

	// Volume with visual indicator
	volIcon := "🔊"
	if volume == 0 {
		volIcon = "🔇"
	} else if volume < 30 {
		volIcon = "🔈"
	} else if volume < 70 {
		volIcon = "🔉"
	}
	volStr := volumeStyle.Render(fmt.Sprintf("%s %d%%", volIcon, volume))

	// Mode indicators
	var modes []string
	if shuffle {
		modes = append(modes, lipgloss.NewStyle().Foreground(accentColor).Render("⤮"))
	}
	switch repeat {
	case player.RepeatAll:
		modes = append(modes, lipgloss.NewStyle().Foreground(secondaryColor).Render("↻"))
	case player.RepeatOne:
		modes = append(modes, lipgloss.NewStyle().Foreground(secondaryColor).Render("↻1"))
	}
	modeStr := ""
	if len(modes) > 0 {
		modeStr = "  " + strings.Join(modes, " ")
	}

	// Progress bar
	barWidth := max(width-16, 10) // leave room for time + vol
	progress := 0.0
	if duration > 0 {
		progress = position / duration
	}
	filled := min(int(float64(barWidth)*progress), barWidth)

	// Gradient-filled progress bar
	var barParts strings.Builder
	for j := 0; j < filled; j++ {
		// Gradient from bright to dim across the filled portion
		frac := float64(j) / float64(max(filled, 1))
		r := int(0xFF - frac*0x8D) // FF→72
		g := int(0x6A + frac*0x87) // 6A→F1
		b2 := int(0xC1 - frac*0x09) // C1→B8
		color := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b2))
		barParts.WriteString(lipgloss.NewStyle().Foreground(color).Render("━"))
	}
	barFilled := barParts.String()
	barEmpty := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("─", barWidth-filled))
	// Playhead dot
	dot := ""
	if filled < barWidth {
		barEmpty = lipgloss.NewStyle().Foreground(primaryColor).Render("●") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("─", barWidth-filled-1))
	} else {
		dot = lipgloss.NewStyle().Foreground(primaryColor).Render("●")
	}
	bar := barFilled + barEmpty + dot

	// Rating stars
	ratingStr := ""
	if rating > 0 {
		stars := strings.Repeat("★", rating) + strings.Repeat("☆", 5-rating)
		ratingStr = "  " + lipgloss.NewStyle().Foreground(warnColor).Render(stars)
	}

	line1 := fmt.Sprintf("  %s  %s  %s%s%s", icon, title, artist, ratingStr, modeStr)
	line2 := fmt.Sprintf("  %s  %s  %s", bar, timeStr, volStr)

	return line1 + "\n" + line2
}

func formatSeconds(s float64) string {
	total := int(s)
	if total < 0 {
		total = 0
	}
	m := total / 60
	sec := total % 60
	return fmt.Sprintf("%d:%02d", m, sec)
}
