package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/ui"
)

type dep struct {
	name, role string
	required   bool
}

func main() {
	deps := []dep{
		{"mpv", "audio playback", true},
		{"yt-dlp", "YouTube/SoundCloud stream resolution", true},
		{"parec", "spectrum visualizer (PulseAudio)", false},
	}
	var missing []dep
	for _, d := range deps {
		if _, err := exec.LookPath(d.name); err != nil {
			missing = append(missing, d)
		}
	}
	hasRequired := false
	for _, d := range missing {
		if d.required {
			hasRequired = true
			break
		}
	}
	if hasRequired {
		fmt.Fprintln(os.Stderr, "chunes: missing required dependencies")
		fmt.Fprintln(os.Stderr)
		for _, d := range missing {
			tag := "required"
			if !d.required {
				tag = "optional"
			}
			fmt.Fprintf(os.Stderr, "  • %-8s (%s) — %s\n", d.name, tag, d.role)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Install:")
		fmt.Fprintln(os.Stderr, "  "+installHint(missing))
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Or run the bundled installer: ./install.sh")
		os.Exit(1)
	}
	for _, d := range missing {
		fmt.Fprintf(os.Stderr, "note: %s not found — %s will be disabled.\n", d.name, d.role)
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

func installHint(missing []dep) string {
	names := make([]string, 0, len(missing))
	for _, d := range missing {
		names = append(names, d.name)
	}
	pkgs := strings.Join(names, " ")
	if runtime.GOOS == "darwin" {
		return "brew install " + pkgs
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return "sudo apt install " + pkgs
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "sudo dnf install " + pkgs
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return "sudo pacman -S " + pkgs
	}
	return "install with your package manager: " + pkgs
}
