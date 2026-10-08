package controller

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"webssh/core"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// metricsCommand 单次采集命令：所有段通过 echo "_TAG_ $(...)" 输出为单行，
// 由 parseMetrics 按 tag 切分解析。对非 Linux 目标（/proc 不存在）各段自然退化为空。
// CPU 使用双采样（间隔 0.4s）计算差值得到使用率。
const metricsCommand = `export LC_ALL=C LC_NUMERIC=C 2>/dev/null; ` +
	`echo "_TIME_ $(date '+%Y-%m-%d %H:%M:%S %Z%z')"; ` +
	`echo "_UP_ $(uptime 2>/dev/null)"; ` +
	`echo "_CPU1_ $(grep '^cpu ' /proc/stat 2>/dev/null)"; ` +
	`sleep 0.4; ` +
	`echo "_CPU2_ $(grep '^cpu ' /proc/stat 2>/dev/null)"; ` +
	`echo "_MEM_ $(grep -E '^(MemTotal|MemFree|MemAvailable|Buffers|Cached|SReclaimable|SwapTotal|SwapFree):' /proc/meminfo 2>/dev/null | tr '\n' '|')"; ` +
	`echo "_LOAD_ $(cat /proc/loadavg 2>/dev/null)"; ` +
	`echo "_DISK_ $(df -kP 2>/dev/null | tr '\n' '|')"; ` +
	`echo "_HOST_ $(hostname 2>/dev/null)"; ` +
	`echo "_KERNEL_ $(uname -r 2>/dev/null)"; ` +
	`echo "_CORES_ $(nproc 2>/dev/null || grep -c '^processor' /proc/cpuinfo 2>/dev/null)"; ` +
	`echo "_END_"`

// MemStats 内存 / 交换分区统计（单位 KB）
type MemStats struct {
	TotalKB uint64  `json:"totalKB"`
	UsedKB  uint64  `json:"usedKB"`
	Percent float64 `json:"percent"`
}

// DiskStat 单个挂载点磁盘统计（单位 KB）
type DiskStat struct {
	Fs      string  `json:"fs"`
	Mount   string  `json:"mount"`
	TotalKB uint64  `json:"totalKB"`
	UsedKB  uint64  `json:"usedKB"`
	Percent float64 `json:"percent"`
}

// ServerStats 推送给前端的服务器状态快照。
// 数值为 -1 表示该指标不可用（如非 Linux 目标）。
type ServerStats struct {
	Time       string     `json:"time"`    // 服务器本地时间（含时区）
	Hostname   string     `json:"hostname"`
	Kernel     string     `json:"kernel"`
	Uptime     string     `json:"uptime"`
	Cores      int        `json:"cores"`
	Load1      string     `json:"load1"`
	Load5      string     `json:"load5"`
	Load15     string     `json:"load15"`
	CpuPercent float64    `json:"cpuPercent"`
	Mem        MemStats   `json:"mem"`
	Swap       MemStats   `json:"swap"`
	Disks      []DiskStat `json:"disks"`
}

var uptimeRe = regexp.MustCompile(`up\s+(.+?),\s+\d+\s+users?`)

// MetricsWs 服务器状态 WebSocket：与 /term 相同的 sshInfo 描述符建立独立 SSH 连接，
// 周期采集目标服务器指标并以 JSON 推送。前端断开后连接自动清理。
func MetricsWs(c *gin.Context) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	sshInfo := c.DefaultQuery("sshInfo", "")
	sshClient, err := core.DecodedMsgToSSHClient(sshInfo)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if err = sshClient.GenerateClient(); err != nil {
		wsConn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		wsConn.Close()
		sshClient.Close()
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer func() {
		wsConn.Close()
		sshClient.Close()
	}()

	// 读协程：仅用于感知前端断开（ReadMessage 返回错误即关闭 done）。
	// 为避免与采集协程并发写 WebSocket，这里对 "ping" 心跳不回写。
	done := make(chan struct{})
	go func() {
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}()

	pushStats := func(stats *ServerStats) bool {
		wsConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := wsConn.WriteJSON(stats); err != nil {
			return false
		}
		return true
	}

	// 连接建立后立即推送一次，随后按固定周期采集
	if stats := collectMetrics(&sshClient); stats != nil {
		if !pushStats(stats) {
			return &responseBody
		}
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return &responseBody
		case <-ticker.C:
			stats := collectMetrics(&sshClient)
			if stats == nil {
				// SSH 会话异常（网络断开等）：通知前端后结束
				wsConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				wsConn.WriteMessage(websocket.TextMessage, []byte("metrics collection failed"))
				return &responseBody
			}
			if !pushStats(stats) {
				return &responseBody
			}
		}
	}
}

// collectMetrics 在独立 SSH 会话上执行一次采集命令并解析结果。
// 任一环节失败返回 nil（不产生半截数据）。
func collectMetrics(client *core.SSHClient) *ServerStats {
	session, err := client.Client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()
	var buf bytes.Buffer
	session.Stdout = &buf
	errCh := make(chan error, 1)
	go func() { errCh <- session.Run(metricsCommand) }()
	select {
	case <-errCh:
	case <-time.After(10 * time.Second):
		session.Close()
		return nil
	}
	return parseMetrics(buf.String())
}

// parseMetrics 解析 metricsCommand 的分段输出。
func parseMetrics(out string) *ServerStats {
	stats := &ServerStats{Disks: []DiskStat{}, CpuPercent: -1}
	stats.Mem.Percent, stats.Swap.Percent = -1, -1
	var cpu1, cpu2, memRaw, diskRaw string
	memKV := map[string]uint64{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		// 注意：echo "_TAG_ ..." 的输出中不含双引号（被 shell 消费），前缀按裸 tag 匹配
		case strings.HasPrefix(line, "_TIME_"):
			stats.Time = trimTag(line, "_TIME_")
		case strings.HasPrefix(line, "_UP_"):
			stats.Uptime = extractUptime(trimTag(line, "_UP_"))
		case strings.HasPrefix(line, "_CPU1_"):
			cpu1 = trimTag(line, "_CPU1_")
		case strings.HasPrefix(line, "_CPU2_"):
			cpu2 = trimTag(line, "_CPU2_")
		case strings.HasPrefix(line, "_MEM_"):
			memRaw = trimTag(line, "_MEM_")
		case strings.HasPrefix(line, "_LOAD_"):
			load := strings.Fields(trimTag(line, "_LOAD_"))
			if len(load) >= 3 {
				stats.Load1, stats.Load5, stats.Load15 = load[0], load[1], load[2]
			}
		case strings.HasPrefix(line, "_DISK_"):
			diskRaw = trimTag(line, "_DISK_")
		case strings.HasPrefix(line, "_HOST_"):
			stats.Hostname = trimTag(line, "_HOST_")
		case strings.HasPrefix(line, "_KERNEL_"):
			stats.Kernel = trimTag(line, "_KERNEL_")
		case strings.HasPrefix(line, "_CORES_"):
			if n, err := strconv.Atoi(strings.TrimSpace(trimTag(line, "_CORES_"))); err == nil {
				stats.Cores = n
			}
		}
	}
	stats.CpuPercent = cpuPercent(cpu1, cpu2)
	for _, kv := range strings.Split(memRaw, "|") {
		fields := strings.Fields(strings.TrimSpace(kv))
		if len(fields) < 2 {
			continue
		}
		if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			memKV[strings.TrimSuffix(fields[0], ":")] = v
		}
	}
	fillMem(&stats.Mem, memKV, "MemTotal", "MemFree", "MemAvailable", "Buffers", "Cached", "SReclaimable")
	fillMem(&stats.Swap, memKV, "SwapTotal", "SwapFree", "", "", "", "")
	stats.Disks = parseDisks(diskRaw)
	return stats
}

// trimTag 去掉 "_TAG_ " 前缀，返回纯值（内容为空时返回空串）。
func trimTag(line, tag string) string {
	v := strings.TrimPrefix(line, tag+" ")
	return strings.TrimSpace(v)
}

// fillMem 按 available 优先（Linux ≥3.14）计算已用内存；
// 无 available 时回退 total-free-buffers-cached。total 为 0 时保持不可用状态。
func fillMem(m *MemStats, kv map[string]uint64, totalKey, freeKey, availKey, bufKey, cacheKey, sreclaimKey string) {
	total := kv[totalKey]
	if total == 0 {
		return
	}
	m.TotalKB = total
	if availKey != "" {
		if avail := kv[availKey]; avail > 0 {
			m.UsedKB = total - avail
		}
	}
	if m.UsedKB == 0 && freeKey != "" {
		used := total - kv[freeKey] - kv[bufKey] - kv[cacheKey]
		if sreclaimKey != "" {
			used += kv[sreclaimKey]
		}
		if used < 0 {
			used = total - kv[freeKey]
		}
		m.UsedKB = used
	}
	if freeKey != "" && m.UsedKB == 0 {
		// swap 情形：used = total - free
		m.UsedKB = total - kv[freeKey]
	}
	m.Percent = float64(m.UsedKB) / float64(total) * 100
}

// cpuPercent 由两次 /proc/stat cpu 行差值计算使用率；样本无效返回 -1。
func cpuPercent(cpu1, cpu2 string) float64 {
	t1, i1, ok1 := cpuBusy(cpu1)
	t2, i2, ok2 := cpuBusy(cpu2)
	if !ok1 || !ok2 {
		return -1
	}
	dt, di := t2-t1, i2-i1
	if dt <= 0 {
		return -1
	}
	return float64(dt-di) / float64(dt) * 100
}

// cpuBusy 返回 (总节拍, 空闲节拍=idle+iowait)。
func cpuBusy(line string) (total, idle uint64, ok bool) {
	fields := strings.Fields(line)
	// fields[0] 为 "cpu" 标签
	if len(fields) < 5 {
		return 0, 0, false
	}
	vals := make([]uint64, 0, len(fields)-1)
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals = append(vals, v)
	}
	for _, v := range vals {
		total += v
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	return total, idle, true
}

// diskSkipFs 过滤伪文件系统
var diskSkipFs = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "udev": true, "none": true, "shm": true,
	"overlay": true, "cgroup": true, "cgroup2": true, "squashfs": true,
	"efivarfs": true, "proc": true, "sysfs": true, "devfs": true, "fdesc": true,
	"devpts": true, "mqueue": true, "hugetlbfs": true, "nsfs": true,
}

// parseDisks 解析 df -kP 输出（换行已被替换为 '|'）。按容量降序，最多返回 8 个挂载点。
func parseDisks(raw string) []DiskStat {
	seen := map[string]bool{}
	disks := make([]DiskStat, 0, 4)
	for _, entry := range strings.Split(raw, "|") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		fields := strings.Fields(entry)
		if len(fields) < 6 || fields[1] == "1024-blocks" {
			continue
		}
		fs := fields[0]
		if diskSkipFs[fs] || strings.HasPrefix(fs, "/dev/loop") || strings.HasPrefix(fs, "://") {
			continue
		}
		mount := strings.Join(fields[5:], " ")
		if !strings.HasPrefix(mount, "/") || seen[mount] {
			continue
		}
		total, err1 := strconv.ParseUint(fields[1], 10, 64)
		used, err2 := strconv.ParseUint(fields[2], 10, 64)
		if err1 != nil || err2 != nil || total == 0 {
			continue
		}
		seen[mount] = true
		disks = append(disks, DiskStat{
			Fs: fs, Mount: mount, TotalKB: total, UsedKB: used,
			Percent: float64(used) / float64(total) * 100,
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].TotalKB > disks[j].TotalKB })
	if len(disks) > 8 {
		disks = disks[:8]
	}
	return disks
}

// extractUptime 从 uptime 输出中截取运行时长，如 "45 days, 3:22"。
func extractUptime(s string) string {
	m := uptimeRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.Join(strings.Fields(m[1]), " ")
}
