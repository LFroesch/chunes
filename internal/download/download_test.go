package download

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lucas/chunes/internal/player"
)

func TestResolvedPathEmptyForSourceFormat(t *testing.T) {
	track := player.Track{Title: "Song", Artist: "Artist"}

	if got := ResolvedPath(track, "/tmp/downloads", "source"); got != "" {
		t.Fatalf("expected empty path for source format, got %q", got)
	}
}

func TestFindDownloadedPathUsesStoredLibraryPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))

	want := filepath.Join(tmp, "stored.opus")
	if err := os.WriteFile(want, []byte("audio"), 0644); err != nil {
		t.Fatalf("write stored file: %v", err)
	}

	track := player.Track{ID: "abc123", Title: "Song", Artist: "Artist"}
	if err := AddToLibrary(track, want); err != nil {
		t.Fatalf("add to library: %v", err)
	}

	got := FindDownloadedPath(track, filepath.Join(tmp, "downloads"), "source")
	if got != want {
		t.Fatalf("expected stored path %q, got %q", want, got)
	}
}

func TestFindDownloadedPathFallsBackToNewestMatchingFile(t *testing.T) {
	tmp := t.TempDir()
	track := player.Track{ID: "abc123", Title: "Song", Artist: "Artist"}

	older := filepath.Join(tmp, "Song - Artist.webm")
	newer := filepath.Join(tmp, "Song - Artist.opus")
	partial := filepath.Join(tmp, "Song - Artist.opus.part")
	if err := os.WriteFile(older, []byte("old"), 0644); err != nil {
		t.Fatalf("write older file: %v", err)
	}
	if err := os.WriteFile(newer, []byte("new"), 0644); err != nil {
		t.Fatalf("write newer file: %v", err)
	}
	if err := os.WriteFile(partial, []byte("partial"), 0644); err != nil {
		t.Fatalf("write partial file: %v", err)
	}

	if err := os.Chtimes(older, olderTime, olderTime); err != nil {
		t.Fatalf("set older times: %v", err)
	}
	if err := os.Chtimes(newer, testTime, testTime); err != nil {
		t.Fatalf("set newer times: %v", err)
	}
	if err := os.Chtimes(partial, partialTime, partialTime); err != nil {
		t.Fatalf("set partial times: %v", err)
	}

	got := FindDownloadedPath(track, tmp, "source")
	if got != newer {
		t.Fatalf("expected newest match %q, got %q", newer, got)
	}
}

func TestExplainDownloadFailureJavascriptRuntime(t *testing.T) {
	err := explainDownloadFailure(errors.New("exit status 1"), "ERROR: No supported JavaScript runtime")

	if got := err.Error(); got != "yt-dlp needs a JavaScript runtime for this download; install nodejs or deno and retry" {
		t.Fatalf("unexpected error: %q", got)
	}
}

var (
	olderTime   = time.Unix(1_600_000_000, 0)
	testTime    = time.Unix(1_700_000_000, 0)
	partialTime = time.Unix(1_800_000_000, 0)
)
