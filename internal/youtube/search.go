package youtube

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lucas/chunes/internal/player"
)

type searchResult struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Channel    string  `json:"channel"`
	Uploader   string  `json:"uploader"`
	Duration   float64 `json:"duration"`
	URL        string  `json:"url"`
	WebpageURL string  `json:"webpage_url"`
}

// IsURL returns true if the string looks like a direct YouTube/SoundCloud URL.
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// GetTrackInfo resolves a direct URL to a Track with metadata via yt-dlp.
func GetTrackInfo(url string) (player.Track, error) {
	cmd := exec.Command("yt-dlp",
		"--dump-json",
		"--no-playlist",
		"--quiet",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return player.Track{}, fmt.Errorf("yt-dlp info failed: %w", err)
	}
	var r searchResult
	if err := json.Unmarshal(out, &r); err != nil {
		return player.Track{}, fmt.Errorf("failed to parse track info: %w", err)
	}
	artist := r.Channel
	if artist == "" {
		artist = r.Uploader
	}
	id := r.ID
	source := "youtube"
	if strings.Contains(url, "soundcloud.com") {
		source = "soundcloud"
		id = r.WebpageURL
		if id == "" {
			id = url
		}
	}
	return player.Track{
		ID:       id,
		Title:    r.Title,
		Artist:   cleanArtist(artist),
		Duration: formatDuration(r.Duration),
		Source:   source,
	}, nil
}

func Search(query string, limit int, source string) ([]player.Track, error) {
	if limit <= 0 {
		limit = 10
	}
	var searchQuery string
	if source == "soundcloud" {
		searchQuery = fmt.Sprintf("scsearch%d:%s", limit, query)
	} else {
		source = "youtube"
		searchQuery = fmt.Sprintf("ytsearch%d:%s music", limit, query)
	}
	cmd := exec.Command("yt-dlp",
		searchQuery,
		"--flat-playlist",
		"--dump-single-json",
		"--no-download",
		"--quiet",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp search failed: %w", err)
	}

	var result struct {
		Entries []searchResult `json:"entries"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	var tracks []player.Track
	for _, r := range result.Entries {
		artist := r.Channel
		if artist == "" {
			artist = r.Uploader
		}
		id := r.ID
		if source == "soundcloud" {
			// SoundCloud needs webpage URL for stream resolution
			id = r.WebpageURL
			if id == "" {
				id = r.URL
			}
		}
		tracks = append(tracks, player.Track{
			ID:       id,
			Title:    r.Title,
			Artist:   cleanArtist(artist),
			Duration: formatDuration(r.Duration),
			Source:   source,
		})
	}
	return tracks, nil
}

func cleanArtist(s string) string {
	s = strings.TrimSuffix(s, " - Topic")
	return s
}

func formatDuration(secs float64) string {
	total := int(secs)
	if total <= 0 {
		return "?"
	}
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
