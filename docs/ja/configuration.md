---
title: 設定
description: Hyla の設定ファイルと、そのキーとデフォルト値。
---

# 設定

Hyla は設定をひとつのプレーンテキストファイルに保存します。設定ウインドウ（<kbd>⌘</kbd> <kbd>,</kbd>）はコメントと順序を保ったままこのファイルを編集し、手で編集したときは Hyla が読み込み直します。

## ファイル

Hyla は次の順に優先して読み込みます:

1. `$HYLA_CONFIG`
2. `$XDG_CONFIG_HOME/hyla/config`
3. `~/.config/hyla/config`

構文は Ghostty と同じです: 1 行に 1 つの `key = value`、コメントは `#`、リストを持つキーは行を繰り返します。Ghostty と同じ意味のキーには、Ghostty と同じ名前が付いています。

```ini
# ~/.config/hyla/config
font-size = 14
theme = light:GitHub Light Default,dark:Catppuccin Mocha
cursor-style = bar

env = EDITOR=nvim
env = HOMEBREW_NO_AUTO_UPDATE=1

keybind = cmd+shift+d=pane.split.down
```

ファイルには変更したものだけを書けば十分です。それ以外はデフォルトのままです。

```sh
hyla config edit              # $EDITOR で開く
hyla config show              # すべてのキーと現在の値
hyla config check             # 不明なキーと不正な値を報告
```

## 一般

| キー                    | デフォルト | 説明                                                          |
| ----------------------- | ---------- | ------------------------------------------------------------- |
| `new-session-directory` | `current`  | 新規セッションの開始場所: `current`、`project`、`home`        |
| `session-name`          | `focused`  | 名前を付けていないセッションの名前の付け方: `focused`、`lead`、`folder`（[詳しく](./sessions.md#how-an-unnamed-session-is-named)） |
| `sidebar-rows`          | `detailed` | サイドバーのセッションの表示: `detailed` は 2 行目にステータスメッセージ、なければブランチと変更を表示、`compact` は 1 行 |
| `focus-follows-mouse`   | `false`    | ポインタが入ったペインにフォーカスを移す                      |
| `confirm-close`         | `true`     | プログラムが実行中のペインを閉じる前に確認する                |
| `restore-sessions`      | `true`     | 再起動のあとなど hylad が再起動したら、セッションを再び開始する |
| `resume-agents`         | `true`     | セッションの再開時に、各ペインで動いていたエージェントの会話を再開する |
| `session-history`       | `false`   | 終了したエージェントのセッションをサイドバーに一覧し、再開やアーカイブができるようにする |
| `macos-show-app-icon`   | `dock-and-menubar` | macOS で Hyla のアイコンを表示する場所: `dock-and-menubar`、`dock`、`menubar`。メニューバーに表示している間は、ウインドウを閉じても Hyla はそこで動き続ける。Windows と Linux では常に通知領域に表示し、ウインドウを閉じても同じく動き続ける |

## 外観

| キー                 | デフォルト | 説明                                                            |
| -------------------- | ---------- | --------------------------------------------------------------- |
| `window-theme`       | `system`   | アプリの外観: `system`、`light`、`dark`                         |
| `language`           | `system`   | アプリの言語: `system`、`en`、`ja`、`zh-Hans`                   |
| `background`         | `petal`    | ウインドウの背景: `petal`、`mist`、`meadow`、`plain`、`dock`（Dock のもの: Liquid Glass と薄いベール）    |
| `window-translucent` | `true`     | ウインドウからデスクトップを透けて見せる                        |
| `window-material`    | `sidebar` | ぼかすときのウインドウ背景の素材: `sidebar`、`window`、`popover`、`plastic`（磨りガラスのコンテンツ背景）、`glass`（Dock のような Liquid Glass）、`glass-clear` |

## ターミナル

| キー                  | デフォルト | 説明                                                                     |
| --------------------- | ---------- | ------------------------------------------------------------------------ |
| `theme`               | `hyla`     | ターミナルの色: `hyla`、任意の Ghostty テーマ、または `light:<theme>,dark:<theme>` |
| `font-family`         |            | 空欄なら Hyla に付属の JetBrains Mono Nerd Font、`monospace` ならシステムのもの、またはインストール済みの任意のファミリー |
| `font-fallback`       |            | `font-family` にない字形（漢字など）を表示するフォントを順に指定：`PingFang SC, Apple Color Emoji`。空欄ならシステムに任せる |
| `font-size`           | `13`       | ポイント単位                                                             |
| `font-weight`         | `400`      | `100` から `900`。Hyla のフォントは `200`（極細）と `300`（細字）を描ける |
| `line-height`         | `1.22`     | フォントの行の高さに対する倍率                                           |
| `font-ligatures`      | `true`     | フォントが描くとおりに文字をつなげる                                     |
| `font-thicken`        | `false`    | Ghostty と同じく、線を太くする                                           |
| `minimum-contrast`    | `3`        | テキストが背景に対して保つ最小のコントラスト。`1`（なし）から `21`。Ghostty と同じく、暗い背景向けの色が明るい背景でも読めるようにする |
| `dim-unfocused-terminals` | `true` | セッションに複数のペインがあるとき、使用中でないターミナルを薄くする |
| `cursor-style`        | `block`    | `block`、`bar`、`underline`。プログラムが変更することがある              |
| `cursor-style-blink`  | `true`     | カーソルを点滅させる                                                     |
| `macos-option-as-alt` | `false`    | Option でプログラムに Alt（メタ）を送る                                  |
| `copy-on-select`      | `false`    | 選択したものをクリップボードにコピーする                                 |
| `clipboard-read`      | `ask`      | プログラムによるクリップボードの読み取り（OSC 52）: `ask`、`allow`、`deny` |
| `right-click-action`  | `context-menu` | `context-menu`、`paste`、`copy-or-paste`                             |
| `mouse-scroll-multiplier` | `precision:1,discrete:3` | スクロールの量。Ghostty と同じく、`precision:` はトラックパッドのスクロールの倍率、`discrete:` はマウスホイール 1 ノッチでスクロールする行数（Windows では 3 行設定のときの 1 ノッチ）。数値を 1 つだけ書くと両方に適用。`0.01` から `10000` |

## シェル

| キー               | デフォルト | 説明                                                           |
| ------------------ | ---------- | -------------------------------------------------------------- |
| `command`          |            | 新しいペインのシェル。空欄ならログインシェル                   |
| `env`              |            | `NAME=value` として、すべてのペインの環境変数に追加する。繰り返して指定する |
| `scrollback-limit` | `4`        | 各ペインが保持する出力（メガバイト単位）。`0` で保持しない     |

## 通知

| キー                | デフォルト | 説明                                           |
| ------------------- | ---------- | ---------------------------------------------- |
| `notify-on-waiting` | `true`     | セッションが対応を求めたとき                   |
| `notify-on-done`    | `true`     | エージェントが完了したとき                     |
| `notify-on-error`   | `true`     | プログラムが失敗を報告したとき                 |
| `notify-sound`      | `false`    | 通知とともにサウンドを再生する                 |
| `dock-badge`        | `true`     | 対応が必要なセッションの数を Dock アイコンに表示する |
| `dock-bounce`       | `false`    | セッションが対応を求めたときに Dock アイコンを 1 回弾ませる |

## エージェント

| キー             | デフォルト | 説明                                                |
| ---------------- | ---------- | --------------------------------------------------- |
| `remote-control` | `true`     | `hyla do` と `hyla focus` によるアプリの操作を許可する |

## ホスト

アプリにセッションを表示し、`hyla -H` で接続する、ほかのマシン。[リモートホスト](./remote.md)を参照してください。

| キー           | デフォルト | 説明 |
| -------------- | ---------- | ---- |
| `host`         |            | `name=address` 形式のホスト: SSH なら `me@devbox` や `ssh://…`、WSL なら `wsl://distro`、hylad の tailnet リスナーなら `http(s)://…`。後ろに ` remote-control` を付けるとそのプログラムにアプリの操作を許可し、` color=teal`（red、orange、amber、green、teal、blue、purple、pink）を付けるとそのセッションに色を付けられる。ホストごとに繰り返して指定する |
| `listen`       |            | ほかのマシンの Hyla からこのマシンへの接続を許可する: Tailscale アドレスで待ち受けるなら `tailnet://:7473`、Tailscale Serve なら `tailnet-serve://`。それぞれ繰り返して指定する |
| `listen-allow` |            | マシンの所有者以外に接続を許可する相手: tailnet のログイン、`tag:name`、または tailnet 上の誰でもなら `*`。それぞれ繰り返して指定する |
| `remote-updates` | `false`  | ほかのマシンの Hyla が、ここの Hyla アプリをそのバージョンにアップデートすることを許可する（アプリは再起動する） |

## Git

| キー          | デフォルト | 説明                                                                                              |
| ------------- | ---------- | ------------------------------------------------------------------------------------------------- |
| `git-status`  | `true`     | 各セッションのブランチと[変更](./changes.md)を表示する                                            |
| `diff-layout` | `auto`     | 変更パネルの差分: `unified`、`split`（左右に並べる）、または広くなると左右に並べる `auto`          |
| `worktree-base` | `default` | [セッション専用のワークツリー](./changes.md#sessions-in-their-own-worktree)の起点: リポジトリのデフォルトブランチ `default`、または今いるセッションのブランチ `current` |
| `worktree-dir` |           | セッション専用のワークツリーを置く場所。それぞれリポジトリ名の下に置かれます。空ならリポジトリ内の `.hyla/worktrees` |

<a id="browser"></a>

## ブラウザ

| キー                  | デフォルト                 | 説明                                                                                |
| --------------------- | -------------------------- | ----------------------------------------------------------------------------------- |
| `browser-block-ads`   | `true`                     | ブラウザペインで[広告とトラッカーをブロック](./panes.md#blocking-ads)する           |
| `browser-filter-list` | `easylist`、`easyprivacy`  | ブロックに使うフィルタリスト。リストごとに繰り返す。空欄ならなし                    |

`browser-filter-list` には、Hyla が提供するリストの名前か、Adblock Plus 構文の任意のリストの `https://` URL を指定します:

| 名前               | リスト                                                              |
| ------------------ | ------------------------------------------------------------------- |
| `easylist`         | EasyList: 広告                                                      |
| `easyprivacy`      | EasyPrivacy: トラッカーとアクセス解析                               |
| `peter-lowe`       | Peter Lowe's list: 広告とトラッカーのサーバー                       |
| `easylist-china`   | EasyList China: 中国語サイトの広告                                  |
| `fanboy-cookie`    | EasyList Cookie List: Cookie の通知と同意バナー                     |
| `fanboy-social`    | Fanboy's Social Blocking List: ソーシャルメディアのボタンと埋め込み |
| `fanboy-annoyance` | Fanboy's Annoyance List: ポップアップと勧誘。上の 2 つも含む        |

```ini
browser-filter-list = easylist
browser-filter-list = easyprivacy
browser-filter-list = fanboy-cookie
browser-filter-list = https://example.com/my-filters.txt
```

<a id="keyboard"></a>

## キーボード

`keybind` は `<keys>=<action>` の形でアクションの割り当てを変えます。割り当てごとに繰り返します。キーを空けるには、アクションに `unbind` を使います。

```ini
keybind = cmd+shift+d=pane.split.down
keybind = cmd+k=palette.open
keybind = cmd+b=unbind
```

すべてのアクション ID は `hyla actions` で確認できます。デフォルトは[キーボードショートカット](./shortcuts.md)を参照してください。
