## DevLog
### 2026-05-01: UX sweep — preflight, ASCII, meta toggle, grouped help
- **Preflight checks** (`main.go`): aggregate all missing deps and print platform-aware install hints (brew/apt/dnf/pacman). `parec` is now an optional dep — runs without spectrum visualizer when absent.
- **Better ASCII art** (`internal/youtube/metadata.go`): bumped render size to 48×22, added a 16-step luma ramp, gamma 2.2 correction, and box-average sampling instead of single-pixel sampling. Cached art is denser and more readable.
- **Now Playing meta/viz toggle** (`internal/ui/{app,nowplaying,help}.go`): `m` toggles the body of the Now Playing tab between the metadata panel (artwork + description) and the full-height visualizer. Header (title/artist/time/rating) stays fixed.
- **Help page sections** (`internal/ui/help.go`): regrouped flat keybind list into Playback / Navigation / Tracks / Playlists / Visualizer / Help sections with headers; help scroll now uses `helpTotalLines()`.

### 2026-05-01: Added source-preserving download mode
- **Best-quality offline path**: `audio_format: "source"` now downloads the best available audio stream without re-encoding, preserving the original codec/container from `yt-dlp`.
- **Path handling fixed**: offline playback and delete now use the actual saved file path from `downloads.json` instead of guessing an extension, so source-preserved files work reliably.
- **Defaults updated**: generated config now defaults to `source`; README explains `source` versus forced formats like `opus`/`mp3`, and notes that `ffmpeg` is only needed for transcoding.
- Files: `internal/download/download.go`, `internal/ui/download.go`, `internal/ui/app.go`, `internal/config/config.go`, `README.md`

### 2026-04-30: Default download format switched to Opus
- **Higher default download quality**: changed the generated config default from `mp3` to `opus`, which better matches YouTube's common audio streams and avoids the weakest default transcode path for offline playback.
- **Docs clarified**: config example now shows `audio_format: "opus"` and explains that existing user configs are not rewritten automatically.
- Files: `internal/config/config.go`, `README.md`

### 2026-04-30: YouTube-first playback UX sweep
- **Now Playing context**: added cached `yt-dlp` metadata fetches plus in-terminal ASCII thumbnail rendering, so the player now shows track description, channel context, and artwork instead of just a title line.
- **Radio improvements**: suggestion fetches now widen their YouTube pull when loading more, and fall back to additional exact-track searches so queue refill behaves more like a radio continuation.
- **Controls**: `p` now replays the current track, and volume can be pushed to 200% through mpv for loudness-starved sources.
- **Download robustness**: download startup now checks for `ffmpeg`, creates the output directory proactively, drains `yt-dlp` pipes concurrently, and surfaces the real download error in the status bar.
- Files: `internal/ui/app.go`, `internal/ui/nowplaying.go`, `internal/ui/suggestions.go`, `internal/youtube/metadata.go`, `internal/player/mpv.go`, `internal/ui/help.go`, `internal/ui/text.go`

### 2026-04-08: UX + visualizer batch
- **Pause fix**: split space/enter — space now always toggles pause globally (was consumed by view-specific enter handlers, so pause broke on non-NowPlaying pages)
- **Link support**: pasting a YouTube/SoundCloud URL into search now resolves metadata via yt-dlp and plays directly (no search needed)
- **Queue end**: when queue empties, auto-fills from suggestions and starts playing; if shuffle is on, re-shuffles the new batch
- **Viz AGC**: auto-gain control adjusts boost so average energy stays around 35% — eliminates flat bars on quiet music, prevents clipping on loud music. Toggle with `G`; manual `[`/`]` disables AGC
- **New viz style**: "mirror" (style 12) — spectrum bars grow outward from the vertical center in both directions
- **Help**: updated bindings to reflect space/enter split and AGC keybind

### 2026-03-20: Radio engine audit + plan
Audited full suggestion pipeline (suggestions.go, related.go, lastfm.go, app.go). Found 9 issues: load-more is a no-op (re-runs same sources), no junk filtering, title keyword extraction too crude, no ranking, SoundCloud gets YouTube suggestions, no refresh keybind, duration "?" passes filter, no feedback loop, goroutine leak on song change. Planned Spotify API integration as primary discovery source with YouTube Radio fallback and continuous chaining. Wrote full architecture plan.

### 2026-03-17: README refresh + help.go tab count fix
Rewrote README to reflect current state. Fixed `1-6` → `1-7` in help.go and CLAUDE.md.

### 2026-03-17: Bug fix — song stuck loading on quick start
`loadingTrackID` wasn't cleared in `streamURLMsg` error path. Added safety timeout (15s).
Files: internal/ui/app.go

### 2026-03-16: Bug fix batch — 4 playback/queue bugs
Queue double-pop, session resume race, phantom stop, dupe prevention.
Files: internal/ui/app.go, internal/player/queue.go, internal/playlist/playlist.go

### 2026-03-12: Now Playing tab — full-screen visualizer
New tab 2 with 6 viz styles using braille 2D canvas. Removed old 3-row bottom visualizer. Tick rate 200→100ms.
Files: internal/ui/nowplaying.go, visualizer.go, app.go

### 2026-03-11: UX polish batch — 7 fixes
`/` global search, `a` reroutes, 3-tier hints, scroll overflow, suggestion dedup, YouTube Radio as primary source, action fallbacks.

### 2026-03-11: Suggestions tab + shuffle fix
Pandora-style suggestions via Last.fm + YouTube. Fisher-Yates queue shuffle. New tab 2, all others shifted.

### 2026-03-11: Fix queue shuffle
Proper Fisher-Yates shuffled index list. PeekNext reads from shuffle order.
Files: internal/player/queue.go
