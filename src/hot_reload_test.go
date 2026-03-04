package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStartHotReloadWatcher_DebouncesBurstChanges(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "index.html")
	writeHotReloadTestFile(t, filePath, "v0")

	allow, err := normalizeExtensions([]string{".html"})
	if err != nil {
		t.Fatalf("normalizeExtensions() error: %v", err)
	}

	oldPoll := hotReloadPollInterval
	oldDebounce := hotReloadDebounceWindow
	hotReloadPollInterval = 40 * time.Millisecond
	hotReloadDebounceWindow = 90 * time.Millisecond
	t.Cleanup(func() {
		hotReloadPollInterval = oldPoll
		hotReloadDebounceWindow = oldDebounce
	})

	var mu sync.Mutex
	triggered := 0
	changeCh := make(chan struct{}, 4)

	stop, err := startHotReloadWatcher(root, allow, func() bool {
		return true
	}, func() {
		mu.Lock()
		triggered++
		mu.Unlock()
		select {
		case changeCh <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("startHotReloadWatcher() error: %v", err)
	}
	defer stop()

	time.Sleep(20 * time.Millisecond)
	for i := 1; i <= 3; i++ {
		writeHotReloadTestFile(t, filePath, fmt.Sprintf("v%d-%d", i, time.Now().UnixNano()))
		time.Sleep(30 * time.Millisecond)
	}

	select {
	case <-changeCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected debounced reload event")
	}

	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	got := triggered
	mu.Unlock()
	if got != 1 {
		t.Fatalf("reload events=%d, want 1", got)
	}
}

func TestStartHotReloadWatcher_RejectsNetworkPath(t *testing.T) {
	allow, err := normalizeExtensions([]string{".html"})
	if err != nil {
		t.Fatalf("normalizeExtensions() error: %v", err)
	}

	_, err = startHotReloadWatcher(`\\server\share\site`, allow, func() bool { return true }, func() {})
	if err == nil {
		t.Fatalf("startHotReloadWatcher() error=nil, want error")
	}
}

func writeHotReloadTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
