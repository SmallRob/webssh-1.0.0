package core

import "strings"

// Protocol 表示 WebSSH 支持的远程连接协议类型。
//
// 设计借鉴 shield-cli 的协议注册表思路：把「协议」提升为一等公民，
// 由协议自身声明默认端口、认证字段需求与展示名称，避免在
// controller / core / 前端三处分别硬编码端口与字段规则。
type Protocol string

const (
	// ProtocolSSH 终端协议（原生 SSH，SSHClient 直接承载）。
	ProtocolSSH Protocol = "ssh"
	// ProtocolRDP 图形化远程桌面（RDCleanPath 代理 + TLS 中继）。
	ProtocolRDP Protocol = "rdp"
	// ProtocolVNC 图形化远程桌面（RFB 透明字节中继）。
	ProtocolVNC Protocol = "vnc"
)

// ProtocolSpec 描述一个协议的元信息。
type ProtocolSpec struct {
	Name         Protocol `json:"name"`
	Label        string   `json:"label"`
	DefaultPort  int      `json:"defaultPort"`
	NeedUsername bool     `json:"needUsername"`
	NeedPassword bool     `json:"needPassword"`
	SupportKey   bool     `json:"supportKey"`
	SupportExec  bool     `json:"supportExec"`
	RemoteDesktop bool    `json:"remoteDesktop"`
}

// protocolSpecs 是内置协议清单，顺序即前端下拉框顺序。
var protocolSpecs = []ProtocolSpec{
	{
		Name:         ProtocolSSH,
		Label:        "SSH",
		DefaultPort:  22,
		NeedUsername: true,
		NeedPassword: true,
		SupportKey:   true,
		SupportExec:  true,
	},
	{
		Name:          ProtocolRDP,
		Label:         "RDP",
		DefaultPort:   3389,
		NeedUsername:  true,
		NeedPassword:  true,
		RemoteDesktop: true,
	},
	{
		Name:          ProtocolVNC,
		Label:         "VNC",
		DefaultPort:   5900,
		NeedUsername:  false,
		NeedPassword:  true,
		RemoteDesktop: true,
	},
}

// ProtocolList 返回全部内置协议（供 /protocols 接口与前端使用）。
func ProtocolList() []ProtocolSpec {
	list := make([]ProtocolSpec, len(protocolSpecs))
	copy(list, protocolSpecs)
	return list
}

// ParseProtocol 宽松解析协议名，大小写不敏感，未知协议回退为 SSH。
func ParseProtocol(s string) Protocol {
	switch Protocol(strings.ToLower(strings.TrimSpace(s))) {
	case ProtocolRDP:
		return ProtocolRDP
	case ProtocolVNC:
		return ProtocolVNC
	default:
		return ProtocolSSH
	}
}

// Spec 返回协议的元信息；未知协议按 SSH 处理。
func (p Protocol) Spec() ProtocolSpec {
	for _, spec := range protocolSpecs {
		if spec.Name == p {
			return spec
		}
	}
	return protocolSpecs[0]
}

// DefaultPort 返回协议默认端口。
func (p Protocol) DefaultPort() int {
	return p.Spec().DefaultPort
}

// IsRemoteDesktop 判断是否为图形化远程桌面协议（RDP / VNC）。
func (p Protocol) IsRemoteDesktop() bool {
	return p.Spec().RemoteDesktop
}

// Valid 判断协议是否在内置清单内。
func (p Protocol) Valid() bool {
	for _, spec := range protocolSpecs {
		if spec.Name == p {
			return true
		}
	}
	return false
}
