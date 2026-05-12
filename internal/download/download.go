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
	"sync"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/youtube"
)

type Progress struct {
	Track   player.Track
	Percent float64
	Done    bool
	Error   error
	Path    string
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
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

func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "source", "best", "original", "keep":
		return "source"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func outputBase(track player.Track, outputDir string) string {
	filename := sanitizeFilename(track.Title + " - " + track.Artist)
	return filepath.Join(outputDir, filename)
}

// ResolvedPath returns the expected file path for extension-stable downloads.
func ResolvedPath(track player.Track, outputDir, format string) string {
	ext := normalizeFormat(format)
	if ext == "source" {
		return ""
	}
	return outputBase(track, outputDir) + "." + ext
}

// FindDownloadedPath returns the stored or discovered file path for a downloaded track.
func FindDownloadedPath(track player.Track, outputDir, format string) string {
	entries, err := LoadLibrary()
	if err == nil {
		for _, e := range entries {
			if e.Track.ID == track.ID && e.Path != "" {
				if _, statErr := os.Stat(e.Path); statErr == nil {
					return e.Path
				}
			}
		}
	}

	if path := ResolvedPath(track, outputDir, format); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	matches, err := filepath.Glob(outputBase(track, outputDir) + ".*")
	if err != nil {
		return ""
	}
	var newest string
	var newestModTime int64
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil || info.IsDir() {
			continue
		}
		if isPartialDownloadPath(match) {
			continue
		}
		if newest == "" || info.ModTime().UnixNano() > newestModTime {
			newest = match
			newestModTime = info.ModTime().UnixNano()
		}
	}
	return newest
}

func isPartialDownloadPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".part") || strings.HasSuffix(lower, ".ytdl")
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
	path := FindDownloadedPath(track, outputDir, format)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

func Download(track player.Track, outputDir, format string, progressCh chan<- Progress) {
	defer close(progressCh)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		progressCh <- Progress{Track: track, Error: fmt.Errorf("create download dir: %w", err)}
		return
	}

	ext := normalizeFormat(format)
	if ext != "source" {
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			progressCh <- Progress{
				Track: track,
				Error: fmt.Errorf("ffmpeg not found in PATH; transcoded downloads require ffmpeg"),
			}
			return
		}
	}
	output := outputBase(track, outputDir) + ".%(ext)s"

	url := youtube.PlayableURL(track)
	if url == "" {
		progressCh <- Progress{Track: track, Error: fmt.Errorf("empty track URL")}
		return
	}
	args := []string{
		"-f", "bestaudio",
		"-o", output,
		"--newline",
		"--no-playlist",
		url,
	}
	if ext != "source" {
		args = append(args[:2],
			append([]string{"--extract-audio", "--audio-format", ext, "--audio-quality", "0"}, args[2:]...)...,
		)
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

	pctRe := regexp.MustCompile(`(\d+\.?\d*)%`)
	var errOutput strings.Builder
	var wg sync.WaitGroup
	parseStream := func(scanner *bufio.Scanner, captureErrors bool) {
		defer wg.Done()
		for scanner.Scan() {
			line := scanner.Text()
			if matches := pctRe.FindStringSubmatch(line); len(matches) > 1 {
				if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
					progressCh <- Progress{Track: track, Percent: pct}
				}
			}
			if captureErrors {
				if errOutput.Len() > 0 {
					errOutput.WriteString(" | ")
				}
				errOutput.WriteString(strings.TrimSpace(line))
			}
		}
	}

	wg.Add(2)
	go parseStream(bufio.NewScanner(stdout), false)
	go parseStream(bufio.NewScanner(stderr), true)

	if err := cmd.Wait(); err != nil {
		wg.Wait()
		progressCh <- Progress{Track: track, Error: explainDownloadFailure(err, errOutput.String())}
		return
	}
	wg.Wait()

	path := FindDownloadedPath(track, outputDir, ext)
	progressCh <- Progress{Track: track, Percent: 100, Done: true, Path: path}
}

func explainDownloadFailure(err error, details string) error {
	details = strings.TrimSpace(details)
	if strings.Contains(strings.ToLower(details), "no supported javascript runtime") {
		return fmt.Errorf("yt-dlp needs a JavaScript runtime for this download; install nodejs or deno and retry")
	}
	if details == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, details)
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
