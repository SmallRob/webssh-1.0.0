package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net"
	"time"
	"webssh/core"
)

// ResponseBody 响应信息结构体
type ResponseBody struct {
	Duration string
	Data     interface{}
	Msg      string
}

// TimeCost 计算方法执行耗时
func TimeCost(start time.Time, body *ResponseBody) {
	body.Duration = time.Since(start).String()
}

// CheckSSH 检查ssh连接是否能连接
func CheckSSH(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	sshInfo := c.DefaultQuery("sshInfo", "")
	sshClient, err := core.DecodedMsgToSSHClient(sshInfo)
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
		return &responseBody
	}

	err = sshClient.GenerateClient()
	defer sshClient.Close()

	if err != nil {
		fmt.Println(err)
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CheckConnection 按协议分派连通性检查：
//   - ssh：执行完整的 SSH 认证，能验证账号密码/密钥是否正确；
//   - rdp / vnc：只做 TCP 可达性探测。这两种协议的认证发生在各自协议内部
//     （RDP 走 CredSSP/NLA，VNC 走 RFB 安全类型协商），过早校验会打断
//     keep-alive 之外的会话语义，因此与 shield-cli 一样只确认端口可达。
func CheckConnection(c *gin.Context) *ResponseBody {
	sshInfo := c.DefaultQuery("sshInfo", "")
	client, err := core.DecodedMsgToSSHClient(sshInfo)
	if err != nil {
		responseBody := ResponseBody{Msg: err.Error(), Duration: "0s"}
		return &responseBody
	}

	if !client.Proto().IsRemoteDesktop() {
		return CheckSSH(c)
	}

	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", client.Addr(), timeout)
	if err != nil {
		fmt.Println(err)
		responseBody.Msg = fmt.Sprintf("无法连接 %s: %v", client.Addr(), err)
		return &responseBody
	}
	_ = conn.Close()
	return &responseBody
}
