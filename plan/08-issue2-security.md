# Issue #2: セキュリティ考慮事項

## 概要

カスタムログパターン機能で考慮すべきセキュリティリスクと対策。

---

## リスク一覧

| リスク | 深刻度 | 対策 |
|-------|--------|------|
| ReDoS攻撃 | 高 | 正規表現タイムアウト（5ms） |
| Wasmサンドボックス脱出 | 高 | Host Function制限 |
| TOCTOU攻撃 | 中 | 原子的ファイル操作 |
| パストラバーサル | 中 | パス検証 |
| 巨大ファイルDoS | 中 | サイズ制限 |
| 不正UTF-8インジェクション | 低 | UTF-8検証 |
| メモリ枯渇 | 中 | Wasmメモリ制限 |

---

## ReDoS攻撃対策

### 脅威

正規表現の複雑なパターン（例: `(a+)+`）に悪意のある入力を与えると、
指数関数的な計算時間がかかる（ReDoS: Regular Expression Denial of Service）。

### 対策

```go
func (p *WasmParser) regexMatch(ctx context.Context, mod api.Module,
    strPtr, strLen, rePtr, reLen uint32) uint32 {

    // 1. パターン長制限（512バイト）
    if reLen > 512 {
        return 0
    }

    // 2. タイムアウト付きマッチ（5ms）
    ctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
    defer cancel()

    re, err := p.regexCache.Get(pattern)
    if err != nil {
        return 0
    }

    resultCh := make(chan bool, 1)
    go func() {
        resultCh <- re.MatchString(str)
    }()

    select {
    case result := <-resultCh:
        if result {
            return 1
        }
        return 0
    case <-ctx.Done():
        // タイムアウト: 0を返す（マッチしなかったとして扱う）
        return 0
    }
}
```

### 将来的な改善

- `regexp2`ライブラリへの移行（RE2互換、タイムアウト内蔵）
- wazeroのfuel機能を使った命令数制限

---

## Wasmサンドボックス

### Wasmのセキュリティ特性

Wasmはデフォルトで以下の特性を持つ：

1. **メモリ安全**: 境界チェックによるバッファオーバーフロー防止
2. **Capability-based**: 明示的に許可されたリソースのみアクセス可能
3. **ホストリソース隔離**: ファイルシステム、ネットワーク等へのアクセス不可

### Host Function設計原則

1. **最小権限**: 必要最小限のHost Functionのみ提供
2. **副作用の制限**: 読み取り専用または限定的な副作用のみ
3. **入力検証**: 全てのポインタとサイズを検証
4. **リソース制限**: タイムアウト、メモリ上限で暴走防止

### log関数の制限

```go
func (p *WasmParser) hostLog(ctx context.Context, mod api.Module, level, ptr, msgLen uint32) {
    // 1. レート制限（10回/秒）
    if !p.logLimiter.Allow() {
        return
    }

    // 2. サイズ制限（256バイト）
    if msgLen > 256 {
        msgLen = 256
    }

    // 3. UTF-8サニタイズ
    msg := strings.ToValidUTF8(string(msgBytes), "\uFFFD")

    // ログ出力（制限付き）
    slog.Log(ctx, levelToSlogLevel(level), "[PLUGIN] "+msg)
}
```

---

## TOCTOU攻撃対策（Phase 3）

### 脅威

Time-of-Check to Time-of-Use攻撃。
ファイルチェック後、使用前にファイルが差し替えられる。

### 対策

```go
func downloadPlugin(ctx context.Context, url, expectedChecksum string) (string, error) {
    // 1. シンボリックリンク解決
    cacheDir, err := filepath.EvalSymlinks(getCacheDir())
    if err != nil {
        return "", err
    }

    // 2. 一時ファイルにダウンロード
    tmpFile, err := os.CreateTemp(cacheDir, "plugin-*.tmp")
    if err != nil {
        return "", err
    }
    tmpPath := tmpFile.Name()
    defer os.Remove(tmpPath) // 失敗時はクリーンアップ

    // 3. ダウンロード
    if err := downloadToFile(ctx, url, tmpFile); err != nil {
        return "", err
    }
    tmpFile.Close()

    // 4. checksum検証
    actualChecksum, err := computeChecksum(tmpPath)
    if err != nil {
        return "", err
    }
    if actualChecksum != expectedChecksum {
        return "", fmt.Errorf("checksum mismatch: got %s, expected %s", actualChecksum, expectedChecksum)
    }

    // 5. 原子的リネーム
    finalPath := filepath.Join(cacheDir, actualChecksum+".wasm")
    if err := os.Rename(tmpPath, finalPath); err != nil {
        return "", err
    }

    return finalPath, nil
}
```

---

## パストラバーサル対策

### 脅威

`../`等を使って許可ディレクトリ外にアクセス。

### 対策

```go
func validatePath(basePath, targetPath string) error {
    // 絶対パスに正規化
    absBase, err := filepath.Abs(basePath)
    if err != nil {
        return err
    }
    absTarget, err := filepath.Abs(targetPath)
    if err != nil {
        return err
    }

    // シンボリックリンク解決
    absBase, err = filepath.EvalSymlinks(absBase)
    if err != nil {
        return err
    }
    absTarget, err = filepath.EvalSymlinks(absTarget)
    if err != nil {
        return err
    }

    // baseの配下であることを確認
    if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) {
        return fmt.Errorf("path traversal detected: %s is not under %s", absTarget, absBase)
    }

    return nil
}
```

---

## ファイルサイズ制限

| ファイル種別 | 上限 | 理由 |
|-------------|------|------|
| YAMLパターンファイル | 1MB | テキストファイルとして十分 |
| Wasmファイル | 10MB | TinyGoビルドでも通常1MB未満 |
| ダウンロード | 50MB | ネットワーク帯域保護 |

```go
const (
    MaxPatternFileSize   = 1 * 1024 * 1024  // 1MB
    MaxWasmFileSize      = 10 * 1024 * 1024 // 10MB
    MaxDownloadSize      = 50 * 1024 * 1024 // 50MB
)

func Load(path string) (*PatternFile, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }
    if info.Size() > MaxPatternFileSize {
        return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), MaxPatternFileSize)
    }
    // ...
}
```

---

## UTF-8検証

### 脅威

不正なUTF-8シーケンスによるログインジェクションや解析エラー。

### 対策

```go
// 全ての文字列入力でUTF-8を検証・サニタイズ
func sanitizeString(s string) string {
    return strings.ToValidUTF8(s, "\uFFFD")
}

// Host Functionで使用
str := sanitizeString(string(strBytes))
pattern := sanitizeString(string(reBytes))
```

---

## Wasmメモリ制限

### デフォルト設定

- メモリ: 4MiB（64 pages）
- TinyGoのデフォルトヒープサイズに合わせる

### 設定可能項目

```go
// wazero.ModuleConfig でメモリ制限可能
config := wazero.NewModuleConfig().
    WithMemoryLimitPages(64) // 64 pages = 4MiB
```

---

## 信頼モデル（Phase 3）

### 信頼レベル

| レベル | 説明 | 例 |
|-------|------|---|
| ローカルファイル | 信頼 | `./vrpoker.wasm` |
| 信頼リスト記載URL | 信頼 | trust.jsonに記載 |
| 未知のURL（TTY） | 確認 | ユーザーに確認プロンプト |
| 未知のURL（非TTY） | 拒否 | `--checksum`必須 |

### 信頼リスト形式

```json
// ~/.config/vrclog/trust.json
{
  "version": 1,
  "trusted": [
    {
      "pattern": "https://github.com/vrclog/",
      "type": "prefix",
      "added_at": "2024-01-01T00:00:00Z"
    },
    {
      "pattern": "https://github.com/user/repo/releases/download/v1/plugin.wasm",
      "type": "exact",
      "added_at": "2024-01-02T00:00:00Z"
    }
  ]
}
```

### ファイル権限

```
~/.cache/vrclog/plugins/
├── <sha256>.wasm       # 0600
└── manifest.json       # 0600

~/.config/vrclog/
└── trust.json          # 0600

ディレクトリ: 0700
```

---

## セキュリティテスト

### ReDoSテスト

```go
func TestReDoS_Timeout(t *testing.T) {
    cache := NewRegexCache(100, 5*time.Millisecond)

    pattern := `(a+)+$`
    input := strings.Repeat("a", 30) + "!"

    re, _ := cache.Get(pattern)

    start := time.Now()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
    defer cancel()

    resultCh := make(chan bool, 1)
    go func() {
        resultCh <- re.MatchString(input)
    }()

    select {
    case <-resultCh:
    case <-ctx.Done():
    }

    elapsed := time.Since(start)
    assert.Less(t, elapsed, 20*time.Millisecond)
}
```

### レート制限テスト

```go
func TestLog_RateLimit(t *testing.T) {
    limiter := rate.NewLimiter(10, 10)

    allowed := 0
    for i := 0; i < 20; i++ {
        if limiter.Allow() {
            allowed++
        }
    }

    assert.Equal(t, 10, allowed)
}
```

### パストラバーサルテスト

```go
func TestValidatePath_Traversal(t *testing.T) {
    base := "/tmp/plugins"
    tests := []struct {
        target string
        valid  bool
    }{
        {"/tmp/plugins/test.wasm", true},
        {"/tmp/plugins/../secrets", false},
        {"/etc/passwd", false},
    }

    for _, tt := range tests {
        err := validatePath(base, tt.target)
        if tt.valid {
            assert.NoError(t, err)
        } else {
            assert.Error(t, err)
        }
    }
}
```

---

## セキュリティチェックリスト

### Phase 1（YAML）

- [ ] ファイルサイズ制限（1MB）
- [ ] 正規表現構文エラー検出
- [ ] パターンID重複検出

### Phase 2（Wasm）

- [ ] ファイルサイズ制限（10MB）
- [ ] ABI版検証
- [ ] 正規表現タイムアウト（5ms）
- [ ] 正規表現キャッシュ
- [ ] logレート制限（10回/秒）
- [ ] logサイズ制限（256バイト）
- [ ] UTF-8サニタイズ
- [ ] parse_lineタイムアウト（50ms）
- [ ] panicリカバリ

### Phase 3（リモートURL）

- [ ] HTTPS必須
- [ ] checksum検証（sha256）
- [ ] TOCTOU対策
- [ ] 信頼リスト
- [ ] 非TTY時checksum必須
- [ ] ファイル権限（0600/0700）

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [ABI仕様](./08-issue2-abi-spec.md)
- [テスト戦略](./08-issue2-testing.md)
