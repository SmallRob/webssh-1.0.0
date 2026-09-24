package controller

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"webssh/core"
)

// 管理员模式：用于门禁 RDP / VNC 远程桌面连接。
//
// 配置由 main 包在解析 flag / 环境变量后调用 InitAdmin 注入：
//   - adminPass 为空：门禁整体关闭，任何人可使用 RDP / VNC；
//   - adminPass 非空：命中开关的协议必须先 POST /admin/login 校验管理员密码，
//     之后浏览器携带签名的 HttpOnly Cookie（WebSocket 升级请求自动带上）。
//
// 令牌格式为「过期时间戳.HMAC-SHA256(过期时间戳)」，密钥每次启动随机生成，
// 重启后旧 Cookie 自动失效；无状态校验，无需服务端存储会话。
var (
	adminPass       string
	rdpNeedAdmin    bool
	vncNeedAdmin    bool
	adminSecret     = make([]byte, 32)
	adminCookieName = "webssh_admin"
	adminTokenTTL   = 8 * time.Hour
)

// InitAdmin 初始化管理员门禁配置；pass 为空表示整体关闭。
func InitAdmin(pass string, needRdp, needVnc bool) {
	adminPass = pass
	rdpNeedAdmin = needRdp
	vncNeedAdmin = needVnc
	if _, err := rand.Read(adminSecret); err != nil {
		panic(fmt.Sprintf("生成管理员令牌密钥失败: %v", err))
	}
}

// ProtocolNeedsAdmin 判断某协议是否要求管理员模式。
func ProtocolNeedsAdmin(p core.Protocol) bool {
	if adminPass == "" {
		return false
	}
	switch p {
	case core.ProtocolRDP:
		return rdpNeedAdmin
	case core.ProtocolVNC:
		return vncNeedAdmin
	}
	return false
}

// IsAdmin 判断当前请求是否具备管理员权限（未启用门禁时视为管理员，
// 与 /admin/status 的 isAdmin 语义保持一致）。
func IsAdmin(c *gin.Context) bool {
	return adminPass == "" || hasAdminCookie(c)
}

// signAdminToken 生成「过期时间.HMAC」格式的管理员令牌。
func signAdminToken(expiry int64) string {
	mac := hmac.New(sha256.New, adminSecret)
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	return fmt.Sprintf("%d.%s", expiry, hex.EncodeToString(mac.Sum(nil)))
}

func validAdminToken(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	mac := hmac.New(sha256.New, adminSecret)
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	return hmac.Equal([]byte(parts[1]), []byte(hex.EncodeToString(mac.Sum(nil))))
}

func hasAdminCookie(c *gin.Context) bool {
	token, err := c.Cookie(adminCookieName)
	if err != nil || token == "" {
		return false
	}
	return validAdminToken(token)
}

// AdminStatus GET /admin/status
// 返回管理员门禁是否启用、各协议开关以及当前会话是否已解锁。
func AdminStatus(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	responseBody.Data = gin.H{
		"enabled": adminPass != "",
		"rdp":     rdpNeedAdmin,
		"vnc":     vncNeedAdmin,
		"isAdmin": adminPass == "" || hasAdminCookie(c),
	}
	return &responseBody
}

// AdminLogin POST /admin/login  body: {"password": "..."}
// 校验管理员密码，成功后下发签名 Cookie。密码错误时返回 success=false，
// 并加入固定延时以缓解暴力枚举。
func AdminLogin(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		responseBody.Msg = "请求格式错误"
		responseBody.Data = gin.H{"success": false}
		return &responseBody
	}

	if adminPass != "" && hmac.Equal([]byte(req.Password), []byte(adminPass)) {
		expiry := time.Now().Add(adminTokenTTL).Unix()
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(adminCookieName, signAdminToken(expiry),
			int(adminTokenTTL.Seconds()), "/", "", false, true)
		responseBody.Data = gin.H{"success": true}
		return &responseBody
	}

	time.Sleep(time.Second)
	responseBody.Msg = "管理员密码错误"
	responseBody.Data = gin.H{"success": false}
	return &responseBody
}

// AdminLogout POST /admin/logout 清除管理员 Cookie，退出管理员模式。
func AdminLogout(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(adminCookieName, "", -1, "/", "", false, true)
	return &responseBody
}

// CheckAdminGate 在 WebSocket 升级前校验管理员门禁；
// 未解锁时写入 401 JSON 响应并返回 false（此时还未 Upgrade，可正常回 HTTP 错误）。
func CheckAdminGate(c *gin.Context, proto core.Protocol) bool {
	if !ProtocolNeedsAdmin(proto) || hasAdminCookie(c) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, ResponseBody{
		Msg: "需要管理员模式：请先输入管理员密码解锁远程桌面",
	})
	return false
}
