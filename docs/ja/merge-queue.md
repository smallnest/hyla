---
title: マージキュー & ブラウザ検証ゲート
description: 列車型ローカルマージキュー、チート防止 diff ガード、LLM 不要のブラウザリプレイ、Feishu ビジュアル承認カードで並列エージェントの変更を安全にマージします。
---

# マージキュー & ブラウザ検証ゲート

並列コーディングエージェントは素早くコードを書けますが、テスト破壊や UI リグレッションを防ぎながら複数ブランチを安全に統合する工程がボトルネックになります。Hyla は **LLM 不要のブラウザリプレイ** と **ローカルマージキュー + チート防止ガード** を組み合わせてこの問題を解決します。

## 1. ブラウザ検証の録画とゼロ LLM リプレイ

Hyla ブラウザペインのツールバーにある **録画 / 停止**・**リプレイ** ボタンから操作を録画・再生するか、CLI から検証手順を JSON フローとして記録できます。

```sh
hyla browser record start --name checkout -o .hyla/flows/checkout.json
hyla browser open http://localhost:3000/checkout
hyla browser click '#pay-now'
hyla browser wait --text "Order confirmed"
hyla browser screenshot
hyla browser record stop
```

記録したフローは LLM を一切使わずに何度でも再生でき、ベースライン画像とのピクセル差分を検証できます。

```sh
hyla browser replay .hyla/flows/checkout.json --update-baseline
hyla browser replay .hyla/flows/checkout.json --feishu-chat <chat-id>
```

## 2. ローカルマージキュー（`hyla mq`）

デスクトップアプリの **セッション › マージキュー…**（macOS: <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>M</kbd>、Windows/Linux: <kbd>Ctrl</kbd> <kbd>Alt</kbd> <kbd>M</kbd>）からキューの追加・実行・チート防止アラートの承認/却下ができます。CLI の場合：

完了したエージェントのワークツリーセッションをキューに追加し、一時統合ワークツリーで最大 4 本ずつまとめてチェック・マージします。

```sh
hyla mq add <session-id> --check "go test ./..." --flow .hyla/flows/checkout.json
hyla mq ls
hyla mq run --train 4 --cleanup --browser <browser-pane> --feishu-chat <chat-id>
```

- **コンフリクト発生時**: 該当レーンを元のペインへエラー付きで差し戻します。
- **トレインのチェック失敗時**: 各レーンを個別に切り分けテストし、問題のないレーンだけをマージして原因レーンを差し戻します。

## 3. チート防止ガード & Feishu ビジュアル承認

マージ前に `git diff <base>...<branch>` を検査し、既存テストの削除（`deleted_test`）、`t.Skip`/`.skip(` などの追加（`test_skip`）、CI やリンター設定の変更（`config_tamper`）を検知した場合は `needs_review` で停止します。`--feishu-chat` を指定すると検証スクリーンショット付きの承認カードが送信され、Feishu 上のボタンまたは `hyla mq approve <lane-id>` で承認できます。
