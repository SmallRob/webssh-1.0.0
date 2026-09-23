package core

import (
	"strconv"
	"strings"
)

// 字节单位常量。原先定义在 controller 包，但 core/sftpclient.go 也需要
// 同一个人性化体积格式化能力，因此下沉到 core 供两个包共用。
const (
	// BYTE 字节
	BYTE = 1 << (10 * iota)
	// KILOBYTE 千字节
	KILOBYTE
	// MEGABYTE 兆字节
	MEGABYTE
	// GIGABYTE 吉字节
	GIGABYTE
	// TERABYTE 太字节
	TERABYTE
	// PETABYTE 拍字节
	PETABYTE
	// EXABYTE 艾字节
	EXABYTE
)

// Bytefmt 把字节数格式化为人类可读字符串，例如 10M、12.5K。
// 可选单位：E/P/T/G/M/K/B，总是取使数值不小于 1 的最小单位。
func Bytefmt(bytes uint64) string {
	unit := ""
	value := float64(bytes)

	switch {
	case bytes >= EXABYTE:
		unit = "E"
		value = value / EXABYTE
	case bytes >= PETABYTE:
		unit = "P"
		value = value / PETABYTE
	case bytes >= TERABYTE:
		unit = "T"
		value = value / TERABYTE
	case bytes >= GIGABYTE:
		unit = "G"
		value = value / GIGABYTE
	case bytes >= MEGABYTE:
		unit = "M"
		value = value / MEGABYTE
	case bytes >= KILOBYTE:
		unit = "K"
		value = value / KILOBYTE
	case bytes >= BYTE:
		unit = "B"
	case bytes == 0:
		return "0B"
	}

	result := strconv.FormatFloat(value, 'f', 2, 64)
	result = strings.TrimSuffix(result, ".00")
	return result + unit
}
