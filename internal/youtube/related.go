package youtube

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/lucas/chunes/internal/player"
)

// ParseArtistTitle tries to extract artist and title from a YouTube video title.
// Many videos use "Artist - Title" format. Falls back to empty artist if no separator found.
func ParseArtistTitle(videoTitle, channelName string) (artist, title string) {
	// Try common separators: " - ", " — ", " – ", " | "
	for _, sep := range []string{" - ", " — ", " – ", " | "} {
		if idx := strings.Index(videoTitle, sep); idx > 0 {
			artist = strings.TrimSpace(videoTitle[:idx])
			title = strings.TrimSpace(videoTitle[idx+len(sep):])
			// Strip common suffixes from title
			for _, suffix := range []string{
				"(Official Video)", "(Official Music Video)", "(Official Audio)",
				"(Lyric Video)", "(Lyrics)", "(Visualizer)", "(Audio)",
				"[Official Video]", "[Official Music Video]", "[Official Audio]",
				"[Lyric Video]", "[Lyrics]", "[Visualizer]", "[Audio]",
			} {
				title = strings.TrimSpace(strings.TrimSuffix(title, suffix))
				// Case-insensitive check
				if len(title) > len(suffix) {
					lower := strings.ToLower(title)
					lowerSuffix := strings.ToLower(suffix)
					if strings.HasSuffix(lower, lowerSuffix) {
						title = strings.TrimSpace(title[:len(title)-len(suffix)])
					}
				}
			}
			return artist, title
		}
	}
	// No separator found — use channel name as artist, full title as title
	return cleanArtist(channelName), videoTitle
}

// GetVideoTags fetches the tags/keywords for a YouTube video.
func GetVideoTags(videoID string) ([]string, error) {
	cmd := exec.Command("yt-dlp",
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
		"--print", "%(tags)s",
		"--no-download",
		"--quiet",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" || raw == "NA" || raw == "None" {
		return nil, nil
	}
	// yt-dlp prints tags as a Python-style list: ['tag1', 'tag2', ...]
	// or sometimes comma-separated
	raw = strings.Trim(raw, "[]")
	var tags []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		t = strings.Trim(t, "'\"")
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags, nil
}

// SearchExact searches YouTube without appending "music" — for suggestion resolution
// where we already have a specific artist+title query.
func SearchExact(query string, limit int) ([]player.Track, error) {
	if limit <= 0 {
		limit = 5
	}
	searchQuery := fmt.Sprintf("ytsearch%d:%s", limit, query)
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
		Entries []struct {
			ID       string  `json:"id"`
			Title    string  `json:"title"`
			Channel  string  `json:"channel"`
			Uploader string  `json:"uploader"`
			Duration float64 `json:"duration"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	var tracks []player.Track
	for _, r := range result.Entries {
		artist := r.Channel
		if artist == "" {
			artist = r.Uploader
		}
		tracks = append(tracks, player.Track{
			ID:       r.ID,
			Title:    r.Title,
			Artist:   cleanArtist(artist),
			Duration: formatDuration(r.Duration),
			Source:   "youtube",
		})
	}
	return tracks, nil
}

// GetChannelURL returns the channel URL for a given YouTube video ID.
func GetChannelURL(videoID string) (string, error) {
	cmd := exec.Command("yt-dlp",
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
		"--print", "%(channel_url)s",
		"--no-download",
		"--quiet",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(out))
	if url == "" || url == "NA" || url == "None" {
		return "", fmt.Errorf("no channel url for %s", videoID)
	}
	return url, nil
}

// GetChannelVideos fetches recent videos from a YouTube channel URL.
func GetChannelVideos(channelURL string, limit int) ([]player.Track, error) {
	if limit <= 0 {
		limit = 15
	}
	cmd := exec.Command("yt-dlp",
		channelURL+"/videos",
		"--flat-playlist",
		"--print", "%(id)s\t%(title)s\t%(channel)s\t%(duration)s",
		"--no-download",
		"--quiet",
		"--playlist-end", fmt.Sprintf("%d", limit),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp channel videos failed: %w", err)
	}

	var tracks []player.Track
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		id, title, channel, durStr := parts[0], parts[1], parts[2], parts[3]
		dur, _ := strconv.ParseFloat(durStr, 64)
		tracks = append(tracks, player.Track{
			ID:         id,
			Title:      title,
			Artist:     cleanArtist(channel),
			Duration:   formatDuration(dur),
			Source:     "youtube",
			ChannelURL: channelURL,
		})
	}
	return tracks, nil
}

// GetRelated fetches YouTube's "Radio" mix playlist for a video ID,
// which contains algorithmically related tracks. Returns up to limit tracks,
// excluding the source video itself.
func GetRelated(videoID string, limit int) ([]player.Track, error) {
	if limit <= 0 {
		limit = 20
	}
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s&list=RD%s", videoID, videoID)
	cmd := exec.Command("yt-dlp",
		url,
		"--flat-playlist",
		"--print", "%(id)s\t%(title)s\t%(channel)s\t%(duration)s",
		"--no-download",
		"--quiet",
		"--playlist-end", fmt.Sprintf("%d", limit+1), // +1 because first entry is the source
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp related failed: %w", err)
	}

	var tracks []player.Track
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		id, title, channel, durStr := parts[0], parts[1], parts[2], parts[3]
		if id == videoID {
			continue // skip the source track
		}
		dur, _ := strconv.ParseFloat(durStr, 64)
		tracks = append(tracks, player.Track{
			ID:       id,
			Title:    title,
			Artist:   cleanArtist(channel),
			Duration: formatDuration(dur),
			Source:   "youtube",
		})
		if len(tracks) >= limit {
			break
		}
	}
	return tracks, nil
}
