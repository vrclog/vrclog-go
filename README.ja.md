# vrclog-go

VRChatのログファイルを解析・監視するGoライブラリ＆CLIツール。

[English version](README.md)

## 特徴

- VRChatログファイルを構造化されたイベントに変換
- リアルタイムでログファイルを監視（`tail -f`相当）
- JSON Lines形式で出力（`jq`などで簡単に処理可能）
- 人間が読みやすいpretty形式にも対応
- 過去のログデータのリプレイ機能
- クロスプラットフォーム対応（VRChatが動作するWindows向け設計）

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

### 基本的な監視

```bash
# ログディレクトリを自動検出して監視
vrclog tail

# ログディレクトリを指定
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# 人間が読みやすい形式で出力
vrclog tail --format pretty
```

### イベントのフィルタリング

```bash
# プレイヤー参加イベントのみ表示
vrclog tail --types player_join

# プレイヤー参加・退出イベントを表示
vrclog tail --types player_join,player_left
```

### 過去ログのリプレイ

```bash
# ログファイルの先頭からリプレイ
vrclog tail --replay-last 0

# 指定時刻以降のイベントをリプレイ
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

### jqとの連携

```bash
# 特定のプレイヤーでフィルタ
vrclog tail | jq 'select(.player_name == "FriendName")'

# イベントタイプごとにカウント
vrclog tail | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'
```

## ライブラリとしての使用

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

    // デフォルトオプションで監視開始（ログディレクトリ自動検出）
    events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
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

### 単一行のパース

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("パースエラー: %v", err)
} else if event != nil {
    fmt.Printf("プレイヤー参加: %s\n", event.PlayerName)
}
```

## イベントタイプ

| タイプ | 説明 | フィールド |
|--------|------|-----------|
| `world_join` | ワールドに参加 | WorldName, WorldID, InstanceID |
| `player_join` | プレイヤーがインスタンスに参加 | PlayerName, PlayerID |
| `player_left` | プレイヤーがインスタンスから退出 | PlayerName |

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

## ライセンス

MIT License

## 免責事項

これは非公式ツールであり、VRChat Inc.とは一切関係ありません。
