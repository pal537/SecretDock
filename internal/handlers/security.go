// Login security: IP policy (country allowlist / whitelist / blacklist)
// and TOTP two-factor authentication.
package handlers

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/secpolicy"
	"github.com/example/secretdock/internal/tg"
)

// ---- policy plumbing ----

// secCfg loads the login security policy from DB settings.
func (s *Server) secCfg() secpolicy.Config {
	cfg := secpolicy.Config{Enabled: s.DB.GetSetting("sec_enabled") == "1"}
	if !cfg.Enabled {
		return cfg
	}
	cfg.Countries = map[string]bool{}
	for _, c := range strings.Split(s.DB.GetSetting("sec_countries"), ",") {
		c = strings.ToUpper(strings.TrimSpace(c))
		if len(c) == 2 {
			cfg.Countries[c] = true
		}
	}
	if wl, err := secpolicy.ParseIPList(s.DB.GetSetting("sec_ip_whitelist")); err == nil {
		cfg.Whitelist = wl
	}
	if bl, err := secpolicy.ParseIPList(s.DB.GetSetting("sec_ip_blacklist")); err == nil {
		cfg.Blacklist = bl
	}
	cfg.Banned = map[string]bool{}
	for ip := range s.bans() {
		cfg.Banned[ip] = true
	}
	return cfg
}

// checkIPPolicy evaluates the client IP against the policy.
// The one-click ban list is enforced even when the IP policy is disabled.
func (s *Server) checkIPPolicy(ip string) (bool, string) {
	if s.isBanned(ip) {
		return false, "该 IP 已被封禁"
	}
	return secpolicy.Evaluate(ip, s.secCfg(), s.Geo)
}

// isBanned reports whether ip is on the one-click ban list.
func (s *Server) isBanned(ip string) bool {
	a, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	_, ok := s.bans()[a.String()]
	return ok
}

// totpEnabled reports whether TOTP 2FA is active.
func (s *Server) totpEnabled() bool {
	return s.DB.GetSetting("sec_totp_enabled") == "1" && s.DB.GetSetting("sec_totp_secret_enc") != ""
}

// trustedIPs loads the IP -> verified-unix map.
func (s *Server) trustedIPs() map[string]int64 {
	m := map[string]int64{}
	_ = json.Unmarshal([]byte(s.DB.GetSetting("sec_trusted_ips")), &m)
	return m
}

func (s *Server) saveTrustedIPs(m map[string]int64) {
	b, _ := json.Marshal(m)
	_ = s.DB.SetSetting("sec_trusted_ips", string(b))
}

// isTrustedIP reports whether this IP already passed 2FA once.
func (s *Server) isTrustedIP(ip string) bool {
	_, ok := s.trustedIPs()[ip]
	return ok
}

func (s *Server) trustIP(ip string) {
	m := s.trustedIPs()
	m[ip] = time.Now().Unix()
	s.saveTrustedIPs(m)
}

// ---- login 2FA step ----

// 2FA code brute-force guard: 5 wrong codes -> pending tokens for the IP
// are revoked and the user must redo the password step.
var (
	twoFAMu    sync.Mutex
	twoFAFails = map[string]int{}
)

func (s *Server) handleLogin2FA(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	ip := secpolicy.ClientIP(r)
	if ok, reason := s.checkIPPolicy(ip); !ok {
		s.DB.Audit("login_ip_denied", reason, ip)
		fail(w, 403, "当前 IP 不允许访问："+reason)
		return
	}
	var req struct {
		Tmp  string `json:"tmp"`
		Code string `json:"code"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	good, remember := s.Auth.ConsumePending(req.Tmp, ip)
	if !good {
		fail(w, 401, "验证已过期，请重新登录")
		return
	}
	if !s.totpEnabled() {
		fail(w, 400, "2FA 未启用")
		return
	}

	code := strings.TrimSpace(req.Code)
	passed := false
	usedRecovery := false

	// TOTP first, then single-use recovery codes.
	if enc := s.DB.GetSetting("sec_totp_secret_enc"); enc != "" {
		if raw, err := crypto.Decrypt(s.EncKey, enc); err == nil {
			passed = secpolicy.ValidateTOTP(string(raw), code)
		}
	}
	if !passed && s.consumeRecoveryCode(code) {
		passed = true
		usedRecovery = true
	}
	if !passed {
		twoFAMu.Lock()
		twoFAFails[ip]++
		n := twoFAFails[ip]
		twoFAMu.Unlock()
		s.DB.Audit("login_2fa_fail", fmt.Sprintf("第%d次", n), ip)
		if n >= 5 {
			s.Auth.DropPendingForIP(ip)
			twoFAMu.Lock()
			delete(twoFAFails, ip)
			twoFAMu.Unlock()
			fail(w, 401, "动态码错误次数过多，请重新输入账号密码登录")
			return
		}
		// re-issue a fresh pending token so the user can retry the code
		// without retyping the password (still IP-bound, 5 min).
		tmp := s.Auth.NewPending(ip, remember)
		writeJSON(w, 401, map[string]interface{}{"error": "动态码错误", "tmp": tmp})
		return
	}

	twoFAMu.Lock()
	delete(twoFAFails, ip)
	twoFAMu.Unlock()
	s.Auth.RecordSuccess(r)
	s.trustIP(ip)
	tok := s.Auth.NewSession(remember, ip)
	s.Auth.SetCookie(w, tok, remember)
	s.DB.Audit("login_2fa", "ok", ip)
	s.notifyLoginSuccess(ip)
	resp := map[string]interface{}{"ok": true}
	if usedRecovery {
		resp["recoveryUsed"] = true
		resp["recoveryLeft"] = len(s.recoveryHashes())
	}
	ok(w, resp)
}

// recoveryHashes loads stored (hashed) recovery codes.
func (s *Server) recoveryHashes() []string {
	var out []string
	_ = json.Unmarshal([]byte(s.DB.GetSetting("sec_totp_recovery")), &out)
	return out
}

// consumeRecoveryCode checks code against stored hashes; on match the code
// is burned (single use). Returns true on success.
func (s *Server) consumeRecoveryCode(code string) bool {
	norm := secpolicy.NormalizeRecoveryCode(code)
	if norm == "" {
		return false
	}
	hashes := s.recoveryHashes()
	for i, h := range hashes {
		if auth.VerifySecret(h, norm) {
			hashes = append(hashes[:i], hashes[i+1:]...)
			b, _ := json.Marshal(hashes)
			_ = s.DB.SetSetting("sec_totp_recovery", string(b))
			return true
		}
	}
	return false
}

// ---- 2FA management (authenticated) ----

func (s *Server) handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	trusted := s.trustedIPs()
	type tip struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		Since   int64  `json:"since"`
	}
	list := []tip{}
	for ip, ts := range trusted {
		cc := ""
		if s.Geo != nil && s.Geo.Loaded() {
			if addr, err := parseIP(ip); err == nil {
				cc, _ = s.Geo.Country(addr)
			}
		}
		list = append(list, tip{IP: ip, Country: cc, Since: ts})
	}
	ok(w, map[string]interface{}{
		"enabled":     s.totpEnabled(),
		"hasPending":  s.DB.GetSetting("sec_totp_pending_enc") != "",
		"trusted":     list,
		"geoipLoaded": s.Geo != nil && s.Geo.Loaded(),
	})
}

func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	if s.totpEnabled() {
		fail(w, 400, "2FA 已启用")
		return
	}
	username, _, _ := s.DB.GetUser()
	secret, uri, err := secpolicy.GenerateTOTP(username)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	enc, err := crypto.Encrypt(s.EncKey, []byte(secret))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	_ = s.DB.SetSetting("sec_totp_pending_enc", enc)

	// Recovery codes: show plaintext once, store hashes.
	codes, err := secpolicy.NewRecoveryCodes(10)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	hashes := make([]string, 0, len(codes))
	for _, c := range codes {
		h, err := auth.HashSecret(secpolicy.NormalizeRecoveryCode(c))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		hashes = append(hashes, h)
	}
	b, _ := json.Marshal(hashes)
	_ = s.DB.SetSetting("sec_totp_pending_recovery", string(b))

	qr, err := secpolicy.QRCodeDataURI(uri)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.DB.Audit("2fa_setup_start", "", secpolicy.ClientIP(r))
	ok(w, map[string]interface{}{
		"secret":        secret,
		"qr":            qr,
		"recoveryCodes": codes,
	})
}

func (s *Server) handle2FAConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	enc := s.DB.GetSetting("sec_totp_pending_enc")
	if enc == "" {
		fail(w, 400, "请先开始 2FA 设置流程")
		return
	}
	raw, err := crypto.Decrypt(s.EncKey, enc)
	if err != nil {
		fail(w, 500, "密钥解密失败")
		return
	}
	if !secpolicy.ValidateTOTP(string(raw), req.Code) {
		s.DB.Audit("2fa_setup_fail", "code mismatch", secpolicy.ClientIP(r))
		fail(w, 401, "动态码不正确，请检查手机时间是否准确后重试")
		return
	}
	// First verification passed: activate.
	_ = s.DB.SetSetting("sec_totp_secret_enc", enc)
	_ = s.DB.SetSetting("sec_totp_enabled", "1")
	_ = s.DB.SetSetting("sec_totp_pending_enc", "")
	if pr := s.DB.GetSetting("sec_totp_pending_recovery"); pr != "" {
		_ = s.DB.SetSetting("sec_totp_recovery", pr)
	}
	_ = s.DB.SetSetting("sec_totp_pending_recovery", "")
	// The IP that set it up is trusted from now on.
	s.trustIP(secpolicy.ClientIP(r))
	s.DB.Audit("2fa_enabled", "ok", secpolicy.ClientIP(r))
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	_, dbHash, err := s.DB.GetUser()
	if err != nil || !auth.VerifySecret(dbHash, auth.NormalizeSecret(req.Password)) {
		s.DB.Audit("2fa_disable_fail", "bad password", secpolicy.ClientIP(r))
		fail(w, 401, "密码错误")
		return
	}
	for _, k := range []string{"sec_totp_enabled", "sec_totp_secret_enc", "sec_totp_pending_enc", "sec_totp_pending_recovery", "sec_totp_recovery", "sec_trusted_ips"} {
		_ = s.DB.SetSetting(k, "")
	}
	s.DB.Audit("2fa_disabled", "ok", secpolicy.ClientIP(r))
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handleTrustedClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		fail(w, 405, "method not allowed")
		return
	}
	_ = s.DB.SetSetting("sec_trusted_ips", "")
	s.DB.Audit("2fa_trusted_clear", "ok", secpolicy.ClientIP(r))
	ok(w, map[string]bool{"ok": true})
}

// ---- IP info + GeoIP update ----

func (s *Server) handleIPInfo(w http.ResponseWriter, r *http.Request) {
	ip := secpolicy.ClientIP(r)
	allow, reason := s.checkIPPolicy(ip)
	cc := ""
	if s.Geo != nil && s.Geo.Loaded() {
		if addr, err := parseIP(ip); err == nil {
			cc, _ = s.Geo.Country(addr)
		}
	}
	ok(w, map[string]interface{}{
		"ip":          ip,
		"country":     cc,
		"allowed":     allow,
		"reason":      reason,
		"secEnabled":  s.DB.GetSetting("sec_enabled") == "1",
		"geoipLoaded": s.Geo != nil && s.Geo.Loaded(),
	})
}

// handleGeoIPUpdate downloads the latest DB-IP country database
// (current month, falling back to previous month) and hot-reloads it.
func (s *Server) handleGeoIPUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	month, err := updateGeoIPDB(s.DataDir)
	if err != nil {
		fail(w, 500, "下载失败："+err.Error())
		return
	}
	dbPath := filepath.Join(s.DataDir, "geoip", "dbip.mmdb")
	if s.Geo == nil {
		g, err := secpolicy.OpenGeoIP(dbPath)
		if err != nil {
			fail(w, 500, "加载失败："+err.Error())
			return
		}
		s.Geo = g
	} else if err := s.Geo.Reload(dbPath); err != nil {
		fail(w, 500, "加载失败："+err.Error())
		return
	}
	s.DB.Audit("geoip_update", month, secpolicy.ClientIP(r))
	ok(w, map[string]string{"ok": "1", "month": month})
}

// updateGeoIPDB downloads the DB-IP country-lite MMDB for the current
// (or previous) month into <dataDir>/geoip/dbip.mmdb.
func updateGeoIPDB(dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "geoip")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 180 * time.Second}
	now := time.Now()
	var lastErr error
	for _, t := range []time.Time{now, now.AddDate(0, -1, 0)} {
		month := fmt.Sprintf("%04d-%02d", t.Year(), int(t.Month()))
		url := "https://download.db-ip.com/free/dbip-country-lite-" + month + ".mmdb.gz"
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "SecretDock/geoip-update")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d (%s)", resp.StatusCode, month)
			continue
		}
		tmp := filepath.Join(dir, "dbip.mmdb.tmp")
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			resp.Body.Close()
			return "", err
		}
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			f.Close()
			resp.Body.Close()
			return "", err
		}
		_, err = io.Copy(f, gz)
		gz.Close()
		resp.Body.Close()
		f.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if err := os.Rename(tmp, filepath.Join(dir, "dbip.mmdb")); err != nil {
			return "", err
		}
		return month, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no monthly database available")
	}
	return "", lastErr
}

// saveSecuritySettings validates and persists the login-security settings
// from a PUT /api/settings body. It writes the error response itself and
// returns non-nil when the caller must stop.
func (s *Server) saveSecuritySettings(req map[string]string, w http.ResponseWriter, r *http.Request) error {
	touched := false
	for _, k := range []string{"sec_ip_whitelist", "sec_ip_blacklist", "sec_countries", "sec_enabled"} {
		if _, ok := req[k]; ok {
			touched = true
		}
	}
	if !touched {
		return nil
	}

	// Validate IP lists.
	for _, k := range []string{"sec_ip_whitelist", "sec_ip_blacklist"} {
		if v, ok := req[k]; ok {
			if _, err := secpolicy.ParseIPList(v); err != nil {
				fail(w, 400, "IP 名单格式错误："+err.Error())
				return err
			}
		}
	}
	// Validate country codes (two letters each).
	countries := s.DB.GetSetting("sec_countries")
	if v, ok := req["sec_countries"]; ok {
		countries = v
	}
	for _, c := range strings.Split(countries, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if len(c) != 2 {
			err := fmt.Errorf("国家代码格式错误：%s", c)
			fail(w, 400, err.Error())
			return err
		}
	}

	enabling := req["sec_enabled"] == "1"
	disabling := req["sec_enabled"] == "0"
	// Guard runs when the policy will be enabled after this save and any
	// policy field changed: a new rule must never lock out the current IP.
	willBeEnabled := enabling || (s.DB.GetSetting("sec_enabled") == "1" && !disabling)
	policyTouched := enabling
	for _, k := range []string{"sec_countries", "sec_ip_whitelist", "sec_ip_blacklist"} {
		if _, ok := req[k]; ok {
			policyTouched = true
		}
	}
	if willBeEnabled && policyTouched {
		whitelist := s.DB.GetSetting("sec_ip_whitelist")
		if v, ok := req["sec_ip_whitelist"]; ok {
			whitelist = v
		}
		hasCountries := strings.Trim(strings.Trim(countries, ","), " ") != ""
		hasWhitelist := strings.TrimSpace(whitelist) != ""
		if !hasCountries && !hasWhitelist {
			err := fmt.Errorf("请至少选择一个允许的国家/地区，或填写 IP 白名单，否则你自己也登录不了")
			fail(w, 400, err.Error())
			return err
		}
		if hasCountries && (s.Geo == nil || !s.Geo.Loaded()) {
			err := fmt.Errorf("IP 国家库未加载，请先点击「更新 IP 库」")
			fail(w, 400, err.Error())
			return err
		}
		// Self-lockout guard: the new policy must allow the current IP.
		cfg := s.secCfg()
		cfg.Enabled = true
		cfg.Countries = map[string]bool{}
		for _, c := range strings.Split(countries, ",") {
			c = strings.ToUpper(strings.TrimSpace(c))
			if len(c) == 2 {
				cfg.Countries[c] = true
			}
		}
		if wl, err := secpolicy.ParseIPList(whitelist); err == nil {
			cfg.Whitelist = wl
		}
		if bl, ok := req["sec_ip_blacklist"]; ok {
			if parsed, err := secpolicy.ParseIPList(bl); err == nil {
				cfg.Blacklist = parsed
			}
		} else if parsed, err := secpolicy.ParseIPList(s.DB.GetSetting("sec_ip_blacklist")); err == nil {
			cfg.Blacklist = parsed
		}
		ip := secpolicy.ClientIP(r)
		if allow, reason := secpolicy.Evaluate(ip, cfg, s.Geo); !allow {
			err := fmt.Errorf("当前 IP %s 会被新规则拒绝（%s），请先把它加入白名单", ip, reason)
			fail(w, 400, err.Error())
			return err
		}
	}

	for _, k := range []string{"sec_enabled", "sec_countries", "sec_ip_whitelist", "sec_ip_blacklist", "sec_tg_alert"} {
		if v, ok := req[k]; ok {
			_ = s.DB.SetSetting(k, v)
		}
	}
	s.DB.Audit("security_settings", "updated", secpolicy.ClientIP(r))
	return nil
}

// ---- one-click IP ban (from TG security alerts) ----

type banInfo struct {
	Reason string `json:"reason"`
	Time   int64  `json:"time"`
}

func (s *Server) bans() map[string]banInfo {
	m := map[string]banInfo{}
	_ = json.Unmarshal([]byte(s.DB.GetSetting("sec_banlist")), &m)
	return m
}

func (s *Server) saveBans(m map[string]banInfo) {
	b, _ := json.Marshal(m)
	_ = s.DB.SetSetting("sec_banlist", string(b))
}

// BanIP bans ip from the website (exact IP, normalized) and revokes its
// live sessions. Idempotent.
func (s *Server) BanIP(ip, reason string) error {
	a, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return fmt.Errorf("IP 无效")
	}
	ip = a.String()
	m := s.bans()
	if _, ok := m[ip]; ok {
		return nil
	}
	m[ip] = banInfo{Reason: reason, Time: time.Now().Unix()}
	s.saveBans(m)
	if s.Auth != nil {
		s.Auth.RevokeByIP(ip)
	}
	s.DB.Audit("ip_banned", reason, ip)
	return nil
}

// UnbanIP removes ip from the ban list.
func (s *Server) UnbanIP(ip string) error {
	a, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return fmt.Errorf("IP 无效")
	}
	ip = a.String()
	m := s.bans()
	if _, ok := m[ip]; !ok {
		return fmt.Errorf("该 IP 不在封禁名单")
	}
	delete(m, ip)
	s.saveBans(m)
	s.DB.Audit("ip_unbanned", "", ip)
	return nil
}

// BanEntry is one banned IP for listing.
type BanEntry struct {
	IP     string `json:"ip"`
	Reason string `json:"reason"`
	Time   int64  `json:"time"`
}

// ListBans returns banned IPs, newest first.
func (s *Server) ListBans() []BanEntry {
	m := s.bans()
	rows := make([]BanEntry, 0, len(m))
	for ip, b := range m {
		rows = append(rows, BanEntry{IP: ip, Reason: b.Reason, Time: b.Time})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Time > rows[j].Time })
	return rows
}
func (s *Server) handleBans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		ok(w, map[string]interface{}{"bans": s.ListBans()})
	case "POST":
		var req struct {
			IP string `json:"ip"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.IP) == "" {
			fail(w, 400, "bad request")
			return
		}
		if err := s.BanIP(req.IP, "后台手动封禁"); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		var req struct {
			IP string `json:"ip"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.IP) == "" {
			fail(w, 400, "bad request")
			return
		}
		if err := s.UnbanIP(req.IP); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// ---- known IPs (for "new IP login" alerts) ----

func (s *Server) knownIPs() map[string]int64 {
	m := map[string]int64{}
	_ = json.Unmarshal([]byte(s.DB.GetSetting("sec_known_ips")), &m)
	return m
}

// isNewIP reports whether ip has never logged in successfully before,
// and records it. Returns true only on the first successful login.
func (s *Server) isNewIP(ip string) bool {
	m := s.knownIPs()
	if _, ok := m[ip]; ok {
		m[ip] = time.Now().Unix()
		b, _ := json.Marshal(m)
		_ = s.DB.SetSetting("sec_known_ips", string(b))
		return false
	}
	m[ip] = time.Now().Unix()
	b, _ := json.Marshal(m)
	_ = s.DB.SetSetting("sec_known_ips", string(b))
	return true
}

// ---- TG security alerts ----

// secAlertOn reports whether TG login-security alerts are enabled.
// Independent of sec_enabled: alerts can run without the IP policy.
func (s *Server) secAlertOn() bool {
	return s.DB.GetSetting("sec_tg_alert") == "1" && s.TG != nil
}

// ipCountry returns the upper-case ISO country code for ip ("" if unknown).
func (s *Server) ipCountry(ip string) string {
	if s.Geo == nil {
		return ""
	}
	a, err := parseIP(ip)
	if err != nil {
		return ""
	}
	cc, _ := s.Geo.Country(a)
	return strings.ToUpper(cc)
}

// notifySecAlert fires a TG security alert without blocking the request.
// kind is one of the tg.SecAlert* constants.
func (s *Server) notifySecAlert(kind, ip string) {
	if !s.secAlertOn() {
		return
	}
	country := s.ipCountry(ip)
	go s.TG.SendSecurityAlert(kind, ip, country)
}

// notifyLoginSuccess sends the login / new-IP alert and tracks the IP.
func (s *Server) notifyLoginSuccess(ip string) {
	if !s.secAlertOn() {
		return
	}
	kind := tg.SecAlertLogin
	if s.isNewIP(ip) {
		kind = tg.SecAlertNewIP
	}
	s.notifySecAlert(kind, ip)
}

// parseIP parses ip into a netip.Addr for GeoIP lookups.
func parseIP(s string) (netip.Addr, error) {
	return netip.ParseAddr(strings.TrimSpace(s))
}
