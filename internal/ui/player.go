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
	artistMax := max(width/4, 10)

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

	// Rating stars
	ratingStr := ""
	if rating > 0 {
		stars := strings.Repeat("★", rating) + strings.Repeat("☆", 5-rating)
		ratingStr = "  " + lipgloss.NewStyle().Foreground(warnColor).Render(stars)
	}

	line1Content := fmt.Sprintf("%s  %s  %s%s%s", icon, title, artist, ratingStr, modeStr)

	line2Fixed := fmt.Sprintf("%s  %s", timeStr, volStr)
	fixedWidth := lipgloss.Width(line2Fixed)
	barWidth := max(width-fixedWidth-6, 10)
	progress := 0.0
	if duration > 0 {
		progress = position / duration
	}
	filled := min(int(float64(barWidth)*progress), barWidth)

	// Gradient-filled progress bar
	var barParts strings.Builder
	for j := 0; j < filled; j++ {
		frac := float64(j) / float64(max(filled, 1))
		r := int(0xFF - frac*0x8D)
		g := int(0x6A + frac*0x87)
		b2 := int(0xC1 - frac*0x09)
		color := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b2))
		barParts.WriteString(lipgloss.NewStyle().Foreground(color).Render("━"))
	}
	barFilled := barParts.String()
	barEmpty := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("─", barWidth-filled))
	dot := ""
	if filled < barWidth {
		barEmpty = lipgloss.NewStyle().Foreground(primaryColor).Render("●") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("─", barWidth-filled-1))
	} else {
		dot = lipgloss.NewStyle().Foreground(primaryColor).Render("●")
	}
	bar := barFilled + barEmpty + dot

	line2Content := fmt.Sprintf("%s  %s", bar, line2Fixed)

	line1 := lipgloss.PlaceHorizontal(width, lipgloss.Center, line1Content)
	line2 := lipgloss.PlaceHorizontal(width, lipgloss.Center, line2Content)

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
