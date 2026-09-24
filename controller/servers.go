package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// 快捷服务器（servers.json）管理：
//   - 配置文件只维护 name/host/port/adminOnly，凭据一律不落盘；
//   - /servers 公开列表：剥离凭据字段，非管理员过滤掉 adminOnly 条目；
//   - /servers/detail 与 /servers/save 仅管理员可用（adminPass 未启用时拒绝写，
//     避免匿名篡改配置）；修改即时生效（每次请求实时读文件）。

const (
	// quickServerMax 限制单次保存的条目数，防止误操作写爆配置
	quickServerMax = 200
	// quickServerDefaultPort 端口缺省值
	quickServerDefaultPort = 22
)

// QuickServer 快捷服务器条目（servers.json 的标准结构）。
// Disabled 为 true 表示已下架：公开列表（所有人）都不返回，仅管理员编辑浮窗可见可恢复。
type QuickServer struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port,omitempty"`
	AdminOnly bool   `json:"adminOnly,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
}

// serversPath 返回 servers.json 的实际路径（SERVERS_FILE 可覆盖）。
func serversPath() string {
	if path := os.Getenv("SERVERS_FILE"); path != "" {
		return path
	}
	return "/webssh/servers.json"
}

// readRawServers 读取原始条目（保留全部字段，便于 detail 还原编辑）。
func readRawServers() ([]map[string]interface{}, error) {
	data, err := os.ReadFile(serversPath())
	if err != nil {
		return nil, err
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// stripCredentials 从原始条目中删除凭据字段，返回安全副本。
func stripCredentials(item map[string]interface{}) map[string]interface{} {
	safe := make(map[string]interface{}, len(item))
	for k, v := range item {
		lk := strings.ToLower(k)
		if lk == "username" || lk == "password" || lk == "privatekey" || lk == "passphrase" {
			continue
		}
		safe[k] = v
	}
	if safe["port"] == nil {
		safe["port"] = quickServerDefaultPort
	}
	return safe
}

// ListPublicServers GET /servers 的公开列表：
// 剥离凭据；过滤已禁用（disabled）条目；非管理员再过滤掉 adminOnly。
func ListPublicServers(c *gin.Context) []map[string]interface{} {
	raw, err := readRawServers()
	if err != nil {
		return []map[string]interface{}{}
	}
	isAdmin := IsAdmin(c)
	list := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if isAdminVal(item["disabled"]) {
			continue
		}
		if isAdminVal(item["adminOnly"]) && !isAdmin {
			continue
		}
		list = append(list, stripCredentials(item))
	}
	return list
}

// DetailServers GET /servers/detail：管理员编辑用，剥离凭据后返回全部条目。
func DetailServers(c *gin.Context) []map[string]interface{} {
	raw, err := readRawServers()
	if err != nil {
		return []map[string]interface{}{}
	}
	list := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		list = append(list, stripCredentials(item))
	}
	return list
}

// isAdminVal 宽松解析 adminOnly 字段（兼容 bool / 字符串 / 数值）。
func isAdminVal(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	case float64:
		return t != 0
	}
	return false
}

// SaveServerList 校验并保存快捷服务器列表（直接覆写 servers.json，即时生效）。
// 说明：文件可能是 bind mount 挂载点，rename 原子替换会失败，故原地覆写。
func SaveServerList(entries []QuickServer) error {
	if len(entries) > quickServerMax {
		return fmt.Errorf("条目数超过上限 %d", quickServerMax)
	}
	cleaned := make([]QuickServer, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		e.Name = strings.TrimSpace(e.Name)
		e.Host = strings.TrimSpace(e.Host)
		if e.Name == "" && e.Host == "" {
			continue // 跳过全空行
		}
		if e.Name == "" || e.Host == "" {
			return fmt.Errorf("名称与主机地址都必须填写")
		}
		if seen[e.Name] {
			return fmt.Errorf("名称重复: %s", e.Name)
		}
		seen[e.Name] = true
		if e.Port <= 0 || e.Port > 65535 {
			e.Port = quickServerDefaultPort
		}
		cleaned = append(cleaned, e)
	}
	data, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(serversPath(), data, 0644)
}

// RequireAdminAPI 管理类接口的统一门禁：未启用管理员密码时拒绝（403），
// 已启用时校验签名 Cookie（401）。失败时已写响应，调用方直接 return。
func RequireAdminAPI(c *gin.Context) bool {
	if adminPass == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, ResponseBody{
			Msg: "未启用管理员门禁（adminPass 未配置），快捷连接编辑不可用",
		})
		return false
	}
	if !hasAdminCookie(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ResponseBody{
			Msg: "需要管理员模式",
		})
		return false
	}
	return true
}
