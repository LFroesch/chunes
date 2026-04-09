package ui

import (
	"fmt"
	"strings"

	"github.com/lucas/chunes/internal/lastfm"
	"github.com/lucas/chunes/internal/player"
	"github.com/lucas/chunes/internal/youtube"
)

type suggestionsModel struct {
	tracks  []player.Track
	cursor  int
	scroll  int
	loading bool
	err     error
	// Track ID that these suggestions are for (avoid re-fetching)
	forTrackID string
	// How many batches have been loaded (for "load more")
	loadCount int
}

// suggestionTrackMsg streams in one track at a time as they resolve.
type suggestionTrackMsg struct {
	track player.Track
	forID string
	ch    <-chan player.Track // nil when done
}

func newSuggestionsModel() suggestionsModel {
	return suggestionsModel{}
}

// startSuggestionFetch kicks off suggestion fetching — radio-style.
//
//  1. YouTube Radio (RD playlist) — primary, works for all genres including niche EDM/mixes
//  2. Same-channel search — for mix/DJ channels (source > 8min), fetch more from same uploader
//  3. Last.fm similar tracks — supplement for proper songs (skipped for long mixes)
func startSuggestionFetch(client *lastfm.Client, t player.Track, existingTracks []player.Track, maxResults int) (<-chan player.Track, func()) {
	ch := make(chan player.Track, 40)

	producer := func() {
		defer close(ch)

		seenID := make(map[string]bool)
		seenKey := make(map[string]bool)

		// Seed source track — multiple title variants to catch re-uploads
		seenID[t.ID] = true
		seenKey[dedupKey("", t.Title)] = true
		_, parsedTitle := youtube.ParseArtistTitle(t.Title, t.Artist)
		seenKey[dedupKey("", parsedTitle)] = true

		for _, et := range existingTracks {
			seenID[et.ID] = true
			seenKey[dedupKey("", et.Title)] = true
		}

		srcDur := parseDurationSecs(t.Duration)
		isMix := srcDur > 480 // >8min treated as DJ set / mix
		maxDur := 600
		if isMix {
			maxDur = 10800
		}

		isYT := t.Source != "soundcloud" && !strings.HasPrefix(t.ID, "http")
		parsedArtist, _ := youtube.ParseArtistTitle(t.Title, t.Artist)

		emit := func(r player.Track) bool {
			if seenID[r.ID] || seenKey[dedupKey("", r.Title)] {
				return false
			}
			secs := parseDurationSecs(r.Duration)
			if r.Duration == "" || r.Duration == "0:00" || r.Duration == "?" {
				return false
			}
			if secs > 0 && secs > maxDur {
				return false
			}
			seenID[r.ID] = true
			seenKey[dedupKey("", r.Title)] = true
			ch <- r
			return true
		}

		count := 0

		// ── Mixes: same-channel via channel URL ──────────────────────────
		// Fetch directly by channel URL — no name search, no ambiguity.
		if isYT && isMix && count < maxResults {
			channelURL := t.ChannelURL
			if channelURL == "" {
				channelURL, _ = youtube.GetChannelURL(t.ID)
			}
			if channelURL != "" {
				results, err := youtube.GetChannelVideos(channelURL, 15)
				if err == nil {
					for _, r := range results {
						if count >= maxResults {
							break
						}
						if emit(r) {
							count++
						}
					}
				}
			}
		}

		// ── Source 1: YouTube Radio ──────────────────────────────────────
		if isYT && count < maxResults {
			related, err := youtube.GetRelated(t.ID, 30)
			if err == nil {
				for _, r := range related {
					if count >= maxResults {
						break
					}
					if emit(r) {
						count++
					}
				}
			}
		}

		// ── Source 3: Last.fm similar tracks ────────────────────────────
		// Good supplement for mainstream/indie/rock. Skipped for mixes since
		// Last.fm won't have data for "60 Minute Breakcore Mix Vol. 3".
		if client != nil && !isMix && count < maxResults {
			similar, _ := client.GetSimilarTracks(parsedArtist, parsedTitle, 20)
			for _, s := range similar {
				if count >= maxResults {
					break
				}
				results, err := youtube.SearchExact(s.Artist+" "+s.Name, 3)
				if err != nil || len(results) == 0 {
					continue
				}
				for i := range results {
					if emit(results[i]) {
						count++
						break
					}
				}
			}
		}
	}

	return ch, producer
}

// waitForSuggestion reads one track from the channel and returns it as a msg.
func waitForSuggestion(ch <-chan player.Track, forID string) func() suggestionTrackMsg {
	return func() suggestionTrackMsg {
		track, ok := <-ch
		if !ok {
			return suggestionTrackMsg{forID: forID, ch: nil}
		}
		return suggestionTrackMsg{track: track, forID: forID, ch: ch}
	}
}

func (m *suggestionsModel) ensureVisible(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	n := len(m.tracks)
	for range 2 {
		effective := maxVisible
		if m.scroll > 0 {
			effective--
		}
		if m.scroll+effective < n {
			effective--
		}
		if effective < 1 {
			effective = 1
		}
		if maxScroll := n - effective; maxScroll > 0 {
			if m.scroll > maxScroll {
				m.scroll = maxScroll
			}
		} else {
			m.scroll = 0
		}
		if m.cursor < m.scroll {
			m.scroll = m.cursor
		}
		if m.cursor >= m.scroll+effective {
			m.scroll = m.cursor - effective + 1
		}
	}
}

func (m suggestionsModel) View(width, maxHeight int, rf ratingFunc, nowPlaying *player.Track) string {
	var b strings.Builder

	if nowPlaying == nil {
		b.WriteString(statusStyle.Render("  Play a track to see suggestions"))
		return b.String()
	}

	header := fmt.Sprintf("  Similar to: %s — %s", truncate(nowPlaying.Title, 30), truncate(nowPlaying.Artist, 20))
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render("  ✗ " + m.err.Error()))
		return b.String()
	}
	if len(m.tracks) == 0 && !m.loading {
		b.WriteString(statusStyle.Render("  No suggestions found for this track"))
		return b.String()
	}

	// Column widths
	fixedCols := 28
	remaining := width - fixedCols - 4
	if remaining < 30 {
		remaining = 30
	}
	titleW := remaining * 3 / 5
	artistW := remaining - titleW

	colHeader := fmt.Sprintf("  %s  %s  %-5s  %-5s  %5s", padRight("Title", titleW), padRight("Artist", artistW), "Dur", "Rate", "Plays")
	b.WriteString(colHeaderStyle.Render(colHeader))
	b.WriteString("\n")

	visibleLines := maxHeight - 4
	hasUp := m.scroll > 0
	if hasUp {
		visibleLines--
	}
	if m.scroll+visibleLines < len(m.tracks) {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.scroll + visibleLines
	if end > len(m.tracks) {
		end = len(m.tracks)
	}

	if hasUp {
		b.WriteString(dimStyle("  ↑ more") + "\n")
	}

	for i := m.scroll; i < end; i++ {
		t := m.tracks[i]
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("> ")
		}

		stars := renderRating(t.ID, rf)
		plays := renderPlays(t.ID, rf)
		line := fmt.Sprintf("%s  %s  %-5s  %s  %s",
			padRight(truncate(t.Title, titleW), titleW),
			padRight(truncate(t.Artist, artistW), artistW),
			t.Duration,
			stars,
			plays,
		)
		if i == m.cursor {
			b.WriteString(pointer + selectedStyle.Render(line))
		} else {
			b.WriteString(pointer + normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(m.tracks) {
		b.WriteString(dimStyle("  ↓ more"))
	}

	b.WriteString("\n")
	status := fmt.Sprintf("  %d suggestions", len(m.tracks))
	if m.loading {
		status += " (loading more...)"
	}
	b.WriteString(statusStyle.Render(status))

	return b.String()
}

// dedupKey normalizes a title into a lowercase key for deduplication.
// Uses title only (not artist) because the "artist" field is the YouTube channel
// name and varies across re-uploads of the same song.
// Strips presentation-only suffixes (lyrics, official video, etc.) but keeps
// remix/version identifiers so remixes don't collide with originals.
func dedupKey(_, title string) string {
	t := strings.ToLower(strings.TrimSpace(title))

	// Strip parenthetical/bracketed presentation suffixes (not remix info)
	presentationSuffixes := []string{
		"(official video)", "(official music video)", "(official audio)",
		"(lyric video)", "(lyrics)", "(lyric)", "(official lyric video)",
		"(visualizer)", "(audio)", "(hd)", "(hq)", "(4k)", "(official)",
		"(music video)", "(topic)", "(vevo)",
		"[official video]", "[official music video]", "[official audio]",
		"[lyric video]", "[lyrics]", "[lyric]", "[visualizer]", "[audio]",
		"[hd]", "[hq]", "[4k]", "[official]", "[music video]", "24/7" , "24 / 7",
		"- official video", "- official music video", "- official audio",
		"| official video", "| official music video",
	}
	for _, suffix := range presentationSuffixes {
		t = strings.ReplaceAll(t, suffix, "")
	}

	// Collapse whitespace and trim
	t = strings.Join(strings.Fields(t), " ")
	return t
}

func (m suggestionsModel) selectedTrack() *player.Track {
	if m.cursor >= 0 && m.cursor < len(m.tracks) {
		t := m.tracks[m.cursor]
		return &t
	}
	return nil
}
