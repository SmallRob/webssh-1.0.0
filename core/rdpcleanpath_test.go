package core

import (
	"bytes"
	"testing"
)

// buildSampleRequest 按 RDCleanPath 草案构造一个请求 PDU，
// 编码方式与 ironrdp-wasm 官方代理 (example/lib/rdp-proxy.js) 的
// derWrapContext / derEncodeInteger / derEncodeUTF8String 完全一致，
// 用于验证 Go 侧解析器能正确还原字段。
func buildSampleRequest(destination string, x224 []byte) []byte {
	var parts []byte
	parts = append(parts, derWrapContext(ctxVersion, derEncodeInteger(rdpCleanPathVersion1))...)
	parts = append(parts, derWrapContext(ctxDestination, derEncodeUTF8String(destination))...)
	parts = append(parts, derWrapContext(ctxProxyAuth, derEncodeUTF8String(""))...)
	parts = append(parts, derWrapContext(ctxX224ConnectionPDU, derEncodeOctetString(x224))...)
	return derWrap(tagSequence, parts)
}

func TestParseRDCleanPathRequest(t *testing.T) {
	x224 := []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x08, 0x00, 0x03, 0x00, 0x00, 0x00}
	raw := buildSampleRequest("10.0.0.5:3389", x224)

	req, err := ParseRDCleanPathRequest(raw)
	if err != nil {
		t.Fatalf("解析请求失败: %v", err)
	}
	if req.Destination != "10.0.0.5:3389" {
		t.Errorf("destination = %q, 期望 %q", req.Destination, "10.0.0.5:3389")
	}
	if !bytes.Equal(req.X224ConnectionPDU, x224) {
		t.Errorf("x224_connection_pdu 未正确还原: %x", req.X224ConnectionPDU)
	}
}

func TestParseRDCleanPathRequestErrors(t *testing.T) {
	// 非 SEQUENCE
	if _, err := ParseRDCleanPathRequest([]byte{0x02, 0x01, 0x00}); err == nil {
		t.Error("非 SEQUENCE 输入应当报错")
	}
	// 版本不匹配
	bad := derWrap(tagSequence, derWrapContext(ctxVersion, derEncodeInteger(1234)))
	if _, err := ParseRDCleanPathRequest(bad); err == nil {
		t.Error("版本不匹配应当报错")
	}
	// 缺少 destination / x224
	missing := derWrap(tagSequence, derWrapContext(ctxVersion, derEncodeInteger(rdpCleanPathVersion1)))
	if _, err := ParseRDCleanPathRequest(missing); err == nil {
		t.Error("缺少 destination 应当报错")
	}
}

func TestBuildRDCleanPathResponse(t *testing.T) {
	cert := []byte{0x30, 0x82, 0x01, 0x00, 0xde, 0xad, 0xbe, 0xef}
	x224Resp := []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00}
	raw := BuildRDCleanPathResponse("10.0.0.5:3389", x224Resp, [][]byte{cert})

	outer, err := derDecodeTLV(raw, 0)
	if err != nil {
		t.Fatalf("响应外层解析失败: %v", err)
	}
	if outer.tag != tagSequence {
		t.Fatalf("响应外层标签 = 0x%02x, 期望 0x30", outer.tag)
	}
	children, err := derDecodeChildren(outer.value)
	if err != nil {
		t.Fatalf("响应字段解析失败: %v", err)
	}

	seen := map[int]bool{}
	for _, child := range children {
		ctxTag := int(child.tag & 0x1f)
		inner, err := derDecodeTLV(child.value, 0)
		if err != nil {
			t.Fatalf("字段 [%d] 解析失败: %v", ctxTag, err)
		}
		seen[ctxTag] = true
		switch ctxTag {
		case ctxVersion:
			if got := derDecodeInteger(inner.value); got != rdpCleanPathVersion1 {
				t.Errorf("version = %d, 期望 %d", got, rdpCleanPathVersion1)
			}
		case ctxX224ConnectionPDU:
			if !bytes.Equal(inner.value, x224Resp) {
				t.Errorf("x224_connection_pdu = %x, 期望 %x", inner.value, x224Resp)
			}
		case ctxServerAddr:
			if string(inner.value) != "10.0.0.5:3389" {
				t.Errorf("server_addr = %q", string(inner.value))
			}
		case ctxServerCertChain:
			certs, err := derDecodeChildren(inner.value)
			if err != nil {
				t.Fatalf("证书链解析失败: %v", err)
			}
			if len(certs) != 1 {
				t.Fatalf("证书数量 = %d, 期望 1", len(certs))
			}
			// certs[0] 已是 OCTET STRING，其 value 即 DER 证书原文
			if certs[0].tag != tagOctetString {
				t.Errorf("证书项标签 = 0x%02x, 期望 OCTET STRING(0x04)", certs[0].tag)
			}
			if !bytes.Equal(certs[0].value, cert) {
				t.Errorf("证书内容不一致: %x", certs[0].value)
			}
		}
	}
	for _, want := range []int{ctxVersion, ctxX224ConnectionPDU, ctxServerCertChain, ctxServerAddr} {
		if !seen[want] {
			t.Errorf("响应缺少字段 [%d]", want)
		}
	}
}

func TestDerEncodeIntegerHighBit(t *testing.T) {
	// 0x80 最高位为 1，需补前导 0x00，避免被解析为负数
	encoded := derEncodeInteger(0x80)
	inner, err := derDecodeTLV(encoded, 0)
	if err != nil {
		t.Fatalf("INTEGER 解析失败: %v", err)
	}
	if len(inner.value) != 2 || inner.value[0] != 0x00 || inner.value[1] != 0x80 {
		t.Errorf("INTEGER(0x80) 编码 = %x, 期望 00 80", inner.value)
	}
	// 长度域大于 127 时应使用长格式
	long := make([]byte, 200)
	wrapped := derWrap(tagOctetString, long)
	if wrapped[1] != 0x81 || wrapped[2] != 200 {
		t.Errorf("长格式长度域编码错误: %x", wrapped[:3])
	}
}

func TestParseRDPDestination(t *testing.T) {
	cases := []struct {
		in   string
		host string
		port int
	}{
		{"10.0.0.5:3389", "10.0.0.5", 3389},
		{"10.0.0.5", "10.0.0.5", 3389},
		{"[fe80::1]:3390", "fe80::1", 3390},
		{"[fe80::1]", "fe80::1", 3389},
		{"win-host:1234", "win-host", 1234},
	}
	for _, c := range cases {
		host, port := parseRDPDestination(c.in)
		if host != c.host || port != c.port {
			t.Errorf("parseRDPDestination(%q) = (%q, %d), 期望 (%q, %d)", c.in, host, port, c.host, c.port)
		}
	}
}

func TestProtocolRegistry(t *testing.T) {
	if got := ParseProtocol("RDP"); got != ProtocolRDP {
		t.Errorf("ParseProtocol(RDP) = %q", got)
	}
	if got := ParseProtocol("vnc"); got != ProtocolVNC {
		t.Errorf("ParseProtocol(vnc) = %q", got)
	}
	if got := ParseProtocol(""); got != ProtocolSSH {
		t.Errorf("ParseProtocol(空) 应回退为 ssh，实际 %q", got)
	}
	if ProtocolRDP.DefaultPort() != 3389 || ProtocolVNC.DefaultPort() != 5900 || ProtocolSSH.DefaultPort() != 22 {
		t.Error("协议默认端口不正确")
	}
	if !ProtocolRDP.IsRemoteDesktop() || ProtocolSSH.IsRemoteDesktop() {
		t.Error("IsRemoteDesktop 判定不正确")
	}
}

func TestNormalizeFillsPortAndProtocol(t *testing.T) {
	c := NewSSHClient()
	c.Hostname = "10.0.0.5"
	c.Protocol = ""
	c.Port = 0
	c.Normalize()
	if c.Protocol != string(ProtocolSSH) || c.Port != 22 {
		t.Errorf("Normalize 后 = (%s, %d), 期望 (ssh, 22)", c.Protocol, c.Port)
	}

	r := NewSSHClient()
	r.Hostname = "fe80::1"
	r.Protocol = "rdp"
	r.Port = 0
	r.Normalize()
	if r.Port != 3389 {
		t.Errorf("RDP 默认端口 = %d, 期望 3389", r.Port)
	}
	if r.Hostname != "[fe80::1]" {
		t.Errorf("IPv6 主机名未加方括号: %q", r.Hostname)
	}
	if r.Addr() != "[fe80::1]:3389" {
		t.Errorf("Addr() = %q", r.Addr())
	}
}
