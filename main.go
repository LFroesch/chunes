package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/ui"
)

func main() {
	// Check dependencies
	for _, dep := range []string{"mpv", "yt-dlp"} {
		if _, err := exec.LookPath(dep); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s not found in PATH. Please install it.\n", dep)
			os.Exit(1)
		}
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %s\n", err)
		os.Exit(1)
	}

	// Ensure download dir exists
	os.MkdirAll(cfg.DownloadDir, 0755)
	os.MkdirAll(config.Dir(), 0755)

	// Start two mpv instances for crossfading
	mpvA, err := player.NewMPV()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting mpv (A): %s\n", err)
		os.Exit(1)
	}
	defer mpvA.Close()

	mpvB, err := player.NewMPV()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting mpv (B): %s\n", err)
		os.Exit(1)
	}
	defer mpvB.Close()

	// Save config on exit
	defer cfg.Save()

	// Run TUI
	model := ui.NewModel(cfg, mpvA, mpvB)
	defer model.Cleanup()
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
