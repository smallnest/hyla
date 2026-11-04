---
title: コーディングエージェント
description: Claude Code、Codex などのエージェントに、自分の横で作業を動かし、Hyla に進捗を報告することを教えます。
---

# コーディングエージェント

Hyla で動くエージェントは、あなたと同じように `hyla` コマンドを使えます: 下のペインで開発サーバーを起動し、2 つ目のエージェントにレビューを任せ、テストを待ち、行き詰まったらあなたに知らせます。

<a id="install-the-skill"></a>

## スキルをインストールする

```sh
hyla skill setup
```

これで 2 つのものがインストールされます:

- エージェントに CLI を教える **スキル**。Claude Code は `~/.claude/skills/hyla` から、Codex、OpenCode などのエージェントは `~/.agents/skills/hyla` から読み込みます。
- エージェントがセッションを始めるときに実行される **フック**。エージェントに、どの Hyla セッションとペインにいるかを伝えるので、それを調べる手間が省けます。また、名前のないセッションに `fix-login-redirect` のようなタスクにちなんだ名前を付けさせるので、あなたが付ける必要はありません。さらに、エージェント自身のセッションを Hyla に伝えるので、再起動のあとなど hylad が再起動したとき、エージェントは[会話を再開](./sessions.md#hylad)します。

フックは、インストールされているエージェントごとに、それぞれが探す場所に置かれます:

| エージェント | フックの場所                                          |
| ------------ | ----------------------------------------------------- |
| Claude Code  | `~/.claude/settings.json` の `SessionStart` フック     |
| Codex        | `~/.codex/hooks.json` の `SessionStart` フック         |
| Pi           | 拡張機能 `~/.pi/agent/extensions/hyla.ts`              |
| OpenCode     | プラグイン `~/.config/opencode/plugins/hyla`           |

`CLAUDE_CONFIG_DIR`、`CODEX_HOME`、`PI_CODING_AGENT_DIR`、`XDG_CONFIG_HOME` でエージェントのファイルの場所が変わっている場合、Hyla はそれに従い、各設定ファイルのほかの部分はそのままにします。

[Feishu](./feishu.md) のチャットでは、Claude Code に `Stop` フックも入ります。各ターンが終わると、フックはエージェントの最後のメッセージを、ボットがそのペインに渡したファイルに書き込みます。ボットはターミナルの画面ではなく、そのメッセージで返信します。有効にするには `hyla skill setup` をもう一度実行します。ほかのエージェントはターミナルのテキストを使います。

1 つのエージェントだけをセットアップするには、`--agent` に `claude`、`codex`、`pi`、`opencode` のいずれかを渡します。セットアップは一度だけです: スキルは Hyla 自身のコピーへのリンクで、Hyla のアップデートに合わせて更新されます。Pi と OpenCode のプラグインも Hyla が最新に保ちます。`hyla skill status` で、各部分が最新かどうかがわかります。

```sh
hyla skill status    # インストール済みのものと、それが最新かどうか
hyla skill show      # スキルを出力
hyla skill remove    # スキルとフックをアンインストール
```

フックが働くのはセットアップ後に始めたエージェントのセッションだけで、エージェントが先に確認を求めることもあります:

- **Claude Code** は、信頼したフォルダでのみフックを実行します。
- **Codex** は新しいフックの承認を求めます: `/hooks` で Hyla のフックを承認してください。
- **OpenCode** は、バックグラウンドサービスの起動時にプラグインを読み込みます: 作業中のセッションがないときに、一度 `opencode service restart` を実行してください。

OpenCode はエージェントのコマンドをペインではなくバックグラウンドサービスで実行するので、`HYLA_SESSION` と `HYLA_PANE` がありません。代わりに Hyla がエージェントにペインを伝え、エージェントはそれを対象として指定します。同じ理由で、OpenCode のエージェントからの `hyla report` はそのペインに届きません。

Hyla の外では、フックは何もしません。

## エージェントにできること

自分の横で何かを動かす:

```sh
hyla split --down -- npm run dev
hyla new -n review -- claude
```

それを操作して結果を読む:

```sh
hyla prompt review "review the changes in src/auth"
hyla wait tests --for idle,error --timeout 10m
hyla read tests -n 60
```

自分の横でテストを実行して、失敗した内容を読む:

```sh
pane=$(hyla split --down -- go test ./...)
hyla wait "$pane" --exit || hyla read "$pane" -n 80
```

状況をあなたに伝える:

```sh
hyla report working "Migrating the database" --progress 40
hyla report blocked "Approve the production deploy?"
hyla report done "Deployed to staging"
```

`blocked` の報告でセッションは*待機中*になり、あなたに通知されます。エージェントがこれを使うのは、あなたなしでは続けられないときだけにしてください。

## リモート操作

**リモート操作** がオン（デフォルト、**設定 › エージェント**）なら、エージェントはアプリ自体も操作できます: `hyla focus` でセッションを表示したり、`hyla do` でメニューのアクションを実行したりします。エージェントを自分のセッションだけにとどめるには、`remote-control = false` でオフにします。

## 環境変数

すべてのペインには次の変数があります:

| 変数           | 値                             |
| -------------- | ------------------------------ |
| `HYLA_SESSION` | ペインのセッションの ID        |
| `HYLA_PANE`    | ペインの ID                    |

スクリプトが Hyla の中で動いているかどうかは、`$HYLA_PANE` で確認できます。
