package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

func wrapText(s string, width, maxLines int) []string {
	if width < 1 || maxLines < 1 {
		return nil
	}
	var lines []string
	for _, para := range strings.Split(strings.TrimSpace(s), "\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		words := strings.Fields(para)
		if len(words) == 0 {
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			candidate := line + " " + word
			if runewidth.StringWidth(candidate) <= width {
				line = candidate
				continue
			}
			lines = append(lines, line)
			if len(lines) >= maxLines {
				return lines
			}
			line = word
		}
		lines = append(lines, line)
		if len(lines) >= maxLines {
			return lines
		}
	}
	return lines
}
