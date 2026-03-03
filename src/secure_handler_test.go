package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasHiddenPathSegment(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/", want: false},
		{path: "/index.html", want: false},
		{path: "/assets/app.js", want: false},
		{path: "/.env", want: true},
		{path: "/a/.git/config", want: true},
	}

	for _, tc := range tests {
		got := hasHiddenPathSegment(tc.path)
		if got != tc.want {
			t.Fatalf("hasHiddenPathSegment(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsUnderBase(t *testing.T) {
	base := "/tmp/jorro"

	tests := []struct {
		target string
		want   bool
	}{
		{target: "/tmp/jorro/index.html", want: true},
		{target: "/tmp/jorro/sub/file.css", want: true},
		{target: "/tmp/jorro", want: true},
		{target: "/tmp/jorro2/file.txt", want: false},
		{target: "/tmp/secret.txt", want: false},
	}

	for _, tc := range tests {
		got := isUnderBase(base, tc.target)
		if got != tc.want {
			t.Fatalf("isUnderBase(%q, %q)=%v, want %v", base, tc.target, got, tc.want)
		}
	}
}

func TestIsAllowedExtension(t *testing.T) {
	allow := map[string]struct{}{
		".html": {},
		".css":  {},
	}

	tests := []struct {
		path string
		want bool
	}{
		{path: "/tmp/index.html", want: true},
		{path: "/tmp/style.css", want: true},
		{path: "/tmp/script.js", want: false},
		{path: "/tmp/noext", want: false},
	}

	for _, tc := range tests {
		got := isAllowedExtension(tc.path, allow)
		if got != tc.want {
			t.Fatalf("isAllowedExtension(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestHasSymlinkInPath(t *testing.T) {
	base := t.TempDir()
	plainDir := filepath.Join(base, "plain")
	if err := os.Mkdir(plainDir, 0o755); err != nil {
		t.Fatalf("mkdir plain: %v", err)
	}
	plainFile := filepath.Join(plainDir, "index.html")
	if err := os.WriteFile(plainFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.html")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkDir := filepath.Join(base, "linked")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Skipf("symlink not available in this environment: %v", err)
	}

	if got := hasSymlinkInPath(base, plainFile); got {
		t.Fatalf("hasSymlinkInPath(%q)=%v, want false", plainFile, got)
	}

	throughLink := filepath.Join(linkDir, "secret.html")
	if got := hasSymlinkInPath(base, throughLink); !got {
		t.Fatalf("hasSymlinkInPath(%q)=%v, want true", throughLink, got)
	}
}
