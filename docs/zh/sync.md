---
title: 同步
description: 通过 WebDAV、S3 或 GitHub，用一句口令把一台电脑的 Hyla 配置同步到所有电脑上。
---

# 同步

Hyla 的配置——主题、字体、按键、智能体、飞书、Tailcat——就是一个文件。**同步** 让它在你用的每台电脑上保持一致，走一个它们都能访问的服务器：WebDAV 文件夹、S3 兼容的存储桶，或一个 GitHub 仓库。

快照在写入它的那台电脑上加密。服务器上只有密文：它读不到你的设置，别人拿到文件、没有口令也一样读不到。

## 同步什么

只有配置文件本身，`hyla config path` 指出的那一个。它记的是与默认值不同的部分，所以一台电脑同步之后，用的就是其它电脑的设置。

**不同步** 属于某台电脑的东西：会话和它们的终端、项目、边栏里的 SSH 与 K8s 主机、窗口尺寸、上次选中的会话。那些是这台电脑自己的。

## 存到哪里

| 服务器 | 要填的 |
| --- | --- |
| **WebDAV**——坚果云、Nextcloud、ownCloud | 地址（如 `https://dav.jianguoyun.com/dav/`）、用户名、应用密码 |
| **S3 兼容**——MinIO、R2、B2、OSS、COS | 服务地址、存储桶、前缀、区域、Access Key、Secret Key |
| **GitHub** | `owner/name`、路径（如 `hyla/config.enc`）、分支、Token——仓库请设为 **私有**，快照里虽然有你的密钥，但它们只是密文 |

在 WebDAV 地址下，Hyla 会建一个 `hyla/` 文件夹；存储桶与仓库里用的是 `hyla/config.enc`。同一个服务器可以放别的东西。

## 口令

一句口令，在每台电脑上输入，用来加密和打开快照。它经 Argon2id 推出密钥，内容用 AES-256-GCM 加密：口令错了，或者文件在服务器上被改过，一点都打不开。

**记住它。** 没有找回的办法：没有口令，文件在任何一台电脑上都打不开，Hyla 也打不开。

口令和服务器的凭据只留在各台电脑上，和配置放在一起的 `sync.json`（只有你自己可读）。它们从不进入同步内容，所以快照里不会有它自己的钥匙。

## 什么时候同步

- **打开 Hyla 时**：拉取一次，久未打开的那台电脑一开就是其它电脑的设置。
- **改动之后**：改了设置，半分钟后推送一次，把这一串改动合成一次。关掉 **自动同步** 就只在你要求时同步。
- **你要求时**：**设置 › 同步 › 立即同步**，或终端里的 `hyla sync`。

没有配置文件的电脑会取服务器上的那份，而不是把空的推上去。

## 两边都改过

同时改同一个设置并不常见，但会有。这时 **本机这份保留**：服务器那份另存为 `config.remote-<机器名>-<时间>`，然后推送覆盖服务器。你写的东西总会留一份，Hyla 会告诉你另一份放在哪。

## 命令行

```sh
hyla sync                    # 立即同步，和打开 Hyla 时一样
hyla sync status             # 同步到哪里、上次到哪一步
hyla sync push               # 把本机推送上去，不管服务器是什么
hyla sync pull               # 取服务器的，不管本机是什么
hyla sync set backend webdav # 还有 webdav.url、webdav.user、webdav.password…
hyla sync set passphrase "…" # 每台电脑上都一样
hyla sync set auto on
```

`hyla sync set` 支持 `backend`（`webdav`、`s3`、`github` 或 `off`）、`passphrase`、`auto`、`webdav.url|user|password`、`s3.endpoint|bucket|prefix|region|accessKey|secretKey`、`github.repo|path|branch|token`。
