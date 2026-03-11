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

// startSuggestionFetch kicks off multi-source suggestion fetching.
//
// YouTube-native discovery first (works for all genres including niche):
//  1. YouTube Radio (RD playlist) — fast, decent starting point
//  2. Video tags → YouTube search (the key niche signal)
//  3. Title keywords → YouTube search (descriptive terms from the title)
//  4. Last.fm similar tracks (supplement — good for mainstream/indie/rock)
//  5. Last.fm tag discovery (supplement — when Last.fm has genre data)
func startSuggestionFetch(client *lastfm.Client, t player.Track, existingTracks []player.Track, maxResults int) (<-chan player.Track, func()) {
	ch := make(chan player.Track, 40)

	producer := func() {
		defer close(ch)

		seenID := make(map[string]bool)
		seenKey := make(map[string]bool)
		seenID[t.ID] = true
		seenKey[dedupKey(t.Artist, t.Title)] = true
		// Skip tracks we already have
		for _, et := range existingTracks {
			seenID[et.ID] = true
			seenKey[dedupKey(et.Artist, et.Title)] = true
		}

		// Duration filter: skip very long content (likely full albums/livestreams)
		maxDur := 600 // 10 minutes default cap for songs
		srcDur := parseDurationSecs(t.Duration)
		if srcDur > 600 {
			maxDur = 10800 // source is a mix, allow up to 3hrs
		}

		// Parse real artist/title from the YouTube video title
		parsedArtist, parsedTitle := youtube.ParseArtistTitle(t.Title, t.Artist)

		// Helper: send a track if not seen, returns true if sent
		emit := func(r player.Track) bool {
			if seenID[r.ID] || seenKey[dedupKey(r.Artist, r.Title)] {
				return false
			}
			secs := parseDurationSecs(r.Duration)
			if secs > 0 && secs > maxDur {
				return false
			}
			// Filter out YouTube livestreams (no duration = still live or stream archive)
			if r.Duration == "" || r.Duration == "0:00" {
				return false
			}
			seenID[r.ID] = true
			seenKey[dedupKey(r.Artist, r.Title)] = true
			ch <- r
			return true
		}

		count := 0
		isYT := t.Source != "soundcloud" && !strings.HasPrefix(t.ID, "http")

		// ── Source 1: YouTube Radio ──────────────────────────────────────
		// Fast first results — YouTube's own "related" mix
		if isYT {
			related, err := youtube.GetRelated(t.ID, 15)
			if err == nil {
				for _, r := range related {
					if count >= 10 { // cap at 10 from Radio — leave room for tag results
						break
					}
					if emit(r) {
						count++
					}
				}
			}
		}

		// ── Source 2: Video tags → YouTube search ────────────────────────
		// This is the niche signal. If a video is tagged "breakcore", searching
		// YouTube for "breakcore" gives you more breakcore. Simple and effective.
		var videoTags []string
		if isYT {
			videoTags, _ = youtube.GetVideoTags(t.ID)
		}

		for _, tag := range videoTags {
			if count >= maxResults {
				break
			}
			results, err := youtube.SearchExact(tag, 5)
			if err != nil || len(results) == 0 {
				continue
			}
			tagCount := 0
			for i := range results {
				if count >= maxResults {
					break
				}
				if emit(results[i]) {
					count++
					tagCount++
				}
				if tagCount >= 3 {
					break
				}
			}
		}

		// ── Source 3: Title keyword search ────────────────────────────────
		// Extract genre-descriptive words from the title itself.
		// "Breakcore Mix 2024 | High Energy DNB" → search "breakcore mix", "high energy dnb"
		keywords := extractTitleKeywords(t.Title, parsedArtist)
		for _, kw := range keywords {
			if count >= maxResults {
				break
			}
			results, err := youtube.SearchExact(kw, 5)
			if err != nil || len(results) == 0 {
				continue
			}
			kwCount := 0
			for i := range results {
				if count >= maxResults {
					break
				}
				if emit(results[i]) {
					count++
					kwCount++
				}
				if kwCount >= 3 {
					break
				}
			}
		}

		// ── Source 4: Last.fm similar tracks (supplement) ────────────────
		if client != nil && count < maxResults {
			similar, _ := client.GetSimilarTracks(parsedArtist, parsedTitle, 15)
			for _, s := range similar {
				if count >= maxResults {
					break
				}
				query := s.Artist + " " + s.Name
				results, err := youtube.SearchExact(query, 3)
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

		// ── Source 5: Last.fm tag discovery (supplement) ──────────────────
		if client != nil && count < maxResults {
			lfmTags, _ := client.GetTopTags(parsedArtist, parsedTitle)
			for _, tag := range lfmTags {
				if count >= maxResults {
					break
				}
				tagTracks, err := client.GetTagTopTracks(tag, 10)
				if err != nil || len(tagTracks) == 0 {
					continue
				}
				tagCount := 0
				for _, s := range tagTracks {
					if count >= maxResults {
						break
					}
					query := s.Artist + " " + s.Name
					results, err := youtube.SearchExact(query, 3)
					if err != nil || len(results) == 0 {
						continue
					}
					for i := range results {
						if emit(results[i]) {
							count++
							tagCount++
							break
						}
					}
					if tagCount >= 2 {
						break
					}
				}
			}
		}
	}

	return ch, producer
}

// extractTitleKeywords pulls genre/descriptive keywords from a video title,
// stripping the artist name and common noise words. Returns up to 3 search queries.
func extractTitleKeywords(title, artist string) []string {
	// Remove artist name from title
	t := title
	if artist != "" {
		t = strings.ReplaceAll(strings.ToLower(t), strings.ToLower(artist), "")
	}

	// Remove common noise: suffixes, brackets, separators
	for _, remove := range []string{
		"(official video)", "(official music video)", "(official audio)",
		"(lyric video)", "(lyrics)", "(visualizer)", "(audio)",
		"[official video]", "[official music video]", "[official audio]",
		"[lyric video]", "[lyrics]", "[visualizer]", "[audio]",
		"official", "video", "audio", "lyric", "lyrics",
		"full album", "full ep",
	} {
		t = strings.ReplaceAll(strings.ToLower(t), remove, "")
	}

	// Split on common separators: |, /, -, ·
	parts := strings.FieldsFunc(t, func(r rune) bool {
		return r == '|' || r == '/' || r == '·' || r == '(' || r == ')' || r == '[' || r == ']'
	})

	var keywords []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Skip very short fragments or just numbers (years etc)
		if len(p) < 3 {
			continue
		}
		// Skip if it's just a number (year like "2024")
		isNum := true
		for _, c := range p {
			if c < '0' || c > '9' {
				isNum = false
				break
			}
		}
		if isNum {
			continue
		}
		keywords = append(keywords, p)
		if len(keywords) >= 3 {
			break
		}
	}
	return keywords
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

// dedupKey normalizes artist+title into a lowercase key for deduplication.
// Strips common YouTube suffixes (official video, lyric video, etc.) and
// normalizes whitespace so the same song from different uploaders dedupes.
func dedupKey(artist, title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	a := strings.ToLower(strings.TrimSpace(artist))
	// Strip common YouTube video suffixes
	for _, suffix := range []string{
		"(official video)", "(official music video)", "(official audio)",
		"(lyric video)", "(lyrics)", "(official lyric video)",
		"(visualizer)", "(audio)", "(hd)", "(hq)",
		"[official video]", "[official music video]", "[official audio]",
		"[lyric video]", "[lyrics]", "[visualizer]", "[audio]",
		"- official video", "- official music video", "- official audio",
		"| official video", "| official music video",
		"(original mix)", "[original mix]",
		"(extended mix)", "[extended mix]",
		"(radio edit)", "[radio edit]",
		"ft.", "feat.", "featuring",
	} {
		t = strings.ReplaceAll(t, suffix, "")
	}
	// Collapse whitespace and trim
	fields := strings.Fields(t)
	t = strings.Join(fields, " ")
	fields = strings.Fields(a)
	a = strings.Join(fields, " ")
	return a + "|" + t
}

func (m suggestionsModel) selectedTrack() *player.Track {
	if m.cursor >= 0 && m.cursor < len(m.tracks) {
		t := m.tracks[m.cursor]
		return &t
	}
	return nil
}
