---
title: ガードレール、タイムトラベル & 夜間ファクトリ
description: パス占有宣言、衝突レーダー、ターン毎のタイムトラベルチェックポイント、却下してルール化、Web ゲーム、夜間自律ファクトリで安全に並列運用します。
---

# ガードレール、タイムトラベル & 夜間ファクトリ

Hyla は複数のコーディングエージェントを同時かつ自律的に動かすための 4 つの安全・自動化機能を備えています。

## 1. クロスペインパスの占有宣言 & 衝突レーダー (`#20`)

特定ファイルやグロブパターンをペインに割り当て、マージ前に使い捨てワークツリーで統合テストを事前検証します：

```sh
hyla claim add "internal/auth/*" --mode block --reason "Migrating auth tokens" --ttl 1800
hyla claim ls
hyla claim release <claim-id>

hyla clash check --base main --branches feat/a,feat/b --cmd "go test ./..."
```

## 2. タイムトラベルチェックポイント & 却下してルール化 (`#23`)

- **GUI**: **セッション › タイムトラベルチェックポイント…**（`⌘⇧H` / `Ctrl+Alt+H`）からスナップショット保存・安全バックアップ付き復元・ワークツリー分岐が可能です。
- **却下してルール化**: **Changes** パネルのコード差分ハンクで **却下してルール化** を押すと、差分を戻して `AGENTS.md` または `.hyla/rules.md` に恒久ルールを追記します。

```sh
hyla checkpoint ls
hyla checkpoint create --label "before refactor"
hyla checkpoint restore <checkpoint-id>
hyla checkpoint fork <checkpoint-id> --branch exp/alt
hyla rule add "生 SQL 文字列連結を禁止する" --file internal/db/query.go --dest agents
```

## 3. 夜間自律ファクトリ・暴走遮断・Feishu 朝レポート (`#24`)

```sh
hyla factory start --issues 17,20,23 --agent claude --max-parallel 2 --max-minutes 45 --cmd "go test ./..." --auto-enqueue --feishu-chat <chat-id>
hyla factory status <run-id>
hyla factory digest <run-id> --feishu-chat <chat-id>
hyla factory stop <run-id>
```

## 4. Web ゲームペイン (`#17`)

**`hyla.webgames`** プラグインでは 2048、スネーク、テトリス、ブロック崩し、マインスイーパー、カスタム URL をプレイでき、エージェントが作業完了または確認待ちになると自動一時停止します。
