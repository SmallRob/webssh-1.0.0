package core

import (
	"errors"
	"fmt"
)

// RDCleanPath 是 IronRDP Web 客户端与代理之间的握手协议，
// 由 Microsoft「RDP Clean Path」草案定义，采用 ASN.1 DER 编码。
//
// 借鉴来源：electerm/ironrdp-wasm 的 example/lib/rdp-proxy.js。
// 浏览器端（ironrdp-wasm）先通过 WebSocket 发来一个 RDCleanPath 请求 PDU，
// 代理据此：
//  1. TCP 连接目标 RDP 服务器；
//  2. 原样转发请求中的 X.224 Connection Request，取回 Connection Confirm；
//  3. 在裸 TCP 上完成 TLS 握手（RDP 服务器通常使用自签名证书）；
//  4. 把 X.224 响应与服务器证书链回填进 RDCleanPath 响应 PDU；
//  5. 之后 WebSocket 与 TLS 通道之间做双向字节中继。
const rdpCleanPathVersion1 = 3390 // 3389 + 1

// ASN.1 DER 标签常量
const (
	tagSequence    = 0x30
	tagInteger     = 0x02
	tagOctetString = 0x04
	tagUTF8String  = 0x0c
)

// RDCleanPath 字段的上下文相关（EXPLICIT）标签号
const (
	ctxVersion           = 0
	ctxError             = 1
	ctxDestination       = 2
	ctxProxyAuth         = 3
	ctxPreconnectionBlob = 5
	ctxX224ConnectionPDU = 6
	ctxServerCertChain   = 7
	ctxServerAddr        = 9
)

// RDCleanPathRequest 是浏览器发来的握手请求。
type RDCleanPathRequest struct {
	Destination       string
	ProxyAuth         string
	PreconnectionBlob string
	X224ConnectionPDU []byte
}

// ---------- DER 编码 ----------

// derEncodeLength 编码 DER 长度域。
func derEncodeLength(length int) []byte {
	if length < 0x80 {
		return []byte{byte(length)}
	}
	var tmp []byte
	for v := length; v > 0; v >>= 8 {
		tmp = append([]byte{byte(v & 0xff)}, tmp...)
	}
	return append([]byte{0x80 | byte(len(tmp))}, tmp...)
}

// derWrap 用给定标签包裹内容（自动补长度域）。
func derWrap(tag byte, content []byte) []byte {
	out := make([]byte, 0, 1+len(content)+2)
	out = append(out, tag)
	out = append(out, derEncodeLength(len(content))...)
	out = append(out, content...)
	return out
}

// derEncodeInteger 编码 DER INTEGER（始终按无符号大端处理）。
func derEncodeInteger(value int64) []byte {
	if value == 0 {
		return derWrap(tagInteger, []byte{0})
	}
	var body []byte
	for v := uint64(value); v > 0; v >>= 8 {
		body = append([]byte{byte(v & 0xff)}, body...)
	}
	// 最高位为 1 时补 0x00，避免被解析为负数
	if body[0]&0x80 != 0 {
		body = append([]byte{0}, body...)
	}
	return derWrap(tagInteger, body)
}

// derEncodeUTF8String 编码 DER UTF8String。
func derEncodeUTF8String(s string) []byte {
	return derWrap(tagUTF8String, []byte(s))
}

// derEncodeOctetString 编码 DER OCTET STRING。
func derEncodeOctetString(b []byte) []byte {
	return derWrap(tagOctetString, b)
}

// derWrapContext 用上下文相关 EXPLICIT 标签 [n] 包裹内容。
func derWrapContext(tagNum int, content []byte) []byte {
	return derWrap(byte(0xa0+tagNum), content)
}

// ---------- DER 解码 ----------

// derDecodeLength 解码长度域，返回长度与消耗的字节数。
func derDecodeLength(buf []byte, offset int) (int, int, error) {
	if offset >= len(buf) {
		return 0, 0, errors.New("der: length out of range")
	}
	first := buf[offset]
	if first < 0x80 {
		return int(first), 1, nil
	}
	numBytes := int(first & 0x7f)
	if numBytes == 0 || numBytes > 4 || offset+1+numBytes > len(buf) {
		return 0, 0, errors.New("der: unsupported length encoding")
	}
	length := 0
	for i := 0; i < numBytes; i++ {
		length = (length << 8) | int(buf[offset+1+i])
	}
	return length, 1 + numBytes, nil
}

// derTLV 是一个 Tag-Length-Value 三元组。
type derTLV struct {
	tag   byte
	value []byte
	total int
}

// derDecodeTLV 解码 offset 处的一个 TLV。
func derDecodeTLV(buf []byte, offset int) (derTLV, error) {
	if offset >= len(buf) {
		return derTLV{}, errors.New("der: tlv out of range")
	}
	tag := buf[offset]
	length, consumed, err := derDecodeLength(buf, offset+1)
	if err != nil {
		return derTLV{}, err
	}
	headerLen := 1 + consumed
	if offset+headerLen+length > len(buf) {
		return derTLV{}, errors.New("der: tlv length exceeds buffer")
	}
	value := buf[offset+headerLen : offset+headerLen+length]
	return derTLV{tag: tag, value: value, total: headerLen + length}, nil
}

// derDecodeChildren 依次解出构造类型值内部的全部 TLV。
func derDecodeChildren(buf []byte) ([]derTLV, error) {
	var out []derTLV
	for offset := 0; offset < len(buf); {
		tlv, err := derDecodeTLV(buf, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, tlv)
		offset += tlv.total
	}
	return out, nil
}

// derDecodeInteger 解码 DER INTEGER 为 int64（无符号大端）。
func derDecodeInteger(buf []byte) int64 {
	var val int64
	for _, b := range buf {
		val = (val << 8) | int64(b)
	}
	return val
}

// ---------- RDCleanPath PDU ----------

// ParseRDCleanPathRequest 解析浏览器发来的 RDCleanPath 请求 PDU。
func ParseRDCleanPathRequest(data []byte) (*RDCleanPathRequest, error) {
	outer, err := derDecodeTLV(data, 0)
	if err != nil {
		return nil, fmt.Errorf("RDCleanPath: 解析外层 SEQUENCE 失败: %w", err)
	}
	if outer.tag != tagSequence {
		return nil, fmt.Errorf("RDCleanPath: 期望 SEQUENCE(0x30)，实际 0x%02x", outer.tag)
	}
	children, err := derDecodeChildren(outer.value)
	if err != nil {
		return nil, fmt.Errorf("RDCleanPath: 解析字段失败: %w", err)
	}

	req := &RDCleanPathRequest{}
	var version int64 = -1

	for _, child := range children {
		ctxTag := int(child.tag & 0x1f) // 去掉 class 位，取出标签号
		inner, err := derDecodeTLV(child.value, 0)
		if err != nil {
			return nil, fmt.Errorf("RDCleanPath: 字段 [%d] 解析失败: %w", ctxTag, err)
		}
		switch ctxTag {
		case ctxVersion:
			version = derDecodeInteger(inner.value)
		case ctxDestination:
			req.Destination = string(inner.value)
		case ctxProxyAuth:
			req.ProxyAuth = string(inner.value)
		case ctxPreconnectionBlob:
			req.PreconnectionBlob = string(inner.value)
		case ctxX224ConnectionPDU:
			req.X224ConnectionPDU = append([]byte(nil), inner.value...)
		}
	}

	if version != rdpCleanPathVersion1 {
		return nil, fmt.Errorf("RDCleanPath: 不支持的版本 %d（期望 %d）", version, rdpCleanPathVersion1)
	}
	if req.Destination == "" {
		return nil, errors.New("RDCleanPath: 请求缺少 destination 字段")
	}
	if len(req.X224ConnectionPDU) == 0 {
		return nil, errors.New("RDCleanPath: 请求缺少 x224_connection_pdu 字段")
	}
	return req, nil
}

// BuildRDCleanPathResponse 构造 RDCleanPath 响应 PDU，回填 X.224 响应、
// 服务器证书链与实际连接到的地址。
func BuildRDCleanPathResponse(serverAddr string, x224Response []byte, certChain [][]byte) []byte {
	var parts []byte

	// [0] version
	parts = append(parts, derWrapContext(ctxVersion, derEncodeInteger(rdpCleanPathVersion1))...)

	// [6] x224_connection_pdu
	parts = append(parts, derWrapContext(ctxX224ConnectionPDU, derEncodeOctetString(x224Response))...)

	// [7] server_cert_chain —— SEQUENCE OF OCTET STRING
	certBody := make([]byte, 0, 1024)
	for _, cert := range certChain {
		certBody = append(certBody, derEncodeOctetString(cert)...)
	}
	parts = append(parts, derWrapContext(ctxServerCertChain, derWrap(tagSequence, certBody))...)

	// [9] server_addr
	parts = append(parts, derWrapContext(ctxServerAddr, derEncodeUTF8String(serverAddr))...)

	return derWrap(tagSequence, parts)
}

// BuildRDCleanPathError 构造 RDCleanPath 错误 PDU。
// errorCode 1=general、2=negotiation；httpStatusCode 可选。
func BuildRDCleanPathError(errorCode int64, httpStatusCode *int64) []byte {
	var errParts []byte
	errParts = append(errParts, derWrapContext(ctxVersion, derEncodeInteger(errorCode))...)
	if httpStatusCode != nil {
		errParts = append(errParts, derWrapContext(1, derEncodeInteger(*httpStatusCode))...)
	}
	errSeq := derWrap(tagSequence, errParts)

	var parts []byte
	parts = append(parts, derWrapContext(ctxVersion, derEncodeInteger(rdpCleanPathVersion1))...)
	parts = append(parts, derWrapContext(ctxError, errSeq)...)

	return derWrap(tagSequence, parts)
}

// parseRDPDestination 解析 "host:port" / "[v6]:port" / "host" 形式的目标地址，
// 缺省端口为 3389。
func parseRDPDestination(destination string) (string, int) {
	if len(destination) > 0 && destination[0] == '[' {
		end := -1
		for i := 1; i < len(destination); i++ {
			if destination[i] == ']' {
				end = i
				break
			}
		}
		if end < 0 {
			return destination, 3389
		}
		host := destination[1:end]
		rest := destination[end+1:]
		if len(rest) > 1 && rest[0] == ':' {
			var port int
			if _, err := fmt.Sscanf(rest[1:], "%d", &port); err == nil && port > 0 && port <= 65535 {
				return host, port
			}
		}
		return host, 3389
	}

	lastColon := -1
	for i := len(destination) - 1; i >= 0; i-- {
		if destination[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon < 0 {
		return destination, 3389
	}
	host := destination[:lastColon]
	var port int
	if _, err := fmt.Sscanf(destination[lastColon+1:], "%d", &port); err != nil || port <= 0 || port > 65535 {
		return destination, 3389
	}
	return host, port
}
