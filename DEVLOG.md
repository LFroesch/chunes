## DevLog
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
