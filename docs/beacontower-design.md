---
title: BeaconTower（烽火台）加密中继与 Hyla 远程连接设计文档
description: 高性能、零信任端到端加密（E2EE）自建中继服务器 BeaconTower 及其在 Hyla 远程连接中的完整集成设计。
---

# BeaconTower（烽火台）加密中继与 Hyla 远程连接设计文档

## 1. 背景与目标

当前 Hyla 已有两种跨机连接方式：
- **P2P (`tcp://`)**：局域网或公网直连，速度快但受 NAT / 防火墙限制，且无传输加密。
- **Tailcat (`tailcat://`)**：穿透能力强，但依赖 Tailscale 公共 DERP 节点（海外节点、限速），且 Hyla 客户端每次 `Dial()`（控制通道及每个终端窗格的 `Attach`）都要独立拉起一套完整的 `tailcat.Client`（DERP Map 拉取 + netcheck + WireGuard + gVisor 用户态协议栈），首连与开窗格延迟高。

**BeaconTower（烽火台）** 的核心设计目标：
1. **极低延迟、高吞吐**：自建单二进制中继服务器，数据面仅做零解密、零业务解析的 `io.CopyBuffer` 转发；客户端针对同一台远端主机只建立 **1 条持久 E2EE 加密隧道**，内部多路复用承载控制 RPC 和全部终端窗格流，新开/切换终端窗格无需重新握手。
2. **零信任端到端加密（E2EE）**：中继服务器天然不可信——即使部署在第三方 VPS 上或采用无域名裸 TCP 监听，中继也**无法解密、篡改或伪造**任何终端输出、键盘输入与 RPC 报文。
3. **零额外第三方依赖、单二进制部署**：完全基于 Go 标准库加密原语与 `go.mod` 已有依赖实现，独立可执行文件 `beacontower` 可零依赖运行在任意 Linux / macOS / Windows VPS 上。
4. **单配对码即连、与 Hyla 一等集成**：配对码内嵌中继地址、主机公钥、预共享密钥与可选中继访问令牌；在 **设置 › 远程连接** 中与 Tailcat、P2P 并列展示（顺序为 **BeaconTower → Tailcat → P2P**），并提供独立 CLI `hyla beacontower`（别名 `hyla beacon`）。

---

## 2. 整体架构

```text
  ┌────────────────────────┐                         ┌────────────────────────┐
  │   Client (Hyla App)    │                         │    Host (hylad)        │
  │  ┌──────────────────┐  │                         │  ┌──────────────────┐  │
  │  │ control / attach │  │                         │  │ d.serve / pocket │  │
  │  └────────┬─────────┘  │                         │  └────────▲─────────┘  │
  │  ┌────────▼─────────┐  │                         │  ┌────────┴─────────┐  │
  │  │ Stream Mux (池化) │  │                         │  │    Stream Mux    │  │
  │  └────────┬─────────┘  │                         │  └────────▲─────────┘  │
  │  ┌────────▼─────────┐  │                         │  ┌────────┴─────────┐  │
  │  │ E2EE AEAD Layer  │◄─┼──── 端到端加密 (中继盲) ───┼─►│ E2EE AEAD Layer  │  │
  │  │ X25519+PSK+ChaCha│  │                         │  │ X25519+PSK+ChaCha│  │
  │  └────────┬─────────┘  │                         │  └────────▲─────────┘  │
  └───────────┼────────────┘                         └───────────┼────────────┘
              │ CONNECT(hostID)                      ① 长连接 REGISTER(hostID)
              │                                      ② 收到 INVITE(sessionID) 后
              │                                         主动拨号 JOIN(sessionID)
              ▼                                                  ▲
  ┌──────────────────────────────────────────────────────────────┴────────────┐
  │                     BeaconTower Relay Server (VPS)                        │
  │  同端口自动识别: Native TCP/TLS ("BT01") + WebSocket (/ws) + /healthz      │
  │  控制面: Host 注册表 (Challenge-Response 防劫持) + Invite 配对 (15s TTL)    │
  │  数据面: 双向 io.CopyBuffer (32 KiB 缓冲池, TCP_NODELAY, 零业务解密)         │
  └───────────────────────────────────────────────────────────────────────────┘
```

### 性能对比

| 维度 | Tailcat | BeaconTower |
|---|---|---|
| 中继节点 | Tailscale 公共 DERP（海外/限速） | 自建就近低延迟 VPS |
| 协议栈开销 | 用户态 WireGuard + gVisor netstack | 原生内核 TCP + ChaCha20-Poly1305 AEAD 帧 |
| 首连握手 | DERP 查询 + netcheck + WG + TCP（约 2~8s） | 1 次控制唤醒 + 1-RTT E2EE 握手（约 3×RTT） |
| 后续终端窗格 `Attach` | 每个窗格重连整套 `tailcat.Client` | **复用已建 E2EE 多路复用隧道（1×RTT，免握手）** |

---

## 3. 核心协议与加密设计

### 3.1 同端口双传输模式（Native TCP/TLS + WebSocket）

服务端在同一监听端口（默认 `:7474`）上接受连接并窥探前 4 字节（超时 10s）进行自动分流：
- **Native 二进制模式（魔数 `"BT01"`）**：客户端或主机建立连接后立即发送 `BT01`，服务端开启 `TCP_NODELAY` 并进入控制协议。支持裸 `host:7474`、`tcp://host:7474`、`tls://host:7474`。
- **HTTP / WebSocket 模式（前 4 字节为 `"GET "`/`"HEAD"` 等 HTTP 方法前缀）**：交由内置 HTTP 服务处理：
  - `/ws`（可通过 `-ws-path` 自定义）：升级为 WebSocket 二进制流并跑相同的 BeaconTower 控制与中继协议，支持 `ws://`、`wss://`，可直接放置在 Caddy / Nginx / CDN 的 443 端口之后。
  - `/healthz`：返回健康状态 JSON（`{"ok":true,"hosts":N,"sessions":M}`），不泄露任何 HostID 或敏感配置。

### 3.2 中继控制面协议与防劫持注册

控制帧格式：`[1B type][2B big-endian length][payload]`（单帧载荷上限 4096 字节）。

| 类型 | 方向 | 载荷 | 说明 |
|---|---|---|---|
| `0x01 SERVER_HELLO` | Relay → Peer | `[32B relayEphPub][32B nonce][1B flags]` | 连接建立即下发服务端临时公钥与挑战随机数，`flags & 1` 表示中继开启令牌鉴权 |
| `0x02 REGISTER` | Host → Relay | `[32B hostPub][32B proof]` | 被控端注册长连接，防劫持证明持有 `HostPriv` |
| `0x03 CONNECT` | Client → Relay | `[16B targetHostID][32B tokenProof]` | 控制端请求连接到指定主机 |
| `0x04 INVITE` | Relay → Host | `[16B sessionID]` | 中继通过控制长连接通知主机建一条数据通道 |
| `0x05 JOIN` | Host → Relay | `[16B sessionID][32B tokenProof]` | 主机另起新连接认领该 `sessionID` |
| `0x06 OK` | Relay → Client/Host | 空 | 双方配对完成，随后进入纯字节流直通桥接 |
| `0x07 ERR` | Relay → Peer | UTF-8 错误信息 | 鉴权失败、主机离线或配对超时 |
| `0x08/0x09 PING/PONG` | 双向 | 8B 时间戳/随机数 | 每 20s 心跳保活，60s 无响应断开 |

身份与鉴权计算方式：
- **`HostID`**：`SHA256("bt-host-id-v1" || HostPub)[:16]`（128-bit 路由标识，**中继不知道 `PSK`**）。
- **`tokenDigest`**：若配置了 `-token`，计算 `SHA256("bt-tok-v1" || token)`；未配置则为全 0 的 32 字节（从不明文在网络上传输令牌本身）。
- **防劫持注册签名（`RegisterProof`）**：
  - `ss = X25519(HostPriv, RelayEphPub)`
  - `proof = HMAC-SHA256(ss, "bt-reg-v1" || nonce || tokenDigest)`
  - 只有真实持有 `HostPriv` 与正确令牌的主机才能算出合法 `proof`，攻击者即便窃得 `HostID` 也无法顶替注册。
- **客户端连接 / 主机 Join 令牌证明**：
  - `ClientTokenProof = HMAC-SHA256(tokenDigest, "bt-cli-v1" || nonce || hostID)`
  - `JoinTokenProof = HMAC-SHA256(tokenDigest, "bt-join-v1" || nonce || sessionID)`
- **资源防泛洪限制**：`sessionID` 有效期 15s；单主机最多积压 64 个待认领 `INVITE`；服务端总主机数与总并发会话数分别受 `-max-hosts` 与 `-max-sessions` 限制；桥接使用 32 KiB `sync.Pool` 缓冲区做双向 `io.CopyBuffer`。

### 3.3 端到端加密（E2EE）1-RTT 握手与 AEAD 记录层

在中继配对并返回 `OK` 之后，Client 与 Host 在桥接通道上直接执行 **1-RTT Noise_IKpsk2 风格认证密钥协商**：

1. **Client → Host (`E2E_INIT`)**：
   - 生成临时 X25519 密钥对 `(Ce_priv, Ce_pub)`。
   - 计算静态-临时共享秘密 `dh_es = X25519(Ce_priv, HostPub)`。
   - 派生握手密钥 `hsKey = HKDF-SHA256(salt=PSK, ikm=dh_es, info="bt-e2e-hs-v1" || HostPub || Ce_pub, len=32)`。
   - 使用 ChaCha20-Poly1305（`nonce=0`，`AAD="bt-e2e-init-v1" || HostPub || Ce_pub`）加密 `[8B unix_sec][16B client_nonce]`。
   - 发送 `[1B ver=1][32B Ce_pub][2B ctLen][ct]`。
2. **Host 校验与响应 (`E2E_RESP`)**：
   - 用相同方式计算 `dh_es` 与 `hsKey` 并解密 `E2E_INIT`；若 `PSK` 不匹配或报文被篡改，AEAD 验证失败立刻关闭连接。
   - 校验时间戳 `|now - unix_sec| <= 300s`，并校验 `Ce_pub` 未出现在最近 10 分钟防重放缓存（`ReplayCache`）中。
   - 生成服务端临时 X25519 密钥对 `(He_priv, He_pub)`，计算双向前向安全秘密 `dh_ee = X25519(He_priv, Ce_pub)`（**PFS 前向保密**）。
   - 派生双向独立会话密钥：
     - `prk = HKDF-Extract(sha256, ikm = dh_es || dh_ee, salt = PSK)`
     - `tr = SHA256("bt-e2e-tr-v1" || HostPub || Ce_pub || He_pub)`
     - `key_c2h = HKDF-Expand(prk, "bt-c2h" || tr, 32)`
     - `key_h2c = HKDF-Expand(prk, "bt-h2c" || tr, 32)`
   - 用 `key_h2c`（`seq=0`，`AAD="bt-e2e-resp-v1" || tr`）加密回显 `[16B client_nonce]`，返回 `[1B ver=1][32B He_pub][2B ctLen][ct]`。
3. **AEAD 加密数据记录层**：
   - 自 `seq=1` 起传输多路复用流数据，每条记录格式为 `[4B big-endian ctLen][ChaCha20-Poly1305 ciphertext]`，明文上限 32 KiB。
   - Nonce 为 `[4B 0x00][8B big-endian seq]`，AAD 为 `[1B direction][8B big-endian seq]`，双向各自单调递增，彻底防止重放、乱序与截断攻击。

### 3.4 隧道内信用窗口流多路复用（Stream Mux）

在 E2EE `net.Conn` 内运行轻量化多路复用器（避免引入重型第三方依赖及单一终端刷屏阻塞控制通道）：
- **帧格式**：9 字节定长头 `[1B type][4B streamID][4B length/window]` + 可选载荷（`DATA` 单帧上限 16 KiB）。
- **帧类型**：`OPEN(target)`、`OPEN_OK`、`OPEN_ERR`、`DATA`、`WINDOW`、`FIN`、`RST`、`PING`/`PONG`。
- **独立流控**：每条逻辑子流初始接收窗口 256 KiB；接收端消费累计达 64 KiB 后自动发送 `WINDOW` 信用回补；如果某子流超出窗口未及时读取，对端会在本地阻塞写该流，但完全不影响同一隧道中控制 RPC 或其他终端窗格的实时交互。
- **目标服务路由**：`OPEN` 载荷携带服务名——`"hyla"` 对应 `hylad` 控制与终端流协议，`"pocket"` 用于承载 HylaPocket 请求。
- **客户端连接池与 Singleflight**：客户端按规范配对码缓存复用的 `*Mux` 会话，同一时刻对同一主机并发发起的多个 `Dial()` 共享同一次中继+E2EE 握手，断线时自动淘汰并重建。

### 3.5 配对码编码格式（内嵌中继令牌）

配对码使用紧凑 TLV 二进制序列化后以无填充 Base64URL 编码，加上版本前缀 `bt1_`：
- **裸配对码**：`bt1_<base64url>`
- **完整地址**：`beacontower://bt1_<base64url>`
- **扫码连接 URI（HylaPocket 与桌面端互通）**：`hyla://pair?code=bt1_<base64url>&name=<hostname>`
- **内部布局**：
  `[1B version=1][32B HostPub][32B PSK][2B relayLen][relayBytes][2B tokenLen][tokenBytes]`

特性：
- **一键即连**：将中继地址和可选中继令牌一起封入配对码，控制端粘贴配对码或扫码即可直连，无需重复输入中继 URL 和中继密码。
- **密码级敏感度**：配对码同时包含端到端 `PSK` 与可选中继接入令牌，仅分享给受信任设备；随时可通过设置页面或 `hyla beacontower rotate` 一键轮换 `HostPriv` 与 `PSK` 使旧配对码立即作废。

---

## 4. BeaconTower 独立中继服务端 (`cmd/beacontower`)

一键从 GitHub Releases 下载并校验安装（Linux / macOS，包含 `linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`，Windows 提供 `.zip`）：

```sh
curl -fsSL https://github.com/smallnest/hyla/releases/latest/download/install-beacontower.sh | sh
```

也可零依赖从源码独立编译，不依赖 GUI、`mygo`、`tailscale` 或 `sqlite`：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o beacontower ./cmd/beacontower
```

### 命令行参数

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `-listen` | `BEACONTOWER_LISTEN` | `:7474` | 监听地址 |
| `-token` | `BEACONTOWER_TOKEN` | 空（不鉴权） | 可选中继访问令牌 |
| `-tls-cert` / `-tls-key` | - | 空 | 启用内置 TLS 1.3+（支持 `tls://` 与 `wss://`） |
| `-ws-path` | - | `/ws` | WebSocket 路径 |
| `-max-hosts` | - | `1024` | 最大同时在线主机数 |
| `-max-sessions` | - | `4096` | 最大并发数据会话数 |
| `-version` | - | - | 打印版本并退出 |

### 常见部署方式

1. **裸 VPS 最简部署（无域名、无需证书，端到端加密仍全程生效）**：
   ```sh
   ./beacontower -listen :7474 -token your-relay-secret
   ```
   Hyla 中继地址填：`your-vps-ip:7474`
2. **直连 TLS 部署**：
   ```sh
   ./beacontower -listen :7474 -tls-cert fullchain.pem -tls-key privkey.pem -token your-relay-secret
   ```
   Hyla 中继地址填：`tls://relay.example.com:7474` 或 `wss://relay.example.com:7474/ws`
3. **置于 Caddy / Nginx 反向代理之后（复用 443 HTTPS 穿透严苛防火墙）**：
   ```sh
   ./beacontower -listen 127.0.0.1:7474 -token your-relay-secret
   ```
   Caddyfile 中转发 `reverse_proxy /ws 127.0.0.1:7474`，Hyla 中继地址填：`wss://relay.example.com/ws`

---

## 5. Hyla 集成设计

### 5.1 配置文件与本机密钥
- **`~/.config/hyla/config`**：
  - `listen = beacontower://`（与 `tailcat://:7473`、`tcp://:7473` 相互独立，可同时启用）
  - `beacontower-relay = relay.example.com:7474`
  - `beacontower-token = your-relay-secret`
  - `beacontower-rotate = false`
- **本机密钥文件**：`~/.hyla/beacontower.json`（权限 `0600`，原子写入），存储 `{ "key": "...", "psk": "..." }`。
- **远端主机列表 `~/.hyla/app.json`**：沿用 `hosts.Remote`，以 `Addr: "beacontower://bt1_..."` 保存远端 BeaconTower 主机。

### 5.2 守护进程 (`internal/daemon/listen.go`) & 客户端 (`internal/app/host`)
- `remote.ParseListen` 支持 `beacontower://`；`listeners.follow` 监听 `BeaconTowerRelay`、`BeaconTowerToken`、`BeaconTowerRotate` 变更并热重启对应监听。
- `host.Host` 新增 `BT bool`，`Dialer()` 路由至 `beaconTowerDialer` 并通过共享池按配对码开流；删除/重连主机时主动关闭池中隧道。

### 5.3 设置 UI（**设置 › 远程连接**）
按 **BeaconTower → Tailcat → P2P** 顺序排列：
- 中继服务器地址输入框与可选中继访问令牌输入框；
- 已添加的 BeaconTower 远端机器列表与配对码快捷添加栏；
- **提供 BeaconTower 服务** 开关，开启后生成并展示本机配对码 `beacontower://bt1_…` 与二维码，支持复制及二次确认更换配对码；
- 支持设置搜索与 `en` / `zh-Hans` / `ja` 三语界面。

### 5.4 CLI (`hyla beacontower` / `hyla beacon`)
```sh
hyla beacontower status                            # 查看运行状态、中继地址与本机配对码
hyla beacontower enable [relay-url] [--token ...]  # 开启本机 BeaconTower 监听
hyla beacontower disable                           # 关闭监听
hyla beacontower relay <url> [--token ...]         # 修改中继地址或令牌（--clear-token 清除令牌）
hyla beacontower code [--qr]                       # 打印配对码（--qr 输出终端二维码）
hyla beacontower rotate                            # 轮换密钥并生成新配对码
```
同时 `hyla daemon status` 与 `hyla daemon serve --listen beacontower://` 均原生支持 BeaconTower。
