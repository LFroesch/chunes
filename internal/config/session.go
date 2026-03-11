package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SessionTrack is a minimal track representation for session persistence.
// Mirrors player.Track but avoids an import cycle.
type SessionTrack struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration string `json:"duration"`
	Source   string `json:"source,omitempty"`
}

type Session struct {
	NowPlaying *SessionTrack  `json:"now_playing,omitempty"`
	Position   float64        `json:"position"`
	Queue      []SessionTrack `json:"queue,omitempty"`
	Shuffle    bool           `json:"shuffle"`
	Repeat     int            `json:"repeat"`
}

func sessionPath() string {
	return filepath.Join(Dir(), "session.json")
}

func LoadSession() (*Session, error) {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func SaveSession(s *Session) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionPath(), data, 0600)
}
