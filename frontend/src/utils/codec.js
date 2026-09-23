// Base64 编解码工具。
//
// 原生 btoa/atob 只接受 Latin-1，遇到中文主机名或用户名会直接抛
// InvalidCharacterError。这里统一做 UTF-8 往返，保证与后端
// base64(JSON) 的契约对 ASCII 输入完全等价、对非 ASCII 输入可用。

export function b64EncodeUtf8(str) {
    return window.btoa(unescape(encodeURIComponent(str)))
}

export function b64DecodeUtf8(b64) {
    return decodeURIComponent(escape(window.atob(b64)))
}
