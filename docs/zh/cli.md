---
title: 命令行
description: 用 hyla 命令行，从 shell 或脚本中操控 Hyla 的会话和窗格。
---

# 命令行

`hyla` 命令可以在任何 shell 中操控会话：你自己的、脚本的，或者智能体的。通过 **Hyla › 安装命令行工具…** 安装它。

## 目标

大多数命令都接受一个**目标**：一个会话或一个窗格。目标可以是：

- 一个 id，或它任意一个唯一的前缀
- 会话的名称
- 一个会话，此时指它当前聚焦的窗格

在 Hyla 中，`$HYLA_SESSION` 和 `$HYLA_PANE` 指向命令所在的会话和窗格，也是默认的目标。

## 会话和窗格

```sh
hyla ls                          # 列出所有会话及其窗格
hyla new -n api -d ~/code/api    # 新建会话，并打印它的 id
hyla new -- npm run dev          # 新建一个运行某条命令的会话
hyla split --down -- npm test    # 在当前窗格旁新建一个窗格
hyla split --url localhost:3000  # 新建一个显示网页的浏览器窗格
hyla rename api backend          # 重命名会话
hyla focus backend               # 在 app 中显示它
hyla close backend               # 结束会话（如果给的是窗格，则关闭该窗格）
hyla close fix-login --remove-worktree   # 同时删除它专属的工作树
hyla restart <pane>              # 重新启动已退出窗格的 shell
```

`hyla new` 的选项：

| 参数                        | 含义                                            |
| --------------------------- | ----------------------------------------------- |
| `-p`, `--project <name\|id>` | 放入哪个项目（默认按目录决定）                  |
| `-n`, `--name <name>`       | 为会话命名                                      |
| `-d`, `--dir <dir>`         | 在该目录启动（默认为当前目录）                  |
| `-f`, `--focus`             | 在 app 中显示它                                 |
| `-w`, `--wait`              | 等待它的命令结束，并以该命令的退出码退出        |
| `-u`, `--url <url>`         | 打开显示该网页的浏览器，而不是 shell            |
| `--worktree <branch>`       | 在专属的 git 工作树中、以新分支启动（[详情](./changes.md#sessions-in-their-own-worktree)） |
| `--base <ref>`              | 工作树分支的起点（默认：默认分支）              |

## 输入与读取

```sh
hyla send api "npm run migrate" --enter   # 输入文本，然后按回车
hyla send api -e 'y\n'                    # 解析 \n \r \t \e \xHH 转义
hyla key api ctrl+c                       # 按键
hyla read api -n 40                       # 屏幕上最后 40 行
hyla read api -a                          # 屏幕内容连同回滚缓冲
```

`hyla key` 支持 `enter`、`tab`、`escape`、`up`、`down`、`left`、`right`、`backspace`，以及 `ctrl+c`、`ctrl+d` 这类带修饰键的组合。

## 等待

```sh
hyla wait api                             # 等到它不再工作
hyla wait api --for waiting,error --timeout 10m
hyla prompt review "summarize the diff"   # 发送并回车，等待，然后打印屏幕
```

`hyla wait` 返回时会打印窗格的状态。`hyla prompt` 发送文本并回车，等到窗格不再工作，再打印它的屏幕。这是向另一个智能体提问最快的方式。

### 等待命令结束

像 `make test` 这样的命令不会显示为*工作中*，所以要等它跑完，就用 `--wait` 启动它。`hyla` 随后会以该命令自身的退出码退出：

```sh
if hyla split --down --wait -- make test; then
  echo "tests passed"
fi
```

对于一个已经在运行启动时所带命令的窗格，`hyla wait --exit` 起同样的作用：

```sh
pane=$(hyla split --down -- go test ./...)
# … 做些别的事 …
hyla wait "$pane" --exit --timeout 10m
echo "go test exited with $?"
```

命令结束后，窗格连同输出会保留下来，你可以用 `hyla read` 读取。如果 shell 是 `cmd`，则无法等待命令结束，因为它判断不出命令何时结束。

## 监听事件

`hyla events` 会在会话和窗格发生变化时立即打印出来，每个事件一行，直到你停止它。脚本可以借此同时跟踪所有智能体，而不必轮询 `hyla status`。

```sh
hyla events                               # 所有会话
hyla events api                           # 一个会话，或一个窗格
hyla events -p hyla                       # 一个项目的会话
hyla events --for waiting,error           # 只看进入这些状态的窗格
hyla events --json | jq -r .name          # 每行一个 JSON 对象
hyla events --for waiting --once          # 第一个事件之后就退出
```

```
12:04:31  api x78n9ss  working → waiting  Allow Bash: rm -rf dist?
12:04:40  api  renamed api → backend
```

| 事件 | 何时 | 字段 |
| --- | --- | --- |
| `pane.status` | 窗格的状态改变 | `pane`、`from`、`to`、`message`、`kind`、`agent`；变为 exited 时还有 `exitCode` |
| `command.exited` | 窗格启动时所带的命令结束，shell 继续运行 | `pane`、`exitCode` |
| `pane.created`、`pane.closed` | 窗格启动或关闭 | `pane` |
| `session.created`、`session.closed` | 会话启动或关闭 | |
| `session.renamed` | 会话改名 | `from`、`to` |
| `session.moved` | 会话移到另一个项目 | `from`、`to`（项目的 ID） |
| `notice` | 会话中的程序发出桌面通知 | `pane`、`title`、`body` |

加上 `--json` 时，每个事件还带有 `event`、`time`（unix 毫秒）、`session`、`project` 和 `name`（会话在侧边栏中显示的名字）。

`hyla events` 只报告从现在开始的变化；已有的内容请用 `hyla ls --json` 查看。指定目标时，目标关闭后它就会停止。`--once` 在第一个匹配的事件之后退出；如果目标先关闭或超过 `--timeout`，则以 `1` 退出。hylad 在状态稳定后才发送它，所以只持续一瞬间的状态（比如短暂的工作中）可能不会出现。

<a id="browsing"></a>
## 浏览网页

`hyla browser` 操控浏览器窗格中的网页，就像 `hyla send` 和 `hyla read` 操控终端一样：点击、输入、读取页面、截图。它的目标是一个浏览器窗格，或者一个会话（指其中的浏览器）。不指定目标时，它操控所在会话的浏览器。

```sh
hyla split --url localhost:3000            # 打开网页，并打印窗格的 id
hyla browser open localhost:3000/todos     # 前往某个网页，并等待加载完成
hyla browser type '#new-todo' "Buy milk" --enter
hyla browser wait '.todo-list li'          # 等到某个元素出现
hyla browser wait --text "1 item left"     # 等到某段文本出现
hyla browser text                          # 页面的文本
hyla browser text h1                       # 某个元素的文本
hyla browser click 'button[type=submit]'
hyla browser screenshot -o page.png
hyla browser eval 'document.title'
```

| 命令                            | 作用 |
| ------------------------------- | ---- |
| `open <url>`                    | 前往某个网页，并等待加载完成 |
| `back`, `forward`, `reload`     | 后退、前进或重新加载页面，并等待完成 |
| `text [selector]`               | 打印页面的可见文本，或第一个匹配元素的文本 |
| `html [selector]`               | 打印页面的 HTML，或该元素的 HTML |
| `click <selector>`              | 像鼠标一样点击该元素 |
| `type <selector> <text…>`       | 向输入框输入文本，替换原有内容；`--enter` 会在之后按回车，从而提交所在表单 |
| `wait [selector] [--text text]` | 等到元素或文本出现；两者都不给时，等到页面加载完成 |
| `eval <js…>`                    | 在页面中运行 JavaScript 并打印结果：字符串原样输出，其他值输出为 JSON |
| `screenshot [-o file.png]`      | 保存页面截图，并打印保存位置；`-o -` 会写到标准输出 |

选择器是 CSS 选择器，在 shell 中记得加引号。需要元素的命令会等它出现，`--timeout`（默认 `10s`）设定这些命令以及页面加载最多等多久。`type` 和 `eval` 接受 `-`，表示从标准输入读取文本。

点击和输入发送的是真实鼠标和键盘会产生的事件，所以 React 等框架构建的页面也能收到。截图需要窗格显示在屏幕上：先用 `hyla focus` 显示它。其余命令对任何浏览器窗格都有效，无论是否显示。

和 `hyla do` 一样，`hyla browser` 需要开启 **设置 › 智能体** 中的 **远程控制**，并且 app 处于打开状态。

## 过去的对话

```sh
hyla history                     # 窗格中运行过的智能体对话，最近运行的在前
hyla history -p hyla -n 50       # 某个项目的对话，最多 50 个
hyla history login --json        # 与 “login” 匹配的对话，输出 JSON
hyla resume 13b9                 # 恢复一个对话：用 ID、ID 的前缀或名字
hyla resume fix-login -f         # 恢复并在 app 中显示
```

`hyla resume` 会在对话的目录和项目中启动一个恢复该对话的会话，并打印会话的 ID。如果某个窗格仍在运行这个对话，则返回那个会话。参见 [过去的对话](./sessions.md#past-conversations)。

## 状态

```sh
hyla status api --json                    # 状态、程序、目录
hyla report working "Running migrations"  # 报告自己的状态
hyla notify "Need your input"             # 将此窗格标记为需要你处理
```

各状态的含义见 [状态与通知](./status.md)。

## 改动

```sh
hyla diff                                 # 窗格中未提交的改动，格式同 git diff
hyla diff --turn                          # 它的智能体在上一轮中改了什么
hyla diff review --branch                 # 自默认分支以来的改动
```

见 [改动](./changes.md)。

## 配置与设置

```sh
hyla config path                 # 配置文件所在位置
hyla config show                 # 每个配置项及其值
hyla config show --defaults      # 每个配置项的默认值
hyla config edit                 # 在 $EDITOR 中打开它
hyla config check                # 报告未知的配置项和无效的值
hyla projects                    # 列出项目
hyla skill setup                 # 教编程智能体使用 hyla
hyla daemon status|stop|restart  # 管理 hylad
hyla daemon install-service      # 让 hylad 随系统启动（systemd、launchd）
hyla daemon serve                # 在前台运行 hylad，供服务管理器使用
hyla clients                     # 查看谁连接着 hylad
hyla pets                        # 窗口上沿的宠物，当前显示的排第一
hyla pets use <名字|hyla|codex>  # 显示某个宠物：~/.codex/pets 中的，或 Hyla 安装的
hyla pets install <包.zip>       # 安装 Codex 宠物包，即 .codex-pet.zip
hyla pets rm <名字>              # 删除 Hyla 安装的宠物
```

## 飞书

```sh
hyla feishu setup                # 输入应用的 ID 和 secret，保存并连接，给出下一步
hyla feishu status               # 机器人是否已连接
hyla feishu test                 # 检查飞书是否认得这个应用
hyla feishu send <会话> <文字…>   # 按会话 ID 发送一条消息
```

飞书会话可以驱动它绑定的 Hyla 会话里的编程智能体：`/new` 新建一个，`/use` 绑定一个已有会话，其他文字作为提示发送。见 [飞书](./feishu.md)。

## 其他机器

另一台机器就是 SSH 项目里的一个会话：在边栏的 **+** 菜单里加进来，`ssh` 跑在窗格里，CLI 像对待其他会话一样驱动它（`hyla ls` 列出它，`hyla send` 往里面输入）。在窗格里，`hyla` 与窗格所连的 hylad 通信。

这台机器作为主机，凭配对码被别人点对点连上：

```sh
hyla p2p status                  # 开没开、端口、配对码
hyla p2p enable 7400             # 在 7400 端口上监听
hyla p2p port 7473               # 换端口
hyla p2p rotate                  # 换一把密钥，换一个配对码
hyla p2p code                    # 只打印配对码（机密）
hyla p2p disable
```

`hyla tailcat` 是同一条命令，用的是承载它的库的名字：两台机器之间走 WireGuard，中继只负责找到彼此。

手机：

```sh
hyla pocket code                 # HylaPocket 连接用的配对码
hyla pocket qr                   # 同一个码，二维码形式
hyla pocket tcp enable           # 改用本网络的 tcp:// 监听，
hyla pocket addr                 #   需要密码：地址
hyla pocket token                #   和密码
```

## 执行 app 操作

`hyla do` 可以执行 app 的任意操作，与菜单中的操作相同。它需要 **远程控制**，该选项在 **设置 › 智能体** 中默认开启。

```sh
hyla actions                     # 列出所有操作
hyla do pane.split.right
hyla do pane.browser url=localhost:3000
hyla do sidebar.toggle
```

## 帮助与补全

```sh
hyla help                        # 按组列出命令
hyla help new                    # 一条命令接受什么
hyla new --help                  # 同上
hyla help all                    # 所有命令及其参数
hyla help --markdown             # 以 Markdown 输出的参考
```

写错的命令会直接说明并以 `2` 退出，而不是去打开应用。

补全脚本由同一张命令表生成：

```sh
hyla completions zsh  > "${fpath[1]}/_hyla"
hyla completions bash > /etc/bash_completion.d/hyla
hyla completions fish > ~/.config/fish/completions/hyla.fish
```

## 退出码

| 退出码 | 含义                           |
| ------ | ------------------------------ |
| `0`    | 成功                           |
| `1`    | 失败                           |
| `2`    | 用法错误                       |
| `3`    | 目标不存在，或有歧义           |
| `4`    | 无法连接 hylad                 |

等待程序的命令，即 `hyla wait --exit` 和 `--wait`，改为以该程序的退出码退出；当窗格的 shell 以错误退出时，`hyla wait` 也是如此。等待超时则以 `1` 退出。

给 `ls`、`status` 和 `projects` 加上 `--json`，可得到便于程序处理的输出；`status` 再加 `--no-header`，则只输出一行、不带列名。
