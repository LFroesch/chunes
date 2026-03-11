package player

import (
	"encoding/binary"
	"io"
	"math"
	"math/cmplx"
	"os/exec"
	"sync"
)

const (
	sampleRate = 44100
	fftSize    = 2048 // ~46ms window, ~21.5 Hz per bin
)

// Band frequency ranges (Hz) — logarithmically spaced across audible spectrum
var bandRanges = [BandCount][2]float64{
	{20, 60},       // sub-bass
	{60, 170},      // bass
	{170, 500},     // low-mid
	{500, 1400},    // mid
	{1400, 4000},   // upper-mid
	{4000, 8000},   // presence
	{8000, 16000},  // brilliance
	{16000, 20000}, // air
}

// Spectrum captures audio from PulseAudio monitor and computes per-band
// frequency levels via FFT.
type Spectrum struct {
	cmd    *exec.Cmd
	mu     sync.Mutex
	bands  [BandCount]float64
	window [fftSize]float64 // pre-computed Hanning window
}

// NewSpectrum starts parec to capture system audio and begins FFT analysis.
// Returns nil (no error) if parec is unavailable — caller should use RMS fallback.
func NewSpectrum() *Spectrum {
	cmd := exec.Command("parec",
		"--device=@DEFAULT_MONITOR@",
		"--format=float32le",
		"--channels=1",
		"--rate=44100",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}

	s := &Spectrum{cmd: cmd}

	// Pre-compute Hanning window
	for i := range fftSize {
		s.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}

	go s.readLoop(stdout)
	return s
}

// readLoop continuously reads PCM from parec, computes FFT, updates band levels.
func (s *Spectrum) readLoop(r io.Reader) {
	buf := make([]byte, fftSize*4) // float32 = 4 bytes
	samples := make([]float64, fftSize)
	freqBuf := make([]complex128, fftSize)

	for {
		// Read exactly one FFT window worth of samples
		if _, err := io.ReadFull(r, buf); err != nil {
			return
		}

		// Convert float32le bytes to float64 samples
		for i := range fftSize {
			bits := binary.LittleEndian.Uint32(buf[i*4 : i*4+4])
			samples[i] = float64(math.Float32frombits(bits))
		}

		// Apply Hanning window and load into complex buffer
		for i := range fftSize {
			freqBuf[i] = complex(samples[i]*s.window[i], 0)
		}

		// In-place FFT
		fftInPlace(freqBuf)

		// Extract per-band energy
		var bands [BandCount]float64
		binWidth := float64(sampleRate) / float64(fftSize)

		for b := range BandCount {
			loFreq, hiFreq := bandRanges[b][0], bandRanges[b][1]
			loBin := int(math.Ceil(loFreq / binWidth))
			hiBin := int(math.Floor(hiFreq / binWidth))
			if loBin < 1 {
				loBin = 1
			}
			if hiBin >= fftSize/2 {
				hiBin = fftSize/2 - 1
			}

			// Find peak magnitude in this band (normalized by fftSize/2)
			norm := float64(fftSize) / 2
			var peak float64
			for k := loBin; k <= hiBin; k++ {
				mag := cmplx.Abs(freqBuf[k]) / norm
				if mag > peak {
					peak = mag
				}
			}
			bands[b] = peak
		}

		// Normalize: convert to 0-1 range using dB-like scaling
		for b := range BandCount {
			if bands[b] < 1e-10 {
				bands[b] = 0
				continue
			}
			// Pre-gain boost so visualization pops at any volume
			bands[b] *= 2.5
			// Convert to dB, map -60..0 dB to 0..1
			db := 20 * math.Log10(bands[b])
			if db < -60 {
				bands[b] = 0
			} else if db >= 0 {
				bands[b] = 1
			} else {
				linear := (db + 60) / 60
				bands[b] = linear * linear // perceptual scaling
			}
			// Extra amp for visual impact
			bands[b] *= 1.25
			if bands[b] > 1 {
				bands[b] = 1
			}
		}

		s.mu.Lock()
		s.bands = bands
		s.mu.Unlock()
	}
}

// GetBandLevels returns the latest per-band frequency levels (0.0-1.0).
func (s *Spectrum) GetBandLevels() [BandCount]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bands
}

// Close stops the parec subprocess.
func (s *Spectrum) Close() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
}

// fftInPlace performs an in-place radix-2 Cooley-Tukey FFT.
func fftInPlace(x []complex128) {
	n := len(x)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}

	// Cooley-Tukey butterfly
	for size := 2; size <= n; size *= 2 {
		half := size / 2
		wBase := -2 * math.Pi / float64(size)
		for i := 0; i < n; i += size {
			for k := 0; k < half; k++ {
				w := cmplx.Rect(1, wBase*float64(k))
				t := w * x[i+k+half]
				x[i+k+half] = x[i+k] - t
				x[i+k] = x[i+k] + t
			}
		}
	}
}
