package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func nowPlayingHints() []helpBinding {
	hints := []helpBinding{
		{"m", "info/viz"},
		{"v/V", "viz"},
		{"c", "cycle"},
	}
	return hints
}

// viewNowPlaying renders the Now Playing tab in one of two body modes:
// a full-height visualizer or a track-info view with metadata/context.
func (m Model) viewNowPlaying(width, height int) string {
	var lines []string

	track := m.nowPlaying
	if track == nil {
		empty := lipgloss.NewStyle().Foreground(dimColor).Render("No track playing — search and play something!")
		lines = append(lines, "")
		lines = append(lines, centerLine(width, empty))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:height], "\n")
	}

	if m.npShowMeta {
		infoBlock := m.renderNowPlayingInfo(width, max(height, 2), m.npMetaScroll)
		lines = append(lines, strings.Split(infoBlock, "\n")...)
	} else {
		vizContent := renderFullViz(m.vizBands, m.vizStyle, width, max(height, 2), m.vizTick)
		vizLines := strings.Split(vizContent, "\n")
		for i := 0; i < height; i++ {
			if i < len(vizLines) {
				lines = append(lines, vizLines[i])
			} else {
				lines = append(lines, "")
			}
		}
	}

	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderNowPlayingInfo(width, height, scroll int) string {
	if height < 4 {
		return ""
	}

	allLines := m.nowPlayingInfoLines(width, height)
	return renderScrollableTextBlock(allLines, width, height, scroll)
}

func (m Model) nowPlayingInfoLines(width, height int) []string {
	desc := m.nowPlayingDescription()
	infoLines := nowPlayingDescriptionLines(width, desc)
	infoLines = append([]string{
		headerStyle.Render("Track Context"),
		dimStyle("Visualizer: " + m.currentVizStatus()),
		"",
	}, infoLines...)
	if height < 8 {
		var out []string
		for _, line := range infoLines {
			for _, wrapped := range wrapText(line, max(width-4, 8), max(len(strings.Fields(line)), 1)) {
				out = append(out, "  "+wrapped)
			}
		}
		return out
	}

	artWidth := min(30, max(width/3, 18))
	artLines := artworkLines(m.metaArt, artWidth)

	var out []string
	if width < 86 {
		out = append(out, renderNowPlayingInfoStacked(width, artLines, infoLines)...)
		return out
	}

	textWidth := width - artWidth - 8
	if textWidth < 24 {
		out = append(out, renderNowPlayingInfoStacked(width, artLines, infoLines)...)
		return out
	}

	rows := max(len(artLines), len(infoLines))
	for i := 0; i < rows; i++ {
		left := ""
		if i < len(artLines) {
			left = artLines[i]
		}
		right := ""
		if i < len(infoLines) {
			right = infoLines[i]
		}
		line := "  " + padStyled(left, artWidth) + "    " + lipgloss.NewStyle().MaxWidth(textWidth).Render(right)
		out = append(out, line)
	}
	return out
}

func (m Model) nowPlayingDescription() string {
	desc := strings.TrimSpace(m.metaDesc)
	if desc != "" {
		return desc
	}
	if m.metaLoading {
		return "Loading track description..."
	}
	if m.metaErr != "" {
		return "Track metadata unavailable."
	}
	return "No description available."
}

func nowPlayingDescriptionLines(width int, desc string) []string {
	textWidth := max(width-40, 28)
	infoLines := []string{}
	for _, line := range wrapText(desc, textWidth, max(len(strings.Fields(desc)), 1)) {
		infoLines = append(infoLines, line)
	}
	return infoLines
}

func (m Model) currentVizStatus() string {
	switch {
	case m.spectrum != nil:
		return fmt.Sprintf("%s · pulse-monitor", vizStyleNames[m.vizStyle])
	default:
		return fmt.Sprintf("%s · mpv-rms fallback", vizStyleNames[m.vizStyle])
	}
}

func renderNowPlayingInfoStacked(width int, artLines, infoLines []string) []string {
	var out []string
	maxArtLines := len(artLines)
	for i := 0; i < maxArtLines; i++ {
		out = append(out, lipgloss.PlaceHorizontal(width, lipgloss.Center, artLines[i]))
	}
	out = append(out, "")
	for _, line := range infoLines {
		out = append(out, "  "+lipgloss.NewStyle().MaxWidth(max(width-4, 1)).Render(line))
	}
	return out
}

func artworkLines(art string, width int) []string {
	if strings.TrimSpace(art) == "" {
		return []string{dimStyle("art unavailable")}
	}
	if strings.Contains(art, "\n") {
		return strings.Split(art, "\n")
	}
	return wrapText(art, width, 12)
}

func padStyled(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	return s + strings.Repeat(" ", width-w)
}

func renderScrollableTextBlock(lines []string, width, height, scroll int) string {
	if height <= 0 {
		return ""
	}

	if len(lines) == 0 {
		lines = []string{""}
	}

	total := len(lines)
	effective := height
	if scroll > 0 {
		effective--
	}
	if scroll+effective < total {
		effective--
	}
	if effective < 1 {
		effective = 1
	}

	maxScroll := total - effective
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	effective = height
	if scroll > 0 {
		effective--
	}
	if scroll+effective < total {
		effective--
	}
	if effective < 1 {
		effective = 1
	}

	end := min(scroll+effective, total)
	var out []string
	if scroll > 0 {
		out = append(out, centerLine(width, dimStyle("↑ more")))
	}
	for i := scroll; i < end; i++ {
		out = append(out, lines[i])
	}
	if end < total {
		out = append(out, centerLine(width, dimStyle("↓ more")))
	}
	for len(out) < height {
		out = append(out, "")
	}
	return strings.Join(out[:height], "\n")
}

// ── Braille canvas ──────────────────────────────────────────────────────────

// brailleCanvas is a 2D pixel grid that renders to braille characters.
// Each cell is 2 wide × 4 tall in braille sub-pixels.
type brailleCanvas struct {
	w, h   int      // pixel dimensions
	pixels [][]bool // [y][x]
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
	if style < 0 || style >= len(vizStyles) {
		style = 0
	}
	return vizStyles[style].render(bands, width, height, tick)
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

	// Tuning knobs:
	// - `a` and `b` control the lobe count.
	// - `passes` and `phaseOff` control line thickness.
	// Keep the ratios smooth; hard quantization makes the figure jump around.
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
		idx := min(max(int(dist*float64(vizBandCount-1)), 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
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

	// Tuning knobs:
	// - `numStars` controls density.
	// - `speed` controls outward travel rate.
	// - `spiralRate` controls how much the field twists instead of reading as a straight tunnel.
	// Drifting vanishing point — slowly wanders around, like sitting at the helm.
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
		progress = 1 - (1-progress)*(1-progress)
		maxDistX := float64(canvas.w) * 0.55
		maxDistY := float64(canvas.h) * 0.55

		angle := baseAngle - progress*spiralRate

		bandIdx := i % vizBandCount
		energy := bands[bandIdx]

		rx := progress * maxDistX
		ry := progress * maxDistY
		x := centerX + rx*math.Cos(angle)
		y := centerY + ry*math.Sin(angle)

		// Shorten the radial distance itself so the trail stays aligned with the
		// star's motion instead of skewing by axis.
		trailLen := 2.0 + energy*10.0 + bassEnergy*5.0
		trailDist := math.Max(progress*float64(min(canvas.w, canvas.h))*0.55-trailLen, 0)
		tx := centerX + trailDist*math.Cos(angle)
		ty := centerY + trailDist*math.Sin(angle)

		canvas.line(int(tx), int(ty), int(x), int(y))
	}

	// Helm / canopy hints near the bottom help the perspective read as forward travel.
	hudY := int(float64(canvas.h) * 0.82)
	canvas.line(0, canvas.h-1, int(centerX*0.72), hudY)
	canvas.line(canvas.w-1, canvas.h-1, int(centerX+(float64(canvas.w)-centerX)*0.72), hudY)
	canvas.line(int(centerX-float64(canvas.w)*0.08), canvas.h-1, int(centerX), hudY+2)
	canvas.line(int(centerX+float64(canvas.w)*0.08), canvas.h-1, int(centerX), hudY+2)

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
	cols := width
	if cols < 4 {
		cols = 4
	}
	rows := height

	// Build heat buffer [row][col] where row 0 = top
	heat := make([][]float64, rows)
	for i := range heat {
		heat[i] = make([]float64, cols)
	}

	// Tuning knobs:
	// - `baseHeat` controls how full the window stays.
	// - `drift` and `decay` shape how chaotic vs columnar the fire feels.
	// Seed the bottom row heavily so the flame fills the full window instead of
	// leaving wide dead zones.
	for x := 0; x < cols; x++ {
		bandIdx := int(float64(x) / float64(cols) * float64(vizBandCount))
		if bandIdx >= vizBandCount {
			bandIdx = vizBandCount - 1
		}
		baseHeat := 0.35 + bands[bandIdx]*0.9
		heat[rows-1][x] = math.Min(baseHeat, 1.0)
	}

	// Propagate upward: each cell pulls from below with decay + horizontal drift
	for y := rows - 2; y >= 0; y-- {
		for x := 0; x < cols; x++ {
			seed := uint32((tick*7 + y*131 + x*997) & 0xFFFFFF)
			seed = seed*1664525 + 1013904223 // LCG
			drift := int(seed%5) - 2         // -2..2
			srcX := x + drift
			if srcX < 0 {
				srcX = 0
			}
			if srcX >= cols {
				srcX = cols - 1
			}
			decay := float64(seed%90) / 1000.0
			sideMix := heat[y+1][min(srcX+1, cols-1)] + heat[y+1][max(srcX-1, 0)]
			val := heat[y+1][srcX]*0.72 + sideMix*0.14 - decay
			if val < 0 {
				val = 0
			}
			heat[y][x] = math.Min(val, 1.0)
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

	var lines []string
	for y := 0; y < rows; y++ {
		var line strings.Builder
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
	detail := vizFieldDetail(cw, ch)
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
	// - blob positions and sizes control the "liquid" feeling.
	// - `threshold` is the main density gate; lower = fuller image.
	blob1X := 0.5 + math.Sin(t*0.6)*0.30
	blob1Y := 0.5 + math.Cos(t*0.8)*0.28
	blob2X := 0.5 + math.Sin(t*0.4+2.0)*0.36
	blob2Y := 0.5 + math.Cos(t*0.5+1.0)*0.36
	blob3X := 0.5 + math.Sin(t*0.9+4.0)*0.24
	blob3Y := 0.5 + math.Cos(t*0.7+3.0)*0.32
	blob4X := 0.5 + math.Sin(t*1.2+5.3)*0.18
	blob4Y := 0.5 + math.Cos(t*0.95+2.4)*0.20

	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			fx := float64(px) / float64(canvas.w)
			fy := float64(py) / float64(canvas.h)
			cfx, cfy, dist, _ := vizFieldPoint(px, py, canvas.w, canvas.h)

			v := 0.0
			v += math.Sin(cfx*3.0*detail*(1+lowEnergy*2.4) + t)
			v += math.Sin(cfy*4.2*detail*(1+midEnergy*2.6) + t*1.3)
			v += math.Sin((cfx+cfy)*2.8*detail*(1+highEnergy*2.1) + t*0.7)
			v += math.Cos((cfx-cfy)*3.7*detail*(1+midEnergy*1.5) - t*0.9)
			v += math.Sin(dist*10.0*detail*(1+totalEnergy*2.2) - t*2.2)

			d1 := math.Sqrt((fx-blob1X)*(fx-blob1X) + (fy-blob1Y)*(fy-blob1Y))
			d2 := math.Sqrt((fx-blob2X)*(fx-blob2X) + (fy-blob2Y)*(fy-blob2Y))
			d3 := math.Sqrt((fx-blob3X)*(fx-blob3X) + (fy-blob3Y)*(fy-blob3Y))
			d4 := math.Sqrt((fx-blob4X)*(fx-blob4X) + (fy-blob4Y)*(fy-blob4Y))
			blobSize1 := 0.15 + lowEnergy*0.32
			blobSize2 := 0.11 + highEnergy*0.24
			blobSize3 := 0.09 + midEnergy*0.2
			blobSize4 := 0.07 + totalEnergy*0.16
			v += (blobSize1 - d1) * 3
			v += (blobSize2 - d2) * 3
			v += (blobSize3 - d3) * 2.8
			v += (blobSize4 - d4) * 2.4
			v += math.Sin((fx*fy)*detail*16+t*1.6) * (0.12 + totalEnergy*0.18)

			v = (v + 8) / 16.0
			threshold := 0.46 - density - totalEnergy*0.16 + dist*0.05
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

func renderVizNebula(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	t := float64(tick) * 0.05
	detail := vizFieldDetail(cw, ch)
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
	// - `swirl` and `ripple` shape the nebula body.
	// - `starNoise` threshold controls how speckled the outskirts get.
	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			fx, fy, r, a := vizFieldPoint(px, py, canvas.w, canvas.h)

			swirl := math.Sin(a*6 + t*1.1 + r*detail*4.6*(1+highEnergy*1.8))
			ripple := math.Cos(r*detail*11 - t*2.4 + lowEnergy*6)
			drift := math.Sin((fx*1.6-fy*1.1)*math.Pi*detail+t*0.7) + math.Cos((fx+fy)*math.Pi*detail-t*0.4)
			core := math.Exp(-r*(2.2-midEnergy*0.8)) * (1.2 + lowEnergy)
			dust := math.Sin((fx*fy)*detail*9+t*0.9) * (0.18 + highEnergy*0.25)
			arms := math.Sin(a*(3.0+midEnergy*2.5)-t*0.8+r*detail*7.0) * math.Exp(-r*(1.6-highEnergy*0.3))

			v := swirl*0.65 + ripple*0.5 + drift*0.35 + core*1.45 + dust + arms*0.4
			threshold := 0.58 - density - totalEnergy*0.22 - core*0.17
			if v > threshold {
				canvas.set(px, py)
			}
			starNoise := math.Sin(float64(px)*12.9898+float64(py)*78.233+float64(tick)*0.17) *
				math.Cos(float64(px)*4.123+float64(py)*15.731-float64(tick)*0.11)
			if r > 0.2 && starNoise > 0.985-totalEnergy*0.05+highEnergy*0.02 {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fx := (float64(cx)/float64(cw) - 0.5) * 2
		fy := (float64(cy)/float64(ch) - 0.5) * 2
		r := math.Sqrt(fx*fx + fy*fy)
		a := math.Atan2(fy, fx)
		v := math.Sin(a*4+t+r*6)*0.5 + math.Cos(r*10-t*1.8)*0.5
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

func renderVizAurora(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	t := float64(tick) * 0.06
	detail := vizFieldDetail(cw, ch)
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
		// - `centerA` / `centerB` control where the curtains hang.
		// - `thicknessA` / `thicknessB` control band width.
		// - `sparkle` and `sideScroll` add breakup without changing the main arc shape.
		for x := 0; x < canvas.w; x++ {
			xf := float64(x) / float64(max(canvas.w-1, 1))
			bandIdx := min(int(xf*float64(vizBandCount)), vizBandCount-1)
			band := bands[bandIdx]
			centerA := 0.27 + math.Sin(xf*math.Pi*(1.4+lowEnergy*1.5)+t*0.9)*0.18
			centerA += math.Cos(xf*math.Pi*(2.6+midEnergy*1.6)*detail-t*0.6) * 0.08
			centerB := 0.58 + math.Sin(xf*math.Pi*(1.0+midEnergy*1.2)-t*0.6)*0.14
			centerB += math.Cos(xf*math.Pi*(3.8+highEnergy*1.2)+t*0.5) * 0.05
			thicknessA := 0.12 + band*0.19 + totalEnergy*0.1 + density*0.28
			thicknessB := 0.09 + bands[(bandIdx+5)%vizBandCount]*0.14 + midEnergy*0.08 + density*0.16
			sparkle := math.Sin(xf*math.Pi*10+t*1.8) * highEnergy * 0.04

			for y := 0; y < canvas.h; y++ {
				yf := float64(y) / float64(max(canvas.h-1, 1))
				distA := math.Abs(yf - (centerA + sparkle))
				distB := math.Abs(yf - (centerB - sparkle*0.6))
				glowA := math.Exp(-distA * (10 + band*24))
				glowB := math.Exp(-distB * (12 + highEnergy*20))
				ripple := math.Sin((yf*detail*4.8-xf*detail*2.8)*math.Pi+t*1.3) * (0.18 + midEnergy*0.16)
				sideScroll := math.Sin((xf*detail*8+yf*detail*2.6)*math.Pi-t*1.7) * (0.12 + highEnergy*0.12)
				v := glowA + glowB*0.85 + ripple + sideScroll
				if (distA < thicknessA || distB < thicknessB) && v > 0.35-density*0.45-band*0.05 {
					canvas.set(x, y)
				}
				if math.Sin(xf*26+yf*15+t) > 0.992-highEnergy*0.025 {
					canvas.set(x, y)
				}
			}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fy := float64(cy) / float64(max(ch-1, 1))
		idx := int((1 - fy*0.85) * float64(vizBandCount-1))
		idx = min(max(idx, 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizSupernova(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	t := float64(tick) * 0.07
	detail := vizFieldDetail(cw, ch)
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
	// - `core` controls the bright center falloff.
	// - `shock` and `petals` control the burst silhouette.
	// - `threshold` is the main balance between readable shell and visual noise.
	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			fx, fy, r, a := vizFieldPoint(px, py, canvas.w, canvas.h)

			core := math.Exp(-r * (4.2 - lowEnergy*1.3))
			shock := math.Cos(r*detail*8.5-t*3.2+lowEnergy*5) * math.Exp(-r*(1.45-highEnergy*0.35))
			petals := math.Sin(a*(6+midEnergy*8)+t*1.4) * math.Exp(-r*(2.4-midEnergy*0.5))
			sparks := math.Sin((fx*detail*9-fy*detail*7)*math.Pi+t*2.1) * math.Cos((fx+fy)*detail*6-t*1.7) * (0.2 + highEnergy*0.35)

			v := core*1.9 + shock*0.92 + petals*0.72 + sparks
			threshold := 0.55 - density - totalEnergy*0.22 - core*0.18
			if v > threshold {
				canvas.set(px, py)
			}
			if r > 0.08 && r < 1.3 && shock > 0.74-totalEnergy*0.09 {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fx := (float64(cx)/float64(cw) - 0.5) * 2
		fy := (float64(cy)/float64(ch) - 0.5) * 2
		r := math.Sqrt(fx*fx + fy*fy)
		a := math.Atan2(fy, fx)
		glow := math.Sin(a*3+t+r*9)*0.45 + math.Cos(r*13-t*2.4)*0.55
		idx := int((glow + 1) / 2 * float64(vizBandCount-1))
		idx = min(max(idx, 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(vizBandCount-1-idx, vizBandCount))
	})
}

func renderVizVortex(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	t := float64(tick) * 0.065
	detail := vizFieldDetail(cw, ch)
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
	// - `spin` sets the vortex winding rate.
	// - `arms` and `tunnel` control whether this reads as spiral arms vs a funnel.
	// - raise `threshold` for a cleaner, emptier core.
	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			fx, fy, r, a := vizFieldPoint(px, py, canvas.w, canvas.h)

			spin := a + r*detail*(5.6+lowEnergy*4.4) - t*(2.2+highEnergy*1.4)
			arms := math.Sin(spin*(2.2+midEnergy*1.6) + math.Sin(t+r*6)*0.8)
			shear := math.Cos((fx-fy)*math.Pi*(1.8+highEnergy*1.1)*detail + t*1.2)
			core := math.Exp(-r * (4.5 - lowEnergy*1.4))
			tunnel := math.Cos(r*detail*14-t*3.1+midEnergy*7) * math.Exp(-r*(1.9-highEnergy*0.4))

			v := arms*0.85 + shear*0.25 + tunnel*0.55 + core*0.45
			threshold := 0.56 - density - totalEnergy*0.12 + r*0.12
			if v > threshold {
				canvas.set(px, py)
			}
			if r < 0.22 && core > 0.32 {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fx := (float64(cx)/float64(cw) - 0.5) * 2
		fy := (float64(cy)/float64(ch) - 0.5) * 2
		r := math.Sqrt(fx*fx + fy*fy)
		a := math.Atan2(fy, fx) + math.Pi
		idx := int((a/(2*math.Pi)*0.6 + (1-r)*0.7) * float64(vizBandCount-1))
		idx = min(max(idx, 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizEclipse(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	t := float64(tick) * 0.055
	shadowShift := 0.08 + math.Sin(t*0.8)*0.06 + midEnergy*0.05
	detail := vizFieldDetail(cw, ch)
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
	// - `shadowShift` controls how offset the moon is from the sun.
	// - `corona` controls the ring sharpness.
	// - `occult` controls how aggressively the moon carves out the center.
	for py := 0; py < canvas.h; py++ {
		for px := 0; px < canvas.w; px++ {
			fx, fy, rSun, a := vizFieldPoint(px, py, canvas.w, canvas.h)
			rMoon := math.Sqrt((fx-shadowShift)*(fx-shadowShift) + fy*fy)

			corona := math.Exp(-math.Abs(rSun-(0.42+lowEnergy*0.08)) * (11 + highEnergy*7 + density*18))
			rays := math.Sin(a*(7+highEnergy*8)+t*1.6+rSun*detail*4.6) * math.Exp(-rSun*(1.7-midEnergy*0.25))
			halo := math.Cos(rSun*detail*12-t*2.6+lowEnergy*4) * math.Exp(-rSun*(1.9-highEnergy*0.3))
			occult := math.Exp(-rMoon * (5.8 + lowEnergy*2.0))

			v := corona*1.3 + rays*0.65 + halo*0.35 - occult*1.25
			threshold := 0.34 - density*0.35 - totalEnergy*0.12
			if v > threshold && rSun < 1.22 {
				canvas.set(px, py)
			}
			if rSun > 0.22 && rSun < 0.95 && corona > 0.48 && occult < 0.82 {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fx := (float64(cx)/float64(cw) - 0.5) * 2
		fy := (float64(cy)/float64(ch) - 0.5) * 2
		r := math.Sqrt(fx*fx + fy*fy)
		a := math.Atan2(fy, fx)
		glow := math.Sin(a*2.5+t)*0.35 + math.Cos(r*12-t*2.1)*0.65
		idx := int((glow + 1) / 2 * float64(vizBandCount-1))
		idx = min(max(idx, 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizPulseGrid(bands [vizBandCount]float64, width, height, tick int) string {
	cellsX := max(min(width/2, 36), 12)
	cellsY := max(min(height, 18), 8)
	cellW := max(width/cellsX, 1)
	cellH := max(height/cellsY, 1)
	t := float64(tick) * 0.12

	// Tuning knobs:
	// - `cellsX` / `cellsY` control how fine the screen-filling grid feels.
	// - `wave` and `ripple` decide whether this reads as blocks vs a pressure field.
	var lines []string
	for gy := 0; gy < cellsY; gy++ {
		for sy := 0; sy < cellH; sy++ {
			var line strings.Builder
			for gx := 0; gx < cellsX; gx++ {
				nx := float64(gx) / float64(max(cellsX-1, 1))
				ny := float64(gy) / float64(max(cellsY-1, 1))
				band := bands[(gx+gy*2)%vizBandCount]
				wave := math.Sin(nx*math.Pi*6+t) + math.Cos(ny*math.Pi*5-t*0.8)
				ripple := math.Sin((nx+ny)*math.Pi*8 + t*1.7)
					cross := math.Sin((nx-ny)*math.Pi*9 - t*1.1)
					v := wave*0.4 + ripple*0.3 + cross*0.25 + band*1.45
					idx := int(math.Round((v + 1.9) / 3.8 * 8))
				if idx < 0 {
					idx = 0
				}
				if idx > 8 {
					idx = 8
				}
				ch := blockChars[idx]
				style := lipgloss.NewStyle().Foreground(vizGradientFor((gx+gy)%vizBandCount, vizBandCount))
				for sx := 0; sx < cellW; sx++ {
					line.WriteString(style.Render(ch))
				}
			}
			lines = append(lines, line.String())
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
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

	// Tuning knobs:
	// - `passes` / `rOff` control shell thickness.
	// - `waveOff` controls how organic vs geometric the ring reads.
	// Clamp the radius scale so negative interference does not fold the ring inward.
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

			scale := math.Max(0.2, 1.0+energy*0.5+waveOff+rOff)
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

	// Tuning knobs:
	// - `R` controls the overall torus size.
	// - `r` controls tube thickness.
	// - `thetaSteps` / `phiSteps` control detail vs CPU cost.
	// Torus parameters — sized to fill the canvas
	R := float64(min(canvas.w, canvas.h)) * 0.38 * (0.85 + totalEnergy*0.2) // major radius
	r := R * (0.4 + bassEnergy*0.15)                                        // minor radius (tube)

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

	// Tuning knobs:
	// - `spacing` controls how far apart the interference sources drift.
	// - `ringSpacing` controls ring density; smaller values make the pattern busier.
	// Keep these as float math so the pattern moves smoothly instead of snapping.
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
			// Distance from each center. Use float deltas so the rings glide
			// continuously as the centers orbit.
			dx1 := float64(px) - c1x
			dy1 := float64(py) - c1y
			dx2 := float64(px) - c2x
			dy2 := float64(py) - c2y
			d1 := math.Sqrt(dx1*dx1 + dy1*dy1)
			d2 := math.Sqrt(dx2*dx2 + dy2*dy2)

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

// ── Style 12: Mirror bars ────────────────────────────────────────────────────

// renderVizMirrorBars renders spectrum bars that grow from the vertical center outward.
func renderVizMirrorBars(bands [vizBandCount]float64, width, height int) string {
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

	halfH := height / 2
	totalLevels := halfH * 8

	// Pre-compute bar height (in sub-rows) for each band
	heights := make([]int, maxBands)
	for b := 0; b < maxBands; b++ {
		val := bands[b]
		if val > 0 {
			val = math.Log1p(val*4) / math.Log1p(4)
		}
		if val > 0.95 {
			val = 0.95
		}
		heights[b] = int(val * float64(totalLevels))
	}

	renderBar := func(b, rowFromCenter int) string {
		// rowFromCenter=0 is at the center, increases outward
		rowBase := rowFromCenter * 8
		fill := heights[b] - rowBase
		if fill < 0 {
			fill = 0
		}
		if fill > 8 {
			fill = 8
		}
		return blockChars[fill]
	}

	var lines []string

	// Top half: row 0 = topmost, center = halfH-1
	for row := 0; row < halfH; row++ {
		rowFromCenter := halfH - 1 - row
		var line strings.Builder
		line.WriteString(prefix)
		for b := 0; b < maxBands; b++ {
			ch := renderBar(b, rowFromCenter)
			color := vizGradientFor(b, maxBands)
			styled := lipgloss.NewStyle().Foreground(color).Render(ch)
			for w := 0; w < barWidth; w++ {
				line.WriteString(styled)
			}
			line.WriteString(strings.Repeat(" ", gap))
		}
		lines = append(lines, line.String())
	}

	// Bottom half: center = 0, grows downward
	for row := 0; row < height-halfH; row++ {
		var line strings.Builder
		line.WriteString(prefix)
		for b := 0; b < maxBands; b++ {
			ch := renderBar(b, row)
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

func renderVizCascade(bands [vizBandCount]float64, width, height, tick int) string {
	lines := make([]string, 0, height)
	total := height * 8
	slope := max(width/10, 6)
	for row := height - 1; row >= 0; row-- {
		var line strings.Builder
		pad := max((width-(vizBandCount*2))/2, 0)
		line.WriteString(strings.Repeat(" ", pad))
		for b := 0; b < vizBandCount; b++ {
			level := int(math.Min(math.Log1p(bands[b]*4)/math.Log1p(4), 1.0) * float64(total))
			rowBase := row * 8
			cellFill := level - rowBase
			if cellFill < 0 {
				cellFill = 0
			}
			if cellFill > 8 {
				cellFill = 8
			}
			if row == (tick+b)%max(height, 1) {
				cellFill = min(cellFill+1, 8)
			}
			ch := blockChars[cellFill]
			color := vizGradientFor((b+row/slope)%vizBandCount, vizBandCount)
			line.WriteString(lipgloss.NewStyle().Foreground(color).Render(ch))
			line.WriteString(" ")
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

func renderVizHorizon(bands [vizBandCount]float64, width, height, tick int) string {
	mid := max(height/2, 1)
	total := mid * 8
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		var line strings.Builder
		for col := 0; col < width; col++ {
			line.WriteByte(' ')
		}
		lines[row] = line.String()
	}

	for b := 0; b < vizBandCount; b++ {
		level := math.Min(math.Pow(bands[b], 0.8)*1.15, 1.0)
		stack := int(level * float64(total))
		col := (b*width)/vizBandCount + width/(vizBandCount*2)
		col = min(max(col, 0), max(width-1, 0))
		// Tuning knobs:
		// - `waveAmp` controls how much the horizon bends.
		// - `waveTilt` controls how quickly the bend increases away from center.
		// Keep this in float math; integer division flattens the effect.
		waveAmp := math.Sin(float64(tick)*0.12+float64(b)*0.7) * 2.0
		waveTilt := 3.0
		for dir := -1; dir <= 1; dir += 2 {
			remaining := stack
			for offset := 0; offset < mid && remaining > 0; offset++ {
				waveOffset := int(math.Round(waveAmp * float64(offset) / float64(max(mid, 1)) * waveTilt))
				row := mid + dir*offset + waveOffset
				if row < 0 || row >= height {
					continue
				}
				fill := min(remaining, 8)
				remaining -= fill
				r := []rune(lines[row])
				r[col] = []rune(blockChars[fill])[0]
				lines[row] = string(r)
			}
		}
	}

	for row := range lines {
		lines[row] = styleHorizonLine(lines[row], width)
	}
	return strings.Join(lines, "\n")
}

func renderVizHelix(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)
	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	// Tuning knobs:
	// - `rx` / `ry` control how much of the window the helix occupies.
	// - `steps` controls smoothness vs clutter.
	// - ladder rungs below are intentionally sparse; raising their cadence makes it noisy fast.
	rx := float64(canvas.w) * 0.44
	ry := float64(canvas.h) * 0.4
	steps := 560
	prev1x, prev1y := -1, -1
	prev2x, prev2y := -1, -1
	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		band := bands[min(int(t*float64(vizBandCount-1)), vizBandCount-1)]
		angle := t*(4.0+lowEnergy*2.2)*math.Pi + float64(tick)*(0.03+highEnergy*0.05)
		coil := math.Sin(t*math.Pi*2 + float64(tick)*0.05)
		pulse := 0.94 + band*0.38 + coil*0.05
		sep := 0.16 + band*0.16 + totalEnergy*0.08
		x1 := int(centerX + rx*math.Cos(angle)*pulse)
		y1 := int(centerY + ry*math.Sin(angle*0.55-sep))
		x2 := int(centerX + rx*math.Cos(angle+math.Pi)*pulse)
		y2 := int(centerY + ry*math.Sin(angle*0.55+sep))
		if prev1x >= 0 {
			canvas.line(prev1x, prev1y, x1, y1)
			canvas.line(prev2x, prev2y, x2, y2)
		}
		rungEvery := max(26-int(totalEnergy*6), 16)
		if i%rungEvery == 0 {
			canvas.line(x1, y1, x2, y2)
		}
		prev1x, prev1y = x1, y1
		prev2x, prev2y = x2, y2
	}
	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := math.Abs(float64(cx)-float64(cw)/2) / float64(max(cw/2, 1))
		dy := math.Abs(float64(cy)-float64(ch)/2) / float64(max(ch/2, 1))
		idx := int((dx*0.65 + dy*0.35) * float64(vizBandCount-1))
		if idx >= vizBandCount {
			idx = vizBandCount - 1
		}
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizConstellation(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)
	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	rx := float64(canvas.w) * 0.4
	ry := float64(canvas.h) * 0.4
	points := make([][2]int, 0, vizBandCount)
	for i := 0; i < vizBandCount; i++ {
		angle := float64(i)/float64(vizBandCount)*2*math.Pi + float64(tick)*0.015
		ripple := 0.5 + bands[i]*0.5
		x := int(centerX + rx*ripple*math.Cos(angle))
		y := int(centerY + ry*ripple*math.Sin(angle))
		points = append(points, [2]int{x, y})
		canvas.set(x, y)
	}
	for i := range points {
		next := (i + 1) % len(points)
		if bands[i] > 0.1 || bands[next] > 0.1 {
			canvas.line(points[i][0], points[i][1], points[next][0], points[next][1])
		}
		opp := (i + len(points)/2) % len(points)
		if bands[i] > 0.45 {
			canvas.line(points[i][0], points[i][1], points[opp][0], points[opp][1])
		}
	}
	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
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

func renderVizPeaks(bands [vizBandCount]float64, width, height, tick int) string {
	barWidth := 2
	gap := 1
	slot := barWidth + gap
	count := min(vizBandCount, max((width-4)/slot, 6))
	total := height * 8
	pad := max((width-(count*slot))/2, 0)
	lines := make([]string, 0, height)
	for row := height - 1; row >= 0; row-- {
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", pad))
		for b := 0; b < count; b++ {
			level := int(math.Min(math.Sqrt(bands[b])*1.1, 1.0) * float64(total))
			rowBase := row * 8
			fill := level - rowBase
			if fill < 0 {
				fill = 0
			}
			if fill > 8 {
				fill = 8
			}
			ch := blockChars[fill]
			peakRow := int((1 - bands[b]) * float64(max(height-1, 1)))
			color := vizGradientFor(b, count)
			style := lipgloss.NewStyle().Foreground(color)
			if row == peakRow || row == peakRow+((tick/4)%2) {
				style = style.Bold(true)
				ch = "•"
			}
			cell := style.Render(ch)
			for i := 0; i < barWidth; i++ {
				line.WriteString(cell)
			}
			line.WriteString(strings.Repeat(" ", gap))
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

func renderVizShards(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)
	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	for i := 0; i < vizBandCount; i++ {
		angle := float64(i)/float64(vizBandCount)*2*math.Pi + float64(tick)*0.045
		energy := 0.25 + bands[i]*0.9
		inner := 0.1 + bands[(i+3)%vizBandCount]*0.2
		outerX := centerX + math.Cos(angle)*float64(canvas.w)*0.45*energy
		outerY := centerY + math.Sin(angle)*float64(canvas.h)*0.42*energy
		innerX := centerX + math.Cos(angle+0.35)*float64(canvas.w)*inner
		innerY := centerY + math.Sin(angle+0.35)*float64(canvas.h)*inner
		sideX := centerX + math.Cos(angle-0.18)*float64(canvas.w)*0.18*(1+bands[(i+7)%vizBandCount])
		sideY := centerY + math.Sin(angle-0.18)*float64(canvas.h)*0.16*(1+bands[(i+11)%vizBandCount])
		// Always close the triangle; open shards read like broken geometry rather than faceted glass.
		canvas.line(int(innerX), int(innerY), int(outerX), int(outerY))
		canvas.line(int(sideX), int(sideY), int(outerX), int(outerY))
		canvas.line(int(innerX), int(innerY), int(sideX), int(sideY))
		if bands[i] > 0.55 {
			facetX := centerX + math.Cos(angle+0.08)*float64(canvas.w)*0.28*(0.6+bands[i]*0.6)
			facetY := centerY + math.Sin(angle+0.08)*float64(canvas.h)*0.24*(0.6+bands[i]*0.6)
			canvas.line(int(innerX), int(innerY), int(facetX), int(facetY))
			canvas.line(int(sideX), int(sideY), int(facetX), int(facetY))
			canvas.line(int(outerX), int(outerY), int(facetX), int(facetY))
		}
	}
	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		// Tuning knob: normalize against the center-to-corner distance so the
		// gradient can actually span the full palette.
		maxDist := math.Sqrt(float64(cw*cw+ch*ch)) / 2
		dist := math.Sqrt(dx*dx+dy*dy) / max(maxDist, 1)
		idx := min(int(dist*float64(vizBandCount)), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizTunnel(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)
	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	// Tuning knobs:
	// - `rings` controls tunnel depth.
	// - the connector lines below decide how architectural vs organic it feels.
	// Anchor the connectors to the actual ring phase so they don't drift off-axis.
	rings := 9
	phase := float64(tick) * 0.08
	for ring := 0; ring < rings; ring++ {
		frac := float64(ring+1) / float64(rings+1)
		band := bands[(ring*3)%vizBandCount]
		rx := float64(canvas.w) * frac * (0.12 + band*0.28)
		ry := float64(canvas.h) * frac * (0.1 + band*0.22)
		steps := 200
		var px, py int
		for i := 0; i <= steps; i++ {
			a := float64(i)/float64(steps)*2*math.Pi + phase + frac*math.Pi
			wobble := 1 + math.Sin(a*3+phase*2+float64(ring))*band*0.35
			x := int(centerX + math.Cos(a)*rx*wobble)
			y := int(centerY + math.Sin(a)*ry*wobble)
			if i > 0 {
				canvas.line(px, py, x, y)
			}
			px, py = x, y
		}
		if ring < rings-1 {
			nextFrac := float64(ring+2) / float64(rings+1)
			connectorAngle := phase + frac*math.Pi
			nextConnectorAngle := phase + nextFrac*math.Pi
			x1 := int(centerX + math.Cos(connectorAngle)*rx*0.7)
			y1 := int(centerY + math.Sin(connectorAngle)*ry*0.7)
			x2 := int(centerX + math.Cos(nextConnectorAngle)*float64(canvas.w)*nextFrac*0.12)
			y2 := int(centerY + math.Sin(nextConnectorAngle)*float64(canvas.h)*nextFrac*0.12)
			canvas.line(x1, y1, x2, y2)
			canvas.line(int(2*centerX)-x1, y1, int(2*centerX)-x2, y2)
		}
	}
	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		angle := math.Atan2(dy, dx) + math.Pi
		idx := min(int(angle/(2*math.Pi)*float64(vizBandCount)), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizRibbons(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)
	t := float64(tick) * 0.09
	strands := 5
	for s := 0; s < strands; s++ {
		prevX, prevY := -1, -1
		base := float64(s+1) / float64(strands+1)
		for x := 0; x < canvas.w; x++ {
			frac := float64(x) / float64(max(canvas.w-1, 1))
			band := bands[min(int(frac*float64(vizBandCount-1)), vizBandCount-1)]
			amp := (0.08 + band*0.22) * float64(canvas.h)
			y := int(float64(canvas.h)*base +
				math.Sin(frac*math.Pi*4+t+float64(s)*0.7)*amp +
				math.Cos(frac*math.Pi*7-t*0.6+float64(s))*amp*0.35)
			if prevX >= 0 {
				canvas.line(prevX, prevY, x, y)
				if band > 0.45 {
					canvas.line(prevX, prevY+1, x, y+1)
				}
			}
			prevX, prevY = x, y
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		idx := min(cy*vizBandCount/max(ch, 1), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizWarp(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)
	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	t := float64(tick) * 0.11
	totalEnergy := avgBands(bands[:], 0, vizBandCount)

	// Tuning knobs:
	// - `length` is the main perspective exaggeration.
	// - `wobble` adds organic motion; reduce it if the rays feel too soft.
	for i := 0; i < vizBandCount; i++ {
		angle := float64(i)/float64(vizBandCount)*2*math.Pi + t*0.5
		energy := 0.2 + bands[i]*0.9
		length := (0.35 + energy*0.55) * float64(min(canvas.w, canvas.h))
		wobble := 1 + math.Sin(t*1.4+float64(i))*0.12
		x0 := int(centerX + math.Cos(angle)*length*0.12)
		y0 := int(centerY + math.Sin(angle)*length*0.08)
		x1 := int(centerX + math.Cos(angle)*length*wobble)
		y1 := int(centerY + math.Sin(angle)*length*0.55*wobble)
		canvas.line(x0, y0, x1, y1)
		if bands[i] > 0.55 {
			angle2 := angle + math.Pi/48
			x2 := int(centerX + math.Cos(angle2)*length*(1+totalEnergy*0.2))
			y2 := int(centerY + math.Sin(angle2)*length*0.58)
			canvas.line(x0, y0, x2, y2)
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		// Keep the color pass centered on the same origin as the ray geometry.
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		angle := math.Atan2(dy, dx) + math.Pi
		idx := min(int(angle/(2*math.Pi)*float64(vizBandCount)), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizCube(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	centerX := float64(canvas.w) / 2
	centerY := float64(canvas.h) / 2
	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	highEnergy := avgBands(bands[:], 16, vizBandCount)

	// Tuning knobs:
	// - `size` controls the cube footprint.
	// - `depthPulse` controls how much the inner/outer box separation breathes.
	// - edge list below is the simplest place to restyle the wireframe.
	size := float64(min(canvas.w, canvas.h)) * (0.28 + totalEnergy*0.12)
	depthPulse := 0.7 + lowEnergy*0.45
	ax := float64(tick) * (0.028 + highEnergy*0.04)
	ay := float64(tick) * (0.021 + totalEnergy*0.03)
	az := float64(tick) * 0.017

	type vec3 struct{ x, y, z float64 }
	verts := []vec3{
		{-1, -1, -1}, {1, -1, -1}, {1, 1, -1}, {-1, 1, -1},
		{-1, -1, 1}, {1, -1, 1}, {1, 1, 1}, {-1, 1, 1},
	}
	edges := [][2]int{
		{0, 1}, {1, 2}, {2, 3}, {3, 0},
		{4, 5}, {5, 6}, {6, 7}, {7, 4},
		{0, 4}, {1, 5}, {2, 6}, {3, 7},
	}

	project := func(v vec3, scale float64) (int, int) {
		x, y, z := v.x*scale, v.y*scale, v.z*scale*depthPulse

		cx, sx := math.Cos(ax), math.Sin(ax)
		cy, sy := math.Cos(ay), math.Sin(ay)
		cz, sz := math.Cos(az), math.Sin(az)

		y, z = y*cx-z*sx, y*sx+z*cx
		x, z = x*cy+z*sy, -x*sy+z*cy
		x, y = x*cz-y*sz, x*sz+y*cz

		depth := z + scale*4.5
		perspective := scale * 2.4 / math.Max(depth, 0.1)
		sx2 := int(centerX + x*perspective)
		sy2 := int(centerY + y*perspective*0.58)
		return sx2, sy2
	}

	scales := []float64{
		size,
		size * (0.68 + totalEnergy*0.08),
		size * (0.42 + highEnergy*0.18),
	}
	for s, scale := range scales {
		for _, edge := range edges {
			x1, y1 := project(verts[edge[0]], scale)
			x2, y2 := project(verts[edge[1]], scale)
			canvas.line(x1, y1, x2, y2)
		}
		if s < len(scales)-1 {
			for i := range verts {
				x1, y1 := project(verts[i], scale)
				x2, y2 := project(verts[i], scales[s+1])
				canvas.line(x1, y1, x2, y2)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		dx := float64(cx) - float64(cw)/2
		dy := float64(cy) - float64(ch)/2
		angle := math.Atan2(dy, dx) + math.Pi
		idx := min(int(angle/(2*math.Pi)*float64(vizBandCount)), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func renderVizGridWarp(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	t := float64(tick) * 0.06
	horizonY := float64(canvas.h) * (0.12 + math.Sin(t*0.7)*0.03)
	vanishX := float64(canvas.w)/2 + math.Sin(t*0.5)*float64(canvas.w)*0.08

	// Tuning knobs:
	// - `rows` controls perceived depth.
	// - `cols` controls grid density.
	// - `horizonY` / `vanishX` are the main perspective controls.
	rows := 12 + int(totalEnergy*8)
	cols := 13 + int(midEnergy*10)

	for r := 0; r < rows; r++ {
		depth0 := float64(r) / float64(max(rows, 1))
		depth1 := float64(r+1) / float64(max(rows, 1))
		y0 := horizonY + math.Pow(depth0, 1.65)*(float64(canvas.h)-horizonY)
		y1 := horizonY + math.Pow(depth1, 1.65)*(float64(canvas.h)-horizonY)
		w0 := (0.09 + depth0*(1.75+highEnergy*0.35)) * float64(canvas.w)
		w1 := (0.09 + depth1*(1.75+highEnergy*0.35)) * float64(canvas.w)

		canvas.line(int(vanishX-w0/2), int(y0), int(vanishX+w0/2), int(y0))
		if r == rows-1 {
			canvas.line(int(vanishX-w1/2), int(y1), int(vanishX+w1/2), int(y1))
		}
	}

	for c := 0; c <= cols; c++ {
		frac := float64(c)/float64(max(cols, 1)) - 0.5
		wobble := math.Sin(t*1.4+float64(c)*0.55) * totalEnergy * 0.06
		topX := vanishX + frac*float64(canvas.w)*0.08
		bottomX := vanishX + (frac+wobble)*float64(canvas.w)*(1.7+midEnergy*0.45)
		canvas.line(int(topX), int(horizonY), int(bottomX), canvas.h-1)
	}

	for s := 0; s < 4; s++ {
		band := bands[(s*5)%vizBandCount]
		baseY := 0.28 + float64(s)*0.14
		prevX, prevY := -1, -1
		for x := 0; x < canvas.w; x++ {
			xf := float64(x) / float64(max(canvas.w-1, 1))
			y := int(float64(canvas.h) * (baseY +
				math.Sin(xf*math.Pi*(2.2+float64(s)*0.8)+t*1.2+float64(s))*0.03*(1+band*3.0) +
				math.Cos(xf*math.Pi*(5.0+band*2.0)-t*0.8)*0.012*(1+midEnergy*2.0)))
			if prevX >= 0 {
				canvas.line(prevX, prevY, x, y)
			}
			prevX, prevY = x, y
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fy := float64(cy) / float64(max(ch-1, 1))
		idx := min(max(int(fy*float64(vizBandCount-1)), 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(vizBandCount-1-idx, vizBandCount))
	})
}

func renderVizMetaballs(bands [vizBandCount]float64, width, height, tick int) string {
	cw := width - 2
	ch := height
	canvas := newBrailleCanvas(cw, ch)

	totalEnergy := avgBands(bands[:], 0, vizBandCount)
	lowEnergy := avgBands(bands[:], 0, 8)
	midEnergy := avgBands(bands[:], 8, 16)
	highEnergy := avgBands(bands[:], 16, vizBandCount)
	t := float64(tick) * 0.07
	density := vizFieldDensity(cw, ch)

	// Tuning knobs:
	// - positions below control orbit paths.
	// - `strength` controls blob pull.
	// - `threshold` controls merge vs separation.
	phaseSplit := (math.Sin(t*0.55) + 1) / 2
	centers := [][4]float64{
		{0.5 - (0.08 + phaseSplit*0.24) + math.Sin(t*0.9)*0.08, 0.5 + math.Cos(t*0.7)*0.16, 0.07 + lowEnergy*0.14, 1.45 + lowEnergy*1.4},
		{0.5 + (0.08 + phaseSplit*0.24) + math.Sin(t*0.6+2.1)*0.08, 0.5 + math.Cos(t*0.8+1.1)*0.18, 0.07 + midEnergy*0.14, 1.35 + midEnergy*1.2},
		{0.5 + math.Sin(t*1.1+4.0)*0.15, 0.5 - (0.06+phaseSplit*0.16) + math.Cos(t*0.9+3.0)*0.08, 0.06 + highEnergy*0.1, 1.2 + highEnergy*1.0},
		{0.5 + math.Sin(t*0.75+5.2)*0.12, 0.5 + (0.06+phaseSplit*0.16) + math.Cos(t*1.0+0.8)*0.08, 0.06 + totalEnergy*0.1, 1.1 + totalEnergy*0.9},
	}

	for py := 0; py < canvas.h; py++ {
		fy := float64(py) / float64(max(canvas.h-1, 1))
		for px := 0; px < canvas.w; px++ {
			fx := float64(px) / float64(max(canvas.w-1, 1))
			field := 0.0
			for _, c := range centers {
				dx := fx - c[0]
				dy := fy - c[1]
				dist2 := dx*dx + dy*dy + c[2]*c[2]
				field += c[3] / dist2
			}
			ripple := math.Sin((fx*8-fy*5+t)*math.Pi) * totalEnergy * 0.12
				threshold := 20.0 - totalEnergy*3.8 - density*8 - phaseSplit*2.2
			if field+ripple > threshold {
				canvas.set(px, py)
			}
		}
	}

	return " " + canvas.render(cw, ch, func(cx, cy int) lipgloss.Style {
		fx := float64(cx) / float64(max(cw-1, 1))
		fy := float64(cy) / float64(max(ch-1, 1))
		v := math.Sin((fx+fy+t*0.1)*math.Pi*2)*0.4 + math.Cos((fx-fy-t*0.08)*math.Pi*3)*0.6
		idx := min(max(int((v+1)/2*float64(vizBandCount-1)), 0), vizBandCount-1)
		return lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount))
	})
}

func styleHorizonLine(raw string, width int) string {
	var out strings.Builder
	runes := []rune(raw)
	if len(runes) > width {
		runes = runes[:width]
	}
	for i, ch := range runes {
		if ch == ' ' {
			out.WriteRune(ch)
			continue
		}
		idx := min(i*vizBandCount/max(width, 1), vizBandCount-1)
		out.WriteString(lipgloss.NewStyle().Foreground(vizGradientFor(idx, vizBandCount)).Render(string(ch)))
	}
	return out.String()
}

func vizFieldPoint(px, py, width, height int) (fx, fy, r, a float64) {
	fx = (float64(px)/float64(max(width, 1)) - 0.5) * 2
	fy = (float64(py)/float64(max(height, 1)) - 0.5) * 2
	aspect := float64(max(width, 1)) / float64(max(height, 1))
	if aspect > 1 {
		fx *= aspect
	} else {
		fy /= aspect
	}
	r = math.Sqrt(fx*fx + fy*fy)
	a = math.Atan2(fy, fx)
	return fx, fy, r, a
}

func vizFieldDetail(width, height int) float64 {
	minDim := float64(max(min(width, height), 1))
	detail := minDim / 18.0
	if detail < 1.0 {
		detail = 1.0
	}
	if detail > 2.8 {
		detail = 2.8
	}
	return detail
}

func vizFieldDensity(width, height int) float64 {
	area := float64(max(width*height, 1))
	density := math.Log(area/500.0+1.0) * 0.06
	if density < 0 {
		density = 0
	}
	if density > 0.12 {
		density = 0.12
	}
	return density
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
