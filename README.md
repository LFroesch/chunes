# chunes

Terminal music player built around YouTube playback, queueing, downloads, and radio-style discovery. `chunes` also understands SoundCloud search and links, but the main flow is YouTube-first.

## Install

Supported platforms: Linux and macOS. On Windows, use WSL.

Recommended:

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/chunes/main/install.sh | bash
```

Other options:

```bash
go install github.com/LFroesch/chunes@latest
make install
```

Run:

```bash
chunes
chunes --version
```

## Requirements

- `mpv` for playback
- `yt-dlp` for stream resolution and downloads
- `ffmpeg` for transcoded downloads like `mp3` or `opus`
- Last.fm API key is optional but improves recommendations

If required binaries are missing, `chunes` prints what is missing and how to install it.

## Config

Config is created on first run at `~/.config/chunes/config.json`.

```json
{
  "volume": 70,
  "download_dir": "~/Music/chunes",
  "audio_format": "source",
  "crossfade_secs": 8,
  "lastfm_api_key": "your-key-here"
}
```

Notes:

- `audio_format: "source"` keeps the best stream without re-encoding
- crossfade uses two `mpv` instances and blends track transitions
- without a Last.fm key, suggestions still work through YouTube Radio

## Tabs

| Tab | Purpose |
|-----|---------|
| Search | Search YouTube or SoundCloud |
| Playing | Current track, visualizer, description, and art |
| Queue | Up next, reorder, shuffle, repeat |
| Playlists | Local playlist management |
| History | Recent playback with ratings and counts |
| Suggest | Similar tracks and radio continuation |
| Downloads | Offline library |

## Features

- Queue music and play it immediately from the terminal
- Crossfade between tracks
- Download tracks for offline playback
- Rate tracks and keep play history
- Use YouTube Radio with Last.fm as an optional enhancement
- Switch between 12 visualizer modes on the Playing tab

## Controls

Press `?` in-app for the full keymap.

| Key | Action |
|-----|--------|
| `1-7` | Switch tabs |
| `space`, `enter` | Play or pause |
| `n`, `p`, `0` | Next, back, restart |
| `left/right`, `</>` | Seek |
| `+/-` | Volume |
| `/` | Search |
| `a` | Add selected track to queue |
| `d` | Download selected track |
| `s` | Save to playlist |
| `R` | Rate selected track |
| `v`, `V`, `C` | Cycle visualizer, randomize, auto-cycle |
| `q`, `ctrl+c` | Quit |

## Notes

- `chunes` relies on external media tools and network-backed providers, so first-run quality depends on local `mpv` and `yt-dlp` health.
- Downloads and config live under `~/.config/chunes` and the directory configured by `download_dir`.

## License

[AGPL-3.0](LICENSE)
