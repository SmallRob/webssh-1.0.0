<template>
  <div class="vnc-wrapper">
    <div class="vnc-toolbar">
      <div class="vnc-title">
        <i class="fas fa-desktop"></i>
        <span class="vnc-host">{{ target }}</span>
        <span class="vnc-badge">VNC</span>
      </div>
      <div class="vnc-actions">
        <span class="vnc-status" :class="statusClass">{{ statusText }}</span>
        <button class="vnc-btn" :disabled="!connected" @click="toggleViewOnly" :title="viewOnly ? '允许控制' : '仅查看（禁止输入）'">
          <i class="fas" :class="viewOnly ? 'fa-eye' : 'fa-hand-pointer'"></i> {{ viewOnly ? '仅查看' : '可控制' }}
        </button>
        <button class="vnc-btn" :disabled="!connected" @click="toggleScale" :title="scaleViewport ? '按原始比例显示' : '缩放到窗口'">
          <i class="fas fa-compress-alt"></i> {{ scaleViewport ? '原始比例' : '适应窗口' }}
        </button>
        <button class="vnc-btn" :disabled="!connected" @click="sendCtrlAltDel" title="发送 Ctrl+Alt+Del">
          <i class="fas fa-keyboard"></i> Ctrl+Alt+Del
        </button>
        <button class="vnc-btn" @click="toggleFullscreen" title="全屏">
          <i class="fas fa-expand-arrows-alt"></i> 全屏
        </button>
        <button class="vnc-btn danger" :disabled="!connected" @click="disconnect" title="断开连接">
          <i class="fas fa-power-off"></i> 断开
        </button>
      </div>
    </div>

    <div ref="container" class="vnc-stage">
      <div ref="screen" class="vnc-screen"></div>
      <div v-if="!connected" class="vnc-overlay">
        <div class="vnc-overlay-box">
          <div class="vnc-overlay-title">{{ overlayTitle }}</div>
          <div class="vnc-overlay-msg">{{ statusText }}</div>
          <button class="vnc-btn primary" :disabled="loading" @click="connect">
            {{ loading ? '连接中…' : '重新连接' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { encodeConnInfo, wsUrl, targetAddr, loadEsm } from '@/utils/remote'

// noVNC 的部署位置，与 vue.config.js 的 copy 规则保持一致
const ASSET_BASE = (process.env.NODE_ENV === 'production' ? '/static' : '') + '/novnc'
const RFB_MODULE = `${ASSET_BASE}/core/rfb.js`

export default {
  name: 'VncConsole',
  data () {
    return {
      RFB: null,
      rfb: null,
      status: 'idle', // idle | connecting | connected | closed | failed | needPassword
      message: '',
      loading: false,
      scaleViewport: true,
      viewOnly: false
    }
  },
  computed: {
    connInfo () {
      const sshInfo = this.$store.state.sshInfo
      return {
        protocol: 'vnc',
        hostname: sshInfo.hostname || '',
        port: Number(sshInfo.port) || 5900,
        username: sshInfo.username || '',
        password: sshInfo.password || ''
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
      case 'connecting': return '正在建立 VNC 会话…'
      case 'connected': return '已连接'
      case 'needPassword': return '需要 VNC 密码'
      case 'closed': return this.message || '会话已结束'
      case 'failed': return this.message || '连接失败'
      default: return '尚未连接'
      }
    },
    statusClass () {
      return {
        'is-connected': this.status === 'connected',
        'is-pending': this.status === 'connecting' || this.status === 'needPassword',
        'is-error': this.status === 'failed' || this.status === 'closed'
      }
    },
    overlayTitle () {
      if (this.status === 'failed') return 'VNC 连接失败'
      if (this.status === 'closed') return 'VNC 会话已结束'
      if (this.status === 'connecting') return '正在连接 VNC'
      return 'VNC 远程桌面'
    }
  },
  mounted () {
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
    async connect () {
      if (this.loading) return
      this.loading = true
      this.setStatus('connecting')
      try {
        if (!this.RFB) {
          this.setStatus('connecting', '正在加载 noVNC 客户端…')
          const mod = await loadEsm(RFB_MODULE)
          this.RFB = mod.default
        }
        this.closeRfb()

        // RFB 的版本/安全类型/编码协商全部由 noVNC 在浏览器侧完成，
        // 后端只提供 WebSocket ↔ VNC TCP 的透明字节管道。
        const url = wsUrl('/vnc', { sshInfo: encodeConnInfo(this.connInfo) })
        const rfb = new this.RFB(this.$refs.screen, url, {
          credentials: { username: this.connInfo.username, password: this.connInfo.password },
          shared: true
        })
        this.rfb = rfb

        rfb.scaleViewport = this.scaleViewport
        rfb.resizeSession = false
        rfb.clipViewport = false
        rfb.viewOnly = this.viewOnly
        rfb.showDotCursor = true

        rfb.addEventListener('connect', () => {
          this.setStatus('connected')
          document.title = this.connInfo.hostname
        })
        rfb.addEventListener('disconnect', e => {
          // clean=false 通常意味着网络中断或服务端关闭
          const clean = e.detail && e.detail.clean
          this.setStatus('closed', clean ? '连接已被关闭' : '连接中断（网络异常或服务端关闭）')
          this.rfb = null
        })
        rfb.addEventListener('credentialsrequired', () => {
          this.setStatus('needPassword')
          if (this.connInfo.password) {
            rfb.sendCredentials({ username: this.connInfo.username, password: this.connInfo.password })
            this.setStatus('connecting')
          } else {
            this.$prompt('该 VNC 服务器需要密码', 'VNC 认证', {
              inputType: 'password',
              confirmButtonText: '确定',
              cancelButtonText: '取消'
            }).then(({ value }) => {
              rfb.sendCredentials({ password: value })
              this.setStatus('connecting')
            }).catch(() => {
              this.setStatus('failed', '已取消密码输入')
              this.closeRfb()
            })
          }
        })
        rfb.addEventListener('securityfailure', e => {
          const reason = (e.detail && e.detail.reason) || '密码或安全类型不被接受'
          this.setStatus('failed', `认证失败：${reason}`)
        })
        rfb.addEventListener('desktopname', e => {
          if (e.detail && e.detail.name) {
            this.$message.success(`已连接：${e.detail.name}`)
          }
        })
        rfb.addEventListener('clipboard', e => {
          if (e.detail && e.detail.text && navigator.clipboard) {
            navigator.clipboard.writeText(e.detail.text).catch(() => {})
          }
        })
      } catch (err) {
        this.setStatus('failed', (err && err.message) || String(err))
      } finally {
        this.loading = false
      }
    },
    toggleScale () {
      this.scaleViewport = !this.scaleViewport
      if (this.rfb) this.rfb.scaleViewport = this.scaleViewport
    },
    toggleViewOnly () {
      this.viewOnly = !this.viewOnly
      if (this.rfb) this.rfb.viewOnly = this.viewOnly
    },
    sendCtrlAltDel () {
      if (this.rfb) this.rfb.sendCtrlAltDel()
    },
    toggleFullscreen () {
      const el = this.$refs.container
      if (!document.fullscreenElement) {
        (el.requestFullscreen || el.webkitRequestFullscreen || (() => {})).call(el)
      } else {
        document.exitFullscreen()
      }
    },
    closeRfb () {
      if (this.rfb) {
        try { this.rfb.disconnect() } catch (e) { /* ignore */ }
        this.rfb = null
      }
    },
    disconnect () {
      this.closeRfb()
      this.setStatus('closed', '已由用户断开')
    },
    cleanup () {
      this.closeRfb()
      this.RFB = null
    }
  }
}
</script>

<style scoped>
.vnc-wrapper {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #0b0f14;
  color: #e6edf3;
}

.vnc-toolbar {
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

.vnc-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  min-width: 0;
}

.vnc-host {
  font-family: 'DejaVu Sans Mono', monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 40vw;
}

.vnc-badge {
  padding: 1px 7px;
  border-radius: 5px;
  font-size: 11px;
  background: rgba(26, 219, 109, 0.16);
  color: #4fe08f;
  border: 1px solid rgba(26, 219, 109, 0.35);
}

.vnc-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.vnc-status {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  color: #9aa7b4;
  white-space: nowrap;
}
.vnc-status.is-connected { color: #1adb6d; background: rgba(26, 219, 109, 0.12); }
.vnc-status.is-pending { color: #ffb454; background: rgba(255, 180, 84, 0.12); }
.vnc-status.is-error { color: #ff7070; background: rgba(255, 112, 112, 0.12); }

.vnc-btn {
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
.vnc-btn:hover:not(:disabled) { background: rgba(26, 219, 109, 0.16); border-color: rgba(26, 219, 109, 0.45); }
.vnc-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.vnc-btn.danger:hover:not(:disabled) { background: rgba(214, 48, 49, 0.2); border-color: rgba(214, 48, 49, 0.5); }
.vnc-btn.primary { background: rgba(26, 219, 109, 0.2); border-color: rgba(26, 219, 109, 0.5); }

.vnc-stage {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: auto;
  background: #000;
}

.vnc-screen {
  width: 100%;
  height: 100%;
}

.vnc-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(11, 15, 20, 0.92);
}

.vnc-overlay-box {
  text-align: center;
  padding: 28px 34px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  max-width: 520px;
}

.vnc-overlay-title {
  font-size: 19px;
  font-weight: 700;
  margin-bottom: 10px;
}

.vnc-overlay-msg {
  font-size: 13px;
  color: #9aa7b4;
  margin-bottom: 18px;
  word-break: break-all;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .vnc-title { display: none; }
  .vnc-actions { width: 100%; justify-content: flex-end; }
}
</style>
