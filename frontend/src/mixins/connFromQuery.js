// 从路由 query 还原连接信息并写入 vuex。
//
// 三种协议的页面（TerminalPage / RdpPage / VncPage）共用同一段
// 「解析 query → 回退 sessionStorage → commit SET_SSH」逻辑，
// 差异仅为默认协议，因此抽成工厂函数。
import { protocolSpec } from '@/utils/remote'

function safeAtob (value) {
    try {
        return window.atob(value)
    } catch (e) {
        return decodeURIComponent(value)
    }
}

export default function connFromQuery (protocol) {
    return {
        beforeCreate () {
            const spec = protocolSpec(protocol)
            const query = this.$route.query || {}

            let hostname = query.hostname
            let username = query.username
            let password = query.password
            let domain = query.domain
            let privateKey = query.privateKey
            let passphrase = query.passphrase
            let command = query.command

            if (hostname) hostname = decodeURIComponent(hostname)
            if (username) username = decodeURIComponent(username)
            if (domain) domain = decodeURIComponent(domain)
            if (command) command = decodeURIComponent(command)
            if (privateKey) privateKey = decodeURIComponent(privateKey)
            if (passphrase) passphrase = decodeURIComponent(passphrase)
            if (password) password = safeAtob(password)

            const useKey = query.useKey

            // 密钥登录 / query 不完整时，回退到 sessionStorage 中的完整信息
            const needFallback = useKey ||
                !hostname || !username && spec.needUsername || (!password && !privateKey && spec.needPassword)
            if (needFallback) {
                const saved = sessionStorage.getItem('sshInfo')
                if (saved) {
                    try {
                        const info = JSON.parse(saved)
                        hostname = info.hostname || hostname
                        username = info.username || username
                        password = info.password || password
                        domain = info.domain || domain
                        command = info.command || command
                        privateKey = info.privateKey || privateKey
                        passphrase = info.passphrase || passphrase
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
