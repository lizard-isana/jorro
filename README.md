# Jorro

`127.0.0.1` のみで待ち受けるローカル専用のミニマムな静的Webサーバです。  
実行ファイルを置いたディレクトリをドキュメントルートとして配信します。

## Download
https://github.com/lizard-isana/jorro/releases/


## User Guide

### 1. 何ができるか

- ローカルPCで静的ファイルを配信
- 起動時にブラウザを開く
- 配信先は `127.0.0.1` 限定（外部公開しない）

### 2. 使い方（最短）

1. 配信したいフォルダに `jorro.app` / `jorro.exe` / `jorro-cli` を配置
2. 起動する
3. 自動でブラウザが開く

配布物:

- `dist/jorro.app`
- `dist/jorro.exe`
- `dist/jorro-cli`（Windowsターゲット時は `dist/jorro-cli.exe`）

### 3. 設定ファイル（`jorro-config.json`）

配信ルート（実行ファイルや `.app` を置く場所）に `jorro-config.json` を置くと設定できます。

```json
{
  "port": 8080,
  "allowExtensions": [".html", ".css", ".js", ".md", ".json"],
  "hotReload": false
}
```

設定ルール:

- `jorro-config.json` が存在しない: デフォルト設定を使用
- `allowExtensions` が存在する: その拡張子のみ配信
- `allowExtensions` が存在しない: デフォルト拡張子を使用
- `hotReload: true`: HTMLに開発用スクリプトを動的挿入し、変更時に自動リロード

デフォルト拡張子:

`[".html", ".css", ".js", ".mjs", ".map", ".json", ".md", ".txt", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".wasm"]`

### 4. 動作仕様（セキュリティ/配信）

- `GET` / `HEAD` 以外は拒否
- ドット始まりパス（例: `.env`, `.git`）は非公開
- ディレクトリ一覧は無効（`index.html` がないディレクトリは `404`）
- ルート外へ抜けるシンボリックリンクを遮断
- `Cache-Control: no-store` を付与
- ポートは `8080` から探索し、競合時は自動フォールバック

### 5. アイコン画像

`assets/icon-source.png` を置くとビルド時に以下を生成します。

- `dist/icons/jorro.icns`
- `dist/icons/jorro.ico`

## Build Guide

### 1. ディレクトリ構成

```text
.
├── assets/
│   └── icon-source.png
├── scripts/
│   ├── build-console.sh
│   ├── build-macos-app.sh
│   ├── build-windows-gui.sh
│   └── prepare-icons.sh
├── src/
│   ├── *.go
│   └── *_test.go
├── go.mod
├── go.sum
└── dist/                 # build output
```

### 2. スクリプトでビルド

Console版:

```bash
./scripts/build-console.sh
```

macOSアプリ版:

```bash
./scripts/build-macos-app.sh
```

Windows GUI版:

```bash
./scripts/build-windows-gui.sh
```

### 3. 手動ビルド

macOS / Linux（Console）:

```bash
go build -o ./dist/jorro-cli ./src
```

Windows（Console）:

```powershell
go build -o .\dist\jorro-cli.exe .\src
```

Windows GUI:

```bash
GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o ./dist/jorro.exe ./src
```

### 4. テスト

```bash
go test ./...
```

