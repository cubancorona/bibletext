//go:build bibletextdev

package bibletext

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	devDiagnosticsEnabled         = true
	devDiagnosticQueueSize        = 128
	devDiagnosticMaxMetrics       = 16
	devDiagnosticEventLimit int64 = 1 << 20
	devDiagnosticCrashLimit int64 = 256 << 10
)

// Build identity is available before Fyne creates the application. Packaging
// updates these same files; no runtime environment or user preference is read.
//
//go:embed cmd/mobile/FyneApp.toml
var devDiagnosticMobileMetadata string

//go:embed cmd/bibletext/FyneApp.toml
var devDiagnosticDesktopMetadata string

type devDiagnosticMetric struct {
	Key   string
	Value int64
}

var devDiagnosticEvents = map[string]string{
	"startup":         "startup",
	"lifecycle_start": "lifecycle_start", "lifecycle_stop": "lifecycle_stop", "panel_error": "panel_error",
	"foreground": "foreground", "background": "background",
	"foreground_enter_done": "foreground_enter_done", "background_done": "background_done",
	"appearance": "appearance", "window_close": "window_close",
	"load_start": "load_start", "load_failed": "load_failed",
	"load_complete": "load_complete", "load_ui_ready": "load_ui_ready",
	"panel_open": "panel_open", "panel_close": "panel_close", "panel_cancel": "panel_cancel",
	"panel_settings": "panel_settings", "panel_details": "panel_details",
	"composer_open": "composer_open", "composer_submit": "composer_submit", "composer_close": "composer_close",
	"request_start": "request_start", "request_complete": "request_complete", "request_ui_delivery": "request_ui_delivery",
	"request_drop_superseded": "request_drop_superseded", "request_drop_closed": "request_drop_closed",
	"panel_evicted": "panel_evicted", "cache_hit": "cache_hit", "cache_miss": "cache_miss",
	"render_start": "render_start", "render_parsed": "render_parsed", "render_wrapped": "render_wrapped",
	"render_fit_done": "render_fit_done",
	"remeasure_start": "remeasure_start", "remeasure_done": "remeasure_done", "remeasure_skipped": "remeasure_skipped",
	"native_will_resign": "native_will_resign", "native_did_background": "native_did_background",
	"native_will_foreground": "native_will_foreground", "native_did_active": "native_did_active",
	"native_memory_warning": "native_memory_warning", "native_will_terminate": "native_will_terminate",
	"memory_sample": "memory_sample",
}

var devDiagnosticMetricKeys = map[string]string{
	"panel_id": "panel_id", "request_id": "request_id", "action": "action", "generation": "generation",
	"bytes": "bytes", "duration_ms": "duration_ms", "width": "width", "height": "height",
	"status_class": "status_class", "segments": "segments", "rows": "rows",
	"cache_entries": "cache_entries", "cache_bytes": "cache_bytes", "selection_count": "selection_count",
	"native": "native", "reason": "reason", "question_bytes": "question_bytes", "passage_bytes": "passage_bytes",
	"queue_ms": "queue_ms", "error_class": "error_class", "http_status": "http_status", "context_class": "context_class",
	"heap_alloc_bytes": "heap_alloc_bytes", "heap_inuse_bytes": "heap_inuse_bytes", "heap_sys_bytes": "heap_sys_bytes",
	"go_sys_bytes": "go_sys_bytes", "gc_count": "gc_count", "goroutines": "goroutines",
	"native_footprint_bytes": "native_footprint_bytes",
}

type devDiagnosticRecord struct {
	at          time.Time
	event       string
	metrics     [devDiagnosticMaxMetrics]devDiagnosticMetric
	metricCount int
}

type devDiagnosticStatus struct {
	Schema       int    `json:"schema"`
	Enabled      bool   `json:"enabled"`
	Ready        bool   `json:"ready"`
	CrashReady   bool   `json:"crashReady"`
	StartupCount uint64 `json:"startupCount"`
	Session      uint64 `json:"session"`
	StartedUTC   string `json:"startedUTC"`
	UpdatedUTC   string `json:"updatedUTC"`
	Version      string `json:"version"`
	Build        int    `json:"build"`
	Next         bool   `json:"next"`
	Dev          bool   `json:"dev"`
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	GoVersion    string `json:"goVersion"`
	Written      uint64 `json:"written"`
	Dropped      uint64 `json:"dropped"`
	Rejected     uint64 `json:"rejected"`
	ErrorClass   int    `json:"errorClass"`
}

type devDiagnosticConfig struct {
	dir            string
	eventLimit     int64
	crashLimit     int64
	queueCapacity  int
	memoryInterval time.Duration
	memorySample   func() []devDiagnosticMetric
	crashOutput    func(*os.File) error
}

type devDiagnosticLogger struct {
	config         devDiagnosticConfig
	queue          chan devDiagnosticRecord
	stopSignal     chan struct{}
	ready          chan struct{}
	done           chan struct{}
	stopOnce       sync.Once
	stopped        atomic.Bool
	dropped        atomic.Uint64
	rejected       atomic.Uint64
	status         devDiagnosticStatus // worker-owned; read only after done in tests
	file           *os.File
	fileBytes      int64
	lastMemory     time.Time
	lastCheckpoint time.Time
	directoryReady bool
}

var (
	devDiagnosticStartOnce sync.Once
	devDiagnosticCurrent   atomic.Pointer[devDiagnosticLogger]
	devDiagnosticNextID    atomic.Uint64
)

func devDiagnosticsStart() {
	devDiagnosticStartOnce.Do(func() {
		logger := devNewDiagnosticLogger(devDiagnosticConfig{})
		devDiagnosticCurrent.Store(logger)
		go logger.run()
	})
}

func devDiagnosticID() uint64     { return devDiagnosticNextID.Add(1) }
func devDiagnosticNow() time.Time { return time.Now() }
func devDiagnosticSince(start time.Time) devDiagnosticMetric {
	var ms int64
	if !start.IsZero() {
		ms = time.Since(start).Milliseconds()
		if ms < 0 {
			ms = 0
		}
	}
	return devDiagnosticMetric{Key: "duration_ms", Value: ms}
}

func devDiagnosticEvent(event string, metrics ...devDiagnosticMetric) {
	if logger := devDiagnosticCurrent.Load(); logger != nil {
		logger.enqueue(event, metrics...)
	}
}

func devNewDiagnosticLogger(config devDiagnosticConfig) *devDiagnosticLogger {
	if config.eventLimit <= 0 {
		config.eventLimit = devDiagnosticEventLimit
	}
	if config.crashLimit <= 0 {
		config.crashLimit = devDiagnosticCrashLimit
	}
	if config.queueCapacity <= 0 {
		config.queueCapacity = devDiagnosticQueueSize
	}
	if config.memoryInterval <= 0 {
		config.memoryInterval = 10 * time.Second
	}
	if config.memorySample == nil {
		config.memorySample = devDiagnosticMemory
	}
	if config.crashOutput == nil {
		config.crashOutput = func(f *os.File) error { return debug.SetCrashOutput(f, debug.CrashOptions{}) }
	}
	return &devDiagnosticLogger{config: config, queue: make(chan devDiagnosticRecord, config.queueCapacity),
		stopSignal: make(chan struct{}), ready: make(chan struct{}), done: make(chan struct{})}
}

func (logger *devDiagnosticLogger) enqueue(event string, metrics ...devDiagnosticMetric) {
	if logger.stopped.Load() {
		return
	}
	if len(event) > 32 {
		logger.rejected.Add(1)
		return
	}
	canonical, ok := devDiagnosticEvents[event]
	if !ok {
		logger.rejected.Add(1)
		return
	}
	record := devDiagnosticRecord{at: time.Now(), event: canonical}
	// Both retained storage and caller-side work are bounded. Canonical strings
	// come from the allowlist, so a substring cannot retain caller-owned text.
	for index, metric := range metrics {
		if index >= devDiagnosticMaxMetrics {
			break
		}
		if len(metric.Key) > 32 {
			logger.rejected.Add(1)
			continue
		}
		key, ok := devDiagnosticMetricKeys[metric.Key]
		if !ok {
			logger.rejected.Add(1)
			continue
		}
		duplicate := false
		for i := 0; i < record.metricCount; i++ {
			if record.metrics[i].Key == key {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		record.metrics[record.metricCount] = devDiagnosticMetric{Key: key, Value: metric.Value}
		record.metricCount++
	}
	select {
	case logger.queue <- record:
	default:
		logger.dropped.Add(1)
	}
}

func (logger *devDiagnosticLogger) stop() {
	logger.stopOnce.Do(func() { logger.stopped.Store(true); close(logger.stopSignal) })
	<-logger.done
}

func (logger *devDiagnosticLogger) run() {
	defer close(logger.done)
	if err := logger.initialize(); err != nil {
		logger.status.Ready = false
		logger.checkpoint()
		close(logger.ready)
		logger.stopped.Store(true)
		if logger.file != nil {
			_ = logger.file.Close()
		}
		return
	}
	close(logger.ready)
	logger.writeRecord(devDiagnosticRecord{at: time.Now(), event: "startup"})
	logger.sampleMemory(time.Now())
	logger.checkpoint()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer func() {
		if logger.file != nil {
			_ = logger.file.Close()
		}
	}()
	for {
		select {
		case record := <-logger.queue:
			logger.writeRecord(record)
			if record.event == "native_memory_warning" ||
				(record.event == "render_fit_done" && time.Since(logger.lastMemory) >= time.Second) {
				logger.lastMemory = time.Time{}
			}
			logger.sampleMemory(time.Now())
			if devDiagnosticCritical(record.event) {
				logger.checkpoint()
			}
		case now := <-ticker.C:
			logger.sampleMemory(now)
			if now.Sub(logger.lastCheckpoint) >= 15*time.Second {
				logger.checkpoint()
			}
		case <-logger.stopSignal:
			for {
				select {
				case record := <-logger.queue:
					logger.writeRecord(record)
				default:
					logger.checkpoint()
					return
				}
			}
		}
	}
}

func (logger *devDiagnosticLogger) initialize() error {
	if logger.config.dir == "" {
		logger.config.dir = filepath.Join(filepath.Dir(defaultCachePath()), product.AppID, "runtime-diagnostic-dev")
	}
	dir := logger.config.dir
	logger.status = devDiagnosticStatus{Schema: 1, Enabled: true, Dev: true, Next: nextRelease,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version()}
	logger.status.Version, logger.status.Build = devDiagnosticBuild()
	logger.status.StartedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	logger.status.ErrorClass = 1
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("diagnostic directory unavailable")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	logger.directoryReady = true
	logger.status.ErrorClass = 2
	previous := filepath.Join(dir, "status.json")
	if devDiagnosticRegular(previous) == nil {
		if f, err := os.Open(previous); err == nil {
			raw, readErr := io.ReadAll(io.LimitReader(f, 8193))
			_ = f.Close()
			var state devDiagnosticStatus
			if readErr == nil && len(raw) <= 8192 && json.Unmarshal(raw, &state) == nil {
				logger.status.StartupCount = state.StartupCount
			}
		}
	}
	logger.status.StartupCount++
	logger.status.Session = logger.status.StartupCount
	if err := logger.writeStatus(); err != nil {
		return err
	}
	logger.status.ErrorClass = 3
	for _, name := range []string{"events.jsonl", "events-previous.jsonl"} {
		path := filepath.Join(dir, name)
		if err := devDiagnosticRetain(path, logger.config.eventLimit, true); err != nil {
			return err
		}
	}
	logger.file, err = devDiagnosticOpen(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY)
	if err != nil {
		return err
	}
	info, err = logger.file.Stat()
	if err != nil {
		return err
	}
	logger.fileBytes = info.Size()
	logger.status.ErrorClass = 4
	if err := logger.prepareCrash(); err != nil {
		return err
	}
	logger.status.CrashReady = true
	logger.status.Ready = true
	logger.status.ErrorClass = 0
	if err := logger.writeStatus(); err != nil {
		logger.status.ErrorClass = 5
		logger.status.Ready = false
		return err
	}
	return nil
}

func devDiagnosticBuild() (string, int) {
	if version, build, ok := devDiagnosticsNativeBuild(); ok {
		return version, build
	}
	source := devDiagnosticDesktopMetadata
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		source = devDiagnosticMobileMetadata
	}
	var metadata struct {
		Details struct {
			Version string
			Build   int
		}
	}
	if _, err := toml.Decode(source, &metadata); err != nil {
		return "", 0
	}
	return metadata.Details.Version, metadata.Details.Build
}

func devDiagnosticRegular(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("diagnostic file is not regular")
	}
	return nil
}

func devDiagnosticOpen(path string, flags int) (*os.File, error) {
	if err := devDiagnosticRegular(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// Retention reads at most limit bytes and replaces atomically. Event tails
// begin at a complete JSONL boundary; fatal tails may begin inside a frame.
func devDiagnosticRetain(path string, limit int64, lines bool) error {
	if err := devDiagnosticRegular(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	if info.Size() <= limit {
		return nil
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err := in.Seek(info.Size()-limit, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(in, limit))
	if err != nil {
		return err
	}
	if lines {
		if at := strings.IndexByte(string(raw), '\n'); at >= 0 {
			raw = raw[at+1:]
		} else {
			raw = nil
		}
	}
	return devDiagnosticAtomic(path, raw)
}

func devDiagnosticAtomic(path string, raw []byte) error {
	if err := devDiagnosticRegular(path); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".diagnostic-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (logger *devDiagnosticLogger) prepareCrash() error {
	dir := logger.config.dir
	current, previous := filepath.Join(dir, "go-panic.log"), filepath.Join(dir, "go-panic.previous.log")
	if err := devDiagnosticRetain(previous, logger.config.crashLimit, false); err != nil {
		return err
	}
	if err := devDiagnosticRetain(current, logger.config.crashLimit, false); err != nil {
		return err
	}
	if info, err := os.Stat(current); err == nil && info.Size() > 0 {
		if err := devDiagnosticRegular(previous); err != nil {
			return err
		}
		if err := os.Rename(current, previous); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := devDiagnosticOpen(current, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer f.Close()
	// The runtime duplicates this descriptor. A single fatal dump can exceed
	// the retention limit and may contain the panic value; it stays private.
	return logger.config.crashOutput(f)
}

func (logger *devDiagnosticLogger) writeRecord(record devDiagnosticRecord) {
	metrics := make(map[string]int64, record.metricCount)
	for i := 0; i < record.metricCount; i++ {
		metrics[record.metrics[i].Key] = record.metrics[i].Value
	}
	entry := struct {
		Schema   int              `json:"schema"`
		Session  uint64           `json:"session"`
		Sequence uint64           `json:"sequence"`
		AtUTC    string           `json:"atUTC"`
		Event    string           `json:"event"`
		Metrics  map[string]int64 `json:"metrics,omitempty"`
	}{1, logger.status.Session, logger.status.Written + 1, record.at.UTC().Format(time.RFC3339Nano), record.event, metrics}
	raw, err := json.Marshal(entry)
	if err != nil {
		logger.status.ErrorClass = 6
		logger.status.Ready = false
		return
	}
	raw = append(raw, '\n')
	if int64(len(raw)) > logger.config.eventLimit {
		logger.dropped.Add(1)
		return
	}
	if logger.fileBytes+int64(len(raw)) > logger.config.eventLimit {
		if err := logger.rotate(); err != nil {
			logger.status.ErrorClass = 7
			logger.status.Ready = false
			return
		}
	}
	n, err := logger.file.Write(raw)
	logger.fileBytes += int64(n)
	if err != nil || n != len(raw) {
		logger.status.ErrorClass = 6
		logger.status.Ready = false
		return
	}
	logger.status.Written++
}

func (logger *devDiagnosticLogger) rotate() error {
	if err := logger.file.Sync(); err != nil {
		return err
	}
	if err := logger.file.Close(); err != nil {
		return err
	}
	current, previous := filepath.Join(logger.config.dir, "events.jsonl"), filepath.Join(logger.config.dir, "events-previous.jsonl")
	if err := devDiagnosticRegular(previous); err != nil {
		return err
	}
	if err := os.Rename(current, previous); err != nil {
		return err
	}
	file, err := devDiagnosticOpen(current, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	logger.file, logger.fileBytes = file, 0
	return nil
}

func devDiagnosticCritical(event string) bool {
	switch event {
	case "background", "lifecycle_stop", "window_close", "native_did_background", "native_memory_warning", "native_will_terminate":
		return true
	}
	return false
}

func (logger *devDiagnosticLogger) checkpoint() {
	if !logger.directoryReady {
		return
	}
	if logger.file != nil {
		if err := logger.file.Sync(); err != nil {
			logger.status.ErrorClass = 6
			logger.status.Ready = false
		}
	}
	logger.status.Dropped, logger.status.Rejected = logger.dropped.Load(), logger.rejected.Load()
	if err := logger.writeStatus(); err != nil {
		logger.status.ErrorClass = 5
		logger.status.Ready = false
	}
	logger.lastCheckpoint = time.Now()
}

func (logger *devDiagnosticLogger) writeStatus() error {
	logger.status.UpdatedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(logger.status)
	if err != nil {
		return err
	}
	return devDiagnosticAtomic(filepath.Join(logger.config.dir, "status.json"), append(raw, '\n'))
}

func (logger *devDiagnosticLogger) sampleMemory(now time.Time) {
	if !logger.lastMemory.IsZero() && now.Sub(logger.lastMemory) < logger.config.memoryInterval {
		return
	}
	logger.lastMemory = now
	record := devDiagnosticRecord{at: now, event: "memory_sample"}
	for _, metric := range logger.config.memorySample() {
		key, ok := devDiagnosticMetricKeys[metric.Key]
		if !ok || record.metricCount >= devDiagnosticMaxMetrics {
			continue
		}
		record.metrics[record.metricCount] = devDiagnosticMetric{Key: key, Value: metric.Value}
		record.metricCount++
	}
	logger.writeRecord(record)
}

func devDiagnosticMemory() []devDiagnosticMetric {
	var memory runtime.MemStats
	// Sampling happens on the logger worker at a ten-second interval, with
	// refreshes on memory warnings and completed renders at least a second
	// after the previous sample. ReadMemStats briefly synchronizes runtime
	// state; no per-event read or forced garbage collection is performed.
	runtime.ReadMemStats(&memory)
	metrics := []devDiagnosticMetric{
		{Key: "heap_alloc_bytes", Value: int64(memory.HeapAlloc)},
		{Key: "heap_inuse_bytes", Value: int64(memory.HeapInuse)},
		{Key: "heap_sys_bytes", Value: int64(memory.HeapSys)},
		{Key: "go_sys_bytes", Value: int64(memory.Sys)},
		{Key: "gc_count", Value: int64(memory.NumGC)},
		{Key: "goroutines", Value: int64(runtime.NumGoroutine())},
	}
	if footprint, ok := devDiagnosticsNativeFootprint(); ok {
		metrics = append(metrics, devDiagnosticMetric{Key: "native_footprint_bytes", Value: footprint})
	}
	return metrics
}
