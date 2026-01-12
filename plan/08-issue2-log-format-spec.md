# VRChat ログフォーマット仕様

## 概要

VRChatのログフォーマットは**公式に文書化されていない**。
本ドキュメントはコミュニティの調査結果に基づく。フォーマットは予告なく変更される可能性がある。

---

## ログファイル基本情報

| 項目 | 内容 |
|------|------|
| 保存場所 | `C:\Users\%Username%\AppData\LocalLow\VRChat\VRChat` |
| ファイル名 | `output_log_yyyy-MM-dd_HH-mm-ss.txt` |
| 保存期間 | 24時間（次回起動時に古いログは削除） |
| タイムスタンプ形式 | `YYYY.MM.DD HH:MM:SS` (Go: `2006.01.02 15:04:05`) |

---

## ログ行の基本構造

```
<タイムスタンプ> <ログレベル> - <メッセージ>
```

例:
```
2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser
2024.01.15 23:59:59 Debug      -  [Seat]: Draw Local Hole Cards: Jc, 6d
2024.01.15 23:59:59 Error      -  [UdonBehaviour] An exception occurred...
```

### ログレベル

| レベル | 用途 |
|--------|------|
| `Log` | 一般的なログ |
| `Debug` | デバッグ情報（`--enable-udon-debug-logging`で増加） |
| `Warning` | 警告 |
| `Error` | エラー |

---

## 主要なログカテゴリ（タグ）

| カテゴリ | 用途 | 例 |
|---------|------|-----|
| `[Behaviour]` | プレイヤー/ワールドイベント | OnPlayerJoined, Entering Room |
| `[UdonBehaviour]` | Udonスクリプトのエラー・例外 | exception occurred |
| `[Player]` | PlayerAPI初期化 | Initialized PlayerAPI |
| `[Network]` | ネットワーク関連 | 接続、同期イベント |
| `[Video Playback]` | 動画プレイヤー | URL解決、エラー |
| `[Avatar]` | アバター読み込み・変更 | OnAvatarChanged |

---

## vrclog-go対応イベント（標準ログ）

### world_join

**パターン1: Entering Room**
```
2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: World Name
```
- 正規表現: `\[Behaviour\] Entering Room: (.+)`
- 抽出: ワールド名

**パターン2: Joining wrld_**
```
2024.01.15 23:59:59 Log        -  [Behaviour] Joining wrld_4e663a21-e44b-453e-8465-2c388683a1eb:12345~private(usr_xxx)~region(use)
```
- 正規表現: `\[Behaviour\] Joining (wrld_[0-9a-f-]+)(:\d+)?(~.*)?`
- 抽出: World ID, Instance ID, モディファイア

### player_join

**旧形式**
```
2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser
```
- 正規表現: `\[Behaviour\] OnPlayerJoined (.+)`

**新形式（2025〜）**
```
2025.05.01 00:43:48 Debug      -  [Behaviour] OnPlayerJoined CMajor7 (usr_4cb5a109-2aa6-4a0d-933b-244388260586)
```
- 正規表現: `\[Behaviour\] OnPlayerJoined (.+?) \(usr_([0-9a-f-]+)\)`
- 抽出: 表示名, User ID

**代替パターン（2018〜）**
```
2024.01.15 23:59:59 Log        -  [Player] Initialized PlayerAPI "TestUser" is remote
```
- 正規表現: `\[Player\] Initialized PlayerAPI "(.+)" is remote`

### player_left

```
2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser
```
- 正規表現: `\[Behaviour\] OnPlayerLeft (.+)`

---

## World ID形式

```
wrld_<UUID>:<InstanceID>~<Modifier1>~<Modifier2>...
```

### UUID形式
```
wrld_4e663a21-e44b-453e-8465-2c388683a1eb
```
- 正規表現: `wrld_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

### インスタンス付き
```
wrld_4e663a21-e44b-453e-8465-2c388683a1eb:12345~private(usr_xxx)~canRequestInvite~region(use)
```

### モディファイア

| モディファイア | 意味 |
|---------------|------|
| `~private(usr_xxx)` | プライベートインスタンス（所有者ID） |
| `~hidden(usr_xxx)` | 非公開インスタンス |
| `~friends(usr_xxx)` | フレンドオンリー |
| `~group(grp_xxx)` | グループインスタンス |
| `~canRequestInvite` | 招待リクエスト可能 |
| `~region(xxx)` | リージョン（use, usw, eu, jp等） |

---

## Udon/ワールド独自ログ

ワールド開発者が`Debug.Log()`で自由に出力可能。

### UdonSharp例
```csharp
Debug.Log($"[{myWorldName}][EVENT] Player: {player.displayName} | Score: {score}");
```

### 出力例
```
2025.12.31 01:46:48 Debug - [My World][EVENT] Player: SomeUser | Score: 100
2025.12.31 01:46:48 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d
```

### 特徴
- フォーマットはワールドごとに異なる
- プレフィックス規則なし（完全に自由）
- vrclog-goではWasmプラグインで対応

---

## ログ形式の変更履歴

| 時期 | 変更内容 |
|------|----------|
| 2018年 | `OnPlayerJoined`/`OnPlayerLeft`がログから一時削除、`[Player] Initialized PlayerAPI`が追加 |
| 2024年頃 | ログフォーマット変更（VRCXなどが対応を迫られる） |
| 2025年〜 | `OnPlayerJoined`にUser ID（`usr_xxx`）が追加 |

---

## デバッグログ有効化

起動オプションで詳細ログを有効化：
```
VRChat.exe --enable-debug-gui --enable-sdk-log-levels --enable-udon-debug-logging
```

| オプション | 効果 |
|-----------|------|
| `--enable-debug-gui` | デバッグGUIを有効化 |
| `--enable-sdk-log-levels` | SDK関連ログを増加 |
| `--enable-udon-debug-logging` | Udon Debug.Logを出力 |

---

## 参考リソース

### 公式
- [VRChat Debugging Docs](https://creators.vrchat.com/worlds/udon/debugging-udon-projects/)
- [VRChat Output Logs Help](https://help.vrchat.com/hc/en-us/articles/9521522810899)

### コミュニティ
- [VRChat Log - Unofficial Wiki](http://vrchat.wikidot.com/worlds:guides:log)
- [VRChat Output Log Parser (nyanpa.su)](https://nyanpa.su/vrchatlog/)
- [VRCX](https://github.com/vrcx-team/VRCX)
- [XSOverlay-VRChat-Parser](https://github.com/nnaaa-vr/XSOverlay-VRChat-Parser)
- [VRChat-Log-Monitor](https://github.com/Kavex/VRChat-Log-Monitor)
- [vrchat-log-rs](https://github.com/sksat/vrchat-log-rs)

### Feature Request
- [UserIDs in output logs](https://feedback.vrchat.com/feature-requests/p/provide-userids-in-output-logs)

---

## 更新履歴

- 2025-01-03: 初版作成（Web調査に基づく）
