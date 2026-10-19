package handlers

import (
	"fmt"
	"io"
	"github.com/example/secretdock/internal/img"
	"github.com/example/secretdock/internal/node"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/backup"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/internal/tg"
)

// ---- merchants ----

func (s *Server) handleMerchants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		list, err := s.DB.ListMerchants()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		if list == nil {
			list = []db.Merchant{}
		}
		ok(w, list)
	case "POST":
		var req struct {
			Name    string   `json:"name"`
			Aliases []string `json:"aliases"`
			LogoURL string   `json:"logoUrl"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "name required")
			return
		}
		id, err := s.DB.CreateMerchant(strings.TrimSpace(req.Name), req.Aliases, req.LogoURL, "")
		if err != nil {
			fail(w, 400, "merchant exists?")
			return
		}
		if req.LogoURL == "" {
			mid := id
			name := strings.TrimSpace(req.Name)
			go func() {
				// auto logo, best effort
				for _, c := range tg.LogoCandidates(name) {
					if logoURLWorks(c.URL) {
						if m, err := s.DB.GetMerchant(mid); err == nil {
							_ = s.DB.UpdateMerchant(mid, m.Name, m.Aliases, c.URL, "")
						}
						return
					}
				}
			}()
		}
		m, _ := s.DB.GetMerchant(id)
		ok(w, m)
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleMerchantOne(w http.ResponseWriter, r *http.Request) {
	// /api/merchants/{id}  or  /api/merchants/{id}/logo
	rest := strings.TrimPrefix(r.URL.Path, "/api/merchants/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		fail(w, 400, "bad id")
		return
	}
	if len(parts) == 2 && parts[1] == "logo" {
		if r.Method != "POST" {
			fail(w, 405, "method not allowed")
			return
		}
		s.handleLogoUpload(w, r, id)
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name    string   `json:"name"`
			Aliases []string `json:"aliases"`
			LogoURL string   `json:"logoUrl"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "name required")
			return
		}
		m, err := s.DB.GetMerchant(id)
		if err != nil {
			fail(w, 404, "not found")
			return
		}
		if err := s.DB.UpdateMerchant(id, strings.TrimSpace(req.Name), req.Aliases, req.LogoURL, m.LogoSVG); err != nil {
			fail(w, 400, err.Error())
			return
		}
		m, _ = s.DB.GetMerchant(id)
		ok(w, m)
	case "DELETE":
		if err := s.DB.DeleteMerchant(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

func logoURLWorks(url string) bool {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: 8 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (s *Server) handleLogoCandidates(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	cands := tg.LogoCandidates(name)
	if cands == nil {
		cands = []tg.LogoCandidate{}
	}
	ok(w, cands)
}

func (s *Server) handleLogoUpload(w http.ResponseWriter, r *http.Request, merchantID int64) {
	if _, err := s.DB.GetMerchant(merchantID); err != nil {
		fail(w, 404, "not found")
		return
	}
	f, hdr, err := r.FormFile("logo")
	if err != nil {
		fail(w, 400, "logo file required")
		return
	}
	defer f.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".svg" && ext != ".webp" {
		fail(w, 400, "png/jpg/svg/webp only")
		return
	}
	dir := filepath.Join(s.DataDir, "logos")
	_ = os.MkdirAll(dir, 0o700)
	data, err := io.ReadAll(io.LimitReader(f, 2<<20))
	if err != nil || len(data) == 0 {
		fail(w, 400, "bad file")
		return
	}
	fname := strconv.FormatInt(merchantID, 10) + ext
	if err := os.WriteFile(filepath.Join(dir, fname), data, 0o600); err != nil {
		fail(w, 500, err.Error())
		return
	}
	m, _ := s.DB.GetMerchant(merchantID)
	_ = s.DB.UpdateMerchant(merchantID, m.Name, m.Aliases, "/logos/"+fname, m.LogoSVG)
	ok(w, map[string]string{"logoUrl": "/logos/" + fname})
}

func (s *Server) handleLogoFile(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(s.DataDir, strings.TrimPrefix(r.URL.Path, "/"))
	if !strings.HasPrefix(p, filepath.Join(s.DataDir, "logos")) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}

// ---- secrets ----

func (s *Server) handleSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		mid, _ := strconv.ParseInt(r.URL.Query().Get("merchantId"), 10, 64)
		list, err := s.DB.ListSecrets(mid)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, s.decryptSecrets(list))
	case "POST":
		var req struct {
			MerchantID int64  `json:"merchantId"`
			Kind       string `json:"kind"`
			Label      string `json:"label"`
			Username   string `json:"username"`
			Value      string `json:"value"`
			Note       string `json:"note"`
			ExpiresAt  int64  `json:"expiresAt"`
		}
		if err := readJSON(r, &req); err != nil || req.MerchantID == 0 || req.Value == "" {
			fail(w, 400, "merchantId and value required")
			return
		}
		if req.Kind != "account" {
			req.Kind = "secret"
		}
		enc, err := crypto.Encrypt(s.EncKey, []byte(req.Value))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, err := s.DB.CreateSecret(db.Secret{
			MerchantID: req.MerchantID, Kind: req.Kind, Label: req.Label,
			Username: req.Username, Value: enc, Note: req.Note, ExpiresAt: req.ExpiresAt,
		})
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		if req.Label != "" {
			_ = s.DB.AddCustomLabel(req.Label)
		}
		sec, _ := s.DB.GetSecret(id)
		out := s.decryptSecrets([]db.Secret{sec})
		ok(w, out[0])
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleSecretOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/secrets/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		sec, err := s.DB.GetSecret(id)
		if err != nil {
			fail(w, 404, "not found")
			return
		}
		var req struct {
			Kind      string `json:"kind"`
			Label     string `json:"label"`
			Username  string `json:"username"`
			Value     string `json:"value"` // empty = keep
			Note      string `json:"note"`
			ExpiresAt int64  `json:"expiresAt"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if req.Kind == "account" || req.Kind == "secret" {
			sec.Kind = req.Kind
		}
		sec.Label, sec.Username, sec.Note, sec.ExpiresAt = req.Label, req.Username, req.Note, req.ExpiresAt
		if req.Value != "" {
			enc, err := crypto.Encrypt(s.EncKey, []byte(req.Value))
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			sec.Value = enc
		}
		if err := s.DB.UpdateSecret(sec); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if req.Label != "" {
			_ = s.DB.AddCustomLabel(req.Label)
		}
		upd, _ := s.DB.GetSecret(id)
		out := s.decryptSecrets([]db.Secret{upd})
		ok(w, out[0])
	case "DELETE":
		if err := s.DB.DeleteSecret(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		ok(w, []interface{}{})
		return
	}
	list, err := s.DB.SearchSecrets(q)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, s.decryptSecrets(list))
}

func (s *Server) handleExpiring(w http.ResponseWriter, r *http.Request) {
	days := 7
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 {
		days = v
	}
	list, err := s.DB.ExpiringSecrets(days)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, s.decryptSecrets(list))
}

func (s *Server) handleLabels(w http.ResponseWriter, r *http.Request) {
	list, _ := s.DB.CustomLabels()
	if list == nil {
		list = []string{}
	}
	ok(w, list)
}

// ---- settings ----

var settingKeys = []string{
	"bot_token_enc", "admin_chat_id", "query_pwd_hash",
	"auto_delete_minutes", "query_pwd_max_tries", "tg_idle_wipe_minutes", "reminder_default_times",
	"backup_enabled", "backup_time", "backup_keep", "backup_dir",
	"backup_include_images", "backup_include_files",
	// login security
	"sec_enabled", "sec_countries", "sec_ip_whitelist", "sec_ip_blacklist",
	"sec_totp_enabled", "sec_tg_alert",
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		out := map[string]string{}
		for _, k := range settingKeys {
			v := s.DB.GetSetting(k)
			if k == "bot_token_enc" {
				out["botTokenSet"] = boolStr(v != "")
				continue
			}
			if k == "query_pwd_hash" {
				out["queryPwdSet"] = boolStr(v != "")
				continue
			}
			out[k] = v
		}
		if out["auto_delete_minutes"] == "" {
			out["auto_delete_minutes"] = "3"
		}
		if out["query_pwd_max_tries"] == "" {
			out["query_pwd_max_tries"] = "3"
		}
		if out["tg_idle_wipe_minutes"] == "" {
			out["tg_idle_wipe_minutes"] = "5"
		}
		if out["reminder_default_times"] == "" {
			out["reminder_default_times"] = "3"
		}
		if v := s.DB.GetSetting("tg_banned_until"); v != "" && v != "0" {
			out["tgBanUntil"] = v
			out["tgBanCount"] = s.DB.GetSetting("tg_ban_count")
		}
		// login security: computed fields (never expose secrets)
		out["totpEnabled"] = boolStr(s.totpEnabled())
		out["geoipLoaded"] = boolStr(s.Geo != nil && s.Geo.Loaded())
		out["trustedCount"] = fmt.Sprintf("%d", len(s.trustedIPs()))
		ok(w, out)
	case "PUT":
		var req map[string]string
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if tok, ok := req["botToken"]; ok {
			if tok == "" {
				_ = s.DB.SetSetting("bot_token_enc", "")
			} else {
				enc, err := crypto.Encrypt(s.EncKey, []byte(tok))
				if err != nil {
					fail(w, 500, err.Error())
					return
				}
				_ = s.DB.SetSetting("bot_token_enc", enc)
			}
		}
		for _, k := range []string{"admin_chat_id", "auto_delete_minutes", "query_pwd_max_tries", "tg_idle_wipe_minutes", "reminder_default_times", "backup_enabled", "backup_time", "backup_keep", "backup_dir"} {
			if v, ok := req[k]; ok {
				_ = s.DB.SetSetting(k, v)
			}
		}
		// ---- login security ----
		if err := s.saveSecuritySettings(req, w, r); err != nil {
			return // response already written
		}
		if s.TG != nil {
			s.TG.Reload()
		}
		s.DB.Audit("settings", "updated", clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

var (
	tgTestMu   sync.Mutex
	tgTestLast time.Time
)

// handleTgTest verifies the bot token (getMe) and sends a test message
// to the admin chat. Body may carry botToken/adminChatId to test
// unsaved values.
func (s *Server) handleTgTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		BotToken    string `json:"botToken"`
		AdminChatID string `json:"adminChatId"`
	}
	_ = readJSON(r, &req)
	token := req.BotToken
	chatStr := req.AdminChatID
	if token == "" {
		if enc := s.DB.GetSetting("bot_token_enc"); enc != "" {
			if raw, err := crypto.Decrypt(s.EncKey, enc); err == nil {
				token = string(raw)
			}
		}
	}
	if chatStr == "" {
		chatStr = s.DB.GetSetting("admin_chat_id")
	}
	chatID, _ := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if token == "" || chatID == 0 {
		fail(w, 400, "请先填写 Bot Token 和管理员 Chat ID")
		return
	}
	tgTestMu.Lock()
	if time.Since(tgTestLast) < 10*time.Second {
		tgTestMu.Unlock()
		fail(w, 429, "测试太频繁，请 10 秒后再试")
		return
	}
	tgTestLast = time.Now()
	tgTestMu.Unlock()
	c := tg.NewClient(token)
	name, err := c.GetMe()
	if err != nil {
		fail(w, 400, "Token 无效："+err.Error())
		return
	}
	if _, err := c.SendMessage(chatID, "✅ SecretDock 机器人测试成功（@"+name+"）", "", nil); err != nil {
		fail(w, 400, "Token 有效，但发消息失败（Chat ID 是否正确 / 是否已 /start 机器人）："+err.Error())
		return
	}
	ok(w, map[string]string{"bot": name})
}

// handleTgUnban clears the TG query ban.
func (s *Server) handleTgUnban(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	_ = s.DB.SetSetting("tg_banned_until", "0")
	_ = s.DB.SetSetting("tg_ban_count", "0")
	s.DB.Audit("settings", "tg unban", clientIP(r))
	ok(w, map[string]bool{"ok": true})
}

// ---- userscripts ----

func scriptIsUserscript(contentEnc string, key []byte) bool {
	raw, err := crypto.Decrypt(key, contentEnc)
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), "==UserScript==")
}

func scriptJSON(sc *db.Script, content string) map[string]interface{} {
	return map[string]interface{}{
		"id": sc.ID, "name": sc.Name,
		"version": sc.Version, "description": sc.Description, "author": sc.Author,
		"content": content, "createdAt": sc.CreatedAt, "updatedAt": sc.UpdatedAt, "isPinned": sc.IsPinned, "categoryId": sc.CategoryID,
	}
}

// handleScripts: GET /api/scripts — list all (no content).
func (s *Server) handleScripts(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
	var list []db.Script
	var err error
	if catID == 0 {
		list, err = s.DB.ListAllScripts()
	} else {
		list, err = s.DB.ListScriptsByCategory(catID)
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, sc := range list {
		scc := sc
		j := scriptJSON(&scc, "")
		j["isUserscript"] = scriptIsUserscript(sc.ContentEnc, s.EncKey)
		out = append(out, j)
	}
	ok(w, out)
}

// handleScriptOne: GET (with decrypted content) / DELETE /api/scripts/{id}.
func (s *Server) handleScriptOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/scripts/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	sc, err := s.DB.GetScript(id)
	if err != nil {
		fail(w, 404, "not found")
		return
	}
	switch r.Method {
	case "GET":
		raw, err := crypto.Decrypt(s.EncKey, sc.ContentEnc)
		if err != nil {
			fail(w, 500, "decrypt failed")
			return
		}
		ok(w, scriptJSON(sc, string(raw)))
	case "PUT":
		var req struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Author      string `json:"author"`
			Content     string `json:"content"`
			CategoryID  *int64 `json:"categoryId"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if req.CategoryID != nil {
			if err := s.DB.MoveScript(id, *req.CategoryID); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		enc := sc.ContentEnc
		if req.Content != "" {
			var err error
			enc, err = crypto.Encrypt(s.EncKey, []byte(req.Content))
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		name := sc.Name
		if req.Name != "" {
			name = req.Name
		}
		if err := s.DB.UpdateScript(id, name, req.Version, req.Description, req.Author, enc); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("script_update", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteScript(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("script_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// ---- links ----

func linkCatJSON(c *db.LinkCategory) map[string]interface{} {
	return map[string]interface{}{"id": c.ID, "name": c.Name, "sort": c.Sort, "createdAt": c.CreatedAt}
}

func linkJSON(l *db.Link) map[string]interface{} {
	return map[string]interface{}{
		"id": l.ID, "categoryId": l.CategoryID, "name": l.Name, "url": l.URL,
		"createdAt": l.CreatedAt, "updatedAt": l.UpdatedAt, "isPinned": l.IsPinned, "autoName": l.AutoName,
	}
}

// handleLinkCategories: GET list / POST {name}.
func (s *Server) handleLinkCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		list, err := s.DB.ListLinkCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, c := range list {
			out = append(out, linkCatJSON(c))
		}
		ok(w, out)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.CreateLinkCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("link_category_create", req.Name, clientIP(r))
		ok(w, map[string]interface{}{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleLinkCategoryOne: PUT {name} / DELETE /api/link-categories/{id}.
func (s *Server) handleLinkCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/link-categories/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameLinkCategory(id, req.Name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("link_category_rename", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteLinkCategory(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("link_category_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleLinks: GET ?categoryId= / POST {categoryId,name,url}.
func (s *Server) handleLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		var list []*db.Link
		var err error
		if catID == 0 {
			list, err = s.DB.ListAllLinks()
		} else {
			list, err = s.DB.ListLinks(catID)
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, l := range list {
			out = append(out, linkJSON(l))
		}
		ok(w, out)
	case "POST":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Name       string `json:"name"`
			URL        string `json:"url"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.URL) == "" {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.CreateLink(req.CategoryID, req.Name, req.URL, 0)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("link_create", req.URL, clientIP(r))
		ok(w, map[string]interface{}{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleLinkOne: PUT {categoryId,name,url} / DELETE /api/links/{id}.
func (s *Server) handleLinkOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/links/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Name       string `json:"name"`
			URL        string `json:"url"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.UpdateLink(id, req.CategoryID, req.Name, req.URL); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("link_update", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteLink(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("link_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleLinksMove: POST {ids:[], categoryId}.
func (s *Server) handleLinksMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		IDs        []int64 `json:"ids"`
		CategoryID int64   `json:"categoryId"`
	}
	if err := readJSON(r, &req); err != nil || len(req.IDs) == 0 {
		fail(w, 400, "bad request")
		return
	}
	if err := s.DB.MoveLinks(req.IDs, req.CategoryID); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.DB.Audit("links_move", fmt.Sprintf("n=%d", len(req.IDs)), clientIP(r))
	ok(w, map[string]bool{"ok": true})
}

// ---- notes ----

func noteJSON(n *db.Note, content string) map[string]interface{} {
	return map[string]interface{}{
		"id": n.ID, "title": n.Title, "content": content,
		"createdAt": n.CreatedAt, "updatedAt": n.UpdatedAt, "isPinned": n.IsPinned, "categoryId": n.CategoryID,
	}
}

// handleNotes: GET list (no content) / POST {title, content}.
func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		list, err := s.DB.ListNotesByCategory(catID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, n := range list {
			out = append(out, noteJSON(n, ""))
		}
		ok(w, out)
	case "POST":
		var req struct {
			Title      string `json:"title"`
			Content    string `json:"content"`
			CategoryID int64  `json:"categoryId"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		enc, err := crypto.Encrypt(s.EncKey, []byte(req.Content))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		catID := int64(0)
		if req.CategoryID != 0 {
			catID = req.CategoryID
		} else {
			catID, _ = s.DB.FindOrCreateNoteCategory("")
		}
		id, err := s.DB.CreateNote(req.Title, enc, catID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("note_create", req.Title, clientIP(r))
		ok(w, map[string]interface{}{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleNoteOne: GET (with content) / PUT {title, content} / DELETE /api/notes/{id}.
func (s *Server) handleNoteOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/notes/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	n, err := s.DB.GetNote(id)
	if err != nil {
		fail(w, 404, "not found")
		return
	}
	switch r.Method {
	case "GET":
		raw, err := crypto.Decrypt(s.EncKey, n.ContentEnc)
		if err != nil {
			fail(w, 500, "decrypt failed")
			return
		}
		ok(w, noteJSON(n, string(raw)))
	case "PUT":
		var req struct {
			Title      string `json:"title"`
			Content    string `json:"content"`
			CategoryID *int64 `json:"categoryId"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if req.CategoryID != nil {
			if err := s.DB.MoveNote(id, *req.CategoryID); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		enc, err := crypto.Encrypt(s.EncKey, []byte(req.Content))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		title := n.Title
		if req.Title != "" {
			title = req.Title
		}
		if err := s.DB.UpdateNote(id, title, enc); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("note_update", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteNote(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("note_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// ---- images ----

func imgCatJSON(c *db.ImageCategory) map[string]interface{} {
	return map[string]interface{}{"id": c.ID, "name": c.Name, "sort": c.Sort, "createdAt": c.CreatedAt}
}

func imgJSON(im *db.Image) map[string]interface{} {
	return map[string]interface{}{
		"id": im.ID, "categoryId": im.CategoryID, "caption": im.Caption,
		"fileSize": im.FileSize, "createdAt": im.CreatedAt,
		"origName": im.OrigName,
	}
}

func (s *Server) handleImageCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		list, err := s.DB.ListImageCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, c := range list {
			out = append(out, imgCatJSON(c))
		}
		ok(w, out)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.CreateImageCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("image_category_create", req.Name, clientIP(r))
		ok(w, map[string]interface{}{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleImageCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/image-categories/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameImageCategory(id, req.Name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("image_category_rename", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		// delete files first
		if list, err := s.DB.ListImages(id); err == nil {
			for _, im := range list {
				for _, nm := range []string{im.FilePath, im.ThumbPath, im.ThumbS, im.ThumbM, im.ThumbL} {
					if nm != "" && !strings.Contains(nm, "/") {
						os.Remove(filepath.Join(s.DataDir, "images", nm))
					}
				}
			}
		}
		if err := s.DB.DeleteImageCategory(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("image_category_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// imageFilePath resolves a stored image filename, guarding against traversal.
func (s *Server) imageFilePath(im *db.Image, thumb bool) (string, bool) {
	name := im.FilePath
	if thumb {
		name = im.ThumbPath
	}
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", false
	}
	return filepath.Join(s.DataDir, "images", name), true
}

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		var list []*db.Image
		var err error
		if catID == 0 {
			list, err = s.DB.ListAllImages()
		} else {
			list, err = s.DB.ListImages(catID)
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, im := range list {
			out = append(out, imgJSON(im))
		}
		ok(w, out)
	case "POST":
		// multipart upload from web: file, categoryId, caption
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			fail(w, 400, "bad upload")
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			fail(w, 400, "no file")
			return
		}
		defer f.Close()
		catID, _ := strconv.ParseInt(r.FormValue("categoryId"), 10, 64)
		caption := r.FormValue("caption")
		dir := filepath.Join(s.DataDir, "images")
		if err := os.MkdirAll(dir, 0755); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ext := strings.ToLower(filepath.Ext(hdr.Filename))
		if ext == "" {
			ext = ".jpg"
		}
		data, err := io.ReadAll(f)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		base := fmt.Sprintf("%d", time.Now().UnixNano())
		name := base + "_orig" + ext
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			fail(w, 500, err.Error())
			return
		}
		thumbS, thumbM, thumbL, err := img.MakeThumbs(data, dir, base)
		if err != nil {
			os.Remove(filepath.Join(dir, name))
			fail(w, 400, "unsupported image")
			return
		}
		id, err := s.DB.CreateImage(catID, caption, hdr.Filename, name, thumbS, thumbM, thumbL, int64(len(data)))
		if err != nil {
			os.Remove(filepath.Join(dir, name))
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("image_upload", hdr.Filename, clientIP(r))
		ok(w, map[string]interface{}{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleImageOne: GET ?thumb=1 file / PUT {categoryId,caption} / DELETE /api/images/{id}.
func (s *Server) handleImageOne(w http.ResponseWriter, r *http.Request) {
	id, okID := idFromPath(r.URL.Path, "/api/images/")
	if !okID {
		fail(w, 400, "bad id")
		return
	}
	im, err := s.DB.GetImage(id)
	if err != nil {
		fail(w, 404, "not found")
		return
	}
	switch r.Method {
	case "GET":
		size := r.URL.Query().Get("size")
		var name string
		switch size {
		case "s":
			name = im.ThumbS
		case "m":
			name = im.ThumbM
		case "l":
			name = im.ThumbL
		case "orig":
			name = im.FilePath
		default:
			name = im.ThumbM
		}
		// 老记录没有服务端缩略图时，回退到 TG 小图
		if name == "" {
			name = im.ThumbPath
		}
		if name == "" {
			name = im.FilePath
		}
		if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
			fail(w, 404, "not found")
			return
		}
		http.ServeFile(w, r, filepath.Join(s.DataDir, "images", name))
	case "PUT":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Caption    string `json:"caption"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.UpdateImage(id, req.CategoryID, req.Caption); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("image_update", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		for _, nm := range []string{im.FilePath, im.ThumbPath, im.ThumbS, im.ThumbM, im.ThumbL} {
			if nm != "" && !strings.Contains(nm, "/") && !strings.Contains(nm, "\\") {
				os.Remove(filepath.Join(s.DataDir, "images", nm))
			}
		}
		if err := s.DB.DeleteImage(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("image_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleImagesMove: POST {ids:[], categoryId}.
func (s *Server) handleImagesMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		IDs        []int64 `json:"ids"`
		CategoryID int64   `json:"categoryId"`
	}
	if err := readJSON(r, &req); err != nil || len(req.IDs) == 0 {
		fail(w, 400, "bad request")
		return
	}
	if err := s.DB.MoveImages(req.IDs, req.CategoryID); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.DB.Audit("images_move", fmt.Sprintf("n=%d", len(req.IDs)), clientIP(r))
	ok(w, map[string]bool{"ok": true})
}

// handlePin: POST {type: secrets|scripts|links|notes, id, pinned}.
func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Type   string `json:"type"`
		ID     int64  `json:"id"`
		Pinned bool   `json:"pinned"`
	}
	if err := readJSON(r, &req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	if err := s.DB.SetPinned(req.Type, req.ID, req.Pinned); err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.DB.Audit("pin", fmt.Sprintf("%s id=%d pinned=%v", req.Type, req.ID, req.Pinned), clientIP(r))
	ok(w, map[string]bool{"ok": true})
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return ""
}

func (s *Server) handleQueryPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil || req.Password == "" {
		fail(w, 400, "password required")
		return
	}
	norm := auth.NormalizeSecret(req.Password)
	hash, err := auth.HashSecret(norm)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	_ = s.DB.SetSetting("query_pwd_hash", hash)
	s.DB.Audit("settings", "query password set", clientIP(r))
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	list, err := s.DB.AuditList(limit)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if list == nil {
		list = []db.AuditEntry{}
	}
	ok(w, list)
}

// ---- backup ----

func (s *Server) handleDataClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := readJSON(r, &req); err != nil || req.Key == "" {
		fail(w, 400, "key required")
		return
	}
	if err := s.DB.ClearData(req.Key); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.DB.Audit("data_clear", req.Key, clientIP(r))
	ok(w, map[string]interface{}{"ok": true})
}

// 创建分享链接
func (s *Server) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Kind string `json:"kind"` // image|file
		ID   int64  `json:"id"`
	}
	if err := readJSON(r, &req); err != nil || (req.Kind != "image" && req.Kind != "file") {
		fail(w, 400, "bad request")
		return
	}
	token, err := s.DB.CreateShareToken(req.Kind, req.ID)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]interface{}{"token": token, "url": "/s/" + token})
}

func (s *Server) serveImage(w http.ResponseWriter, r *http.Request, id int64) {
	im, err := s.DB.GetImage(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := im.FilePath
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.DataDir, "images", name))
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, id int64) {
	f, err := s.DB.GetFile(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := f.FilePath
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, filepath.Join(s.DataDir, "files", name))
}

// 公开分享访问（无需登录）
func (s *Server) handleShareView(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/s/"):]
	kind, refID, err := s.DB.GetShareToken(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if kind == "image" {
		// 复用图片 serving 逻辑（不鉴权）
		s.serveImage(w, r, refID)
	} else if kind == "file" {
		s.serveFile(w, r, refID)
	} else {
		http.NotFound(w, r)
	}
}

func (s *Server) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		Password      string `json:"password"`
		IncludeImages bool   `json:"includeImages"`
		IncludeFiles  bool   `json:"includeFiles"`
	}
	if err := readJSON(r, &req); err != nil || req.Password == "" {
		fail(w, 400, "password required")
		return
	}
	raw, err := backup.ExportWithPassword(s.DB, s.EncKey, req.Password, backup.BackupOptions{
		IncludeImages: req.IncludeImages,
		IncludeFiles:  req.IncludeFiles,
		DataDir:       s.DataDir,
	})
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.DB.Audit("backup_export", "", clientIP(r))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="secretdock-backup.enc.json"`)
	_, _ = w.Write(raw)
}

func (s *Server) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		fail(w, 400, "bad upload")
		return
	}
	password := r.FormValue("password")
	includeImages := r.FormValue("includeImages") == "1"
	includeFiles := r.FormValue("includeFiles") == "1"
	f, _, err := r.FormFile("file")
	if err != nil || password == "" {
		fail(w, 400, "file and password required")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 512<<20))
	if err != nil {
		fail(w, 400, "bad file")
		return
	}
	n, err := backup.ImportWithPassword(s.DB, s.EncKey, raw, password, backup.BackupOptions{
		IncludeImages: includeImages,
		IncludeFiles:  includeFiles,
		DataDir:       s.DataDir,
	})
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.DB.Audit("backup_import", strconv.Itoa(n)+" items", clientIP(r))
	ok(w, map[string]interface{}{"ok": true, "imported": n})
}

// ---- files ----

func fileJSON(f *db.File) map[string]interface{} {
	return map[string]interface{}{
		"id": f.ID, "categoryId": f.CategoryID, "name": f.OrigName,
		"fileSize": f.FileSize, "mime": f.Mime, "isText": f.IsText,
		"createdAt": f.CreatedAt,
	}
}

func (s *Server) handleFileCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cats, err := s.DB.ListFileCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(cats))
		for _, c := range cats {
			out = append(out, map[string]interface{}{"id": c.ID, "name": c.Name, "sort": c.Sort, "createdAt": c.CreatedAt})
		}
		ok(w, out)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.CreateFileCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("filecat_create", req.Name, clientIP(r))
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleFileCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, idOK := idFromPath(r.URL.Path, "/api/file-categories/")
	if !idOK {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameFileCategory(id, req.Name); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		// 先删文件
		if list, err := s.DB.ListFiles(id); err == nil {
			for _, f := range list {
				if f.FilePath != "" && !strings.Contains(f.FilePath, "/") {
					os.Remove(filepath.Join(s.DataDir, "files", f.FilePath))
				}
			}
		}
		if err := s.DB.DeleteFileCategory(id); err != nil {
			fail(w, 400, err.Error())
			return
		}
		s.DB.Audit("filecat_delete", fmt.Sprintf("id=%d", id), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		var list []db.File
		var err error
		if catID == 0 {
			list, err = s.DB.ListAllFiles()
		} else {
			list, err = s.DB.ListFiles(catID)
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, f := range list {
			ff := f
			out = append(out, fileJSON(&ff))
		}
		ok(w, out)
	case "POST":
		// multipart upload: file, categoryId
		if err := r.ParseMultipartForm(200 << 20); err != nil {
			fail(w, 400, err.Error())
			return
		}
		fh, hdr, err := r.FormFile("file")
		if err != nil {
			fail(w, 400, "no file")
			return
		}
		defer fh.Close()
		catID, _ := strconv.ParseInt(r.FormValue("categoryId"), 10, 64)
		if catID == 0 {
			catID, _ = s.DB.FindOrCreateFileCategory("未分类")
		}
		data, err := io.ReadAll(io.LimitReader(fh, 200<<20))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		dir := filepath.Join(s.DataDir, "files")
		os.MkdirAll(dir, 0755)
		ext := strings.ToLower(filepath.Ext(hdr.Filename))
		name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			fail(w, 500, err.Error())
			return
		}
		isText := sniffTextLocal(data)
		origName := hdr.Filename
		if origName == "" {
			origName = name
		}
		id, err := s.DB.CreateFile(catID, name, origName, name, hdr.Header.Get("Content-Type"), isText, int64(len(data)))
		if err != nil {
			os.Remove(filepath.Join(dir, name))
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("file_upload", origName, clientIP(r))
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleFileOne(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/files/")
	parts := strings.SplitN(rest, "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		fail(w, 400, "bad id")
		return
	}
	f, err := s.DB.GetFile(id)
	if err != nil {
		fail(w, 404, "not found")
		return
	}
	safePath := func() (string, bool) {
		if f.FilePath == "" || strings.Contains(f.FilePath, "/") || strings.Contains(f.FilePath, "\\") {
			return "", false
		}
		return filepath.Join(s.DataDir, "files", f.FilePath), true
	}
	// /api/files/:id/text
	if len(parts) == 2 && parts[1] == "text" {
		p, pathOK := safePath()
		if !pathOK {
			fail(w, 404, "not found")
			return
		}
		switch r.Method {
		case "GET":
			data, err := os.ReadFile(p)
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			if int64(len(data)) > 2<<20 {
				fail(w, 400, "file too large to preview")
				return
			}
			ok(w, map[string]string{"content": string(data)})
		case "PUT":
			var req struct {
				Content string `json:"content"`
			}
			if err := readJSON(r, &req); err != nil {
				fail(w, 400, "bad request")
				return
			}
			if err := os.WriteFile(p, []byte(req.Content), 0644); err != nil {
				fail(w, 500, err.Error())
				return
			}
			s.DB.Audit("file_edit", f.OrigName, clientIP(r))
			ok(w, map[string]bool{"ok": true})
		default:
			fail(w, 405, "method not allowed")
		}
		return
	}
	switch r.Method {
	case "GET":
		p, pathOK := safePath()
		if !pathOK {
			fail(w, 404, "not found")
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+f.OrigName+"\"")
		http.ServeFile(w, r, p)
	case "PUT":
		var req struct {
			Name       *string `json:"name"`
			CategoryID *int64  `json:"categoryId"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if req.Name != nil {
			if err := s.DB.RenameFile(id, *req.Name); err != nil {
				fail(w, 400, err.Error())
				return
			}
		}
		if req.CategoryID != nil {
			if err := s.DB.MoveFile(id, *req.CategoryID); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if p, pathOK := safePath(); pathOK {
			os.Remove(p)
		}
		if err := s.DB.DeleteFile(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("file_delete", f.OrigName, clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// sniffTextLocal 与 tg.sniffText 同逻辑（handlers 包内复用）
func sniffTextLocal(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	chunk := data[:n]
	for _, b := range chunk {
		if b == 0 {
			return false
		}
	}
	return true
}

// ---- script categories ----
func (s *Server) handleScriptCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cats, err := s.DB.ListScriptCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(cats))
		for _, c := range cats {
			out = append(out, map[string]interface{}{"id": c.ID, "name": c.Name, "sort": c.Sort, "createdAt": c.CreatedAt})
		}
		ok(w, out)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.FindOrCreateScriptCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleScriptCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, idOK := idFromPath(r.URL.Path, "/api/script-categories/")
	if !idOK {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameScriptCategory(id, req.Name); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteScriptCategory(id); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// ---- note categories ----
func (s *Server) handleNoteCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cats, err := s.DB.ListNoteCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(cats))
		for _, c := range cats {
			out = append(out, map[string]interface{}{"id": c.ID, "name": c.Name, "sort": c.Sort, "createdAt": c.CreatedAt})
		}
		ok(w, out)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.FindOrCreateNoteCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleNoteCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, idOK := idFromPath(r.URL.Path, "/api/note-categories/")
	if !idOK {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameNoteCategory(id, req.Name); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteNoteCategory(id); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// ---- reminders ----

func reminderJSON(r db.Reminder, catName string) map[string]interface{} {
	return map[string]interface{}{
		"id": r.ID, "categoryId": r.CategoryID, "categoryName": catName,
		"title": r.Title, "note": r.Note, "remindAt": r.RemindAt,
		"repeatType": r.RepeatType, "repeatDetail": r.RepeatDetail,
		"times": r.Times, "isActive": r.IsActive, "createdAt": r.CreatedAt,
	}
}

func (s *Server) handleReminders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		list, err := s.DB.ListReminders(catID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		cats, _ := s.DB.ListReminderCategories()
		catMap := map[int64]string{}
		for _, c := range cats {
			catMap[c.ID] = c.Name
		}
		out := make([]map[string]interface{}, 0, len(list))
		for _, rem := range list {
			out = append(out, reminderJSON(rem, catMap[rem.CategoryID]))
		}
		ok(w, out)
	case "POST":
		var req struct {
			Title        string `json:"title"`
			Note         string `json:"note"`
			CategoryID   int64  `json:"categoryId"`
			RemindAt     int64  `json:"remindAt"`
			RepeatType   string `json:"repeatType"`
			RepeatDetail string `json:"repeatDetail"`
			Times        int    `json:"times"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Title) == "" {
			fail(w, 400, "bad request")
			return
		}
		if req.CategoryID == 0 {
			req.CategoryID, _ = s.DB.FindOrCreateReminderCategory("")
		}
		if req.RepeatType == "" {
			req.RepeatType = "none"
		}
		if req.RemindAt <= 0 {
			fail(w, 400, "remindAt required")
			return
		}
		times := req.Times
		if times < 1 || times > 10 {
			if v := s.DB.GetSetting("reminder_default_times"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 10 {
					times = n
				} else {
					times = 3
				}
			} else {
				times = 3
			}
		}
		id, err := s.DB.CreateReminder(db.Reminder{
			CategoryID: req.CategoryID, Title: strings.TrimSpace(req.Title), Note: req.Note,
			RemindAt: req.RemindAt, RepeatType: req.RepeatType, RepeatDetail: req.RepeatDetail,
			Times: times, IsActive: true,
		})
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("reminder_create", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleReminderOne(w http.ResponseWriter, r *http.Request) {
	id, idOK := idFromPath(r.URL.Path, "/api/reminders/")
	if !idOK {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "GET":
		rem, err := s.DB.GetReminder(id)
		if err != nil {
			fail(w, 404, "not found")
			return
		}
		ok(w, reminderJSON(rem, ""))
	case "PUT":
		rem, err := s.DB.GetReminder(id)
		if err != nil {
			fail(w, 404, "not found")
			return
		}
		var req struct {
			Title        *string `json:"title"`
			Note         *string `json:"note"`
			CategoryID   *int64  `json:"categoryId"`
			RemindAt     *int64  `json:"remindAt"`
			RepeatType   *string `json:"repeatType"`
			RepeatDetail *string `json:"repeatDetail"`
			Times        *int    `json:"times"`
			IsActive     *bool   `json:"isActive"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if req.Title != nil {
			rem.Title = strings.TrimSpace(*req.Title)
		}
		if req.Note != nil {
			rem.Note = *req.Note
		}
		if req.CategoryID != nil {
			rem.CategoryID = *req.CategoryID
		}
		if req.RemindAt != nil {
			rem.RemindAt = *req.RemindAt
		}
		if req.RepeatType != nil {
			rem.RepeatType = *req.RepeatType
		}
		if req.RepeatDetail != nil {
			rem.RepeatDetail = *req.RepeatDetail
		}
		if req.Times != nil && *req.Times >= 1 && *req.Times <= 10 {
			rem.Times = *req.Times
		}
		if req.IsActive != nil {
			rem.IsActive = *req.IsActive
		}
		if err := s.DB.UpdateReminder(rem); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("reminder_update", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteReminder(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("reminder_delete", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleReminderCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cats, err := s.DB.ListReminderCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]interface{}, 0, len(cats))
		for _, c := range cats {
			out = append(out, map[string]interface{}{"id": c.ID, "name": c.Name, "sort": c.Sort, "createdAt": c.CreatedAt})
		}
		ok(w, out)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.FindOrCreateReminderCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleReminderCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, idOK := idFromPath(r.URL.Path, "/api/reminder-categories/")
	if !idOK {
		fail(w, 400, "bad id")
		return
	}
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameReminderCategory(id, req.Name); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteReminderCategory(id); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// ---- 订阅节点 ----

func (s *Server) handleNodeCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		list, err := s.DB.ListNodeCategories()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, list)
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			fail(w, 400, "bad request")
			return
		}
		id, err := s.DB.CreateNodeCategory(req.Name)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("node_category_create", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleNodeCategoryOne(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/node-categories/"), 10, 64)
	switch r.Method {
	case "PUT":
		var req struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.DB.RenameNodeCategory(id, req.Name); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteNodeCategory(id); err != nil {
			fail(w, 400, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		catID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		list, err := s.DB.ListNodes(catID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, list)
	case "POST":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Name       string `json:"name"`
			URL        string `json:"url"`
		}
		if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.URL) == "" {
			fail(w, 400, "bad request")
			return
		}
		if req.CategoryID == 0 {
			req.CategoryID, _ = s.DB.FindOrCreateNodeCategory("")
		}
		info := node.Parse(req.URL)
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = info.Name
		}
		if name == "" {
			name = "未命名"
		}
		id, err := s.DB.CreateNode(req.CategoryID, name, strings.TrimSpace(req.URL), info.Kind, info.Proto, info.Host, info.Port)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		s.DB.Audit("node_create", "id="+strconv.FormatInt(id, 10), clientIP(r))
		ok(w, map[string]int64{"id": id})
	default:
		fail(w, 405, "method not allowed")
	}
}

func (s *Server) handleNodeOne(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/nodes/"), 10, 64)
	switch r.Method {
	case "PUT":
		var req struct {
			CategoryID int64  `json:"categoryId"`
			Name       string `json:"name"`
		}
		if err := readJSON(r, &req); err != nil {
			fail(w, 400, "bad request")
			return
		}
		n, err := s.DB.GetNode(id)
		if err != nil {
			fail(w, 404, "not found")
			return
		}
		catID := req.CategoryID
		if catID == 0 {
			catID = n.CategoryID
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "未命名"
		}
		if err := s.DB.UpdateNode(id, catID, name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "DELETE":
		if err := s.DB.DeleteNode(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}

// handleNodesTest 测试单个或批量节点连通性
func (s *Server) handleNodesTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var req struct {
		IDs        []int64 `json:"ids"`
		CategoryID int64   `json:"categoryId"`
	}
	readJSON(r, &req)
	var nodes []db.Node
	if len(req.IDs) > 0 {
		for _, id := range req.IDs {
			if n, err := s.DB.GetNode(id); err == nil {
				nodes = append(nodes, n)
			}
		}
	} else {
		var err error
		nodes, err = s.DB.ListNodes(req.CategoryID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	// 并发测试，最多 10 个并发
	sem := make(chan struct{}, 10)
	type res struct {
		ID      int64  `json:"id"`
		OK      bool   `json:"ok"`
		Latency int64  `json:"latency"`
		Err     string `json:"err,omitempty"`
	}
	results := make([]res, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func(i int, n db.Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tr := node.Test(n.URL)
			status := "fail"
			if tr.OK {
				status = "ok"
			}
			s.DB.UpdateNodeCheck(n.ID, status, tr.Latency)
			results[i] = res{ID: n.ID, OK: tr.OK, Latency: tr.Latency, Err: tr.Err}
		}(i, n)
	}
	wg.Wait()
	ok(w, results)
}
