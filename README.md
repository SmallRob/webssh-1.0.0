# WebSSH

## 项目简介

WebSSH 是一个基于Go(后端)和Vue2(前端)的Web端远程连接工具，集成SFTP文件管理，
支持 **SSH 终端**、**RDP 远程桌面**、**VNC 远程桌面** 三种连接模式。
* 连接界面
![image](https://github.com/user-attachments/assets/b2e5ccae-4be5-47ff-b1d0-3c1654a72486)

* 终端和sftp管理页面
![image](https://github.com/user-attachments/assets/c0ee38c2-e336-4ec6-a845-2e527062b20c)


## 功能介绍

- **Web 终端**：通过浏览器直接连接远程服务器，支持一键生成快捷链接。
- **RDP 远程桌面**：浏览器内直连 Windows 远程桌面，含鼠标键盘、剪贴板互通与 Ctrl+Alt+Del。
- **VNC 远程桌面**：浏览器内连接任意 VNC 服务器，支持 VNC Auth / VeNCrypt 等安全类型与 Tight/ZRLE 编码。
- **多种认证方式**：支持密码和密钥两种 SSH 登录方式。
- **文件管理**：支持远程文件的上传、下载与浏览（SSH/SFTP）。
- **多标签页**：可同时管理多个连接会话。
- **主题切换**：支持明暗主题自由切换。
- **初始命令**：支持登录后自动执行指定命令。
- **安全认证**：可选开启 Web 端登录认证。

## 三种连接模式

| 模式 | 入口 | 默认端口 | 浏览器侧渲染 | 后端通道 |
|---|---|---|---|---|
| SSH | `/terminal` | 22 | xterm.js | `x/crypto/ssh` 会话 + PTY |
| RDP | `/rdp` | 3389 | IronRDP (WebAssembly) | RDCleanPath 代理 + TLS 中继 |
| VNC | `/vnc` | 5900 | noVNC (原生 ESM) | RFB 透明字节中继 |

在登录页选择协议即可，端口与表单字段会按协议自动切换；「生成链接」按钮同样支持三种协议。

### 为什么 RDP / VNC 需要服务端中转

浏览器无法建立裸 TCP 连接，而 RDP 与 VNC 都是基于 TCP 的二进制协议，因此必须由
WebSSH 服务端提供 `WebSocket ↔ TCP` 通道。两条通道的差异在于协议栈放在哪一侧：

- **VNC**：RFB 是「服务端先说话」的协议，版本协商、安全类型协商与编码协商都可以
  由浏览器端 noVNC 独立完成，所以后端只做一条**透明字节管道**，不解析任何 RFB 报文。
  好处是天然兼容 VNC Auth / VeNCrypt / RA2 / None 等各种变体。
- **RDP**：RDP 协议栈极其复杂，且 Windows 域环境需要 CredSSP/NLA。直接复用成熟实现
  IronRDP 的 WebAssembly 构建（`ironrdp-wasm`），由**浏览器完成 RDP 协议处理**，
  后端实现 RDCleanPath 代理：TCP 连接目标 → 转发 X.224 Connection Request →
  在裸 TCP 上完成 TLS 握手 → 把 X.224 响应与服务器证书链回填给浏览器 → 双向中继。

### RDP 通道时序

```
浏览器 (ironrdp-wasm)             WebSSH 后端 (/rdp)              RDP 服务器
        │                                │                           │
        │  ① WebSocket 升级               │                           │
        ├───────────────────────────────►│                           │
        │  ② RDCleanPath 请求 (DER)       │                           │
        ├───────────────────────────────►│  ③ TCP 连接                │
        │                                ├──────────────────────────►│
        │                                │  ④ X.224 Conn Request     │
        │                                ├──────────────────────────►│
        │                                │  ⑤ X.224 Conn Confirm     │
        │                                │◄──────────────────────────┤
        │                                │  ⑥ TLS 握手（自签证书）      │
        │                                ├──────────────────────────►│
        │  ⑦ RDCleanPath 响应(DER)        │                           │
        │◄───────────────────────────────┤                           │
        │  ⑧ 后续 RDP 报文双向中继（TLS 承载）                          │
        │◄──────────────────────────────►│◄─────────────────────────►│
```

### 设计借鉴说明（参考项目 fengyily/shield-cli）

本次 RDP/VNC 支持的架构设计参考了 [shield-cli](https://github.com/fengyily/shield-cli)
的工程实践，借鉴点如下：

| 借鉴点 | shield-cli 的做法 | 在 WebSSH 中的落地 |
|---|---|---|
| 协议作为一等公民 | 内置协议表声明 ssh/http/https/rdp/vnc/telnet 的默认端口，并提供 `/api/protocols` | `core/protocol.go` 协议注册表 + `GET /protocols` 接口，前端按协议驱动字段可见性与默认端口 |
| 统一连接描述符 | `ConnectParams` / `AppConfig` 用一个结构承载所有协议的连接参数与认证字段 | 扩展 `core.SSHClient` 为统一连接描述符（新增 `protocol` / `domain`），三种协议共用同一个 `sshInfo=base64(JSON)` 契约 |
| 连接管理器与状态机 | `ConnectionManager` 管理 connecting/connected/failed/disconnected 状态 | 前端 `RdpConsole` / `VncConsole` 的 `status` 状态机（idle/connecting/connected/closed/failed） |
| 协议能力差异收敛在协议层 | 通过 `defaultPort(protocol)`、插件注册表区分协议能力 | `ProtocolSpec` 声明 `NeedUsername` / `NeedPassword` / `SupportKey` / `SupportExec` / `RemoteDesktop` |
| 目标地址只信服务端 | 只允许连接已登记的资源，不采信客户端上报的目标 | `/rdp` 只连服务端下发的 `hostname:port`，请求体中的 `destination` 仅用于一致性校验 |

差异点：shield-cli 是「把本地服务通过隧道暴露到公网」（出向隧道），
WebSSH 是「浏览器通过服务端连到目标主机」（入向代理），因此没有采用 chisel 隧道与
凭据加密存储，而是复用了它的**协议抽象与连接管理思想**。

## 安装与运行方法

### 1. Docker 镜像快速启动

```bash
docker run -d \
  -p 8888:8888 \
  -e USER=youruser     # 可选，Web登录用户名
  -e PASS=yourpass     # 可选，Web登录密码（需与USER同时设置）
  -e PORT=8888         # 可选，服务端口，默认8888
  --name webssh \
  eooce/webssh:latest
```

### 2. Docker Compose 部署

新建 `docker-compose.yml`：

```yaml
version: '3'
services:
  webssh:
    image: eooce/webssh:latest
    container_name: webssh
    ports:
      - "8888:8888"
    environment:
      - USER=      # 可选，Web登录用户名（需与PASS同时设置）
      - PASS=      # 可选，Web登录密码
      - PORT=8888  # 可选，服务端口，默认8888
    restart: unless-stopped
```

启动服务：
```bash
docker-compose up -d
```

---

### 3. 源码构建（前端+后端）

1. **环境要求**：Node.js 14+，Go 1.21+
2. **安装前端依赖**：
   ```bash
   cd webssh/frontend
   npm install
   ```
3. **构建前端**：
   ```bash
   npm run fix && npm run build
   ```
   构建产物在 根目录public
4. **启动后端服务**：
   ```bash
   cd .. && go run main.go
   ```
   - 默认监听端口为 8888，可通过 `-p` 参数指定端口。
   - 可通过 `-a user:pass` 启用 Web 登录认证。

## 鸣谢

[Jrohy](https://github.com/Jrohy)

RDP / VNC 模式的实现依赖以下上游项目，对应资源已内置在 `frontend/public/` 下，
构建时由 `vue.config.js` 拷贝到 `public/static/`，通过原生 ES Module 在浏览器端按需加载：

| 项目 | 用途 | 许可证 |
|---|---|---|
| [noVNC](https://github.com/novnc/noVNC) (`@novnc/novnc` 1.7.0) | 浏览器端 VNC(RFB) 客户端 | MPL-2.0 |
| [IronRDP](https://github.com/Devolutions/IronRDP) / [ironrdp-wasm](https://github.com/electerm/ironrdp-wasm) 1.1.0 | 浏览器端 RDP 协议栈（WebAssembly） | MIT |
| [shield-cli](https://github.com/fengyily/shield-cli) | 协议抽象与连接管理的设计参考 | Apache-2.0 |
