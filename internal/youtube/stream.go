package youtube

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetStreamURL(id string) (string, error) {
	// If it looks like a URL already, use it directly
	url := id
	if !strings.HasPrefix(id, "http") {
		url = fmt.Sprintf("https://www.youtube.com/watch?v=%s", id)
	}
	cmd := exec.Command("yt-dlp",
		"-f", "bestaudio",
		"--get-url",
		"--no-playlist",
		"--quiet",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get stream URL for %s: %w", id, err)
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "", fmt.Errorf("empty stream URL for %s", id)
	}
	return result, nil
}
