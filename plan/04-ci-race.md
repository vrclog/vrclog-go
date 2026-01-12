# 計画: CI に go test -race を追加

## 目的

Race Detector を使用したテストを CI に追加し、並行処理のバグを早期に検出する。

## 背景

Watcher でgoroutineを使用しており、race condition のリスクがある。
`-race` フラグによるテストで、データ競合を検出できる。

## 変更範囲

- `Makefile` (test-race ターゲットの確認/追加)
- `.github/workflows/*.yml` (CI設定)

## 実装手順

### 1. Makefile の確認

既存の Makefile に `test-race` ターゲットがあるか確認：

```makefile
test-race:
	CGO_ENABLED=1 go test -race ./...
```

なければ追加する。

### 2. GitHub Actions の更新

`.github/workflows/` 内のCI設定に race テストを追加：

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Run tests
        run: go test ./...

      - name: Run tests with race detector
        run: CGO_ENABLED=1 go test -race ./...
```

### 注意事項

- **CGO_ENABLED=1** が必要（race detector は cgo を使用）
- **対応アーキテクチャ**: linux/amd64, darwin/amd64, darwin/arm64, windows/amd64
- **実行時間**: 通常テストより2-10倍遅くなる

### オプション: 別ジョブとして分離

実行時間を考慮して、race テストを別ジョブにすることも可能：

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go test ./...

  test-race:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: CGO_ENABLED=1 go test -race ./...
```

## 受け入れ基準

- [ ] `make test-race` がローカルで実行できる
- [ ] CI で race detector 付きテストが実行される
- [ ] 現在のコードで race condition が検出されない

## テスト

```bash
# ローカルで確認
make test-race

# または直接実行
CGO_ENABLED=1 go test -race ./...
```

## 注意点/リスク

- race detector はメモリを多く消費する（通常の5-10倍）
- Windows では CGO が必要なため、クロスコンパイル環境では動作しない可能性
- CI の実行時間が増加する（別ジョブにして並列化で緩和可能）

## 参考

- Go Race Detector: https://go.dev/doc/articles/race_detector
- 既存のMakefile: `/Users/gra/Documents/graaaaa/vrclog/vrclog-go/Makefile`
