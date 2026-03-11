package download

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
)

type Progress struct {
	Track   player.Track
	Percent float64
	Done    bool
	Error   error
}

// LibraryEntry is a completed download stored on disk
type LibraryEntry struct {
	Track player.Track `json:"track"`
	Path  string       `json:"path"`
}

func libraryPath() string {
	return filepath.Join(config.Dir(), "downloads.json")
}

func LoadLibrary() ([]LibraryEntry, error) {
	data, err := os.ReadFile(libraryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []LibraryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func SaveLibrary(entries []LibraryEntry) error {
	dir := config.Dir()
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(libraryPath(), data, 0644)
}

func AddToLibrary(track player.Track, path string) error {
	entries, _ := LoadLibrary()
	// Dedupe by track ID
	for i, e := range entries {
		if e.Track.ID == track.ID {
			entries[i].Path = path
			return SaveLibrary(entries)
		}
	}
	entries = append(entries, LibraryEntry{Track: track, Path: path})
	return SaveLibrary(entries)
}

// ResolvedPath returns the actual file path for a downloaded track
func ResolvedPath(track player.Track, outputDir, format string) string {
	ext := format
	if ext == "" {
		ext = "mp3"
	}
	filename := sanitizeFilename(track.Title+" - "+track.Artist) + "." + ext
	return filepath.Join(outputDir, filename)
}

func RemoveFromLibrary(id string) error {
	entries, err := LoadLibrary()
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, e := range entries {
		if e.Track.ID != id {
			filtered = append(filtered, e)
		}
	}
	return SaveLibrary(filtered)
}

func DeleteFile(track player.Track, outputDir, format string) error {
	path := ResolvedPath(track, outputDir, format)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

func Download(track player.Track, outputDir, format string, progressCh chan<- Progress) {
	defer close(progressCh)

	ext := format
	if ext == "" {
		ext = "mp3"
	}
	filename := sanitizeFilename(track.Title + " - " + track.Artist)
	output := filepath.Join(outputDir, filename+".%(ext)s")

	url := track.ID
	if !strings.HasPrefix(url, "http") {
		url = fmt.Sprintf("https://www.youtube.com/watch?v=%s", track.ID)
	}
	args := []string{
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", ext,
		"--audio-quality", "0",
		"-o", output,
		"--newline",
		"--no-playlist",
		url,
	}

	cmd := exec.Command("yt-dlp", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		progressCh <- Progress{Track: track, Error: err}
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		progressCh <- Progress{Track: track, Error: err}
		return
	}

	if err := cmd.Start(); err != nil {
		progressCh <- Progress{Track: track, Error: err}
		return
	}

	// Parse progress from stdout
	pctRe := regexp.MustCompile(`(\d+\.?\d*)%`)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := pctRe.FindStringSubmatch(line); len(matches) > 1 {
			if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
				progressCh <- Progress{Track: track, Percent: pct}
			}
		}
	}

	// Drain stderr for error info
	var errOutput strings.Builder
	errScanner := bufio.NewScanner(stderr)
	for errScanner.Scan() {
		errOutput.WriteString(errScanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		progressCh <- Progress{Track: track, Error: fmt.Errorf("%w: %s", err, errOutput.String())}
		return
	}

	progressCh <- Progress{Track: track, Percent: 100, Done: true}
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
