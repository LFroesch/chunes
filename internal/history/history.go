package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
)

type Entry struct {
	Track     player.Track `json:"track"`
	PlayedAt  time.Time    `json:"played_at"`
	PlayCount int          `json:"play_count"`
	Rating    int          `json:"rating"` // 0=unrated, 1-5 stars
}

type History struct {
	Entries []Entry `json:"entries"`
}

func historyPath() string {
	return filepath.Join(config.Dir(), "history.json")
}

func Load() (*History, error) {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, err
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func (h *History) Save() error {
	dir := config.Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath(), data, 0644)
}

func (h *History) Add(t player.Track) {
	// Find existing entry to preserve play count + rating
	var oldCount int
	var oldRating int
	filtered := h.Entries[:0]
	for _, e := range h.Entries {
		if e.Track.ID == t.ID {
			oldCount = e.PlayCount
			oldRating = e.Rating
		} else {
			filtered = append(filtered, e)
		}
	}
	h.Entries = append(filtered, Entry{
		Track:     t,
		PlayedAt:  time.Now(),
		PlayCount: oldCount + 1,
		Rating:    oldRating,
	})
	// Keep last 500 entries
	if len(h.Entries) > 500 {
		h.Entries = h.Entries[len(h.Entries)-500:]
	}
}

// Dedupe removes all duplicate entries, keeping only the most recent per track.
// Call this once on load to clean up existing history.
func (h *History) Dedupe() {
	seen := make(map[string]bool)
	// Walk backwards to keep most recent
	unique := make([]Entry, 0, len(h.Entries))
	for i := len(h.Entries) - 1; i >= 0; i-- {
		id := h.Entries[i].Track.ID
		if !seen[id] {
			seen[id] = true
			unique = append(unique, h.Entries[i])
		}
	}
	// Reverse back to chronological order
	for i, j := 0, len(unique)-1; i < j; i, j = i+1, j-1 {
		unique[i], unique[j] = unique[j], unique[i]
	}
	h.Entries = unique
}

// RatingFor returns (rating, playCount) for a track ID.
func (h *History) RatingFor(id string) (int, int) {
	for _, e := range h.Entries {
		if e.Track.ID == id {
			return e.Rating, e.PlayCount
		}
	}
	return 0, 0
}

// SetRating sets the rating for a track by ID.
func (h *History) SetRating(id string, rating int) {
	for i := range h.Entries {
		if h.Entries[i].Track.ID == id {
			h.Entries[i].Rating = rating
			return
		}
	}
}

func (h *History) Delete(id string) {
	filtered := h.Entries[:0]
	for _, e := range h.Entries {
		if e.Track.ID != id {
			filtered = append(filtered, e)
		}
	}
	h.Entries = filtered
}

func (h *History) Recent(n int) []Entry {
	if n <= 0 || len(h.Entries) == 0 {
		return nil
	}
	start := len(h.Entries) - n
	if start < 0 {
		start = 0
	}
	// Return in reverse chronological order
	entries := make([]Entry, 0, n)
	for i := len(h.Entries) - 1; i >= start; i-- {
		entries = append(entries, h.Entries[i])
	}
	return entries
}
