package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lucas/chunes/internal/config"
	"github.com/lucas/chunes/internal/player"
)

type Metadata struct {
	Description string `json:"description,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	ChannelURL  string `json:"channel_url,omitempty"`
	ArtASCII    string `json:"art_ascii,omitempty"`
}

type metadataResult struct {
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
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
		Description: strings.TrimSpace(raw.Description),
		Thumbnail:   strings.TrimSpace(raw.Thumbnail),
		ChannelURL:  strings.TrimSpace(raw.ChannelURL),
	}

	if meta.Thumbnail == "" && t.Source != "soundcloud" && !strings.HasPrefix(t.ID, "http") {
		meta.Thumbnail = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", t.ID)
	}
	if art, err := renderThumbnailASCII(meta.Thumbnail, 48, 22); err == nil {
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

func renderThumbnailASCII(url string, width, height int) (string, error) {
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

	// 16-step luma ramp from sparse to dense for smoother gradients.
	const ramp = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$"
	// Box-average sample: average all source pixels in the cell, gamma-correct,
	// then map to ramp. Aspect-correct: terminal cells are ~2:1 tall/wide, so
	// each cell already covers more vertical pixels naturally via height/width.
	var b strings.Builder
	for y := 0; y < height; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		y0 := bounds.Min.Y + y*bounds.Dy()/height
		y1 := bounds.Min.Y + (y+1)*bounds.Dy()/height
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < width; x++ {
			x0 := bounds.Min.X + x*bounds.Dx()/width
			x1 := bounds.Min.X + (x+1)*bounds.Dx()/width
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum float64
			var n int
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					r, g, bl, _ := img.At(sx, sy).RGBA()
					sum += 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)
					n++
				}
			}
			luma := sum / float64(n) / 65535.0
			// Gamma 2.2 correction — perceived brightness, not linear light.
			luma = math.Pow(luma, 1.0/2.2)
			idx := int(luma*float64(len(ramp)-1) + 0.5)
			if idx < 0 {
				idx = 0
			}
			if idx >= len(ramp) {
				idx = len(ramp) - 1
			}
			b.WriteByte(ramp[idx])
		}
	}
	return b.String(), nil
}

