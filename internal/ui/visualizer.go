package ui

import (
	"fmt"
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucas/chunes/internal/player"
)

const vizBandCount = 24

// Block characters for partial fill within a single row cell
// Index 0 = empty, 1-8 = increasing fill
var blockChars = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

var vizStyleNames = []string{"bars", "lissajous", "scope", "radial", "spiral", "starfield", "flame", "plasma", "ring", "donut", "moire", "mirror"}

// vizGradientFor returns a color interpolated across the band range.
func vizGradientFor(bandIdx, totalBands int) lipgloss.Color {
	// Gradient: primaryColor (#FF6AC1) → secondaryColor (#9B72FF) → accentColor (#72F1B8)
	frac := float64(bandIdx) / float64(totalBands-1)
	var r, g, b int
	if frac < 0.5 {
		// primary → secondary
		t := frac * 2
		r = int(float64(0xFF)*(1-t) + float64(0x9B)*t)
		g = int(float64(0x6A)*(1-t) + float64(0x72)*t)
		b = int(float64(0xC1)*(1-t) + float64(0xFF)*t)
	} else {
		// secondary → accent
		t := (frac - 0.5) * 2
		r = int(float64(0x9B)*(1-t) + float64(0x72)*t)
		g = int(float64(0x72)*(1-t) + float64(0xF1)*t)
		b = int(float64(0xFF)*(1-t) + float64(0xB8)*t)
	}
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b))
}

// hasBandData returns true if any band level is non-zero.
func hasBandData(levels [player.BandCount]float64) bool {
	for _, v := range levels {
		if v > 0.001 {
			return true
		}
	}
	return false
}

// updateVizBands updates the visualizer bands using real per-band frequency levels.
func updateVizBands(bands *[vizBandCount]float64, bandLevels [player.BandCount]float64, rms float64, tick int, playing, paused bool, boost float64) {
	if !playing || paused {
		for i := range bands {
			bands[i] *= 0.65
			if bands[i] < 0.01 {
				bands[i] = 0
			}
		}
		return
	}

	if hasBandData(bandLevels) {
		// Logarithmic mapping: more display bands for low/mid frequencies
		// Source bands 0-4 (sub-bass through upper-mid) get ~70% of display bands
		// bandWeights controls how many display bands each source band gets
		bandWeights := [player.BandCount]float64{3, 4, 4, 4, 3, 2, 2, 2} // sum=24
		var cumulative [player.BandCount + 1]float64
		for b := 0; b < player.BandCount; b++ {
			cumulative[b+1] = cumulative[b] + bandWeights[b]
		}

		for i := range bands {
			// Find which source band(s) this display band maps to
			pos := float64(i) / float64(vizBandCount-1) * cumulative[player.BandCount]
			srcBand := 0
			for b := 0; b < player.BandCount-1; b++ {
				if pos >= cumulative[b] && pos < cumulative[b+1] {
					srcBand = b
					break
				}
				if b == player.BandCount-2 {
					srcBand = player.BandCount - 1
				}
			}
			// Interpolate within the source band region
			lo := srcBand
			hi := lo + 1
			if hi >= player.BandCount {
				hi = player.BandCount - 1
			}
			localFrac := (pos - cumulative[lo]) / bandWeights[lo]
			if localFrac > 1 {
				localFrac = 1
			}
			target := bandLevels[lo]*(1-localFrac) + bandLevels[hi]*localFrac

			// Boost to make dynamics more visible (most music sits 0.0-0.5)
			target = math.Min(target*boost, 1.0)

			// Near-instant attack, moderate decay — transients pop
			if target > bands[i] {
				bands[i] = bands[i]*0.02 + target*0.98
			} else {
				bands[i] = bands[i]*0.45 + target*0.55
			}
		}
	} else {
		energy := math.Min(rms*boost, 1.0) // boost RMS fallback too
		ft := float64(tick) * 0.2
		for i := range bands {
			fi := float64(i)
			fn := float64(vizBandCount)
			freqPos := fi / fn
			bassBoost := 1.0 - freqPos*0.5
			jitter := math.Sin(ft+fi*1.3)*0.2 + math.Cos(ft*1.7+fi*0.6)*0.15
			target := energy*bassBoost + jitter*energy
			if target < 0 {
				target = 0
			}
			if target > 1 {
				target = 1
			}
			if target > bands[i] {
				bands[i] = bands[i]*0.02 + target*0.98
			} else {
				bands[i] = bands[i]*0.45 + target*0.55
			}
		}
	}
}
