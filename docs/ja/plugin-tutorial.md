---
title: プラグインを作る
description: 手を動かすチュートリアル。10 分で作るテーマとエージェントのパック、daemon でネイティブなカードを描くプラグイン、サンドボックス WebView のペイン、そして設定・権限・デバッグ・パッケージ化まで。
---

# プラグインを作る

Hyla のプラグインは、Hyla 自体を作り直さずに拡張します。知らないコーディングエージェント、テーマ、サイドパネルのカード、設定のページ、自分だけのペイン。層は三つあり、プラグインは必要な層だけを使います——ほとんどは一つで足ります。

| 層 | 何か | 要るもの |
| --- | --- | --- |
| **1 — パック** | `hyla-plugin.toml` と静的ファイル: エージェント検出、マーク、テーマ、背景、ペット、フック、スキル | プロセスなし、コードなし |
| **2 — ホストプロセス** | Hyla が JSON-RPC 2.0 で話すプログラム。stdio かソケット経由で、エージェント・タイムライン・チャット・アクション・UI ツリーを扱う | Go、TypeScript、stdin を読めるものなら何でも |
| **3 — UI** | ネイティブな宣言的 UI（JSON ノード）、または HTML のためのサンドボックス WebView | マウントする場所 |

このページで各層を一つずつ作ります。リファレンスは[プラグイン](./plugins.md)にあります——マニフェストの全フィールド、RPC メソッド、UI ノード、権限スコープ、エラーコード。

::: tip
完成したプラグインを最短で見るなら、Hyla が同梱している **`hyla.magpie`** です。情報パネルにカードを描き、パレットのアクションと設定を持っています。ソースはリポジトリの `internal/plugins/builtins/magpie`。
:::

## はじめる前に

- Hyla 3.3 以降、`hyla` コマンドが `PATH` に（`hyla --version`）。
- 作業用のフォルダ。プラグインはディレクトリからリンクまたはインストールするもので、Hyla のフォルダに入れる必要はありません。
- 層 2 か 3 なら Go 1.22+ か [Bun](https://bun.sh)。テンプレートはどちらかを使います。

どのプラグインにも id があります。二段以上の逆ドメイン名、小文字で `com.example.hello`。ディレクトリ、設定、アクション、WebView のオリジンを決めるので、決めたら変えないでください。

```sh
hyla plugin ls            # インストール済み、リンク済み、同梱のプラグイン
```

## パート 1 — パック: テーマ、背景、エージェント

パックはファイルとマニフェストだけ。ひな形を作ります:

```sh
hyla plugin init --template pack --id com.example.aurora --name "Aurora" ./aurora
cd aurora
```

```text
aurora/
├── hyla-plugin.toml
├── detect/agent.toml
├── marks/agent.svg
├── themes/aurora.ghostty
└── washes/aurora.toml
```

マニフェストが、パックが持ってくるものを並べます:

```toml
api_version = 1
id = "com.example.aurora"
name = "Aurora"
version = "0.1.0"
description = "Declarative asset pack plugin for Hyla"
author = "You"

[[agents]]
id = "sample-agent"
name = "Sample Agent"
detect = "detect/agent.toml"
mark = "marks/agent.svg"
processes = ["sample-agent"]

[[themes]]
name = "Aurora Dusk"
file = "themes/aurora.ghostty"

[[washes]]
id = "aurora-mist"
name = "Aurora Mist"
file = "washes/aurora.toml"
```

**エージェント**はプロセス名で見分け、マークで描き、`detect/agent.toml` で読み取ります——ペインの画面を状態に変えるルールです:

```toml
id = "sample-agent"

[[rules]]
id = "sample-agent.permission"
state = "blocked"
visible_blocker = true
regex = ["(?i)allow|approve\\?"]
```

ルールの `state` は `working`、`blocked`、`idle` のいずれか。`visible_blocker = true` の `blocked` がセッションを*待機*に上げ、通知します。マークは 16×16、`currentColor` で描いた SVG なので、サイドバーの色をそのまま受けます。

**テーマ**は Ghostty のテーマファイルです。Hyla のターミナルは libghostty が描くので、形式は Ghostty のもの:

```text
background = #161822
foreground = #e2e8f0
cursor-color = #7dd3fc
```

**背景**はウィンドウの下地。明るいときと暗いときに色を一組ずつ:

```toml
id = "aurora-mist"
name = "Aurora Mist"
light = ["#f4f7fb", "#e6eefb"]
dark = ["#141824", "#1b2438"]
light_glow = "#cfe5ff"
dark_glow = "#263b5e"
```

フォルダをリンクすれば、すぐ効きます:

```sh
hyla plugin link .
```

**設定 › プラグイン**に現れ、必要なスコープ（パックならなし）と出どころが書かれています。*Aurora Dusk* は **設定 › 外観** に、*Aurora Mist* はその隣に、`sample-agent` を動かすペインは Hyla 自身のエージェントと同じように描かれます。

ファイルを編集するときは、リンクだけで足ります——**設定 › プラグイン**の**再読み込み**を押すか、フォルダを見張らせて保存のたびに再読み込みさせます:

```sh
hyla plugin dev .
```

::: tip
パックは何も再起動しません。`hyla plugin dev` は変更のたびにリンクし直すので、テーマやマークがその場で反映されます。
:::

## パート 2 — カードを、daemon の中で

次は動くプラグイン。情報パネルにカードを描き、ボタンのクリックを数え、自分がどのセッションにいるかを出します。

```sh
hyla plugin init --template go --id com.example.hello --name "Hello" ./hello
cd hello
```

```text
hello/
├── hyla-plugin.toml
├── main.go
├── Makefile
└── go.mod
```

マニフェストはカードのマウント位置とセッション状態を求め、バイナリを `hylad` で動かします:

```toml
[permissions]
scopes = ["ui.slot", "session.read"]

[daemon]
command = ["./bin/plugin"]
transport = "stdio"
auto_start = false

[[actions]]
id = "refresh"
title = "Refresh Hello"
palette = true

[[ui.slots]]
id = "card"
mount = "info.card"
title = "Hello"
mode = "declarative"
host = "daemon"
```

`hyla plugin init` が書く `main.go` はプロトコルを手で話します。一行の JSON を受けて一行の JSON を返し、`ui.render` はツリーを返し、`ui.event` はクリックを数えます。

```go
case "initialize":
    res = map[string]any{"name": "go-plugin", "version": "0.1.0"}
case "ui.render", "ui.event":
    if req.Method == "ui.event" {
        clicks++
    }
    res = map[string]any{
        "root": map[string]any{
            "type": "column",
            "children": []any{
                map[string]any{"type": "text", "props": map[string]any{"text": fmt.Sprintf("Clicks: %d", clicks)}},
                map[string]any{"type": "button", "id": "inc", "onClick": "inc", "props": map[string]any{"label": "Click Me"}},
            },
        },
    }
```

カード一枚のプロトコルはこれだけです。`initialize` で名乗り、`ui.render` が `{root}` を返し、`ui.event` はまたツリーを返します（`invalidate: true` で次の描画を促す）。`action.invoke` はパレットのアクションに答え、`shutdown` で終わります。**stdout を行くのは JSON-RPC だけ**です——`fmt.Println` を一つ混ぜるだけで流れが壊れます。ログは stderr へ。Hyla がプラグインのログに残します。

ビルドしてリンクし、セッションの情報パネルを開きます:

```sh
make                      # マニフェストのコマンドどおり ./bin/plugin を作る
hyla plugin link . --grant-all
```

`--grant-all` はマニフェストが求めるスコープをまとめて与えます。開発中のリンクにはこれで十分です。他人がインストールするときは **設定 › プラグイン**で同意します。

### 同じカードを SDK で

カード一枚なら手書きのループで足ります。エージェント、タイムライン、設定、テストまで扱うなら Go SDK、`github.com/smallnest/hyla/sdk/go/hylaplugin`:

```go
plg := &hylaplugin.Plugin{
    Name:    "hello-agent",
    Version: "0.1.0",
    OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
        st, err := host.GetState(ctx)          // session.read が要る
        if err != nil {
            return hylaplugin.UIRenderResult{}, err
        }
        return hylaplugin.UIRenderResult{
            Title: "Hello Agent",
            Root: hylaplugin.Column([]*hylaplugin.UINode{
                hylaplugin.Text("Active: "+st.ActiveSession, hylaplugin.WithProp("tone", "muted")),
                hylaplugin.Button("ping", "Ping", "primary", hylaplugin.WithOnClick("ping")),
            }, hylaplugin.WithStyle(hylaplugin.UIStyle{Gap: 8})),
        }, nil
    },
    OnUIEvent: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIEventParams) (hylaplugin.UIEventResult, error) {
        return hylaplugin.UIEventResult{Handled: true, Invalidate: true}, nil
    },
}

if err := hylaplugin.Run(plg); err != nil {
    log.Fatal(err)
}
```

`hylaplugin.Run` がループとハンドシェイク、ログの作法を引き受けます。`Host` は Hyla への呼び出し口（`GetState`、`ReadScreen`、`SendInput`、`Focus`、`Notify`、`GetSettings`）。ノードのビルダーも揃っています: `Column`、`Row`、`Text`、`Badge`、`Button`、`Toggle`、`TextInput`、`Progress`、`InfoCard`。`sdk/go/hylaplugin/examples/hello-agent` はこのプラグインを最後まで書いた例で、エージェントとタイムラインも付いています。ビルドして `hyla plugin link sdk/go/hylaplugin/examples/hello-agent --grant-all`、ペインで `hello-agent` を動かしてください。

SDK の `NewTestHarness(plg)` は Hyla なしでハンドラを呼べます。SDK 自身のテストも、同梱の Magpie プラグインもこうやって試しています。

::: tip
動かすマシンに合わせてビルドしてください。daemon プラグインは `hylad` で動き、それが別のマシンのこともあります。**Go** のバイナリはそのプラットフォーム向けに、**TypeScript** ならそのマシンの Bun の上で。
:::

## パート 3 — 自分のペインを HTML で

WebView のスロットは、Hyla がプラグイン自身のオリジンから配るページで、専用のサンドボックスセッションの中で動きます。テンプレートは TypeScript、ページは `web/`:

```sh
hyla plugin init --template ts-webview --id com.example.inspector --name "Inspector" ./inspector
cd inspector
```

```text
inspector/
├── hyla-plugin.toml
├── package.json
├── src/plugin.ts
└── web/index.html
```

マニフェストはパレットのアクションでペインを開き、そこにページを載せます:

```toml
[permissions]
scopes = ["ui.webview", "session.read"]

[app]
command = ["bun", "run", "src/plugin.ts"]
transport = "stdio"

[[actions]]
id = "open"
title = "Open Inspector"
menu = "Plugins"
palette = true
target = "open_pane"
view = "main"

[[ui.slots]]
id = "main"
mount = "pane"
title = "Inspector"
mode = "webview"
web_root = "web"
entry = "index.html"
host = "app"
```

ページは普通の HTML です。Hyla がその隣に二つの仮想エンドポイントを用意します——生きたテーマと、プラグインプロセスへの橋です。

```html
<link rel="stylesheet" href="/_hyla/theme.css" />
<script src="/_hyla/bridge.js"></script>
<script>
  document.getElementById("ping").addEventListener("click", async () => {
    const res = await window.hyla.rpc("ping", { hello: "hyla" });   // → プラグインの webview.rpc
    document.getElementById("out").textContent = JSON.stringify(res, null, 2);
  });
</script>
```

`/_hyla/theme.css` はウィンドウの配色を CSS 変数にしたもの（`--hyla-bg`、`--hyla-text`、`--hyla-accent`、`--hyla-divider`、`--hyla-font-ui` など）で、ページはテーマに追従します。`window.hyla.rpc(method, params)` はプラグインプロセスの `webview.rpc` 呼び出しになり、`src/plugin.ts` が答えます:

```ts
if (msg.method === "webview.rpc") {
  result = { data: { echo: msg.params?.method, time: new Date().toISOString() } };
}
```

動かします: `hyla plugin link . --grant-all`、そして **プラグイン › Open Inspector** かパレットから。ページは `Untrusted` の WebView で、CSP は厳しく、クロスオリジンの遷移は止められ、`window.hyla` のほかは Hyla に触れません。プラグインのペインはウェブページであって、アプリ本体ではありません。

逆にプロセスからページへ押すなら `webview.emit`、ページ側は `window.hyla.on(event, listener)` で受けます。

## パート 4 — 設定とシークレット

プラグインの設定はマニフェストに書き、Hyla が描きます。UI のコードは要りません:

```toml
[permissions]
scopes = ["secrets"]

[[settings]]
key = "endpoint"
title = "API Endpoint"
description = "Base URL of the backend this plugin reads"
type = "string"
default = "http://localhost:8080"
placeholder = "https://…"

[[settings]]
key = "token"
title = "API Token"
description = "Kept in the system keychain, never in the configuration"
type = "secret"
```

Hyla はこれを **設定 › プラグイン**のそのプラグインの引き出しに描き、設定検索に載せ、値を預かります。シークレットでないものはプラグインの設定ファイルへ、`secret` はシステムのキーチェーンへ——設定ファイルにも、ログにも、他のマシンへの同期にも出ません。

読むときは呼び出しで、環境変数からではありません:

```go
var cfg hylaplugin.PluginGetSettingsResult
if err := host.Call(ctx, "plugin.getSettings", hylaplugin.PluginGetSettingsParams{}, &cfg); err != nil {
    return err
}
endpoint, _ := cfg.Settings["endpoint"].(string)
```

`plugin.setSettings` は値を書き戻します。サインインで得たトークンを更新するのがその用途で、`secrets` スコープが要る理由でもあります。設定が変わると `event.notify` の `settings.changed` でも届くので、読んだ値を覚えているプラグインに向きます。

### プラグインにできること

プラグインからの呼び出しはすべて権限の門を通ります。プラグインは `[permissions] scopes` で何が要るかを書き、Hyla はリンクやインストールのときにそれをそのまま見せます:

| スコープ | 何を許すか |
| --- | --- |
| `session.read` | `host.getState`——セッション、ペイン、フォーカス |
| `pane.read` | ペインの画面と環境変数を読む |
| `pane.write` | ペインへ入力、キー送出、状態の報告 |
| `pane.focus` | ペインにフォーカス |
| `session.create` | セッションを作る |
| `agent.provider` | エージェントを提供する: 検出、タイムライン、チャット、サブエージェント |
| `timeline.read` | エージェントのタイムライン、チャット、サブエージェントを読む |
| `ui.slot` | UI のマウント、描画、イベント、無効化、ペインを開く |
| `ui.webview` | WebView とブリッジを配る |
| `secrets` | シークレット設定の読み書き |
| `notifications` | 通知を出す |
| `filesystem.workspace` | ペインのフォルダのファイルを読み書き |

`hyla plugin permissions <id>` は与えられているものを表示し、`hyla plugin permissions <id> grant pane.write` で変えられます。高危険なものを含む完全な表は[プラグイン](./plugins.md#permission-scopes)に。

## パート 5 — デバッグ、パッケージ、共有

まずログ。プラグインの stderr、ライフサイクル、クラッシュは一つのログに入ります:

```sh
hyla plugin logs com.example.hello -n 100      # hylad がいればそれ経由
```

| 症状 | 何が起きているか |
| --- | --- |
| 設定 › プラグインで **degraded** | `max_crashes_per_minute` を超えて落ちた。直してから**再起動**（または `hyla plugin disable <id> && hyla plugin enable <id>`）。 |
| エラー `-32001` | マニフェストが求めたスコープが未許可: `hyla plugin permissions <id> grant <scope>`。 |
| カードが空、`window.hyla` が undefined | `mode = "webview"` なのに `ui.webview` がない、`<script src="/_hyla/bridge.js">` がない、`web_root` が違うフォルダを指している。 |
| 保存しても何も変わらない | リンクは今のファイルを見ます。*パック*は**再読み込み**、`hyla plugin dev`、またはホストプロセスの再起動が要ります。 |
| Go プラグインがすぐ終わる | stdout に JSON-RPC でないものが混ざった——ログは stderr へ。 |

人に渡すときは固めます:

```sh
hyla plugin pack . -o aurora.hylapkg
hyla plugin install aurora.hylapkg
```

`.hylapkg` は決定的な ZIP で、SHA-256 がレジストリに残ります。インストールはパスを一つずつ検査し（`..` も絶対パスもシンボリックリンクも不可）、ファイル数と展開後のサイズを制限し、バイナリとフックスクリプトの実行ビットを保ちます。**コード署名はありません**——パッケージが信頼されるのは、あなたがインストールし、そのスコープを与えたからです。

プラグインは Hyla の同期でマシン間を移動しますが、移動すべきでないものは動きません。レジストリとシークレットでない設定は渡り、シークレットは決して渡らず、他のマシンにないプラグインは**インストール待ち**として届き、勝手には動きません。

## 次に読むもの

- [プラグイン](./plugins.md) —— リファレンス: マニフェスト、RPC メソッドとイベント、UI ノードと配色トークン、マウント位置、エラーコード。
- `sdk/go/hylaplugin` —— Go SDK。`examples/hello-agent`（エージェント、タイムライン、カード）と `examples/git-summary`。
- `sdk/ts` —— `@hyla/plugin-sdk`（ホストプロセス用）と、WebView ページ用の `/web` エントリ。
- 同梱の `hyla.magpie`（`internal/plugins/builtins/magpie`）は「サービスを読み、カードを描き、設定を持つ」型の手本です。
- `hyla plugin --help` にコマンド一覧があります。
