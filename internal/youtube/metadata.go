package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
)

const artworkRenderVersion = 2

type Metadata struct {
	RenderVersion int    `json:"render_version,omitempty"`
	Description   string `json:"description,omitempty"`
	Thumbnail     string `json:"thumbnail,omitempty"`
	Channel       string `json:"channel,omitempty"`
	ChannelURL    string `json:"channel_url,omitempty"`
	ArtASCII      string `json:"art_ascii,omitempty"`
}

type metadataResult struct {
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	Channel     string `json:"channel"`
	Uploader    string `json:"uploader"`
	ChannelURL  string `json:"channel_url"`
}

func FetchMetadata(t player.Track) (Metadata, error) {
	if t.ID == "" {
		return Metadata{}, fmt.Errorf("empty track id")
	}
	if cached, ok := loadMetadataCache(t.ID); ok {
		return cached, nil
	}

	url := t.ID
	if t.Source != "soundcloud" && !strings.HasPrefix(url, "http") {
		url = fmt.Sprintf("https://www.youtube.com/watch?v=%s", t.ID)
	}

	cmd := exec.Command("yt-dlp",
		"--dump-json",
		"--no-playlist",
		"--quiet",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("yt-dlp metadata failed: %w", err)
	}

	var raw metadataResult
	if err := json.Unmarshal(out, &raw); err != nil {
		return Metadata{}, fmt.Errorf("failed to parse metadata: %w", err)
	}

	meta := Metadata{
		RenderVersion: artworkRenderVersion,
		Description:   strings.TrimSpace(raw.Description),
		Thumbnail:     strings.TrimSpace(raw.Thumbnail),
		Channel:       strings.TrimSpace(raw.Channel),
		ChannelURL:    strings.TrimSpace(raw.ChannelURL),
	}
	if meta.Channel == "" {
		meta.Channel = strings.TrimSpace(raw.Uploader)
	}

	if meta.Thumbnail == "" && t.Source != "soundcloud" && !strings.HasPrefix(t.ID, "http") {
		meta.Thumbnail = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", t.ID)
	}
	if art, err := fetchThumbnailArt(meta.Thumbnail, t); err == nil {
		meta.ArtASCII = art
	}

	saveMetadataCache(t.ID, meta)
	return meta, nil
}

func metadataCachePath(id string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "?", "_", "&", "_", "=", "_").Replace(id)
	return filepath.Join(config.Dir(), "cache", "metadata", safe+".json")
}

func loadMetadataCache(id string) (Metadata, bool) {
	path := metadataCachePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, false
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, false
	}
	if meta.RenderVersion != artworkRenderVersion {
		return Metadata{}, false
	}
	if meta.Thumbnail != "" && meta.ArtASCII == "" {
		return Metadata{}, false
	}
	return meta, true
}

func saveMetadataCache(id string, meta Metadata) {
	path := metadataCachePath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

func renderThumbnailANSI(url string, width, height int) (string, error) {
	if url == "" || width < 4 || height < 4 {
		return "", fmt.Errorf("no thumbnail")
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("thumbnail status %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	img, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return "", fmt.Errorf("empty image")
	}

	var b strings.Builder
	for y := 0; y < height; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		topY0 := bounds.Min.Y + (y*2)*bounds.Dy()/(height*2)
		topY1 := bounds.Min.Y + (y*2+1)*bounds.Dy()/(height*2)
		botY0 := bounds.Min.Y + (y*2+1)*bounds.Dy()/(height*2)
		botY1 := bounds.Min.Y + (y*2+2)*bounds.Dy()/(height*2)
		if topY1 <= topY0 {
			topY1 = topY0 + 1
		}
		if botY1 <= botY0 {
			botY1 = botY0 + 1
		}
		for x := 0; x < width; x++ {
			x0 := bounds.Min.X + x*bounds.Dx()/width
			x1 := bounds.Min.X + (x+1)*bounds.Dx()/width
			if x1 <= x0 {
				x1 = x0 + 1
			}
			top := averageBlockColor(img, x0, x1, topY0, topY1)
			bot := averageBlockColor(img, x0, x1, botY0, botY1)
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(rgbHex(boostArtworkColor(top)))).
				Background(lipgloss.Color(rgbHex(boostArtworkColor(bot))))
			b.WriteString(style.Render("▀"))
		}
	}
	return b.String(), nil
}

func fetchThumbnailArt(primaryURL string, t player.Track) (string, error) {
	var urls []string
	if primaryURL != "" {
		urls = append(urls, primaryURL)
	}
	if t.Source != "soundcloud" && !strings.HasPrefix(t.ID, "http") {
		urls = append(urls,
			fmt.Sprintf("https://i.ytimg.com/vi/%s/maxresdefault.jpg", t.ID),
			fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", t.ID),
			fmt.Sprintf("https://i.ytimg.com/vi/%s/mqdefault.jpg", t.ID),
		)
	}

	seen := make(map[string]bool, len(urls))
	var lastErr error
	for _, url := range urls {
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		art, err := renderThumbnailANSI(url, 30, 15)
		if err == nil && strings.TrimSpace(art) != "" {
			return art, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable thumbnail art")
	}
	return "", lastErr
}

func averageBlockColor(img image.Image, x0, x1, y0, y1 int) color.RGBA {
	var sumR, sumG, sumB float64
	var count float64
	for sy := y0; sy < y1; sy++ {
		for sx := x0; sx < x1; sx++ {
			r, g, b, _ := img.At(sx, sy).RGBA()
			sumR += float64(r >> 8)
			sumG += float64(g >> 8)
			sumB += float64(b >> 8)
			count++
		}
	}
	if count == 0 {
		return color.RGBA{}
	}
	return color.RGBA{
		R: uint8(sumR / count),
		G: uint8(sumG / count),
		B: uint8(sumB / count),
		A: 255,
	}
}

func boostArtworkColor(c color.RGBA) color.RGBA {
	r := float64(c.R) / 255.0
	g := float64(c.G) / 255.0
	b := float64(c.B) / 255.0

	// Slight contrast and saturation lift so thumbnail mosaics stay readable
	// instead of collapsing into muddy midtones in the terminal.
	r = math.Pow(r, 0.92)
	g = math.Pow(g, 0.92)
	b = math.Pow(b, 0.92)

	mean := (r + g + b) / 3
	const saturation = 1.18
	r = mean + (r-mean)*saturation
	g = mean + (g-mean)*saturation
	b = mean + (b-mean)*saturation

	return color.RGBA{
		R: uint8(clamp01(r) * 255),
		G: uint8(clamp01(g) * 255),
		B: uint8(clamp01(b) * 255),
		A: 255,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func rgbHex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
