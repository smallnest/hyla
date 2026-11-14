---
title: Hyla Pocket
description: 用 Android 手机读取并操作这台机器的会话：通过 Tailcat，或者地址加密码。
---

# Hyla Pocket

**HylaPocket** 是 Hyla 的 Android 客户端。它把跑在这台机器上的会话放进手机屏幕：同样的会话、同样的窗格、同样的程序。

![Android 手机上的 HylaPocket：打开着的会话](./images/pocket-session.jpg)

*同一批会话，在手机上：HylaPocket 读的是这台机器 hylad 上的内容。*

## 连接

手机用两种方式之一连上这台机器的 `hylad`：

- **Tailcat 配对码。** 这台机器在**设置 › 远程**里显示配对码；在手机上输入或扫描它，两者就能找到彼此——不需要账号，网络允许直连时中继也不在中间。见[远程机器](./remote.md#tailcat)。
- **地址加密码。** 在**设置 › 远程**里打开网络监听，把显示的地址、端口和密码填进手机。只要手机能访问到这台机器就行，不需要装别的东西。

手机和电脑要说同一种协议：版本对不上时，手机会直接说明，而不是打开它读不懂的会话。应用提示时更新 APK 即可。

## 它能显示什么

- **每个会话**，实时，带着各自的状态和里面的窗格。
- **终端本身**，不是转录：程序的屏幕，变了就重画。
- **屏幕下方的键行**，补上触屏键盘发不出的键：<kbd>esc</kbd>、<kbd>tab</kbd>、<kbd>ctrl</kbd>、<kbd>alt</kbd> 和方向键，按住即当修饰键用。
- **分屏**：一个会话里既有智能体又有测试，手机上也能都看到。

再打开应用会接着上次的位置：手机是会话的一扇窗口，不是它的副本，关掉它什么也不会变。

## 安装

从[发布页](https://github.com/smallnest/hyla-pocket/releases/latest)下载已签名的 APK，在手机上打开；Android 第一次会要求允许从此来源安装。需要 Android 9 或更高版本。

客户端是一个独立项目：[smallnest/hyla-pocket](https://github.com/smallnest/hyla-pocket)。
