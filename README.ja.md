# vrclog-go

[![Go Reference](https://pkg.go.dev/badge/github.com/vrclog/vrclog-go.svg)](https://pkg.go.dev/github.com/vrclog/vrclog-go)
[![CI](https://github.com/vrclog/vrclog-go/actions/workflows/ci.yml/badge.svg)](https://github.com/vrclog/vrclog-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vrclog/vrclog-go)](https://goreportcard.com/report/github.com/vrclog/vrclog-go)
[![codecov](https://codecov.io/gh/vrclog/vrclog-go/branch/main/graph/badge.svg)](https://codecov.io/gh/vrclog/vrclog-go)

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

- Go 1.25以上（このプロジェクトの最小バージョン。`iter.Seq2`はGo 1.23で導入されました）
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

## クイックスタート

30秒で始める：

```bash
# 1. インストール
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest

# 2. VRChatイベントを監視（VRChatが起動している必要があります）
vrclog tail
```

ライブラリの使い方や高度な機能については、下記の[ライブラリとしての使用](#ライブラリとしての使用)を参照してください。

## CLIの使い方

### コマンド一覧

```bash
vrclog tail       # VRChatログを監視（リアルタイム）
vrclog parse      # VRChatログを解析（バッチ/オフライン）
vrclog completion # シェル補完スクリプトを生成
vrclog version    # バージョン情報を表示
vrclog --help     # ヘルプを表示
```

### シェル補完

タブ補完を有効にする：

```bash
# Bash
vrclog completion bash > /etc/bash_completion.d/vrclog

# Zsh
vrclog completion zsh > "${fpath[1]}/_vrclog"

# Fish
vrclog completion fish > ~/.config/fish/completions/vrclog.fish

# PowerShell
vrclog completion powershell | Out-String | Invoke-Expression
```

### ストリーミング vs バッチ

| 機能 | `tail` | `parse` |
|------|--------|---------|
| モード | リアルタイム監視 | バッチ処理 |
| ファイル処理 | 最新ファイル + ローテーション | 全マッチファイル |
| 用途 | ライブ監視 | 過去ログ分析 |
| イベント配信 | チャネルベース | イテレータベース |

### グローバルフラグ

| フラグ | 説明 |
|--------|------|
| `--verbose`, `-v` | 詳細なログを有効化 |

### 共通オプション

`tail` と `parse` の両方で使用可能:

| フラグ | 短縮形 | 説明 |
|--------|--------|------|
| `--log-dir` | `-d` | VRChatログディレクトリ（未設定時は自動検出） |
| `--format` | `-f` | 出力形式: `jsonl`（デフォルト）, `pretty` |
| `--include-types` | | 含めるイベントタイプ（カンマ区切り） |
| `--exclude-types` | | 除外するイベントタイプ（カンマ区切り） |
| `--raw` | | 生のログ行を出力に含める |

### tailコマンド

リアルタイムでログを監視:

```bash
# ログディレクトリを自動検出して監視
vrclog tail

# ログディレクトリを指定
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# 人間が読みやすい形式で出力
vrclog tail --format pretty

# プレイヤーイベントのみ表示
vrclog tail --include-types player_join,player_left

# ワールド参加イベントを除外
vrclog tail --exclude-types world_join

# ログファイルの先頭からリプレイ
vrclog tail --replay-last 0

# 直近100行をリプレイ
vrclog tail --replay-last 100

# 指定時刻以降のイベントをリプレイ
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

#### tail固有フラグ

| フラグ | デフォルト | 説明 |
|--------|------------|------|
| `--replay-last` | -1（無効） | 直近N非空行をリプレイ（0 = 先頭から） |
| `--replay-since` | | 指定時刻以降をリプレイ（RFC3339形式） |

注意: `--replay-last` と `--replay-since` は同時に使用できません。

### parseコマンド

過去ログを解析（バッチモード）:

```bash
# 自動検出されたディレクトリの全ログを解析
vrclog parse

# ログディレクトリを指定
vrclog parse --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# 時間範囲でフィルタ（複数日クエリ）
vrclog parse --since "2024-01-15T00:00:00Z" --until "2024-01-16T00:00:00Z"

# イベントタイプでフィルタ
vrclog parse --include-types world_join --format pretty

# 特定のファイルを解析
vrclog parse output_log_2024-01-15.txt output_log_2024-01-16.txt
```

#### parse固有フラグ

| フラグ | デフォルト | 説明 |
|--------|------------|------|
| `--since` | | 指定時刻以降のイベントのみ（RFC3339形式） |
| `--until` | | 指定時刻より前のイベントのみ（RFC3339形式） |
| `--stop-on-error` | false | 最初のエラーで停止（スキップではなく） |
| `[files...]` | | 解析する特定のファイルパス |

### jqとの連携

`tail` と `parse` の両方がJSON Lines形式で出力:

```bash
# 特定のプレイヤーでフィルタ
vrclog tail | jq 'select(.player_name == "FriendName")'

# イベントタイプごとにカウント
vrclog parse | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'

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
| `WithReplayLastN(n)` | 直近N非空行を読み込んでから監視開始 |
| `WithReplaySinceTime(since)` | 指定時刻以降のイベントを読み込み |
| `WithMaxReplayLines(n)` | ReplayLastNの上限（デフォルト: 10000） |
| `WithParser(p)` | カスタムパーサーを使用（デフォルトを置換） |
| `WithParsers(parsers...)` | 複数のパーサーを結合（ChainAllモード） |
| `WithLogger(logger)` | デバッグ用のslog.Loggerを設定 |
| `WithWaitForLogs(wait)` | ディレクトリは存在するがログファイルがない場合に待機 |
| `WithMaxReplayBytes(max)` | リプレイ中に読み込む最大バイト数（デフォルト: 10MB） |
| `WithMaxReplayLineBytes(max)` | リプレイ中の1行あたりの最大バイト数（デフォルト: 512KB） |

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
| `WithParseParser(p)` | カスタムパーサーを使用（デフォルトを置換） |

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
| `WithDirParser(p)` | カスタムパーサーを使用（デフォルトを置換） |

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

## カスタムパーサー

### Parserインターフェース

vrclog-goは非標準ログ形式の処理や追加データ抽出のためのカスタムパーサーをサポートしています。

#### Parserインターフェース定義

```go
type Parser interface {
    ParseLine(ctx context.Context, line string) (ParseResult, error)
}

type ParseResult struct {
    Events  []event.Event
    Matched bool
}
```

#### カスタムパーサーの使用

```go
// Watchで使用
events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithParser(myCustomParser),
)

// ParseFileで使用
for ev, err := range vrclog.ParseFile(ctx, "log.txt",
    vrclog.WithParseParser(myCustomParser),
) {
    // ...
}

// ParseDirで使用
for ev, err := range vrclog.ParseDir(ctx,
    vrclog.WithDirParser(myCustomParser),
) {
    // ...
}
```

#### ParserChain

複数のパーサーを組み合わせて複雑な解析を実現:

```go
chain := &vrclog.ParserChain{
    Mode: vrclog.ChainAll, // ChainFirst, ChainContinueOnError
    Parsers: []vrclog.Parser{
        vrclog.DefaultParser{},  // 組み込みイベント
        customParser,            // カスタムイベント
    },
}

events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithParser(chain),
)
```

| モード | 動作 |
|-------|------|
| `ChainAll` | 全パーサーを実行し、結果を結合 |
| `ChainFirst` | 最初にマッチしたパーサーで停止 |
| `ChainContinueOnError` | エラーの出たパーサーをスキップして継続 |

#### ParserFuncアダプター

関数をパーサーに変換:

```go
myParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
    // パース処理
    return vrclog.ParseResult{Events: events, Matched: true}, nil
})
```

### YAMLパターンファイル (RegexParser)

Goコードを書かずにYAMLパターンファイルでカスタムイベントを定義できます。

#### パターンファイル形式

```yaml
version: 1
patterns:
  - id: poker_hole_cards
    event_type: poker_hole_cards
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
  - id: poker_winner
    event_type: poker_winner
    regex: '\[PotManager\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+)'
```

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `version` | はい | スキーマバージョン（現在は `1`） |
| `id` | はい | パターンの一意識別子 |
| `event_type` | はい | `Event.Type`フィールドの値 |
| `regex` | はい | 正規表現（最大512バイト） |

名前付きキャプチャグループ `(?P<name>...)` は `Event.Data` に抽出されます。

#### RegexParserの使用

```go
import "github.com/vrclog/vrclog-go/pkg/vrclog/pattern"

// ファイルから読み込み
parser, err := pattern.NewRegexParserFromFile("patterns.yaml")
if err != nil {
    log.Fatal(err)
}

// Watchで使用
events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithParser(parser),
)

// デフォルトパーサーと組み合わせ
chain := &vrclog.ParserChain{
    Mode: vrclog.ChainAll,
    Parsers: []vrclog.Parser{
        vrclog.DefaultParser{},
        parser,
    },
}
```

#### 出力例

入力ログ行:
```
2024.01.15 23:59:59 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d
```

出力イベント:
```json
{
  "type": "poker_hole_cards",
  "timestamp": "2024-01-15T23:59:59+09:00",
  "data": {
    "card1": "Jc",
    "card2": "6d"
  }
}
```

#### セキュリティ制限

| 制限 | 値 | 目的 |
|------|-----|------|
| 最大ファイルサイズ | 1 MB | OOM攻撃防止 |
| 最大パターン長 | 512バイト | ReDoS軽減 |
| ファイルタイプ | 通常ファイルのみ | FIFO/デバイスDoS防止 |

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
| `type` | `Type` | `string` | イベントタイプ（`world_join`, `player_join`, `player_left`、またはカスタム） |
| `timestamp` | `Timestamp` | `string` | RFC3339形式のタイムスタンプ |
| `player_name` | `PlayerName` | `string` | プレイヤー表示名（プレイヤーイベント） |
| `player_id` | `PlayerID` | `string` | `usr_xxx`形式のプレイヤーID（player_joinのみ） |
| `world_name` | `WorldName` | `string` | ワールド名（world_joinのみ） |
| `world_id` | `WorldID` | `string` | `wrld_xxx`形式のワールドID（world_joinのみ） |
| `instance_id` | `InstanceID` | `string` | 完全なインスタンスID（world_joinのみ） |
| `data` | `Data` | `map[string]string` | カスタムキー・バリューデータ（カスタムパーサーのみ） |
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
| `ErrReplayLimitExceeded` | リプレイがメモリ制限を超過（maxReplayBytesまたはmaxReplayLineBytes） |
| `ParseError` | 不正なログ行（元のエラーをラップ） |
| `LineTooLongError` | ログ行が最大長を超過（512KB） |
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
│   ├── event/         # イベント型定義
│   └── pattern/       # カスタムパターンマッチング（YAML）
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

## FAQ

### Q: Windows以外で使えますか？
A: いいえ。VRChat PCクライアントはWindowsのみで動作するため、このライブラリもWindows専用です。ただし、ログファイルをコピーすれば他のOSでオフライン解析は可能です。

### Q: ログファイルはどこにありますか？
A: `%LOCALAPPDATA%Low\VRChat\VRChat\output_log_*.txt` にあります。

### Q: リアルタイム監視が動かないのですが？
A: VRChatが起動しているか確認してください。`WithWaitForLogs(true)` オプションを使うとログファイルが作成されるまで待機します。

### Q: iter.Seq2とは何ですか？
A: Go 1.23で導入されたイテレータ型です。メモリ効率の良いストリーミング処理が可能です。詳細は[Go公式ドキュメント](https://go.dev/doc/go1.23)を参照してください。

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
