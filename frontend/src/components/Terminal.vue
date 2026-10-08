<template>
  <div class="terminal-page-wrapper">
    <div class="terminal-page-container">
      <div class="terminal-area">
        <div id="xterm-container"></div>
      </div>
      <div class="file-tree" :class="{ 'is-visible': isSftpVisible }">
        <FileList />
      </div>
    </div>
    <div class="terminal-footer status-bar">
      <div class="status-items">
        <span class="status-item status-host" :title="$t('statusHostname')">
          <i class="fas fa-server"></i>
          <span class="status-text">{{ metrics.hostname || '—' }}</span>
        </span>
        <span class="status-item" :title="$t('statusTime')">
          <i class="far fa-clock"></i>
          <span class="status-text">{{ metrics.time || '—' }}</span>
        </span>
        <span class="status-item" :title="$t('statusCpu')">
          <i class="fas fa-microchip"></i>
          <span class="status-label">CPU</span>
          <span class="status-meter"><span class="status-meter-fill" :class="levelClass(metrics.cpuPercent)" :style="{ width: meterWidth(metrics.cpuPercent) }"></span></span>
          <span :class="levelClass(metrics.cpuPercent)">{{ fmtPercent(metrics.cpuPercent) }}</span>
        </span>
        <span class="status-item" :title="memTitle">
          <i class="fas fa-memory"></i>
          <span class="status-label">{{ $t('statusMem') }}</span>
          <span class="status-meter"><span class="status-meter-fill" :class="levelClass(metrics.mem.percent)" :style="{ width: meterWidth(metrics.mem.percent) }"></span></span>
          <span :class="levelClass(metrics.mem.percent)">{{ fmtPercent(metrics.mem.percent) }}</span>
          <span class="status-sub">{{ fmtKB(metrics.mem.usedKB) }} / {{ fmtKB(metrics.mem.totalKB) }}</span>
        </span>
        <span v-for="d in visibleDisks" :key="d.mount" class="status-item" :title="d.fs + ' → ' + d.mount">
          <i class="fas fa-hdd"></i>
          <span class="status-label">{{ $t('statusDisk') }} {{ d.mount }}</span>
          <span class="status-meter"><span class="status-meter-fill" :class="levelClass(d.percent)" :style="{ width: meterWidth(d.percent) }"></span></span>
          <span :class="levelClass(d.percent)">{{ fmtPercent(d.percent) }}</span>
        </span>
        <span class="status-item" :title="$t('statusLoad')">
          <i class="fas fa-tachometer-alt"></i>
          <span class="status-text">{{ loadText }}</span>
        </span>
        <span class="status-item" :title="$t('statusUptime')">
          <i class="fas fa-hourglass-half"></i>
          <span class="status-text">{{ metrics.uptime || '—' }}</span>
        </span>
        <span v-if="!metricsOk" class="status-item status-offline">
          <i class="fas fa-plug"></i>
          <span>{{ $t('statusOffline') }}</span>
        </span>
      </div>
      <button @click="toggleSftpPanel" class="sftp-toggle-btn" :title="$t('toggleSFTP')">
        <i class="fas fa-folder-open"></i>
      </button>
    </div>
  </div>
</template>

<script>
import { checkSSH } from '@/api/common'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { AttachAddon } from 'xterm-addon-attach'
import FileList from '@/components/FileList'

export default {
    name: 'Terminal',
    components: { FileList },
    data() {
        return {
            term: null,
            ws: null,
            resetClose: false,
            ssh: null,
            savePass: false,
            fontSize: 15,
            isSftpVisible: false,
            // 服务器状态栏
            metrics: {
                hostname: '', time: '', uptime: '',
                load1: '', load5: '', load15: '',
                cpuPercent: -1,
                mem: { totalKB: 0, usedKB: 0, percent: -1 },
                disks: []
            },
            metricsOk: false
        }
    },
    computed: {
        visibleDisks() {
            return (this.metrics.disks || []).slice(0, 2)
        },
        memTitle() {
            return this.$t('statusMem') + ': ' + this.fmtKB(this.metrics.mem.usedKB) + ' / ' + this.fmtKB(this.metrics.mem.totalKB)
        },
        loadText() {
            const m = this.metrics
            if (!m.load1) return '—'
            return `${m.load1} ${m.load5} ${m.load15}`
        }
    },
    mounted() {
        this.$nextTick(() => {
            this.createTerm()
            this.connectMetrics()
        })
    },
    methods: {
        // ---------- 服务器状态栏 ----------
        connectMetrics() {
            const sshReq = this.$store.getters.sshReq
            if (!sshReq) return
            const prefix = process.env.NODE_ENV === 'production' ? '' : '/api'
            const proto = location.protocol === 'http:' ? 'ws' : 'wss'
            const self = this
            this._metricsRetry = (this._metricsRetry || 0)
            this._metricsWs = new WebSocket(`${proto}://${location.host}${prefix}/server/metrics?sshInfo=${sshReq}`)
            this._metricsWs.onmessage = (event) => {
                self.metricsOk = true
                self._metricsRetry = 0
                try {
                    const stats = JSON.parse(event.data)
                    if (stats && stats.hostname !== undefined) {
                        self.metrics = stats
                    }
                } catch (e) { /* 非 JSON 消息（错误提示等），忽略 */ }
            }
            this._metricsWs.onclose = () => {
                self.metricsOk = false
                // 断线退避重连（3s → 最大 30s），页面销毁后停止
                if (self._destroyed) return
                const delay = Math.min(30000, 3000 * (++self._metricsRetry))
                self._metricsTimer = setTimeout(() => self.connectMetrics(), delay)
            }
            this._metricsWs.onerror = () => {
                self.metricsOk = false
            }
        },
        closeMetrics() {
            this._destroyed = true
            if (this._metricsTimer) {
                clearTimeout(this._metricsTimer)
                this._metricsTimer = null
            }
            if (this._metricsWs) {
                this._metricsWs.onclose = null
                this._metricsWs.close()
                this._metricsWs = null
            }
        },
        levelClass(v) {
            if (v === null || v === undefined || v < 0) return ''
            if (v < 60) return 'lv-ok'
            if (v < 85) return 'lv-warn'
            return 'lv-crit'
        },
        meterWidth(v) {
            if (v === null || v === undefined || v < 0) return '0%'
            return Math.min(100, Math.max(0, v)) + '%'
        },
        fmtPercent(v) {
            if (v === null || v === undefined || v < 0) return '—'
            return v.toFixed(0) + '%'
        },
        fmtKB(kb) {
            if (!kb || kb <= 0) return '—'
            const units = ['K', 'M', 'G', 'T']
            let v = kb, i = 0
            while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
            return (v >= 100 ? v.toFixed(0) : v.toFixed(1)) + units[i]
        },
        toggleSftpPanel() {
            this.isSftpVisible = !this.isSftpVisible
        },
        setSSH() {
            this.$store.commit('SET_SSH', this.ssh)
        },
        resizeTerm() {
            // This is now handled by CSS flexbox and xterm's fit-addon.
            // The logic is kept here in case of future need.
        },
        createTerm() {
            const sshInfo = this.$store.state.sshInfo;
            if (!sshInfo || !sshInfo.hostname) {
                this.$message.error('无效的连接信息！正在返回登录页...')
                // 带上原始 query 返回登录页，便于回填快捷链接中的主机信息
                this.$router.push({ path: '/', query: this.$route.query })
                return
            }
            const termWeb = document.getElementById('xterm-container')
            if (!termWeb) {
                console.error('Terminal container #xterm-container not found.')
                return
            }
            const sshReq = this.$store.getters.sshReq
            this.close()
            const prefix = process.env.NODE_ENV === 'production' ? '' : '/api'
            const fitAddon = new FitAddon()
            this.term = new Terminal({
                cursorBlink: true,
                cursorStyle: 'bar',
                cursorWidth: 4,
                fontFamily: 'DejaVu Sans Mono, monospace',  // 设置字体
                fontSize: this.fontSize,            // 字号
                theme: {
                    background: '#000000',          // 背景色
                    foreground: '#ffffff',          // 字体颜色
                    cursor: '#ffffff',              // 光标颜色
                    selection: '#daffe77a',         // 选中区域颜色
                    blue: '#1981ff',                // 蓝色
                    brightMagenta: '#e879f9',       // 紫色
                    brightBlue: '#6eb0ff',          // 亮蓝色
                }
            })
            this.term.loadAddon(fitAddon)
            this.term.open(document.getElementById('xterm-container'))
            this.term.focus()
            this.term.write('\x1b[1;1H\x1b[1;32m正在连接，请稍后...\x1b[0m\r\n')
            try { fitAddon.fit() } catch (e) {/**/}
            const self = this
            const heartCheck = {
                timeout: 5000, // 5s发一次心跳
                intervalObj: null,
                stop: function() {
                    clearInterval(this.intervalObj)
                },
                start: function() {
                    this.intervalObj = setInterval(function() {
                        if (self.ws !== null && self.ws.readyState === 1) {
                            self.ws.send('ping')
                        }
                    }, this.timeout)
                }
            }
            let closeTip = '已超时关闭!'
            if (this.$store.state.language === 'en') {
                closeTip = 'Connection timed out!'
            }
            // open websocket
            this.ws = new WebSocket(`${(location.protocol === 'http:' ? 'ws' : 'wss')}://${location.host}${prefix}/term?sshInfo=${sshReq}&rows=${this.term.rows}&cols=${this.term.cols}&closeTip=${closeTip}`)
            this.ws.onopen = () => {
                console.log(Date(), 'onopen')
                self.connected()
                heartCheck.start()
                self._initCmdSent = false
            }
            // 监听ws消息，检测到提示符再自动执行初始命令并清理提示
            this.ws.onmessage = (event) => {
                if (typeof event.data === 'string') {
                    // We need to wait for the attach addon to process the data and write it to the terminal.
                    // A timeout of 0ms should be enough to push this to the end of the execution queue.
                    setTimeout(() => {
                        if (!self._initCmdSent && self.ssh) {
                            const term = self.term;
                            if (!term || !term.buffer || !term.buffer.active) return;

                            const currentLineNumber = term.buffer.active.baseY + term.buffer.active.cursorY;
                            const line = term.buffer.active.getLine(currentLineNumber);
                            if (line) {
                                const lineText = line.translateToString();
                                // More robust regex to detect various shell prompts at the end of the line
                                if (/[>$#%]\s*$/.test(lineText.trimEnd())) {
                                    self._initCmdSent = true;
                                    // This ANSI sequence saves cursor, moves to 1,1, clears line, and restores cursor
                                    self.term.write('\x1b[s\x1b[1;1H\x1b[2K\x1b[u');
                                    if (self.ssh.command) {
                                        setTimeout(() => {
                                            if (self.ws && self.ws.readyState === 1) {
                                                self.ws.send(self.ssh.command + '\r'); // 让后端执行命令
                                            }
                                        }, 100);
                                    }
                                }
                            }
                        }
                    }, 10);
                }
            }
            this.ws.onclose = () => {
                console.log(Date(), 'onclose')
                if (!self.resetClose) {
                    if (self.ssh && !this.savePass) {
                        this.$store.commit('SET_PASS', '')
                        self.ssh.password = ''
                    }
                    this.$message({
                        message: this.$t('wsClose'),
                        type: 'warning',
                        duration: 0,
                        showClose: true
                    })
                    this.ws = null
                }
                heartCheck.stop()
                self.resetClose = false
                if (self.ws !== null && self.ws.readyState === 1) {
                    self.ws.send(`resize:${self.term.rows}:${self.term.cols}`)
                }
            }
            this.ws.onerror = () => {
                console.log(Date(), 'onerror')
            }
            const attachAddon = new AttachAddon(this.ws, { bidirectional: false })
            this.term.loadAddon(attachAddon)

            // 恢复终端输入到ws
            this.term.onData(data => {
                if (self.ws && self.ws.readyState === 1) {
                    self.ws.send(data)
                }
            })

            this.term.attachCustomKeyEventHandler((e) => {
                const keyArray = ['F5', 'F11', 'F12']
                if (keyArray.indexOf(e.key) > -1) {
                    return false
                }
                // ctrl + v
                if (e.ctrlKey && e.key === 'v') {
                    document.execCommand('copy')
                    return false
                }
                // ctrl + c
                if (e.ctrlKey && e.key === 'c' && self.term.hasSelection()) {
                    document.execCommand('copy')
                    return false
                }
            })
            // detect available wheel event
            // 各个厂商的高版本浏览器都支持"wheel"
            // Webkit 和 IE一定支持"mousewheel"
            // "DOMMouseScroll" 用于低版本的firefox
            const wheelSupport = 'onwheel' in document.createElement('div') ? 'wheel' : document.onmousewheel !== undefined ? 'mousewheel' : 'DOMMouseScroll'
            termWeb.addEventListener(wheelSupport, (e) => {
                if (e.ctrlKey) {
                    e.preventDefault()
                    if (e.deltaY < 0) {
                        self.term.setOption('fontSize', ++this.fontSize)
                    } else {
                        self.term.setOption('fontSize', --this.fontSize)
                    }
                    try { fitAddon.fit() } catch (e) {/**/}
                    if (self.ws !== null && self.ws.readyState === 1) {
                        self.ws.send(`resize:${self.term.rows}:${self.term.cols}`)
                    }
                }
            })
            window.addEventListener('resize', () => {
                try { fitAddon.fit() } catch (e) {/**/}
                if (self.ws !== null && self.ws.readyState === 1) {
                    self.ws.send(`resize:${self.term.rows}:${self.term.cols}`)
                }
            })
        },
        async connected() {
            const sshInfo = this.$store.state.sshInfo
            // 深度拷贝对象
            this.ssh = Object.assign({}, sshInfo)
            // 校验ssh连接信息是否正确
            const result = await checkSSH(this.$store.getters.sshReq)
            if (result.Msg !== 'success') {
                return
            } else {
                this.savePass = result.Data.savePass
            }
            document.title = sshInfo.hostname
            let sshList = this.$store.state.sshList
            if (sshList === null) {
                if (this.savePass) {
                    sshList = `[{"hostname": "${sshInfo.hostname}", "username": "${sshInfo.username}", "port":${sshInfo.port}, "logintype":${sshInfo.logintype}, "password":"${sshInfo.password}"}]`
                } else {
                    sshList = `[{"hostname": "${sshInfo.hostname}", "username": "${sshInfo.username}", "port":${sshInfo.port},  "logintype":${sshInfo.logintype}}]`
                }
            } else {
                const sshListObj = JSON.parse(window.atob(sshList))
                sshListObj.forEach((v, i) => {
                    if (v.hostname === sshInfo.hostname) {
                        sshListObj.splice(i, 1)
                    }
                })
                sshListObj.push({
                    hostname: sshInfo.hostname,
                    username: sshInfo.username,
                    port: sshInfo.port,
                    logintype: sshInfo.logintype
                })
                if (this.savePass) {
                    sshListObj[sshListObj.length - 1].password = sshInfo.password
                }
                sshList = JSON.stringify(sshListObj)
            }
            this.$store.commit('SET_LIST', window.btoa(sshList))
        },
        close() {
            if (this.ws !== null) {
                this.ws.close()
                this.resetClose = true
            }
            if (this.term !== null) {
                this.term.dispose()
            }
        }
    },
    beforeDestroy() {
        this.closeMetrics()
        this.close()
    }
}
</script>

<style scoped>
.terminal-page-wrapper {
  display: flex;
  flex-direction: column;
  flex-grow: 1; /* This will make it fill the space given by App.vue */
  min-height: 0; /* Prevents overflow issues */
  background: var(--card-bg);
  box-shadow: var(--shadow);
}

.terminal-page-container {
  display: flex;
  flex-grow: 1;
  min-height: 0;
  overflow: hidden;
}

.terminal-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background-color: black;
}

#xterm-container {
  flex-grow: 1;
  width: 100%;
  padding-left: 2px;
}

.file-tree {
  width: 350px;
  border-left: 1px solid var(--input-border);
  background: var(--input-bg);
  display: flex;
  flex-direction: column;
}

.terminal-footer {
  width: 100%;
  margin-left: -3rem;
  padding: 4px 12px;
  font-size: 12px;
  color: var(--text-primary);
  background: var(--card-bg);
  border-top: 1px solid var(--input-border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

/* ---------- 服务器状态栏 ---------- */
.status-bar {
  font-family: 'DejaVu Sans Mono', 'SFMono-Regular', Consolas, monospace;
  font-variant-numeric: tabular-nums;
}

.status-items {
  display: flex;
  align-items: center;
  gap: 16px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  opacity: 0.85;
  flex-shrink: 0;
}

.status-item i {
  font-size: 11px;
  opacity: 0.65;
}

.status-text {
  max-width: 210px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: inline-block;
  vertical-align: middle;
}

.status-label {
  opacity: 0.75;
}

.status-sub {
  opacity: 0.6;
  font-size: 11px;
}

.status-meter {
  width: 44px;
  height: 5px;
  border-radius: 3px;
  background: rgba(128, 128, 128, 0.28);
  overflow: hidden;
}

.status-meter-fill {
  display: block;
  height: 100%;
  border-radius: 3px;
  transition: width 0.6s ease, background-color 0.3s;
}

/* 阈值着色：<60 绿 / <85 黄 / >=85 红 */
.lv-ok { color: #4caf50; }
.lv-warn { color: #ff9800; }
.lv-crit { color: #f44336; }

.lv-ok .status-meter-fill,
.status-meter-fill.lv-ok { background: #4caf50; }
.status-meter-fill.lv-warn { background: #ff9800; }
.status-meter-fill.lv-crit { background: #f44336; }

.status-offline {
  color: #f44336;
  opacity: 0.9;
}

.status-host {
  font-weight: 600;
}

.sftp-toggle-btn {
  display: none;
  background: none;
  border: none;
  color: var(--text-primary);
  font-size: 16px;
  cursor: pointer;
  padding: 0 2px;
  line-height: 1;
  flex-shrink: 0;
}

.sftp-toggle-btn:hover {
  color: var(--text-primary);
}

@media (max-width: 768px) {
  .sftp-toggle-btn {
    display: inline-block;
  }

  .status-text,
  .status-sub,
  .status-label {
    max-width: 90px;
  }

  .status-items {
    gap: 10px;
    overflow-x: auto;
  }

  .terminal-page-container {
    position: relative;
    overflow: hidden; /* Contain the sliding panel */
  }

  .file-tree {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    width: 85%;
    max-width: 350px;
    transform: translateX(100%);
    transition: transform 0.3s ease-in-out;
    z-index: 20;
    border-left: none;
    box-shadow: -2px 0 10px rgba(0,0,0,0.15);
  }

  .file-tree.is-visible {
    transform: translateX(0);
  }

  .terminal-footer {
    margin-left: 0;
    width: 100%;
  }
}
</style>
