package history

import (
	"testing"

	"github.com/lucas/chunes/internal/player"
)

func TestPreviousUniqueTrack(t *testing.T) {
	h := &History{
		Entries: []Entry{
			{Track: player.Track{ID: "a", Title: "A"}},
			{Track: player.Track{ID: "b", Title: "B"}},
			{Track: player.Track{ID: "c", Title: "C"}},
		},
	}

	prev := h.PreviousUniqueTrack("c")
	if prev == nil || prev.ID != "b" {
		t.Fatalf("PreviousUniqueTrack(c) = %#v, want track b", prev)
	}

	if got := h.PreviousUniqueTrack("a"); got != nil {
		t.Fatalf("PreviousUniqueTrack(a) = %#v, want nil", got)
	}

	if got := h.PreviousUniqueTrack("missing"); got != nil {
		t.Fatalf("PreviousUniqueTrack(missing) = %#v, want nil", got)
	}
}
