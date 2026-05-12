package youtube

import (
	"testing"

	"github.com/lucas/chunes/internal/player"
)

func TestPlayableURLPrefersExplicitURL(t *testing.T) {
	track := player.Track{
		ID:  "abc123",
		URL: "https://www.youtube.com/watch?v=abc123",
	}

	if got := PlayableURL(track); got != track.URL {
		t.Fatalf("expected explicit URL, got %q", got)
	}
}

func TestNormalizeReferenceBuildsYouTubeURL(t *testing.T) {
	got := normalizeReference("abc123")
	want := "https://www.youtube.com/watch?v=abc123"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSelectStreamURLPrefersAudioURL(t *testing.T) {
	out := "https://example.com/video?mime=video%2Fmp4\nhttps://example.com/audio?mime=audio%2Fwebm\n"

	got := selectStreamURL(out)

	if got != "https://example.com/audio?mime=audio%2Fwebm" {
		t.Fatalf("expected audio URL, got %q", got)
	}
}

func TestSelectStreamURLFallsBackToLastLine(t *testing.T) {
	out := "https://example.com/first\nhttps://example.com/second\n"

	got := selectStreamURL(out)

	if got != "https://example.com/second" {
		t.Fatalf("expected last URL, got %q", got)
	}
}
