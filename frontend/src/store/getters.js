import { b64EncodeUtf8 } from '@/utils/codec'

export default {
    sshReq: state => {
        const sshInfo = {
            protocol: state.sshInfo.protocol || 'ssh',
            hostname: state.sshInfo.hostname,
            port: Number(state.sshInfo.port),
            username: state.sshInfo.username,
            logintype: state.sshInfo.privateKey && state.sshInfo.privateKey.trim() ? 1 : 0 // 只有当privateKey存在且不为空时才使用密钥登录
        }
        if (state.sshInfo.password) {
            sshInfo.password = state.sshInfo.password
        }
        if (state.sshInfo.domain) {
            sshInfo.domain = state.sshInfo.domain
        }
        if (state.sshInfo.privateKey && state.sshInfo.privateKey.trim()) {
            sshInfo.privateKey = state.sshInfo.privateKey
        }
        if (state.sshInfo.passphrase) {
            sshInfo.passphrase = state.sshInfo.passphrase
        }
        const jsonStr = JSON.stringify(sshInfo)
        return b64EncodeUtf8(jsonStr)
    }
}
