// Package secpolicy implements the login security policy for SecretDock:
// GeoIP country allowlist, IP whitelist/blacklist (CIDR, IPv4+IPv6),
// private-IP bypass, and proxy-aware client IP extraction.
//
// Evaluation order for a client IP (when the policy is enabled):
//  1. private/loopback IPs are always allowed (intranet default whitelist)
//  2. banned IPs (one-click ban from TG alerts) are always denied
//  3. blacklisted IPs/CIDRs are always denied
//  4. whitelisted IPs/CIDRs are always allowed
//  5. otherwise the IP's country must be in the allowed country list
package secpolicy

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

// privatePrefixes are always allowed when the policy is enabled:
// loopback, RFC1918, link-local and unique-local ranges ("intranet
// default whitelist").
var privatePrefixes = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
}

// IsPrivate reports whether ip is a loopback/private/link-local address.
func IsPrivate(ip netip.Addr) bool {
	for _, p := range privatePrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseIPList parses newline/comma/semicolon separated IPs and CIDRs
// (IPv4 and IPv6). A bare IP is treated as a /32 (or /128) prefix.
func ParseIPList(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	}) {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if strings.Contains(f, "/") {
			p, err := netip.ParsePrefix(f)
			if err != nil {
				return nil, err
			}
			out = append(out, p.Masked())
		} else {
			a, err := netip.ParseAddr(f)
			if err != nil {
				return nil, err
			}
			bits := 128
			if a.Is4() {
				bits = 32
			}
			out = append(out, netip.PrefixFrom(a, bits))
		}
	}
	return out, nil
}

// InList reports whether ip is covered by any prefix in list.
func InList(ip netip.Addr, list []netip.Prefix) bool {
	for _, p := range list {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// GeoIP is a hot-swappable country database reader.
type GeoIP struct {
	mu sync.RWMutex
	db *geoip2.Reader
}

// OpenGeoIP opens the MMDB at path.
func OpenGeoIP(path string) (*GeoIP, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, err
	}
	return &GeoIP{db: db}, nil
}

// Reload atomically swaps the database to the one at path.
func (g *GeoIP) Reload(path string) error {
	db, err := geoip2.Open(path)
	if err != nil {
		return err
	}
	g.mu.Lock()
	old := g.db
	g.db = db
	g.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Loaded reports whether a database is currently open.
func (g *GeoIP) Loaded() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.db != nil
}

// Country returns the ISO 3166-1 alpha-2 country code (e.g. "CN") for ip.
func (g *GeoIP) Country(ip netip.Addr) (string, error) {
	g.mu.RLock()
	db := g.db
	g.mu.RUnlock()
	if db == nil {
		return "", errNoDB
	}
	rec, err := db.Country(net.ParseIP(ip.String()))
	if err != nil {
		return "", err
	}
	return rec.Country.IsoCode, nil
}

type geoErr string

func (e geoErr) Error() string { return string(e) }

const errNoDB geoErr = "IP 国家库未加载"

// Config is the evaluated login security policy.
type Config struct {
	Enabled   bool
	Countries map[string]bool // upper-case ISO codes
	Whitelist []netip.Prefix
	Blacklist []netip.Prefix
	Banned    map[string]bool // one-click banned IPs (exact match)
}

// Evaluate returns (allow, reasonCN) for ipStr under cfg.
func Evaluate(ipStr string, cfg Config, geo *GeoIP) (bool, string) {
	if !cfg.Enabled {
		return true, "未启用"
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ipStr))
	if err != nil {
		return false, "IP 格式无效"
	}
	if IsPrivate(addr) {
		return true, "内网 IP"
	}
	if cfg.Banned[ipStr] || cfg.Banned[addr.String()] {
		return false, "该 IP 已被封禁"
	}
	if InList(addr, cfg.Blacklist) {
		return false, "命中 IP 黑名单"
	}
	if InList(addr, cfg.Whitelist) {
		return true, "命中 IP 白名单"
	}
	if len(cfg.Countries) == 0 {
		return false, "未配置允许的国家/地区"
	}
	if geo == nil || !geo.Loaded() {
		return false, "IP 国家库未加载"
	}
	cc, err := geo.Country(addr)
	if err != nil || cc == "" {
		return false, "无法识别 IP 归属地"
	}
	cc = strings.ToUpper(cc)
	if cfg.Countries[cc] {
		return true, "归属地 " + cc
	}
	return false, "归属地 " + cc + " 不在允许列表"
}

// ClientIP extracts the real client IP behind a reverse proxy.
// X-Forwarded-For / X-Real-IP are only trusted when the direct TCP peer
// is loopback or private (i.e. the request came via the local nginx),
// which defeats XFF spoofing through direct connections.
func ClientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if h, _, err := net.SplitHostPort(peer); err == nil {
		peer = h
	}
	// Cloudflare 真实 IP（优先级最高）
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return strings.TrimSpace(cf)
	}
	if a, err := netip.ParseAddr(strings.TrimSpace(peer)); err == nil && IsPrivate(a) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.Index(xff, ","); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		if xr := r.Header.Get("X-Real-IP"); xr != "" {
			return strings.TrimSpace(xr)
		}
	}
	return strings.TrimSpace(peer)
}
