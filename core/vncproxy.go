package core

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// vncDialTimeout VNC 目标连接超时。
const vncDialTimeout = 10 * time.Second

// ServeVNC 处理浏览器端 noVNC 发起的 VNC 连接。
//
// VNC 使用 RFB 协议，而 RFB 是「服务端先说话」的纯 TCP 协议：
// 版本协商、安全类型协商（VNC Auth / VeNCrypt / RA2 / None）、
// 以及各种编码（Tight / ZRLE / Hextile…）全部由浏览器端 noVNC 完成。
// 因此服务端只需做一条透明字节管道，不需要解析 RFB 报文，
// 这样也就自然支持了各种 VNC 变体与加密认证方式。
func (sclient *SSHClient) ServeVNC(ws *websocket.Conn) error {
	addr := sclient.Addr()

	conn, err := dialRemote(addr, vncDialTimeout)
	if err != nil {
		return fmt.Errorf("连接 VNC 服务器 %s 失败: %w", addr, err)
	}
	defer conn.Close()

	return relayWebSocketTCP(ws, conn)
}
