package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuntimeConfig_DefaultWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if cfg.StartPort != defaultStartPort {
		t.Fatalf("StartPort=%d, want %d", cfg.StartPort, defaultStartPort)
	}
	if len(cfg.AllowExtensions) != len(defaultAllowExtensions) {
		t.Fatalf("allowExtensions len=%d, want %d", len(cfg.AllowExtensions), len(defaultAllowExtensions))
	}
	if cfg.HotReload {
		t.Fatalf("HotReload=%v, want false", cfg.HotReload)
	}
	if cfg.DevConsoleErrors {
		t.Fatalf("DevConsoleErrors=%v, want false", cfg.DevConsoleErrors)
	}
	if cfg.IndexFile != defaultIndexFile {
		t.Fatalf("IndexFile=%q, want %q", cfg.IndexFile, defaultIndexFile)
	}
	if len(cfg.HotReloadWatchExtensions) != len(defaultHotReloadWatchExtensions) {
		t.Fatalf("hotReloadWatchExtensions len=%d, want %d", len(cfg.HotReloadWatchExtensions), len(defaultHotReloadWatchExtensions))
	}
	if cfg.HTMLInclude {
		t.Fatalf("HTMLInclude=%v, want false", cfg.HTMLInclude)
	}
	if cfg.HTMLIncludeMaxDepth != defaultHTMLIncludeDepth {
		t.Fatalf("HTMLIncludeMaxDepth=%d, want %d", cfg.HTMLIncludeMaxDepth, defaultHTMLIncludeDepth)
	}
	for _, ext := range defaultHotReloadWatchExtensions {
		if _, ok := cfg.HotReloadWatchExtensions[ext]; !ok {
			t.Fatalf("expected %q in hotReloadWatchExtensions", ext)
		}
	}
}

func TestLoadRuntimeConfig_AllowExtensionsOverrideWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"port":9090,"allowExtensions":[".html","md"]}`
	if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if cfg.StartPort != 9090 {
		t.Fatalf("StartPort=%d, want 9090", cfg.StartPort)
	}
	if len(cfg.AllowExtensions) != 2 {
		t.Fatalf("allowExtensions len=%d, want 2", len(cfg.AllowExtensions))
	}
	if _, ok := cfg.AllowExtensions[".html"]; !ok {
		t.Fatalf("expected .html in allowExtensions")
	}
	if _, ok := cfg.AllowExtensions[".md"]; !ok {
		t.Fatalf("expected .md in allowExtensions")
	}
}

func TestLoadRuntimeConfig_AllowExtensionsDefaultWhenFieldMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"port":9090}`
	if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if cfg.StartPort != 9090 {
		t.Fatalf("StartPort=%d, want 9090", cfg.StartPort)
	}
	if len(cfg.AllowExtensions) != len(defaultAllowExtensions) {
		t.Fatalf("allowExtensions len=%d, want %d", len(cfg.AllowExtensions), len(defaultAllowExtensions))
	}
}

func TestLoadRuntimeConfig_HotReloadWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"hotReload":true}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if !cfg.HotReload {
		t.Fatalf("HotReload=%v, want true", cfg.HotReload)
	}
}

func TestLoadRuntimeConfig_DevConsoleErrorsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"devConsoleErrors":true}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if !cfg.DevConsoleErrors {
		t.Fatalf("DevConsoleErrors=%v, want true", cfg.DevConsoleErrors)
	}
}

func TestLoadRuntimeConfig_IndexFileWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"indexFile":"home.html"}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if cfg.IndexFile != "home.html" {
		t.Fatalf("IndexFile=%q, want %q", cfg.IndexFile, "home.html")
	}
}

func TestLoadRuntimeConfig_InvalidIndexFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"indexFile":"../home.html"}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := loadRuntimeConfig(dir)
	if err == nil {
		t.Fatalf("loadRuntimeConfig() error=nil, want error")
	}
}

func TestLoadRuntimeConfig_HotReloadWatchExtensionsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"hotReloadWatchExtensions":["html",".ts"]}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if len(cfg.HotReloadWatchExtensions) != 2 {
		t.Fatalf("hotReloadWatchExtensions len=%d, want 2", len(cfg.HotReloadWatchExtensions))
	}
	if _, ok := cfg.HotReloadWatchExtensions[".html"]; !ok {
		t.Fatalf("expected .html in hotReloadWatchExtensions")
	}
	if _, ok := cfg.HotReloadWatchExtensions[".ts"]; !ok {
		t.Fatalf("expected .ts in hotReloadWatchExtensions")
	}
}

func TestLoadRuntimeConfig_HTMLIncludeWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"htmlInclude":true}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if !cfg.HTMLInclude {
		t.Fatalf("HTMLInclude=%v, want true", cfg.HTMLInclude)
	}
}

func TestLoadRuntimeConfig_HTMLIncludeMaxDepthWhenPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"htmlIncludeMaxDepth":3}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error: %v", err)
	}
	if cfg.HTMLIncludeMaxDepth != 3 {
		t.Fatalf("HTMLIncludeMaxDepth=%d, want 3", cfg.HTMLIncludeMaxDepth)
	}
}

func TestLoadRuntimeConfig_InvalidHTMLIncludeMaxDepth(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"htmlIncludeMaxDepth":0}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := loadRuntimeConfig(dir)
	if err == nil {
		t.Fatalf("loadRuntimeConfig() error=nil, want error")
	}
}

func TestLoadRuntimeConfig_UnknownFieldReturnsError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	body := `{"port":8080,"unknownField":true}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := loadRuntimeConfig(dir)
	if err == nil {
		t.Fatalf("loadRuntimeConfig() error=nil, want error")
	}
}
