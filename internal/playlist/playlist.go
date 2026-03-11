package playlist

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
)

type Playlist struct {
	Name   string         `json:"name"`
	Tracks []player.Track `json:"tracks"`
}

func Dir() string {
	return filepath.Join(config.Dir(), "playlists")
}

// safeName validates a playlist name to prevent path traversal.
func safeName(name string) error {
	if name == "" {
		return fmt.Errorf("playlist name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("invalid playlist name: %q", name)
	}
	return nil
}

func List() ([]Playlist, error) {
	dir := Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var playlists []Playlist
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p, err := Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		playlists = append(playlists, *p)
	}
	return playlists, nil
}

func Load(name string) (*Playlist, error) {
	if err := safeName(name); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(Dir(), name+".json"))
	if err != nil {
		return nil, err
	}
	var p Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Playlist) Save() error {
	if err := safeName(p.Name); err != nil {
		return err
	}
	dir := Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, p.Name+".json"), data, 0644)
}

func Delete(name string) error {
	if err := safeName(name); err != nil {
		return err
	}
	path := filepath.Join(Dir(), name+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("playlist %q not found", name)
	}
	return os.Remove(path)
}

// ContainsTrack returns true if the playlist already has a track with this ID.
func (p *Playlist) ContainsTrack(id string) bool {
	for _, t := range p.Tracks {
		if t.ID == id {
			return true
		}
	}
	return false
}

func (p *Playlist) AddTrack(t player.Track) {
	p.Tracks = append(p.Tracks, t)
}

func (p *Playlist) RemoveTrack(index int) {
	if index < 0 || index >= len(p.Tracks) {
		return
	}
	p.Tracks = append(p.Tracks[:index], p.Tracks[index+1:]...)
}

func Rename(oldName, newName string) error {
	if err := safeName(newName); err != nil {
		return err
	}
	p, err := Load(oldName)
	if err != nil {
		return err
	}
	if err := Delete(oldName); err != nil {
		return err
	}
	p.Name = newName
	return p.Save()
}

func (p *Playlist) Shuffle() {
	// Fisher-Yates shuffle
	for i := len(p.Tracks) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		p.Tracks[i], p.Tracks[j] = p.Tracks[j], p.Tracks[i]
	}
}

func (p *Playlist) MoveTrack(from, to int) {
	if from < 0 || from >= len(p.Tracks) || to < 0 || to >= len(p.Tracks) || from == to {
		return
	}
	track := p.Tracks[from]
	p.Tracks = append(p.Tracks[:from], p.Tracks[from+1:]...)
	p.Tracks = append(p.Tracks[:to], append([]player.Track{track}, p.Tracks[to:]...)...)
}
