package controller

import (
	"fmt"
	"net/http"
	"time"
	"webssh/core"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// remoteUpgrader 用于 RDP / VNC 的 WebSocket 升级器。
// 远程桌面单帧可能远超 1KB，这里放宽读写缓冲以避免频繁扩容。
var remoteUpgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ProtocolList 返回内置协议清单，供前端渲染协议选择器。
// 对应 shield-cli 的 /api/protocols 能力。
func ProtocolList(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	responseBody.Data = core.ProtocolList()
	return &responseBody
}

// RemoteWs 是 RDP / VNC 的统一 WebSocket 入口。
//
// 通道契约（与 SSH 的 /term 保持一致，便于复用鉴权与链接生成逻辑）：
//
//	GET /rdp?sshInfo=<base64(JSON)>&...
//	GET /vnc?sshInfo=<base64(JSON)>&...
//
// 其中 JSON 为统一连接描述符：{protocol, hostname, port, username, password, domain}
// 升级为 WebSocket 之后，RDP 走 RDCleanPath 握手 + TLS 中继，
// VNC 走 RFB 透明字节中继。
func RemoteWs(c *gin.Context, proto core.Protocol) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	sshInfo := c.DefaultQuery("sshInfo", "")
	client, err := core.DecodedMsgToSSHClient(sshInfo)
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}

	// 以路由为准覆写协议，避免前端漏传 protocol 字段
	client.Protocol = string(proto)
	client.Normalize()

	if client.Hostname == "" {
		responseBody.Msg = "缺少主机地址"
		return &responseBody
	}
	if !proto.IsRemoteDesktop() {
		responseBody.Msg = fmt.Sprintf("不支持的协议: %s", proto)
		return &responseBody
	}

	wsConn, err := remoteUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}

	switch proto {
	case core.ProtocolRDP:
		err = client.ServeRDP(wsConn)
	case core.ProtocolVNC:
		err = client.ServeVNC(wsConn)
	}
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
	}
	return &responseBody
}
