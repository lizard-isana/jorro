package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultStartPort = 8080
	configFileName   = "jorro-config.json"
)

var defaultAllowExtensions = []string{
	".html", ".css", ".js", ".mjs", ".map", ".json",
	".md", ".txt", ".svg", ".png", ".jpg", ".jpeg",
	".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".wasm",
}

type fileConfig struct {
	Port            *int      `json:"port"`
	AllowExtensions *[]string `json:"allowExtensions"`
	HotReload       *bool     `json:"hotReload"`
}

type runtimeConfig struct {
	StartPort       int
	AllowExtensions map[string]struct{}
	HotReload       bool
}

func loadRuntimeConfig(root string) (runtimeConfig, error) {
	cfg := runtimeConfig{
		StartPort:       defaultStartPort,
		AllowExtensions: normalizeExtensionsOrPanic(defaultAllowExtensions),
		HotReload:       false,
	}

	configPath := filepath.Join(root, configFileName)
	body, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return runtimeConfig{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(body, &fc); err != nil {
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
	if fc.HotReload != nil {
		cfg.HotReload = *fc.HotReload
	}

	return cfg, nil
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
