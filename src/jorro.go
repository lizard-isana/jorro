//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

func main() {
	const host = "127.0.0.1"

	root, err := rootDir(os.Args)
	if err != nil {
		fmt.Printf("Error resolving root directory: %v\n", err)
		return
	}

	cfg, err := loadRuntimeConfig(root)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}
	if indexPath, ok := rootIndexFilePath(root, cfg.IndexFile); !ok {
		fmt.Printf("Warning: configured index file not found: %s (GET / may return 404)\n", indexPath)
	}

	var hotReload *hotReloadHub
	var stopHotReloadWatcher func()
	if cfg.HotReload {
		hotReload = newHotReloadHub()
		stopHotReloadWatcher, err = startHotReloadWatcher(root, cfg.HotReloadWatchExtensions, hotReload.HasSubscribers, hotReload.Publish)
		if err != nil {
			fmt.Printf("Warning: hot reload is disabled: %v\n", err)
			hotReload = nil
		} else {
			defer stopHotReloadWatcher()
		}
	}

	ln, port, err := listenLocalhost(host, cfg.StartPort, 100)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer ln.Close()

	url := "http://" + host + ":" + strconv.Itoa(port)
	handler, err := newSecureHandler(root, cfg.AllowExtensions, cfg.IndexFile, hotReload, htmlIncludeConfig{
		Enabled:  cfg.HTMLInclude,
		MaxDepth: cfg.HTMLIncludeMaxDepth,
	})
	if err != nil {
		fmt.Printf("Error building secure handler: %v\n", err)
		return
	}
	server := newHTTPServer(handler, hotReload != nil)

	fmt.Printf("Serving from: %s\n", root)
	fmt.Printf("Listening on: %s\n", url)

	if os.Getenv("JORRO_NO_AUTO_OPEN") != "1" {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(url)
		}()
	}

	if err := server.Serve(ln); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = fmt.Errorf("windows browser launch is not supported in this build")
	case "darwin":
		err = runTrustedOpen(url, []string{"/usr/bin/open"})
	case "linux":
		err = runTrustedOpen(url, []string{"/usr/bin/xdg-open", "/bin/xdg-open"})
	default:
		err = fmt.Errorf("unsupported runtime OS: %s", runtime.GOOS)
	}
	if err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}
}

func runTrustedOpen(url string, candidates []string) error {
	for _, cmdPath := range candidates {
		info, err := os.Stat(cmdPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return exec.Command(cmdPath, url).Start()
	}
	return fmt.Errorf("trusted browser launcher was not found")
}
