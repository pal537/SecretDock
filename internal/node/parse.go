// 节点/订阅链接解析：识别类型、协议、主机端口
package node

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

type Info struct {
	Kind  string // "sub" 订阅 | "node" 单节点
	Proto string // vless, vmess, trojan, shadowsocks, hysteria2, tuic, http, https...
	Host  string
	Port  int
	Name  string // 链接 #fragment 中的备注名
}

func b64decode(s string) []byte {
	s = strings.TrimSpace(s)
	// 补 padding
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		b, _ = base64.StdEncoding.DecodeString(s)
	}
	return b
}

// Parse 解析节点/订阅链接
func Parse(raw string) Info {
	raw = strings.TrimSpace(raw)
	info := Info{}
	if raw == "" {
		return info
	}
	lower := strings.ToLower(raw)
	// http/https = 订阅链接
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		info.Kind = "sub"
		if strings.HasPrefix(lower, "https://") {
			info.Proto = "https"
		} else {
			info.Proto = "http"
		}
		if u, err := url.Parse(raw); err == nil {
			info.Host = u.Hostname()
			if p := u.Port(); p != "" {
				info.Port, _ = strconv.Atoi(p)
			} else if info.Proto == "https" {
				info.Port = 443
			} else {
				info.Port = 80
			}
		}
		return info
	}
	info.Kind = "node"
	// 取 #fragment 做名称
	if i := strings.Index(raw, "#"); i >= 0 {
		if frag := raw[i+1:]; frag != "" {
			if decoded, err := url.QueryUnescape(frag); err == nil {
				info.Name = decoded
			} else {
				info.Name = frag
			}
		}
		raw = raw[:i]
	}
	scheme := ""
	if i := strings.Index(raw, "://"); i >= 0 {
		scheme = strings.ToLower(raw[:i])
	}
	switch scheme {
	case "vless", "trojan", "tuic", "hysteria", "hysteria2", "hy2", "wireguard", "socks", "socks5":
		info.Proto = scheme
		if scheme == "hy2" {
			info.Proto = "hysteria2"
		}
		if u, err := url.Parse(raw); err == nil {
			info.Host = u.Hostname()
			info.Port, _ = strconv.Atoi(u.Port())
		}
	case "vmess":
		info.Proto = "vmess"
		b := b64decode(strings.TrimPrefix(raw, "vmess://"))
		var m map[string]interface{}
		if json.Unmarshal(b, &m) == nil {
			if v, ok := m["ps"].(string); ok {
				info.Name = v
			}
			if v, ok := m["add"].(string); ok {
				info.Host = v
			}
			switch p := m["port"].(type) {
			case string:
				info.Port, _ = strconv.Atoi(p)
			case float64:
				info.Port = int(p)
			}
		}
	case "ss", "shadowsocks":
		info.Proto = "shadowsocks"
		rest := raw[strings.Index(raw, "://")+3:]
		// ss://base64(method:password@host:port) 或 ss://base64(method:password)@host:port
		if i := strings.Index(rest, "@"); i >= 0 {
			// 后半是 host:port
			hp := rest[i+1:]
			if h, p, ok := splitHostPort(hp); ok {
				info.Host, info.Port = h, p
			}
		} else {
			b := b64decode(rest)
			s := string(b)
			if i := strings.LastIndex(s, "@"); i >= 0 {
				if h, p, ok := splitHostPort(s[i+1:]); ok {
					info.Host, info.Port = h, p
				}
			}
		}
	case "ssr":
		info.Proto = "shadowsocksr"
		b := b64decode(strings.TrimPrefix(raw, "ssr://"))
		parts := strings.Split(string(b), ":")
		if len(parts) >= 2 {
			info.Host = parts[0]
			info.Port, _ = strconv.Atoi(parts[1])
		}
	default:
		info.Proto = scheme
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			info.Host = u.Hostname()
			info.Port, _ = strconv.Atoi(u.Port())
		}
	}
	return info
}

func splitHostPort(hp string) (string, int, bool) {
	// 处理 [ipv6]:port
	if strings.HasPrefix(hp, "[") {
		if i := strings.LastIndex(hp, "]:"); i >= 0 {
			p, _ := strconv.Atoi(hp[i+2:])
			return hp[1:i], p, true
		}
		return strings.Trim(hp, "[]"), 0, true
	}
	if i := strings.LastIndex(hp, ":"); i >= 0 {
		p, err := strconv.Atoi(hp[i+1:])
		if err == nil {
			return hp[:i], p, true
		}
	}
	return hp, 0, hp != ""
}

// ProtoLabel 协议显示名
func ProtoLabel(proto string) string {
	switch proto {
	case "vless":
		return "VLESS"
	case "vmess":
		return "VMess"
	case "trojan":
		return "Trojan"
	case "shadowsocks":
		return "SS"
	case "shadowsocksr":
		return "SSR"
	case "hysteria2":
		return "Hysteria2"
	case "hysteria":
		return "Hysteria"
	case "tuic":
		return "TUIC"
	case "wireguard":
		return "WireGuard"
	case "http", "https":
		return "订阅"
	default:
		if proto == "" {
			return "未知"
		}
		return strings.ToUpper(proto)
	}
}
