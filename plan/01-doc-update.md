# 計画: doc.go の古いAPI参照を更新

## 目的

`pkg/vrclog/doc.go` のパッケージドキュメントで使用されている古いAPI例を、現在のFunctional Options APIに更新する。

## 背景

現在のdoc.goでは以下のような古いAPI参照がある：
```go
events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
```

これは存在しないAPIであり、新規ユーザーの混乱を招く。

## 変更範囲

- `pkg/vrclog/doc.go` のみ

## 実装手順

1. `pkg/vrclog/doc.go` を開く
2. 古いAPI例を以下のように更新：

**Before:**
```go
events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
```

**After:**
```go
events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
)
```

3. 前後の説明文を確認し、API変更に伴う齟齬がないか確認
4. `go vet ./pkg/vrclog` で構文エラーがないことを確認

## 受け入れ基準

- [ ] doc.go内のすべてのAPI例が現在のAPIを使用している
- [ ] `go vet` がエラーなく通る
- [ ] `go doc github.com/vrclog/vrclog-go/pkg/vrclog` で正しくドキュメントが表示される

## 注意点/リスク

- ドキュメントのみの変更のため、機能への影響なし
- 既存のexample_test.goと整合性を確認すること

## 参考

- 現在のAPI: `pkg/vrclog/watcher.go` の `WatchWithOptions()`, `NewWatcherWithOptions()`
- example_test.go: 正しいAPI使用例がある
