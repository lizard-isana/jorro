package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSecureHandler_MethodRestriction(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "<h1>ok</h1>")

	handler := mustSecureHandler(t, root, []string{".html"})

	req := httptest.NewRequest(http.MethodPost, "/index.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestSecureHandler_ServesAllowedFileAndIndex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "<h1>root</h1>")
	writeTestFile(t, filepath.Join(root, "docs", "index.html"), "<h1>docs</h1>")

	handler := mustSecureHandler(t, root, []string{".html"})

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "root index", path: "/", want: "<h1>root</h1>"},
		{name: "dir index", path: "/docs/", want: "<h1>docs</h1>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != tc.want {
				t.Fatalf("body=%q, want %q", rec.Body.String(), tc.want)
			}
		})
	}
}

func TestSecureHandler_RejectsDisallowedExtensionAndDirectoryListing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "<h1>ok</h1>")
	writeTestFile(t, filepath.Join(root, "script.js"), "console.log('x')")
	writeTestFile(t, filepath.Join(root, "list-only", "note.txt"), "note")

	handler := mustSecureHandler(t, root, []string{".html"})

	tests := []struct {
		name string
		path string
	}{
		{name: "disallowed extension", path: "/script.js"},
		{name: "directory listing disabled", path: "/list-only/"},
		{name: "hidden path blocked", path: "/.env"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

func TestSecureHandler_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "<h1>ok</h1>")

	outsideDir := t.TempDir()
	writeTestFile(t, filepath.Join(outsideDir, "secret.html"), "<h1>secret</h1>")

	linkPath := filepath.Join(root, "leak.html")
	if err := os.Symlink(filepath.Join(outsideDir, "secret.html"), linkPath); err != nil {
		t.Skipf("symlink not available in this environment: %v", err)
	}

	handler := mustSecureHandler(t, root, []string{".html"})

	req := httptest.NewRequest(http.MethodGet, "/leak.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSecureHandler_InjectsHotReloadScriptOnce(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "<html><body>Hello</body></html>")

	handler, _ := mustSecureHandlerWithHot(t, root, []string{".html"})

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data-jorro-hot-reload") {
		t.Fatalf("expected hot reload script to be injected")
	}
	if got := strings.Count(body, "data-jorro-hot-reload"); got != 1 {
		t.Fatalf("injected marker count=%d, want 1", got)
	}
}

func TestSecureHandler_HotReloadEventsStream(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "<html><body>ok</body></html>")

	handler, hub := mustSecureHandlerWithHot(t, root, []string{".html"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, hotReloadEventsPath, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Give the handler time to subscribe, then publish a few times to avoid races.
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 3; i++ {
		hub.Publish()
		time.Sleep(20 * time.Millisecond)
	}

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		cancel()
		<-done
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: reload") {
		t.Fatalf("expected reload event, got body=%q", body)
	}
}

func mustSecureHandler(t *testing.T, root string, extensions []string) http.Handler {
	t.Helper()

	allow, err := normalizeExtensions(extensions)
	if err != nil {
		t.Fatalf("normalizeExtensions() error: %v", err)
	}
	handler, err := newSecureHandler(root, allow, nil)
	if err != nil {
		t.Fatalf("newSecureHandler() error: %v", err)
	}
	return handler
}

func mustSecureHandlerWithHot(t *testing.T, root string, extensions []string) (http.Handler, *hotReloadHub) {
	t.Helper()

	allow, err := normalizeExtensions(extensions)
	if err != nil {
		t.Fatalf("normalizeExtensions() error: %v", err)
	}
	hub := newHotReloadHub()
	handler, err := newSecureHandler(root, allow, hub)
	if err != nil {
		t.Fatalf("newSecureHandler() error: %v", err)
	}
	return handler, hub
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
