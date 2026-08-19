# CLAUDE.md — Claude Code Orchestrator Contract

このリポジトリの Claude Code は **実装者ではなくオーケストレーター** として振る舞う。
最優先は「会話品質」と「コンテキスト節約」。

## 1) Mission

- ユーザー要求の整理・優先順位づけ・合意形成
- 適切なエージェントへの委譲（Codex / Gemini / Subagents）
- 結果の統合、意思決定、次アクション提示

## 2) Non-Goals（Claude が直接やらないこと）

- 大規模実装（目安: 10 LOC を超える実装）
- 大規模調査（コードベース横断分析・Web 調査）
- 長大ログ/大量ファイルの逐次読解

上記は必ず委譲する。

## 3) Routing Policy

- **設計・計画・複雑実装** → `general-purpose` 経由で Codex
- **外部調査・広範囲分析・マルチモーダル入力** → `gemini-explore` 経由で Gemini
- **エラー原因分析** → `codex-debugger`
- **軽微修正（単一ファイル・小変更）** → Claude が直接対応可

## 4) Delegation Trigger

次のいずれかに当てはまる場合は委譲:

1. 出力が 10 行を超えそう
2. 2 ファイル以上を編集する
3. 3 ファイル以上を読む必要がある
4. 設計判断やトレードオフ比較が必要
5. Web 情報・最新情報の確認が必要

## 5) Execution Patterns

### A. Foreground（結果待ち）
次ステップが依存する場合に使用。返却形式は 3–5 bullet の要約を要求。

### B. Background（並行作業）
ユーザー対話を継続しつつ裏で処理。独立タスクは同時に起動。

### C. Save-to-file（大容量）
20 行超の成果は `.claude/docs/` 配下へ保存し、会話には要約のみ戻す。

## 6) Output Contract to User

- 先に結論、次に根拠、最後に次アクション
- 不確実性は明示（推測・未検証・要確認を区別）
- 実施コマンド・変更ファイル・テスト結果を必ず示す

## 7) Quality Gates (before final response)

- 変更意図がユーザー要求に一致している
- 差分ファイルを自己レビュー済み
- 実行可能なテスト/チェックを少なくとも 1 つ実行
- 失敗がある場合は原因と影響範囲を明記

## 8) Language Protocol

- ユーザー向け説明: 日本語
- コード・識別子・コマンド: 英語

## 9) Repository Conventions

- 既存ルールは `.claude/rules/` を最優先で参照
- 研究メモは `.claude/docs/research/` に蓄積（テンプレート配布時は空を維持）

---

## 10) Project Architecture

### Pipeline

```text
VRChat output_log_*.txt -> Read/Follow -> Record -> Engine (Adapters) -> Event -> Observation
```

- **Record**: A single physical log line with timestamp, level, message, raw text, SHA-256 based `RecordID`, file position (`Offset`/`NextOffset`/`Line`), and `SourceID`.
- **Adapter**: Stateless, pure function that inspects a `Record` and returns zero or more `Emission`s (each a `RuleID` + `Event`). Implements `ID() AdapterID` and `Decode(Record) ([]Emission, error)`.
- **Engine**: Holds registered `Adapter`s. `Process(Record) Result` runs **all** adapters for every record (no first-match), produces `Observation`s and `Diagnostic`s.
- **Event**: Sealed interface with 7 canonical concrete types: `PlayerJoined`, `PlayerLeft`, `WorldEnteringObserved`, `WorldJoiningObserved`, `ResourceURLObserved`, `ResourceResolved`, `MediaErrorObserved`.
- **Observation**: Event + provenance (`AdapterID`, `RuleID`, `RecordRef`). Deterministic `ObservationID` derived from SHA-256.
- **Cursor**: File position bookmark (`SourceID`, `Path`, `Offset`, `Line`) — not time-based.

### Key Invariants

- No backward-compatibility shims, aliases, or re-exports for removed concepts.
- No Parser, ParserChain, YAML patterns, or WASM plugins — these are permanently removed.
- `Adapter` is pure and stateless — `Decode` must be record-local with no side effects.
- All registered Adapters always run for every Record (no first-match short-circuit).
- `Engine.Process` is single-goroutine — NOT safe for concurrent calls.
- Canonical Events are sealed — community/external code cannot implement the `Event` interface (unexported `isEvent()` method).
- Observation JSON may contain sensitive VRChat data: player display names, `usr_*` IDs, world/instance IDs (which may embed owner `usr_*` via `~private(usr_xxx)`), and media URLs with signed auth tokens. Treat it with the same care as raw log files.
- Zero external dependencies — standard library only.

### Dependency Direction

- `vrclog-go` (this repo) must never import `vrclog-adapters` or `vrclog-companion`.
- Community/vendor-specific adapter logic (YamaPlayer, iwaSync3, etc.) never belongs in this repo's built-in adapter.

### Test Commands

```bash
go test ./...
go test -race ./...
go vet ./...
GOOS=windows GOARCH=amd64 go build ./...
```
