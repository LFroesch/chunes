package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var mpvCounter atomic.Int64

const BandCount = 8

type MPV struct {
	cmd        *exec.Cmd
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader
	mu         sync.Mutex
	reqID      int
	Playing    bool
	Paused     bool

	// Cached values — updated via PollProperties(), read without IPC
	CachedPosition float64
	CachedDuration float64
	CachedIdle     bool
}

type mpvCommand struct {
	Command   []interface{} `json:"command"`
	RequestID int           `json:"request_id,omitempty"`
}

type mpvResponse struct {
	Data      interface{} `json:"data"`
	RequestID int         `json:"request_id"`
	Error     string      `json:"error"`
}

func NewMPV() (*MPV, error) {
	n := mpvCounter.Add(1)
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("chunes-mpv-%d-%d.sock", os.Getpid(), n))
	// Clean up stale socket
	os.Remove(socketPath)

	cmd := exec.Command("mpv",
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--af=@stats:lavfi=[astats=metadata=1:reset=1:length=0.05]",
		"--input-ipc-server="+socketPath,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	m := &MPV{
		cmd:        cmd,
		socketPath: socketPath,
	}

	// Wait for socket to appear
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			m.conn = conn
			m.reader = bufio.NewReaderSize(conn, 64*1024)
			m.CachedIdle = true
			return m, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	cmd.Process.Kill()
	return nil, fmt.Errorf("mpv socket did not appear at %s", socketPath)
}

func (m *MPV) sendCommand(args ...interface{}) (*mpvResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		return nil, fmt.Errorf("not connected to mpv")
	}

	m.reqID++
	cmd := mpvCommand{Command: args, RequestID: m.reqID}
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	m.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := m.conn.Write(data); err != nil {
		return nil, err
	}

	// Read complete JSON lines until we find our response.
	// bufio.Reader survives deadline errors — next read with a fresh deadline works fine.
	for {
		m.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := m.reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("mpv read error: %w", err)
		}
		if len(line) == 0 {
			continue
		}
		var resp mpvResponse
		if json.Unmarshal(line, &resp) == nil && resp.RequestID == m.reqID {
			if resp.Error != "" && resp.Error != "success" {
				return nil, fmt.Errorf("mpv error: %s", resp.Error)
			}
			return &resp, nil
		}
		// else: event line, discard and keep reading
	}
}

func (m *MPV) Play(url string) error {
	_, err := m.sendCommand("loadfile", url, "replace")
	if err != nil {
		return err
	}
	// Session resume uses PlayPaused (pause=yes). mpv keeps pause across loadfile, so a
	// different track would load but stay silent unless we clear pause explicitly.
	_, err = m.sendCommand("set_property", "pause", false)
	if err != nil {
		return err
	}
	m.Playing = true
	m.Paused = false
	return nil
}

// PlayPaused loads a file but starts it paused (no audio blast).
func (m *MPV) PlayPaused(url string) error {
	// Set pause before loading so mpv never starts outputting audio
	m.sendCommand("set_property", "pause", true)
	_, err := m.sendCommand("loadfile", url, "replace")
	if err == nil {
		m.Playing = true
		m.Paused = true
	}
	return err
}

// dbToLinear converts a dB RMS level to a 0.0-1.0 linear scale.
func dbToLinear(db float64) float64 {
	if db <= -60 {
		return 0
	}
	if db >= 0 {
		return 1
	}
	linear := (db + 60) / 60
	return linear * linear
}

// parseRMSFromMetadata extracts an RMS dB value from a metadata key.
func parseRMSFromMetadata(md map[string]interface{}, key string) (float64, bool) {
	rmsStr, ok := md[key]
	if !ok {
		return 0, false
	}
	s, ok := rmsStr.(string)
	if !ok {
		return 0, false
	}
	db, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return db, true
}

// GetRMS returns the current overall RMS level (0.0 to 1.0) from the astats filter.
func (m *MPV) GetRMS() float64 {
	val, err := m.GetProperty("af-metadata/stats")
	if err != nil {
		return 0
	}
	md, ok := val.(map[string]interface{})
	if !ok {
		return 0
	}
	db, ok := parseRMSFromMetadata(md, "lavfi.astats.Overall.RMS_level")
	if !ok {
		return 0
	}
	return dbToLinear(db)
}

func (m *MPV) Pause() error {
	_, err := m.sendCommand("set_property", "pause", true)
	if err == nil {
		m.Paused = true
	}
	return err
}

func (m *MPV) Resume() error {
	_, err := m.sendCommand("set_property", "pause", false)
	if err == nil {
		m.Paused = false
	}
	return err
}

func (m *MPV) TogglePause() error {
	if m.Paused {
		return m.Resume()
	}
	return m.Pause()
}

func (m *MPV) Stop() error {
	_, err := m.sendCommand("stop")
	if err == nil {
		m.Playing = false
		m.Paused = false
	}
	return err
}

func (m *MPV) SetVolume(vol int) error {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	_, err := m.sendCommand("set_property", "volume", float64(vol))
	return err
}

func (m *MPV) Seek(seconds float64) error {
	_, err := m.sendCommand("seek", seconds, "relative")
	return err
}

func (m *MPV) SeekAbsolute(seconds float64) error {
	_, err := m.sendCommand("seek", seconds, "absolute")
	return err
}

func (m *MPV) GetProperty(name string) (interface{}, error) {
	resp, err := m.sendCommand("get_property", name)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (m *MPV) GetPosition() float64 {
	val, err := m.GetProperty("time-pos")
	if err != nil {
		return 0
	}
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func (m *MPV) GetDuration() float64 {
	val, err := m.GetProperty("duration")
	if err != nil {
		return 0
	}
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func (m *MPV) IsIdle() bool {
	val, err := m.GetProperty("idle-active")
	if err != nil {
		return true
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return true
}

// PollProperties updates cached position/duration/idle in one shot.
// Call this from the tick handler so View() never blocks on IPC.
func (m *MPV) PollProperties() {
	m.CachedPosition = m.GetPosition()
	m.CachedDuration = m.GetDuration()
	m.CachedIdle = m.IsIdle()
}

func (m *MPV) Close() {
	if m.conn != nil {
		m.conn.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
	}
	os.Remove(m.socketPath)
}
