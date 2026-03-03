package main

import (
	"net/http"
	"time"
)

func newHTTPServer(handler http.Handler, enableStreaming bool) *http.Server {
	writeTimeout := 30 * time.Second
	if enableStreaming {
		// SSE/hot-reload keeps responses open for a long time.
		// Disable write timeout to avoid forced chunked stream termination.
		writeTimeout = 0
	}

	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
}
