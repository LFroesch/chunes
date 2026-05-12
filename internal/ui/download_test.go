package ui

import (
	"testing"

	"github.com/lucas/chunes/internal/player"
)

func TestDownloadBeginResetsFailedItemForRetry(t *testing.T) {
	track := player.Track{ID: "abc123", Title: "Song", Artist: "Artist"}
	model := downloadModel{
		items: []downloadItem{
			{
				track:   track,
				percent: 42,
				done:    true,
				err:     assertErr("boom"),
				path:    "/tmp/song.mp3",
			},
		},
	}

	model.begin(track)

	if len(model.items) != 1 {
		t.Fatalf("expected existing item to be reused, got %d items", len(model.items))
	}
	item := model.items[0]
	if item.percent != 0 {
		t.Fatalf("expected percent reset, got %v", item.percent)
	}
	if item.done {
		t.Fatalf("expected done reset")
	}
	if item.err != nil {
		t.Fatalf("expected err reset, got %v", item.err)
	}
	if item.path != "" {
		t.Fatalf("expected path reset, got %q", item.path)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
