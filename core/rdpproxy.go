package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// rdpDialTimeout TCP 连接与 X.224 阶段的超时时间。
	rdpDialTimeout = 15 * time.Second
	// rdpHandshakeTimeout TLS 握手超时时间。
	rdpHandshakeTimeout = 15 * time.Second
	// rdpRequestTimeout 等待浏览器首个 RDCleanPath 请求的时限。
	rdpRequestTimeout = 30 * time.Second
)

// ServeRDP 处理浏览器端 ironrdp-wasm 发起的 RDP 连接。
//
// 与 shield-cli 相同，WebSSH 不自己实现 RDP 协议栈，
// 而是充当「RDCleanPath 代理」：把 RDP 协议的处理留给浏览器里的
// IronRDP(WASM)，服务端只负责 TCP/TLS 直连与字节中继。
// 这样既能复用成熟实现，又天然绕开了浏览器无法直接建立 TCP 的限制。
func (sclient *SSHClient) ServeRDP(ws *websocket.Conn) error {
	addr := sclient.Addr()

	// 1) 等待首个二进制帧：RDCleanPath 请求
	_ = ws.SetReadDeadline(time.Now().Add(rdpRequestTimeout))
	msgType, first, err := ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取 RDCleanPath 请求失败: %w", err)
	}
	_ = ws.SetReadDeadline(time.Time{})
	if msgType != websocket.BinaryMessage {
		err = fmt.Errorf("RDCleanPath 请求必须为二进制帧，实际类型 %d", msgType)
		_ = ws.WriteMessage(websocket.BinaryMessage, BuildRDCleanPathError(1, int64Ptr(400)))
		return err
	}

	req, err := ParseRDCleanPathRequest(first)
	if err != nil {
		// 回一个错误 PDU，让浏览器端能给出可读的失败原因
		_ = ws.WriteMessage(websocket.BinaryMessage, BuildRDCleanPathError(1, int64Ptr(400)))
		return err
	}

	// 2) 目标地址以服务端下发的连接信息为准。
	//    不采信请求体中的 destination，避免客户端篡改后把代理当作
	//    任意主机的跳板（与 shield-cli 只允许访问已登记资源的思路一致）。
	if host, port := parseRDPDestination(req.Destination); host != "" {
		if host != sclient.Hostname || port != sclient.Port {
			logRelayErr("rdp", addr, fmt.Errorf(
				"请求目标 %s 与授权目标 %s 不一致，已按授权目标连接", req.Destination, addr))
		}
	}

	// 3) TCP + X.224 + TLS 握手
	x224Response, certChain, tlsConn, err := sclient.rdpHandshake(req.X224ConnectionPDU)
	if err != nil {
		_ = ws.WriteMessage(websocket.BinaryMessage, BuildRDCleanPathError(1, int64Ptr(502)))
		return err
	}
	defer tlsConn.Close()

	// 4) 回发 RDCleanPath 响应（含 X.224 响应与服务器证书链）
	if err = ws.WriteMessage(websocket.BinaryMessage,
		BuildRDCleanPathResponse(addr, x224Response, certChain)); err != nil {
		return fmt.Errorf("回发 RDCleanPath 响应失败: %w", err)
	}

	// 5) WebSocket ↔ TLS 双向中继
	return relayWebSocketTCP(ws, tlsConn)
}

// rdpHandshake 完成 RDCleanPath 代理侧的网络握手：
// TCP 连接 → 发送 X.224 Connection Request → 读取 Connection Confirm →
// 在裸 TCP 上完成 TLS 握手 → 提取服务器证书链。
func (sclient *SSHClient) rdpHandshake(x224Request []byte) ([]byte, [][]byte, *tls.Conn, error) {
	addr := sclient.Addr()

	rawConn, err := dialRemote(addr, rdpDialTimeout)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("连接 RDP 服务器 %s 失败: %w", addr, err)
	}

	// X.224 阶段仍为明文，必须在 TLS 之前完成
	_ = rawConn.SetDeadline(time.Now().Add(rdpDialTimeout))
	if _, err = rawConn.Write(x224Request); err != nil {
		rawConn.Close()
		return nil, nil, nil, fmt.Errorf("发送 X.224 Connection Request 失败: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := rawConn.Read(buf)
	if err != nil {
		rawConn.Close()
		return nil, nil, nil, fmt.Errorf("读取 X.224 Connection Confirm 失败: %w", err)
	}
	if n == 0 {
		rawConn.Close()
		return nil, nil, nil, fmt.Errorf("RDP 服务器 %s 未返回 X.224 响应即关闭连接", addr)
	}
	x224Response := append([]byte(nil), buf[:n]...)
	_ = rawConn.SetDeadline(time.Time{})

	// RDP 服务器普遍使用自签名证书，浏览器端会依据回传的证书链自行校验，
	// 因此代理侧不做证书链校验（与 ironrdp-wasm 官方代理行为一致）。
	host, _ := sclient.rdpHostPort()
	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS10,
		ClientSessionCache: nil,
	})

	ctx, cancel := context.WithTimeout(context.Background(), rdpHandshakeTimeout)
	defer cancel()
	if err = tlsConn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, nil, nil, fmt.Errorf("TLS 握手失败: %w", err)
	}

	var certChain [][]byte
	if state := tlsConn.ConnectionState(); len(state.PeerCertificates) > 0 {
		for _, cert := range state.PeerCertificates {
			certChain = append(certChain, cert.Raw)
		}
	}
	return x224Response, certChain, tlsConn, nil
}

// rdpHostPort 取出不含方括号的主机名与端口，用于 TLS ServerName。
func (sclient *SSHClient) rdpHostPort() (string, int) {
	return stripBrackets(sclient.Hostname), sclient.Port
}

// stripBrackets 去掉 IPv6 地址的方括号。
func stripBrackets(host string) string {
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		return host[1 : len(host)-1]
	}
	return host
}

func int64Ptr(v int64) *int64 { return &v }

// rdpAddrOf 便捷构造 host:port（IPv6 自动加方括号）。
func rdpAddrOf(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
