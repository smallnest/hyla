---
title: 外观
description: 把窗口变成你的：半透明、模糊与材质、边栏和终端背后的图片、会话切换动画，以及字体。
---

# 外观

Hyla 的窗口是为了坐在你的桌面上，而不是挡在它前面。这些都在**设置 › 外观**里改，[配置文件](./configuration.md#appearance)里是同样的键。

## 半透明与材质

| 键                  | 作用                                                        |
| ------------------- | ----------------------------------------------------------- |
| `window-translucent` | 让桌面透过窗口显示                                        |
| `window-opacity`    | 窗口底色覆盖多少，0 到 1                                     |
| `window-blur`       | 模糊窗口后面的桌面；关闭就是纯透明                          |
| `window-material`   | 窗口用什么材质：`sidebar`、`window`、`popover`、`plastic`、`glass`（和程序坞一样的 Liquid Glass）或 `glass-clear` |
| `pane-opacity`      | 对终端卡片同理，终端可以比边栏更通透                         |
| `pane-blur`         | 模糊透过终端显示的内容                                       |
| `pane-material`     | 终端自己的玻璃：`glass`、`plastic` 或 `glass-clear`          |

文字永远是不透明的，底色怎么调都一样：调低透明度改变的是字周围的东西，不是字的对比度。

## 图片

工作背后可以放两张图：一张在边栏的会话后面，一张在终端卡片里。

| 键                     | 作用                                                     |
| ---------------------- | -------------------------------------------------------- |
| `sidebar-image`        | 边栏会话背后的图片                                       |
| `sidebar-image-opacity` | 它显示多少                                              |
| `pane-image`           | 终端里的图片，压在文字下面                                |
| `pane-image-opacity`   | 它显示多少                                                |
| `pane-image-fit`       | `fill`、`fit`、`stretch`、`center` 或 `tile`，和桌面图片的选项一样 |

## 切换会话

`session-switch` 决定下一个会话怎么进场：`dissolve` 让旧的淡出、新的淡入；`slide`、`push`、`cover` 从右侧滑入；`zoom` 放大就位；`none` 直接显示。

## 底色

`background` 是窗口铺的底色：`petal`、`mist`、`meadow`、`plain`，或者 `dock`——程序坞那种玻璃，上面盖一层薄纱。`window-theme` 跟随系统的浅色或深色，也可以固定其中一种。

## 字体

终端默认用 Hyla 自带的 JetBrains Mono Nerd Font，除非 `font-family` 指定别的。**设置**里的字体菜单列出这台机器上真正安装的等宽字体，而不是固定的几个，所以你自己装的字体也会在里面。

`font-weight` 从 100 到 900；字体没有 200 和 300 时，Hyla 自己画出来。其余的排版——`font-ligatures`、`font-thicken`、`line-height`、`minimum-contrast`——见[配置](./configuration.md#terminal)。
