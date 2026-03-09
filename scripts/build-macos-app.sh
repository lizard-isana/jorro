#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SRC_DIR="$ROOT_DIR/src"
DIST_DIR="$ROOT_DIR/dist"
APP_DIR="$DIST_DIR/jorro.app"
DMG_PATH="$DIST_DIR/jorro.dmg"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"
BIN_PATH="$MACOS_DIR/jorro-bin"
LAUNCHER_PATH="$MACOS_DIR/jorro-launcher"
LAUNCHER_SRC="$ROOT_DIR/.jorro-launcher.swift"
PLIST_PATH="$CONTENTS_DIR/Info.plist"
ICONS_DIR="$DIST_DIR/icons"
PREPARE_ICONS_SCRIPT="$ROOT_DIR/scripts/prepare-icons.sh"

mkdir -p "$MACOS_DIR" "$RESOURCES_DIR"

if ! "$PREPARE_ICONS_SCRIPT"; then
  if [[ -f "$ICONS_DIR/jorro.icns" ]]; then
    echo "Warning: failed to regenerate .icns, reusing existing file: $ICONS_DIR/jorro.icns"
  else
    echo "Error: failed to generate .icns and no existing fallback is available."
    exit 1
  fi
fi

go build -trimpath -ldflags "-s -w" -o "$BIN_PATH" "$SRC_DIR"

cat >"$LAUNCHER_SRC" <<'EOF'
import Cocoa
import Foundation

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var server: Process?
    private var logHandle: FileHandle?
    private var outputPipe: Pipe?
    private var outputBuffer = Data()
    private var window: NSWindow?
    private var statusLabel: NSTextField?
    private var urlButton: NSButton?
    private var currentURL: URL?
    private var didAutoOpenURL = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        setupMenu()
        setupWindow()
        launchServer()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }

    func applicationWillTerminate(_ notification: Notification) {
        if let server, server.isRunning {
            server.terminate()
        }
        try? logHandle?.close()
    }

    private func setupMenu() {
        let app = NSApplication.shared
        let mainMenu = NSMenu()
        let appItem = NSMenuItem()
        mainMenu.addItem(appItem)
        let appMenu = NSMenu()
        appItem.submenu = appMenu
        appMenu.addItem(withTitle: "Quit jorro", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        app.mainMenu = mainMenu
    }

    private func setupWindow() {
        let frame = NSRect(x: 0, y: 0, width: 620, height: 200)
        let win = NSWindow(
            contentRect: frame,
            styleMask: [.titled, .closable, .miniaturizable],
            backing: .buffered,
            defer: false
        )
        win.title = "jorro"
        win.center()

        guard let content = win.contentView else {
            return
        }

        let label = NSTextField(labelWithString: "Running URL (click to open):")
        label.frame = NSRect(x: 20, y: 148, width: 560, height: 20)
        content.addSubview(label)

        let urlBtn = NSButton(title: "Starting server...", target: self, action: #selector(openCurrentURL))
        urlBtn.frame = NSRect(x: 20, y: 108, width: 560, height: 32)
        urlBtn.bezelStyle = .rounded
        urlBtn.isEnabled = false
        content.addSubview(urlBtn)
        urlButton = urlBtn

        let status = NSTextField(labelWithString: "Starting server...")
        status.frame = NSRect(x: 20, y: 76, width: 560, height: 20)
        content.addSubview(status)
        statusLabel = status

        let quitBtn = NSButton(title: "Quit", target: NSApplication.shared, action: #selector(NSApplication.terminate(_:)))
        quitBtn.frame = NSRect(x: 20, y: 28, width: 100, height: 30)
        quitBtn.bezelStyle = .rounded
        content.addSubview(quitBtn)

        window = win
        win.makeKeyAndOrderFront(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)
    }

    @objc private func openCurrentURL() {
        guard let currentURL else {
            return
        }
        NSWorkspace.shared.open(currentURL)
    }

    private func launchServer() {
        let execURL = URL(fileURLWithPath: CommandLine.arguments[0]).resolvingSymlinksInPath()
        let macOSDir = execURL.deletingLastPathComponent()
        let serveRoot = macOSDir
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .path
        let serverURL = macOSDir.appendingPathComponent("jorro-bin")

        let logURL = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Logs/jorro.log")
        try? FileManager.default.createDirectory(
            at: logURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        if !FileManager.default.fileExists(atPath: logURL.path) {
            FileManager.default.createFile(atPath: logURL.path, contents: nil)
        }
        if let handle = try? FileHandle(forWritingTo: logURL) {
            _ = try? handle.seekToEnd()
            logHandle = handle
        }

        let pipe = Pipe()
        outputPipe = pipe

        let proc = Process()
        proc.executableURL = serverURL
        proc.arguments = [serveRoot]
        var env = ProcessInfo.processInfo.environment
        env["JORRO_NO_AUTO_OPEN"] = "1"
        proc.environment = env
        proc.standardOutput = pipe
        proc.standardError = pipe
        proc.terminationHandler = { _ in
            DispatchQueue.main.async {
                NSApplication.shared.terminate(nil)
            }
        }

        do {
            try proc.run()
            server = proc
            startReadingOutput(pipe)
        } catch {
            let alert = NSAlert()
            alert.messageText = "Failed to start jorro"
            alert.informativeText = error.localizedDescription
            alert.runModal()
            NSApplication.shared.terminate(nil)
        }
    }

    private func startReadingOutput(_ pipe: Pipe) {
        let handle = pipe.fileHandleForReading
        handle.readabilityHandler = { [weak self] fileHandle in
            let data = fileHandle.availableData
            guard !data.isEmpty else {
                fileHandle.readabilityHandler = nil
                return
            }
            self?.appendLog(data)
            self?.consumeOutput(data)
        }
    }

    private func appendLog(_ data: Data) {
        if let logHandle {
            try? logHandle.write(contentsOf: data)
        }
    }

    private func consumeOutput(_ data: Data) {
        outputBuffer.append(data)
        while let lineRange = outputBuffer.range(of: Data([0x0a])) {
            let lineData = outputBuffer.subdata(in: 0..<lineRange.lowerBound)
            outputBuffer.removeSubrange(0...lineRange.lowerBound)
            guard let line = String(data: lineData, encoding: .utf8) else {
                continue
            }
            parseServerLine(line.trimmingCharacters(in: .whitespacesAndNewlines))
        }
    }

    private func parseServerLine(_ line: String) {
        let prefix = "Listening on: "
        if line.hasPrefix(prefix) {
            let urlString = String(line.dropFirst(prefix.count))
            DispatchQueue.main.async { [weak self] in
                guard let self else {
                    return
                }
                self.statusLabel?.stringValue = "Server is running."
                self.urlButton?.title = urlString
                self.urlButton?.isEnabled = true
                self.currentURL = URL(string: urlString)
                if !self.didAutoOpenURL, let url = self.currentURL {
                    self.didAutoOpenURL = true
                    NSWorkspace.shared.open(url)
                }
            }
        }
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
EOF

if command -v xcrun >/dev/null 2>&1; then
  xcrun swiftc -O -framework Cocoa "$LAUNCHER_SRC" -o "$LAUNCHER_PATH"
elif command -v swiftc >/dev/null 2>&1; then
  swiftc -O -framework Cocoa "$LAUNCHER_SRC" -o "$LAUNCHER_PATH"
else
  echo "Error: swiftc is not available. Install Xcode Command Line Tools."
  exit 1
fi

rm -f "$LAUNCHER_SRC"
cp "$ICONS_DIR/jorro.icns" "$RESOURCES_DIR/jorro.icns"

cat >"$PLIST_PATH" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>jorro</string>
	<key>CFBundleDisplayName</key>
	<string>jorro</string>
	<key>CFBundleIdentifier</key>
	<string>local.jorro.macos</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleExecutable</key>
	<string>jorro-launcher</string>
	<key>CFBundleIconFile</key>
	<string>jorro</string>
	<key>LSMinimumSystemVersion</key>
	<string>12.0</string>
	<key>LSUIElement</key>
	<false/>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
</dict>
</plist>
EOF

echo "Built: $APP_DIR"

if ! command -v hdiutil >/dev/null 2>&1; then
  echo "Warning: hdiutil not found. Skipped DMG packaging."
  exit 0
fi

rm -f "$DMG_PATH"
hdiutil create -volname "jorro" -srcfolder "$APP_DIR" -ov -format UDZO "$DMG_PATH" >/dev/null
echo "Built: $DMG_PATH"
