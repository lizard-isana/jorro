package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServer_DefaultWriteTimeout(t *testing.T) {
	srv := newHTTPServer(http.NotFoundHandler(), false)
	if srv.WriteTimeout != 30*time.Second {
		t.Fatalf("WriteTimeout=%v, want %v", srv.WriteTimeout, 30*time.Second)
	}
}

func TestNewHTTPServer_StreamingWriteTimeoutDisabled(t *testing.T) {
	srv := newHTTPServer(http.NotFoundHandler(), true)
	if srv.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout=%v, want 0", srv.WriteTimeout)
	}
}
