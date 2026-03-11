# API Workbench

**CLI-first API workbench — requests live in Git, run in terminal & CI, with a desktop GUI.**

[English](#english) | [繁體中文](#繁體中文) | [日本語](#日本語)

---

## English

### What is API Workbench?

A lightweight, zero-dependency API testing tool built around a simple idea:

- **Requests live in your repo** as JSON files, not inside a desktop app
- **Terminal is the default** — run from CLI or CI with the same files
- **Assertions built in** — validate status, headers, body, JSON paths, regex, and timing
- **Request chaining** — extract values from responses and feed them into subsequent requests
- **Desktop GUI included** — Tauri-based native app with i18n (EN/繁中/簡中/日) and themes

### Install

**macOS Desktop App (Apple Silicon)**

Download `API.Workbench_0.1.0_aarch64.dmg` from the [Releases](https://github.com/MakiDevelop/api-workbench/releases) page.

**CLI only**

```bash
go install github.com/MakiDevelop/api-workbench/cmd/apiw@latest
```

Or build from source:

```bash
git clone https://github.com/MakiDevelop/api-workbench.git
cd api-workbench
go build -o bin/apiw ./cmd/apiw
```

### Quick Start

```bash
# Create a project with 8 built-in demo requests
apiw init --demo

# Run all demos (hits httpbin.org, dog.ceo, jsonplaceholder)
apiw run --all --env demo

# Run a single request
apiw run requests/01-get-ip.json --env demo

# Open terminal UI
apiw tui
```

### Project Structure

```
my-project/
├── .apiw/
│   ├── apiw.json          # project config
│   ├── env/
│   │   ├── local.env      # variables for local
│   │   └── demo.env       # variables for demo
│   └── snapshots/         # saved response snapshots
└── requests/
    ├── 01-get-ip.json
    ├── 02-post-json.json
    └── ...
```

### Request Spec Format

```json
{
  "name": "Create User",
  "method": "POST",
  "url": "$BASE_URL/users",
  "headers": {
    "Authorization": "Bearer $TOKEN"
  },
  "query": {
    "source": "apiw"
  },
  "body": {
    "type": "json",
    "content": {
      "name": "Maki",
      "role": "developer"
    }
  },
  "assertions": [
    { "type": "status", "equals": 201 },
    { "type": "json_path", "path": "/name", "expected": "Maki" },
    { "type": "duration_under", "under": 2000 }
  ],
  "extract": {
    "USER_ID": "/id"
  }
}
```

### Body Types

| Type | Content-Type | Content format |
|------|-------------|----------------|
| `json` (default) | `application/json` | JSON object/array/value |
| `text` | `text/plain` | JSON string (`"hello"`) |
| `form` | `application/x-www-form-urlencoded` | JSON object with string values |

### Assertion Types

| Type | Fields | Description |
|------|--------|-------------|
| `status` | `equals` | HTTP status code matches |
| `body_contains` | `contains` | Response body contains string |
| `body_regex` | `pattern` | Response body matches regex |
| `header_equals` | `key`, `value` | Response header matches (case-insensitive) |
| `json_path` | `path`, `expected` | JSON Pointer (RFC 6901) value matches |
| `json_path_count` | `path`, `equals` | Array length at JSON Pointer matches |
| `duration_under` | `under` | Response time under N milliseconds |

### Request Chaining

Use `extract` to pull values from a response and inject them into subsequent requests during collection runs:

```json
{
  "name": "Login",
  "method": "POST",
  "url": "$BASE_URL/auth/login",
  "body": { "type": "json", "content": { "user": "admin", "pass": "secret" } },
  "extract": {
    "TOKEN": "/accessToken"
  }
}
```

The next request in the collection can use `$TOKEN`:

```json
{
  "name": "Get Profile",
  "method": "GET",
  "url": "$BASE_URL/me",
  "headers": { "Authorization": "Bearer $TOKEN" }
}
```

Cookies are automatically shared across all requests in a collection run.

### Environment Variables

Variables are defined in `.apiw/env/<name>.env` files:

```
BASE_URL=https://api.example.com
TOKEN=my-secret-token
```

Use `$VAR` or `${VAR}` syntax in request specs. File values always take priority over process environment variables.

### CLI Commands

```bash
apiw init                  # Create a new project
apiw init --demo           # Create project with 8 demo requests
apiw run <file> [flags]    # Run a single request
apiw run --all [flags]     # Run all requests in collection
apiw tui                   # Open terminal UI
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--env` | `local` | Environment name |
| `--timeout` | `15s` | Request timeout |
| `--snapshot` | `false` | Save response snapshot |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All passed |
| 1 | Invalid request spec |
| 2 | Transport error (network, timeout) |
| 3 | Assertion failure |

### Desktop GUI

The Tauri-based desktop app provides:

- Visual request editor and response viewer
- One-click collection runs
- Environment and request switching
- 4 languages: English, 繁體中文, 簡體中文, 日本語
- 3 themes: Light, Dark, The Matrix

Build from source:

```bash
npm install
npm run tauri:build:mac
```

---

## 繁體中文

### 什麼是 API Workbench？

輕量級、零外部依賴的 API 測試工具：

- **Request 存在 repo 裡** — JSON 檔案，不是藏在桌面 app 裡
- **終端機優先** — CLI 和 CI 用同一份檔案
- **內建斷言** — 驗證 status、header、body、JSON path、regex、回應時間
- **Request 串連** — 從 response 取值，自動注入後續 request
- **桌面 GUI** — Tauri 原生 app，支援繁中/簡中/英/日 + 三種佈景主題

### 安裝

**macOS 桌面 App（Apple Silicon）**

從 [Releases](https://github.com/MakiDevelop/api-workbench/releases) 頁面下載 `API.Workbench_0.1.0_aarch64.dmg`。

**僅安裝 CLI**

```bash
go install github.com/MakiDevelop/api-workbench/cmd/apiw@latest
```

或從原始碼建置：

```bash
git clone https://github.com/MakiDevelop/api-workbench.git
cd api-workbench
go build -o bin/apiw ./cmd/apiw
```

### 快速開始

```bash
# 建立專案並包含 8 個內建範例
apiw init --demo

# 執行所有範例（打 httpbin.org、dog.ceo、jsonplaceholder）
apiw run --all --env demo

# 執行單一 request
apiw run requests/01-get-ip.json --env demo

# 開啟終端機 UI
apiw tui
```

### 專案結構

```
my-project/
├── .apiw/
│   ├── apiw.json          # 專案設定
│   ├── env/
│   │   ├── local.env      # local 環境變數
│   │   └── demo.env       # demo 環境變數
│   └── snapshots/         # 回應快照
└── requests/
    ├── 01-get-ip.json
    ├── 02-post-json.json
    └── ...
```

### Request 格式

```json
{
  "name": "建立使用者",
  "method": "POST",
  "url": "$BASE_URL/users",
  "headers": {
    "Authorization": "Bearer $TOKEN"
  },
  "body": {
    "type": "json",
    "content": { "name": "Maki", "role": "developer" }
  },
  "assertions": [
    { "type": "status", "equals": 201 },
    { "type": "json_path", "path": "/name", "expected": "Maki" },
    { "type": "duration_under", "under": 2000 }
  ],
  "extract": {
    "USER_ID": "/id"
  }
}
```

### Body 類型

| 類型 | Content-Type | 內容格式 |
|------|-------------|---------|
| `json`（預設） | `application/json` | JSON 物件/陣列 |
| `text` | `text/plain` | JSON 字串（`"hello"`） |
| `form` | `application/x-www-form-urlencoded` | JSON 物件（值為字串） |

### 斷言類型

| 類型 | 欄位 | 說明 |
|------|------|------|
| `status` | `equals` | HTTP 狀態碼 |
| `body_contains` | `contains` | body 包含字串 |
| `body_regex` | `pattern` | body 符合正規表達式 |
| `header_equals` | `key`, `value` | header 值（不分大小寫） |
| `json_path` | `path`, `expected` | JSON Pointer（RFC 6901）值 |
| `json_path_count` | `path`, `equals` | JSON Pointer 位置的陣列長度 |
| `duration_under` | `under` | 回應時間低於 N 毫秒 |

### Request 串連（Chaining）

用 `extract` 從 response 取值，collection run 時自動注入後續 request：

```json
{
  "name": "登入",
  "method": "POST",
  "url": "$BASE_URL/auth/login",
  "body": { "type": "json", "content": { "user": "admin", "pass": "secret" } },
  "extract": { "TOKEN": "/accessToken" }
}
```

下一個 request 可用 `$TOKEN`：

```json
{
  "name": "取得個人資料",
  "method": "GET",
  "url": "$BASE_URL/me",
  "headers": { "Authorization": "Bearer $TOKEN" }
}
```

Collection run 時 cookie 自動跨 request 共用。

### 環境變數

在 `.apiw/env/<名稱>.env` 定義變數：

```
BASE_URL=https://api.example.com
TOKEN=my-secret-token
```

在 request spec 中用 `$VAR` 或 `${VAR}` 引用。.env 檔案的值永遠優先於 process 環境變數。

### CLI 指令

```bash
apiw init                  # 建立新專案
apiw init --demo           # 建立專案 + 8 個範例
apiw run <檔案> [flags]     # 執行單一 request
apiw run --all [flags]     # 執行 collection 所有 request
apiw tui                   # 開啟終端機 UI
```

| Flag | 預設值 | 說明 |
|------|-------|------|
| `--env` | `local` | 環境名稱 |
| `--timeout` | `15s` | 請求逾時 |
| `--snapshot` | `false` | 儲存回應快照 |

### Exit Code

| Code | 意義 |
|------|------|
| 0 | 全部通過 |
| 1 | Request spec 格式錯誤 |
| 2 | 傳輸錯誤（網路、逾時） |
| 3 | 斷言失敗 |

### 桌面 GUI

Tauri 原生桌面 app 提供：

- 視覺化 request 編輯器與 response 檢視器
- 一鍵執行 collection
- 環境與 request 切換
- 4 語言：English、繁體中文、簡體中文、日本語
- 3 佈景主題：Light、Dark、The Matrix

---

## 日本語

### API Workbench とは？

軽量でゼロ外部依存の API テストツール：

- **リクエストはリポジトリに保存** — JSON ファイルとして管理、デスクトップアプリ内ではない
- **ターミナルファースト** — CLI と CI で同じファイルを使用
- **アサーション内蔵** — status、header、body、JSON path、正規表現、レスポンス時間を検証
- **リクエストチェイニング** — レスポンスから値を抽出し、後続リクエストに自動注入
- **デスクトップ GUI 付属** — Tauri ネイティブアプリ、4言語対応 + 3テーマ

### インストール

**macOS デスクトップアプリ（Apple Silicon）**

[Releases](https://github.com/MakiDevelop/api-workbench/releases) ページから `API.Workbench_0.1.0_aarch64.dmg` をダウンロード。

**CLI のみ**

```bash
go install github.com/MakiDevelop/api-workbench/cmd/apiw@latest
```

ソースからビルド：

```bash
git clone https://github.com/MakiDevelop/api-workbench.git
cd api-workbench
go build -o bin/apiw ./cmd/apiw
```

### クイックスタート

```bash
# 8つのデモリクエスト付きでプロジェクトを作成
apiw init --demo

# 全デモを実行（httpbin.org、dog.ceo、jsonplaceholder にアクセス）
apiw run --all --env demo

# 単一リクエストを実行
apiw run requests/01-get-ip.json --env demo

# ターミナル UI を起動
apiw tui
```

### プロジェクト構成

```
my-project/
├── .apiw/
│   ├── apiw.json          # プロジェクト設定
│   ├── env/
│   │   ├── local.env      # local 環境変数
│   │   └── demo.env       # demo 環境変数
│   └── snapshots/         # レスポンススナップショット
└── requests/
    ├── 01-get-ip.json
    ├── 02-post-json.json
    └── ...
```

### リクエスト仕様フォーマット

```json
{
  "name": "ユーザー作成",
  "method": "POST",
  "url": "$BASE_URL/users",
  "headers": {
    "Authorization": "Bearer $TOKEN"
  },
  "body": {
    "type": "json",
    "content": { "name": "Maki", "role": "developer" }
  },
  "assertions": [
    { "type": "status", "equals": 201 },
    { "type": "json_path", "path": "/name", "expected": "Maki" },
    { "type": "duration_under", "under": 2000 }
  ],
  "extract": {
    "USER_ID": "/id"
  }
}
```

### Body タイプ

| タイプ | Content-Type | 内容形式 |
|-------|-------------|---------|
| `json`（デフォルト） | `application/json` | JSON オブジェクト/配列 |
| `text` | `text/plain` | JSON 文字列（`"hello"`） |
| `form` | `application/x-www-form-urlencoded` | JSON オブジェクト（値は文字列） |

### アサーションタイプ

| タイプ | フィールド | 説明 |
|-------|-----------|------|
| `status` | `equals` | HTTP ステータスコード一致 |
| `body_contains` | `contains` | レスポンスボディに文字列を含む |
| `body_regex` | `pattern` | レスポンスボディが正規表現にマッチ |
| `header_equals` | `key`, `value` | レスポンスヘッダー一致（大文字小文字区別なし） |
| `json_path` | `path`, `expected` | JSON Pointer（RFC 6901）の値が一致 |
| `json_path_count` | `path`, `equals` | JSON Pointer 位置の配列長が一致 |
| `duration_under` | `under` | レスポンス時間が N ミリ秒未満 |

### リクエストチェイニング

`extract` でレスポンスから値を取得し、コレクション実行時に後続リクエストへ自動注入：

```json
{
  "name": "ログイン",
  "method": "POST",
  "url": "$BASE_URL/auth/login",
  "body": { "type": "json", "content": { "user": "admin", "pass": "secret" } },
  "extract": { "TOKEN": "/accessToken" }
}
```

次のリクエストで `$TOKEN` を使用可能：

```json
{
  "name": "プロフィール取得",
  "method": "GET",
  "url": "$BASE_URL/me",
  "headers": { "Authorization": "Bearer $TOKEN" }
}
```

コレクション実行時、Cookie は全リクエスト間で自動共有されます。

### 環境変数

`.apiw/env/<名前>.env` ファイルで変数を定義：

```
BASE_URL=https://api.example.com
TOKEN=my-secret-token
```

リクエスト仕様内で `$VAR` または `${VAR}` 構文で参照。.env ファイルの値はプロセス環境変数より常に優先されます。

### CLI コマンド

```bash
apiw init                  # 新規プロジェクト作成
apiw init --demo           # デモ付きプロジェクト作成
apiw run <ファイル> [flags]  # 単一リクエスト実行
apiw run --all [flags]     # コレクション全リクエスト実行
apiw tui                   # ターミナル UI 起動
```

| フラグ | デフォルト | 説明 |
|-------|-----------|------|
| `--env` | `local` | 環境名 |
| `--timeout` | `15s` | リクエストタイムアウト |
| `--snapshot` | `false` | レスポンススナップショット保存 |

### 終了コード

| コード | 意味 |
|-------|------|
| 0 | 全て成功 |
| 1 | リクエスト仕様のフォーマットエラー |
| 2 | トランスポートエラー（ネットワーク、タイムアウト） |
| 3 | アサーション失敗 |

### デスクトップ GUI

Tauri ネイティブデスクトップアプリ：

- ビジュアルリクエストエディタとレスポンスビューア
- ワンクリックコレクション実行
- 環境とリクエストの切り替え
- 4言語：English、繁體中文、簡體中文、日本語
- 3テーマ：Light、Dark、The Matrix

---

## Built-in Demo Requests

`apiw init --demo` includes 8 ready-to-run examples:

| # | Scenario | API | Features |
|---|----------|-----|----------|
| 01 | Get My IP | httpbin.org | GET, duration_under |
| 02 | POST JSON Echo | httpbin.org | POST, json_path, header_equals |
| 03 | POST Form Data | httpbin.org | form body, json_path |
| 04 | Random Dog Image | dog.ceo | body_regex, json_path |
| 05 | List Posts | jsonplaceholder | query params, json_path_count |
| 06 | Create Post | jsonplaceholder | POST, extract (chaining) |
| 07 | Verify Chaining | httpbin.org | uses extracted `$NEW_POST_ID` |
| 08 | 418 Teapot | httpbin.org | non-200 status assertion |

## License

MIT
