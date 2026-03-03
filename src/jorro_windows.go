//go:build windows

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	wmCommand          = 0x0111
	wmClose            = 0x0010
	wmDestroy          = 0x0002
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	ssLeft             = 0x00000000
	bsPushButton       = 0x00000000
	cwUseDefault       = 0x80000000
	swShowDefault      = 10
	mbIconError        = 0x00000010
	idiApplication     = 32512
	idOpenURL          = 1001
	idQuit             = 1002
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modUser32   = syscall.NewLazyDLL("user32.dll")

	procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")
	procRegisterClassW   = modUser32.NewProc("RegisterClassW")
	procCreateWindowExW  = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW   = modUser32.NewProc("DefWindowProcW")
	procDestroyWindow    = modUser32.NewProc("DestroyWindow")
	procPostQuitMessage  = modUser32.NewProc("PostQuitMessage")
	procPostMessageW     = modUser32.NewProc("PostMessageW")
	procGetMessageW      = modUser32.NewProc("GetMessageW")
	procTranslateMessage = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW = modUser32.NewProc("DispatchMessageW")
	procLoadIconW        = modUser32.NewProc("LoadIconW")
	procMessageBoxW      = modUser32.NewProc("MessageBoxW")
	procShowWindow       = modUser32.NewProc("ShowWindow")
	procUpdateWindow     = modUser32.NewProc("UpdateWindow")

	windowURL string
	appQuit   func()
)

type point struct {
	X int32
	Y int32
}

type msg struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClass struct {
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
}

func main() {
	const host = "127.0.0.1"

	root, err := rootDir(os.Args)
	if err != nil {
		showError("jorro", fmt.Sprintf("Error resolving root directory: %v", err))
		return
	}

	cfg, err := loadRuntimeConfig(root)
	if err != nil {
		showError("jorro", fmt.Sprintf("Error loading config: %v", err))
		return
	}

	var hotReload *hotReloadHub
	var stopHotReloadWatcher func()
	if cfg.HotReload {
		hotReload = newHotReloadHub()
		stopHotReloadWatcher, err = startHotReloadWatcher(root, cfg.AllowExtensions, hotReload.Publish)
		if err != nil {
			showError("jorro", fmt.Sprintf("Error starting hot reload watcher: %v", err))
			return
		}
	}

	ln, port, err := listenLocalhost(host, cfg.StartPort, 100)
	if err != nil {
		showError("jorro", fmt.Sprintf("Error: %v", err))
		return
	}

	url := "http://" + host + ":" + strconv.Itoa(port)
	handler, err := newSecureHandler(root, cfg.AllowExtensions, hotReload)
	if err != nil {
		showError("jorro", fmt.Sprintf("Error building secure handler: %v", err))
		return
	}
	server := newHTTPServer(handler, cfg.HotReload)

	var stopOnce sync.Once
	stopServer := func() {
		stopOnce.Do(func() {
			if stopHotReloadWatcher != nil {
				stopHotReloadWatcher()
			}
			_ = ln.Close()
			_ = server.Close()
		})
	}

	serverErrCh := make(chan error, 1)
	go func() {
		err := server.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	go func() {
		time.Sleep(400 * time.Millisecond)
		openBrowser(url)
	}()

	if err := runWindow(url, stopServer, serverErrCh); err != nil {
		stopServer()
		showError("jorro", err.Error())
	}
}

func runWindow(url string, onQuit func(), serverErrCh <-chan error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	appQuit = onQuit
	windowURL = url

	instance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("JorroMainWindowClass")
	title, _ := syscall.UTF16PtrFromString("jorro")

	icon, _, _ := procLoadIconW.Call(0, idiApplication)

	wc := wndClass{
		WndProc:   syscall.NewCallback(windowProc),
		Instance:  instance,
		Icon:      icon,
		ClassName: className,
	}

	atom, _, regErr := procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassW failed: %v", regErr)
	}

	hwnd, _, createErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow|wsVisible,
		cwUseDefault, cwUseDefault, 620, 200,
		0, 0,
		instance,
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed: %v", createErr)
	}

	if err := createControls(hwnd, windowURL); err != nil {
		_, _, _ = procDestroyWindow.Call(hwnd)
		return err
	}

	_, _, _ = procShowWindow.Call(hwnd, swShowDefault)
	_, _, _ = procUpdateWindow.Call(hwnd)

	go func() {
		for err := range serverErrCh {
			if err != nil {
				showError("jorro", fmt.Sprintf("Server error: %v", err))
				_, _, _ = procPostMessageW.Call(hwnd, wmClose, 0, 0)
				return
			}
		}
	}()

	var m msg
	for {
		ret, _, getErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			return fmt.Errorf("GetMessageW failed: %v", getErr)
		case 0:
			return nil
		default:
			_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}

func createControls(parent uintptr, url string) error {
	labelClass, _ := syscall.UTF16PtrFromString("STATIC")
	buttonClass, _ := syscall.UTF16PtrFromString("BUTTON")
	labelText, _ := syscall.UTF16PtrFromString("Running URL (click to open):")
	urlText, _ := syscall.UTF16PtrFromString(url)
	quitText, _ := syscall.UTF16PtrFromString("Quit")

	label, _, labelErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(labelClass)),
		uintptr(unsafe.Pointer(labelText)),
		wsChild|wsVisible|ssLeft,
		20, 20, 560, 24,
		parent,
		0,
		0,
		0,
	)
	if label == 0 {
		return fmt.Errorf("create label failed: %v", labelErr)
	}

	openBtn, _, openErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(urlText)),
		wsChild|wsVisible|bsPushButton,
		20, 52, 560, 36,
		parent,
		idOpenURL,
		0,
		0,
	)
	if openBtn == 0 {
		return fmt.Errorf("create open-url button failed: %v", openErr)
	}

	quitBtn, _, quitErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(quitText)),
		wsChild|wsVisible|bsPushButton,
		20, 104, 120, 32,
		parent,
		idQuit,
		0,
		0,
	)
	if quitBtn == 0 {
		return fmt.Errorf("create quit button failed: %v", quitErr)
	}

	return nil
}

func windowProc(hwnd uintptr, msgID uint32, wParam, lParam uintptr) uintptr {
	switch msgID {
	case wmCommand:
		switch loword(wParam) {
		case idOpenURL:
			openBrowser(windowURL)
			return 0
		case idQuit:
			_, _, _ = procPostMessageW.Call(hwnd, wmClose, 0, 0)
			return 0
		}
	case wmClose:
		_, _, _ = procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		if appQuit != nil {
			appQuit()
		}
		_, _, _ = procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msgID), wParam, lParam)
	return ret
}

func loword(v uintptr) uintptr {
	return v & 0xFFFF
}

func showError(title, text string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	_, _, _ = procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		mbIconError,
	)
}

func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
