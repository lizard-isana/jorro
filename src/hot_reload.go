package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var hotReloadPollInterval = 800 * time.Millisecond

var hotReloadDebounceWindow = 500 * time.Millisecond

type fileFingerprint struct {
	size    int64
	modTime int64
}

type hotReloadHub struct {
	mu          sync.Mutex
	subscribers map[chan struct{}]struct{}
}

func newHotReloadHub() *hotReloadHub {
	return &hotReloadHub{
		subscribers: map[chan struct{}]struct{}{},
	}
}

func (h *hotReloadHub) Subscribe(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{}, 1)

	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
		h.mu.Unlock()
	}()

	return ch
}

func (h *hotReloadHub) Publish() {
	h.mu.Lock()
	for ch := range h.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *hotReloadHub) HasSubscribers() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers) > 0
}

func startHotReloadWatcher(root string, watchExtensions map[string]struct{}, hasSubscribers func() bool, onChange func()) (func(), error) {
	if isLikelyNetworkPath(root) {
		return nil, fmt.Errorf("hot reload is disabled for network paths: %s", root)
	}

	stopCh := make(chan struct{})
	var once sync.Once
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(hotReloadPollInterval)
		defer ticker.Stop()
		prev := make(map[string]fileFingerprint)
		initialized := false
		var pending bool
		var lastChangeAt time.Time

		for {
			select {
			case <-stopCh:
				return
			case now := <-ticker.C:
				if hasSubscribers != nil && !hasSubscribers() {
					pending = false
					initialized = false
					continue
				}
				if !initialized {
					initial, err := scanWatchedFiles(root, watchExtensions)
					if err != nil {
						continue
					}
					prev = initial
					initialized = true
					continue
				}

				next, err := scanWatchedFiles(root, watchExtensions)
				if err != nil {
					continue
				}
				if !sameSnapshot(prev, next) {
					prev = next
					pending = true
					lastChangeAt = now
				}
				if pending && now.Sub(lastChangeAt) >= hotReloadDebounceWindow {
					pending = false
					onChange()
				}
			}
		}
	}()

	stop := func() {
		once.Do(func() {
			close(stopCh)
			<-done
		})
	}
	return stop, nil
}

func isLikelyNetworkPath(root string) bool {
	clean := filepath.Clean(root)
	vol := filepath.VolumeName(clean)
	if strings.HasPrefix(vol, `\\`) {
		return true
	}
	return strings.HasPrefix(clean, `\\`) || strings.HasPrefix(clean, "//")
}

func scanWatchedFiles(root string, watchExtensions map[string]struct{}) (map[string]fileFingerprint, error) {
	files := make(map[string]fileFingerprint, 64)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !isAllowedExtension(path, watchExtensions) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files[filepath.ToSlash(rel)] = fileFingerprint{
			size:    info.Size(),
			modTime: info.ModTime().UnixNano(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func sameSnapshot(a, b map[string]fileFingerprint) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if va != vb {
			return false
		}
	}
	return true
}
