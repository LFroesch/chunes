package ui

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/download"
	"github.com/lucas/chunes/internal/history"
	"github.com/lucas/chunes/internal/lastfm"
	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/playlist"
	"github.com/lucas/chunes/internal/youtube"
)

type View int

const (
	ViewSearch View = iota
	ViewNowPlaying
	ViewQueue
	ViewPlaylists
	ViewHistory
	ViewSuggestions
	ViewDownloads
)

var viewNames = []string{"Search", "Playing", "Queue", "Playlists", "History", "Suggest", "Downloads"}

type tickMsg time.Time
type streamURLMsg struct {
	track player.Track
	url   string
	err   error
}
type playlistSaveMsg struct {
	name string
	err  error
}

type crossfadePreloadMsg struct {
	track player.Track
	url   string
	err   error
}

type seekDebounceMsg struct {
	seqNo int
}

type seekDoneMsg struct{}

type sessionResumeMsg struct {
	track player.Track
	url   string
	pos   float64
	err   error
}

type sessionSeekMsg struct {
	pos float64
}

type Model struct {
	cfg        *config.Config
	mpvA       *player.MPV
	mpvB       *player.MPV
	queue      *player.Queue
	history    *history.History
	width      int
	height     int
	view       View
	showHelp   bool
	helpScroll int
	status     string
	err        string

	search      searchModel
	suggestions suggestionsModel
	queueView   queueModel
	playlists   playlistModel
	histView    historyModel
	downloads   downloadModel

	// Last.fm client for suggestions
	lastfm *lastfm.Client

	// Playlist picker overlay
	picker      pickerModel
	pickerTrack *player.Track

	// Confirmation overlay
	confirm confirmModel

	// Visualizer
	vizStyle      int // 0=bars, 1=lissajous, etc.
	vizBands      [vizBandCount]float64
	vizTick       int
	vizBandLevels [player.BandCount]float64 // real per-band frequency levels
	spectrum      *player.Spectrum          // PulseAudio FFT analyzer (nil if unavailable)
	vizBoost      float64                   // energy multiplier (default 2.5, adjustable with [/])
	vizAutoCycle  bool                      // auto-rotate viz styles
	vizCycleTick  int                       // ticks since last auto-cycle

	// Status auto-clear
	statusAt time.Time

	// Currently playing track (persists even if removed from queue)
	nowPlaying *player.Track

	// Seek debounce — accumulate rapid seeks and send one combined command
	pendingSeek float64
	seekSeqNo   int
	lastSeekAt  time.Time // cooldown: minimum gap between actual seek commands
	seeking     bool      // true while a seek is in-flight

	// Grace period after Play() — don't check IsIdle() until mpv has time to start
	playStarted time.Time

	// Session resume — position to seek to after stream loads
	resumePos      float64
	resumeCanceled bool // set when user plays a track before resume completes

	// Track ID we're currently loading — used to discard stale streamURLMsg
	loadingTrackID string

	// Crossfade state
	activeIsB      bool // false=A is active, true=B is active
	crossfading    bool
	crossfadeStart time.Time
	preloadedURL   string
	preloadedTrack *player.Track
	preloading     bool
}

func NewModel(cfg *config.Config, mpvA, mpvB *player.MPV) Model {
	hist, _ := history.Load()
	if hist == nil {
		hist = &history.History{}
	}
	hist.Dedupe()
	hist.Save()
	var lfm *lastfm.Client
	if cfg.LastFMKey != "" {
		lfm = lastfm.NewClient(cfg.LastFMKey)
	}
	m := Model{
		cfg:         cfg,
		mpvA:        mpvA,
		mpvB:        mpvB,
		queue:       player.NewQueue(),
		history:     hist,
		view:        ViewSearch,
		search:      newSearchModel(),
		suggestions: newSuggestionsModel(),
		queueView:   newQueueModel(),
		playlists:   newPlaylistModel(),
		histView:    newHistoryModel(),
		downloads:   newDownloadModel(cfg.DownloadDir, cfg.AudioFormat),
		picker:      newPickerModel(),
		lastfm:      lfm,
		spectrum:    player.NewSpectrum(), // nil if parec unavailable
		vizBoost:    2.5,
	}

	// Restore session state (queue, shuffle, repeat)
	if sess, err := config.LoadSession(); err == nil {
		for _, st := range sess.Queue {
			m.queue.Add(player.Track{
				ID: st.ID, Title: st.Title, Artist: st.Artist,
				Duration: st.Duration, Source: st.Source,
			})
		}
		if sess.Shuffle {
			m.queue.ToggleShuffle()
		}
		for i := 0; i < sess.Repeat; i++ {
			m.queue.CycleRepeat()
		}
		if sess.NowPlaying != nil {
			t := player.Track{
				ID:       sess.NowPlaying.ID,
				Title:    sess.NowPlaying.Title,
				Artist:   sess.NowPlaying.Artist,
				Duration: sess.NowPlaying.Duration,
				Source:   sess.NowPlaying.Source,
			}
			m.nowPlaying = &t
			m.resumePos = sess.Position
		}
	}

	return m
}

func (m *Model) activeMPV() *player.MPV {
	if m.activeIsB {
		return m.mpvB
	}
	return m.mpvA
}

func (m *Model) inactiveMPV() *player.MPV {
	if m.activeIsB {
		return m.mpvA
	}
	return m.mpvB
}

func (m Model) contentHeight() int {
	chrome := 11
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}

// ensureAllVisible adjusts scroll positions so cursors stay visible.
// Call after any cursor movement or window resize.
func (m *Model) ensureAllVisible() {
	ch := m.contentHeight()
	vis := ch - 5 // most views use 4 lines of chrome; search uses 5
	if vis < 1 {
		vis = 1
	}
	m.search.ensureVisible(vis)
	m.suggestions.ensureVisible(vis + 1)
	m.queueView.ensureVisible(vis+1, m.queue.Len())
	m.histView.ensureVisible(vis + 1)
	m.downloads.ensureVisible(vis + 1)
	m.playlists.ensureVisible(vis + 1)
}

// Cleanup releases resources owned by the model (spectrum analyzer, etc.)
func (m *Model) Cleanup() {
	if m.spectrum != nil {
		m.spectrum.Close()
	}
}

// SaveSession persists the current playback state so it can be restored on next launch.
func (m *Model) SaveSession() {
	sess := &config.Session{}
	if m.nowPlaying != nil {
		sess.NowPlaying = &config.SessionTrack{
			ID:       m.nowPlaying.ID,
			Title:    m.nowPlaying.Title,
			Artist:   m.nowPlaying.Artist,
			Duration: m.nowPlaying.Duration,
			Source:   m.nowPlaying.Source,
		}
		sess.Position = m.activeMPV().CachedPosition
	}
	tracks := m.queue.Tracks()
	for _, t := range tracks {
		sess.Queue = append(sess.Queue, config.SessionTrack{
			ID:       t.ID,
			Title:    t.Title,
			Artist:   t.Artist,
			Duration: t.Duration,
			Source:   t.Source,
		})
	}
	sess.Shuffle = m.queue.Shuffle()
	sess.Repeat = int(m.queue.Repeat())
	config.SaveSession(sess)
}

func (m *Model) refreshHistory() {
	m.histView.entries = m.history.Entries
	m.histView.allEntries = m.history.Entries
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(), loadPlaylists()}
	if m.nowPlaying != nil && m.resumePos > 0 {
		t := *m.nowPlaying
		pos := m.resumePos
		cmds = append(cmds, m.resumeSession(t, pos))
	}
	return tea.Batch(cmds...)
}

func (m *Model) resumeSession(t player.Track, pos float64) tea.Cmd {
	// Check local file first, otherwise resolve stream URL
	if path := m.localFilePath(t); path != "" {
		return func() tea.Msg {
			return sessionResumeMsg{track: t, url: path, pos: pos}
		}
	}
	return func() tea.Msg {
		url, err := youtube.GetStreamURL(t.ID)
		return sessionResumeMsg{track: t, url: url, pos: pos, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func getStreamURL(t player.Track) tea.Cmd {
	return func() tea.Msg {
		url, err := youtube.GetStreamURL(t.ID)
		return streamURLMsg{track: t, url: url, err: err}
	}
}

// inputActive returns true when a text input has focus
func (m Model) inputActive() bool {
	return m.search.focused || m.playlists.creating || m.playlists.renaming || m.picker.active
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureAllVisible()
		return m, nil

	case tickMsg:
		// Update spinner
		m.search.spinTick++

		// Poll mpv properties (caches position/duration/idle so View() never blocks)
		if m.activeMPV().Playing {
			m.activeMPV().PollProperties()
		}

		// Update visualizer with per-band frequency data
		m.vizTick++
		var vizRMS float64
		if m.activeMPV().Playing && !m.activeMPV().Paused {
			if m.spectrum != nil {
				m.vizBandLevels = m.spectrum.GetBandLevels()
			}
			vizRMS = m.activeMPV().GetRMS()
		}
		updateVizBands(&m.vizBands, m.vizBandLevels, vizRMS, m.vizTick, m.activeMPV().Playing, m.activeMPV().Paused, m.vizBoost)

		// Auto-cycle visualizer styles
		if m.vizAutoCycle && m.activeMPV().Playing && !m.activeMPV().Paused {
			m.vizCycleTick++
			if m.vizCycleTick >= 100 { // ~10 seconds at 100ms tick
				m.vizCycleTick = 0
				m.vizStyle = (m.vizStyle + 1) % len(vizStyleNames)
			}
		}

		// Auto-clear status after 3 seconds
		if m.status != "" {
			if m.statusAt.IsZero() {
				m.statusAt = time.Now()
			} else if time.Since(m.statusAt) > 3*time.Second {
				m.status = ""
				m.statusAt = time.Time{}
			}
		} else {
			m.statusAt = time.Time{}
		}

		// Safety timeout: clear stuck loadingTrackID after 15s
		if m.loadingTrackID != "" && time.Since(m.playStarted) > 15*time.Second {
			m.loadingTrackID = ""
		}

		// Crossfade logic
		if m.activeMPV().Playing && !m.activeMPV().Paused && !m.crossfading {
			position := m.activeMPV().CachedPosition
			duration := m.activeMPV().CachedDuration
			remaining := duration - position

			// Preload next track URL when ~15s remain
			if duration > 0 && remaining < 15 && remaining > float64(m.cfg.CrossfadeSecs) && !m.preloading && m.preloadedURL == "" {
				if next := m.queue.Peek(); next != nil {
					m.preloading = true
					m.preloadedTrack = next
					return m, tea.Batch(tickCmd(), m.preloadNextURL(*next))
				}
			}

			// Start crossfade when remaining time <= crossfadeSecs
			if duration > 0 && remaining <= float64(m.cfg.CrossfadeSecs) && remaining > 0 && m.preloadedURL != "" {
				// Play on inactive mpv at volume 0
				inactive := m.inactiveMPV()
				if err := inactive.Play(m.preloadedURL); err == nil {
					inactive.SetVolume(0)
					m.crossfading = true
					m.crossfadeStart = time.Now()
				}
				m.preloadedURL = ""
			}
		}

		// Volume ramp during crossfade
		if m.crossfading {
			elapsed := time.Since(m.crossfadeStart).Seconds()
			cfDur := float64(m.cfg.CrossfadeSecs)
			progress := elapsed / cfDur

			if progress >= 1.0 {
				// Crossfade complete — swap
				m.activeMPV().Stop()
				m.activeIsB = !m.activeIsB
				m.activeMPV().SetVolume(m.cfg.Volume)
				m.crossfading = false
				m.preloadedURL = ""
				m.preloading = false

				// Advance queue — pop the track that was preloaded
				if t := m.queue.Pop(); t != nil {
					m.nowPlaying = t
					m.status = fmt.Sprintf("Now playing: %s", t.Title)
					m.history.Add(*t)
					m.history.Save()
					m.refreshHistory()
				}
				m.playStarted = time.Now()
				if cmd := m.maybeFetchSuggestions(); cmd != nil {
					return m, tea.Batch(tickCmd(), cmd)
				}
				return m, tickCmd()
			} else {
				// Ramp volumes
				oldVol := int(float64(m.cfg.Volume) * (1 - progress))
				newVol := int(float64(m.cfg.Volume) * progress)
				m.activeMPV().SetVolume(oldVol)
				m.inactiveMPV().SetVolume(newVol)
			}
		}

		// Auto-advance when track ends (grace period: don't check within 3s of starting)
		if !m.crossfading && m.activeMPV().Playing && !m.activeMPV().Paused && time.Since(m.playStarted) > 3*time.Second && m.activeMPV().CachedIdle {
			dur := m.activeMPV().CachedDuration
			pos := m.activeMPV().CachedPosition
			// Only advance if track actually finished (position near end) or duration unknown
			if dur <= 0 || pos >= dur-2.0 {
				return m, tea.Batch(tickCmd(), m.playNext())
			}
			// mpv went idle mid-track — likely a buffer/network hiccup, try to resume
			m.status = "Playback interrupted — resuming..."
			m.activeMPV().Resume()
		}
		return m, tickCmd()

	case seekDebounceMsg:
		if msg.seqNo == m.seekSeqNo && m.pendingSeek != 0 {
			// If still in cooldown or a seek is in-flight, re-delay
			if m.seeking || time.Since(m.lastSeekAt) < 400*time.Millisecond {
				seqNo := m.seekSeqNo
				return m, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
					return seekDebounceMsg{seqNo: seqNo}
				})
			}
			offset := m.pendingSeek
			m.pendingSeek = 0
			m.seeking = true
			m.lastSeekAt = time.Now()
			mpv := m.activeMPV()
			return m, func() tea.Msg {
				// Read pause state at execution time to avoid race with user toggling pause
				wasPaused := mpv.Paused
				// Pause before seeking to avoid audio subsystem churn
				if !wasPaused {
					mpv.Pause()
				}
				mpv.Seek(offset)
				// Small settle time for WSL audio pipeline
				time.Sleep(50 * time.Millisecond)
				if !wasPaused {
					mpv.Resume()
				}
				return seekDoneMsg{}
			}
		}
		return m, nil

	case seekDoneMsg:
		m.seeking = false
		return m, nil

	case crossfadePreloadMsg:
		m.preloading = false
		if msg.err == nil && msg.url != "" {
			m.preloadedURL = msg.url
			m.preloadedTrack = &msg.track
		}
		return m, nil

	case sessionResumeMsg:
		// If user already started playing something, discard the resume
		if m.resumeCanceled {
			m.resumePos = 0
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Sprintf("Resume error: %s", msg.err)
			m.nowPlaying = nil
			m.resumePos = 0
			m.activeMPV().Playing = false
			return m, nil
		}
		// PlayPaused loads the file without any audio output
		if err := m.activeMPV().PlayPaused(msg.url); err != nil {
			m.err = fmt.Sprintf("Resume error: %s", err)
			m.nowPlaying = nil
			m.resumePos = 0
			m.activeMPV().Playing = false
			return m, nil
		}
		t := msg.track
		m.nowPlaying = &t
		m.playStarted = time.Now()
		m.activeMPV().SetVolume(m.cfg.Volume)
		m.status = fmt.Sprintf("Paused: %s — press Space to play", truncate(t.Title, 30))
		// Seek to saved position after a short delay to let mpv buffer
		pos := msg.pos
		return m, tea.Tick(800*time.Millisecond, func(_ time.Time) tea.Msg {
			return sessionSeekMsg{pos: pos}
		})

	case sessionSeekMsg:
		// Seek to saved position — stream is already paused, user hits Space to play
		if m.nowPlaying != nil && msg.pos > 0 {
			m.activeMPV().SeekAbsolute(msg.pos)
		}
		m.resumePos = 0
		return m, nil

	case streamURLMsg:
		// Discard stale stream URL from a previously requested track
		if m.loadingTrackID != "" && msg.track.ID != m.loadingTrackID {
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Sprintf("Stream error: %s", msg.err)
			m.status = ""
			m.nowPlaying = nil
			m.loadingTrackID = ""
			return m, nil
		}
		m.err = ""
		m.loadingTrackID = ""
		m.status = fmt.Sprintf("Now playing: %s", msg.track.Title)
		if err := m.activeMPV().Play(msg.url); err != nil {
			m.err = fmt.Sprintf("Playback error: %s", err)
			m.status = ""
			return m, nil
		}
		t := msg.track
		m.nowPlaying = &t
		m.playStarted = time.Now()
		m.activeMPV().SetVolume(m.cfg.Volume)
		m.history.Add(msg.track)
		m.history.Save()
		m.refreshHistory()
		// Fetch new suggestions for the now-playing track
		return m, m.maybeFetchSuggestions()

	case suggestionTrackMsg:
		if msg.forID != m.suggestions.forTrackID {
			return m, nil // stale result from a previous song
		}
		if msg.ch == nil {
			// Channel closed — all done
			m.suggestions.loading = false
			return m, nil
		}
		m.suggestions.tracks = append(m.suggestions.tracks, msg.track)
		return m, func() tea.Msg {
			return waitForSuggestion(msg.ch, msg.forID)()
		}

	case searchResultsMsg:
		m.search, _ = m.search.Update(msg)
		m.status = fmt.Sprintf("Found %d results", len(m.search.results))
		return m, nil

	case downloadProgressMsg:
		m.downloads.updateProgress(msg.progress)
		if msg.progress.Done || msg.progress.Error != nil {
			if msg.progress.Done {
				m.status = fmt.Sprintf("Downloaded: %s", truncate(msg.progress.Track.Title, 30))
			}
			return m, nil
		}
		// Chain next progress read
		if msg.ch != nil {
			return m, waitForDownload(msg.ch)
		}
		return m, nil

	case playlistsLoadedMsg:
		m.playlists, _ = m.playlists.Update(msg)
		return m, nil

	case pickerListMsg:
		m.picker, _ = m.picker.Update(msg)
		return m, nil

	case playlistSaveMsg:
		if msg.err != nil {
			m.err = fmt.Sprintf("Save error: %s", msg.err)
		} else {
			m.status = fmt.Sprintf("Saved to playlist: %s", msg.name)
		}
		return m, loadPlaylists()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Always allow quit
	if key == "ctrl+c" {
		m.SaveSession()
		return m, tea.Quit
	}

	// Confirmation overlay takes priority
	if m.confirm.active {
		switch key {
		case "y":
			switch m.confirm.action {
			case confirmClearQueue:
				m.queue.Clear()
				m.queueView.cursor = 0
				m.status = "Queue cleared"
			case confirmDeletePlaylist:
				if pl := m.playlists.selectedPlaylist(); pl != nil {
					playlist.Delete(pl.Name)
					m.confirm.close()
					return m, loadPlaylists()
				}
			case confirmDeleteDownload:
				if m.downloads.cursor < len(m.downloads.items) {
					item := m.downloads.items[m.downloads.cursor]
					download.RemoveFromLibrary(item.track.ID)
					download.DeleteFile(item.track, m.downloads.outputDir, m.downloads.format)
					m.downloads.items = append(m.downloads.items[:m.downloads.cursor], m.downloads.items[m.downloads.cursor+1:]...)
					if m.downloads.cursor >= len(m.downloads.items) && m.downloads.cursor > 0 {
						m.downloads.cursor--
					}
					m.status = "Deleted download"
				}
			case confirmDeleteHistory:
				if e := m.histView.selectedEntry(); e != nil {
					m.history.Delete(e.Track.ID)
					m.history.Save()
					m.refreshHistory()
					if m.histView.cursor >= len(m.histView.entries) && m.histView.cursor > 0 {
						m.histView.cursor--
					}
					m.status = "Deleted from history"
				}
			}
			m.confirm.close()
			return m, nil
		case "n", "esc":
			m.confirm.close()
			return m, nil
		}
		return m, nil
	}

	// Picker overlay takes priority
	if m.picker.active {
		if key == "enter" && !m.picker.creating {
			// Save track to selected playlist
			if pl := m.picker.selectedPlaylist(); pl != nil && m.pickerTrack != nil {
				if pl.ContainsTrack(m.pickerTrack.ID) {
					m.status = fmt.Sprintf("Already in: %s", pl.Name)
					m.picker.close()
					m.pickerTrack = nil
					return m, nil
				}
				pl.AddTrack(*m.pickerTrack)
				pl.Save()
				m.status = fmt.Sprintf("Saved to: %s", pl.Name)
				m.picker.close()
				m.pickerTrack = nil
				return m, loadPlaylists()
			}
			m.picker.close()
			return m, nil
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	// Unfocus search input when not on the search page
	if m.search.focused && m.view != ViewSearch {
		m.search.focused = false
		m.search.input.Blur()
	}

	// When text input is active, only route to the input
	if m.inputActive() {
		switch m.view {
		case ViewSearch:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		case ViewPlaylists:
			var cmd tea.Cmd
			m.playlists, cmd = m.playlists.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Help overlay navigation (fully modal)
	if m.showHelp {
		totalHelpLines := len(helpBindings) + 3 // title + divider + blank
		maxScroll := totalHelpLines - m.contentHeight()
		if maxScroll < 0 {
			maxScroll = 0
		}
		switch key {
		case "up", "k":
			if m.helpScroll > 0 {
				m.helpScroll--
			}
		case "down", "j":
			if m.helpScroll < maxScroll {
				m.helpScroll++
			}
		case "q", "esc", "?":
			m.showHelp = false
			m.helpScroll = 0
		}
		return m, nil
	}

	// Global keybindings (only when no input is focused)
	switch key {
	case "q":
		// q goes back if there's somewhere to go; otherwise quit
		if m.view == ViewPlaylists && m.playlists.viewing {
			m.playlists.viewing = false
			m.playlists.trackCursor = 0
			m.playlists.trackScroll = 0
			return m, nil
		}
		m.SaveSession()
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		if !m.showHelp {
			m.helpScroll = 0
		}
		return m, nil
	case " ", "enter":
		// Unified: try to play selected track, fallback to toggle pause
		switch m.view {
		case ViewSearch:
			if t := m.search.Selected(); t != nil {
				return m, m.playTrack(*t)
			}
		case ViewSuggestions:
			if t := m.suggestions.selectedTrack(); t != nil {
				return m, m.playTrack(*t)
			}
		case ViewQueue:
			if t := m.queueView.selectedTrack(m.queue); t != nil {
				m.queue.Remove(m.queueView.cursor)
				if m.queueView.cursor >= m.queue.Len() && m.queueView.cursor > 0 {
					m.queueView.cursor--
				}
				return m, m.playTrack(*t)
			}
		case ViewPlaylists:
			if m.playlists.viewing {
				if t := m.playlists.selectedTrack(); t != nil {
					return m, m.playTrack(*t)
				}
			} else if len(m.playlists.playlists) > 0 {
				m.playlists.viewing = true
				m.playlists.trackCursor = 0
				m.playlists.trackScroll = 0
				return m, nil
			}
		case ViewHistory:
			if t := m.histView.selectedTrack(); t != nil {
				return m, m.playTrack(*t)
			}
		case ViewDownloads:
			if t := m.downloads.selectedTrack(); t != nil {
				return m, m.playTrack(*t)
			}
		}
		// Fallback: toggle pause
		if m.activeMPV().Playing {
			m.activeMPV().TogglePause()
			if m.activeMPV().Paused {
				m.status = "Paused"
			} else {
				m.status = "Resumed"
			}
		}
		return m, nil
	case "n":
		return m, m.playNext()
	case "p":
		return m, m.playPrev()
	case "0":
		if m.nowPlaying != nil && m.activeMPV().Playing {
			m.activeMPV().SeekAbsolute(0)
			m.status = "Restarted track"
		}
		return m, nil
	case "+", "=":
		m.cfg.Volume = min(m.cfg.Volume+5, 100)
		m.activeMPV().SetVolume(m.cfg.Volume)
		m.status = fmt.Sprintf("Volume: %d%%", m.cfg.Volume)
		return m, nil
	case "-":
		m.cfg.Volume = max(m.cfg.Volume-5, 0)
		m.activeMPV().SetVolume(m.cfg.Volume)
		m.status = fmt.Sprintf("Volume: %d%%", m.cfg.Volume)
		return m, nil
	case "1":
		m.view = ViewSearch
		return m, nil
	case "2":
		m.view = ViewNowPlaying
		return m, nil
	case "3":
		m.view = ViewQueue
		return m, nil
	case "4":
		m.view = ViewPlaylists
		m.playlists.viewing = false
		m.playlists.trackCursor = 0
		m.playlists.trackScroll = 0
		return m, loadPlaylists()
	case "5":
		m.view = ViewHistory
		m.refreshHistory()
		return m, nil
	case "6":
		m.view = ViewSuggestions
		return m, m.maybeFetchSuggestions()
	case "7":
		m.view = ViewDownloads
		return m, nil
	case "left":
		if m.activeMPV().Playing {
			return m, m.debouncedSeek(-5)
		}
		return m, nil
	case "right":
		if m.activeMPV().Playing {
			return m, m.debouncedSeek(5)
		}
		return m, nil
	case "<":
		if m.activeMPV().Playing {
			return m, m.debouncedSeek(-30)
		}
		return m, nil
	case ">":
		if m.activeMPV().Playing {
			return m, m.debouncedSeek(30)
		}
		return m, nil
	case "/":
		// Global: jump to search page and focus input from any view
		m.view = ViewSearch
		m.search.focused = true
		m.search.input.Focus()
		return m, textinput.Blink
	case "v":
		m.vizStyle = (m.vizStyle + 1) % len(vizStyleNames)
		m.status = fmt.Sprintf("Visualizer: %s", vizStyleNames[m.vizStyle])
		return m, nil
	case "V":
		// Random viz style (different from current)
		if len(vizStyleNames) > 1 {
			for {
				n := rand.Intn(len(vizStyleNames))
				if n != m.vizStyle {
					m.vizStyle = n
					break
				}
			}
		}
		m.status = fmt.Sprintf("Visualizer: %s", vizStyleNames[m.vizStyle])
		return m, nil
	case "C":
		m.vizAutoCycle = !m.vizAutoCycle
		m.vizCycleTick = 0
		if m.vizAutoCycle {
			m.status = "Viz auto-cycle: ON"
		} else {
			m.status = "Viz auto-cycle: OFF"
		}
		return m, nil
	case "]":
		m.vizBoost = math.Min(m.vizBoost+0.5, 5.0)
		m.status = fmt.Sprintf("Viz energy: %.1fx", m.vizBoost)
		return m, nil
	case "[":
		m.vizBoost = math.Max(m.vizBoost-0.5, 1.0)
		m.status = fmt.Sprintf("Viz energy: %.1fx", m.vizBoost)
		return m, nil
	case "R":
		// Rate highlighted track (or currently playing if nothing selected)
		t := m.selectedTrack()
		if t == nil {
			t = m.nowPlaying
		}
		if t != nil {
			rating, _ := m.history.RatingFor(t.ID)
			rating = (rating + 1) % 6
			m.history.SetRating(t.ID, rating)
			m.history.Save()
			m.refreshHistory()
			if rating == 0 {
				m.status = "Rating cleared"
			} else {
				stars := strings.Repeat("★", rating) + strings.Repeat("☆", 5-rating)
				m.status = fmt.Sprintf("Rated: %s", stars)
			}
		}
		return m, nil
	case "a":
		// On playlist list view (not inside a playlist), reroute to queue-all
		if m.view == ViewPlaylists && !m.playlists.viewing {
			pl := m.playlists.selectedPlaylist()
			if pl != nil && len(pl.Tracks) > 0 {
				added := 0
				for _, t := range pl.Tracks {
					if !m.queue.Contains(t.ID) {
						m.queue.Add(t)
						added++
					}
				}
				if added > 0 {
					m.status = fmt.Sprintf("Queued %d tracks from: %s", added, pl.Name)
				} else {
					m.status = "All tracks already in queue"
				}
			}
			return m, nil
		}
		// Universal: add highlighted track to queue, fallback to now playing
		t := m.selectedTrack()
		if t == nil {
			t = m.nowPlaying
		}
		if t != nil {
			if m.queue.Contains(t.ID) {
				m.status = fmt.Sprintf("Already in queue: %s", truncate(t.Title, 30))
			} else {
				m.queue.Add(*t)
				m.status = fmt.Sprintf("Queued: %s", truncate(t.Title, 30))
			}
		}
		return m, nil
	case "d":
		// Universal: download highlighted track, fallback to now playing
		t := m.selectedTrack()
		if t == nil {
			t = m.nowPlaying
		}
		if t != nil {
			return m, m.startDownload(*t)
		}
		return m, nil
	case "s":
		// Universal: save highlighted track to playlist, fallback to now playing
		t := m.selectedTrack()
		if t == nil {
			t = m.nowPlaying
		}
		if t != nil {
			return m, m.openPicker(*t)
		}
		return m, nil
	case "S":
		shuffle := m.queue.ToggleShuffle()
		if shuffle {
			m.status = "Shuffle: ON"
		} else {
			m.status = "Shuffle: OFF"
		}
		return m, nil
	case "r":
		mode := m.queue.CycleRepeat()
		switch mode {
		case player.RepeatOff:
			m.status = "Repeat: OFF"
		case player.RepeatAll:
			m.status = "Repeat: ALL"
		case player.RepeatOne:
			m.status = "Repeat: ONE"
		}
		return m, nil
	}

	// View-specific keybindings
	switch m.view {
	case ViewSearch:
		// Pass navigation keys to search model
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.ensureAllVisible()
		return m, cmd

	case ViewSuggestions:
		switch key {
		case "up", "k":
			if m.suggestions.cursor > 0 {
				m.suggestions.cursor--
			}
		case "down", "j":
			if m.suggestions.cursor < len(m.suggestions.tracks)-1 {
				m.suggestions.cursor++
			}
		case "l":
			if !m.suggestions.loading {
				m.status = "Loading more suggestions..."
				m.ensureAllVisible()
				return m, m.loadMoreSuggestions()
			}
		}
		m.ensureAllVisible()
		return m, nil

	case ViewQueue:
		switch key {
		case "up", "k":
			if m.queueView.cursor > 0 {
				m.queueView.cursor--
			}
		case "down", "j":
			if m.queueView.cursor < m.queue.Len()-1 {
				m.queueView.cursor++
			}
		case "delete", "backspace", "x":
			m.queue.Remove(m.queueView.cursor)
			if m.queueView.cursor >= m.queue.Len() && m.queueView.cursor > 0 {
				m.queueView.cursor--
			}
			m.status = "Removed from queue"
		case "C":
			m.confirm.show("Clear entire queue?", confirmClearQueue)
		}
		m.ensureAllVisible()
		return m, nil

	case ViewPlaylists:
		if m.playlists.viewing {
			switch key {
			case "A":
				pl := m.playlists.selectedPlaylist()
				if pl != nil && len(pl.Tracks) > 0 {
					added := 0
					for _, t := range pl.Tracks {
						if !m.queue.Contains(t.ID) {
							m.queue.Add(t)
							added++
						}
					}
					if added > 0 {
						m.status = fmt.Sprintf("Queued %d tracks from: %s", added, pl.Name)
					} else {
						m.status = "All tracks already in queue"
					}
				}
				return m, nil
			case "J":
				pl := m.playlists.selectedPlaylist()
				if pl != nil && m.playlists.trackCursor < len(pl.Tracks)-1 {
					pl.MoveTrack(m.playlists.trackCursor, m.playlists.trackCursor+1)
					pl.Save()
					m.playlists.playlists[m.playlists.cursor] = *pl
					m.playlists.trackCursor++
				}
				return m, nil
			case "K":
				pl := m.playlists.selectedPlaylist()
				if pl != nil && m.playlists.trackCursor > 0 {
					pl.MoveTrack(m.playlists.trackCursor, m.playlists.trackCursor-1)
					pl.Save()
					m.playlists.playlists[m.playlists.cursor] = *pl
					m.playlists.trackCursor--
				}
				return m, nil
			case "Z":
				pl := m.playlists.selectedPlaylist()
				if pl != nil && len(pl.Tracks) > 1 {
					pl.Shuffle()
					pl.Save()
					m.playlists.playlists[m.playlists.cursor] = *pl
					m.status = fmt.Sprintf("Shuffled: %s", pl.Name)
				}
				return m, nil
			}
		} else {
			// Playlist list view
			switch key {
			case "e":
				if pl := m.playlists.selectedPlaylist(); pl != nil {
					m.playlists.renaming = true
					m.playlists.nameInput.SetValue(pl.Name)
					m.playlists.nameInput.Focus()
					return m, textinput.Blink
				}
				return m, nil
			case "A":
				pl := m.playlists.selectedPlaylist()
				if pl != nil && len(pl.Tracks) > 0 {
					added := 0
					for _, t := range pl.Tracks {
						if !m.queue.Contains(t.ID) {
							m.queue.Add(t)
							added++
						}
					}
					if added > 0 {
						m.status = fmt.Sprintf("Queued %d tracks from: %s", added, pl.Name)
					} else {
						m.status = "All tracks already in queue"
					}
				}
				return m, nil
			case "Z":
				if pl := m.playlists.selectedPlaylist(); pl != nil && len(pl.Tracks) > 1 {
					pl.Shuffle()
					pl.Save()
					m.playlists.playlists[m.playlists.cursor] = *pl
					m.status = fmt.Sprintf("Shuffled: %s", pl.Name)
				}
				return m, nil
			case "delete", "backspace":
				if pl := m.playlists.selectedPlaylist(); pl != nil {
					m.confirm.show(fmt.Sprintf("Delete playlist '%s'?", pl.Name), confirmDeletePlaylist)
					return m, nil
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.playlists, cmd = m.playlists.Update(msg)
		m.ensureAllVisible()
		return m, cmd

	case ViewHistory:
		switch key {
		case "up", "k":
			if m.histView.cursor > 0 {
				m.histView.cursor--
			}
		case "down", "j":
			if m.histView.cursor < len(m.histView.entries)-1 {
				m.histView.cursor++
			}
		case "o":
			m.histView.sortMode = (m.histView.sortMode + 1) % 3
			m.status = fmt.Sprintf("Sort: %s", sortModeNames[m.histView.sortMode])
			return m, nil
		case "delete", "backspace", "x":
			if e := m.histView.selectedEntry(); e != nil {
				m.confirm.show(fmt.Sprintf("Delete '%s' from history?", truncate(e.Track.Title, 30)), confirmDeleteHistory)
			}
			return m, nil
		}
		m.ensureAllVisible()
		return m, nil

	case ViewDownloads:
		switch key {
		case "up", "k":
			if m.downloads.cursor > 0 {
				m.downloads.cursor--
			}
		case "down", "j":
			if m.downloads.cursor < len(m.downloads.items)-1 {
				m.downloads.cursor++
			}
		case "delete", "backspace", "x":
			if m.downloads.cursor < len(m.downloads.items) {
				item := m.downloads.items[m.downloads.cursor]
				m.confirm.show(fmt.Sprintf("Delete download '%s'?", truncate(item.track.Title, 30)), confirmDeleteDownload)
			}
			return m, nil
		}
		m.ensureAllVisible()
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	iw := m.width - 2 // inner width between │ borders
	contentHeight := m.contentHeight()

	var b strings.Builder

	// Top border
	b.WriteString(frameTop(iw))
	b.WriteByte('\n')

	// Header
	title := brandStyle.Render("♪  C H U N E S")
	b.WriteString(frameRow(lipgloss.PlaceHorizontal(iw, lipgloss.Center, title), iw))
	b.WriteByte('\n')

	// Divider
	b.WriteString(frameDivider(iw))
	b.WriteByte('\n')

	// Tab row
	b.WriteString(frameRow(m.renderTabs(iw), iw))
	b.WriteByte('\n')

	// Divider
	b.WriteString(frameDivider(iw))
	b.WriteByte('\n')

	// Content area
	rf := ratingFunc(m.history.RatingFor)
	var content string
	if m.showHelp {
		content = renderHelp(iw, m.helpScroll, contentHeight)
	} else {
		switch m.view {
		case ViewSearch:
			content = m.search.View(iw, contentHeight, rf)
		case ViewNowPlaying:
			content = m.viewNowPlaying(iw, contentHeight)
		case ViewSuggestions:
			content = m.suggestions.View(iw, contentHeight, rf, m.nowPlaying)
		case ViewQueue:
			content = m.queueView.View(m.queue, iw, contentHeight, rf)
		case ViewPlaylists:
			content = m.playlists.View(iw, contentHeight, rf)
		case ViewHistory:
			content = m.histView.View(iw, contentHeight)
		case ViewDownloads:
			content = m.downloads.View(iw, contentHeight, rf)
		}
	}

	// Render exactly contentHeight rows in the frame
	lines := strings.Split(content, "\n")
	for i := 0; i < contentHeight; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		b.WriteString(frameRow(line, iw))
		b.WriteByte('\n')
	}

	// Divider
	b.WriteString(frameDivider(iw))
	b.WriteByte('\n')

	// Status line
	var statusContent string
	if m.err != "" {
		statusContent = errorBarStyle.Render(" ✗ " + m.err)
	} else if m.status != "" {
		statusContent = statusBarStyle.Render(" ● " + m.status)
	}
	b.WriteString(frameRow(statusContent, iw))
	b.WriteByte('\n')

	// Keybind hints
	hints := m.currentHints()
	b.WriteString(frameRow(renderHints(hints, iw), iw))
	b.WriteByte('\n')

	// Player bar (2 lines) — uses cached values, no IPC calls in View()
	// Use nowPlaying so the bar persists even if track is removed from queue
	track := m.nowPlaying
	position := m.activeMPV().CachedPosition
	duration := m.activeMPV().CachedDuration
	// Get rating for current track
	var rating int
	if track != nil {
		rating, _ = m.history.RatingFor(track.ID)
	}
	playerContent := renderPlayerBar(track, position, duration, m.cfg.Volume, m.activeMPV().Paused,
		m.queue.Shuffle(), m.queue.Repeat(), rating, iw)
	playerLines := strings.Split(playerContent, "\n")
	for i := 0; i < 2; i++ {
		pl := ""
		if i < len(playerLines) {
			pl = playerLines[i]
		}
		b.WriteString(frameRow(pl, iw))
		b.WriteByte('\n')
	}

	// Bottom border
	b.WriteString(frameBottom(iw))

	rendered := b.String()

	// Picker overlay — float on top of the content
	if m.picker.active {
		overlay := m.picker.View()
		rendered = placeOverlay(m.width, m.height, overlay, rendered)
	}

	// Confirmation overlay
	if m.confirm.active {
		overlay := m.confirm.View()
		rendered = placeOverlay(m.width, m.height, overlay, rendered)
	}

	return rendered
}

func (m Model) renderTabs(width int) string {
	var tabs []string
	for i, name := range viewNames {
		label := fmt.Sprintf(" %d:%s ", i+1, name)
		if View(i) == m.view {
			label = fmt.Sprintf(" » %d:%s ", i+1, name)
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, tabStyle.Render(label))
		}
	}
	// Now-playing badge
	if m.activeMPV().Playing {
		badge := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(" ♪ ")
		tabs = append(tabs, badge)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m Model) currentHints() []helpBinding {
	// Very narrow: just show help key
	if m.width < 40 {
		return []helpBinding{{"?", "help"}}
	}

	var hints []helpBinding

	switch m.view {
	case ViewSearch:
		hints = searchHints(m.search.focused)
	case ViewNowPlaying:
		hints = nowPlayingHints()
	case ViewSuggestions:
		hints = suggestionsHints()
	case ViewQueue:
		hints = queueHints()
	case ViewPlaylists:
		hints = playlistHints(m.playlists.viewing, m.playlists.creating, m.playlists.renaming)
	case ViewHistory:
		hints = historyHints()
	case ViewDownloads:
		hints = downloadHints()
	}

	// Narrow: show only the first 3 view hints + help
	if m.width < 60 {
		if len(hints) > 3 {
			hints = hints[:3]
		}
		hints = append(hints, helpBinding{"?", "help"})
		return hints
	}

	// Add player hints when not in text input mode
	if !m.inputActive() {
		hints = append(hints, helpBinding{"│", ""})
		hints = append(hints, playerHints()...)
	}

	return hints
}

func (m *Model) preloadNextURL(t player.Track) tea.Cmd {
	// Offline mode: use local file if downloaded
	if path := m.localFilePath(t); path != "" {
		return func() tea.Msg {
			return crossfadePreloadMsg{track: t, url: path, err: nil}
		}
	}
	return func() tea.Msg {
		url, err := youtube.GetStreamURL(t.ID)
		return crossfadePreloadMsg{track: t, url: url, err: err}
	}
}

func (m *Model) debouncedSeek(seconds float64) tea.Cmd {
	m.pendingSeek += seconds
	m.seekSeqNo++
	seqNo := m.seekSeqNo
	m.status = fmt.Sprintf("Seek %+.0fs", m.pendingSeek)

	// If a seek is in-flight or in cooldown, use a longer delay to batch more
	delay := 200 * time.Millisecond
	if m.seeking || time.Since(m.lastSeekAt) < 400*time.Millisecond {
		delay = 500 * time.Millisecond
	}

	return tea.Tick(delay, func(time.Time) tea.Msg {
		return seekDebounceMsg{seqNo: seqNo}
	})
}

func (m *Model) resetCrossfade() {
	if m.crossfading {
		m.inactiveMPV().Stop()
	}
	m.crossfading = false
	m.preloadedURL = ""
	m.preloadedTrack = nil
	m.preloading = false
}

func (m *Model) localFilePath(t player.Track) string {
	for _, item := range m.downloads.items {
		if item.track.ID == t.ID && item.done {
			path := download.ResolvedPath(t, m.downloads.outputDir, m.downloads.format)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func (m *Model) playTrack(t player.Track) tea.Cmd {
	m.resetCrossfade()
	m.err = ""
	m.resumePos = 0         // cancel any pending session resume
	m.resumeCanceled = true  // discard in-flight sessionResumeMsg
	m.loadingTrackID = t.ID // tag so we can discard stale streamURLMsg
	m.playStarted = time.Now() // prevent auto-advance from double-popping while URL resolves
	// Offline mode: play local file if downloaded
	if path := m.localFilePath(t); path != "" {
		m.status = fmt.Sprintf("Playing (local): %s", truncate(t.Title, 30))
		return func() tea.Msg {
			return streamURLMsg{track: t, url: path, err: nil}
		}
	}
	m.status = fmt.Sprintf("Loading: %s...", truncate(t.Title, 30))
	return getStreamURL(t)
}

func (m *Model) playNext() tea.Cmd {
	t := m.queue.Pop()
	if t == nil {
		m.status = "End of queue"
		m.activeMPV().Playing = false
		m.nowPlaying = nil
		return nil
	}
	return m.playTrack(*t)
}

func (m *Model) playPrev() tea.Cmd {
	// No history stack — prev is not supported with consume queue
	m.status = "No previous track"
	return nil
}

func (m *Model) startDownload(t player.Track) tea.Cmd {
	m.downloads.add(t)
	m.status = fmt.Sprintf("Downloading: %s", truncate(t.Title, 30))
	m.view = ViewDownloads

	ch := make(chan download.Progress, 16)
	go download.Download(t, m.cfg.DownloadDir, m.cfg.AudioFormat, ch)
	return waitForDownload(ch)
}

// waitForDownload reads the next progress message from the channel and returns
// it as a tea.Msg, then chains another waitForDownload for the next message.
func waitForDownload(ch <-chan download.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		var nextCh <-chan download.Progress
		if !p.Done && p.Error == nil {
			nextCh = ch
		}
		return downloadProgressMsg{progress: p, ch: nextCh}
	}
}

func (m Model) handleScrollWheel(button tea.MouseButton) (tea.Model, tea.Cmd) {
	delta := 3 // scroll 3 lines per wheel tick
	if button == tea.MouseButtonWheelUp {
		delta = -delta
	}

	// Help overlay
	if m.showHelp {
		totalHelpLines := len(helpBindings) + 3
		maxScroll := totalHelpLines - m.contentHeight()
		if maxScroll < 0 {
			maxScroll = 0
		}
		m.helpScroll += delta
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		if m.helpScroll > maxScroll {
			m.helpScroll = maxScroll
		}
		return m, nil
	}

	// Move cursor in active view
	switch m.view {
	case ViewSearch:
		m.search.cursor += delta
		if m.search.cursor < 0 {
			m.search.cursor = 0
		}
		if m.search.cursor >= len(m.search.results) {
			m.search.cursor = max(len(m.search.results)-1, 0)
		}
	case ViewSuggestions:
		m.suggestions.cursor += delta
		if m.suggestions.cursor < 0 {
			m.suggestions.cursor = 0
		}
		if m.suggestions.cursor >= len(m.suggestions.tracks) {
			m.suggestions.cursor = max(len(m.suggestions.tracks)-1, 0)
		}
	case ViewQueue:
		m.queueView.cursor += delta
		if m.queueView.cursor < 0 {
			m.queueView.cursor = 0
		}
		if m.queueView.cursor >= m.queue.Len() {
			m.queueView.cursor = max(m.queue.Len()-1, 0)
		}
	case ViewPlaylists:
		if m.playlists.viewing {
			pl := m.playlists.selectedPlaylist()
			if pl != nil {
				m.playlists.trackCursor += delta
				if m.playlists.trackCursor < 0 {
					m.playlists.trackCursor = 0
				}
				if m.playlists.trackCursor >= len(pl.Tracks) {
					m.playlists.trackCursor = max(len(pl.Tracks)-1, 0)
				}
			}
		} else {
			m.playlists.cursor += delta
			if m.playlists.cursor < 0 {
				m.playlists.cursor = 0
			}
			if m.playlists.cursor >= len(m.playlists.playlists) {
				m.playlists.cursor = max(len(m.playlists.playlists)-1, 0)
			}
		}
	case ViewHistory:
		m.histView.cursor += delta
		if m.histView.cursor < 0 {
			m.histView.cursor = 0
		}
		if m.histView.cursor >= len(m.histView.entries) {
			m.histView.cursor = max(len(m.histView.entries)-1, 0)
		}
	case ViewDownloads:
		m.downloads.cursor += delta
		if m.downloads.cursor < 0 {
			m.downloads.cursor = 0
		}
		if m.downloads.cursor >= len(m.downloads.items) {
			m.downloads.cursor = max(len(m.downloads.items)-1, 0)
		}
	}
	m.ensureAllVisible()
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle scroll wheel
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		return m.handleScrollWheel(msg.Button)
	}

	if msg.Action != tea.MouseActionRelease {
		return m, nil
	}

	x, y := msg.X, msg.Y
	iw := m.width - 2
	contentHeight := m.contentHeight()

	// Tab row click (row 3) — compute actual tab positions
	if y == 3 && x > 0 {
		cumX := 1 // after left │ border
		for i, name := range viewNames {
			label := fmt.Sprintf(" %d:%s ", i+1, name)
			if View(i) == m.view {
				label = fmt.Sprintf(" » %d:%s ", i+1, name)
			}
			var rendered string
			if View(i) == m.view {
				rendered = activeTabStyle.Render(label)
			} else {
				rendered = tabStyle.Render(label)
			}
			tw := lipgloss.Width(rendered)
			if x >= cumX && x < cumX+tw {
				m.view = View(i)
				if m.view == ViewSuggestions {
					return m, m.maybeFetchSuggestions()
				}
				if m.view == ViewPlaylists {
					m.playlists.viewing = false
					m.playlists.trackCursor = 0
					m.playlists.trackScroll = 0
					return m, loadPlaylists()
				}
				if m.view == ViewHistory {
					m.refreshHistory()
				}
				return m, nil
			}
			cumX += tw
		}
	}

	// Content area click (rows 5 to 5+contentHeight-1)
	contentStart := 5
	contentEnd := contentStart + contentHeight
	if y >= contentStart && y < contentEnd {
		row := y - contentStart
		return m.handleContentClick(row, x)
	}

	// Player bar area - progress bar is on the second player line
	// After content: div + status + hints + player1 + player2
	playerLine2 := contentEnd + 3 + 1 // +3 for div+status+hints, +1 for player line 1
	if y == playerLine2 && m.activeMPV().Playing {
		// Map X to seek position - bar starts at col 3, width is iw-16
		barStart := 3
		barWidth := max(iw-16, 10)
		if x >= barStart && x < barStart+barWidth && !m.seeking {
			frac := float64(x-barStart) / float64(barWidth)
			duration := m.activeMPV().GetDuration()
			if duration > 0 {
				target := frac * duration
				m.seeking = true
				m.lastSeekAt = time.Now()
				m.status = fmt.Sprintf("Seek to %s", formatSeconds(target))
				mpv := m.activeMPV()
				wasPaused := mpv.Paused
				return m, func() tea.Msg {
					if !wasPaused {
						mpv.Pause()
					}
					mpv.SeekAbsolute(target)
					time.Sleep(50 * time.Millisecond)
					if !wasPaused {
						mpv.Resume()
					}
					return seekDoneMsg{}
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleContentClick(row, x int) (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSearch:
		// Click on the search box (rows 0-2) focuses the input
		if row <= 2 {
			m.search.focused = true
			m.search.input.Focus()
			return m, textinput.Blink
		}
		// Account for search box (3 lines) + empty line + header (1) + possible scroll indicator
		listStart := 5
		if m.search.scroll > 0 {
			listStart++
		}
		idx := m.search.scroll + (row - listStart)
		if idx >= 0 && idx < len(m.search.results) {
			if m.search.cursor == idx {
				// Double-click effect: play
				t := m.search.results[idx]
				return m, m.playTrack(t)
			}
			m.search.cursor = idx
		}
	case ViewSuggestions:
		listStart := 3
		if m.suggestions.scroll > 0 {
			listStart++
		}
		idx := m.suggestions.scroll + (row - listStart)
		if idx >= 0 && idx < len(m.suggestions.tracks) {
			if m.suggestions.cursor == idx {
				t := m.suggestions.tracks[idx]
				return m, m.playTrack(t)
			}
			m.suggestions.cursor = idx
		}
	case ViewQueue:
		listStart := 3
		if m.queueView.scroll > 0 {
			listStart++
		}
		idx := m.queueView.scroll + (row - listStart)
		if idx >= 0 && idx < m.queue.Len() {
			if m.queueView.cursor == idx {
				// Double-click: remove from queue and play
				if t := m.queueView.selectedTrack(m.queue); t != nil {
					m.queue.Remove(idx)
					if m.queueView.cursor >= m.queue.Len() && m.queueView.cursor > 0 {
						m.queueView.cursor--
					}
					return m, m.playTrack(*t)
				}
			}
			m.queueView.cursor = idx
		}
	case ViewHistory:
		listStart := 3
		if historyStats(m.histView.allEntries) != "" {
			listStart++
		}
		if m.histView.scroll > 0 {
			listStart++
		}
		idx := m.histView.scroll + (row - listStart)
		sorted := m.histView.sortedEntries()
		if idx >= 0 && idx < len(sorted) {
			if m.histView.cursor == idx {
				t := sorted[idx].Track
				return m, m.playTrack(t)
			}
			m.histView.cursor = idx
		}
	case ViewPlaylists:
		if m.playlists.viewing {
			// Viewing tracks within a playlist: header + column header + items
			listStart := 3
			pl := m.playlists.selectedPlaylist()
			if pl == nil {
				break
			}
			idx := m.playlists.trackScroll + (row - listStart)
			if idx >= 0 && idx < len(pl.Tracks) {
				if m.playlists.trackCursor == idx {
					t := pl.Tracks[idx]
					return m, m.playTrack(t)
				}
				m.playlists.trackCursor = idx
			}
		} else {
			// Viewing playlist list: header + column header + items
			listStart := 3
			idx := m.playlists.scroll + (row - listStart)
			if idx >= 0 && idx < len(m.playlists.playlists) {
				if m.playlists.cursor == idx {
					// Double-click: open playlist
					m.playlists.viewing = true
					m.playlists.trackCursor = 0
					m.playlists.trackScroll = 0
				}
				m.playlists.cursor = idx
			}
		}
	case ViewDownloads:
		listStart := 3
		if m.downloads.scroll > 0 {
			listStart++
		}
		idx := m.downloads.scroll + (row - listStart)
		if idx >= 0 && idx < len(m.downloads.items) {
			if m.downloads.cursor == idx {
				if t := m.downloads.selectedTrack(); t != nil {
					return m, m.playTrack(*t)
				}
			}
			m.downloads.cursor = idx
		}
	}
	return m, nil
}

// selectedTrack returns the currently highlighted track in any view.
func (m *Model) selectedTrack() *player.Track {
	switch m.view {
	case ViewSearch:
		return m.search.Selected()
	case ViewNowPlaying:
		return m.nowPlaying
	case ViewSuggestions:
		return m.suggestions.selectedTrack()
	case ViewQueue:
		return m.queueView.selectedTrack(m.queue)
	case ViewPlaylists:
		return m.playlists.selectedTrack()
	case ViewHistory:
		return m.histView.selectedTrack()
	case ViewDownloads:
		return m.downloads.selectedTrack()
	}
	return nil
}

func (m *Model) maybeFetchSuggestions() tea.Cmd {
	if m.nowPlaying == nil {
		return nil
	}
	// Don't re-fetch if we already have suggestions for this track
	if m.suggestions.forTrackID == m.nowPlaying.ID && !m.suggestions.loading {
		return nil
	}
	m.suggestions.forTrackID = m.nowPlaying.ID
	m.suggestions.loading = true
	m.suggestions.tracks = nil
	m.suggestions.err = nil
	m.suggestions.cursor = 0
	m.suggestions.scroll = 0
	m.suggestions.loadCount = 1
	t := *m.nowPlaying
	client := m.lastfm
	forID := t.ID
	ch, producer := startSuggestionFetch(client, t, nil, 25)
	go producer()
	return func() tea.Msg {
		return waitForSuggestion(ch, forID)()
	}
}

func (m *Model) loadMoreSuggestions() tea.Cmd {
	if m.nowPlaying == nil || m.suggestions.loading {
		return nil
	}
	m.suggestions.loading = true
	m.suggestions.loadCount++
	t := *m.nowPlaying
	client := m.lastfm
	forID := t.ID
	existing := make([]player.Track, len(m.suggestions.tracks))
	copy(existing, m.suggestions.tracks)
	// Each "load more" asks for 25 more results, skipping what we have
	ch, producer := startSuggestionFetch(client, t, existing, 25)
	go producer()
	return func() tea.Msg {
		return waitForSuggestion(ch, forID)()
	}
}

func (m *Model) openPicker(t player.Track) tea.Cmd {
	m.pickerTrack = &t
	return m.picker.open()
}
