---
title: 配置
description: Hyla 的配置文件、配置项及其默认值。
---

# 配置

Hyla 把所有设置放在一个纯文本文件里。设置窗口（<kbd>⌘</kbd> <kbd>,</kbd>）会替你修改它，并保留你的注释和顺序；你手动编辑后，Hyla 也会自动重新加载。

## 配置文件

Hyla 按以下优先顺序读取：

1. `$HYLA_CONFIG`
2. `$XDG_CONFIG_HOME/hyla/config`
3. `~/.config/hyla/config`

它采用 Ghostty 的语法：每行一个 `key = value`，`#` 开头为注释，列表类的配置项写成多行重复。凡是含义与 Ghostty 相同的配置项，都沿用 Ghostty 的名字。

```ini
# ~/.config/hyla/config
font-size = 14
theme = light:GitHub Light Default,dark:Catppuccin Mocha
cursor-style = bar

env = EDITOR=nvim
env = HOMEBREW_NO_AUTO_UPDATE=1

keybind = cmd+shift+d=pane.split.down
```

文件里只需写你要改的项，其余都保持默认。

```sh
hyla config edit              # 在 $EDITOR 中打开
hyla config show              # 列出所有配置项及其当前值
hyla config check             # 报告未知的配置项和无效的值
```

## 通用

| 配置项                  | 默认值    | 说明                                                          |
| ----------------------- | --------- | ------------------------------------------------------------- |
| `new-session-directory` | `current` | 新会话的起始目录：`current`、`project` 或 `home`               |
| `session-name`          | `focused` | 未手动命名的会话如何取名：`focused`、`lead` 或 `folder`（[详情](./sessions.md#how-an-unnamed-session-is-named)） |
| `sidebar-rows`          | `detailed` | 侧边栏如何显示会话：`detailed` 在第二行显示状态消息，没有时显示分支和改动；`compact` 只占一行 |
| `focus-follows-mouse`   | `false`   | 指针移入哪个窗格，就聚焦哪个窗格                              |
| `confirm-close`         | `true`    | 关闭仍有程序在运行的窗格前先确认                              |
| `restore-sessions`      | `true`    | hylad 重启后（例如开机后）重新启动之前的会话                   |
| `resume-agents`         | `true`    | 会话重新启动时，恢复每个窗格中智能体的对话                    |
| `quit-ends-sessions`    | `false`   | 退出 Hyla（点关闭按钮、按 ⌘Q 或使用菜单）时，结束本机的全部会话及其中的程序并停止 hylad，与“退出并结束所有会话”相同；有程序在运行时，按 `confirm-close` 的设置先确认。其他机器上的会话继续运行（[详情](./sessions.md#退出)） |
| `session-history`       | `false`   | 在侧边栏中列出智能体已结束的对话，可恢复或归档（[详情](./sessions.md#ended-sessions)） |
| `macos-show-app-icon`   | `dock-and-menubar` | 在 macOS 上显示 Hyla 图标的位置：`dock-and-menubar`、`dock` 或 `menubar`。图标显示在菜单栏时，关闭窗口后 Hyla 会继续在那里运行。Windows 和 Linux 上始终显示在通知区域，关闭窗口后同样会继续运行 |

## 外观

| 配置项               | 默认值   | 说明                                                            |
| -------------------- | -------- | --------------------------------------------------------------- |
| `window-theme`       | `system` | 应用外观：`system`、`light` 或 `dark`                            |
| `language`           | `system` | 应用语言：`system`、`en`、`ja` 或 `zh-Hans`                      |
| `background`         | `petal`  | 窗口底色：`petal`、`mist`、`meadow`、`plain` 或 `dock`（Dock 的：Liquid Glass 加一层薄纱）           |
| `window-translucent` | `true`   | 让桌面透过窗口显示出来                                          |
| `window-opacity`     | `0.82`   | 开启半透明时窗口背景的覆盖程度，0 到 1                        |
| `window-blur`        | `true`   | 模糊半透明背景后的桌面；关闭后是普通透明                      |
| `window-material`    | `sidebar` | 模糊时窗口背景的材质：`sidebar`、`window`、`popover`、`plastic`（磨砂内容背景）、`glass`（Liquid Glass，和 Dock 一样）或 `glass-clear` |
| `sidebar-image`      |          | 左边栏会话后面的一张图片：高度为整个左边栏的 80%，垂直居中，靠左且与左边留一点距离；留空则不显示 |
| `sidebar-image-opacity` | `0.35` | 左边栏背景图的可见程度，0（不显示）到 1（完全不透明） |
| `pane-image`        |          | 终端卡片里的图片，在文字后面，替代窗口背景：与终端同宽，从顶部开始；留空则不显示 |
| `pane-image-opacity` | `0.6`   | 终端背景图的可见程度，0（不显示）到 1（完全不透明） |
| `pane-opacity`       | `0.82`   | 窗口半透明时，终端中显示多少窗口背景，0 到 1，与 `window-opacity` 相同：为 0 时直接看到桌面；文字始终不透明 |
| `pane-blur`          | `false`  | 模糊终端背后透出的内容；关闭后是普通透明 |
| `pane-material`      | `glass`  | 终端模糊开启时玻璃的材质：`glass`（Liquid Glass）、`plastic`（磨砂内容背景）或 `glass-clear` |
| `pet`                | `true`   | 小树蛙 Hyla：趴在窗口上沿，跟着你的会话                          |
| `pet-offset`         | `0.5`    | Hyla 站在窗口宽度的哪里，0 到 1；直接拖动即可                    |
| `pet-set`            | `hyla`   | 窗口上沿显示的宠物：`hyla` 小树蛙、`codex` 表示 Codex 应用当前启用的宠物，或 `~/.codex/pets` 中某个宠物目录的名字
| `pet-size`           | `1.5`    | Codex 宠物的显示高度，相对 Codex 应用的大小：0.5 到 3

## 终端

| 配置项                | 默认值  | 说明                                                                     |
| --------------------- | ------- | ------------------------------------------------------------------------ |
| `theme`               | `hyla`  | 终端配色：`hyla`、任意 Ghostty 主题，或 `light:<theme>,dark:<theme>`      |
| `font-family`         |         | 留空则使用 Hyla 自带的 JetBrains Mono Nerd Font；设为 `monospace` 则使用系统等宽字体；也可以填任何已安装的字体族 |
| `font-fallback`       |         | `font-family` 缺少字形时（如中文）依次使用的字体：`PingFang SC, Apple Color Emoji`。留空则由系统选择 |
| `font-size`           | `13`    | 单位为磅                                                                 |
| `font-weight`         | `400`   | `100` 到 `900`；Hyla 自带字体画得出 `200`（加细）与 `300`（细体）           |
| `line-height`         | `1.22`  | 字体默认行高的倍数                                                       |
| `font-ligatures`      | `true`  | 按字体的设计连写字符                                                     |
| `font-thicken`        | `false` | 加粗笔画，同 Ghostty                                                     |
| `minimum-contrast`    | `3`     | 文字与背景之间保持的最低对比度，从 `1`（不限制）到 `21`，让为深色背景设计的颜色在浅色背景上也清晰可读，同 Ghostty |
| `dim-unfocused-terminals` | `true` | 会话有多个窗格时，淡化未聚焦的终端                                  |
| `cursor-style`        | `block` | `block`、`bar` 或 `underline`；程序可以更改它                            |
| `cursor-style-blink`  | `true`  | 光标闪烁                                                                 |
| `macos-option-as-alt` | `false` | Option 键向程序发送 Alt（Meta）                                          |
| `copy-on-select`      | `false` | 选中内容即复制到剪贴板                                                   |
| `clipboard-read`      | `ask`   | 程序读取剪贴板（OSC 52）时：`ask`、`allow` 或 `deny`                     |
| `right-click-action`  | `context-menu` | `context-menu`、`paste` 或 `copy-or-paste`                        |
| `mouse-scroll-multiplier` | `precision:1,discrete:3` | 滚动的幅度，同 Ghostty：`precision:` 是触控板滚动的倍数，`discrete:` 是鼠标滚轮每一格滚动的行数（Windows 上指系统设为 3 行时的一格）；只写一个数字则同时设置两者，范围 `0.01` 到 `10000` |

## Shell

| 配置项             | 默认值  | 说明                                                           |
| ------------------ | ------- | -------------------------------------------------------------- |
| `command`          |         | 新窗格使用的 shell；留空则使用你的登录 shell                    |
| `env`              |         | 以 `NAME=value` 形式加入每个窗格的环境变量；可重复多行          |
| `scrollback-limit` | `4`     | 每个窗格保留的回滚缓冲大小，单位 MB；`0` 表示不保留             |

## 通知

| 配置项              | 默认值  | 说明                                           |
| ------------------- | ------- | ---------------------------------------------- |
| `notify-on-waiting` | `true`  | 会话需要你时                                   |
| `notify-on-done`    | `true`  | 智能体完成时                                   |
| `notify-on-error`   | `true`  | 程序报告失败时                                 |
| `notify-sound`      | `false` | 通知时播放提示音                               |
| `dock-badge`        | `true`  | 在 Dock 图标上显示有多少会话需要你             |
| `dock-bounce`       | `false` | 有会话开始需要你时，Dock 图标跳动一次          |

## 智能体

| 配置项           | 默认值  | 说明                                                |
| ---------------- | ------- | --------------------------------------------------- |
| `remote-control` | `true`  | 允许 `hyla do` 和 `hyla focus` 操控应用             |

## 主机

应用会显示这些机器上的会话，`hyla -H` 也能连到它们。参见[远程主机](./remote.md)。

| 配置项         | 默认值  | 说明 |
| -------------- | ------- | ---- |
| `host`         |         | 一台主机，写作 `name=address`：通过 SSH 连接时用 `me@devbox` 或 `ssh://…`，WSL 用 `wsl://distro`，hylad 的 tailnet 监听地址用 `http(s)://…`。在后面加上 ` remote-control` 可允许它的程序操控应用，加上 ` color=teal`（red、orange、amber、green、teal、blue、purple 或 pink）可为它的会话标色。每台主机写一行 |
| `listen`       |         | 允许其他机器上的 Hyla 连接本机：`tailnet://:7473` 监听本机的 Tailscale 地址，`tailnet-serve://` 使用 Tailscale Serve。每个地址写一行 |
| `listen-allow` |         | 除本机所有者外还允许谁连接：某个 tailnet 登录账号、`tag:name`，或用 `*` 允许 tailnet 上的所有人。每项写一行 |
| `remote-updates` | `false` | 允许你其他机器上的 Hyla 把这里的 Hyla 应用更新到它的版本，应用会随之重新启动 |

## Git

| 配置项        | 默认值  | 说明                                                                                              |
| ------------- | ------- | ------------------------------------------------------------------------------------------------- |
| `git-status`  | `true`  | 显示每个会话的分支和[改动](./changes.md)                                                          |
| `diff-layout` | `auto`  | 改动面板中的 diff 布局：`unified`、`split`（左右对照）或 `auto`（面板够宽时左右对照）             |
| `worktree-base` | `default` | [会话专属工作树](./changes.md#sessions-in-their-own-worktree)的起点：`default` 为仓库的默认分支，`current` 为你所在会话的分支 |
| `worktree-dir` |        | 会话专属工作树的存放位置，各自放在仓库名之下；留空则放在仓库内的 `.hyla/worktrees` |

<a id="browser"></a>

## 浏览器

| 配置项                | 默认值                     | 说明                                                                                |
| --------------------- | -------------------------- | ----------------------------------------------------------------------------------- |
| `browser-block-ads`   | `true`                     | 在浏览器窗格中[拦截广告和跟踪器](./panes.md#blocking-ads)                            |
| `browser-filter-list` | `easylist`、`easyprivacy`  | 用于拦截的过滤列表。每个列表写一行；留空则不使用任何列表                            |

`browser-filter-list` 可以填 Hyla 内置列表的名称，也可以填任意 Adblock Plus 语法列表的 `https://` URL：

| 名称               | 列表                                                                |
| ------------------ | ------------------------------------------------------------------- |
| `easylist`         | EasyList：广告                                                      |
| `easyprivacy`      | EasyPrivacy：跟踪器和统计分析                                       |
| `peter-lowe`       | Peter Lowe 列表：广告和跟踪服务器                                   |
| `easylist-china`   | EasyList China：中文网站上的广告                                    |
| `fanboy-cookie`    | EasyList Cookie List：Cookie 提示和同意横幅                         |
| `fanboy-social`    | Fanboy's Social Blocking List：社交媒体按钮和嵌入内容               |
| `fanboy-annoyance` | Fanboy's Annoyance List：弹窗和提示，并包含上面两个列表             |

```ini
browser-filter-list = easylist
browser-filter-list = easyprivacy
browser-filter-list = fanboy-cookie
browser-filter-list = https://example.com/my-filters.txt
```

<a id="keyboard"></a>

## 键盘

`keybind` 用于重新绑定操作，格式为 `<keys>=<action>`。每个绑定写一行。把操作设为 `unbind` 即可释放对应按键。

```ini
keybind = cmd+shift+d=pane.split.down
keybind = cmd+k=palette.open
keybind = cmd+b=unbind
```

运行 `hyla actions` 查看所有操作 id，默认快捷键见[键盘快捷键](./shortcuts.md)。
