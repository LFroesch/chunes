package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// placeOverlay renders the overlay centered on top of the background.
// It writes overlay lines over the middle of the background string.
// Uses lipgloss.MaxWidth for ANSI-safe truncation of background lines.
func placeOverlay(width, height int, overlay, background string) string {
	bgLines := strings.Split(background, "\n")
	olLines := strings.Split(overlay, "\n")

	olH := len(olLines)
	olW := lipgloss.Width(overlay)

	// Center position
	startY := (height - olH) / 2
	startX := (width - olW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Ensure background has enough lines
	for len(bgLines) < startY+olH {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	for i, olLine := range olLines {
		y := startY + i
		if y >= len(bgLines) {
			break
		}
		// ANSI-safe: truncate background to startX visual columns
		bg := bgLines[y]
		left := lipgloss.NewStyle().MaxWidth(startX).Render(bg)
		leftW := lipgloss.Width(left)
		if leftW < startX {
			left += strings.Repeat(" ", startX-leftW)
		}
		// Pad right side to maintain full width
		olLineW := lipgloss.Width(olLine)
		rightPad := width - startX - olLineW
		if rightPad < 0 {
			rightPad = 0
		}
		bgLines[y] = left + olLine + strings.Repeat(" ", rightPad)
	}

	return strings.Join(bgLines, "\n")
}

// ratingFunc looks up a track's rating by ID. Returns (rating, playCount).
type ratingFunc func(id string) (int, int)

// renderRating returns a 5-char rating string: stars if rated, blank if not.
func renderRating(id string, rf ratingFunc) string {
	if rf == nil {
		return "     "
	}
	r, _ := rf(id)
	if r > 0 {
		return strings.Repeat("★", r) + strings.Repeat("☆", 5-r)
	}
	return "     "
}

// renderPlays returns a 5-char right-aligned play count string.
func renderPlays(id string, rf ratingFunc) string {
	if rf == nil {
		return "    -"
	}
	_, plays := rf(id)
	if plays > 0 {
		return fmt.Sprintf("%5d", plays)
	}
	return "    -"
}

var (
	// Colors
	primaryColor   = lipgloss.Color("#FF6AC1")
	secondaryColor = lipgloss.Color("#9B72FF")
	accentColor    = lipgloss.Color("#72F1B8")
	dimColor       = lipgloss.Color("#555555")
	textColor      = lipgloss.Color("#EEEEEE")
	errorColor     = lipgloss.Color("#FF5555")
	warnColor      = lipgloss.Color("#FFB86C")

	// Brand header
	brandStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Frame border color
	borderFgStyle = lipgloss.NewStyle().Foreground(dimColor)

	// Header / tabs
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(dimColor)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(textColor).
			Background(primaryColor).
			Bold(true)

	trackTitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	trackArtistStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	progressStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	volumeStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Lists
	selectedStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#3A3A5C")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	cursorStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Padding(0, 1)

	errorBarStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Padding(0, 1)

	// Search
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	searchBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			Padding(0, 1)

	searchBoxFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	// Help
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Width(12)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Hint bar at bottom
	hintBarStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	hintKeyStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Background(lipgloss.Color("#2A2A2A")).
			Bold(true).
			Padding(0, 1)

	hintDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Padding(0, 1, 0, 0)

	hintSepStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// General purpose
	statusStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	// Section headers
	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 0, 1, 0)

	dividerStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Column headers
	colHeaderStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Bold(true).
			Underline(true)
)

// Frame rendering helpers for the outer bordered layout

func frameTop(innerW int) string {
	return borderFgStyle.Render("╭" + strings.Repeat("─", innerW) + "╮")
}

func frameBottom(innerW int) string {
	return borderFgStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")
}

func frameDivider(innerW int) string {
	return borderFgStyle.Render("├" + strings.Repeat("─", innerW) + "┤")
}

func frameRow(content string, innerW int) string {
	bv := borderFgStyle.Render("│")
	// Truncate content that exceeds inner width
	if lipgloss.Width(content) > innerW {
		content = lipgloss.NewStyle().MaxWidth(innerW).Render(content)
	}
	cw := lipgloss.Width(content)
	pad := innerW - cw
	if pad < 0 {
		pad = 0
	}
	return bv + content + strings.Repeat(" ", pad) + bv
}
