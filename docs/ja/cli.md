---
title: コマンドライン
description: hyla CLI で、シェルやスクリプトから Hyla のセッションとペインを操作します。
---

# コマンドライン

`hyla` コマンドは、あなたのシェル、スクリプト、エージェントなど、どのシェルからでもセッションを操作できます。**Hyla › コマンドラインツールをインストール…** でインストールします。

## 対象

ほとんどのコマンドは**対象**を取ります: セッションかペインです。対象には次のものを指定できます:

- ID、またはその一意な前方部分
- セッションの名前
- セッション（フォーカスのあるペインを表す）

Hyla の中では、`$HYLA_SESSION` と `$HYLA_PANE` がコマンドを実行するセッションとペインを示し、それがデフォルトの対象になります。

## セッションとペイン

```sh
hyla ls                          # すべてのセッションとそのペイン
hyla new -n api -d ~/code/api    # セッションを開始し、その ID を出力
hyla new -- npm run dev          # コマンドを実行するセッションを開始
hyla split --down -- npm test    # このペインの横にペイン
hyla split --url localhost:3000  # ページを表示するブラウザペイン
hyla rename api backend          # セッション名を変更
hyla focus backend               # アプリで表示
hyla close backend               # セッションを終える（ペインを指定するとそのペイン）
hyla close fix-login --remove-worktree   # 専用のワークツリーも削除する
hyla restart <pane>              # 終了したペインのシェルを再び開始
```

`hyla new` のオプション:

| フラグ                      | 意味                                            |
| --------------------------- | ----------------------------------------------- |
| `-p`, `--project <name\|id>` | 入れるプロジェクト（デフォルト: ディレクトリから決まる） |
| `-n`, `--name <name>`       | セッションに名前を付ける                        |
| `-d`, `--dir <dir>`         | そこで開始する（デフォルト: 現在のディレクトリ） |
| `-f`, `--focus`             | アプリで表示する                                |
| `-w`, `--wait`              | コマンドの終了を待ち、その終了ステータスで終了する |
| `-u`, `--url <url>`         | シェルの代わりに、ページを表示するブラウザを開く |
| `--worktree <branch>`       | 専用の git ワークツリーで、新しいブランチで開始する（[詳しく](./changes.md#sessions-in-their-own-worktree)） |
| `--base <ref>`              | ワークツリーのブランチの起点（既定: デフォルトブランチ） |

## 入力と読み取り

```sh
hyla send api "npm run migrate" --enter   # テキストを入力してから Enter を押す
hyla send api -e 'y\n'                    # \n \r \t \e \xHH のエスケープを解釈
hyla key api ctrl+c                       # キーを押す
hyla read api -n 40                       # 画面の最後の 40 行
hyla read api -a                          # スクロールバックを含む画面
```

`hyla key` は `enter`、`tab`、`escape`、`up`、`down`、`left`、`right`、`backspace` と、`ctrl+c` や `ctrl+d` のような修飾キーを理解します。

## 待機

```sh
hyla wait api                             # 作業が止まるまで
hyla wait api --for waiting,error --timeout 10m
hyla prompt review "summarize the diff"   # 送信 + Enter、待機、画面を出力
```

`hyla wait` は戻るときにペインの状態を出力します。`hyla prompt` はテキストと Enter を送り、ペインの作業が止まるまで待ってから画面を出力します。ほかのエージェントに何かを尋ねる最も手早い方法です。

### コマンドの終了を待つ

`make test` のようなコマンドは*作業中*と表示されないので、終了を待つには `--wait` を付けて開始します。すると `hyla` はそのコマンド自身の終了ステータスで終了します:

```sh
if hyla split --down --wait -- make test; then
  echo "tests passed"
fi
```

`hyla wait --exit` は、開始時のコマンドをすでに実行しているペインに対して同じことをします:

```sh
pane=$(hyla split --down -- go test ./...)
# … ほかのことをする …
hyla wait "$pane" --exit --timeout 10m
echo "go test exited with $?"
```

コマンドの終了後も出力とともにペインは開いたままなので、`hyla read` で読めます。コマンドの終了を判断できない `cmd` をシェルにしている場合、コマンドの終了待ちは使えません。

## イベントの監視

`hyla events` は、セッションとペインに起きたことを起きたときに 1 イベント 1 行で出力し、止めるまで続けます。スクリプトは `hyla status` をポーリングせずに、すべてのエージェントを一度に追えます。

```sh
hyla events                               # すべてのセッション
hyla events api                           # 1 つのセッション、または 1 つのペイン
hyla events -p hyla                       # 1 つのプロジェクトのセッション
hyla events --for waiting,error           # これらの状態になったペインだけ
hyla events --json | jq -r .name          # 1 行に 1 つの JSON オブジェクト
hyla events --for waiting --once          # 最初の 1 件で終了
```

```
12:04:31  api x78n9ss  working → waiting  Allow Bash: rm -rf dist?
12:04:40  api  renamed api → backend
```

| イベント | いつ | フィールド |
| --- | --- | --- |
| `pane.status` | ペインの状態が変わったとき | `pane`、`from`、`to`、`message`、`kind`、`agent`。exited になったときは `exitCode` も |
| `command.exited` | ペインの開始時のコマンドが終わり、シェルが続くとき | `pane`、`exitCode` |
| `pane.created`、`pane.closed` | ペインが開始または終了したとき | `pane` |
| `session.created`、`session.closed` | セッションが開始または終了したとき | |
| `session.renamed` | セッションの名前が変わったとき | `from`、`to` |
| `session.moved` | セッションが別のプロジェクトに移ったとき | `from`、`to`（プロジェクトの ID） |
| `notice` | セッション内のプログラムがデスクトップ通知を送ったとき | `pane`、`title`、`body` |

`--json` では、どのイベントにも `event`、`time`（unix ミリ秒）、`session`、`project`、`name`（サイドバーに表示されるセッション名）が付きます。

`hyla events` が報告するのは今からの変化です。すでにあるものは `hyla ls --json` で確認します。対象を指定すると、対象が閉じたときに止まります。`--once` は最初に一致したイベントで終了し、対象が先に閉じるか `--timeout` が過ぎると `1` で終了します。hylad は状態が落ち着いてから送るので、一瞬だけの状態（短い作業中など）は出ないことがあります。

<a id="browsing"></a>

## ブラウザの操作

`hyla browser` は、`hyla send` と `hyla read` がターミナルを操作するのと同じように、ブラウザペインのページを操作します: クリック、入力、ページの読み取り、スクリーンショット。対象はブラウザペイン、またはセッション（そのブラウザ）です。対象を指定しなければ、実行したセッションのブラウザを操作します。

```sh
hyla split --url localhost:3000            # ページを開き、ペインの ID を出力
hyla browser open localhost:3000/todos     # ページへ移動し、読み込みを待つ
hyla browser type '#new-todo' "Buy milk" --enter
hyla browser wait '.todo-list li'          # 要素が表示されるまで
hyla browser wait --text "1 item left"     # テキストが表示されるまで
hyla browser text                          # ページのテキスト
hyla browser text h1                       # 要素のテキスト
hyla browser click 'button[type=submit]'
hyla browser screenshot -o page.png
hyla browser eval 'document.title'
```

| コマンド                        | 動作 |
| ------------------------------- | ---- |
| `open <url>`                    | ページへ移動し、読み込みを待つ |
| `back`、`forward`、`reload`     | 戻る、進む、または再読み込みして、それを待つ |
| `text [selector]`               | ページの表示テキスト、または最初に一致した要素のテキストを出力 |
| `html [selector]`               | ページ、または要素の HTML を出力 |
| `click <selector>`              | マウスと同じように要素をクリック |
| `type <selector> <text…>`       | フィールドの内容を置き換えてテキストを入力。`--enter` を付けるとその後 Enter を押し、フォームを送信する |
| `wait [selector] [--text text]` | 要素かテキストが表示されるまで待つ。どちらもなければページの読み込みまで |
| `eval <js…>`                    | ページで JavaScript を実行し、その値を出力: 文字列はそのまま、それ以外は JSON |
| `screenshot [-o file.png]`      | ページの画像を保存し、その場所を出力。`-o -` で標準出力に書き出す |

セレクタは CSS セレクタです。シェルでは引用符で囲んでください。要素が必要なコマンドは要素が現れるのを待ち、`--timeout`（デフォルト `10s`）でそれらとページの読み込みにかけてよい時間を設定します。`type` と `eval` は、`-` で標準入力からテキストを受け取ります。

クリックと入力は本物のマウスとキーボードと同じイベントを送るので、React などのフレームワークで作られたページもそれを受け取ります。スクリーンショットにはペインが画面に表示されている必要があります: 先に `hyla focus` で表示してください。それ以外は、表示されているかどうかにかかわらず、どのブラウザペインでも動きます。

`hyla do` と同じく、`hyla browser` には **設定 › エージェント** のリモート操作と、アプリが開いていることが必要です。

## 過去の会話

```sh
hyla history                     # ペインで動いたエージェントの会話を、最後に動いたものから順に
hyla history -p hyla -n 50       # ひとつのプロジェクトの会話を、最大 50 件
hyla history login --json        # 「login」に一致するものを JSON で
hyla resume 13b9                 # 再開する: ID、その先頭部分、または名前で
hyla resume fix-login -f         # 再開して、アプリに表示する
```

`hyla resume` は、その会話を再開するセッションを会話のディレクトリとプロジェクトで開始し、セッションの ID を表示します。まだペインで動いている会話なら、そのセッションを返します。[過去の会話](./sessions.md#past-conversations)を参照してください。

## 状態

```sh
hyla status api --json                    # 状態、プログラム、ディレクトリ
hyla report working "Running migrations"  # 自分の状態を報告
hyla notify "Need your input"             # このペインを対応が必要なものとして示す
```

状態については[状態と通知](./status.md)を参照してください。

## 変更

```sh
hyla diff                                 # ペインの未コミットの変更を git diff の形式で
hyla diff --turn                          # エージェントが直前のターンで変更したもの
hyla diff review --branch                 # デフォルトブランチ以降
```

[変更](./changes.md)を参照してください。

## 構成とセットアップ

```sh
hyla config path                 # 設定ファイルの場所
hyla config show                 # すべてのキーとその値
hyla config show --defaults      # すべてのキーの初期値
hyla config edit                 # $EDITOR で開く
hyla config check                # 不明なキーと不正な値を報告
hyla projects                    # プロジェクトを一覧
hyla skill setup                 # コーディングエージェントに hyla の使い方を教える
hyla daemon status|stop|restart  # hylad を管理
hyla daemon install-service      # マシンの起動とともに hylad を実行（systemd、launchd）
hyla daemon serve                # サービスマネージャ向けに hylad をフォアグラウンドで実行
hyla clients                     # hylad に接続しているもの
hyla pets                        # ウィンドウ上端のペット、表示中のものが先頭
hyla pets use <名前|hyla|codex>  # ペットを表示する：~/.codex/pets のもの、または Hyla のもの
hyla pets install <pack.zip>     # Codex ペットパック（.codex-pet.zip）をインストールする
hyla pets rm <名前>              # Hyla がインストールしたペットを削除する
```

## Feishu

```sh
hyla feishu setup                # アプリの ID と secret を尋ね、保存して接続し、次にすることを示す
hyla feishu status               # ボットが接続しているか
hyla feishu test                 # Feishu がアプリを認識するか確認する
hyla feishu send <チャット> <テキスト…>  # チャット ID でメッセージを送る
```

Feishu のチャットは、結び付いたセッションのコーディングエージェントを動かします: `/new` で始め、`/use` で開いているセッションに結び付け、そのほかのテキストはプロンプトです。[Feishu](./feishu.md)を参照してください。

## ほかのマシン


```sh
hyla hosts                       # 追加したホスト
hyla hosts add devbox me@devbox  # 追加する（アプリにも表示される）
hyla hosts check devbox          # 接続して、結果を表示
hyla hosts install devbox        # SSH でそこに Hyla をインストール
hyla ls --all                    # このマシンとすべてのホストのセッション
hyla -H devbox ls
hyla -H devbox new -n review -- claude
hyla -H devbox daemon update     # そこの Hyla をこちらのバージョンにアップデート
```

ペインの中では、`hyla` はアプリがどこにあっても、ペインが動いているマシンの hylad と通信します。

`hyla ls --all` はすべてのホストに同時に問い合わせ、それぞれのセッションを `-H` に渡す名前（このマシンは `local`）の下に一覧表示します。届かないホストは、セッションの代わりにその理由を表示します。`--json` を付けると `{host, addr, error, sessions}` のリストを出力します。

## アプリのアクションを実行する

`hyla do` は、メニューと同じアプリのアクションをどれでも実行します。**設定 › エージェント** のリモート操作（デフォルトでオン）が必要です。

```sh
hyla actions                     # 一覧を表示
hyla do pane.split.right
hyla do pane.browser url=localhost:3000
hyla do sidebar.toggle
```

## ヘルプと補完

```sh
hyla help                        # コマンドをグループごとに
hyla help new                    # 一つのコマンドの使い方
hyla new --help                  # 同じ
hyla help all                    # すべてのコマンドと引数
hyla help --markdown             # Markdown のリファレンス
```

知らないコマンドはその旨を言って `2` で終わり、アプリを開きません。

補完は同じコマンド表から作ります:

```sh
hyla completions zsh  > "${fpath[1]}/_hyla"
hyla completions bash > /etc/bash_completion.d/hyla
hyla completions fish > ~/.config/fish/completions/hyla.fish
```

## 終了ステータス

| コード | 意味                                 |
| ------ | ------------------------------------ |
| `0`    | 成功                                 |
| `1`    | 失敗                                 |
| `2`    | 使い方の誤り                         |
| `3`    | 対象が存在しない、またはあいまい     |
| `4`    | hylad に接続できない                 |

プログラムを待つコマンド（`hyla wait --exit` と `--wait`）は、代わりにそのプログラムの終了ステータスで終了します。ペインのシェルがエラーで終了したときの `hyla wait` も同じです。タイムアウトした待機は `1` で終了します。

`ls`、`status`、`projects` に `--json` を付けると、機械で読める形式で出力します。`status` に `--no-header` を付けると、列名のない一行だけになります。
