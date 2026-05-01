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
		{"p", "Replay current track"},
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
	{"Playlists", []helpBinding{
		{"A", "Queue all"},
		{"e", "Rename playlist"},
		{"Z", "Shuffle playlist tracks"},
	}},
	{"Visualizer (Now Playing)", []helpBinding{
		{"m", "Toggle metadata / visualizer panel"},
		{"v", "Next viz style"},
		{"V", "Random viz style"},
		{"C", "Auto-cycle viz styles"},
		{"[ ]", "Viz energy down / up"},
		{"G", "Toggle viz AGC (auto-gain)"},
	}},
	{"Help", []helpBinding{
		{"?", "Toggle this help"},
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
		{"p", "replay"},
		{"+/-", "vol"},
		{"←/→", "seek"},
		{"R", "rate"},
		{"v", "viz"},
	}
}
