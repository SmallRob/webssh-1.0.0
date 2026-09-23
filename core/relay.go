package core

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// relayBufferSize 中继缓冲区大小。远程桌面单帧（位图更新）可能较大，
// 用 64KB 可以减少系统调用次数。
const relayBufferSize = 64 * 1024

// relayWebSocketTCP 在 WebSocket 与任意 net.Conn 之间做双向透明字节中继。
//
// 返回值语义：
//   - 正常关闭（任一侧 EOF / 关闭）返回 nil；
//   - 读写异常返回对应 error，供调用方判断是否需要提示用户。
//
// 并发约束：gorilla/websocket 只允许一个 goroutine 调用写方法，
// 因此「TCP→WS」方向的写操作全部集中在该 goroutine 内完成，
// 主 goroutine 只负责「WS→TCP」方向的读。
func relayWebSocketTCP(ws *websocket.Conn, conn net.Conn) error {
	var (
		once     sync.Once
		closeErr error
		done     = make(chan struct{})
	)

	shutdown := func(err error) {
		once.Do(func() {
			closeErr = err
			_ = ws.Close()
			_ = conn.Close()
			close(done)
		})
	}

	// ---- TCP → WebSocket ----
	go func() {
		buf := make([]byte, relayBufferSize)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if wErr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); wErr != nil {
					shutdown(wErr)
					return
				}
			}
			if err != nil {
				shutdown(nil)
				return
			}
		}
	}()

	// ---- WebSocket → TCP ----
	go func() {
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				shutdown(nil)
				return
			}
			// 远程桌面通道只承载二进制帧；文本帧（心跳等）直接忽略。
			if msgType != websocket.BinaryMessage {
				continue
			}
			if _, err := conn.Write(data); err != nil {
				shutdown(err)
				return
			}
		}
	}()

	<-done
	return closeErr
}

// dialRemote 建立到目标地址的 TCP 连接，并按需打开 keepalive。
func dialRemote(addr string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
		_ = tcpConn.SetNoDelay(true)
	}
	return conn, nil
}

// logRelayErr 统一记录中继层的非致命错误。
func logRelayErr(protocol string, addr string, err error) {
	if err == nil {
		return
	}
	log.Printf("[%s] %s 通道结束: %v", protocol, addr, err)
}
