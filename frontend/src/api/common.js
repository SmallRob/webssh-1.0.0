import request from '@/utils/request'
export function checkSSH(sshInfo) {
    return request.get(`/check?sshInfo=${sshInfo}`)
}

// 管理员模式：状态查询 / 解锁 / 退出
export function getAdminStatus() {
    return request.get('/admin/status')
}
export function adminLogin(password) {
    return request.post('/admin/login', { password })
}
export function adminLogout() {
    return request.post('/admin/logout')
}

// 快捷连接：公开列表 / 管理员读取全部 / 管理员保存（增删改）
export function getQuickServers() {
    return request.get('/servers')
}
export function getServerDetail() {
    return request.get('/servers/detail')
}
export function saveQuickServers(servers) {
    return request.post('/servers/save', servers)
}
