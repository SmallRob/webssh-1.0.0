import { getLanguage } from '@/lang/index'

const state = () => ({
    sshInfo: {
        // 协议：ssh / rdp / vnc
        protocol: 'ssh',
        hostname: '',
        username: '',
        port: '',
        password: '',
        // RDP 专用：Windows 域 / 工作组
        domain: '',
        command: ''
    },
    sshList: Object.prototype.hasOwnProperty.call(localStorage, 'sshList') ? localStorage.getItem('sshList') : null,
    termList: [],
    currentTab: {},
    currentPath: '/',
    language: getLanguage(),
    // 传输队列（上传/下载）：全局共享，供传输浮窗展示
    transfer: {
        items: [],
        visible: false
    }
})

export default state
