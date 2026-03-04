package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootIndexFilePath_ReturnsFoundFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "home.html")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	gotPath, ok := rootIndexFilePath(root, "home.html")
	if !ok {
		t.Fatalf("rootIndexFilePath() ok=false, want true")
	}
	if gotPath != path {
		t.Fatalf("path=%q, want %q", gotPath, path)
	}
}

func TestRootIndexFilePath_ReturnsMissingFile(t *testing.T) {
	root := t.TempDir()

	gotPath, ok := rootIndexFilePath(root, "home.html")
	if ok {
		t.Fatalf("rootIndexFilePath() ok=true, want false")
	}
	if gotPath != filepath.Join(root, "home.html") {
		t.Fatalf("path=%q, want %q", gotPath, filepath.Join(root, "home.html"))
	}
}
