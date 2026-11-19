---
title: プラグイン
description: 宣言的なアセットパック、プロセス外の JSON-RPC 2.0 プラグイン、ネイティブな宣言的 UI、サンドボックス WebView のペインで Hyla を拡張します。
---

# Hyla のプラグインシステム

Hyla のプラグインシステムは、Hyla 自体を作り直さずに、ターミナルのエージェント検出、UI の外殻、コマンドパレット、独自のペイン、外部のチャット経路を拡張できます。

::: tip
プラグインが初めてなら、[プラグインを作る](./plugin-tutorial.md)が各層を一つずつ——テーマとエージェントのパック、daemon のカード、WebView のペイン——設定・ログ・パッケージ化まで含めて案内します。
:::

## アーキテクチャ（三つの層）

プラグインは必要な層だけを組み合わせます:

1. **層 1 —— 宣言的パック（`hyla-plugin.toml` + 静的アセット）**
   プロセスなし、ホットリロード可能。Hyla が直接読み込みます:
   - エージェント検出ルール（`detect/*.toml`）
   - エージェントのアイコン（`marks/*.svg|png|webp`）
   - ターミナルの配色テーマ（`themes/*.ghostty`）
   - ウィンドウの背景（`washes/*.toml`）
   - デスクトップペット（`pets/*`、`pet.json`、`FrameW=192`、`FrameH=208`、`Columns=8`）
   - コーディングエージェントのフックとスキル（`[[hooks]]`、`[[skills]]`）
   - 静的アクションとキーバインド（`[[actions]]`）
2. **層 2 —— プロセス外の JSON-RPC 2.0 ホスト**
   常駐または必要時に起動するプラグインプロセスが、**NDJSON（一行 16 MiB まで）**を **stdio** または**ローカル Unix ソケット / Windows 名前付きパイプ**で話します:
   - **Daemon ホスト（`hylad` / `host = "daemon"`）**: エージェント検出、タイムライン、チャット、サブエージェント、再開コマンド、チャネルドライバ、セッションをまたぐホスト状態の照会。ヘッドレスの `hyla-server` も含みます。
   - **App ホスト（`Hyla.app` / `host = "app"`）**: ローカル UI の操作、アプリのアクション、ネイティブな宣言的 UI スロット、WebView の RPC。
3. **層 3 —— UI のペイロード（宣言的 UI とサンドボックス WebView）**
   - **3A 宣言的 UI（`mode = "declarative"`）**: JSON の UI ツリーを `mygo/ui` がネイティブに描き、Hyla のライブな配色トークン（`$text`、`$accent`、`$card` など）で色づけします。
   - **3B サンドボックス WebView（`mode = "webview"`）**: HTML/JS/CSS を `hylaplugin://<plugin-id>/...` で、隔離した `Untrusted` WebView セッション（`"plugin:" + id`）に載せ、`/_hyla/theme.css` と `/_hyla/bridge.js` を注入します。

```text
+-------------------------------------------------------------------+
| Hyla Desktop App (window)                                         |
|  - Mounts: info.card, side.tab, pane, menus, palette, Settings    |
|  - Declarative UI Renderer (uihost -> mygo/ui)                    |
|  - Sandboxed WebView (hylaplugin://<id>/... -> Untrusted session) |
+---------------------------+-----------------------+---------------+
                            | Local/Remote RPC      | JSON-RPC 2.0
                            | (plugins.v1 cap)      | (App Host)
                            v                       v
+----------------------------------------+   +----------------------+
| hylad (Daemon Host)                    |   | App Plugin Process   |
|  - Agent detection, chat, timeline     |   | (stdio or socket)    |
|  - ChannelDriver (a chat channel)      |   +----------------------+
|  - keep_alive & idle-exit supervision  |
+---------------------------+------------+
                            | JSON-RPC 2.0 (NDJSON / stdio or socket)
                            v
                 +-----------------------+
                 | Daemon Plugin Process |
                 +-----------------------+
```

---

## プラグイン ID と保存先

### プラグイン ID のルール

プラグイン ID は、小文字で DNS に安全な逆ドメイン名で、点で区切った二段以上（アンダースコアや大文字は不可）:

```text
^[a-z0-9][a-z0-9-]{0,61}[a-z0-9](\.[a-z0-9][a-z0-9-]{0,61}[a-z0-9])+$
```

有効な例: `com.example.hello-agent`、`org.acme.git-radar`、`hyla.magpie`。
無効な例: `my_plugin`、`hello`、`Com.Example.Test`。

### 隔離されたディレクトリ構成

プラグインのファイルは `<HYLA_HOME>`（既定 `~/.hyla`）の下に置かれ、Hyla 本体の `config` の検証には混ざりません:

| パス | 用途 | マシン間で同期するか |
| --- | --- | --- |
| `~/.hyla/plugins/<id>/` | 展開したプラグインのディレクトリ（`hyla-plugin.toml`、アセット、バイナリ） | いいえ（無い側は `pendingInstall` になる） |
| `~/.hyla/plugins.json` | インストール済み・リンク済み・同梱の登録、有効状態、SHA-256、許可済みスコープ | はい（`linkPath` は除く） |
| `~/.hyla/plugin-config/<id>.json` | `[[settings]]` に対応する秘密でない設定 | はい |
| `~/.hyla/plugin-secrets/` | シークレット値（ヘッドレスの `hyla-server` で OS のキーチェーンが使えないときは `0600` のファイル。デスクトップは `mygo.Secrets` の `"hyla-plugin:" + id` サービス） | **決して** |
| `~/.hyla/plugin-data/<id>/` | プラグインの作業用・キャッシュ用ディレクトリ。`dataDir` / `HYLA_PLUGIN_DATA_DIR` として渡る | いいえ |
| `~/.hyla/logs/plugins/<id>.log` | ローテーションする stdout/stderr/ライフサイクルのログ（4 MiB でローテーション） | いいえ |

---

## マニフェストの仕様（`hyla-plugin.toml`）

```toml
api_version = 1
id = "com.example.demo"
name = "Demo Plugin"
version = "0.2.0"
description = "Short description shown in Settings > Plugins"
author = "Your Name"
homepage = "https://example.com/demo"

[engines]
hyla = ">=3.0.0"

[permissions]
scopes = ["ui.slot", "session.read"]

[daemon]
command = ["./bin/demo-daemon"]
transport = "stdio"          # "stdio"（既定）| "socket"
auto_start = false           # 最初の RPC まで起動しない。true ならホストと同時に
keep_alive = false           # true ならアイドルでも hylad を残す
min_restart_ms = 500
max_crashes_per_minute = 5   # 超えると degraded になる

[app]
command = ["bun", "run", "src/app.ts"]
transport = "stdio"          # "stdio" | "socket"
auto_start = false

[[agents]]
id = "my-agent"
name = "My Agent"
detect = "detect/my-agent.toml"
mark = "marks/my-agent.svg"
processes = ["my-agent"]
timeline = true
chat = true
subagents = false
resume_argv = ["my-agent", "--resume", "{session_id}"]

[[actions]]
id = "open"
title = "Open Demo View"
shortcut = "Cmd+Shift+D"
menu = "Plugins"
palette = true
target = "open_pane"         # "app" | "daemon" | "open_pane"
view = "main"                # target = "open_pane" のときのスロット ID

[[ui.slots]]
id = "main"
mount = "pane"               # info.card | side.tab | pane | sidebar.menu | files.menu | pane.menu | palette | settings.plugins | settings.page | settings.<page>
title = "Demo"
mode = "declarative"         # "declarative" | "webview"（settings.* は宣言的のみ）
host = "app"                 # "app" | "daemon"
icon = "marks/icon.svg"
web_root = "web"             # webview では必須
entry = "index.html"         # web_root 内の入口
order = 10

[[themes]]
name = "Aurora Dusk"
file = "themes/aurora.ghostty"

[[washes]]
id = "aurora-mist"
name = "Aurora Mist"
file = "washes/aurora.toml"

[[pets]]
id = "pixel-fox"
dir = "pets/pixel-fox"

[[hooks]]
agent = "claude"
event = "Stop"
script = "hooks/on-stop.sh"

[[skills]]
agent = "claude"
name = "my-skill"
dir = "skills/my-skill"

[[settings]]
key = "endpoint"
title = "API Endpoint"
description = "Custom backend base URL"
type = "string"              # string | secret | boolean | integer | number | select
default = "http://localhost:8080"
placeholder = "https://..."
options = []                 # type = "select" のときの候補

[env]
CUSTOM_FLAG = "1"
```

### アクション ID の修飾

`[[actions]]` のアクションは自動で `<plugin-id>.<action-id>` に修飾されます（`id` がすでに `<plugin-id>.` か `plugin.` で始まる場合はそのまま）。グローバルのアクションやショートカットは、Hyla の静的アクション表を触らずにこの ID を参照できます。

### 権限スコープ

プラグインからホストへの呼び出しはすべてホストの RPC ゲートウェイを通り、`plugins.json` の `grantedPermissions` を検査します。許可がなければ JSON-RPC エラー `-32001`（`ErrCodePermissionDenied`）を返します。

| スコープ | 高危険 | 何を許すか |
| --- | --- | --- |
| `session.read` | いいえ | `host.getState` |
| `pane.read` | いいえ | `pane.readScreen`、`pane.getEnv` |
| `pane.write` | **はい** | `pane.sendInput`、`pane.sendKey`、`pane.reportStatus` |
| `pane.focus` | いいえ | `pane.focus` |
| `session.create` | **はい** | `session.create` |
| `agent.provider` | いいえ | `agent.detect`、`agent.resumeCommand`、`agent.sendChat`、エージェントの登録（`agent.readTimeline`/`agent.readChat`/`agent.readSubagents` も伴う） |
| `timeline.read` | いいえ | `agent.readTimeline`、`agent.readChat`、`agent.readSubagents` |
| `ui.slot` | いいえ | `ui.invalidate`、`ui.openPane`、`ui.render`、`ui.event`、`action.invoke`、スロットとメニュー |
| `ui.webview` | いいえ | `hylaplugin://` の WebView、`webview.rpc`、`webview.emit` |
| `network` | いいえ | 外部ネットワークへのアクセスを宣言 |
| `secrets` | **はい** | `plugin.getSettings` で渡り、`plugin.setSettings` で書くシークレット |
| `notifications` | いいえ | `ui.notify` |
| `filesystem.workspace` | **はい** | ペインが参照するワークスペースのファイル |

### パックのファイル: マニフェストが指すもの

マニフェストはファイルに名前を付けるだけです。形式は次のとおり。

#### エージェント検出（`detect/*.toml`）

`[[agents]]` の `processes` はペインで動いているプログラム、検出ファイルはそれが動いている間の画面の意味を書きます:

```toml
id = "my-agent"

[[rules]]
id = "my-agent.permission"
state = "blocked"            # idle | working | blocked | unknown
visible_blocker = true       # セッションを待機に上げ、通知する
priority = 10
contains = ["Do you want to proceed?"]
line_regex = ["^\\s*[›>]\\s"]
```

ルールのゲートはすべて成立する必要があります: `contains`（部分文字列、大文字小文字は無視）、`regex`、`line_regex`（どれか一行に一致する正規表現）、そして入れ子にできる `all`、`any`、`not`。`region` は画面の一部だけを読みます:

| フィールド | 意味 |
| --- | --- |
| `state` | エージェントの状態: `idle`、`working`、`blocked`、`unknown` |
| `visible_*` | `visible_idle`、`visible_working`、`visible_blocker`: その状態を表示する（前回表示した状態を引き継がない） |
| `skip_state_update` | 画面は一致するが状態は変えない——動作中の上に重なるブロッカーなど |
| `priority` | 複数が一致したときの優先度。大きいほど先 |

表示される `blocked` のルールが、セッションを*待機*にしてサイドバーで上げ、通知します。

エージェントの `mark` は 16×16、`currentColor` で描いた SVG なので、表示する場所の色を取ります。`mark_full_color = true` なら SVG 自身の色のまま。

#### テーマ（`themes/*.ghostty`）

Ghostty のテーマファイルをそのまま。`background`、`foreground`、`cursor-color`、`selection-background`、`palette = 0=#…` から `15=#…` まで。

#### 背景（`washes/*.toml`）

ウィンドウの下地。明暗それぞれのグラデーション:

```toml
id = "aurora-mist"          # Hyla 同梱の背景と重ならないこと
name = "Aurora Mist"
light = ["#f4f7fb", "#e6eefb"]   # グラデーションの色
light_glow = "#cfe5ff"           # ウィンドウの後ろの光
light_angle = 135                # 角度
dark = ["#141824", "#1b2438"]
dark_glow = "#263b5e"
dark_angle = 135
```

#### ペット（`pets/<dir>/pet.json`）

スプライトシートと、その隣の `pet.json`:

```json
{
  "id": "pixel-fox",
  "displayName": "Pixel Fox",
  "name": "Pixel Fox",
  "description": "A fox that watches the panes",
  "spritesheetPath": "sheet.webp",
  "spriteVersionNumber": 2
}
```

フレームは **192×208** ピクセル、**一行に 8 枚**。一行が一つのアニメーションで、順に idle、右へ走る、左へ走る、手を振る、跳ぶ、失敗、待機、作業中、レビュー（バージョン 2 は右を見る、左を見るが加わる）。行の最初の数枚だけがアニメーションの全体で、残りは空で構いません（idle は 8 枚のうち 6 枚）。

#### フックとスキル

```toml
[[hooks]]
agent = "claude"      # claude | codex
event = "Stop"        # そのエージェント自身のイベント名
script = "hooks/on-stop.sh"

[[skills]]
agent = "claude"
name = "my-skill"
dir = "skills/my-skill"
```

フックはそのエージェントのフック設定にインストールされ、そのイベントで動きます。スキルはそのエージェントのスキルのフォルダにコピーされ、エージェントは自分の他のスキルと同じように読めます。プラグインを無効化・削除すると、どちらも外されます。

### スロットとアクションを出す条件

`[[ui.slots]]` や `[[actions]]` の `when = "…"` が、いつ出すかを決めます。条件は `,` か `&` でつなぎ——すべて成立する必要があります——`!` で否定します:

```toml
when = "pane.agent,!agent:claude"
```

| 原子 | 成立するとき |
| --- | --- |
| `session.selected` | セッションを表示している |
| `project.selected` | そのセッションがプロジェクトに属する |
| `pane.selected` | そのセッションにフォーカスのペインがある |
| `pane.terminal` | フォーカスのペインがターミナル |
| `pane.agent` | コーディングエージェントが動いている |
| `pane.browser`、`pane.file`、`pane.plugin` | ブラウザ、ファイル、プラグインのペイン |
| `agent:<id>` | Hyla が知るエージェントが `<id>`（例 `agent:claude`） |
| `always` | 常に——既定 |

アクションの `target` は誰が処理するか: `app` と `daemon` はそのホストのプラグインプロセスへ、`open_pane` は `view` のスロットを開きます。`menu` はそのメニューに、`palette = true` はコマンドパレットに、`shortcut` はキーを割り当て、`remote = true` はリモートのセッションから実行できるようにします。

---

## 層 2: プロセス外の JSON-RPC 2.0 プロトコル

### トランスポートと認証

1. **`stdio`（既定）**: ホストは `command` を独立したプロセスグループで起動します（Unix は `Setpgid`、Windows はプロセス/ジョブの後始末）。要求・応答・通知は改行区切りの JSON（`\n`）で、**一行 16 MiB** まで。`stderr` の出力は `~/.hyla/logs/plugins/<id>.log` に集まります。`stdio` のプラグインは**必ず `stderr` にだけ**ログを出し、`stdout` には書かないこと。
2. **`socket`**: ホストがローカルの Unix ソケット（または Windows の名前付きパイプ）を作り、`HYLA_PLUGIN_SOCKET` と `HYLA_PLUGIN_TOKEN` を環境変数で渡します。接続後の最初の一行は:

```json
{"token":"<HYLA_PLUGIN_TOKEN>"}
```

さらに次の環境変数が渡ります:
- `HYLA_PLUGIN_ID`
- `HYLA_PLUGIN_DIR`
- `HYLA_PLUGIN_DATA_DIR`
- `HYLA_PLUGIN_HOST`（`"daemon"` または `"app"`）

### ライフサイクル

```text
Host                                      Plugin
  |--- initialize {hylaVersion, host, ...} --->|
  |<-- {name, version, capabilities, subs} ----|
  |--- initialized (notification) ------------>|
  |<== bidirectional active JSON-RPC calls ===>|
  |--- shutdown ------------------------------>|
  |<-- {ok: true} -----------------------------|
  |--- exit (notification) ------------------->|
  [grace wait -> SIGTERM/kill process group]
```

### エラーコード

| コード | 定数 | 意味 |
| --- | --- | --- |
| `-32700` | `ErrCodeParse` | JSON フレームが不正 |
| `-32600` | `ErrCodeInvalidRequest` | JSON-RPC の封筒が不正 |
| `-32601` | `ErrCodeMethodNotFound` | メソッド未実装 |
| `-32602` | `ErrCodeInvalidParams` | 引数が不正 |
| `-32603` | `ErrCodeInternal` | プラグインまたはホストの内部エラー |
| `-32001` | `ErrCodePermissionDenied` | 許可されたスコープがない |
| `-32002` | `ErrCodeRemoteControlDisabled` | `remoteControl` が切で `pane.focus` が止められた |
| `-32003` | `ErrCodeTargetNotFound` | セッション、ペイン、スロットがない |
| `-32004` | `ErrCodeCapabilityUnsupported` | 対象がその能力を持たない |
| `-32005` | `ErrCodeTimeout` | RPC の期限切れ |
| `-32006` | `ErrCodePayloadTooLarge` | フレームが 16 MiB を超えた |

### ホスト → プラグインの RPC

- **ライフサイクル**: `initialize`、`initialized`（通知）、`shutdown`、`exit`（通知）
- **エージェント**: `agent.detect`、`agent.readTimeline`、`agent.readChat`、`agent.sendChat`、`agent.readSubagents`、`agent.resumeCommand`
- **UI とアクション**: `ui.render`、`ui.event`、`action.invoke`、`webview.rpc`
- **イベント（`event.notify`）**: `session.state`、`pane.status`、`pane.osc`、`theme.changed`（`InitializeResult.subscriptions` で購読したもの）

### プラグイン → ホストの RPC

- **状態とターミナル**: `host.getState`、`pane.readScreen`、`pane.getEnv`、`session.create`、`pane.sendInput`、`pane.sendKey`、`pane.reportStatus`、`pane.focus`
- **UI と通知**: `ui.invalidate`、`ui.openPane`、`ui.notify`、`webview.emit`
- **設定とシークレット**: `plugin.getSettings`、`plugin.setSettings`

### Daemon RPC の拡張とプロトコル互換

Hyla は `proto.Version = 4` を保ち、`Welcome.Capabilities` に `"plugins.v1"` を載せます:
- `plugin.list` —— 実行時の状態（`[]rpc.PluginStatus`）
- `plugin.call` —— daemon のプラグインへの転送（`{pluginId, method, params}`）
- `plugin.reload` —— `hylad` 上でマニフェスト、パック、プロセスを読み直す
- `plugin.restart` —— サーキットブレーカを戻して `{pluginId}` を再起動
- `plugin.logs` —— プラグインのログの末尾（`{pluginId, lines}`）

`"plugins.v1"` を持たない古い `hyla-server` に繋ぐと、デスクトップはリモートの `plugin.*` 呼び出しを飛ばします。リモートホスト（`remoteControl == false`）では、リモートのプラグインはローカルのウィンドウ操作、セッション選択、フォーカス変更、ローカルでのペイン表示を行えません。

### プロセスの管理とサーキットブレーカ

- ふつうの `hylad` プラグイン（`keep_alive = false`）は `hylad` の 30 秒のアイドル終了を**止めません**。
- `keep_alive = true` で動いている有効なプラグイン（または有効な `ChannelDriver`）だけが `hylad` を残します。
- クラッシュが `max_crashes_per_minute`（既定は 60 秒に 5 回）を超えるとサーキットブレーカが働き、再起動されるまで `degraded` になります（`hyla plugin`、設定画面、`plugin.restart`）。

---

## 層 3A: 宣言的 UI の JSON

プラグインは `UIRenderResult` を返し、その中の `UINode` のツリーを Hyla のネイティブ UI エンジン（`internal/plugins/uihost`）が非同期に描きます。

### 対応するノード（23）

- **レイアウト**: `column`、`row`、`box`、`spacer`、`divider`、`scroll`、`list`
- **コンテナ**: `infoCard`、`settingsGroup`、`settingsRow`
- **表示**: `text`、`richText`、`badge`、`icon`、`image`、`spinner`、`progress`
- **操作**: `button`、`toggle`、`segmented`、`select`、`textInput`、`slider`

### 配色トークンと書体

`style.background`、`style.border`、`style.color`、`props.color`、text の tone では、色を `$token` または `$token@alpha`（例 `"$accent@0.2"`）で書き、テーマに合わせて解決されます:

- `$bg`、`$sidebar`、`$card`、`$chip`、`$popover`、`$control`
- `$text`、`$muted`、`$faint`、`$accent`、`$attention`、`$done`、`$danger`、`$divider`

`UIStyle` は `width`、`height`、`minWidth`、`minHeight`、`maxWidth`、`maxHeight`、`padding`（`[all]`、`[v,h]`、`[t,r,b,l]`）、`gap`、`radius`、`flex`、`align`（`start|center|end|stretch`）、`justify`（`start|center|end|between`）、`background`、`border`、`borderWidth`、`color`、`fontSize`、`fontWeight`、`monospace`、`tabular`、`wrap` を持ちます。

### マウント位置

| マウント | 場所 |
| --- | --- |
| `info.card` | 右の情報パネルのカード |
| `side.tab` | サイドパネルのファイル / 情報 / 変更と並ぶタブ |
| `pane` | ワークスペースのペイン全体（`proto.CreateParams{Plugin, PluginView, PluginTitle, PluginArgs}`） |
| `sidebar.menu` | 左サイドバーのセッションのコンテキストメニュー |
| `files.menu` | ファイルタブのファイル・フォルダのコンテキストメニュー |
| `pane.menu` | ペインのヘッダのコンテキストメニュー |
| `palette` | コマンドパレットのコマンド |
| `settings.plugins` | **設定 › プラグイン**のそのプラグインの引き出しの中の宣言的 UI |
| `settings.page` | 設定ウィンドウの左に独立した一ページ |
| `settings.<page>` | 組み込みの設定ページに足す宣言的 UI（`settings.general`、`settings.appearance`、`settings.terminal`、`settings.shell`、`settings.browser`、`settings.notifications`、`settings.keyboard`、`settings.feishu`、`settings.agents`、`settings.sync`、`settings.hosts`、`settings.advanced`） |

`pane` のセッションは `hylad` の再起動を越えて残ります（`savedPane` / `restorePane`）。プラグインが無効、不在、またはそのホストでインストール待ちなら、Hyla は閉じるボタン付きの「使えない」カードを描きます。

### 設定の拡張

プラグインは四つの方法で設定ウィンドウを拡張できます:

1. **構造化された `[[settings]]`**: `hyla-plugin.toml` に書く（`string`、`secret`、`boolean`、`integer`、`number`、`select`）。Hyla が **設定 › プラグイン**（宣言があればそのプラグインの最初の `settings.page` にも）に描き、秘密でない値は `~/.hyla/plugin-config/<id>.json` に、シークレットは資格情報ストアに保存し、各項目の `title`、`key`、`description` を設定検索に載せます（一致を選ぶとそのプラグインの引き出しが開きます）。
2. **`mount = "settings.plugins"`（引き出しの UI）**: `ui.slot` を持つ有効なプラグインが、**設定 › プラグイン**の詳細の引き出しに `settingsGroup` / `settingsRow` / `button` で操作できる宣言的 UI（状態表示や「接続を試す」ボタンなど）を描けます。
3. **`mount = "settings.page"`（独立した設定ページ）**: `ui.slot` を持つ有効なプラグインが、スロットの `title` と `icon`（プラグイン内の SVG、なければ既定のアイコン）で設定の左に一階層の入口を足します。設定検索とコマンドパレットからも届きます。プラグインが無効化・削除されると、設定はきれいに戻ります。
4. **`mount = "settings.<page>"`（組み込みページへの追記）**: `ui.slot` を持つ有効なプラグインが、任意の組み込み設定ページ（上と同じ一覧）に宣言的 UI の区画を足せます。

`settings.*` のマウントはすべて層 3A の宣言的 UI で描きます（`mode = "declarative"`、`ui.slot` が必要。設定のマウントは `mode = "webview"` 非対応）。`ui.invalidate` を呼ぶか、`ui.event` から `{root, invalidate, message}` を返すと、設定ウィンドウにその場で描き直し・通知ができます。

---

## 層 3B: サンドボックス WebView（`hylaplugin://`）

`mode = "webview"` のスロットは、`MainWindow` の作成前に登録された一つのカスタムスキーム `hylaplugin://<plugin-id>/...` で読み込まれます:

- プラグインごとに隔離された `Untrusted` の WebView セッション（`mygo.NewSession("plugin:" + id)`）。
- 厳しい CSP ヘッダが外部への遷移と接続を閉じます（`default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'`）。
- WebView 内のクロスオリジンの遷移は `OnWillNavigate`（`e.PreventDefault()`）で止めます。
- プラグインのオリジンに次の仮想エンドポイントが用意されます:
  - `/_hyla/theme.css` —— ライブな CSS 変数（`--hyla-bg`、`--hyla-sidebar`、`--hyla-card`、`--hyla-chip`、`--hyla-popover`、`--hyla-control`、`--hyla-text`、`--hyla-muted`、`--hyla-text-muted`、`--hyla-faint`、`--hyla-accent`、`--hyla-attention`、`--hyla-done`、`--hyla-danger`、`--hyla-divider`、`--hyla-font-ui`、`--hyla-font-mono`）。
  - `/_hyla/bridge.js` —— `window.hyla.rpc(method, params, slotId)` と `window.hyla.on(event, listener)` を注入。
  - `POST /_hyla/rpc` —— `mygo.CallerPage(r.Context())` で呼び出し元を認証し、`webview.rpc` をプラグインのホストプロセスに渡します。
- ホストからページへの push は `Event.EmitPage` の `"hyla:plugin"` チャネル（Go は `host.WebViewEmit`、TS は `host.webviewEmit`）。

---

## 公式 SDK

### Go SDK（`sdk/go/hylaplugin`）

`github.com/smallnest/hyla/sdk/go/hylaplugin` を import（`sdk/go/hylaplugin/go.mod`）:

```go
package main

import (
	"context"
	"log"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func main() {
	plg := &hylaplugin.Plugin{
		Name:         "hello-go",
		Version:      "0.1.0",
		Capabilities: []string{"ui.declarative"},
		OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
			st, err := host.GetState(ctx)
			if err != nil {
				return hylaplugin.UIRenderResult{}, err
			}
			return hylaplugin.UIRenderResult{
				Title: "Sessions",
				Root: hylaplugin.InfoCard("Workspace", "", false, []*hylaplugin.UINode{
					hylaplugin.Text("Active: "+st.ActiveSession, hylaplugin.WithKey("act")),
					hylaplugin.Button("refresh", "Refresh", "secondary", hylaplugin.WithKey("btn"), hylaplugin.WithOnClick("refresh")),
				}),
			}, nil
		},
	}
	if err := hylaplugin.Run(plg); err != nil {
		log.Fatal(err)
	}
}
```

単体テストには `hylaplugin.NewTestHarness(plg)`（`Call(ctx, method, params, &out)`、`RegisterHostHandler(method, fn)`、`Close()`）。エージェントのハンドラは `Timeline`（`Turns []TimelineStep`）、`Chat`（`Lines []ChatLine`）、`[]Subagent` を返し、Hyla のワイヤ形式に一致します。

### TypeScript / Bun SDK（`sdk/ts`）

依存ゼロの TypeScript SDK で、二つの入口を公開します:
- `@hyla/plugin-sdk`: `runPlugin`、`attachPluginPeer`、`RPCPeer`、`HostClient`、`ui.*` のビルダー、型付きの RPC ペイロード。
- `@hyla/plugin-sdk/web`: ブラウザ側の `rpc`、`onEvent`、`onThemeChanged`、`readThemeTokens`。

---

## 三つの実例

### 例 1: 層 1 の宣言的パック（テーマ + エージェント検出 + マーク）

```sh
hyla plugin init --template pack --id org.acme.aurora ./aurora-pack
cd aurora-pack
hyla plugin link .
```

`hyla-plugin.toml`（と `detect/*.toml`、`marks/*.svg`、`themes/*.ghostty`、`washes/*.toml`）を編集します。保存した内容は `hyla plugin dev .` か **設定 › プラグイン › 再読み込み**で反映されます。

### 例 2: 層 2 の Go エージェントと宣言的 UI

`sdk/go/hylaplugin/examples/hello-agent` に動く例があります:

```sh
cd sdk/go/hylaplugin/examples/hello-agent
go build -o hello-agent .
hyla plugin link . --grant-all
```

宣言しているもの:
- `hyla-plugin.toml` の `agent.provider`、`ui.slot`、`session.read`。
- `[[agents]]`（`processes = ["hello-agent"]`、`timeline = true`）。
- `[[ui.slots]]`（`mount = "info.card"`、`mode = "declarative"`、`host = "daemon"`）。

### 例 3: 層 3 の TypeScript とサンドボックス WebView のペイン

```sh
hyla plugin init --template ts-webview --id org.acme.inspector ./inspector-webview
cd inspector-webview
hyla plugin link . --grant-all
```

Hyla の **プラグイン**メニューかコマンドパレット（`Cmd+K`）で **Open inspector-webview** を実行します。`web/index.html` のページは `/_hyla/theme.css` で Hyla のライブテーマを引き継ぎ、`/_hyla/bridge.js` 経由で `await window.hyla.rpc("ping", { hello: "hyla" })` を呼び、`src/plugin.ts` の `webview.rpc` が答えいます。

---

## コマンドライン（`hyla plugin`）

```text
hyla plugin ls [--json]                         同梱・リンク済み・インストール済み・インストール待ちを一覧
hyla plugin init [--template pack|go|ts-webview] [--id <id>] [--name <name>] [dir]
hyla plugin pack <dir> [-o out.hylapkg]         プラグインのディレクトリを .hylapkg に固める
hyla plugin install <pkg.hylapkg|dir> [--grant-all] [--no-enable]
hyla plugin link <dir> [--grant-all]            開発用のフォルダをシンボリックリンクで登録
hyla plugin dev <dir> [--grant-all]             リンクし、ファイルの変更でホットリロード
hyla plugin enable <id>                         有効にして daemon に伝える
hyla plugin disable <id>                        無効にしてホストプロセスを止める
hyla plugin rm <id> [--purge]                   削除（--purge は設定・データ・ログも）
hyla plugin permissions <id> [grant|revoke <scope…>|--all]
hyla plugin logs <id> [-n N]                    ログの末尾（hylad がいればそれ経由、なければローカルのファイル）
```

---

## パッケージとセキュリティ（`.hylapkg`）

`hyla plugin pack` は決定的な `.hylapkg` の ZIP を作り、SHA-256 を `plugins.json` に残します。`InstallPackage` は次を強制します:
- `..` によるパス脱出、絶対パス、アーカイブ内のシンボリックリンクを拒否。
- ファイル数と展開後のバイト数の上限（zip 爆弾を防ぐ）。
- バイナリとフックスクリプトの `0755` の実行ビットを保つ。
- **コード署名は必須ではなく**、**OS のサンドボックスもなし**。安全はパスの検証、利用者の明示的な許可（`grantedPermissions`）、WebView のオリジンとセッションの隔離で担保します。

---

## マシン間の同期と同梱プラグイン

### 公式の同梱プラグイン

Hyla は既定で有効な公式プラグインを一つ同梱しています:
- **`hyla.magpie`**: Magpie のコーディングエージェント向けゲートウェイの指標を表示（`info.card` スロット + `Refresh Magpie Usage` パレットアクション + 設定できるゲートウェイ URL）。

Feishu は Hyla 自身の機能で、プラグインではありません。ボットは hylad で動き、**設定 › Feishu** で設定します。3.17.0 の `hyla.feishu` プラグインで設定していた場合は、その設定が残り、この版の初回起動で Feishu の設定へ移してからプラグインを削除します。

### マシン間の同期

Hyla の暗号化同期（`Snapshot` v1）に含まれるもの:
- `~/.hyla/plugins.json`（そのマシン固有の `linkPath` は除く）。同期されたプラグインが他のマシンにない場合、Hyla は外部のコードを勝手に取得せず、**`pendingInstall: true`** と **`enabled: false`** にします。
- `~/.hyla/plugin-config/<id>.json` の秘密でない設定。
- **`~/.hyla/plugin-secrets/` はスナップショットから厳密に除外**され、そのマシンから出ません。

---

## 困ったとき

- **プラグインが `degraded` になる**: `hyla plugin logs <id>` か **設定 › プラグイン › ログ**を見ます。直したら**再起動**を押すか、`hyla plugin disable <id> && hyla plugin enable <id>`。
- **権限エラー `-32001`**: `hyla plugin permissions <id>` で許可済みを確認し、`hyla plugin permissions <id> grant <scope>` か **設定 › プラグイン**で足します。
- **WebView が空、`window.hyla` が undefined**: `[[ui.slots]]` が `mode = "webview"` か、`[permissions]` に `"ui.webview"` があるか、`web/index.html` に `<script src="/_hyla/bridge.js"></script>` があるか、`web_root` が正しいかを確認します。
- **リモートホストでプラグインのアクションが出ない**: リモートの `hyla-server` が `"plugins.v1"` を名乗っているか確認します（プラグイン対応のビルドに更新します）。
