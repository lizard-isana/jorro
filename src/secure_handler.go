package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const hotReloadEventsPath = "/__jorro/events"

var htmlIncludeDirectivePattern = regexp.MustCompile(`<!--\s*#include\s+(file|virtual)="([^"\r\n]+)"\s*-->`)

type htmlIncludeConfig struct {
	Enabled  bool
	MaxDepth int
}

func newSecureHandler(root string, allowExtensions map[string]struct{}, indexFile string, hotReload *hotReloadHub, includeCfg htmlIncludeConfig) (http.Handler, error) {
	baseRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	if includeCfg.MaxDepth < 1 {
		includeCfg.MaxDepth = defaultHTMLIncludeDepth
	}
	indexFile, err = normalizeIndexFile(indexFile)
	if err != nil {
		return nil, fmt.Errorf("invalid index file: %w", err)
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
		notFound := func() {
			serveNotFoundPage(w, r, baseRoot)
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if hotReload != nil && r.URL.Path == hotReloadEventsPath {
			serveHotReloadEvents(w, r, hotReload)
			return
		}

		rawURLPath := r.URL.Path
		if rawURLPath == "" {
			rawURLPath = "/"
		}
		cleanURLPath := path.Clean(rawURLPath)
		if !strings.HasPrefix(cleanURLPath, "/") {
			cleanURLPath = "/" + cleanURLPath
		}
		hasTrailingSlash := strings.HasSuffix(rawURLPath, "/")
		if hasHiddenPathSegment(cleanURLPath) {
			notFound()
			return
		}

		rel := strings.TrimPrefix(cleanURLPath, "/")
		fullPath := filepath.Join(baseRoot, filepath.FromSlash(rel))

		info, err := os.Stat(fullPath)
		if err != nil {
			notFound()
			return
		}

		if info.IsDir() {
			if !hasTrailingSlash {
				target := cleanURLPath + "/"
				if r.URL.RawQuery != "" {
					target += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, target, http.StatusTemporaryRedirect)
				return
			}

			indexPath := filepath.Join(fullPath, indexFile)
			indexInfo, err := os.Stat(indexPath)
			if err != nil || indexInfo.IsDir() {
				notFound()
				return
			}
			fullPath = indexPath
		}

		resolvedPath := fullPath
		if evalPath, err := filepath.EvalSymlinks(fullPath); err == nil {
			resolvedPath = evalPath
		} else {
			if hasSymlinkInPath(baseRoot, fullPath) {
				notFound()
				return
			}
		}
		if !isUnderBase(baseRoot, resolvedPath) {
			notFound()
			return
		}
		if !isAllowedExtension(resolvedPath, allowExtensions) {
			notFound()
			return
		}
		if strings.EqualFold(filepath.Ext(resolvedPath), ".html") {
			serveHTMLWithTransforms(w, r, resolvedPath, baseRoot, allowExtensions, hotReload != nil, includeCfg)
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

func serveHTMLWithTransforms(w http.ResponseWriter, r *http.Request, filePath, baseRoot string, allowExtensions map[string]struct{}, hotReload bool, includeCfg htmlIncludeConfig) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		serveNotFoundPage(w, r, baseRoot)
		return
	}
	if includeCfg.Enabled {
		body = renderHTMLIncludes(body, filePath, baseRoot, allowExtensions, includeCfg.MaxDepth)
	}
	if hotReload {
		body = injectHotReloadScript(body)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func renderHTMLIncludes(body []byte, filePath, baseRoot string, allowExtensions map[string]struct{}, maxDepth int) []byte {
	stack := map[string]struct{}{
		filePath: {},
	}
	return renderHTMLIncludesRecursive(body, filePath, baseRoot, allowExtensions, maxDepth, stack)
}

func renderHTMLIncludesRecursive(body []byte, parentFilePath, baseRoot string, allowExtensions map[string]struct{}, remainingDepth int, stack map[string]struct{}) []byte {
	matches := htmlIncludeDirectivePattern.FindAllSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body
	}

	out := make([]byte, 0, len(body))
	last := 0
	for _, m := range matches {
		out = append(out, body[last:m[0]]...)
		includeKind := strings.TrimSpace(string(body[m[2]:m[3]]))
		includePath := strings.TrimSpace(string(body[m[4]:m[5]]))

		if remainingDepth < 1 {
			out = append(out, includeErrorComment("max include depth exceeded")...)
			last = m[1]
			continue
		}

		includeBody, resolvedIncludePath, err := readIncludeFile(includeKind, includePath, parentFilePath, baseRoot, allowExtensions)
		if err != nil {
			out = append(out, includeErrorComment(err.Error())...)
			last = m[1]
			continue
		}
		if _, exists := stack[resolvedIncludePath]; exists {
			out = append(out, includeErrorComment("cyclic include detected")...)
			last = m[1]
			continue
		}

		stack[resolvedIncludePath] = struct{}{}
		includeBody = renderHTMLIncludesRecursive(includeBody, resolvedIncludePath, baseRoot, allowExtensions, remainingDepth-1, stack)
		delete(stack, resolvedIncludePath)

		out = append(out, includeBody...)
		last = m[1]
	}
	out = append(out, body[last:]...)
	return out
}

func readIncludeFile(includeKind, includeRef, parentFilePath, baseRoot string, allowExtensions map[string]struct{}) ([]byte, string, error) {
	ref := strings.TrimSpace(includeRef)
	if ref == "" {
		return nil, "", fmt.Errorf("include path is empty")
	}
	if strings.ContainsAny(ref, "?#") {
		return nil, "", fmt.Errorf("include path must not contain query or fragment: %s", ref)
	}

	unified := strings.ReplaceAll(ref, "\\", "/")
	cleanRef := path.Clean(unified)
	var candidate string
	switch includeKind {
	case "file":
		if strings.HasPrefix(cleanRef, "/") {
			return nil, "", fmt.Errorf("absolute include path is not allowed")
		}
		if cleanRef == "." || cleanRef == ".." {
			return nil, "", fmt.Errorf("invalid include path")
		}
		if hasHiddenIncludePathSegment(cleanRef) {
			return nil, "", fmt.Errorf("hidden include path is not allowed: %s", ref)
		}
		candidate = filepath.Join(filepath.Dir(parentFilePath), filepath.FromSlash(cleanRef))
	case "virtual":
		if !strings.HasPrefix(cleanRef, "/") {
			return nil, "", fmt.Errorf("virtual include path must start with /: %s", ref)
		}
		relRef := strings.TrimPrefix(cleanRef, "/")
		if hasHiddenIncludePathSegment(relRef) {
			return nil, "", fmt.Errorf("hidden include path is not allowed: %s", ref)
		}
		candidate = filepath.Join(baseRoot, filepath.FromSlash(relRef))
	default:
		return nil, "", fmt.Errorf("unsupported include type: %s", includeKind)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return nil, "", fmt.Errorf("include not found: %s", ref)
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("include target is a directory: %s", ref)
	}

	resolved := candidate
	if evalPath, err := filepath.EvalSymlinks(candidate); err == nil {
		resolved = evalPath
	} else if hasSymlinkInPath(baseRoot, candidate) {
		return nil, "", fmt.Errorf("include via symlink is not allowed: %s", ref)
	}
	if !isUnderBase(baseRoot, resolved) {
		return nil, "", fmt.Errorf("include outside root is not allowed: %s", ref)
	}
	if !isAllowedExtension(resolved, allowExtensions) {
		return nil, "", fmt.Errorf("include extension is not allowed: %s", ref)
	}

	body, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("include read failed: %s", ref)
	}
	return body, resolved, nil
}

func includeErrorComment(reason string) []byte {
	clean := strings.TrimSpace(reason)
	clean = strings.ReplaceAll(clean, "\r", " ")
	clean = strings.ReplaceAll(clean, "\n", " ")
	clean = strings.ReplaceAll(clean, "--", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		clean = "include failed"
	}
	return []byte("<!-- jorro-include-error: " + clean + " -->")
}

func serveNotFoundPage(w http.ResponseWriter, r *http.Request, baseRoot string) {
	customPath := filepath.Join(baseRoot, "404.html")
	info, err := os.Stat(customPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	body, err := os.ReadFile(customPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if r.Method == http.MethodHead {
		return
	}
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

func hasHiddenIncludePathSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
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
