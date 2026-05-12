package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpBinding struct {
	key  string
	desc string
}

type helpSection struct {
	name     string
	bindings []helpBinding
}

var helpSections = []helpSection{
	{"Playback", []helpBinding{
		{"Space", "Toggle pause (global)"},
		{"Enter", "Play selected / toggle pause"},
		{"n", "Next track"},
		{"p", "Back"},
		{"0", "Restart track"},
		{"←/→", "Seek ±5 seconds"},
		{"</>", "Seek ±30 seconds"},
		{"+/-", "Volume up / down (to 200%)"},
		{"S", "Toggle queue shuffle"},
		{"r", "Cycle repeat mode"},
	}},
	{"Navigation", []helpBinding{
		{"1-7", "Switch views"},
		{"/", "Search"},
		{"j/k", "Navigate up / down"},
		{"Esc / q", "Back / unfocus"},
		{"ctrl+c", "Quit"},
	}},
	{"Tracks", []helpBinding{
		{"a", "Add to queue"},
		{"d", "Download track"},
		{"s", "Save to playlist"},
		{"R", "Cycle track rating (★)"},
		{"l", "Load more suggestions"},
	}},
	{"Queue", []helpBinding{
		{"Del / x", "Remove track"},
		{"C", "Clear queue"},
	}},
	{"Playlists", []helpBinding{
		{"A", "Queue all"},
		{"c", "Create playlist"},
		{"e", "Rename playlist"},
		{"Z", "Shuffle playlist tracks"},
		{"J/K", "Reorder track (inside playlist)"},
		{"Del", "Delete playlist / remove track"},
	}},
	{"History", []helpBinding{
		{"o", "Cycle sort mode"},
		{"Del / x", "Delete entry"},
	}},
	{"Downloads", []helpBinding{
		{"Del / x", "Delete download"},
	}},
	{"Visualizer (Now Playing)", []helpBinding{
		{"m", "Toggle track info / visualizer view"},
		{"j/k", "Scroll track info"},
		{"v / V", "Next / previous viz style"},
		{"c", "Auto-cycle viz styles"},
	}},
	{"Help", []helpBinding{
		{"?", "Toggle this help"},
		{"j/k", "Scroll help"},
		{"q / Esc", "Close help"},
	}},
}

// helpTotalLines returns the rendered line count of the help overlay.
func helpTotalLines() int {
	n := 2 // title + divider
	for _, sec := range helpSections {
		n += 2 + len(sec.bindings) // blank + section header + bindings
	}
	return n
}

func renderHelp(width, scroll, height int) string {
	title := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("Keybindings")

	var allLines []string
	allLines = append(allLines, "  "+title)
	allLines = append(allLines, "  "+dividerStyle.Render(strings.Repeat("─", min(40, width-4))))
	for _, sec := range helpSections {
		allLines = append(allLines, "")
		allLines = append(allLines, "  "+headerStyle.Render(sec.name))
		for _, h := range sec.bindings {
			allLines = append(allLines, "  "+helpKeyStyle.Render(h.key)+helpDescStyle.Render(h.desc))
		}
	}

	total := len(allLines)
	if total <= height {
		return strings.Join(allLines, "\n")
	}

	maxScroll := total - height
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + height
	if end > total {
		end = total
	}

	var b strings.Builder
	for i := scroll; i < end; i++ {
		if i == scroll && scroll > 0 {
			b.WriteString(dimStyle("  ↑ more"))
		} else if i == end-1 && end < total {
			b.WriteString(dimStyle("  ↓ more"))
		} else {
			b.WriteString(allLines[i])
		}
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderHints returns a single condensed footer row in the sb style:
// key action · key action · key action
func renderHints(hints []helpBinding, width int) string {
	if width < 1 || len(hints) == 0 {
		return hintBarStyle.Render("")
	}

	sep := hintSepStyle.Render(" · ")
	sepW := lipgloss.Width(sep)

	var parts []string
	lineW := 0
	for _, h := range hints {
		part := hintKeyStyle.Render(h.key) + hintDescStyle.Render(" "+h.desc)
		partW := lipgloss.Width(part)
		extraW := partW
		if len(parts) > 0 {
			extraW += sepW
		}
		if len(parts) > 0 && lineW+extraW > width {
			break
		}
		parts = append(parts, part)
		lineW += extraW
	}

	line := strings.Join(parts, sep)
	return hintBarStyle.Render(lipgloss.PlaceHorizontal(width, lipgloss.Center, line))
}

// Context-specific hint sets
func searchHints(focused bool) []helpBinding {
	if focused {
		return []helpBinding{
			{"Enter", "find"},
			{"Esc", "back"},
		}
	}
	return []helpBinding{
		{"/", "find"},
		{"Tab", "src"},
		{"Enter", "play"},
		{"a", "add"},
	}
}

func suggestionsHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"a", "add"},
		{"d", "dl"},
		{"l", "more"},
	}
}

func queueHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"Del", "rm"},
		{"C", "clear"},
		{"s", "save"},
	}
}

func playlistHints(viewing, creating, renaming bool) []helpBinding {
	if creating || renaming {
		label := "create"
		if renaming {
			label = "rename"
		}
		return []helpBinding{
			{"Enter", label},
			{"Esc", "back"},
		}
	}
	if viewing {
		return []helpBinding{
			{"Enter", "play"},
			{"A", "all"},
			{"J/K", "move"},
			{"Z", "shuf"},
			{"Del", "rm"},
			{"Esc", "back"},
		}
	}
	return []helpBinding{
		{"Enter", "open"},
		{"A", "all"},
		{"c", "new"},
		{"e", "ren"},
		{"Del", "rm"},
	}
}

func historyHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"a", "add"},
		{"o", "sort"},
		{"Del", "rm"},
	}
}

func downloadHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"a", "add"},
		{"s", "save"},
		{"Del", "rm"},
	}
}
