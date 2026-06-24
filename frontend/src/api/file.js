import request from '@/utils/request'

export function fileList(path, sshInfo) {
    return request.get(`/file/list?path=${path}&sshInfo=${sshInfo}`)
}

export function deleteFile(path, sshInfo) {
    return request.get(`/file/delete?path=${encodeURIComponent(path)}&sshInfo=${sshInfo}`)
}

export function renameFile(oldPath, newPath, sshInfo) {
    return request.get(`/file/rename?oldPath=${encodeURIComponent(oldPath)}&newPath=${encodeURIComponent(newPath)}&sshInfo=${sshInfo}`)
}

export function createFolder(path, sshInfo) {
    return request.get(`/file/mkdir?path=${encodeURIComponent(path)}&sshInfo=${sshInfo}`)
}

export function readFile(path, sshInfo) {
    return request.get(`/file/read?path=${encodeURIComponent(path)}&sshInfo=${sshInfo}`)
}

export function saveFile(path, content, sshInfo) {
    const formData = new FormData()
    formData.append('path', path)
    formData.append('content', content)
    formData.append('sshInfo', sshInfo)
    return request.post('/file/save', formData)
}

export function searchFiles(keyword, path, sshInfo) {
    return request.get(`/file/search?keyword=${encodeURIComponent(keyword)}&path=${encodeURIComponent(path)}&sshInfo=${sshInfo}`)
}

export function getFileInfo(path, sshInfo) {
    return request.get(`/file/info?path=${encodeURIComponent(path)}&sshInfo=${sshInfo}`)
}
