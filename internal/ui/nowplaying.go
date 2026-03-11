package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func nowPlayingHints() []helpBinding {
	return []helpBinding{
		{"v/V", "viz/random"},
		{"C", "auto-cycle"},
		{"[]", "energy ↓↑"},
		{"Space", "pause"},
		{"n", "next"},
		{"+/-", "vol"},
		{"←/→", "seek"},
		{"R", "rate"},
		{"?", "help"},
	}
}

// viewNowPlaying renders the Now Playing tab: track info + full-height visualizer.
func (m Model) viewNowPlaying(width, height int) string {
	var lines []string

	track := m.nowPlaying
	if track == nil {
		empty := lipgloss.NewStyle().Foreground(dimColor).Render("  No track playing — search and play something!")
		lines = append(lines, "")
		lines = append(lines, empty)
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:height], "\n")
	}

	// Track info section (3 lines: blank, title/artist, progress)
	icon := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("▶")
	if m.activeMPV().Paused {
		icon = lipgloss.NewStyle().Foreground(warnColor).Bold(true).Render("⏸")
	}

	title := trackTitleStyle.Render(truncate(track.Title, width/2))
	artist := trackArtistStyle.Render(truncate(track.Artist, width/3))

	// Rating
	ratingStr := ""
	if rating, _ := m.history.RatingFor(track.ID); rating > 0 {
		stars := strings.Repeat("★", rating) + strings.Repeat("☆", 5-rating)
		ratingStr = "  " + lipgloss.NewStyle().Foreground(warnColor).Render(stars)
	}

	// Time
	position := m.activeMPV().CachedPosition
	duration := m.activeMPV().CachedDuration
	timeStr := lipgloss.NewStyle().Foreground(dimColor).Render(
		fmt.Sprintf("%s / %s", formatSeconds(position), formatSeconds(duration)))

	trackLine := fmt.Sprintf("  %s  %s  %s%s", icon, title, artist, ratingStr)
	timeLine := fmt.Sprintf("  %s", timeStr)

	lines = append(lines, "")
	lines = append(lines, lipgloss.PlaceHorizontal(width, lipgloss.Center, trackLine))
	lines = append(lines, lipgloss.PlaceHorizontal(width, lipgloss.Center, timeLine))
	lines = append(lines, "")

	// Visualizer fills the rest
	vizHeight := height - len(lines)
	if vizHeight < 2 {
		vizHeight = 2
	}

	vizContent := renderFullViz(m.vizBands, m.vizStyle, width, vizHeight, m.vizTick)
	vizLines := strings.Split(vizContent, "\n")
	for i := 0; i < vizHeight; i++ {
		if i < len(vizLines) {
			lines = append(lines, vizLines[i])
		} else {
			lines = append(lines, "")
		}
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// ── Braille canvas ──────────────────────────────────────────────────────────

// brailleCanvas is a 2D pixel grid that renders to braille characters.
// Each cell is 2 wide × 4 tall in braille sub-pixels.
type brailleCanvas struct {
	w, h   int       // pixel dimensions
	pixels [][]bool  // [y][x]
}

func newBrailleCanvas(charW, charH int) *brailleCanvas {
	pw, ph := charW*2, charH*4
	pixels := make([][]bool, ph)
	for i := range pixels {
		pixels[i] = make([]bool, pw)
	}
	return &brailleCanvas{w: pw, h: ph, pixels: pixels}
}

func (c *brailleCanvas) set(x, y int) {
	if x >= 0 && x < c.w && y >= 0 && y < c.h {
		c.pixels[y][x] = true
	}
}

// line draws a Bresenham line between two points.
func (c *brailleCanvas) line(x0, y0, x1, y1 int) {
	dx := x1 - x0
	dy := y1 - y0
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		c.set(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// render converts the pixel grid to braille characters with a color function.
// colorFn maps (charX, charY) to a lipgloss style.
func (c *brailleCanvas) render(charW, charH int, colorFn func(cx, cy int) lipgloss.Style) string {
	// Braille dot positions within a 2x4 cell:
	// (0,0)=0x01  (1,0)=0x08
	// (0,1)=0x02  (1,1)=0x10
	// (0,2)=0x04  (1,2)=0x20
	// (0,3)=0x40  (1,3)=0x80
	dotMap := [4][2]rune{
		{0x01, 0x08},
		{0x02, 0x10},
		{0x04, 0x20},
		{0x40, 0x80},
	}

	var lines []string
	for cy := 0; cy < charH; cy++ {
		var line strings.Builder
		for cx := 0; cx < charW; cx++ {
			var bits rune
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					px := cx*2 + dx
					py := cy*4 + dy
					if px < c.w && py < c.h && c.pixels[py][px] {
						bits |= dotMap[dy][dx]
					}
				}
			}
			ch := string(rune(0x2800) + bits)
			style := colorFn(cx, cy)
			line.WriteString(style.Render(ch))
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// ── Full-height visualizer styles ───────────────────────────────────────────

// renderFullViz renders a full-height visualizer for the Now Playing tab.
func renderFullViz(bands [vizBandCount]float64, style int, width, height, tick int) string {
	if width < 6 || height < 2 {
		return ""
	}

	switch style {
	case 0: // bars
		return renderVizBars(bands, width, height)
	case 1: // lissajous
		return renderVizLissajous(bands, width, height, tick)
	case 2: // oscilloscope
		return renderVizOscilloscope(bands, width, height, tick)
	case 3: // radial
		return renderVizRadial(bands, width, height, tick)
	case 4: // spiral
		return renderVizSpiral(bands, width, height, tick)
	case 5: // starfield
		return renderVizStarfield(bands, width, height, tick)
	case 6: // flame
		return renderVizFlame(bands, width, height, tick)
	case 7: // plasma
		return renderVizPlasma(bands, width, height, tick)
	case 8: // ring
		return renderVizRing(bands, width, height, tick)
	case 9: // donut
		return renderVizDonut(bands, width, height, tick)
	case 10: // moire
		return renderVizMoire(bands, width, height, tick)
	}
	return ""
}

// ── Style 1: Full-height spectrum bars ──────────────────────────────────────

func renderVizBars(bands [vizBandCount]float64, width, height int) string {
	// Scale bar width to terminal: wider terminals get fatter bars
	barWidth := 1
	if width >= 120 {
		barWidth = 3
	} else if width >= 80 {
		barWidth = 2
	}
	gap := 1
	bandSlot := barWidth + gap
	maxBands := (width - 4) / bandSlot
	if maxBands > vizBandCount {
		maxBands = vizBandCount
	}
	if maxBands < 4 {
		maxBands = 4
	}

	vizWidth := maxBands * bandSlot
	padLeft := (width - vizWidth) / 2
	prefix := strings.Repeat(" ", padLeft)

	totalLevels := height * 8

	var lines []string
	for row := height - 1; row >= 0; row-- {
		var line strings.Builder
		line.WriteString(prefix)
		for b := 0; b < maxBands; b++ {
			// Logarithmic compression: tames loud peaks while keeping dynamics
			val := bands[b]
			if val > 0 {
				val = math.Log1p(val*4) / math.Log1p(4) // log compress into 0-1
			}
			if val > 0.92 {
				val = 0.92
			}
			level := int(val * float64(totalLevels))
			if level < 0 {
				level = 0
			}
			if level > totalLevels {
				level = totalLevels
			}

			rowBase := row * 8
			cellFill := level - rowBase
			if cellFill < 0 {
				cellFill = 0
			}
			if cellFill > 8 {
				cellFill = 8
			}

			ch := blockChars[cellFill]
			color := vizGradientFor(b, maxBands)
			styled := lipgloss.NewStyle().Foreground(color).Render(ch)
			for w := 0; w < barWidth; w++ {
				line.WriteString(styled)
			}
			line.WriteString(strings.Repeat(" ", gap))
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// ── Style 2: Lissajous curve ────────────────────────────────────────────────

func renderVizLissajous(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	totalEnergy := (lowEnergy + midEnergy + highEnergy) / 3

	// Frequency ratios — smoothly interpolated, not floor'd (less jumpy)
	a := 1.0 + lowEnergy*4
	b := 1.0 + midEnergy*3
	delta := float64(tick)*0.05 + highEnergy*math.Pi*2

	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	// Fill most of the canvas — use 90% at full energy
	radiusX := float64(canvas.w) * (0.2 + totalEnergy*0.25)
	radiusY := float64(canvas.h) * (0.2 + totalEnergy*0.25)

	// More steps for smoother curves, draw multiple phase-offset passes for thickness
	steps := 1000
	passes := 3
	for p := 0; p < passes; p++ {
		phaseOff := float64(p) * 0.015
		prevX, prevY := -1, -1
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps) * 2 * math.Pi
			segBand := int(float64(i) / float64(steps) * float64(vizBandCount-1))
			if segBand >= vizBandCount {
				segBand = vizBandCount - 1
			}
			distort := 1.0 + bands[segBand]*0.4

			x := centerX + radiusX*distort*math.Sin(a*t+delta+phaseOff)
			y := centerY + radiusY*distort*math.Sin(b*t+phaseOff)
			px, py := int(x), int(y)
			if prevX >= 0 {
				canvas.line(prevX, prevY, px, py)
			}
			prevX, prevY = px, py
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		dist := math.Sqrt(dx*dx+dy*dy) / math.Sqrt(float64(cw*cw+ch*ch)/4)
		return lipgloss.NewStyle().Foreground(vizGradientFor(int(dist*float64(vizBandCount-1)), vizBandCount))
	})
}

// ── Style 3: Oscilloscope ───────────────────────────────────────────────────

func renderVizOscilloscope(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	centerY := float64(canvas.h) / 2
	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	bassEnergy := avgBands(bands[:], 0, 8)

	// Draw multiple waveform lines with different frequency emphasis
	numWaves := 3
	for w := 0; w < numWaves; w++ {
		prevPy := -1
		freqMult := 1.0 + float64(w)*0.8
		ampScale := 1.0 - float64(w)*0.25
		for x := 0; x < canvas.w; x++ {
			frac := float64(x) / float64(canvas.w-1)

			yOff := 0.0
			for b := 0; b < vizBandCount; b++ {
				freq := (0.04 + float64(b)*0.02) * freqMult
				phase := float64(tick) * (0.25 + float64(b)*0.06)
				yOff += bands[b] * math.Sin(frac*float64(canvas.w)*freq+phase)
			}
			yOff = yOff / float64(vizBandCount) * float64(canvas.h) * (0.7 + totalEnergy*0.7) * ampScale

			py := int(centerY + yOff)
			if py < 0 {
				py = 0
			}
			if py >= canvas.h {
				py = canvas.h - 1
			}

			canvas.set(x, py)
			canvas.set(x, py-1)
			canvas.set(x, py+1)
			if prevPy >= 0 {
				canvas.line(x-1, prevPy, x, py)
			}
			prevPy = py
		}
	}

	// Animated scatter particles that drift around the waveform area
	numParticles := 30 + int(totalEnergy*60)
	for i := 0; i < numParticles; i++ {
		seed := uint32(i*6271 + 3079)
		seed = seed*1664525 + 1013904223
		baseX := float64(seed % uint32(canvas.w))
		seed = seed*1664525 + 1013904223

		t := float64(tick) * 0.15
		bandIdx := i % vizBandCount
		px := int(baseX + math.Sin(t+float64(i)*0.7)*float64(canvas.w)*0.02)
		py := int(centerY + math.Sin(t*1.3+float64(i)*1.1)*float64(canvas.h)*0.4*bands[bandIdx] +
			math.Cos(t*0.8+float64(i)*0.5)*bassEnergy*float64(canvas.h)*0.3)
		canvas.set(px, py)
	}

	// Dashed center line
	for x := 0; x < canvas.w; x += 4 {
		canvas.set(x, int(centerY))
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dist := math.Abs(float64(cy)-float64(ch)/2) / (float64(ch) / 2)
		idx := int(dist * float64(vizBandCount-1))
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Style 4: Radial burst ───────────────────────────────────────────────────

func renderVizRadial(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	// Use the full canvas — aspect-correct the radii
	maxRadiusX := float64(canvas.w) * 0.48
	maxRadiusY := float64(canvas.h) * 0.48

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	bassEnergy := avgBands(bands[:], 0, 6)
	rotation := float64(tick) * (0.02 + totalEnergy*0.06)

	// More rays for denser fill
	raysPerBand := 10
	numRays := vizBandCount * raysPerBand

	for i := 0; i < numRays; i++ {
		angle := float64(i)/float64(numRays)*2*math.Pi + rotation
		bandIdx := (i / raysPerBand) % vizBandCount
		energy := bands[bandIdx]

		rx := maxRadiusX * energy
		ry := maxRadiusY * energy

		innerR := 0.03 * (1 + bassEnergy*2)
		x0 := centerX + maxRadiusX*innerR*math.Cos(angle)
		y0 := centerY + maxRadiusY*innerR*math.Sin(angle)
		x1 := centerX + rx*math.Cos(angle)
		y1 := centerY + ry*math.Sin(angle)
		canvas.line(int(x0), int(y0), int(x1), int(y1))
	}

	// Outer ring: smooth, follows total energy
	ringSteps := 300
	for i := 0; i < ringSteps; i++ {
		angle := float64(i) / float64(ringSteps) * 2 * math.Pi
		x := centerX + maxRadiusX*totalEnergy*math.Cos(angle)
		y := centerY + maxRadiusY*totalEnergy*math.Sin(angle)
		canvas.set(int(x), int(y))
	}

	// Inner ring: jumpy — snaps to discrete levels for punchy feel
	// Quantize bass energy to steps for that "level meter" jump
	jumpLevels := 8.0
	jumpBass := math.Floor(bassEnergy*jumpLevels) / jumpLevels
	for i := 0; i < ringSteps; i++ {
		angle := float64(i) / float64(ringSteps) * 2 * math.Pi
		r := 0.2 + jumpBass*0.35
		x := centerX + maxRadiusX*r*math.Cos(angle)
		y := centerY + maxRadiusY*r*math.Sin(angle)
		canvas.set(int(x), int(y))
	}

	// Mid ring: sin-wave modulated radius for organic movement
	midEnergy := avgBands(bands[:], 8, 16)
	t := float64(tick) * 0.12
	for i := 0; i < ringSteps; i++ {
		angle := float64(i) / float64(ringSteps) * 2 * math.Pi
		wave := math.Sin(angle*6+t) * midEnergy * 0.15
		r := totalEnergy*0.6 + wave
		x := centerX + maxRadiusX*r*math.Cos(angle)
		y := centerY + maxRadiusY*r*math.Sin(angle)
		canvas.set(int(x), int(y))
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		dist := math.Sqrt(dx*dx+dy*dy) / (float64(max(cw, ch)) / 2)
		idx := int(dist * float64(vizBandCount-1))
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Style 5: Spiral ─────────────────────────────────────────────────────────

func renderVizSpiral(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	maxRX := float64(canvas.w) * 0.45
	maxRY := float64(canvas.h) * 0.45

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	bassEnergy := avgBands(bands[:], 0, 8)
	// Slower rotation — less chaos
	rotation := float64(tick) * (0.02 + totalEnergy*0.08)

	// Fixed arm count for cleaner look, but tightness varies with bass
	arms := 3
	steps := 600
	// Fewer turns = less visual noise; energy extends the spiral outward
	maxTheta := 3*math.Pi + totalEnergy*math.Pi

	for arm := 0; arm < arms; arm++ {
		armOffset := float64(arm) / float64(arms) * 2 * math.Pi
		prevX, prevY := -1, -1

		for i := 0; i <= steps; i++ {
			baseFrac := float64(i) / float64(steps)
			theta := baseFrac*maxTheta + rotation + armOffset

			// Smooth radius growth — energy creates gentle pulse, not bumps
			bandIdx := int(baseFrac * float64(vizBandCount-1))
			if bandIdx >= vizBandCount {
				bandIdx = vizBandCount - 1
			}
			energy := bands[bandIdx]

			// Smooth pulse: sine modulation along the arm instead of raw band jumps
			pulse := math.Sin(baseFrac*math.Pi*4+float64(tick)*0.1) * energy * 0.15
			growthFrac := baseFrac * (0.8 + totalEnergy*0.2)
			rx := growthFrac*maxRX + pulse*maxRX
			ry := growthFrac*maxRY + pulse*maxRY

			x := centerX + rx*math.Cos(theta)
			y := centerY + ry*math.Sin(theta)
			px, py := int(x), int(y)

			if prevX >= 0 {
				canvas.line(prevX, prevY, px, py)
			}
			prevX, prevY = px, py
		}
	}

	// Central dot cluster that breathes with bass
	dotR := bassEnergy * maxRX * 0.12
	dotSteps := 80
	for i := 0; i < dotSteps; i++ {
		angle := float64(i) / float64(dotSteps) * 2 * math.Pi
		canvas.set(int(centerX+dotR*math.Cos(angle)), int(centerY+dotR*math.Sin(angle)))
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		dist := math.Sqrt(dx*dx+dy*dy) / (float64(max(cw, ch)) / 2)
		idx := int(dist * float64(vizBandCount-1))
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(vizBandCount-1-idx, vizBandCount))
	})
}

// ── Style 6: Starfield ──────────────────────────────────────────────────────

func renderVizStarfield(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	// Drifting vanishing point — slowly wanders around, feels like flying through space
	t := float64(tick) * 0.03
	driftX := math.Sin(t*0.7) * float64(canvas.w) * 0.15
	driftY := math.Cos(t*0.5) * float64(canvas.h) * 0.1
	centerX := float64(canvas.w)/2 + driftX
	centerY := float64(canvas.h)/2 + driftY

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	bassEnergy := avgBands(bands[:], 0, 8)

	numStars := 250 + int(totalEnergy*400)
	speed := 0.3 + totalEnergy*4.0
	spiralRate := 0.3 + totalEnergy*0.5

	for i := 0; i < numStars; i++ {
		seed := uint32(i*7919 + 1013)
		baseAngle := float64(seed%36000) / 36000.0 * 2 * math.Pi
		baseSpeed := 0.2 + float64(seed%1000)/1000.0

		progress := math.Mod(float64(tick)*speed*baseSpeed*0.015+float64(seed%1000)/1000.0, 1.0)
		progress = progress * progress
		maxDistX := float64(canvas.w) * 0.55
		maxDistY := float64(canvas.h) * 0.55

		angle := baseAngle + progress*spiralRate

		bandIdx := i % vizBandCount
		energy := bands[bandIdx]

		rx := progress * maxDistX
		ry := progress * maxDistY
		x := centerX + rx*math.Cos(angle)
		y := centerY + ry*math.Sin(angle)

		trailLen := (2.0 + energy*10.0 + bassEnergy*5.0) * progress
		trailRx := rx - trailLen*math.Abs(math.Cos(angle))
		trailRy := ry - trailLen*math.Abs(math.Sin(angle))
		if trailRx < 0 {
			trailRx = 0
		}
		if trailRy < 0 {
			trailRy = 0
		}
		tx := centerX + trailRx*math.Cos(angle)
		ty := centerY + trailRy*math.Sin(angle)

		canvas.line(int(tx), int(ty), int(x), int(y))
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		dist := math.Sqrt(dx*dx+dy*dy) / (float64(max(cw, ch)) / 2)
		idx := int((1 - dist) * float64(vizBandCount-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Style 7: Doom Flame ──────────────────────────────────────────────────────

func renderVizFlame(bands [vizBandCount]float64, width, height, tick int) string {
	// Doom fire: bottom row = heat from bands, propagates upward with decay
	cols := width - 2
	if cols < 4 {
		cols = 4
	}
	rows := height

	// Build heat buffer [row][col] where row 0 = top
	heat := make([][]float64, rows)
	for i := range heat {
		heat[i] = make([]float64, cols)
	}

	// Seed bottom row from band energy
	for x := 0; x < cols; x++ {
		bandIdx := int(float64(x) / float64(cols) * float64(vizBandCount))
		if bandIdx >= vizBandCount {
			bandIdx = vizBandCount - 1
		}
		heat[rows-1][x] = bands[bandIdx]
	}

	// Propagate upward: each cell pulls from below with decay + horizontal drift
	for y := rows - 2; y >= 0; y-- {
		for x := 0; x < cols; x++ {
			seed := uint32((tick*7 + y*131 + x*997) & 0xFFFFFF)
			seed = seed*1664525 + 1013904223 // LCG
			drift := int(seed%3) - 1         // -1, 0, or 1
			srcX := x + drift
			if srcX < 0 {
				srcX = 0
			}
			if srcX >= cols {
				srcX = cols - 1
			}
			decay := float64(seed%120) / 1000.0
			val := heat[y+1][srcX] - decay
			if val < 0 {
				val = 0
			}
			heat[y][x] = val
		}
	}

	// Render: map heat to block chars + fire colors
	fireColors := []lipgloss.Color{
		lipgloss.Color("#1a0000"),
		lipgloss.Color("#8B0000"),
		lipgloss.Color("#FF2400"),
		lipgloss.Color("#FF6600"),
		lipgloss.Color("#FFaa00"),
		lipgloss.Color("#FFDD00"),
		lipgloss.Color("#FFFF66"),
		lipgloss.Color("#FFFFFF"),
	}

	pad := (width - cols) / 2
	prefix := strings.Repeat(" ", pad)

	var lines []string
	for y := 0; y < rows; y++ {
		var line strings.Builder
		line.WriteString(prefix)
		for x := 0; x < cols; x++ {
			h := heat[y][x]
			fillIdx := int(h * 8)
			if fillIdx > 8 {
				fillIdx = 8
			}
			if fillIdx < 0 {
				fillIdx = 0
			}
			colorIdx := int(h * float64(len(fireColors)-1))
			if colorIdx >= len(fireColors) {
				colorIdx = len(fireColors) - 1
			}
			if colorIdx < 0 {
				colorIdx = 0
			}
			ch := blockChars[fillIdx]
			line.WriteString(lipgloss.NewStyle().Foreground(fireColors[colorIdx]).Render(ch))
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// ── Style 8: Plasma ─────────────────────────────────────────────────────────

func renderVizPlasma(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)

	t := float64(tick) * 0.08

	// Moving blob centers — drift around based on energy
	blob1X := 0.5 + math.Sin(t*0.6)*0.3
	blob1Y := 0.5 + math.Cos(t*0.8)*0.3
	blob2X := 0.5 + math.Sin(t*0.4+2)*0.35
	blob2Y := 0.5 + math.Cos(t*0.5+1)*0.35

	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			fx := float64(px) / float64(canvas.w)
			fy := float64(py) / float64(canvas.h)

			v := 0.0
			// Horizontal + vertical waves
			v += math.Sin(fx*6.0*(1+lowEnergy*3) + t)
			v += math.Sin(fy*8.0*(1+midEnergy*3) + t*1.3)
			// Diagonal
			v += math.Sin((fx+fy)*5.0*(1+highEnergy*2) + t*0.7)
			// Radial ripple from center
			cx := fx - 0.5
			cy := fy - 0.5
			dist := math.Sqrt(cx*cx + cy*cy)
			v += math.Sin(dist*12.0*(1+totalEnergy*2) - t*2)

			// Blob fields — create larger/smaller globs that shift with energy
			d1 := math.Sqrt((fx-blob1X)*(fx-blob1X) + (fy-blob1Y)*(fy-blob1Y))
			d2 := math.Sqrt((fx-blob2X)*(fx-blob2X) + (fy-blob2Y)*(fy-blob2Y))
			blobSize1 := 0.15 + lowEnergy*0.3  // bass makes blob1 bigger
			blobSize2 := 0.1 + highEnergy*0.25 // highs make blob2 bigger
			v += (blobSize1 - d1) * 3
			v += (blobSize2 - d2) * 3

			v = (v + 6) / 12.0
			threshold := 0.55 - totalEnergy*0.2
			if v > threshold {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fx := float64(cx) / float64(cw)
		fy := float64(cy) / float64(ch)
		v := math.Sin(fx*3+t)*0.5 + math.Cos(fy*4+t*1.2)*0.5
		idx := int((v + 1) / 2 * float64(vizBandCount-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Style 9: Ring (circular oscilloscope) ────────────────────────────────────

func renderVizRing(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	// Aspect-correct radii to fill canvas
	baseRX := float64(canvas.w) * 0.35
	baseRY := float64(canvas.h) * 0.35

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	rotation := float64(tick) * (0.01 + totalEnergy*0.03)

	// Multiple passes for thickness
	steps := 400
	passes := 3
	for p := 0; p < passes; p++ {
		rOff := float64(p-1) * 0.02 // -0.02, 0, +0.02
		prevX, prevY := -1, -1
		for i := 0; i <= steps; i++ {
			angle := float64(i)/float64(steps)*2*math.Pi + rotation
			frac := float64(i) / float64(steps)

			bandIdx := int(frac * float64(vizBandCount))
			if bandIdx >= vizBandCount {
				bandIdx = vizBandCount - 1
			}
			energy := bands[bandIdx]

			waveOff := 0.0
			for b := 0; b < vizBandCount; b++ {
				freq := 1.0 + float64(b)*0.5
				phase := float64(tick) * (0.1 + float64(b)*0.02)
				waveOff += bands[b] * math.Sin(frac*freq*math.Pi*2+phase) * 0.3
			}
			waveOff /= float64(vizBandCount)

			scale := 1.0 + energy*0.5 + waveOff + rOff
			rx := baseRX * scale
			ry := baseRY * scale

			x := centerX + rx*math.Cos(angle)
			y := centerY + ry*math.Sin(angle)
			px, py := int(x), int(y)

			if prevX >= 0 {
				canvas.line(prevX, prevY, px, py)
			}
			prevX, prevY = px, py
		}
	}

	// Inner circle — pulses with bass
	bassEnergy := avgBands(bands[:], 0, 6)
	innerRX := baseRX * (0.3 + bassEnergy*0.2)
	innerRY := baseRY * (0.3 + bassEnergy*0.2)
	innerSteps := 200
	prevX, prevY := -1, -1
	for i := 0; i <= innerSteps; i++ {
		angle := float64(i) / float64(innerSteps) * 2 * math.Pi
		x := centerX + innerRX*math.Cos(angle)
		y := centerY + innerRY*math.Sin(angle)
		px, py := int(x), int(y)
		if prevX >= 0 {
			canvas.line(prevX, prevY, px, py)
		}
		prevX, prevY = px, py
	}

	// Radiating spokes on bass hits — short lines bursting outward
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	numSpokes := 16
	for i := 0; i < numSpokes; i++ {
		angle := float64(i)/float64(numSpokes)*2*math.Pi + rotation*0.5
		bandIdx := i % vizBandCount
		energy := bands[bandIdx]
		if energy < 0.2 {
			continue
		}
		// Spoke starts just outside the waveform, extends outward
		innerScale := 1.0 + energy*0.5
		outerScale := innerScale + energy*0.3 + highEnergy*0.2
		x0 := centerX + baseRX*innerScale*math.Cos(angle)
		y0 := centerY + baseRY*innerScale*math.Sin(angle)
		x1 := centerX + baseRX*outerScale*math.Cos(angle)
		y1 := centerY + baseRY*outerScale*math.Sin(angle)
		canvas.line(int(x0), int(y0), int(x1), int(y1))
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		dist := math.Sqrt(dx*dx+dy*dy) / (float64(max(cw, ch)) / 2)
		idx := int(dist * float64(vizBandCount-1))
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Style 10: Donut (rotating torus) ─────────────────────────────────────────

func renderVizDonut(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	bassEnergy := avgBands(bands[:], 0, 8)

	// Torus parameters — sized to fill the canvas
	R := float64(min(canvas.w, canvas.h)) * 0.38 * (0.85 + totalEnergy*0.2) // major radius
	r := R * (0.4 + bassEnergy*0.15)                                         // minor radius (tube)

	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2

	// Rotation angles — driven by energy
	t := float64(tick)
	A := t * (0.03 + totalEnergy*0.05) // rotation around X
	B := t * (0.02 + totalEnergy*0.04) // rotation around Z

	cosA, sinA := math.Cos(A), math.Sin(A)
	cosB, sinB := math.Cos(B), math.Sin(B)

	// Z-buffer for hidden surface removal
	zBuf := make([][]float64, canvas.h)
	for i := range zBuf {
		zBuf[i] = make([]float64, canvas.w)
		for j := range zBuf[i] {
			zBuf[i][j] = -1e9
		}
	}

	// Sample the torus surface
	thetaSteps := 120 + int(totalEnergy*60)
	phiSteps := 60 + int(totalEnergy*30)

	for ti := 0; ti < thetaSteps; ti++ {
		theta := float64(ti) / float64(thetaSteps) * 2 * math.Pi
		cosT, sinT := math.Cos(theta), math.Sin(theta)

		// Per-theta band modulation — the tube wobbles with the music
		bandIdx := int(float64(ti) / float64(thetaSteps) * float64(vizBandCount))
		if bandIdx >= vizBandCount {
			bandIdx = vizBandCount - 1
		}
		localR := r * (1 + bands[bandIdx]*0.4)

		for pi := 0; pi < phiSteps; pi++ {
			phi := float64(pi) / float64(phiSteps) * 2 * math.Pi
			cosP, sinP := math.Cos(phi), math.Sin(phi)

			// 3D torus point
			cx := (R + localR*cosT) * cosP
			cy := (R + localR*cosT) * sinP
			cz := localR * sinT

			// Rotate around X axis
			y1 := cy*cosA - cz*sinA
			z1 := cy*sinA + cz*cosA

			// Rotate around Z axis
			x2 := cx*cosB - y1*sinB
			y2 := cx*sinB + y1*cosB

			// Project to 2D (perspective)
			depth := z1 + R*3
			if depth < 0.1 {
				continue
			}
			scale := R * 3.5 / depth
			sx := int(centerX + x2*scale)
			sy := int(centerY + y2*scale*0.5) // squish Y for terminal aspect ratio

			if sx >= 0 && sx < canvas.w && sy >= 0 && sy < canvas.h {
				if z1 > zBuf[sy][sx] {
					zBuf[sy][sx] = z1
					canvas.set(sx, sy)
				}
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		dist := math.Sqrt(dx*dx+dy*dy) / (float64(max(cw, ch)) / 2)
		idx := int(dist * float64(vizBandCount-1))
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Style 11: Moiré ─────────────────────────────────────────────────────────

func renderVizMoire(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	highEnergy := avgBands(bands[:], 16, vizBandCount)

	t := float64(tick) * 0.06

	// Two circle centers that orbit each other — spacing driven by energy
	spacing := float64(canvas.w) * (0.05 + lowEnergy*0.15)
	c1x := float64(canvas.w)/2 + math.Cos(t)*spacing
	c1y := float64(canvas.h)/2 + math.Sin(t)*spacing*0.6
	c2x := float64(canvas.w)/2 - math.Cos(t*1.3)*spacing
	c2y := float64(canvas.h)/2 - math.Sin(t*0.9)*spacing*0.6

	// Ring spacing driven by energy — tighter rings = higher frequency pattern
	ringSpacing := 4.0 + (1-totalEnergy)*6.0 // 4-10 pixels between rings

	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			// Distance from each center
			d1 := math.Sqrt(float64((px-int(c1x))*(px-int(c1x)) + (py-int(c1y))*(py-int(c1y))))
			d2 := math.Sqrt(float64((px-int(c2x))*(px-int(c2x)) + (py-int(c2y))*(py-int(c2y))))

			// Concentric rings: sin creates alternating on/off bands
			r1 := math.Sin(d1 / ringSpacing * math.Pi)
			r2 := math.Sin(d2 / ringSpacing * math.Pi)

			// Third pattern: diagonal waves modulated by high frequency
			r3 := math.Sin((float64(px)+float64(py))*0.15+t*2) * highEnergy

			// Moiré = interference between the two ring patterns
			interference := r1*r2 + r3*0.3

			threshold := 0.1 - totalEnergy*0.2
			if interference > threshold {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		// Color shifts with angle from center
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		angle := math.Atan2(dy, dx)
		idx := int((angle/(2*math.Pi) + 0.5) * float64(vizBandCount-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func avgBands(bands []float64, from, to int) float64 {
	if to <= from {
		return 0
	}
	sum := 0.0
	for i := from; i < to && i < len(bands); i++ {
		sum += bands[i]
	}
	return sum / float64(to-from)
}
