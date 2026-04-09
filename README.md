# Chunes

Terminal music player that streams from YouTube and SoundCloud. Playlists, play history, offline downloads, track suggestions, and a bunch of visualizers — all from your terminal.

## Quick Install

Recommended (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/chunes/main/install.sh | bash
```

Or download a binary from [GitHub Releases](https://github.com/LFroesch/chunes/releases).

Or install with Go:

```bash
go install github.com/LFroesch/chunes@latest
```

Or build from source:

```bash
make install
```

Command:

```bash
chunes
```
## Requirements

- Go 1.23+
- [mpv](https://mpv.io/) — audio playback
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — stream resolution + downloads
- Last.fm API key (optional — improves suggestions but not required)

## Install & Run

```bash
go build -o chunes .
./chunes
```

## Config

Config lives at `~/.config/chunes/config.json` and is created automatically on first run.

```json
{
  "volume": 70,
  "download_dir": "~/Music/chunes",
  "audio_format": "mp3",
  "crossfade_secs": 8,
  "lastfm_api_key": "your-key-here"
}
```

Crossfade runs two mpv instances and blends between them during track transitions.

Last.fm key is optional — suggestions work without it using YouTube Radio. Adding a key improves results with Last.fm similar-track data. Get one free at https://www.last.fm/api/account/create

## Tabs

| # | Tab | What it does |
|---|-----|-------------|
| 1 | Search | YouTube/SoundCloud search (Tab to switch source) |
| 2 | Playing | Full-screen visualizer + track info |
| 3 | Queue | Up next — shuffle, repeat, reorder |
| 4 | Playlists | Local JSON playlists — create, rename, reorder |
| 5 | History | Play history with ratings, play counts, and sorting |
| 6 | Suggest | Similar tracks (YouTube Radio + Last.fm fallback) |
| 7 | Downloads | Offline library — downloaded tracks play locally |

## Visualizers

11 styles, cycle with `v` or randomize with `V`:

bars, lissajous, scope, radial, spiral, starfield, flame, plasma, ring, donut, moire

Auto-cycle mode with `C`. Adjust energy boost with `[` / `]`.

## Features

- **Crossfade** — smooth transitions between tracks (configurable duration)
- **Shuffle & repeat** — Fisher-Yates shuffle (every track once per cycle), repeat one/all/off
- **Ratings** — rate tracks 0-5 stars, persisted across sessions
- **Mouse support** — click to select, scroll to navigate, click progress bar to seek
- **Offline playback** — downloaded tracks skip network resolution entirely
- **Seeking** — arrow keys ±5s, `<`/`>` ±30s, `0` to restart

## Key Bindings

Press `?` in-app for the full list. Hints at the bottom of each tab show context-relevant keys.

## License

AGPL-3.0 — see [LICENSE](LICENSE).
