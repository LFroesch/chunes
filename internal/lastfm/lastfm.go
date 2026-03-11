package lastfm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://ws.audioscrobbler.com/2.0/"

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

type SimilarTrack struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
}

// GetSimilarTracks returns tracks similar to the given track.
func (c *Client) GetSimilarTracks(artist, track string, limit int) ([]SimilarTrack, error) {
	params := url.Values{
		"method":  {"track.getsimilar"},
		"artist":  {artist},
		"track":   {track},
		"limit":   {fmt.Sprintf("%d", limit)},
		"api_key": {c.apiKey},
		"format":  {"json"},
	}
	resp, err := c.http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		SimilarTracks struct {
			Track []struct {
				Name   string `json:"name"`
				Artist struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"track"`
		} `json:"similartracks"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != 0 {
		return nil, fmt.Errorf("last.fm: %s", result.Message)
	}

	tracks := make([]SimilarTrack, 0, len(result.SimilarTracks.Track))
	for _, t := range result.SimilarTracks.Track {
		tracks = append(tracks, SimilarTrack{
			Name:   t.Name,
			Artist: t.Artist.Name,
		})
	}
	return tracks, nil
}

// GetTopTags returns the top tags for a track (genre/mood labels).
func (c *Client) GetTopTags(artist, track string) ([]string, error) {
	params := url.Values{
		"method":  {"track.gettoptags"},
		"artist":  {artist},
		"track":   {track},
		"api_key": {c.apiKey},
		"format":  {"json"},
	}
	resp, err := c.http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		TopTags struct {
			Tag []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tag"`
		} `json:"toptags"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != 0 {
		return nil, fmt.Errorf("last.fm: %s", result.Message)
	}

	var tags []string
	for _, t := range result.TopTags.Tag {
		if t.Count < 10 {
			continue // skip low-confidence tags
		}
		tags = append(tags, t.Name)
		if len(tags) >= 5 {
			break
		}
	}
	return tags, nil
}

// GetTagTopTracks returns popular tracks for a given tag/genre.
func (c *Client) GetTagTopTracks(tag string, limit int) ([]SimilarTrack, error) {
	params := url.Values{
		"method":  {"tag.gettoptracks"},
		"tag":     {tag},
		"limit":   {fmt.Sprintf("%d", limit)},
		"api_key": {c.apiKey},
		"format":  {"json"},
	}
	resp, err := c.http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Tracks struct {
			Track []struct {
				Name   string `json:"name"`
				Artist struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"track"`
		} `json:"tracks"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != 0 {
		return nil, fmt.Errorf("last.fm: %s", result.Message)
	}

	tracks := make([]SimilarTrack, 0, len(result.Tracks.Track))
	for _, t := range result.Tracks.Track {
		tracks = append(tracks, SimilarTrack{
			Name:   t.Name,
			Artist: t.Artist.Name,
		})
	}
	return tracks, nil
}

// GetSimilarArtists returns artists similar to the given artist.
// Fallback when track-level similarity has no results.
func (c *Client) GetSimilarArtists(artist string, limit int) ([]string, error) {
	params := url.Values{
		"method":  {"artist.getsimilar"},
		"artist":  {artist},
		"limit":   {fmt.Sprintf("%d", limit)},
		"api_key": {c.apiKey},
		"format":  {"json"},
	}
	resp, err := c.http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		SimilarArtists struct {
			Artist []struct {
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"similarartists"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != 0 {
		return nil, fmt.Errorf("last.fm: %s", result.Message)
	}

	artists := make([]string, 0, len(result.SimilarArtists.Artist))
	for _, a := range result.SimilarArtists.Artist {
		artists = append(artists, a.Name)
	}
	return artists, nil
}
