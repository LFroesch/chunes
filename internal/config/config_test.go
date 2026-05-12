package config

import "testing"

func TestDefaultConfigIncludesRevampDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.RadioEnabled {
		t.Fatalf("expected radio to be enabled by default")
	}
	if cfg.RadioAutoFillCount != 1 {
		t.Fatalf("expected radio autofill count 1, got %d", cfg.RadioAutoFillCount)
	}
	if cfg.RadioPrefetchCount != 10 {
		t.Fatalf("expected radio prefetch count 10, got %d", cfg.RadioPrefetchCount)
	}
	if cfg.VisualizerStyle != "plasma" {
		t.Fatalf("expected default visualizer style plasma, got %q", cfg.VisualizerStyle)
	}
	if cfg.VisualizerIntensity != 2.2 {
		t.Fatalf("expected visualizer intensity 2.2, got %v", cfg.VisualizerIntensity)
	}
	if cfg.VisualizerBackend != "auto" {
		t.Fatalf("expected visualizer backend auto, got %q", cfg.VisualizerBackend)
	}
}

func TestNormalizeRepairsInvalidValues(t *testing.T) {
	cfg := &Config{
		Volume:              -5,
		CrossfadeSecs:       -1,
		RadioAutoFillCount:  0,
		RadioPrefetchCount:  0,
		VisualizerIntensity: 0,
		VisualizerBackend:   "invalid",
	}

	cfg.Normalize()

	if cfg.Volume != 0 {
		t.Fatalf("expected volume clamp to 0, got %d", cfg.Volume)
	}
	if cfg.CrossfadeSecs != 0 {
		t.Fatalf("expected crossfade clamp to 0, got %d", cfg.CrossfadeSecs)
	}
	if cfg.RadioAutoFillCount != 1 {
		t.Fatalf("expected radio autofill normalize to 1, got %d", cfg.RadioAutoFillCount)
	}
	if cfg.RadioPrefetchCount != 10 {
		t.Fatalf("expected radio prefetch normalize to 10, got %d", cfg.RadioPrefetchCount)
	}
	if cfg.VisualizerIntensity != 2.2 {
		t.Fatalf("expected visualizer intensity normalize to 2.2, got %v", cfg.VisualizerIntensity)
	}
	if cfg.VisualizerBackend != "auto" {
		t.Fatalf("expected visualizer backend normalize to auto, got %q", cfg.VisualizerBackend)
	}
}
