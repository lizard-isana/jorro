package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultStartPort        = 8080
	defaultIndexFile        = "index.html"
	defaultHTMLInclude      = false
	defaultHTMLIncludeDepth = 1
	maxHTMLIncludeDepth     = 16
	configFileName          = "jorro-config.json"
)

var defaultAllowExtensions = []string{
	".html", ".css", ".js", ".mjs", ".map", ".json",
	".md", ".txt", ".svg", ".png", ".jpg", ".jpeg",
	".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".wasm",
}

var defaultHotReloadWatchExtensions = []string{
	".html", ".css", ".js",
}

type fileConfig struct {
	Port                     *int      `json:"port"`
	AllowExtensions          *[]string `json:"allowExtensions"`
	IndexFile                *string   `json:"indexFile"`
	HotReload                *bool     `json:"hotReload"`
	DevConsoleErrors         *bool     `json:"devConsoleErrors"`
	HotReloadWatchExtensions *[]string `json:"hotReloadWatchExtensions"`
	HTMLInclude              *bool     `json:"htmlInclude"`
	HTMLIncludeMaxDepth      *int      `json:"htmlIncludeMaxDepth"`
}

type runtimeConfig struct {
	StartPort                int
	AllowExtensions          map[string]struct{}
	IndexFile                string
	HotReload                bool
	DevConsoleErrors         bool
	HotReloadWatchExtensions map[string]struct{}
	HTMLInclude              bool
	HTMLIncludeMaxDepth      int
}

func loadRuntimeConfig(root string) (runtimeConfig, error) {
	cfg := runtimeConfig{
		StartPort:                defaultStartPort,
		AllowExtensions:          normalizeExtensionsOrPanic(defaultAllowExtensions),
		IndexFile:                defaultIndexFile,
		HotReload:                false,
		DevConsoleErrors:         false,
		HotReloadWatchExtensions: normalizeExtensionsOrPanic(defaultHotReloadWatchExtensions),
		HTMLInclude:              defaultHTMLInclude,
		HTMLIncludeMaxDepth:      defaultHTMLIncludeDepth,
	}

	configPath := filepath.Join(root, configFileName)
	body, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return runtimeConfig{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	fc, err := parseFileConfig(body)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("parse %s: %w", configPath, err)
	}

	if fc.Port != nil {
		if *fc.Port < 1 || *fc.Port > 65535 {
			return runtimeConfig{}, fmt.Errorf("%s: port must be between 1 and 65535", configFileName)
		}
		cfg.StartPort = *fc.Port
	}

	if fc.AllowExtensions != nil {
		normalized, err := normalizeExtensions(*fc.AllowExtensions)
		if err != nil {
			return runtimeConfig{}, fmt.Errorf("%s: invalid allowExtensions: %w", configFileName, err)
		}
		cfg.AllowExtensions = normalized
	}
	if fc.IndexFile != nil {
		normalized, err := normalizeIndexFile(*fc.IndexFile)
		if err != nil {
			return runtimeConfig{}, fmt.Errorf("%s: invalid indexFile: %w", configFileName, err)
		}
		cfg.IndexFile = normalized
	}
	if fc.HotReload != nil {
		cfg.HotReload = *fc.HotReload
	}
	if fc.DevConsoleErrors != nil {
		cfg.DevConsoleErrors = *fc.DevConsoleErrors
	}
	if fc.HotReloadWatchExtensions != nil {
		normalized, err := normalizeExtensions(*fc.HotReloadWatchExtensions)
		if err != nil {
			return runtimeConfig{}, fmt.Errorf("%s: invalid hotReloadWatchExtensions: %w", configFileName, err)
		}
		cfg.HotReloadWatchExtensions = normalized
	}
	if fc.HTMLInclude != nil {
		cfg.HTMLInclude = *fc.HTMLInclude
	}
	if fc.HTMLIncludeMaxDepth != nil {
		if *fc.HTMLIncludeMaxDepth < 1 || *fc.HTMLIncludeMaxDepth > maxHTMLIncludeDepth {
			return runtimeConfig{}, fmt.Errorf("%s: htmlIncludeMaxDepth must be between 1 and %d", configFileName, maxHTMLIncludeDepth)
		}
		cfg.HTMLIncludeMaxDepth = *fc.HTMLIncludeMaxDepth
	}

	return cfg, nil
}

func parseFileConfig(body []byte) (fileConfig, error) {
	var fc fileConfig
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fc); err != nil {
		return fileConfig{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fileConfig{}, fmt.Errorf("multiple JSON values are not allowed")
		}
		return fileConfig{}, err
	}
	return fc, nil
}

func normalizeExtensionsOrPanic(list []string) map[string]struct{} {
	normalized, err := normalizeExtensions(list)
	if err != nil {
		panic(err)
	}
	return normalized
}

func normalizeExtensions(list []string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(list))
	for _, raw := range list {
		ext := strings.ToLower(strings.TrimSpace(raw))
		if ext == "" {
			return nil, fmt.Errorf("empty extension is not allowed")
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if ext == "." || strings.Contains(ext, "/") || strings.Contains(ext, "\\") {
			return nil, fmt.Errorf("invalid extension %q", raw)
		}
		result[ext] = struct{}{}
	}
	return result, nil
}

func normalizeIndexFile(raw string) (string, error) {
	indexFile := strings.TrimSpace(raw)
	if indexFile == "" {
		return "", fmt.Errorf("empty file name is not allowed")
	}
	if strings.Contains(indexFile, "/") || strings.Contains(indexFile, "\\") {
		return "", fmt.Errorf("path separators are not allowed")
	}
	if indexFile == "." || indexFile == ".." {
		return "", fmt.Errorf("invalid file name %q", raw)
	}
	if strings.HasPrefix(indexFile, ".") {
		return "", fmt.Errorf("hidden file is not allowed")
	}
	return indexFile, nil
}
