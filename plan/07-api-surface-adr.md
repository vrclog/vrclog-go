# ADR: API面積の最小化検討

## ステータス

**提案中** (Proposed)

## コンテキスト

vrclog-go は v0.x.x シリーズであり、v1.0.0 に向けて公開APIの面積を最小化したい。
現在エクスポートされている型・定数の中には、ユーザーが直接使用しないものが含まれている可能性がある。

### 現在の公開API一覧

| カテゴリ | エクスポート名 | 用途 |
|---------|---------------|------|
| 型 | `WatchOption` | With*関数の戻り値型 |
| 型 | `ParseOption` | WithParse*関数の戻り値型 |
| 型 | `WatchOp` | WatchError.Opの型 |
| 型 | `ReplayMode` | ReplayConfig.Modeの型 |
| 型 | `ReplayConfig` | WithReplay()の引数 |
| 定数 | `WatchOpFindLatest` 他 | エラー分類用 |
| 定数 | `ReplayNone` 他 | Replay設定用 |

## 検討対象

### 1. WatchOption / ParseOption

**現状**: 公開されている
```go
type WatchOption func(*watchConfig)
type ParseOption func(*parseConfig)
```

**選択肢**:
- A) 公開のまま維持
- B) 非公開化 (`watchOption`, `parseOption`)

**分析**:
- ユーザーは `With*` 関数を呼ぶだけで、型を直接使用することはほぼない
- カスタムオプションを作成させたい場合は公開が必要
- 非公開化しても `With*` 関数は問題なく使用可能

**推奨**: **B) 非公開化**
- カスタムオプションのニーズは現時点で想定していない
- API面積削減のメリットが大きい

---

### 2. WatchOp 定数群

**現状**: 公開されている
```go
type WatchOp string
const (
    WatchOpFindLatest WatchOp = "find_latest"
    WatchOpTail       WatchOp = "tail"
    WatchOpParse      WatchOp = "parse"
    WatchOpReplay     WatchOp = "replay"
    WatchOpRotation   WatchOp = "rotation"
)
```

**選択肢**:
- A) 公開のまま維持
- B) WatchOp を非公開化、WatchError.Op を string に変更
- C) WatchError.Op 自体を削除し、エラーメッセージに含める

**分析**:
- ユーザーが `switch watchErr.Op` で分岐することは想定していない
- エラーメッセージには Op が含まれるため、文字列で十分
- 公開定数はユーザーに「使うべき」という印象を与える

**推奨**: **B) 非公開化 + string化**
- `WatchError.Op` は `string` 型に変更
- 内部では引き続き定数を使用可能

---

### 3. ReplayConfig / ReplayMode

**現状**: 公開されている
```go
type ReplayMode int
const (
    ReplayNone ReplayMode = iota
    ReplayFromStart
    ReplayLastN
    ReplaySinceTime
)
type ReplayConfig struct {
    Mode  ReplayMode
    LastN int
    Since time.Time
}
```

**選択肢**:
- A) 公開のまま維持
- B) ReplayMode のみ非公開化
- C) 両方非公開化

**分析**:
- `WithReplayFromStart()`, `WithReplayLastN()`, `WithReplaySinceTime()` が存在
- 直接 `ReplayConfig` を使うケースは稀
- テストやCLI設定読み込みで使う可能性はある

**推奨**: **A) 公開のまま維持**
- 便利関数があっても、高度なユースケースで必要になる可能性
- 破壊的変更のリスクを避ける

---

## 決定

v0.x.x の間に以下を実施：

1. **WatchOption / ParseOption**: 非公開化を検討（影響調査後に決定）
2. **WatchOp**: 非公開化 + WatchError.Op を string 化
3. **ReplayConfig / ReplayMode**: 現状維持

## 影響調査手順

実装前に以下を確認：

```bash
# リポジトリ内での使用箇所
grep -r "WatchOption" --include="*.go" .
grep -r "ParseOption" --include="*.go" .
grep -r "WatchOp" --include="*.go" .

# 外部での使用（GitHub検索）
# "github.com/vrclog/vrclog-go" WatchOption
```

## 移行ガイド

### WatchOption/ParseOption の非公開化

**Before**:
```go
var opts []vrclog.WatchOption
opts = append(opts, vrclog.WithLogDir("/path"))
```

**After** (変更なし):
```go
// 型を明示しなくても動作する
opts := []any{vrclog.WithLogDir("/path")}
// または
watcher, _ := vrclog.NewWatcherWithOptions(vrclog.WithLogDir("/path"))
```

### WatchOp の非公開化

**Before**:
```go
if watchErr.Op == vrclog.WatchOpTail {
    // ...
}
```

**After**:
```go
if watchErr.Op == "tail" {
    // ...
}
// または
if strings.Contains(watchErr.Error(), "tail") {
    // ...
}
```

## 結果

（実装後に記載）

## 参考

- Codex MCPとの相談結果
- Go API Guidelines: https://go.dev/wiki/CodeReviewComments
- Effective Go: https://go.dev/doc/effective_go
