package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Volume              int     `json:"volume"`
	DownloadDir         string  `json:"download_dir"`
	AudioFormat         string  `json:"audio_format"`
	CrossfadeSecs       int     `json:"crossfade_secs"`
	LastFMKey           string  `json:"lastfm_api_key,omitempty"`
	RadioEnabled        bool    `json:"radio_enabled"`
	RadioAutoFillCount  int     `json:"radio_autofill_count"`
	RadioPrefetchCount  int     `json:"radio_prefetch_count"`
	VisualizerStyle     string  `json:"visualizer_style"`
	VisualizerAutoCycle bool    `json:"visualizer_autocycle"`
	VisualizerIntensity float64 `json:"visualizer_intensity"`
	VisualizerBackend   string  `json:"visualizer_backend"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Volume:              70,
		DownloadDir:         filepath.Join(home, "Music", "chunes"),
		AudioFormat:         "source",
		CrossfadeSecs:       8,
		RadioEnabled:        true,
		RadioAutoFillCount:  1,
		RadioPrefetchCount:  10,
		VisualizerStyle:     "plasma",
		VisualizerAutoCycle: false,
		VisualizerIntensity: 2.2,
		VisualizerBackend:   "auto",
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
	cfg.Normalize()
	return cfg, nil
}

func (c *Config) Save() error {
	c.Normalize()
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

func (c *Config) Normalize() {
	if c.Volume < 0 {
		c.Volume = 0
	}
	if c.CrossfadeSecs < 0 {
		c.CrossfadeSecs = 0
	}
	if c.RadioAutoFillCount <= 0 {
		c.RadioAutoFillCount = 1
	}
	if c.RadioPrefetchCount <= 0 {
		c.RadioPrefetchCount = 10
	}
	if c.VisualizerIntensity <= 0 {
		c.VisualizerIntensity = 2.2
	}
	if c.VisualizerStyle == "" {
		c.VisualizerStyle = "plasma"
	}
	switch c.VisualizerBackend {
	case "", "auto", "pulse-monitor", "mpv-rms":
		if c.VisualizerBackend == "" {
			c.VisualizerBackend = "auto"
		}
	default:
		c.VisualizerBackend = "auto"
	}
}
