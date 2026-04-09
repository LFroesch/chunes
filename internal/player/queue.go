package player

import (
	"math/rand"
	"sync"
)

type Track struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Duration   string `json:"duration"`
	URL        string `json:"url,omitempty"`        // stream URL (resolved lazily)
	Source     string `json:"source,omitempty"`     // "youtube" or "soundcloud"
	ChannelURL string `json:"channel_url,omitempty"` // YouTube channel URL for same-channel fetch
}

type Queue struct {
	tracks  []Track
	shuffle bool
	repeat  RepeatMode
	mu      sync.RWMutex
}

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

func NewQueue() *Queue {
	return &Queue{}
}

// Contains returns true if the queue already has a track with this ID.
func (q *Queue) Contains(id string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, t := range q.tracks {
		if t.ID == id {
			return true
		}
	}
	return false
}

func (q *Queue) Add(t Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = append(q.tracks, t)
}

// AddNext inserts a track at the front of the queue (plays next).
func (q *Queue) AddNext(t Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = append([]Track{t}, q.tracks...)
}

func (q *Queue) Remove(index int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index < 0 || index >= len(q.tracks) {
		return
	}
	q.tracks = append(q.tracks[:index], q.tracks[index+1:]...)
}

func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = nil
}

// Pop removes and returns the next track to play from the front.
// RepeatOne returns the front track without removing it.
// RepeatAll re-appends the track to the end after popping.
func (q *Queue) Pop() *Track {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tracks) == 0 {
		return nil
	}
	t := q.tracks[0]

	if q.repeat == RepeatOne {
		return &t
	}

	// Remove front — copy to avoid leaking backing array
	newTracks := make([]Track, len(q.tracks)-1)
	copy(newTracks, q.tracks[1:])
	q.tracks = newTracks

	if q.repeat == RepeatAll {
		q.tracks = append(q.tracks, t)
	}

	return &t
}

// Peek returns the next track without removing it.
func (q *Queue) Peek() *Track {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.tracks) == 0 {
		return nil
	}
	t := q.tracks[0]
	return &t
}

func (q *Queue) Tracks() []Track {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]Track, len(q.tracks))
	copy(out, q.tracks)
	return out
}

func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tracks)
}

func (q *Queue) ToggleShuffle() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.shuffle = !q.shuffle
	if q.shuffle {
		// Fisher-Yates shuffle the tracks in-place
		for i := len(q.tracks) - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			q.tracks[i], q.tracks[j] = q.tracks[j], q.tracks[i]
		}
	}
	return q.shuffle
}

func (q *Queue) CycleRepeat() RepeatMode {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.repeat = (q.repeat + 1) % 3
	return q.repeat
}

func (q *Queue) Shuffle() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.shuffle
}

func (q *Queue) Repeat() RepeatMode {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.repeat
}

func (q *Queue) Move(from, to int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if from < 0 || from >= len(q.tracks) || to < 0 || to >= len(q.tracks) || from == to {
		return
	}
	track := q.tracks[from]
	// Build new slice to avoid aliasing bugs
	newTracks := make([]Track, 0, len(q.tracks))
	for i, t := range q.tracks {
		if i == from {
			continue
		}
		if i == to && from > to {
			newTracks = append(newTracks, track)
		}
		newTracks = append(newTracks, t)
		if i == to && from < to {
			newTracks = append(newTracks, track)
		}
	}
	q.tracks = newTracks
}
