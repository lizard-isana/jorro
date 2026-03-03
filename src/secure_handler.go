package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const hotReloadEventsPath = "/__jorro/events"

func newSecureHandler(root string, allowExtensions map[string]struct{}, hotReload *hotReloadHub) (http.Handler, error) {
	baseRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	rootInfo, err := os.Stat(baseRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", baseRoot)
	}

	// Some Windows network/virtualized paths fail on EvalSymlinks even when readable.
	// Fall back to the absolute path, but keep per-request symlink checks.
	if resolvedRoot, evalErr := filepath.EvalSymlinks(baseRoot); evalErr == nil {
		baseRoot = resolvedRoot
	}

	fileHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if hotReload != nil && r.URL.Path == hotReloadEventsPath {
			serveHotReloadEvents(w, r, hotReload)
			return
		}

		cleanURLPath := path.Clean("/" + r.URL.Path)
		if hasHiddenPathSegment(cleanURLPath) {
			http.NotFound(w, r)
			return
		}

		rel := strings.TrimPrefix(cleanURLPath, "/")
		fullPath := filepath.Join(baseRoot, filepath.FromSlash(rel))

		info, err := os.Stat(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if info.IsDir() {
			indexPath := filepath.Join(fullPath, "index.html")
			indexInfo, err := os.Stat(indexPath)
			if err != nil || indexInfo.IsDir() {
				http.NotFound(w, r)
				return
			}
			fullPath = indexPath
		}

		resolvedPath := fullPath
		if evalPath, err := filepath.EvalSymlinks(fullPath); err == nil {
			resolvedPath = evalPath
		} else {
			if hasSymlinkInPath(baseRoot, fullPath) {
				http.NotFound(w, r)
				return
			}
		}
		if !isUnderBase(baseRoot, resolvedPath) {
			http.NotFound(w, r)
			return
		}
		if !isAllowedExtension(resolvedPath, allowExtensions) {
			http.NotFound(w, r)
			return
		}
		if hotReload != nil && strings.EqualFold(filepath.Ext(resolvedPath), ".html") {
			serveHTMLWithHotReload(w, r, resolvedPath)
			return
		}

		http.ServeFile(w, r, resolvedPath)
	})

	return withSecurityHeaders(fileHandler), nil
}

func serveHotReloadEvents(w http.ResponseWriter, r *http.Request, hub *hotReloadHub) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("retry: 1000\n\n"))
	flusher.Flush()

	updates := hub.Subscribe(r.Context())
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			_, _ = w.Write([]byte("event: reload\ndata: changed\n\n"))
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

func serveHTMLWithHotReload(w http.ResponseWriter, r *http.Request, filePath string) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body = injectHotReloadScript(body)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func injectHotReloadScript(html []byte) []byte {
	if bytes.Contains(html, []byte("data-jorro-hot-reload")) {
		return html
	}

	snippet := []byte(`<script data-jorro-hot-reload>(function(){if(globalThis.__JORRO_HOT_RELOAD_ACTIVE__){return;}Object.defineProperty(globalThis,"__JORRO_HOT_RELOAD_ACTIVE__",{value:true,writable:false,configurable:false});var sse=new EventSource("/__jorro/events");sse.addEventListener("reload",function(){location.reload();});})();</script>`)
	lower := bytes.ToLower(html)
	closeBody := []byte("</body>")
	if idx := bytes.LastIndex(lower, closeBody); idx >= 0 {
		out := make([]byte, 0, len(html)+len(snippet))
		out = append(out, html[:idx]...)
		out = append(out, snippet...)
		out = append(out, html[idx:]...)
		return out
	}

	out := make([]byte, 0, len(html)+len(snippet))
	out = append(out, html...)
	out = append(out, snippet...)
	return out
}

func hasHiddenPathSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." {
			continue
		}
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func hasSymlinkInPath(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return true
	}
	if rel == "." {
		return false
	}

	current := base
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return true
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

func isUnderBase(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func isAllowedExtension(filePath string, allowExtensions map[string]struct{}) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false
	}
	_, ok := allowExtensions[ext]
	return ok
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
