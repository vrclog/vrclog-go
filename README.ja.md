# vrclog-go

[![Go Reference](https://pkg.go.dev/badge/github.com/vrclog/vrclog-go.svg)](https://pkg.go.dev/github.com/vrclog/vrclog-go)

VRChatのログファイルを解析・監視するGoライブラリ＆CLIツール。

[English version](README.md)

## API安定性

> **注意**: このライブラリはpre-1.0（`v0.x.x`）です。マイナーバージョン間でAPIが予告なく変更される可能性があります。安定性が必要な場合は特定のバージョンに固定してください。

## 特徴

- VRChatログファイルを構造化されたイベントに変換
- リアルタイムでログファイルを監視（`tail -f`相当）
- JSON Lines形式で出力（`jq`などで簡単に処理可能）
- 人間が読みやすいpretty形式にも対応
- 過去のログデータのリプレイ機能
- VRChatが動作するWindows向け設計

## 動作要件

- Go 1.23以上（`iter.Seq2`イテレータサポートに必要）
- Windows（実際のVRChatログ監視用）

## インストール

```bash
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest
```

または、ソースからビルド:

```bash
git clone https://github.com/vrclog/vrclog-go.git
cd vrclog-go
go build -o vrclog ./cmd/vrclog/
```

## CLIの使い方

### コマンド一覧

```bash
vrclog tail      # VRChatログを監視
vrclog version   # バージョン情報を表示
vrclog --help    # ヘルプを表示
```

### グローバルフラグ

| フラグ | 説明 |
|--------|------|
| `--verbose`, `-v` | 詳細なログを有効化 |

### 基本的な監視

```bash
# ログディレクトリを自動検出して監視
vrclog tail

# ログディレクトリを指定
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# 人間が読みやすい形式で出力
vrclog tail --format pretty

# 生のログ行も出力に含める
vrclog tail --raw
```

### イベントのフィルタリング

```bash
# プレイヤー参加イベントのみ表示
vrclog tail --types player_join

# ワールド参加イベントのみ表示
vrclog tail --types world_join

# プレイヤー参加・退出イベントを表示
vrclog tail --types player_join,player_left

# 短縮形
vrclog tail -t player_join,player_left
```

### 過去ログのリプレイ

```bash
# ログファイルの先頭からリプレイ
vrclog tail --replay-last 0

# 直近100行をリプレイ
vrclog tail --replay-last 100

# 指定時刻以降のイベントをリプレイ
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

注意: `--replay-last` と `--replay-since` は同時に使用できません。

### tailコマンドのフラグ一覧

| フラグ | 短縮形 | デフォルト | 説明 |
|--------|--------|------------|------|
| `--log-dir` | `-d` | 自動検出 | VRChatログディレクトリ |
| `--format` | `-f` | `jsonl` | 出力形式: `jsonl`, `pretty` |
| `--types` | `-t` | 全て | 表示するイベントタイプ（カンマ区切り） |
| `--raw` | | false | 生のログ行を出力に含める |
| `--replay-last` | | -1（無効） | 直近N行をリプレイ（0 = 先頭から） |
| `--replay-since` | | | 指定時刻以降をリプレイ（RFC3339形式） |

### jqとの連携

```bash
# 特定のプレイヤーでフィルタ
vrclog tail | jq 'select(.player_name == "FriendName")'

# イベントタイプごとにカウント
vrclog tail | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'

# 参加イベントからプレイヤー名を抽出
vrclog tail | jq 'select(.type == "player_join") | .player_name'
```

## ライブラリとしての使用

### クイックスタート（リアルタイム監視）

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Functional Optionsで監視開始（推奨）
    events, errs, err := vrclog.WatchWithOptions(ctx,
        vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
        vrclog.WithReplayLastN(100),
    )
    if err != nil {
        log.Fatal(err)
    }

    for {
        select {
        case event, ok := <-events:
            if !ok {
                return
            }
            switch event.Type {
            case vrclog.EventPlayerJoin:
                fmt.Printf("%sが参加しました\n", event.PlayerName)
            case vrclog.EventPlayerLeft:
                fmt.Printf("%sが退出しました\n", event.PlayerName)
            case vrclog.EventWorldJoin:
                fmt.Printf("ワールドに参加: %s\n", event.WorldName)
            }
        case err, ok := <-errs:
            if !ok {
                return
            }
            log.Printf("エラー: %v", err)
        }
    }
}
```

### Watch オプション（Functional Options パターン）

| オプション | 説明 |
|------------|------|
| `WithLogDir(dir)` | VRChatログディレクトリを設定（未設定時は自動検出） |
| `WithPollInterval(d)` | ログローテーション確認間隔（デフォルト: 2秒） |
| `WithIncludeRawLine(bool)` | イベントに生のログ行を含める |
| `WithIncludeTypes(types...)` | 指定したイベントタイプのみを取得 |
| `WithExcludeTypes(types...)` | 指定したイベントタイプを除外 |
| `WithReplayFromStart()` | ファイル先頭から読み込み |
| `WithReplayLastN(n)` | 直近N行を読み込んでから監視開始 |
| `WithReplaySinceTime(t)` | 指定時刻以降のイベントを読み込み |
| `WithMaxReplayLines(n)` | ReplayLastNの上限（デフォルト: 10000） |
| `WithLogger(logger)` | デバッグ用のslog.Loggerを設定 |

### Watcherを使った高度な使用法

ライフサイクルをより細かく制御する場合:

```go
// Functional Optionsを使ってWatcherを作成
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithLogDir("/custom/path"),
    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin),
    vrclog.WithReplayLastN(100),
)
if err != nil {
    log.Fatal(err)
}
defer watcher.Close()

// 監視開始
events, errs, err := watcher.Watch(ctx)
// ... イベントを処理
```

### オフライン解析（iter.Seq2）

Watcherを起動せずにログファイルを解析。Go 1.23+のイテレータを使用してメモリ効率の良いストリーミング処理が可能:

```go
// 単一ファイルを解析
for ev, err := range vrclog.ParseFile(ctx, "output_log.txt",
    vrclog.WithParseIncludeTypes(vrclog.EventPlayerJoin),
) {
    if err != nil {
        log.Printf("エラー: %v", err)
        break
    }
    fmt.Printf("プレイヤー参加: %s\n", ev.PlayerName)
}

// 全イベントをスライスに収集
events, err := vrclog.ParseFileAll(ctx, "output_log.txt")

// ディレクトリ内の全ログファイルを解析（時系列順）
for ev, err := range vrclog.ParseDir(ctx,
    vrclog.WithDirLogDir("/path/to/logs"),
    vrclog.WithDirIncludeTypes(vrclog.EventWorldJoin),
) {
    if err != nil {
        break
    }
    fmt.Printf("ワールド: %s\n", ev.WorldName)
}
```

### Parse オプション

| オプション | 説明 |
|------------|------|
| `WithParseIncludeTypes(types...)` | 指定したイベントタイプのみを取得 |
| `WithParseExcludeTypes(types...)` | 指定したイベントタイプを除外 |
| `WithParseTimeRange(since, until)` | 時間範囲でフィルタ |
| `WithParseSince(t)` | 指定時刻以降のイベントを取得 |
| `WithParseUntil(t)` | 指定時刻より前のイベントを取得 |
| `WithParseIncludeRawLine(bool)` | 生のログ行を含める |
| `WithParseStopOnError(bool)` | 最初のエラーで停止（デフォルト: スキップ） |

### ParseDir オプション

| オプション | 説明 |
|------------|------|
| `WithDirLogDir(dir)` | ログディレクトリ（未設定時は自動検出） |
| `WithDirPaths(paths...)` | 解析するファイルパスを明示的に指定 |
| `WithDirIncludeTypes(types...)` | 指定したイベントタイプのみを取得 |
| `WithDirExcludeTypes(types...)` | 指定したイベントタイプを除外 |
| `WithDirTimeRange(since, until)` | 時間範囲でフィルタ |
| `WithDirIncludeRawLine(bool)` | 生のログ行を含める |
| `WithDirStopOnError(bool)` | 最初のエラーで停止 |

### 単一行のパース

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("パースエラー: %v", err)
} else if event != nil {
    fmt.Printf("プレイヤー参加: %s\n", event.PlayerName)
}
// event == nil && err == nil の場合、認識されないイベント行
```

## イベントタイプ

| タイプ | 説明 | フィールド |
|--------|------|-----------|
| `world_join` | ワールドに参加 | WorldName, WorldID, InstanceID |
| `player_join` | プレイヤーがインスタンスに参加 | PlayerName, PlayerID |
| `player_left` | プレイヤーがインスタンスから退出 | PlayerName |

### Event JSON スキーマ

すべてのイベントに共通のフィールド:

| JSONフィールド | Goフィールド | 型 | 説明 |
|----------------|--------------|-----|------|
| `type` | `Type` | `string` | イベントタイプ（`world_join`, `player_join`, `player_left`） |
| `timestamp` | `Timestamp` | `string` | RFC3339形式のタイムスタンプ |
| `player_name` | `PlayerName` | `string` | プレイヤー表示名（プレイヤーイベント） |
| `player_id` | `PlayerID` | `string` | `usr_xxx`形式のプレイヤーID（player_joinのみ） |
| `world_name` | `WorldName` | `string` | ワールド名（world_joinのみ） |
| `world_id` | `WorldID` | `string` | `wrld_xxx`形式のワールドID（world_joinのみ） |
| `instance_id` | `InstanceID` | `string` | 完全なインスタンスID（world_joinのみ） |
| `raw_line` | `RawLine` | `string` | 元のログ行（IncludeRawLine有効時） |

## 実行時の動作

### チャネルのライフサイクル

- `events`と`errs`の両チャネルは以下の場合に閉じられます:
  - コンテキストがキャンセルされた時（`ctx.Done()`）
  - 致命的なエラーが発生した時（例: ログディレクトリが削除された）
  - `watcher.Close()`が呼ばれた時
- チャネルから受信する際は必ず`ok`値を確認してください

### ログローテーション

- Watcherは`PollInterval`（デフォルト: 2秒）で新しいログファイルをポーリングします
- VRChatが新しいログファイルを作成すると、自動的に切り替えます
- 新しいログファイルは先頭から読み込まれます
- 古いログファイルには戻りません

### エラー処理

エラーはエラーチャネルに送信され、`errors.Is()`で検査できます:

```go
import "errors"

case err := <-errs:
    if errors.Is(err, vrclog.ErrLogDirNotFound) {
        // ログディレクトリが削除された
    }
    var parseErr *vrclog.ParseError
    if errors.As(err, &parseErr) {
        // 不正なログ行
        fmt.Printf("不正な行: %s\n", parseErr.Line)
    }
```

| エラー | 説明 |
|--------|------|
| `ErrLogDirNotFound` | ログディレクトリが見つからない |
| `ErrNoLogFiles` | ディレクトリにログファイルがない |
| `ErrWatcherClosed` | Close後にWatchが呼ばれた |
| `ErrAlreadyWatching` | Watchが二重に呼ばれた |
| `ParseError` | 不正なログ行（元のエラーをラップ） |
| `WatchError` | Watch操作エラー（操作タイプを含む） |

## 出力形式

### JSON Lines（デフォルト）

```json
{"type":"player_join","timestamp":"2024-01-15T23:59:59+09:00","player_name":"TestUser"}
{"type":"player_left","timestamp":"2024-01-16T00:00:05+09:00","player_name":"TestUser"}
```

### Pretty

```
[23:59:59] + TestUser joined
[00:00:05] - TestUser left
[00:01:00] > Joined world: Test World
```

## 環境変数

| 変数 | 説明 |
|------|------|
| `VRCLOG_LOGDIR` | デフォルトのログディレクトリを上書き |

## プロジェクト構成

```
vrclog-go/
├── cmd/vrclog/        # CLIアプリケーション
├── pkg/vrclog/        # 公開API
│   └── event/         # イベント型定義
└── internal/          # 内部パッケージ
    ├── parser/        # ログ行パーサー
    ├── tailer/        # ファイルテーリング
    └── logfinder/     # ログディレクトリ検出
```

## テスト

```bash
# 全テスト実行
go test ./...

# 詳細出力
go test -v ./...

# レースディテクター付き
go test -race ./...

# カバレッジ付き
go test -cover ./...
```

## コントリビューション

1. リポジトリをフォーク
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing-feature`)
3. コードをフォーマット (`go fmt ./...`)
4. テストを実行 (`go test ./...`)
5. 変更をコミット
6. ブランチをプッシュ
7. プルリクエストを作成

## ライセンス

MIT License

## 免責事項

これは非公式ツールであり、VRChat Inc.とは一切関係ありません。
