package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpBinding struct {
	key  string
	desc string
}

var helpBindings = []helpBinding{
	{"/", "Search"},
	{"Enter/Space", "Play selected / pause"},
	{"n", "Next track"},
	{"p", "Previous track"},
	{"+/-", "Volume up / down"},
	{"a", "Add to queue"},
	{"s", "Save to playlist"},
	{"d", "Download track"},
	{"A", "Queue all (playlists)"},
	{"←/→", "Seek ±5 seconds"},
	{"</>", "Seek ±30 seconds"},
	{"0", "Restart track"},
	{"1-7", "Switch views"},
	{"j/k", "Navigate up / down"},
	{"v", "Next viz style"},
	{"V", "Random viz style"},
	{"C", "Auto-cycle viz"},
	{"[ ]", "Viz energy down / up"},
	{"l", "Load more suggestions"},
	{"R", "Cycle track rating (★)"},
	{"S", "Toggle queue shuffle"},
	{"r", "Cycle repeat mode"},
	{"Z", "Shuffle playlist tracks"},
	{"e", "Rename playlist"},
	{"Esc / q", "Back / unfocus"},
	{"ctrl+c", "Quit"},
	{"?", "Toggle this help"},
}

func renderHelp(width, scroll, height int) string {
	title := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("Keybindings")

	var allLines []string
	allLines = append(allLines, "  "+title)
	allLines = append(allLines, "  "+dividerStyle.Render(strings.Repeat("─", min(40, width-4))))
	allLines = append(allLines, "")
	for _, h := range helpBindings {
		allLines = append(allLines, "  "+helpKeyStyle.Render(h.key)+helpDescStyle.Render(h.desc))
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

// renderHints returns a compact keybind hint bar for the current context,
// truncating hints that don't fit within the available width.
func renderHints(hints []helpBinding, width int) string {
	var parts []string
	totalWidth := 0
	for _, h := range hints {
		var part string
		if h.desc == "" && h.key == "│" {
			part = hintSepStyle.Render("│")
		} else {
			part = hintKeyStyle.Render(h.key) + hintDescStyle.Render(h.desc)
		}
		partW := lipgloss.Width(part)
		if totalWidth+partW+1 > width && totalWidth > 0 {
			break
		}
		parts = append(parts, part)
		totalWidth += partW + 1 // +1 for space separator
	}
	line := strings.Join(parts, " ")
	return hintBarStyle.Render(line)
}

// Context-specific hint sets
func searchHints(focused bool) []helpBinding {
	if focused {
		return []helpBinding{
			{"Enter", "search"},
			{"Esc", "cancel"},
		}
	}
	return []helpBinding{
		{"/", " search"},
		{"Tab", " source"},
		{"Enter", " play"},
		{"a", " queue"},
		{"d", " download"},
		{"s", " playlist"},
		{"R", " rate"},
		{"?", " help"},
	}
}

func suggestionsHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"j/k", "navigate"},
		{"l", "load more"},
		{"a", "queue"},
		{"d", "download"},
		{"s", "playlist"},
		{"R", "rate"},
		{"?", "help"},
	}
}

func queueHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"Del", "remove"},
		{"C", "clear"},
		{"j/k", "navigate"},
		{"a", "queue"},
		{"d", "download"},
		{"s", "playlist"},
		{"R", "rate"},
		{"?", "help"},
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
			{"Esc", "cancel"},
		}
	}
	if viewing {
		return []helpBinding{
			{"Enter", "play"},
			{"A", "queue all"},
			{"J/K", "reorder"},
			{"Z", "shuffle"},
			{"Del", "remove"},
			{"a", "queue"},
			{"d", "download"},
			{"s", "playlist"},
			{"R", "rate"},
			{"Esc", "back"},
			{"?", "help"},
		}
	}
	return []helpBinding{
		{"Enter", "open"},
		{"A", "queue all"},
		{"c", "create"},
		{"e", "rename"},
		{"Z", "shuffle"},
		{"Del", "delete"},
		{"?", "help"},
	}
}

func historyHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"j/k", "navigate"},
		{"a", "queue"},
		{"d", "download"},
		{"s", "playlist"},
		{"R", "rate"},
		{"o", "sort"},
		{"Del", "delete"},
		{"?", "help"},
	}
}

func downloadHints() []helpBinding {
	return []helpBinding{
		{"Enter", "play"},
		{"j/k", "navigate"},
		{"a", "queue"},
		{"s", "playlist"},
		{"R", "rate"},
		{"Del", "delete"},
		{"?", "help"},
	}
}

func playerHints() []helpBinding {
	return []helpBinding{
		{"Space", "pause"},
		{"n", "next"},
		{"p", "prev"},
		{"+/-", "vol"},
		{"←/→", "seek"},
		{"R", "rate"},
		{"v", "viz"},
	}
}
