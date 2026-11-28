---
title: マルチエージェント・アリーナ & クロスレビュー
description: 複数のコーディングエージェントを隔離されたワークツリーで並列競走させ、テストや差分を比較して採用したり、別エージェントで対抗レビューを実行したりできます。
---

# マルチエージェント・アリーナ & クロスレビュー

Hyla は Claude Code、Codex、OpenCode、Pi、Antigravity（`agy`）を並べて動かせます。**Arena** と **Cross-Review** を使うと、同じプロンプトで複数のエージェントを競争させて最良の結果を選んだり、別のモデルに読み取り専用のコードレビューを依頼したりできます。

## Arena：エージェント並列競走

デスクトップアプリのメニュー **セッション › マルチエージェントアリーナ…**（macOS: <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>A</kbd>、Windows/Linux: <kbd>Ctrl</kbd> <kbd>Alt</kbd> <kbd>A</kbd>）または CLI から開始・管理できます。

任意の Git リポジトリでアリーナを開始すると、Hyla は各エージェント用に独立した worktree とブランチを作成して同時に起動し、ステータスと差分統計を追跡します。

```sh
hyla arena start --agents claude,codex,agy --test "go test ./..." "ログインハンドラにレート制限を追加"
```

進行状況の確認、全ワークツリーでの並列テスト実行、勝者のマージは次のコマンドで行えます。

```sh
hyla arena ls
hyla arena status <arena-id>
hyla arena test <arena-id>
hyla arena pick <arena-id> claude
```

`hyla arena pick` は勝者のワークツリーに未コミットの変更がないことを確認した上でベースブランチへマージし、ほかのワークツリーを片付けます（`--no-merge` や `--keep-others` で保持も可能）。

## Cross-Review：別エージェントによる対抗レビュー

任意のターミナルペインから、今動いているものとは別のエージェントを隣のペインに開いて読み取り専用レビューを実行できます。

- **メニュー**：**ペイン › 別エージェントでクロスレビュー**（macOS: <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>V</kbd>、Windows/Linux: <kbd>Ctrl</kbd> <kbd>Alt</kbd> <kbd>R</kbd>）
- **CLI**：

```sh
hyla review                  # 別のインストール済みエージェントで未コミット差分をレビュー
hyla review --branch         # main/master からのブランチ全体の差分をレビュー
hyla review --reviewer codex # レビュアーを明示的に指定
```
