// 从路由 query 还原连接信息并写入 vuex。
//
// 三种协议的页面（TerminalPage / RdpPage / VncPage）共用同一段
// 「解析 query → 回退 sessionStorage → commit SET_SSH」逻辑，
// 差异仅为默认协议，因此抽成工厂函数。
//
// 安全约定：快捷链接 / 连接跳转的 URL 只允许携带目标（协议、地址、端口），
// password / privateKey / passphrase 一律不接受 URL 传参，
// 凭据统一通过 sessionStorage（连接页跳转时写入）回退获取，
// 避免用户名密码进入链接、浏览器历史与服务端访问日志。
import { protocolSpec } from '@/utils/remote'

export default function connFromQuery (protocol) {
    return {
        beforeCreate () {
            const spec = protocolSpec(protocol)
            const query = this.$route.query || {}

            let hostname = query.hostname
            let username = query.username
            let domain = query.domain
            let command = query.command

            if (hostname) hostname = decodeURIComponent(hostname)
            if (username) username = decodeURIComponent(username)
            if (domain) domain = decodeURIComponent(domain)
            if (command) command = decodeURIComponent(command)

            let password = ''
            let privateKey = ''
            let passphrase = ''

            // query 不完整时，回退到 sessionStorage 中的完整信息
            const needFallback = !hostname || !username && spec.needUsername ||
                (!password && !privateKey && spec.needPassword)
            if (needFallback) {
                const saved = sessionStorage.getItem('sshInfo')
                if (saved) {
                    try {
                        const info = JSON.parse(saved)
                        hostname = info.hostname || hostname
                        username = info.username || username
                        password = info.password || ''
                        domain = info.domain || domain
                        command = info.command || command
                        privateKey = info.privateKey || ''
                        passphrase = info.passphrase || ''
                    } catch (e) { /* 忽略损坏的缓存 */ }
                }
            }

            const hasAuth = password || privateKey || !spec.needPassword
            if (hostname && hasAuth) {
                this.$store.commit('SET_SSH', {
                    protocol: spec.value,
                    hostname,
                    port: Number(query.port) || spec.defaultPort,
                    username: username || '',
                    password: password || '',
                    domain: domain || '',
                    command: command || '',
                    privateKey: privateKey || '',
                    passphrase: passphrase || ''
                })
            }
        }
    }
}
