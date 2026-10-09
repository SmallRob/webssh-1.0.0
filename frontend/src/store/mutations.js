export default {
    SET_PASS(state, pass) {
        state.sshInfo.password = pass
    },
    SET_LIST(state, list) {
        state.sshList = list
        localStorage.setItem('sshList', list)
    },
    SET_TERMLIST(state, list) {
        state.termList = list
    },
    SET_SSH(state, ssh) {
        state.sshInfo.hostname = ssh.hostname
        state.sshInfo.port = ssh.port
        state.sshInfo.username = ssh.username
        if (ssh.protocol !== undefined) {
            state.sshInfo.protocol = ssh.protocol
        }
        if (ssh.domain !== undefined) {
            state.sshInfo.domain = ssh.domain
        }
        if (ssh.password !== undefined) {
            state.sshInfo.password = ssh.password
        }
        if (ssh.command !== undefined) {
            state.sshInfo.command = ssh.command
        }
        if (ssh.privateKey !== undefined) {
            state.sshInfo.privateKey = ssh.privateKey
        }
        if (ssh.passphrase !== undefined) {
            state.sshInfo.passphrase = ssh.passphrase
        }
    },
    SET_TAB(state, tab) {
        state.currentTab = tab
    },
    SET_CURRENT_PATH(state, path) {
        state.currentPath = path
    },
    SET_LANGUAGE: (state, language) => {
        state.language = language
        localStorage.setItem('language', language)
    },
    // ---------- 传输队列 ----------
    TRANSFER_ADD(state, item) {
        state.transfer.items.push(Object.assign({
            id: '',
            name: '',
            action: 'upload', // upload | download
            size: 0,
            source: '',
            target: '',
            percent: 0,
            status: 'uploading', // uploading | downloading | success | error
            error: ''
        }, item))
        state.transfer.visible = true
    },
    TRANSFER_PATCH(state, payload) {
        const item = state.transfer.items.find(i => i.id === payload.id)
        if (item) {
            Object.assign(item, payload.patch)
        }
    },
    TRANSFER_CLEAR(state) {
        // 只清已完成/失败项，进行中的传输保留
        const active = state.transfer.items.filter(i => i.status === 'uploading' || i.status === 'downloading')
        state.transfer.items = active
    },
    TRANSFER_VISIBLE(state, visible) {
        state.transfer.visible = visible
    }
}
