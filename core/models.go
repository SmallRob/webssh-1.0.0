package core

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"strings"
	"unicode/utf8"
)

// WcList 全局counter list变量
var WcList []*WriteCounter

// WriteCounter 结构体
type WriteCounter struct {
	Total int
	Id    string
}

// Write: implement Write interface to write bytes from ssh server into bytes.Buffer.
func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += n
	return n, nil
}

type wsOutput struct {
	ws *websocket.Conn
}

// Write: implement Write interface to write bytes from ssh server into bytes.Buffer.
func (w *wsOutput) Write(p []byte) (int, error) {
	// 处理非utf8字符
	if !utf8.Valid(p) {
		bufStr := string(p)
		buf := make([]rune, 0, len(bufStr))
		for _, r := range bufStr {
			if r == utf8.RuneError {
				buf = append(buf, []rune("@")...)
			} else {
				buf = append(buf, r)
			}
		}
		p = []byte(string(buf))
	}
	err := w.ws.WriteMessage(websocket.TextMessage, p)
	return len(p), err
}

// SSHClient 结构体
//
// 虽然名字沿用 SSHClient，但它实际上已是「统一连接描述符」：
// Protocol 为 ssh 时承载原生 SSH 会话与会话终端；
// Protocol 为 rdp / vnc 时仅承载目标地址与该协议的认证参数，
// 由 ServeRDP / ServeVNC 建立到目标端口的桥接通道。
type SSHClient struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Hostname  string `json:"hostname"`
	Port      int    `json:"port"`
	LoginType int    `json:"logintype"`
	PrivateKey string `json:"privateKey"`
	Passphrase string `json:"passphrase"`
	// Protocol 连接协议：ssh / rdp / vnc，空值按 ssh 处理。
	Protocol string `json:"protocol"`
	// Domain 仅 RDP 使用，对应 Windows 域 / 工作组。
	Domain string `json:"domain"`
	// RDPGateway / RDPGatewayPort：RDP 网关转发（可选）。
	// 当配置时，RDP 连接不直连目标主机:3389，而是通过网关中转：
	//   WebSSH → gateway:gatewayPort → (网关内部转发) → 目标主机:3389
	// 典型场景：内网 RDP 不对外暴露，通过 rdp.leansoftx.com:443 转发。
	RDPGateway     string `json:"rdpGateway,omitempty"`
	RDPGatewayPort int    `json:"rdpGatewayPort,omitempty"`
	Client    *ssh.Client
	Sftp      *sftp.Client
	StdinPipe io.WriteCloser
	Session   *ssh.Session
}

// NewSSHClient 返回默认ssh信息
func NewSSHClient() SSHClient {
	client := SSHClient{}
	client.Protocol = string(ProtocolSSH)
	client.Port = 22
	return client
}

// Proto 返回规整后的协议类型。
func (sclient *SSHClient) Proto() Protocol {
	return ParseProtocol(sclient.Protocol)
}

// Normalize 补齐协议与端口的默认值。
// 前端可能只传主机名不传端口，这里按协议回填默认端口
// （ssh 22 / rdp 3389 / vnc 5900），并把协议名规整为小写。
func (sclient *SSHClient) Normalize() {
	proto := sclient.Proto()
	sclient.Protocol = string(proto)
	if sclient.Port == 0 {
		sclient.Port = proto.DefaultPort()
	}
	if strings.Contains(sclient.Hostname, ":") && !strings.HasPrefix(sclient.Hostname, "[") {
		sclient.Hostname = "[" + sclient.Hostname + "]"
	}
}

// Addr 返回 host:port 形式的连接地址。
func (sclient *SSHClient) Addr() string {
	return fmt.Sprintf("%s:%d", sclient.Hostname, sclient.Port)
}

// RDPAddr 返回 RDP 实际要连接的地址。
// 当配置了 RDPGateway 时返回网关地址（由网关内部转发到真实目标），
// 否则直连目标主机。RDP 协议的 destination 仍填原始目标，
// 让 ironrdp-wasm 知道它真正要连接的是哪台机器。
func (sclient *SSHClient) RDPAddr() string {
	if sclient.RDPGateway != "" && sclient.RDPGatewayPort > 0 {
		return fmt.Sprintf("%s:%d", sclient.RDPGateway, sclient.RDPGatewayPort)
	}
	return sclient.Addr()
}

// UseRDPGateway 判断是否配置了 RDP 网关。
func (sclient *SSHClient) UseRDPGateway() bool {
	return sclient.RDPGateway != "" && sclient.RDPGatewayPort > 0
}

// Close all closable fields of SSHClient that is opened:
//
//	StdinPipe, Session, Sftp, Client
func (sclient *SSHClient) Close() {
	defer func() { // just in case
		if err := recover(); err != nil {
			log.Println("SSHClient Close recover from panic: ", err)
		}
	}()

	if sclient.StdinPipe != nil {
		sclient.StdinPipe.Close()
		sclient.StdinPipe = nil
	}
	if sclient.Session != nil {
		sclient.Session.Close()
		sclient.Session = nil
	}
	if sclient.Sftp != nil {
		sclient.Sftp.Close()
		sclient.Sftp = nil
	}
	if sclient.Client != nil {
		sclient.Client.Close()
		sclient.Client = nil
	}
}
