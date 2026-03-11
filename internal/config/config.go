package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Volume        int    `json:"volume"`
	DownloadDir   string `json:"download_dir"`
	AudioFormat   string `json:"audio_format"`
	CrossfadeSecs int    `json:"crossfade_secs"`
	LastFMKey     string `json:"lastfm_api_key,omitempty"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Volume:        70,
		DownloadDir:   filepath.Join(home, "Music", "chunes"),
		AudioFormat:   "mp3",
		CrossfadeSecs: 8,
	}
}

func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "chunes")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "chunes")
}

func configPath() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save() error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}
