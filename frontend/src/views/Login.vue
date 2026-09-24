<template>
  <div class="login-container" :class="{ 'dark-theme': isDarkTheme }">
    <!-- 右上角：管理员模式（主题切换按钮左侧） -->
    <div class="theme-switch-wrapper">
      <el-button
        v-if="admin.enabled && !admin.isAdmin"
        size="mini"
        type="warning"
        plain
        icon="el-icon-lock"
        class="admin-toggle"
        @click="openAdminDialog('')"
      >管理员</el-button>
      <el-button
        v-else-if="admin.enabled && admin.isAdmin"
        size="mini"
        type="success"
        plain
        icon="el-icon-unlock"
        class="admin-toggle is-on"
        @click="exitAdmin"
      >管理员中</el-button>
      <div class="theme-switch" @click="toggleTheme">
        <i class="fas" :class="isDarkTheme ? 'fa-sun' : 'fa-moon'" style="margin-top: -30px;"></i>
      </div>
    </div>
    <div class="card" style="margin: 12px auto;">
      <div class="title">WebSSH Console</div>
      <el-form :model="sshInfo" label-position="top" class="form-grid">
        <!-- 连接协议：SSH / RDP / VNC 扁平紧凑切换，受门禁保护的协议显示锁标识 -->
        <el-form-item label="连接协议 (Protocol)">
          <div class="protocol-picker">
            <button
              v-for="p in protocols"
              :key="p.value"
              type="button"
              class="protocol-pill"
              :class="{ 'is-active': sshInfo.protocol === p.value }"
              @click="onPickProtocol(p.value)"
            >
              {{ p.label }}
              <i v-if="isProtocolLocked(p.value)" class="fas fa-lock protocol-lock"></i>
            </button>
          </div>
        </el-form-item>
                 <el-row :gutter="20">
           <el-col :span="12">
             <el-form-item label="主机地址 (Hostname)">
               <el-input ref="hostnameInput" v-model="sshInfo.hostname" placeholder="请输入主机地址" />
             </el-form-item>
           </el-col>
           <el-col :span="12">
             <el-form-item label="端口 (Port)">
               <el-input v-model.number="sshInfo.port" :placeholder="`请输入端口(默认${currentProtocol.defaultPort})`" />
             </el-form-item>
           </el-col>
         </el-row>
                 <el-row :gutter="20">
           <el-col v-if="currentProtocol.needUsername" :span="12">
             <el-form-item label="用户名 (Username)">
               <el-input ref="usernameInput" v-model="sshInfo.username" placeholder="请输入用户名" />
             </el-form-item>
           </el-col>
           <el-col :span="currentProtocol.needUsername ? 12 : 24">
             <el-form-item :label="passwordLabel">
               <el-input ref="passwordInput" v-model="sshInfo.password" type="password" :placeholder="passwordPlaceholder" show-password/>
             </el-form-item>
           </el-col>
         </el-row>
         <el-row v-if="currentProtocol.needDomain" :gutter="20">
           <el-col :span="24">
             <el-form-item label="域 (Domain)">
               <el-input v-model="sshInfo.domain" placeholder="Windows 域 / 工作组，可留空（默认本机）" />
             </el-form-item>
           </el-col>
         </el-row>
                 <el-row v-if="currentProtocol.needPrivateKey" :gutter="20">
           <el-col :xs="24" :sm="12">
             <el-form-item label="私钥 (Private Key)">
               <el-upload
                 class="upload-key"
                 :show-file-list="false"
                 :before-upload="handlePrivateKeyUpload"
                 accept=".pem,.ppk,.key,.rsa,.id_rsa,.id_dsa,.txt"
               >
                 <div class="upload-flex-row">
                   <div class="upload-btn">
                     <i class="el-icon-folder-opened" style="margin-right:8px;"></i>
                     上传密钥
                   </div>
                   <div class="upload-filename" style="width: 12rem">
                     {{ privateKeyFileName || '未上传密钥文件' }}
                   </div>
                 </div>
               </el-upload>
             </el-form-item>
           </el-col>
           <el-col :xs="24" :sm="12">
             <el-form-item label="密钥口令 (PIN)">
               <el-input v-model="sshInfo.passphrase" placeholder="如果有设置请输入密钥口令" />
             </el-form-item>
           </el-col>
         </el-row>
        <el-row v-if="currentProtocol.needCommand">
          <el-col :span="24">
            <el-form-item label="初始命令 (Initial command)">
              <el-input v-model="sshInfo.command" placeholder="登录后要执行的命令" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row type="flex" justify="center" style="margin-top: 10px;">
          <el-button type="danger" icon="el-icon-refresh" @click="onReset">重置输入</el-button>
          <el-button type="primary" icon="el-icon-link" @click="onGenerateLink">生成链接</el-button>
          <el-button type="success" @click="onConnect"><i class="fas fa-terminal" style="margin-right: 6px;"></i>{{ connectLabel }}</el-button>
        </el-row>
        <el-row v-if="generatedLink" style="margin-top: 18px;">
          <el-col :span="24">
            <el-input v-model="generatedLink" readonly class="gen-link-input">
              <template slot="append">
                <el-button style="color: #1adb6d;" @click="copyLink" icon="el-icon-document-copy"></el-button>
              </template>
            </el-input>
          </el-col>
        </el-row>
      </el-form>
    </div>
    <!-- 快捷连接：仅显示名称（隐藏 IP），管理员可编辑（含「仅管理员可见」条目） -->
    <div class="quick-servers" v-if="quickServers.length || admin.enabled">
      <div class="qs-head">
        <span class="qs-title">快捷连接 (Quick Connect)</span>
        <el-button
          v-if="admin.enabled && admin.isAdmin"
          size="mini"
          type="text"
          icon="el-icon-setting"
          class="qs-edit"
          @click="openServerEditor"
        >编辑</el-button>
      </div>
      <div class="qs-list" v-if="quickServers.length">
        <button
          v-for="s in quickServers"
          :key="s.name"
          type="button"
          class="qs-btn"
          :class="{ 'is-admin-only': s.adminOnly }"
          :title="'连接 ' + s.name"
          @click="fillFromQuick(s)"
        >
          <b>{{ s.name }}</b>
        </button>
      </div>
      <div v-else class="qs-empty">暂无快捷连接</div>
    </div>
    <!-- 管理员模式解锁弹窗：RDP/VNC 受门禁保护时先校验管理员密码 -->
    <el-dialog
      title="进入管理员模式"
      :visible.sync="adminDialogVisible"
      width="360px"
      append-to-body
      custom-class="admin-dialog"
    >
      <div class="admin-dialog-tip">{{ adminDialogTip }}</div>
      <el-input
        ref="adminPasswordInput"
        v-model="adminPassword"
        type="password"
        placeholder="请输入管理员密码"
        show-password
        @keyup.enter.native="unlockAdmin"
      />
      <div slot="footer">
        <el-button size="small" @click="adminDialogVisible = false">取消</el-button>
        <el-button size="small" type="primary" :loading="adminLoading" @click="unlockAdmin">解锁</el-button>
      </div>
    </el-dialog>
    <!-- 快捷连接编辑浮窗（仅管理员）：新增 / 修改 / 删除 / 启用禁用 / 仅管理员 -->
    <el-dialog
      title="编辑快捷连接"
      :visible.sync="serverEditorVisible"
      width="700px"
      append-to-body
      custom-class="server-editor-dialog"
    >
      <div class="server-editor-tip">
        仅保存名称与地址端口，连接时用户手动输入凭据。停用的条目对所有人隐藏；勾选「仅管理员」后该条目仅管理员可见。
      </div>
      <div class="srv-head-row">
        <span class="srv-col-name">名称</span>
        <span class="srv-col-host">主机地址</span>
        <span class="srv-col-port">端口</span>
        <span class="srv-col-state">启用</span>
        <span class="srv-col-admin">仅管理员</span>
        <span class="srv-col-op"></span>
      </div>
      <div v-for="(s, idx) in editableServers" :key="idx" class="srv-row" :class="{ 'is-disabled': !s.enabled }">
        <el-input v-model="s.name" size="small" placeholder="名称" class="srv-col-name" />
        <el-input v-model="s.host" size="small" placeholder="IP / 域名" class="srv-col-host" />
        <el-input v-model.number="s.port" size="small" placeholder="22" class="srv-col-port" />
        <div class="srv-col-state">
          <el-switch v-model="s.enabled" />
        </div>
        <div class="srv-col-admin">
          <el-checkbox v-model="s.adminOnly" />
        </div>
        <el-button size="mini" type="text" icon="el-icon-delete" class="srv-col-op" @click="removeServer(idx)" />
      </div>
      <el-button size="small" plain icon="el-icon-plus" @click="addServer">新增条目</el-button>
      <div slot="footer">
        <el-button size="small" @click="serverEditorVisible = false">取消</el-button>
        <el-button size="small" type="primary" :loading="serverSaving" @click="saveServers">保存</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import { PROTOCOLS, protocolSpec, PROTOCOL_ROUTES } from '@/utils/remote'
import { getAdminStatus, adminLogin, adminLogout, getQuickServers, getServerDetail, saveQuickServers } from '@/api/common'

const PROTOCOL_VALUES = PROTOCOLS.map(p => p.value)

export default {
  data () {
    return {
      protocols: PROTOCOLS,
      sshInfo: {
        protocol: 'ssh',
        hostname: '',
        port: '',
        username: '',
        password: '',
        domain: '',
        privateKey: '',
        passphrase: '',
        command: ''
      },
      privateKeyFileName: '',
      generatedLink: '',
      isDarkTheme: false,
      // 管理员门禁状态（由后端 /admin/status 提供）
      admin: {
        enabled: false, // 是否配置了管理员密码
        rdp: false,     // RDP 是否需要管理员
        vnc: false,     // VNC 是否需要管理员
        isAdmin: true   // 当前会话是否已解锁
      },
      adminDialogVisible: false,
      adminPassword: '',
      adminLoading: false,
      // 解锁后要继续的动作：'' 仅解锁 / 'connect' 继续连接 / 'ssh' 等协议值切换过去
      pendingAfterUnlock: '',
      // 快捷连接（/servers 实时下发，仅含名称；host 仅用于回填不展示）
      quickServers: [],
      // 快捷连接编辑浮窗
      serverEditorVisible: false,
      editableServers: [],
      serverSaving: false
    }
  },
  computed: {
    currentProtocol () {
      return protocolSpec(this.sshInfo.protocol)
    },
    connectLabel () {
      return `连接${this.currentProtocol.label}`
    },
    passwordLabel () {
      return this.sshInfo.protocol === 'vnc' ? '密码 (VNC Password)' : '密码 (Password)'
    },
    passwordPlaceholder () {
      if (this.sshInfo.protocol === 'vnc') return '请输入 VNC 密码（无密码可留空）'
      if (this.sshInfo.protocol === 'rdp') return '请输入 Windows 登录密码'
      return '请输入密码'
    },
    // 当前所选协议是否需要先解锁管理员模式
    needAdminUnlock () {
      return this.isProtocolLocked(this.sshInfo.protocol)
    },
    adminDialogTip () {
      if (this.pendingAfterUnlock && this.pendingAfterUnlock !== 'connect') {
        const spec = protocolSpec(this.pendingAfterUnlock)
        return `${spec.label} 远程桌面需要管理员权限，请输入管理员密码解锁`
      }
      if (this.pendingAfterUnlock === 'connect') {
        return `${this.currentProtocol.label} 连接需要管理员权限，请输入管理员密码解锁`
      }
      return '输入管理员密码以启用远程桌面等管理能力'
    }
  },
  watch: {
    sshInfo: {
      handler(newVal) {
        localStorage.setItem('sshInfo', JSON.stringify(newVal));
      },
      deep: true
    }
  },
  created() {
    // 从 localStorage 恢复完整的连接信息
    const savedInfo = localStorage.getItem('connectionInfo')
    if (savedInfo) {
      const info = JSON.parse(savedInfo)
      this.sshInfo = {
        protocol: info.protocol || 'ssh',
        hostname: info.hostname || '',
        port: info.port || '',
        username: info.username || '',
        password: info.password || '',
        domain: info.domain || '',
        privateKey: info.privateKey || '',
        passphrase: info.passphrase || '',
        command: info.command || ''
      }
      // 如果有私钥，恢复文件名显示
      if (info.privateKey) {
        this.privateKeyFileName = '已保存的密钥文件'
      }
    }

    // 快捷链接只携带协议与 IP/端口（无凭据）：打开后回填目标，凭据由用户手动输入
    const query = this.$route.query || {}
    if (query.hostname) {
      if (query.protocol && PROTOCOL_VALUES.indexOf(query.protocol) > -1) {
        this.sshInfo.protocol = query.protocol
      }
      this.sshInfo.hostname = query.hostname
      if (query.port) this.sshInfo.port = Number(query.port) || ''
      this.$message.info('已载入快捷链接的主机信息，请输入用户名密码连接')
    }

    this.loadAdminStatus()
    this.loadQuickServers()

    // 检查主题设置
    const savedTheme = localStorage.getItem('isDarkTheme')
    if (savedTheme !== null) {
      this.isDarkTheme = savedTheme === 'true'
    }
    // 恢复主题时同步到 html 根元素，保证 body/#app 背景正确
    document.documentElement.classList.toggle('dark-theme', this.isDarkTheme)

    // 添加 Font Awesome CSS
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = 'https://cdnjs.cloudflare.com/ajax/libs/font-awesome/5.15.4/css/all.min.css'
    document.head.appendChild(link)
  },
  methods: {
    loadAdminStatus () {
      getAdminStatus().then(res => {
        const d = (res && res.Data) || {}
        this.admin = {
          enabled: !!d.enabled,
          rdp: !!d.rdp,
          vnc: !!d.vnc,
          isAdmin: d.isAdmin !== false
        }
      }).catch(() => { /* 查询失败时按未启用处理，后端 WS 门禁兜底 */ })
    },
    // 某协议是否处于管理员门禁锁定状态
    isProtocolLocked (value) {
      if (!this.admin.enabled || this.admin.isAdmin) return false
      if (value === 'rdp') return this.admin.rdp
      if (value === 'vnc') return this.admin.vnc
      return false
    },
    openAdminDialog (pending) {
      this.pendingAfterUnlock = pending || ''
      this.adminPassword = ''
      this.adminDialogVisible = true
      this.$nextTick(() => {
        this.$refs.adminPasswordInput && this.$refs.adminPasswordInput.focus()
      })
    },
    unlockAdmin () {
      if (this.adminLoading) return
      if (!this.adminPassword) {
        this.$message.error('请输入管理员密码！')
        return
      }
      this.adminLoading = true
      adminLogin(this.adminPassword).then(res => {
        if (res && res.Data && res.Data.success) {
          this.admin.isAdmin = true
          this.adminDialogVisible = false
          this.$message.success('管理员模式已开启')
          // 重新拉取快捷连接：管理员可见 adminOnly 条目
          this.loadQuickServers()
          const pending = this.pendingAfterUnlock
          this.pendingAfterUnlock = ''
          if (pending === 'connect') {
            this.doConnect()
          } else if (pending && pending !== '') {
            this.selectProtocol(pending)
          }
        } else {
          this.$message.error((res && res.Msg) || '管理员密码错误')
        }
      }).catch(() => {
        this.$message.error('验证请求失败，请稍后重试')
      }).finally(() => {
        this.adminLoading = false
      })
    },
    exitAdmin () {
      adminLogout().then(() => {
        this.admin.isAdmin = false
        this.$message.success('已退出管理员模式')
        this.loadQuickServers()
      }).catch(() => {
        this.$message.error('退出管理员模式失败')
      })
    },
    // ---- 快捷连接 ----
    loadQuickServers () {
      getQuickServers().then(list => {
        this.quickServers = Array.isArray(list) ? list : []
      }).catch(() => { /* 加载失败静默，不影响主表单 */ })
    },
    // 点击快捷按钮：回填目标（IP 仅回填不展示），清空凭据并聚焦用户名
    fillFromQuick (s) {
      if (this.sshInfo.protocol !== 'ssh') {
        this.selectProtocol('ssh')
      }
      this.sshInfo.hostname = s.host || ''
      this.sshInfo.port = Number(s.port) || 22
      this.sshInfo.username = ''
      this.sshInfo.password = ''
      this.generatedLink = ''
      this.$message.success(`已载入 ${s.name}，请输入用户名密码连接`)
      this.$nextTick(() => {
        this.$refs.usernameInput && this.$refs.usernameInput.focus()
      })
    },
    openServerEditor () {
      getServerDetail().then(res => {
        const list = (res && res.Data) || []
        this.editableServers = list.map(s => ({
          name: s.name || '',
          host: s.host || '',
          port: Number(s.port) || 22,
          adminOnly: !!s.adminOnly,
          enabled: !s.disabled
        }))
        this.serverEditorVisible = true
      }).catch(err => {
        this.$message.error((err && err.data && err.data.Msg) || '读取快捷连接配置失败')
      })
    },
    addServer () {
      this.editableServers.push({ name: '', host: '', port: 22, adminOnly: false, enabled: true })
    },
    removeServer (idx) {
      this.editableServers.splice(idx, 1)
    },
    saveServers () {
      const rows = this.editableServers
        .filter(s => (s.name || '').trim() || (s.host || '').trim())
        .map(s => ({
          name: (s.name || '').trim(),
          host: (s.host || '').trim(),
          port: Number(s.port) || 22,
          adminOnly: !!s.adminOnly,
          disabled: !s.enabled
        }))
      for (const s of rows) {
        if (!s.name || !s.host) {
          this.$message.error('名称与主机地址都必须填写！')
          return
        }
      }
      this.serverSaving = true
      saveQuickServers(rows).then(res => {
        if (res && res.Msg === 'success') {
          this.$message.success('快捷连接已保存')
          this.serverEditorVisible = false
          this.loadQuickServers()
        } else {
          this.$message.error((res && res.Msg) || '保存失败')
        }
      }).catch(() => {
        this.$message.error('保存请求失败，请稍后重试')
      }).finally(() => {
        this.serverSaving = false
      })
    },
    // 点击协议卡片：受门禁保护的协议先解锁再切换
    onPickProtocol (value) {
      if (this.sshInfo.protocol === value) return
      if (this.isProtocolLocked(value)) {
        this.openAdminDialog(value)
        return
      }
      this.selectProtocol(value)
    },
    // 切换协议：仅在端口为空或仍是上一协议默认端口时才替换，
    // 避免覆盖用户手填的自定义端口；同时清理该协议不适用的字段。
    selectProtocol (value) {
      if (this.sshInfo.protocol === value) return
      const prev = protocolSpec(this.sshInfo.protocol)
      const next = protocolSpec(value)
      const currentPort = Number(this.sshInfo.port)
      if (!currentPort || currentPort === prev.defaultPort) {
        this.sshInfo.port = next.defaultPort
      }
      this.sshInfo.protocol = value
      if (!next.needPrivateKey) {
        this.sshInfo.privateKey = ''
        this.sshInfo.passphrase = ''
        this.privateKeyFileName = ''
      }
      if (!next.needCommand) {
        this.sshInfo.command = ''
      }
      if (!next.needDomain) {
        this.sshInfo.domain = ''
      }
      if (!next.needUsername) {
        this.sshInfo.username = ''
      }
      this.generatedLink = ''
    },
    // 按协议校验必填项，返回 true 表示通过
    validate () {
      const spec = this.currentProtocol
      if (!this.sshInfo.hostname) {
        this.$message.error('请输入主机地址！')
        this.$nextTick(() => {
          this.$refs.hostnameInput && this.$refs.hostnameInput.focus()
        })
        return false
      }
      if (spec.needUsername && !this.sshInfo.username) {
        this.$message.error(`请输入${spec.label}用户名！`)
        this.$nextTick(() => {
          this.$refs.usernameInput && this.$refs.usernameInput.focus()
        })
        return false
      }
      if (this.sshInfo.protocol === 'ssh') {
        if (!this.sshInfo.password && !this.sshInfo.privateKey) {
          this.$message.error('请输入密码或上传密钥！')
          this.$nextTick(() => {
            this.$refs.passwordInput && this.$refs.passwordInput.focus()
          })
          return false
        }
      } else if (this.sshInfo.protocol === 'rdp' && !this.sshInfo.password) {
        // VNC 允许空密码（部分服务器使用 None 安全类型），RDP 必须提供凭据
        this.$message.error('请输入 Windows 登录密码！')
        this.$nextTick(() => {
          this.$refs.passwordInput && this.$refs.passwordInput.focus()
        })
        return false
      }
      return true
    },
    buildConnectionInfo () {
      return {
        protocol: this.sshInfo.protocol,
        hostname: this.sshInfo.hostname,
        port: this.sshInfo.port || this.currentProtocol.defaultPort,
        username: this.sshInfo.username,
        password: this.sshInfo.password || '',
        domain: this.sshInfo.domain || '',
        privateKey: this.sshInfo.privateKey || '',
        passphrase: this.sshInfo.passphrase || '',
        command: this.sshInfo.command || ''
      }
    },
    onConnect () {
      // 管理员门禁：受保护的远程桌面协议需先解锁
      if (this.needAdminUnlock) {
        this.openAdminDialog('connect')
        return
      }
      this.doConnect()
    },
    doConnect () {
      // 清除之前的认证信息
      sessionStorage.removeItem('sshInfo')

      if (!this.validate()) return
      // 根据实际使用的登录方式清理未使用的认证信息（仅 SSH 支持密钥登录）
      if (this.sshInfo.privateKey && this.sshInfo.privateKey.trim()) {
        this.sshInfo.password = ''
      } else if (this.sshInfo.password) {
        // 使用密码登录时，清除密钥相关信息
        this.sshInfo.privateKey = ''
        this.sshInfo.passphrase = ''
        this.privateKeyFileName = ''
      }

      // 保存完整连接信息到 localStorage
      localStorage.setItem('connectionInfo', JSON.stringify(this.buildConnectionInfo()))

      const spec = this.currentProtocol
      // 安全：URL 只携带目标与协议，凭据放 sessionStorage（连接页自动回退读取），
      // 避免用户名密码出现在链接、浏览器历史与服务端访问日志中
      sessionStorage.setItem('sshInfo', JSON.stringify(this.sshInfo))
      const query = {
        protocol: spec.value,
        hostname: encodeURIComponent(this.sshInfo.hostname),
        port: Number(this.sshInfo.port) || spec.defaultPort,
        useKey: 1
      }

      // 按协议在对应控制台中打开：ssh→终端，rdp/vnc→远程桌面
      const url = this.$router.resolve({ path: PROTOCOL_ROUTES[spec.value], query }).href
      window.open(url, '_blank')
    },
    onReset () {
      // 清除表单数据（保留当前所选协议，仅清空连接目标）
      this.sshInfo = {
        protocol: this.sshInfo.protocol,
        hostname: '',
        port: '',
        username: '',
        password: '',
        domain: '',
        command: '',
        privateKey: '',
        passphrase: ''
      }
      this.privateKeyFileName = ''
      this.generatedLink = ''

      // 清除所有存储的认证信息
      localStorage.removeItem('connectionInfo')
      sessionStorage.removeItem('sshInfo')

      // 清除文件输入框
      const fileInput = document.querySelector('.upload-key input[type="file"]')
      if (fileInput) {
        fileInput.value = ''
      }
    },
    onGenerateLink () {
      if (!this.validate()) return
      const spec = this.currentProtocol
      // 安全：快捷链接只保留协议与 IP/端口，不携带用户名密码；
      // 打开链接的人需要手动输入凭据才能连接
      const url = new URL(window.location.href)
      url.pathname = PROTOCOL_ROUTES[spec.value]
      url.search = new URLSearchParams({
        protocol: spec.value,
        hostname: this.sshInfo.hostname,
        port: Number(this.sshInfo.port) || spec.defaultPort
      }).toString()
      this.generatedLink = url.href
      this.$message.success('链接已生成（仅含地址端口，不含凭据）')
    },
    copyLink () {
      if (this.generatedLink) {
        navigator.clipboard.writeText(this.generatedLink).then(() => {
          this.$message.success('链接已复制！')
        }).catch(err => {
          this.$message.error('复制失败: ' + err)
        })
      }
    },
    toggleTheme () {
      this.isDarkTheme = !this.isDarkTheme;
      localStorage.setItem('isDarkTheme', this.isDarkTheme);
      // 同步到 html 根元素，使 body/#app 背景跟随暗黑模式
      document.documentElement.classList.toggle('dark-theme', this.isDarkTheme);
    },
    handlePrivateKeyUpload(file) {
      // 上传密钥时清除密码，确保使用密钥登录
      this.sshInfo.password = ''
      
      const reader = new FileReader()
      reader.onload = (e) => {
        this.sshInfo.privateKey = e.target.result
        this.privateKeyFileName = file.name
      }
      reader.readAsText(file)
      return false // 阻止自动上传
    }
  }
}
</script>

<style lang="scss" scoped>
.login-container ::v-deep .el-input__inner {
  font-size: medium;
  border-radius: 10px;
  background: hsl(0deg 0% 100% / 5%);
  backdrop-filter: blur(5px);
  -webkit-backdrop-filter: blur(5px);
  border: 1px solid rgba(255, 255, 255, 0.3);
  transition: all 0.3s;
  color: #333;
}

.login-container ::v-deep .el-input__inner:focus {
  border-color: #409eff !important;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2) !important;
  caret: 2px solid #409eff !important;
}

.login-container ::v-deep .el-input.is-focus .el-input__inner {
  border-color: #409eff !important;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2) !important;
  caret: 2px solid #409eff !important;
}

.login-container ::v-deep .el-input__inner::placeholder {
  color: #565454 !important;
  opacity: 1;
}

/* 密码显示/隐藏按钮样式 */
.login-container ::v-deep .el-input__suffix {
  background: transparent;
  margin-right: 5px;
}

.login-container ::v-deep .el-input__suffix .el-input__suffix-inner {
  display: flex;
  align-items: center;
  justify-content: center;
}

.login-container ::v-deep .el-input__suffix .el-input__suffix-inner .el-input__icon {
  color: #666;
  font-size: 16px;
  transition: color 0.3s;
}

.login-container ::v-deep .el-input__suffix .el-input__suffix-inner .el-input__icon:hover {
  color: #409eff;
}

.login-container.dark-theme ::v-deep .el-input__inner {
  background: hsl(0deg 0% 100% / 5%);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
}

.login-container.dark-theme ::v-deep .el-input__inner:focus {
  border-color: #409eff !important;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2) !important;
  caret: 2px solid #409eff !important;
}

.login-container.dark-theme ::v-deep .el-input.is-focus .el-input__inner {
  border-color: #409eff !important;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2) !important;
  caret: 2px solid #409eff !important;
}

.login-container.dark-theme ::v-deep .el-input__inner::placeholder {
  color: #ccc !important;
  opacity: 1;
}

/* 深色主题密码显示/隐藏按钮样式 */
.login-container.dark-theme ::v-deep .el-input__suffix {
  background: transparent;
  margin-right: 5px;
}

.login-container.dark-theme ::v-deep .el-input__suffix .el-input__suffix-inner .el-input__icon {
  color: #ccc;
  font-size: 16px;
  transition: color 0.3s;
}

.login-container.dark-theme ::v-deep .el-input__suffix .el-input__suffix-inner .el-input__icon:hover {
  color: #409eff;
}

.login-container ::v-deep .el-form-item {
  margin-bottom: 12px;
}

.login-container {
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  align-items: center;
  background: var(--bg-color);
  background-image: var(--bg-image);
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
  background-attachment: fixed;
  position: relative;
  padding-top: 4vh;
  padding-bottom: 60px;
  transition: background-color 0.3s, color 0.3s, background-image 0.3s;
  overflow-y: auto;
}

.card {
  background: var(--card-bg);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  box-shadow: var(--shadow);
  border-radius: 20px;
  padding-top: 12px;
  padding-bottom: 18px;
  width: 100%;
  max-width: 42rem;
  position: relative;
  transition: background-color 0.3s, box-shadow 0.3s, backdrop-filter 0.3s;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.title {
  text-align: center;
  font-size: 2rem;
  font-weight: 800;
  color: var(--title-color);
  margin-bottom: 1.4rem;
  letter-spacing: 1px;
  position: relative;
  padding-bottom: 0.8rem;
  font-family: none;
  transition: color 0.3s;
}

.title::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 300px;
  height: 4px;
  background-color: var(--title-color);
  border-radius: 2px;
  transition: background-color 0.3s;
}

.form-grid ::v-deep .el-form-item__label {
  padding-bottom: 0;
  font-size: 15px;
  color: var(--text-color);
  line-height: 30px;
  transition: color 0.3s;
}

.form-grid ::v-deep .el-button {
  font-size: 1rem;
  font-weight: 600;
  padding: 0.9rem 1rem;
  border-radius: 10px;
  transition: all 0.3s;
}

/* ---- 协议选择器（SSH / RDP / VNC）：扁平紧凑按钮 ---- */
.protocol-picker {
  display: flex;
  gap: 8px;
  width: 100%;
}

.protocol-pill {
  flex: 1;
  cursor: pointer;
  padding: 7px 0;
  font-size: 14px;
  font-weight: 600;
  text-align: center;
  border-radius: 8px;
  border: 1px solid rgba(128, 128, 128, 0.4);
  background: transparent;
  color: var(--text-color);
  transition: all 0.2s;
  user-select: none;
  line-height: 1.4;
}

.protocol-pill:hover {
  border-color: var(--primary);
  color: var(--primary);
}

.protocol-pill.is-active {
  border-color: var(--primary);
  background: rgba(64, 158, 255, 0.14);
  color: var(--primary);
}

.protocol-lock {
  font-size: 11px;
  margin-left: 4px;
  opacity: 0.8;
}

.admin-dialog-tip {
  font-size: 13px;
  color: #666;
  margin-bottom: 12px;
  line-height: 1.6;
}

/* ---- 右上角管理员切换（主题按钮左侧） ---- */
.theme-switch-wrapper {
  position: absolute;
  top: 25px;
  right: 30px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 10px;
}

.admin-toggle {
  border-radius: 8px;
  font-weight: 600;
}
.admin-toggle.is-on {
  cursor: pointer;
}

/* ---- 快捷连接：仅展示名称（隐藏 IP） ---- */
.quick-servers {
  width: 100%;
  max-width: 42rem;
  margin: 12px auto 0;
  padding: 0 4px;
}

.qs-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.qs-title {
  font-size: 13px;
  color: var(--text-color);
  opacity: 0.65;
}

.qs-edit {
  padding: 3px 6px;
}

.qs-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.qs-btn {
  cursor: pointer;
  border: 1px solid rgba(128, 128, 128, 0.4);
  border-radius: 8px;
  background: transparent;
  color: var(--text-color);
  padding: 6px 14px;
  font-size: 13px;
  line-height: 1.4;
  text-align: center;
  transition: all 0.2s;
  user-select: none;
}

.qs-btn:hover {
  border-color: var(--primary);
  color: var(--primary);
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.25);
}

.qs-btn b {
  display: block;
  font-size: 14px;
}

/* 仅管理员条目：黄色边框标识（不再显示文字标签） */
.qs-btn.is-admin-only {
  border-color: rgba(255, 180, 84, 0.75);
}
.qs-btn.is-admin-only:hover {
  border-color: #ffb454;
  color: #ffb454;
  box-shadow: 0 2px 8px rgba(255, 180, 84, 0.25);
}

.qs-empty {
  font-size: 12px;
  color: var(--text-color);
  opacity: 0.5;
}

/* ---- 快捷连接编辑浮窗 ---- */
.server-editor-tip {
  font-size: 12.5px;
  color: #888;
  margin-bottom: 10px;
  line-height: 1.6;
}

.srv-head-row,
.srv-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.srv-head-row {
  font-size: 12px;
  color: #999;
  margin-bottom: 6px;
}

.srv-row {
  margin-bottom: 8px;
}
.srv-row.is-disabled {
  opacity: 0.55;
}

.srv-col-name { flex: 0 0 118px; }
.srv-col-host { flex: 1; min-width: 0; }
.srv-col-port { flex: 0 0 72px; }
.srv-row .srv-col-state,
.srv-row .srv-col-admin {
  flex: 0 0 62px;
  display: flex;
  justify-content: center;
  align-items: center;
}
.srv-head-row .srv-col-state,
.srv-head-row .srv-col-admin {
  flex: 0 0 62px;
  text-align: center;
}
.srv-col-op { flex: 0 0 32px; text-align: center; }

.server-editor-dialog .el-dialog__body {
  max-height: 55vh;
  overflow-y: auto;
}

.login-container ::v-deep .el-form-item .el-upload.upload-key {
  width: 100% !important;
  height: 48px !important;
  background: none !important;
  border: none !important;
  box-shadow: none !important;
  margin: 0 !important;
  padding: 0 !important;
}

.login-container ::v-deep .upload-flex-row {
  display: flex !important;
  flex-direction: row !important;
  align-items: stretch !important;
  width: 100%;
  height: 100%;
}

.login-container ::v-deep .upload-flex-row .upload-btn,
.login-container ::v-deep .upload-flex-row .upload-filename {
  height: 100%;
  display: flex;
  align-items: center;
}

.login-container ::v-deep .upload-btn {
  display: flex;
  align-items: center;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  font-size: 16px;
  border-radius: 10px 0 0 10px;
  padding: 0 28px;
  cursor: pointer;
  transition: background 0.2s;
  height: 100%;
  backdrop-filter: blur(5px);
  -webkit-backdrop-filter: blur(5px);
}

.login-container ::v-deep .upload-btn:hover {
  background: #202f3e;
}

.login-container ::v-deep .upload-filename {
  display: flex;
  align-items: center;
  background: hsl(0deg 0% 100% / 15%);
  backdrop-filter: blur(5px);
  -webkit-backdrop-filter: blur(5px);
  color: #333;
  font-size: 15px;
  border-radius: 0 12px 12px 0;
  padding: 0 10px;
  height: 100%;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border: 1px solid rgba(255, 255, 255, 0.3);
}

/* 移动端响应式优化*/
@media (max-width: 768px) {
  .card{
    width: 98% !important;
  }
  
  .form-grid ::v-deep .el-button {
    font-size: 0.9rem !important;
    padding: 0.7rem 0.8rem !important;
    margin: 0 2px !important;
  }
  
  .el-row[type="flex"] {
    margin-top: 8px !important;
  }

  .login-container {
    padding-bottom: 40px !important;
    min-height: auto !important;
  }

  .card {
    margin: 10px auto !important;
  }
}

.login-container.dark-theme ::v-deep .upload-key {
  border-color: #4d4d4d;
  background-color: var(--card-bg);
}
.login-container.dark-theme ::v-deep .upload-key .el-button {
  background: #232323;
  color: #409eff;
}
.login-container.dark-theme ::v-deep .upload-key .el-button:hover,
.login-container.dark-theme ::v-deep .upload-key .el-button:focus {
  background: #409eff !important;
  color: #fff !important;
}
.login-container.dark-theme ::v-deep .upload-key span {
  background: #23232339;
  color: #aaa !important;
}

.login-container.dark-theme ::v-deep .upload-filename {
  background: rgba(45, 45, 45, 0.2) !important;
  backdrop-filter: blur(5px) !important;
  -webkit-backdrop-filter: blur(5px) !important;
  color: #fff !important;
  border: 1px solid rgba(255, 255, 255, 0.2) !important;
}

.theme-switch {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background-color 0.3s;
}

.theme-switch i {
  margin-top: -20px;
  font-size: 20px;
  color: var(--icon-color);
  transition: color 0.3s;
}

/* Light theme variables */
.login-container {
  --bg-color: #ffff;
  --bg-image: none;
  --card-bg: hsl(0deg 0% 100% / 15%);
  --title-color: #1b58c9; /* Darker blue for title */
  --text-color: #3b3d3d;
  --shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  --success: #13af54;
  --success-hover: #0e8942; 
  --danger: #d63031;
  --danger-hover: #b247c2; 
  --primary: #409eff;
  --primary-hover: #0f9281; 
  --switch-bg: #f0f0f0;
  --icon-color: #232323;
}

/* Dark theme variables */
.login-container.dark-theme {
  --bg-color: #ffffff;
  --bg-image: none;
  --card-bg: hsl(0deg 0% 100% / 5%);
  --title-color: #ffffff;
  --text-color: #e0e0e0;
  --shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
  --success: #13af54;
  --success-hover: #0e8942;
  --danger: #d63031;
  --danger-hover: #b247c2;
  --primary: #409eff;
  --primary-hover: #0f9281;
  --switch-bg: #333333;
  --icon-color: #f5f5f5;
}

/* Button styles with custom hover colors */
.el-button--success {
  background-color: var(--success);
  border-color: var(--success);
  color: white;
}

.el-button--danger {
  background-color: var(--danger);
  border-color: var(--danger);
  color: white;
}

.el-button--primary {
  background-color: var(--primary);
  border-color: var(--primary);
  color: white;
}

.el-button--success:hover {
  background-color: var(--success-hover);
  border-color: var(--success-hover);
  color: white;
}

.el-button--danger:hover {
  background-color: var(--danger-hover);
  border-color: var(--danger-hover);
  color: white;
}

.el-button--primary:hover {
  background-color: var(--primary-hover);
  border-color: var(--primary-hover);
  color: white;
}

.login-container ::v-deep .gen-link-input .el-input__inner {
  border-radius: 10px 0 0 10px !important;
  padding-right: 5px;
}
.login-container ::v-deep .gen-link-input .el-input-group__append {
  border-radius: 0 10px 10px 0 !important;
}

.login-container ::v-deep .el-input-group__append {
  background-color: rgba(255, 255, 255, 0.2) !important;
  backdrop-filter: blur(5px) !important;
  -webkit-backdrop-filter: blur(5px) !important;
  border: 1px solid rgba(255, 255, 255, 0.3) !important;
  transition: background-color 0.3s;
}
.login-container.dark-theme ::v-deep .el-input-group__append {
  background-color: rgba(45, 45, 45, 0.2) !important;
  backdrop-filter: blur(5px) !important;
  -webkit-backdrop-filter: blur(5px) !important;
  border: 1px solid rgba(255, 255, 255, 0.2) !important;
}
</style>

<style lang="scss">
/* ===== 弹窗暗黑适配（非 scoped）=====
   append-to-body 会把弹窗挂到 body 下、脱离 .login-container 作用域，
   Element 的弹窗外框（背景/标题/关闭按钮）拿不到组件内的暗黑变量，需在此全局覆盖。
   .dark-theme 由 index.html 内联脚本同步到 <html> 上。 */
.admin-dialog.el-dialog,
.server-editor-dialog.el-dialog {
  border-radius: 12px;
}

html.dark-theme .admin-dialog.el-dialog,
html.dark-theme .server-editor-dialog.el-dialog {
  background: #232a34;
  border: 1px solid rgba(255, 255, 255, 0.12);
}

html.dark-theme .admin-dialog .el-dialog__title,
html.dark-theme .server-editor-dialog .el-dialog__title {
  color: #e6edf3;
}

html.dark-theme .admin-dialog .el-dialog__headerbtn .el-dialog__close,
html.dark-theme .server-editor-dialog .el-dialog__headerbtn .el-dialog__close {
  color: #9aa7b4;
}

html.dark-theme .admin-dialog .admin-dialog-tip,
html.dark-theme .server-editor-dialog .server-editor-tip {
  color: #9aa7b4;
}

html.dark-theme .server-editor-dialog .srv-head-row {
  color: #8a94a3;
}

html.dark-theme .admin-dialog .el-input__inner {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #e6edf3;
}
</style>
