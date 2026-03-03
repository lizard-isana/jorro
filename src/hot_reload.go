package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const hotReloadPollInterval = 800 * time.Millisecond

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

func startHotReloadWatcher(root string, allowExtensions map[string]struct{}, onChange func()) (func(), error) {
	prev, err := scanServedFiles(root, allowExtensions)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})
	var once sync.Once
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(hotReloadPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				next, err := scanServedFiles(root, allowExtensions)
				if err != nil {
					continue
				}
				if !sameSnapshot(prev, next) {
					prev = next
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

func scanServedFiles(root string, allowExtensions map[string]struct{}) (map[string]fileFingerprint, error) {
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
		if !isAllowedExtension(path, allowExtensions) {
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
