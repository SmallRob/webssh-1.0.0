<template>
  <div class="rdp-wrapper">
    <div class="rdp-toolbar">
      <div class="rdp-title">
        <i class="fas fa-desktop"></i>
        <span class="rdp-host">{{ target }}</span>
        <span class="rdp-badge">RDP</span>
      </div>
      <div class="rdp-actions">
        <span class="rdp-status" :class="statusClass">{{ statusText }}</span>
        <button class="rdp-btn" :disabled="!connected" @click="sendCtrlAltDel" title="发送 Ctrl+Alt+Del">
          <i class="fas fa-keyboard"></i> Ctrl+Alt+Del
        </button>
        <button class="rdp-btn" :disabled="!connected" @click="syncResolution" title="把远程桌面分辨率调整为当前窗口大小">
          <i class="fas fa-expand"></i> 同步分辨率
        </button>
        <button class="rdp-btn" @click="toggleFit" :title="fitMode ? '按原始比例显示' : '缩放到窗口'">
          <i class="fas fa-compress-alt"></i> {{ fitMode ? '原始比例' : '适应窗口' }}
        </button>
        <button class="rdp-btn" @click="toggleFullscreen" title="全屏">
          <i class="fas fa-expand-arrows-alt"></i> 全屏
        </button>
        <button class="rdp-btn danger" :disabled="!connected" @click="disconnect" title="断开连接">
          <i class="fas fa-power-off"></i> 断开
        </button>
      </div>
    </div>

    <div ref="container" class="rdp-stage" tabindex="0">
      <canvas ref="screen" class="rdp-canvas" :class="{ 'is-fit': fitMode }"></canvas>
      <div v-if="!connected" class="rdp-overlay">
        <div class="rdp-overlay-box">
          <div class="rdp-overlay-title">{{ overlayTitle }}</div>
          <div class="rdp-overlay-msg">{{ statusText }}</div>
          <button class="rdp-btn primary" :disabled="loading" @click="connect">
            {{ loading ? '连接中…' : '重新连接' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { encodeConnInfo, wsUrl, targetAddr, loadEsm } from '@/utils/remote'

// ironrdp-wasm 的部署位置。
// 生产环境在 /static 下（由 vue.config.js 的 copy 规则产出），
// 开发环境 vue-cli 直接以 public/ 为根目录提供。
const ASSET_BASE = (process.env.NODE_ENV === 'production' ? '/static' : '') + '/rdp'
const MODULE_URL = `${ASSET_BASE}/rdp_client.js`
const WASM_URL = `${ASSET_BASE}/rdp_client_bg.wasm`

// PC/AT 扫描码表（含扩展键 0xE0 前缀），抄自 IronRDP Web 官方示例。
// IronRDP 的 DeviceEvent 直接消费该编码，因此这里不做任何转换。
const SCANCODE_MAP = {
  Escape: 0x01, Digit1: 0x02, Digit2: 0x03, Digit3: 0x04,
  Digit4: 0x05, Digit5: 0x06, Digit6: 0x07, Digit7: 0x08,
  Digit8: 0x09, Digit9: 0x0A, Digit0: 0x0B, Minus: 0x0C,
  Equal: 0x0D, Backspace: 0x0E, Tab: 0x0F,
  KeyQ: 0x10, KeyW: 0x11, KeyE: 0x12, KeyR: 0x13,
  KeyT: 0x14, KeyY: 0x15, KeyU: 0x16, KeyI: 0x17,
  KeyO: 0x18, KeyP: 0x19, BracketLeft: 0x1A, BracketRight: 0x1B,
  Enter: 0x1C, ControlLeft: 0x1D,
  KeyA: 0x1E, KeyS: 0x1F, KeyD: 0x20, KeyF: 0x21,
  KeyG: 0x22, KeyH: 0x23, KeyJ: 0x24, KeyK: 0x25,
  KeyL: 0x26, Semicolon: 0x27, Quote: 0x28, Backquote: 0x29,
  ShiftLeft: 0x2A, Backslash: 0x2B,
  KeyZ: 0x2C, KeyX: 0x2D, KeyC: 0x2E, KeyV: 0x2F,
  KeyB: 0x30, KeyN: 0x31, KeyM: 0x32, Comma: 0x33,
  Period: 0x34, Slash: 0x35, ShiftRight: 0x36,
  NumpadMultiply: 0x37, AltLeft: 0x38, Space: 0x39,
  CapsLock: 0x3A,
  F1: 0x3B, F2: 0x3C, F3: 0x3D, F4: 0x3E,
  F5: 0x3F, F6: 0x40, F7: 0x41, F8: 0x42,
  F9: 0x43, F10: 0x44,
  NumLock: 0x45, ScrollLock: 0x46,
  Numpad7: 0x47, Numpad8: 0x48, Numpad9: 0x49,
  NumpadSubtract: 0x4A, Numpad4: 0x4B, Numpad5: 0x4C,
  Numpad6: 0x4D, NumpadAdd: 0x4E, Numpad1: 0x4F,
  Numpad2: 0x50, Numpad3: 0x51, Numpad0: 0x52,
  NumpadDecimal: 0x53,
  F11: 0x57, F12: 0x58,
  NumpadEnter: 0xE01C, ControlRight: 0xE01D,
  NumpadDivide: 0xE035, PrintScreen: 0xE037,
  AltRight: 0xE038, Home: 0xE047, ArrowUp: 0xE048,
  PageUp: 0xE049, ArrowLeft: 0xE04B, ArrowRight: 0xE04D,
  End: 0xE04F, ArrowDown: 0xE050, PageDown: 0xE051,
  Insert: 0xE052, Delete: 0xE053,
  MetaLeft: 0xE05B, MetaRight: 0xE05C, ContextMenu: 0xE05D,
  Pause: 0xE11D45
}

const CTRL_LEFT = 0x1D
const ALT_LEFT = 0x38
const DELETE = 0xE053

export default {
  name: 'RdpConsole',
  data () {
    return {
      rdp: null, // 已初始化的 ironrdp-wasm 模块
      session: null,
      canvas: null,
      status: 'idle', // idle | connecting | connected | closed | failed
      message: '',
      loading: false,
      fitMode: true,
      unloaders: []
    }
  },
  computed: {
    connInfo () {
      const sshInfo = this.$store.state.sshInfo
      return {
        protocol: 'rdp',
        hostname: sshInfo.hostname || '',
        port: Number(sshInfo.port) || 3389,
        username: sshInfo.username || '',
        password: sshInfo.password || '',
        domain: sshInfo.domain || ''
      }
    },
    target () {
      return targetAddr(this.connInfo.hostname, this.connInfo.port) || '未配置目标'
    },
    connected () {
      return this.status === 'connected'
    },
    statusText () {
      switch (this.status) {
      case 'connecting': return '正在建立 RDP 会话…'
      case 'connected': return '已连接'
      case 'closed': return this.message || '会话已结束'
      case 'failed': return this.message || '连接失败'
      default: return '尚未连接'
      }
    },
    statusClass () {
      return {
        'is-connected': this.status === 'connected',
        'is-pending': this.status === 'connecting',
        'is-error': this.status === 'failed' || this.status === 'closed'
      }
    },
    overlayTitle () {
      if (this.status === 'failed') return 'RDP 连接失败'
      if (this.status === 'closed') return 'RDP 会话已结束'
      if (this.status === 'connecting') return '正在连接 RDP'
      return 'RDP 远程桌面'
    }
  },
  mounted () {
    this.canvas = this.$refs.screen
    if (!this.connInfo.hostname) {
      this.$message.error('无效的连接信息！正在返回登录页...')
      this.$router.push('/')
      return
    }
    this.connect()
  },
  beforeDestroy () {
    this.cleanup()
  },
  methods: {
    setStatus (status, message) {
      this.status = status
      this.message = message || ''
    },
    // ---- 连接主体 ----
    async connect () {
      if (this.loading) return
      this.loading = true
      this.setStatus('connecting')
      try {
        if (!this.rdp) {
          this.setStatus('connecting', '正在加载 RDP 协议栈（WebAssembly）…')
          const mod = await loadEsm(MODULE_URL)
          await mod.default(WASM_URL)
          mod.setup('warn')
          this.rdp = mod
        }

        const { SessionBuilder, DesktopSize, Extension } = this.rdp
        const container = this.$refs.container
        const width = Math.max(container.clientWidth || 1280, 640)
        const height = Math.max(container.clientHeight || 720, 480)

        const builder = new SessionBuilder()
        builder.username(this.connInfo.username)
        builder.password(this.connInfo.password)
        if (this.connInfo.domain) {
          builder.serverDomain(this.connInfo.domain)
        }
        // destination 是 RDP 目标；proxyAddress 指向本服务的 /rdp 通道，
        // 由后端完成 TCP/TLS 直连（浏览器无法直接建立裸 TCP）。
        builder.destination(this.target)
        builder.proxyAddress(wsUrl('/rdp', { sshInfo: encodeConnInfo(this.connInfo) }))
        builder.authToken('none')
        builder.desktopSize(new DesktopSize(width, height))
        builder.renderCanvas(this.canvas)
        // CredSSP/NLA 支持，Windows 域环境必需
        builder.extension(new Extension('enable_credssp', true))

        builder.setCursorStyleCallbackContext(this.canvas)
        builder.setCursorStyleCallback(style => {
          this.canvas.style.cursor = style || 'default'
        })
        builder.canvasResizedCallback(size => this.onCanvasResized(size))
        this.setupClipboard(builder)

        const session = await builder.connect()
        this.session = session

        const ds = session.desktopSize()
        this.applyDesktopSize(ds.width, ds.height)
        this.bindInputs()

        this.setStatus('connected')
        this.canvas.focus()

        // run() 会一直挂起直到会话结束
        session.run().then(info => {
          this.setStatus('closed', info && info.reason ? info.reason() : '会话已结束')
          this.cleanupSession()
        }).catch(err => {
          this.setStatus('closed', this.describeError(err))
          this.cleanupSession()
        })
      } catch (err) {
        this.setStatus('failed', this.describeError(err))
      } finally {
        this.loading = false
      }
    },

    // ---- 剪贴板互通（CLIPRDR 通道）----
    setupClipboard (builder) {
      builder.remoteClipboardChangedCallback(content => {
        try {
          if (!content || content.isEmpty()) return
          const items = content.items() || []
          const textItem = items.find(i => i.mimeType() === 'text/plain')
          if (textItem && navigator.clipboard) {
            // 浏览器要求用户手势才能写剪贴板，失败时静默忽略
            navigator.clipboard.writeText(String(textItem.value())).catch(() => {})
          }
        } catch (e) { /* 剪贴板异常不影响会话 */ }
      })
      builder.forceClipboardUpdateCallback(async () => {
        try {
          const { ClipboardData } = this.rdp
          const data = new ClipboardData()
          try {
            const text = await navigator.clipboard.readText()
            if (text) data.addText('text/plain', text)
          } catch (e) { /* 无权限时发送空剪贴板 */ }
          if (this.session) await this.session.onClipboardPaste(data)
        } catch (e) { /* ignore */ }
      })
    },

    // ---- 画布尺寸 ----
    applyDesktopSize (width, height) {
      if (!width || !height) return
      this.canvas.width = width
      this.canvas.height = height
    },
    onCanvasResized (size) {
      try {
        if (size && typeof size === 'object' && size.width) {
          this.applyDesktopSize(size.width, size.height)
        }
      } catch (e) { /* ignore */ }
    },
    syncResolution () {
      if (!this.session) return
      const container = this.$refs.container
      const width = Math.max(container.clientWidth - 4, 640)
      const height = Math.max(container.clientHeight - 4, 480)
      try {
        this.session.resize(width, height)
        this.$message.success(`已请求把远程分辨率调整为 ${width}×${height}`)
      } catch (e) {
        this.$message.warning('当前服务器不支持动态调整分辨率')
      }
    },
    toggleFit () {
      this.fitMode = !this.fitMode
    },
    toggleFullscreen () {
      const el = this.$refs.container
      if (!document.fullscreenElement) {
        (el.requestFullscreen || el.webkitRequestFullscreen || (() => {})).call(el)
      } else {
        document.exitFullscreen()
      }
    },

    // ---- 输入事件 ----
    addUnloader (fn) {
      this.unloaders.push(fn)
    },
    bindInputs () {
      const canvas = this.canvas
      const { DeviceEvent, InputTransaction } = this.rdp

      const apply = events => {
        if (!this.session) return
        try {
          const tx = new InputTransaction()
          events.forEach(e => tx.addEvent(e))
          this.session.applyInputs(tx)
        } catch (e) { /* ignore */ }
      }

      const onKeyDown = e => {
        const code = SCANCODE_MAP[e.code]
        if (code === undefined) return
        e.preventDefault()
        e.stopPropagation()
        apply([DeviceEvent.keyPressed(code)])
        this.syncLockKeys(e)
      }
      const onKeyUp = e => {
        const code = SCANCODE_MAP[e.code]
        if (code === undefined) return
        e.preventDefault()
        e.stopPropagation()
        apply([DeviceEvent.keyReleased(code)])
        this.syncLockKeys(e)
      }
      const onMouseMove = e => {
        if (!this.session) return
        const rect = canvas.getBoundingClientRect()
        const x = Math.round((e.clientX - rect.left) * (canvas.width / rect.width))
        const y = Math.round((e.clientY - rect.top) * (canvas.height / rect.height))
        apply([DeviceEvent.mouseMove(x, y)])
      }
      const onMouseDown = e => {
        e.preventDefault()
        canvas.focus()
        apply([DeviceEvent.mouseButtonPressed(e.button)])
      }
      const onMouseUp = e => {
        e.preventDefault()
        apply([DeviceEvent.mouseButtonReleased(e.button)])
      }
      const onWheel = e => {
        e.preventDefault()
        const events = []
        if (e.deltaY !== 0) events.push(DeviceEvent.wheelRotations(true, e.deltaY > 0 ? -1 : 1, 1))
        if (e.deltaX !== 0) events.push(DeviceEvent.wheelRotations(false, e.deltaX > 0 ? -1 : 1, 1))
        if (events.length) apply(events)
      }
      const onContextMenu = e => e.preventDefault()

      canvas.addEventListener('keydown', onKeyDown)
      canvas.addEventListener('keyup', onKeyUp)
      canvas.addEventListener('mousemove', onMouseMove)
      canvas.addEventListener('mousedown', onMouseDown)
      canvas.addEventListener('mouseup', onMouseUp)
      canvas.addEventListener('wheel', onWheel, { passive: false })
      canvas.addEventListener('contextmenu', onContextMenu)

      this.addUnloader(() => {
        canvas.removeEventListener('keydown', onKeyDown)
        canvas.removeEventListener('keyup', onKeyUp)
        canvas.removeEventListener('mousemove', onMouseMove)
        canvas.removeEventListener('mousedown', onMouseDown)
        canvas.removeEventListener('mouseup', onMouseUp)
        canvas.removeEventListener('wheel', onWheel)
        canvas.removeEventListener('contextmenu', onContextMenu)
      })
    },
    syncLockKeys (e) {
      if (!this.session) return
      try {
        this.session.synchronizeLockKeys(
          e.getModifierState && e.getModifierState('ScrollLock'),
          e.getModifierState && e.getModifierState('NumLock'),
          e.getModifierState && e.getModifierState('CapsLock'),
          false
        )
      } catch (err) { /* ignore */ }
    },
    sendCtrlAltDel () {
      if (!this.session) return
      const { DeviceEvent, InputTransaction } = this.rdp
      try {
        const tx = new InputTransaction()
        tx.addEvent(DeviceEvent.keyPressed(CTRL_LEFT))
        tx.addEvent(DeviceEvent.keyPressed(ALT_LEFT))
        tx.addEvent(DeviceEvent.keyPressed(DELETE))
        tx.addEvent(DeviceEvent.keyReleased(DELETE))
        tx.addEvent(DeviceEvent.keyReleased(ALT_LEFT))
        tx.addEvent(DeviceEvent.keyReleased(CTRL_LEFT))
        this.session.applyInputs(tx)
      } catch (e) {
        this.$message.warning('发送 Ctrl+Alt+Del 失败')
      }
    },

    // ---- 收尾 ----
    describeError (err) {
      if (!err) return '未知错误'
      const kind = typeof err.kind === 'function' ? err.kind() : undefined
      const kindMap = {
        0: '通用错误',
        1: '账号或密码错误',
        2: '登录被拒绝（用户名/域不正确）',
        3: '权限不足，服务器拒绝访问',
        4: 'RDCleanPath 通道异常',
        5: '无法连接到代理',
        6: 'RDP 协议协商失败'
      }
      const base = kind !== undefined && kindMap[kind] ? kindMap[kind] : (err.message || String(err))
      const backtrace = typeof err.backtrace === 'function' ? err.backtrace() : ''
      return backtrace ? `${base}（${backtrace.split('\n')[0]}）` : base
    },
    cleanupSession () {
      this.session = null
      this.unloaders.forEach(fn => { try { fn() } catch (e) { /* ignore */ } })
      this.unloaders = []
    },
    disconnect () {
      if (this.session) {
        try { this.session.shutdown() } catch (e) { /* ignore */ }
      }
      this.setStatus('closed', '已由用户断开')
      this.cleanupSession()
    },
    cleanup () {
      this.disconnect()
      this.rdp = null
    }
  }
}
</script>

<style scoped>
.rdp-wrapper {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #0b0f14;
  color: #e6edf3;
}

.rdp-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 14px;
  background: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.rdp-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  min-width: 0;
}

.rdp-host {
  font-family: 'DejaVu Sans Mono', monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 40vw;
}

.rdp-badge {
  padding: 1px 7px;
  border-radius: 5px;
  font-size: 11px;
  background: rgba(64, 158, 255, 0.18);
  color: #6eb0ff;
  border: 1px solid rgba(64, 158, 255, 0.35);
}

.rdp-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.rdp-status {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  color: #9aa7b4;
  white-space: nowrap;
}
.rdp-status.is-connected { color: #1adb6d; background: rgba(26, 219, 109, 0.12); }
.rdp-status.is-pending { color: #ffb454; background: rgba(255, 180, 84, 0.12); }
.rdp-status.is-error { color: #ff7070; background: rgba(255, 112, 112, 0.12); }

.rdp-btn {
  font-size: 12px;
  padding: 5px 11px;
  border-radius: 7px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.05);
  color: #d7e0ea;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.rdp-btn:hover:not(:disabled) { background: rgba(64, 158, 255, 0.18); border-color: rgba(64, 158, 255, 0.45); }
.rdp-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.rdp-btn.danger:hover:not(:disabled) { background: rgba(214, 48, 49, 0.2); border-color: rgba(214, 48, 49, 0.5); }
.rdp-btn.primary { background: rgba(64, 158, 255, 0.22); border-color: rgba(64, 158, 255, 0.5); }

.rdp-stage {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: auto;
  background: #000;
  outline: none;
}

.rdp-canvas {
  display: block;
  outline: none;
  image-rendering: auto;
}
.rdp-canvas.is-fit {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.rdp-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(11, 15, 20, 0.92);
}

.rdp-overlay-box {
  text-align: center;
  padding: 28px 34px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  max-width: 520px;
}

.rdp-overlay-title {
  font-size: 19px;
  font-weight: 700;
  margin-bottom: 10px;
}

.rdp-overlay-msg {
  font-size: 13px;
  color: #9aa7b4;
  margin-bottom: 18px;
  word-break: break-all;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .rdp-title { display: none; }
  .rdp-actions { width: 100%; justify-content: flex-end; }
}
</style>
