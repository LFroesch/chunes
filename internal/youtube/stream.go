package youtube

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lucas/chunes/internal/player"
)

func GetStreamURL(id string) (string, error) {
	return getStreamURLFromReference(id)
}

func GetStreamURLForTrack(track player.Track) (string, error) {
	var refs []string
	if ref := playbackReference(track); ref != "" {
		refs = append(refs, ref)
	}
	if track.URL != "" && track.URL != track.ID {
		refs = append(refs, track.URL)
	}
	if track.ID != "" && track.ID != track.URL {
		refs = append(refs, track.ID)
	}

	var lastErr error
	for _, ref := range refs {
		url, err := getStreamURLWithRetry(ref)
		if err == nil {
			return url, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("empty track reference")
	}
	return "", lastErr
}

func PlayableURL(track player.Track) string {
	return playbackReference(track)
}

func playbackReference(track player.Track) string {
	if url := strings.TrimSpace(track.URL); url != "" {
		return url
	}
	return strings.TrimSpace(track.ID)
}

func getStreamURLWithRetry(ref string) (string, error) {
	url, err := getStreamURLFromReference(ref)
	if err == nil {
		return url, nil
	}
	time.Sleep(200 * time.Millisecond)
	return getStreamURLFromReference(ref)
}

func getStreamURLFromReference(ref string) (string, error) {
	url := normalizeReference(ref)
	if url == "" {
		return "", fmt.Errorf("empty stream reference")
	}
	cmd := exec.Command("yt-dlp",
		"-f", "bestaudio",
		"--get-url",
		"--no-playlist",
		"--quiet",
		url,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", explainYTDLPFailure(ref, err, out)
	}
	result := selectStreamURL(string(out))
	if result == "" {
		return "", fmt.Errorf("empty stream URL for %s", ref)
	}
	return result, nil
}

func selectStreamURL(out string) string {
	lines := strings.FieldsFunc(strings.TrimSpace(out), func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	if len(lines) == 0 {
		return ""
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		lower := strings.ToLower(line)
		if strings.Contains(lower, "mime=audio") || strings.Contains(lower, "audio%2f") {
			return line
		}
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

func normalizeReference(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", ref)
}

func explainYTDLPFailure(ref string, err error, out []byte) error {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return fmt.Errorf("failed to get stream URL for %s: %w", ref, err)
	}
	if strings.Contains(strings.ToLower(msg), "no supported javascript runtime") {
		return fmt.Errorf("yt-dlp needs a JavaScript runtime for this source; install nodejs or deno and retry")
	}
	return fmt.Errorf("failed to get stream URL for %s: %s", ref, msg)
}
