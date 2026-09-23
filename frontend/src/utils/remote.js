// RDP / VNC 远程桌面在前端的公共工具。
//
// 设计说明（借鉴 shield-cli 的协议抽象）：
// 三种协议共用同一个「连接描述符」——base64(JSON)，字段与后端
// core.SSHClient 一一对应。协议差异只体现在：
//   1) 浏览器侧渲染组件（xterm / noVNC / ironrdp-wasm）
//   2) 后端侧通道实现（SSH 会话 / RDP RDCleanPath 代理 / VNC 字节中继）

import { b64EncodeUtf8, b64DecodeUtf8 } from '@/utils/codec'

// 前端协议清单，与后端 core.ProtocolList() 保持一致。
// 放在前端是为了让协议切换零延迟（不需要先请求 /protocols）。
export const PROTOCOLS = [
    {
        value: 'ssh',
        label: 'SSH',
        title: 'SSH 终端',
        desc: '命令行终端与会话管理',
        defaultPort: 22,
        icon: 'el-icon-monitor',
        needUsername: true,
        needPassword: true,
        needPrivateKey: true,
        needCommand: true,
        remoteDesktop: false
    },
    {
        value: 'rdp',
        label: 'RDP',
        title: 'RDP 远程桌面',
        desc: '浏览器内直连 Windows 远程桌面',
        defaultPort: 3389,
        icon: 'el-icon-monitor',
        needUsername: true,
        needPassword: true,
        needPrivateKey: false,
        needCommand: false,
        needDomain: true,
        remoteDesktop: true
    },
    {
        value: 'vnc',
        label: 'VNC',
        title: 'VNC 远程桌面',
        desc: 'RFB 屏幕共享与远程控制',
        defaultPort: 5900,
        icon: 'el-icon-monitor',
        needUsername: false,
        needPassword: true,
        needPrivateKey: false,
        needCommand: false,
        remoteDesktop: true
    }
]

export function protocolSpec (value) {
    return PROTOCOLS.find(p => p.value === value) || PROTOCOLS[0]
}

// 各协议的客户端入口路由
export const PROTOCOL_ROUTES = {
    ssh: '/terminal',
    rdp: '/rdp',
    vnc: '/vnc'
}

// nativeImport 绕过 webpack 的原生动态 import。
// noVNC 与 ironrdp-wasm 都以浏览器原生 ESM 形式部署在 /static 下，
// 其中 ironrdp-wasm 依赖 import.meta.url 定位 rdp_client_bg.wasm，
// 交给 webpack 打包会破坏该语义，因此运行时再按需加载。
const nativeImport = new Function('p', 'return import(p)')

export function loadEsm (path) {
    return nativeImport(path)
}

// 组装后端 /term、/rdp、/vnc 共用的 sshInfo 查询参数
export function encodeConnInfo (info) {
    const payload = {
        protocol: info.protocol || 'ssh',
        hostname: info.hostname || '',
        port: Number(info.port) || 0,
        username: info.username || '',
        password: info.password || '',
        domain: info.domain || ''
    }
    return b64EncodeUtf8(JSON.stringify(payload))
}

export function decodeConnInfo (b64) {
    return JSON.parse(b64DecodeUtf8(b64))
}

// 生成 ws:// 或 wss:// 地址（跟随当前页面协议，避免混合内容被浏览器拦截）
export function wsUrl (path, query) {
    const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const search = new URLSearchParams(query).toString()
    return `${scheme}://${window.location.host}${path}?${search}`
}

// 规范化目标地址，供 RDP 的 SessionBuilder.destination 使用
export function targetAddr (hostname, port) {
    if (!hostname) return ''
    if (hostname.startsWith('[')) return `${hostname}:${port}`
    return hostname.includes(':') ? `[${hostname}]:${port}` : `${hostname}:${port}`
}
