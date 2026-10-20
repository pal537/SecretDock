// Package handlers implements the SecretDock JSON API and static serving.
package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/internal/secpolicy"
	"github.com/example/secretdock/internal/tg"
	"github.com/example/secretdock/web"
)

// Server holds shared dependencies.
type Server struct {
	DB      *db.DB
	Auth    *auth.Auth
	EncKey  []byte
	DataDir string
	TG      *tg.Service
	Geo     *secpolicy.GeoIP // IP country database (may be unloaded)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

func ok(w http.ResponseWriter, v interface{}) { writeJSON(w, 200, v) }
func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// securityHeaders adds baseline hardening headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// checkIPPolicyMiddleware blocks the page itself if IP is not allowed
func (s *Server) checkIPPolicyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := secpolicy.ClientIP(r)
		if ok, reason := s.checkIPPolicy(ip); !ok {
			s.DB.Audit("ip_denied", reason, ip)
			w.WriteHeader(403)
			return
		}
		next(w, r)
	}
}

// requireAuth guards API routes (except login). Sessions are bound to the
// login IP (IP change drops the session) and the IP security policy is
// re-evaluated on every request.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := secpolicy.ClientIP(r)
		tok := auth.CookieToken(r)
		if !s.Auth.Valid(tok, ip) {
			fail(w, 401, "unauthorized")
			return
		}
		if ok, reason := s.checkIPPolicy(ip); !ok {
			s.Auth.Revoke(tok)
			s.DB.Audit("ip_denied", reason, ip)
			fail(w, 403, "当前 IP 不允许访问："+reason)
			return
		}
		next(w, r)
	}
}

// Routes registers all routes on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/setup/status", s.handleSetupStatus)
	mux.HandleFunc("/api/setup", s.handleSetup)
	mux.HandleFunc("/api/change-password", s.requireAuth(s.handleChangePassword))
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/login/2fa", s.handleLogin2FA)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/me", s.requireAuth(s.handleMe))

	mux.HandleFunc("/api/merchants", s.requireAuth(s.handleMerchants))
	mux.HandleFunc("/api/merchants/", s.requireAuth(s.handleMerchantOne))
	mux.HandleFunc("/api/logo/candidates", s.requireAuth(s.handleLogoCandidates))

	mux.HandleFunc("/api/secrets", s.requireAuth(s.handleSecrets))
	mux.HandleFunc("/api/secrets/", s.requireAuth(s.handleSecretOne))
	mux.HandleFunc("/api/scripts", s.requireAuth(s.handleScripts))
	mux.HandleFunc("/api/scripts/", s.requireAuth(s.handleScriptOne))
	mux.HandleFunc("/api/script-categories", s.requireAuth(s.handleScriptCategories))
	mux.HandleFunc("/api/script-categories/", s.requireAuth(s.handleScriptCategoryOne))
	mux.HandleFunc("/api/note-categories", s.requireAuth(s.handleNoteCategories))
	mux.HandleFunc("/api/note-categories/", s.requireAuth(s.handleNoteCategoryOne))
	mux.HandleFunc("/api/reminders", s.requireAuth(s.handleReminders))
	mux.HandleFunc("/api/reminders/", s.requireAuth(s.handleReminderOne))
	mux.HandleFunc("/api/reminder-categories", s.requireAuth(s.handleReminderCategories))
	mux.HandleFunc("/api/reminder-categories/", s.requireAuth(s.handleReminderCategoryOne))
	mux.HandleFunc("/api/link-categories", s.requireAuth(s.handleLinkCategories))
	mux.HandleFunc("/api/link-categories/", s.requireAuth(s.handleLinkCategoryOne))
	mux.HandleFunc("/api/links", s.requireAuth(s.handleLinks))
	mux.HandleFunc("/api/links/", s.requireAuth(s.handleLinkOne))
	mux.HandleFunc("/api/links-move", s.requireAuth(s.handleLinksMove))
	mux.HandleFunc("/api/node-categories", s.requireAuth(s.handleNodeCategories))
	mux.HandleFunc("/api/node-categories/", s.requireAuth(s.handleNodeCategoryOne))
	mux.HandleFunc("/api/nodes", s.requireAuth(s.handleNodes))
	mux.HandleFunc("/api/nodes/", s.requireAuth(s.handleNodeOne))
	mux.HandleFunc("/api/nodes-test", s.requireAuth(s.handleNodesTest))
	mux.HandleFunc("/api/ssh-categories", s.requireAuth(s.handleSSHCategories))
	mux.HandleFunc("/api/ssh-categories/", s.requireAuth(s.handleSSHCategoryOne))
	mux.HandleFunc("/api/ssh-hosts", s.requireAuth(s.handleSSHHosts))
	mux.HandleFunc("/api/ssh-hosts/", s.requireAuth(s.handleSSHHostOne))
	mux.HandleFunc("/api/ssh-test", s.requireAuth(s.handleSSHTest))
	mux.HandleFunc("/api/ssh-stats", s.requireAuth(s.handleSSHStats))
	mux.HandleFunc("/api/ssh-netspeed", s.requireAuth(s.handleSSHNetSpeed))
	mux.HandleFunc("/api/ssh-detect-os", s.requireAuth(s.handleSSHDetectOS))
	mux.HandleFunc("/api/ssh-files", s.requireAuth(s.handleSSHFiles))
	mux.HandleFunc("/api/ssh-download", s.requireAuth(s.handleSSHFileDownload))
	mux.HandleFunc("/api/ssh-upload", s.requireAuth(s.handleSSHUpload))
	mux.HandleFunc("/ws/ssh", s.handleSSHWS)
	mux.HandleFunc("/api/ssh-diag", s.requireAuth(s.handleSSHDiag))
	mux.HandleFunc("/api/notes", s.requireAuth(s.handleNotes))
	mux.HandleFunc("/api/notes/", s.requireAuth(s.handleNoteOne))
	mux.HandleFunc("/api/image-categories", s.requireAuth(s.handleImageCategories))
	mux.HandleFunc("/api/image-categories/", s.requireAuth(s.handleImageCategoryOne))
	mux.HandleFunc("/api/images", s.requireAuth(s.handleImages))
	mux.HandleFunc("/api/file-categories", s.requireAuth(s.handleFileCategories))
	mux.HandleFunc("/api/file-categories/", s.requireAuth(s.handleFileCategoryOne))
	mux.HandleFunc("/api/files", s.requireAuth(s.handleFiles))
	mux.HandleFunc("/api/files/", s.requireAuth(s.handleFileOne))
	mux.HandleFunc("/api/images/", s.requireAuth(s.handleImageOne))
	mux.HandleFunc("/api/images-move", s.requireAuth(s.handleImagesMove))
	mux.HandleFunc("/api/pin", s.requireAuth(s.handlePin))
	mux.HandleFunc("/api/search", s.requireAuth(s.handleSearch))
	mux.HandleFunc("/api/expiring", s.requireAuth(s.handleExpiring))
	mux.HandleFunc("/api/labels", s.requireAuth(s.handleLabels))

	mux.HandleFunc("/api/settings", s.requireAuth(s.handleSettings))
	mux.HandleFunc("/api/settings/query-password", s.requireAuth(s.handleQueryPassword))
	mux.HandleFunc("/api/settings/tg-test", s.requireAuth(s.handleTgTest))
	mux.HandleFunc("/api/settings/tg-unban", s.requireAuth(s.handleTgUnban))
	// login security
	mux.HandleFunc("/api/settings/security/ipinfo", s.requireAuth(s.handleIPInfo))
	mux.HandleFunc("/api/settings/security/geoip-update", s.requireAuth(s.handleGeoIPUpdate))
	mux.HandleFunc("/api/settings/2fa/status", s.requireAuth(s.handle2FAStatus))
	mux.HandleFunc("/api/settings/2fa/setup", s.requireAuth(s.handle2FASetup))
	mux.HandleFunc("/api/settings/2fa/confirm", s.requireAuth(s.handle2FAConfirm))
	mux.HandleFunc("/api/settings/2fa/disable", s.requireAuth(s.handle2FADisable))
	mux.HandleFunc("/api/settings/security/trusted", s.requireAuth(s.handleTrustedClear))
	mux.HandleFunc("/api/settings/security/bans", s.requireAuth(s.handleBans))
	mux.HandleFunc("/api/audit", s.requireAuth(s.handleAudit))

	mux.HandleFunc("/api/share", s.requireAuth(s.handleShareCreate))
	mux.HandleFunc("/s/", s.handleShareView)
	mux.HandleFunc("/api/data/clear", s.requireAuth(s.handleDataClear))
	mux.HandleFunc("/api/backup/export", s.requireAuth(s.handleBackupExport))
	mux.HandleFunc("/api/backup/import", s.requireAuth(s.handleBackupImport))

	mux.HandleFunc("/api/menu-icon", s.requireAuth(s.handleMenuIconFetch))
	mux.HandleFunc("/menu-icons/", s.requireAuth(s.handleMenuIconFile))
	mux.HandleFunc("/logos/", s.requireAuth(s.handleLogoFile))
	mux.HandleFunc("/static/", s.checkIPPolicyMiddleware(s.handleStatic()))
	mux.HandleFunc("/", s.checkIPPolicyMiddleware(s.handleIndex()))
}

func (s *Server) handleStatic() http.HandlerFunc {
	sub, _ := web.FS.ReadDir("static")
	_ = sub
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/static/")
		p = path.Clean(p)
		if strings.Contains(p, "..") {
			http.NotFound(w, r)
			return
		}
		data, err := web.FS.ReadFile("static/" + p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// js 里的 __APPV__ 也替换，保证 logo 等资源刷新
		if strings.HasSuffix(p, ".js") {
			ver := os.Getenv("APP_VERSION")
			if ver == "" {
				ver = "dev"
			}
			data = []byte(strings.ReplaceAll(string(data), "__APPV__", ver))
		}
		switch {
		case strings.HasSuffix(p, ".js"):
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case strings.HasSuffix(p, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(p, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(data)
	}
}

func (s *Server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := web.FS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "missing frontend", 500)
			return
		}
		ver := os.Getenv("APP_VERSION")
		if ver == "" {
			ver = "dev"
		}
		html := strings.ReplaceAll(string(data), "__APPV__", ver)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(html))
	}
}

// Wrap applies security headers to the whole mux.
func (s *Server) Wrap(mux *http.ServeMux) http.Handler {
	return securityHeaders(mux)
}

// ---- auth endpoints ----

// validatePassword: >10位，含大写+小写+数字
func validatePassword(pw string) bool {
	if len(pw) <= 10 {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range pw {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]bool{"needSetup": !s.DB.HasUser()})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	if s.DB.HasUser() {
		fail(w, 403, "already set up")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		fail(w, 400, "用户名不能为空")
		return
	}
	if !validatePassword(req.Password) {
		fail(w, 400, "密码必须大于10位，包含大写字母、小写字母和数字")
		return
	}
	hash, err := auth.HashSecret(auth.NormalizeSecret(req.Password))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := s.DB.CreateUser(req.Username, hash); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.DB.Audit("setup", "user="+req.Username, clientIP(r))
	// 直接登录
	tok := s.Auth.NewSession(false, clientIP(r))
	s.Auth.SetCookie(w, tok, false)
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	_, dbHash, err := s.DB.GetUser()
	if err != nil || !auth.VerifySecret(dbHash, auth.NormalizeSecret(req.OldPassword)) {
		fail(w, 401, "当前密码错误")
		return
	}
	if !validatePassword(req.NewPassword) {
		fail(w, 400, "新密码必须大于10位，包含大写字母、小写字母和数字")
		return
	}
	hash, err := auth.HashSecret(auth.NormalizeSecret(req.NewPassword))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := s.DB.UpdatePassword(hash); err != nil {
		fail(w, 500, err.Error())
		return
	}
	// 撤销所有 session，强制重新登录
	s.Auth.RevokeAll()
	s.DB.Audit("change_password", "ok", clientIP(r))
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	ip := clientIP(r)
	// IP security policy applies before the password check.
	if ok, reason := s.checkIPPolicy(ip); !ok {
		s.DB.Audit("login_ip_denied", reason, ip)
		fail(w, 403, "当前 IP 不允许访问："+reason)
		return
	}
	if s.Auth.Locked(r) {
		s.DB.Audit("login_locked", "", ip)
		fail(w, 403, "登录已锁定，请重启容器解锁")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	// DB 验证
	dbUser, dbHash, err := s.DB.GetUser()
	if err != nil || req.Username != dbUser || !auth.VerifySecret(dbHash, auth.NormalizeSecret(req.Password)) {
		locked := s.Auth.RecordFail(r)
		s.DB.Audit("login_fail", "user="+req.Username, ip)
		if locked {
			s.notifySecAlert(tg.SecAlertLocked, ip)
			fail(w, 403, "登录已锁定，请重启容器解锁")
		} else {
			s.notifySecAlert(tg.SecAlertPassFail, ip)
			fail(w, 401, "用户名或密码错误")
		}
		return
	}
	s.Auth.RecordSuccess(r)
	// 2FA: already-verified IPs skip the code step; new IPs must verify.
	if s.totpEnabled() && !s.isTrustedIP(ip) {
		tmp := s.Auth.NewPending(ip, req.Remember)
		s.DB.Audit("login_2fa_required", "", ip)
		ok(w, map[string]interface{}{"need2fa": true, "tmp": tmp})
		return
	}
	tok := s.Auth.NewSession(req.Remember, ip)
	s.Auth.SetCookie(w, tok, req.Remember)
	s.DB.Audit("login", "ok", ip)
	s.notifyLoginSuccess(ip)
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.Revoke(auth.CookieToken(r))
	s.Auth.ClearCookie(w)
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]bool{"ok": true})
}

// clientIP extracts the real client IP. X-Forwarded-For is only trusted
// when the direct peer is loopback/private (local reverse proxy), which
// defeats XFF spoofing through direct connections.
func clientIP(r *http.Request) string {
	return secpolicy.ClientIP(r)
}

// idFromPath extracts the trailing integer id from /api/x/{id}[/*].
func idFromPath(p, prefix string) (int64, bool) {
	rest := strings.TrimPrefix(p, prefix)
	rest = strings.Trim(rest, "/")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	return id, err == nil
}

// decryptSecrets converts ciphertext values to plaintext for API output.
func (s *Server) decryptSecrets(in []db.Secret) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(in))
	for _, x := range in {
		raw, err := crypto.Decrypt(s.EncKey, x.Value)
		val := ""
		if err == nil {
			val = string(raw)
		}
		out = append(out, map[string]interface{}{
			"id": x.ID, "merchantId": x.MerchantID, "kind": x.Kind,
			"label": x.Label, "username": x.Username, "value": val,
			"note": x.Note, "expiresAt": x.ExpiresAt, "createdAt": x.CreatedAt,
		})
	}
	return out
}

