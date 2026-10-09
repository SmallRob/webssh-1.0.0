package controller

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 全局预制命令（presets.json）：
//   - 读取（/presets/list）对所有用户开放 —— 终端执行端需要；
//   - 编辑（/presets/save）仅管理员 —— 由路由层 RequireAdminAPI 把关。
// 配置文件与 servers.json 同目录、同挂载，重启/重建容器均不丢失。

// maxPresets 单文件最大条目数
const maxPresets = 100

// maxPresetCmdLen 单条命令最大长度
const maxPresetCmdLen = 2000

// PresetCommand 预制命令条目
type PresetCommand struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

// presetsPath 返回 presets.json 的实际路径（PRESETS_FILE 可覆盖）。
func presetsPath() string {
	if p := os.Getenv("PRESETS_FILE"); p != "" {
		return p
	}
	return "/webssh/presets.json"
}

// readPresets 读取预制命令列表。文件不存在按空列表处理；
// JSON 损坏时返回空列表并附带提示（不阻断读取方）。
func readPresets() ([]PresetCommand, string) {
	data, err := os.ReadFile(presetsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []PresetCommand{}, ""
		}
		return []PresetCommand{}, "读取配置失败: " + err.Error()
	}
	list := []PresetCommand{}
	if err := json.Unmarshal(data, &list); err != nil {
		return []PresetCommand{}, "配置格式错误: " + err.Error()
	}
	return list, ""
}

// ListPresets GET /presets/list
func ListPresets(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	list, warn := readPresets()
	responseBody.Data = gin.H{
		"list":  list,
		"warn":  warn,
		"count": len(list),
	}
	return &responseBody
}

// SavePresets POST /presets/save  body: []PresetCommand（整表覆写，原子落盘）
func SavePresets(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	var list []PresetCommand
	if err := c.ShouldBindJSON(&list); err != nil {
		responseBody.Msg = "请求格式错误: " + err.Error()
		return &responseBody
	}
	if len(list) > maxPresets {
		responseBody.Msg = fmt.Sprintf("预制命令最多 %d 条", maxPresets)
		return &responseBody
	}
	cleaned := make([]PresetCommand, 0, len(list))
	seen := map[string]bool{}
	for _, p := range list {
		p.Name = strings.TrimSpace(p.Name)
		p.Command = strings.TrimSpace(p.Command)
		if p.Name == "" && p.Command == "" {
			continue // 整行空白：忽略
		}
		if p.Name == "" || p.Command == "" {
			responseBody.Msg = "命令名称与内容都不能为空"
			return &responseBody
		}
		if len(p.Command) > maxPresetCmdLen {
			responseBody.Msg = fmt.Sprintf("命令过长（最多 %d 字符）", maxPresetCmdLen)
			return &responseBody
		}
		if p.ID == "" {
			p.ID = "p" + strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatInt(rand.Int63(), 36)
		}
		if seen[p.ID] {
			responseBody.Msg = "命令 ID 重复"
			return &responseBody
		}
		seen[p.ID] = true
		cleaned = append(cleaned, p)
	}
	if err := writePresets(cleaned); err != nil {
		responseBody.Msg = "保存失败: " + err.Error()
		return &responseBody
	}
	responseBody.Data = gin.H{"list": cleaned, "count": len(cleaned)}
	return &responseBody
}

// writePresets 原子写入：先写同目录临时文件再 rename，避免半截 JSON。
func writePresets(list []PresetCommand) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	path := presetsPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
