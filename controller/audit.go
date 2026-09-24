package controller

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"webssh/core"
)

// 审计日志查看（仅管理员）：
//
// 管理模式下通过 SSH 在目标服务器上执行只读命令，直接查看审计日志：
//   GET  /audit/list?sshInfo=<b64>          列出审计目录下的日志文件
//   GET  /audit/tail?sshInfo=<b64>&file=xx  WebSocket 实时 tail -F 日志
//
// 配置（环境变量 / flag）：
//   AUDIT_DIR  审计日志目录，默认 /data/audit
//   AUDIT_USER 登录目标服务器的用户，默认 root
//   AUDIT_SSH_KEY 服务器本地私钥路径（可选）；挂载到容器内即可免密审计，
//              未配置时由管理员在前端输入一次 root 密码（不落盘、不记录）。
//
// 安全：
//   - 两个接口都要求管理员门禁（adminPass 未配置时一律 403）；
//   - 文件名白名单校验（字母数字 ._@-，禁止 .. 与路径分隔符），杜绝目录穿越；
//   - 命令固定为 ls / tail 只读模板，参数经单引号转义后拼接。

var (
	auditFileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]*$`)
)

// AuditDir 返回全局审计日志目录。
func AuditDir() string {
	if dir := os.Getenv("AUDIT_DIR"); dir != "" {
		return dir
	}
	return "/data/audit"
}

// auditUser 返回审计 SSH 登录用户。
func auditUser() string {
	if u := os.Getenv("AUDIT_USER"); u != "" {
		return u
	}
	return "root"
}

// auditPrivateKey 读取服务器本地私钥（AUDIT_SSH_KEY），不存在则返回空。
// 优先级：本地密钥 > 前端传入的密码/密钥。
func auditPrivateKey() string {
	keyPath := os.Getenv("AUDIT_SSH_KEY")
	if keyPath == "" {
		keyPath = "/webssh/audit_key"
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// prepareAuditClient 解析 sshInfo 并强制套用审计专用认证配置。
func prepareAuditClient(c *gin.Context) (core.SSHClient, error) {
	client, err := core.DecodedMsgToSSHClient(c.Query("sshInfo"))
	if err != nil {
		return client, fmt.Errorf("连接参数无效: %v", err)
	}
	// 以服务端配置为准：协议与用户名不可被客户端改写
	client.Protocol = string(core.ProtocolSSH)
	client.Username = auditUser()
	client.Normalize()
	if key := auditPrivateKey(); key != "" {
		client.PrivateKey = key
		client.LoginType = 1
		client.Passphrase = os.Getenv("AUDIT_KEY_PASSPHRASE")
	} else if client.PrivateKey != "" {
		client.LoginType = 1
	} else {
		client.LoginType = 0
	}
	return client, nil
}

// shellQuote 单引号转义，用于拼接只读命令参数。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// AuditList GET /audit/list：列出审计目录下的日志文件（按修改时间倒序）。
// 响应统一由本函数写出：门禁失败为 403/401，其余为 200 + Msg。
func AuditList(c *gin.Context) {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	if adminPass == "" {
		c.JSON(http.StatusForbidden, ResponseBody{Msg: "未启用管理员门禁（adminPass 未配置），审计功能不可用"})
		return
	}
	if !hasAdminCookie(c) {
		c.JSON(http.StatusUnauthorized, ResponseBody{Msg: "需要管理员模式"})
		return
	}

	client, err := prepareAuditClient(c)
	if err != nil {
		responseBody.Msg = err.Error()
		c.JSON(http.StatusBadRequest, responseBody)
		return
	}
	if err := client.GenerateClient(); err != nil {
		responseBody.Msg = "SSH 连接失败: " + err.Error()
		c.JSON(http.StatusOK, responseBody)
		return
	}
	defer client.Close()

	session, err := client.Client.NewSession()
	if err != nil {
		responseBody.Msg = "创建会话失败: " + err.Error()
		c.JSON(http.StatusOK, responseBody)
		return
	}
	defer session.Close()

	out, err := session.CombinedOutput(fmt.Sprintf("ls -1t -- %s 2>/dev/null || echo __AUDIT_DIR_MISSING__", shellQuote(AuditDir())))
	if err != nil {
		responseBody.Msg = "读取目录失败: " + err.Error()
		c.JSON(http.StatusOK, responseBody)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "__AUDIT_DIR_MISSING__" {
			responseBody.Msg = fmt.Sprintf("目标服务器上不存在审计目录 %s", AuditDir())
			responseBody.Data = []string{}
			c.JSON(http.StatusOK, responseBody)
			return
		}
		if auditFileRe.MatchString(line) {
			files = append(files, line)
		}
	}
	responseBody.Data = files
	c.JSON(http.StatusOK, responseBody)
}

// AuditTailWs GET /audit/tail：WebSocket 实时输出日志（tail -n 500 -F）。
// 浏览器断开或后端关闭时自动清理 SSH 会话；ping 帧维持 nginx 代理不断链。
func AuditTailWs(c *gin.Context) {
	if adminPass == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, ResponseBody{Msg: "未启用管理员门禁（adminPass 未配置），审计功能不可用"})
		return
	}
	if !hasAdminCookie(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ResponseBody{Msg: "审计日志仅管理员可查看"})
		return
	}
	file := path.Clean(c.Query("file"))
	if !auditFileRe.MatchString(file) {
		c.JSON(200, ResponseBody{Msg: "非法的日志文件名"})
		return
	}

	client, err := prepareAuditClient(c)
	if err != nil {
		c.JSON(200, ResponseBody{Msg: err.Error()})
		return
	}
	if err := client.GenerateClient(); err != nil {
		c.JSON(200, ResponseBody{Msg: "SSH 连接失败: " + err.Error()})
		return
	}
	defer client.Close()

	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	command := fmt.Sprintf("tail -n 500 -F -- %s", shellQuote(path.Join(AuditDir(), file)))
	done := make(chan struct{})
	// 30s 一次 ping 保活（浏览器自动 pong，nginx 计时刷新）
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
		}
	}()
	// 读端：浏览器关闭时立即终止会话
	go func() {
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				close(done)
				client.Close()
				return
			}
		}
	}()

	_ = client.RunStream(command, wsConn)
	close(done)
	wsConn.Close()
}
