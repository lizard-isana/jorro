package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

func listenLocalhost(host string, startPort, maxTry int) (net.Listener, int, error) {
	var lastErr error
	for i := 0; i < maxTry; i++ {
		port := startPort + i
		addr := host + ":" + strconv.Itoa(port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, port, nil
		}
		lastErr = err
	}

	ln, err := net.Listen("tcp", host+":0")
	if err == nil {
		addr, ok := ln.Addr().(*net.TCPAddr)
		if ok {
			return ln, addr.Port, nil
		}
		_ = ln.Close()
		return nil, 0, fmt.Errorf("fallback listener did not return TCPAddr")
	}

	if lastErr != nil {
		return nil, 0, fmt.Errorf("no available port in %d-%d (last error: %v)", startPort, startPort+maxTry-1, lastErr)
	}
	return nil, 0, fmt.Errorf("no available port in %d-%d", startPort, startPort+maxTry-1)
}

func rootDir(args []string) (string, error) {
	if len(args) >= 2 && args[1] != "" {
		return filepath.Abs(args[1])
	}

	exePath, err := os.Executable()
	if err != nil {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			return cwd, nil
		}
		return "", err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			return cwd, nil
		}
		return "", err
	}
	return filepath.Dir(exePath), nil
}
