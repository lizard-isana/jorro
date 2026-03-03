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
