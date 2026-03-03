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

	var hotReload *hotReloadHub
	var stopHotReloadWatcher func()
	if cfg.HotReload {
		hotReload = newHotReloadHub()
		stopHotReloadWatcher, err = startHotReloadWatcher(root, cfg.AllowExtensions, hotReload.Publish)
		if err != nil {
			fmt.Printf("Error starting hot reload watcher: %v\n", err)
			return
		}
		defer stopHotReloadWatcher()
	}

	ln, port, err := listenLocalhost(host, cfg.StartPort, 100)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer ln.Close()

	url := "http://" + host + ":" + strconv.Itoa(port)
	handler, err := newSecureHandler(root, cfg.AllowExtensions, hotReload)
	if err != nil {
		fmt.Printf("Error building secure handler: %v\n", err)
		return
	}
	server := newHTTPServer(handler, cfg.HotReload)

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
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}
}
