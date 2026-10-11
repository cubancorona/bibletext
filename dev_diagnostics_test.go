//go:build bibletextdev

package bibletext

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func devDiagnosticTestConfig(dir string) devDiagnosticConfig {
	return devDiagnosticConfig{dir: dir, memoryInterval: time.Hour,
		memorySample: func() []devDiagnosticMetric { return nil },
		crashOutput:  func(*os.File) error { return nil }}
}

func devDiagnosticTestStart(t *testing.T, config devDiagnosticConfig) *devDiagnosticLogger {
	t.Helper()
	logger := devNewDiagnosticLogger(config)
	go logger.run()
	select {
	case <-logger.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("diagnostic worker did not initialize")
	}
	t.Cleanup(logger.stop)
	return logger
}

func devDiagnosticTestStatus(t *testing.T, dir string) devDiagnosticStatus {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status devDiagnosticStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestDevDiagnosticsAllowlistAndQueueBound(t *testing.T) {
	logger := devNewDiagnosticLogger(devDiagnosticTestConfig(t.TempDir()))
	privateText := "synthetic-private-question-value"
	logger.enqueue(privateText, devDiagnosticMetric{Key: privateText, Value: 1})
	logger.enqueue("panel_open", devDiagnosticMetric{Key: privateText, Value: 1},
		devDiagnosticMetric{Key: "panel_id", Value: 12}, devDiagnosticMetric{Key: "panel_id", Value: 99})
	record := <-logger.queue
	if record.event != "panel_open" || record.metricCount != 1 || record.metrics[0].Value != 12 {
		t.Fatalf("unexpected filtered event: %+v", record)
	}
	if logger.rejected.Load() != 2 {
		t.Fatal("unknown event or metric was not rejected")
	}
	for i := 0; i < 4*devDiagnosticQueueSize; i++ {
		logger.enqueue("render_start")
	}
	if len(logger.queue) != devDiagnosticQueueSize || logger.dropped.Load() != 3*devDiagnosticQueueSize {
		t.Fatal("nonblocking queue did not retain its fixed capacity")
	}
	for len(logger.queue) > 0 {
		if record := <-logger.queue; record.metricCount > devDiagnosticMaxMetrics {
			t.Fatal("unbounded metrics")
		}
	}
	metrics := make([]devDiagnosticMetric, 100)
	for i := range metrics {
		metrics[i] = devDiagnosticMetric{Key: "bytes", Value: int64(i)}
	}
	logger.enqueue("render_start", metrics...)
	if record := <-logger.queue; record.metricCount != 1 {
		t.Fatal("duplicate metrics were retained")
	}
}

func TestDevDiagnosticsPrivateRecordsAndRelaunch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "diagnostics")
	config := devDiagnosticTestConfig(dir)
	first := devDiagnosticTestStart(t, config)
	first.enqueue("panel_open", devDiagnosticMetric{Key: "panel_id", Value: 8},
		devDiagnosticMetric{Key: "synthetic-private-question", Value: 999})
	first.enqueue("request_complete", devDiagnosticMetric{Key: "bytes", Value: 1234})
	first.stop()
	status := devDiagnosticTestStatus(t, dir)
	if !status.Enabled || !status.Ready || !status.CrashReady || status.StartupCount != 1 || status.Session != 1 {
		t.Fatalf("unexpected first startup status: %+v", status)
	}
	if !status.Dev || status.Next != nextRelease || status.Version == "" || status.Build <= 0 {
		t.Fatal("build identity not recorded")
	}
	before, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(before, []byte("synthetic-private-question")) {
		t.Fatal("private metric key was persisted")
	}
	for _, line := range bytes.Split(bytes.TrimSpace(before), []byte("\n")) {
		var event struct {
			Event   string           `json:"event"`
			Metrics map[string]int64 `json:"metrics"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if _, ok := devDiagnosticEvents[event.Event]; !ok {
			t.Fatal("noncanonical event persisted")
		}
		for key := range event.Metrics {
			if _, ok := devDiagnosticMetricKeys[key]; !ok {
				t.Fatal("noncanonical metric persisted")
			}
		}
	}
	second := devDiagnosticTestStart(t, config)
	second.enqueue("native_did_active")
	second.stop()
	status = devDiagnosticTestStatus(t, dir)
	if status.StartupCount != 2 || status.Session != 2 {
		t.Fatal("relaunch did not advance the persisted session")
	}
	after, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil || !bytes.HasPrefix(after, before) {
		t.Fatal("relaunch lost earlier event records")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatal("directory is not private")
		}
		for _, name := range []string{"status.json", "events.jsonl", "go-panic.log"} {
			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("%s is not private", name)
			}
		}
	}
}

func TestDevDiagnosticsRetentionAndActiveFileClosed(t *testing.T) {
	dir := t.TempDir()
	config := devDiagnosticTestConfig(dir)
	config.eventLimit, config.queueCapacity = 1024, 256
	for _, name := range []string{"events.jsonl", "events-previous.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), bytes.Repeat([]byte("{\"event\":\"startup\"}\n"), 400), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	logger := devDiagnosticTestStart(t, config)
	for i := 0; i < 100; i++ {
		logger.enqueue("render_start", devDiagnosticMetric{Key: "bytes", Value: int64(i)})
	}
	logger.stop()
	for _, name := range []string{"events.jsonl", "events-previous.jsonl"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.Size() > config.eventLimit {
			t.Fatalf("%s exceeded retention", name)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
			if len(line) != 0 && !json.Valid(line) {
				t.Fatal("rotation retained incomplete JSONL")
			}
		}
	}
	if _, err := logger.file.Write([]byte("synthetic")); err == nil {
		t.Fatal("rotated active descriptor leaked after stop")
	}
}

func TestDevDiagnosticsConcurrentProducers(t *testing.T) {
	logger := devDiagnosticTestStart(t, devDiagnosticTestConfig(t.TempDir()))
	const workers, perWorker = 8, 500
	var producers sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for i := 0; i < perWorker; i++ {
				logger.enqueue("render_wrapped", devDiagnosticMetric{Key: "request_id", Value: int64(devDiagnosticID())})
			}
		}()
	}
	producers.Wait()
	logger.stop()
	// Startup and its first memory sample account for the two worker records.
	if logger.status.Written+logger.dropped.Load() != workers*perWorker+2 {
		t.Fatalf("events disappeared outside bounded queue accounting: written=%d dropped=%d", logger.status.Written, logger.dropped.Load())
	}
	before := logger.status.Written
	logger.enqueue("render_start")
	if logger.status.Written != before {
		t.Fatal("stopped logger accepted more writes")
	}
}

func TestDevDiagnosticsMemorySamplingIsWorkerOnlyAndBounded(t *testing.T) {
	logger := devNewDiagnosticLogger(devDiagnosticTestConfig(t.TempDir()))
	var samples int
	logger.config.memorySample = func() []devDiagnosticMetric {
		samples++
		return []devDiagnosticMetric{{Key: "heap_alloc_bytes", Value: 99}}
	}
	logger.enqueue("render_start")
	if samples != 0 {
		t.Fatal("enqueue sampled memory on its caller")
	}
	if err := logger.initialize(); err != nil {
		t.Fatal(err)
	}
	defer logger.file.Close()
	now := time.Now()
	logger.sampleMemory(now)
	for i := 0; i < 100; i++ {
		logger.sampleMemory(now.Add(time.Millisecond))
	}
	if samples != 1 {
		t.Fatal("ordinary events bypassed sample interval")
	}
	logger.sampleMemory(now.Add(time.Hour))
	if samples != 2 {
		t.Fatal("eligible memory sample was missed")
	}
}

func TestDevDiagnosticsFailureReadinessAndSymlink(t *testing.T) {
	dir := t.TempDir()
	config := devDiagnosticTestConfig(dir)
	config.crashOutput = func(*os.File) error { return errors.New("synthetic-private-runtime-error") }
	failed := devDiagnosticTestStart(t, config)
	failed.stop()
	status := devDiagnosticTestStatus(t, dir)
	if status.Ready || status.CrashReady || status.ErrorClass != 4 {
		t.Fatal("failed crash setup reported readiness")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "status.json"))
	if err != nil || bytes.Contains(raw, []byte("synthetic-private-runtime-error")) {
		t.Fatal("raw error text persisted")
	}
	other := t.TempDir()
	symlink := filepath.Join(t.TempDir(), "diagnostics")
	if err := os.Symlink(other, symlink); err != nil {
		t.Skip("symlink creation unavailable")
	}
	rejected := devDiagnosticTestStart(t, devDiagnosticTestConfig(symlink))
	rejected.stop()
	entries, err := os.ReadDir(other)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected directory symlink received writes")
	}
}

func TestDevDiagnosticsSyncFailureMarksReadinessFalse(t *testing.T) {
	logger := devNewDiagnosticLogger(devDiagnosticTestConfig(t.TempDir()))
	if err := logger.initialize(); err != nil {
		t.Fatal(err)
	}
	if err := logger.file.Close(); err != nil {
		t.Fatal(err)
	}
	logger.checkpoint()
	status := devDiagnosticTestStatus(t, logger.config.dir)
	if status.Ready || status.ErrorClass != 6 {
		t.Fatal("failed flush left readiness true")
	}
}

func TestDevDiagnosticsPanicRetentionPreservesPrevious(t *testing.T) {
	dir := t.TempDir()
	config := devDiagnosticTestConfig(dir)
	config.crashLimit = 128
	current, previous := filepath.Join(dir, "go-panic.log"), filepath.Join(dir, "go-panic.previous.log")
	panicTail := []byte("synthetic latest fatal tail")
	if err := os.WriteFile(current, append(bytes.Repeat([]byte("x"), 400), panicTail...), 0o600); err != nil {
		t.Fatal(err)
	}
	for count := uint64(1); count <= 3; count++ {
		logger := devDiagnosticTestStart(t, config)
		logger.stop()
		raw, err := os.ReadFile(previous)
		if err != nil || len(raw) != int(config.crashLimit) || !bytes.HasSuffix(raw, panicTail) {
			t.Fatal("latest fatal tail was not retained across ordinary relaunches")
		}
		info, err := os.Stat(current)
		if err != nil || info.Size() != 0 {
			t.Fatal("new fatal output file was not empty")
		}
		if status := devDiagnosticTestStatus(t, dir); status.StartupCount != count {
			t.Fatal("startup count did not persist")
		}
	}
}

func TestDevDiagnosticsPanicSurvivesRelaunch(t *testing.T) {
	const childVariable = "BIBLETEXT_DIAGNOSTICS_CHILD_DIR"
	if dir := os.Getenv(childVariable); dir != "" {
		logger := devNewDiagnosticLogger(devDiagnosticConfig{dir: dir, memoryInterval: time.Hour})
		go logger.run()
		<-logger.ready
		if status := devDiagnosticTestStatus(t, dir); !status.Ready {
			t.Fatal("child crash recorder not ready")
		}
		logger.stop()
		panic("synthetic diagnostic panic")
	}
	dir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestDevDiagnosticsPanicSurvivesRelaunch$")
	child.Env = append(os.Environ(), childVariable+"="+dir)
	output, err := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("synthetic panic child did not exit2: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "go-panic.log"))
	if err != nil || !bytes.Contains(raw, []byte("synthetic diagnostic panic")) || !bytes.Contains(raw, []byte("TestDevDiagnosticsPanicSurvivesRelaunch")) {
		t.Fatal("runtime panic value and stack were not retained")
	}
	logger := devDiagnosticTestStart(t, devDiagnosticTestConfig(dir))
	logger.stop()
	previous, err := os.ReadFile(filepath.Join(dir, "go-panic.previous.log"))
	if err != nil || !bytes.Equal(previous, raw) {
		t.Fatal("relaunch did not preserve captured fatal output")
	}
}

func TestDevDiagnosticsReleaseStub(t *testing.T) {
	dir := t.TempDir()
	stub, err := os.ReadFile("dev_diagnostics_off.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dev_diagnostics_off.go"), stub, 0o600); err != nil {
		t.Fatal(err)
	}
	probe := `package bibletext
import ("os"; "testing"; "time")
func TestReleaseDiagnostics(t *testing.T) {
 before, _ := os.ReadDir(".")
 devDiagnosticsStart()
 devDiagnosticEvent("panel_open", devDiagnosticMetric{Key:"bytes", Value:123})
 if devDiagnosticsEnabled || devDiagnosticID()!=0 || !devDiagnosticNow().IsZero() || devDiagnosticSince(time.Now()).Value!=0 { t.Fatal("release diagnostics are enabled") }
 after, _ := os.ReadDir(".")
 if len(before)!=len(after) { t.Fatal("release diagnostics wrote files") }
}
`
	if err := os.WriteFile(filepath.Join(dir, "release_test.go"), []byte(probe), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "dev_diagnostics_off.go", "release_test.go")
	command.Dir = dir
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "GOFLAGS=") && !strings.HasPrefix(variable, "GOWORK=") {
			command.Env = append(command.Env, variable)
		}
	}
	command.Env = append(command.Env, "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("release no-op probe failed: %v\n%s", err, output)
	}
}
