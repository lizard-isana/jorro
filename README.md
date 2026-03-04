# Jorro

`127.0.0.1` のみで待ち受けるローカル専用のミニマムな静的Webサーバです。  
実行ファイルを置いたディレクトリをドキュメントルートとして配信します。

## 注意事項

- Jorro は公開サーバ用途ではなく、ローカル確認・開発中の動作確認を意図したツールです
- `127.0.0.1` バインドのため外部公開リスクは低い設計ですが、誤配布や誤公開を避けるため、サーバ上の公開ディレクトリにはアップロードしないでください

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
  "indexFile": "index.html",
  "allowExtensions": [".html", ".css", ".js", ".md", ".json"],
  "hotReload": false,
  "hotReloadWatchExtensions": [".html", ".css", ".js"],
  "htmlInclude": false,
  "htmlIncludeMaxDepth": 1
}
```

設定ルール:

- `jorro-config.json` が存在しない: デフォルト設定を使用
- 未知の設定キーがある: 起動時にエラー（設定ミスを防ぐため）
- `indexFile` が存在する: ディレクトリアクセス時の既定ファイル名を上書き（デフォルト: `index.html`）
- 起動時にルート直下の `indexFile` が見つからない場合は警告を表示
- `allowExtensions` が存在する: その拡張子のみ配信
- `allowExtensions` が存在しない: デフォルト拡張子を使用
- `hotReload: true`: HTMLに開発用スクリプトを動的挿入し、変更時に自動リロード
- `hotReloadWatchExtensions` が存在する: ホットリロード変更検知の拡張子を上書き（デフォルト: `.html/.css/.js`）
- ホットリロードは短いデバウンスを行い、保存直後の連続リロードを抑制
- ネットワークドライブ/UNC パスなどでは、ホットリロードが自動的に無効化される場合がある
- `htmlInclude: true`: HTML内の固定記法を展開（`<!--#include file="relative/path.html"-->` または `<!--#include virtual="/path/from/root.html"-->`）
- `htmlIncludeMaxDepth` が存在する: includeの最大深さを上書き（デフォルト: `1`、許容範囲: `1..16`）
- include失敗時はレスポンスをエラーにせず `<!-- jorro-include-error: ... -->` コメントを埋め込む
- include対象は配信ルート配下のみ（絶対パス/ルート外/隠しパス/不許可拡張子/シンボリックリンク逸脱は拒否）

デフォルト拡張子:

`[".html", ".css", ".js", ".mjs", ".map", ".json", ".md", ".txt", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".wasm"]`

### 4. 動作仕様（セキュリティ/配信）

- `GET` / `HEAD` 以外は拒否
- ドット始まりパス（例: `.env`, `.git`）は非公開
- ディレクトリ一覧は無効（`indexFile` に該当するファイルがないディレクトリは `404`）
- ルートに `404.html` がある場合、`404` 応答でその内容を返す
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

※ これらのスクリプトは配布向けに `-trimpath` と `-ldflags "-s -w"` を付けてビルドします。

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
