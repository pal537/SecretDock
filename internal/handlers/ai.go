package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"

	"github.com/example/secretdock/internal/ai"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/web"
)

// ---------------- providers catalog (shared static JSON) ----------------

type aiProviderPreset struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Format       string            `json:"format"`
	BaseURL      string            `json:"base_url"`
	ChatPath     string            `json:"chat_path"`
	ModelsPath   string            `json:"models_path"`
	NativeSearch bool              `json:"native_search"`
	Headers      map[string]string `json:"headers"`
	Balance      *struct {
		Method      string `json:"method"`
		URL         string `json:"url"`
		BalancePath string `json:"balance_path"`
	} `json:"balance"`
}

var aiPresetsCache []aiProviderPreset

func aiPresets() []aiProviderPreset {
	if aiPresetsCache != nil {
		return aiPresetsCache
	}
	b, err := web.FS.ReadFile("static/ai-providers.json")
	if err != nil {
		return nil
	}
	var ps []aiProviderPreset
	if json.Unmarshal(b, &ps) != nil {
		return nil
	}
	aiPresetsCache = ps
	return ps
}

func aiPresetByID(id string) *aiProviderPreset {
	for i, p := range aiPresets() {
		if p.ID == id {
			return &aiPresets()[i]
		}
	}
	return nil
}

// ---------------- config resolution ----------------

func (s *Server) resolveAIConfig(c *db.AIConfig) (ai.Config, error) {
	key := ""
	if c.KeyEnc != "" {
		b, err := crypto.Decrypt(s.EncKey, c.KeyEnc)
		if err != nil {
			return ai.Config{}, fmt.Errorf("API Key 解密失败")
		}
		key = string(b)
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return ai.Config{}, fmt.Errorf("未配置接口地址")
	}
	if key == "" {
		return ai.Config{}, fmt.Errorf("未配置 API Key")
	}
	model := c.DefaultModel
	return ai.Config{
		Format: c.Format, BaseURL: c.BaseURL, ChatPath: c.ChatPath,
		APIKey: key, Model: model,
		NativeSearch: c.NativeSearch, PriceIn: c.PriceIn, PriceOut: c.PriceOut,
	}, nil
}

func aiConfigView(c *db.AIConfig, keyLast4 string) map[string]interface{} {
	var models []string
	_ = json.Unmarshal([]byte(c.Models), &models)
	if models == nil {
		models = []string{}
	}
	kinds := map[string]string{}
	if c.ModelKinds != "" {
		_ = json.Unmarshal([]byte(c.ModelKinds), &kinds)
	}
	return map[string]interface{}{
		"id": c.ID, "name": c.Name, "providerId": c.ProviderID, "format": c.Format,
		"baseUrl": c.BaseURL, "chatPath": c.ChatPath, "models": models, "modelKinds": kinds,
		"defaultModel": c.DefaultModel, "nativeSearch": c.NativeSearch,
		"priceIn": c.PriceIn, "priceOut": c.PriceOut,
		"hasKey": c.KeyEnc != "", "keyLast4": keyLast4,
		"lastLatency": c.LastLatency, "lastStatus": c.LastStatus, "lastCheckAt": c.LastCheckAt,
		"balanceJson": c.BalanceJSON, "balanceAt": c.BalanceAt,
		"createdAt": c.CreatedAt,
	}
}

// ---- model kind classification: "chat" | "image" | "video" ----
// Decided automatically, never by hand: a provider's own typed model
// listing when available, otherwise a name dictionary.

var aiVideoNamePatterns = []string{"t2v", "i2v", "cogvideo", "kling", "hailuo", "seedance", "wan2", "text-to-video", "image-to-video", "ltx-video", "veo3", "veo-3", "sora", "pika", "gen-3", "gen-4"}

var aiImageNamePatterns = []string{"qwen-image", "z-image", "kolors", "dall-e", "dall_e", "flux", "stable-diffusion", "sdxl", "cogview", "seedream", "imagen", "gpt-image", "glm-image", "image-edit", "-image", "/image"}

func aiDictKind(model string) string {
	m := strings.ToLower(model)
	for _, p := range aiVideoNamePatterns {
		if strings.Contains(m, p) {
			return "video"
		}
	}
	for _, p := range aiImageNamePatterns {
		if strings.Contains(m, p) {
			return "image"
		}
	}
	return "chat"
}

func aiDictKindsJSON(models []string) string {
	kinds := map[string]string{}
	for _, m := range models {
		if k := aiDictKind(m); k != "chat" {
			kinds[m] = k
		}
	}
	b, _ := json.Marshal(kinds)
	return string(b)
}

func aiModelKind(c *db.AIConfig, model string) string {
	if c.ModelKinds != "" {
		kinds := map[string]string{}
		if json.Unmarshal([]byte(c.ModelKinds), &kinds) == nil {
			if k, ok := kinds[model]; ok && k != "" {
				return k
			}
		}
	}
	return aiDictKind(model)
}

// aiClassifyKinds classifies a freshly fetched model list. Providers
// like SiliconFlow answer ?type=image / ?type=video with just that
// subset — trusted only when the typed answer is a strict subset of
// the full list (a provider ignoring the parameter returns everything,
// which must not mark every model as media). The rest falls back to
// the name dictionary.
func (s *Server) aiClassifyKinds(ctx context.Context, c *db.AIConfig, cfg ai.Config, modelsPath string, headers map[string]string, ids []string) map[string]string {
	kinds := map[string]string{}
	full := map[string]bool{}
	for _, id := range ids {
		full[id] = true
	}
	if c.Format != "anthropic" {
		for _, t := range []string{"image", "video"} {
			sub, err := ai.FetchModels(ctx, cfg, modelsPath+"?type="+t, headers)
			if err != nil || len(sub) == 0 || len(sub) >= len(ids) {
				continue
			}
			for _, id := range sub {
				if full[id] {
					kinds[id] = t
				}
			}
		}
	}
	for _, id := range ids {
		if _, ok := kinds[id]; !ok {
			if k := aiDictKind(id); k != "chat" {
				kinds[id] = k
			}
		}
	}
	return kinds
}

func (s *Server) keyLast4(c *db.AIConfig) string {
	if c.KeyEnc == "" {
		return ""
	}
	b, err := crypto.Decrypt(s.EncKey, c.KeyEnc)
	if err != nil || len(b) < 4 {
		return ""
	}
	return string(b[len(b)-4:])
}

// ---------------- configs CRUD ----------------

func (s *Server) handleAIConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.DB.ListAIConfigs()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := []map[string]interface{}{}
		for _, c := range list {
			out = append(out, aiConfigView(c, s.keyLast4(c)))
		}
		ok(w, out)
	case http.MethodPost:
		var in struct {
			Name         string   `json:"name"`
			ProviderID   string   `json:"providerId"`
			Format       string   `json:"format"`
			BaseURL      string   `json:"baseUrl"`
			ChatPath     string   `json:"chatPath"`
			Key          string   `json:"key"`
			Models       []string `json:"models"`
			DefaultModel string   `json:"defaultModel"`
			NativeSearch bool     `json:"nativeSearch"`
			PriceIn      float64  `json:"priceIn"`
			PriceOut     float64  `json:"priceOut"`
		}
		if err := readJSON(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			fail(w, 400, "名称不能为空")
			return
		}
		if in.Format != "anthropic" {
			in.Format = "openai"
		}
		if in.ChatPath == "" {
			in.ChatPath = "/v1/chat/completions"
			if in.Format == "anthropic" {
				in.ChatPath = "/v1/messages"
			}
		}
		enc := ""
		if in.Key != "" {
			e, err := crypto.Encrypt(s.EncKey, []byte(in.Key))
			if err != nil {
				fail(w, 500, "加密失败")
				return
			}
			enc = e
		}
		mb, _ := json.Marshal(in.Models)
		c := &db.AIConfig{
			Name: in.Name, ProviderID: in.ProviderID, Format: in.Format,
			BaseURL: strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"),
			ChatPath: in.ChatPath, KeyEnc: enc, Models: string(mb),
			DefaultModel: in.DefaultModel, NativeSearch: in.NativeSearch,
			PriceIn: in.PriceIn, PriceOut: in.PriceOut,
		}
		id, err := s.DB.CreateAIConfig(c)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		c.ID = id
		c.ModelKinds = aiDictKindsJSON(in.Models)
		s.DB.UpdateAIConfigKinds(id, c.ModelKinds)
		ok(w, aiConfigView(c, s.keyLast4(c)))
	default:
		fail(w, 405, "method")
	}
}

func parseAIID(path, prefix string) (int64, string) {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	parts := strings.SplitN(rest, "/", 2)
	id, _ := strconv.ParseInt(parts[0], 10, 64)
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	return id, sub
}

func (s *Server) handleAIConfigOne(w http.ResponseWriter, r *http.Request) {
	id, sub := parseAIID(r.URL.Path, "/api/ai/configs/")
	if id == 0 {
		fail(w, 404, "not found")
		return
	}
	c, err := s.DB.GetAIConfig(id)
	if err != nil {
		fail(w, 404, "配置不存在")
		return
	}
	switch {
	case sub == "test" && r.Method == http.MethodPost:
		cfg, err := s.resolveAIConfig(c)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		var in struct {
			Model string `json:"model"`
		}
		_ = readJSON(r, &in)
		if in.Model != "" {
			cfg.Model = in.Model
		}
		if cfg.Model == "" {
			fail(w, 400, "先选择一个模型")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		ms, err := ai.TestConfig(ctx, cfg)
		if err != nil {
			s.DB.UpdateAIConfigCheck(id, 0, "fail")
			fail(w, 502, "测活失败: "+err.Error())
			return
		}
		s.DB.UpdateAIConfigCheck(id, ms, "ok")
		ok(w, map[string]interface{}{"latencyMs": ms})
	case sub == "models" && r.Method == http.MethodPost:
		cfg, err := s.resolveAIConfig(c)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		modelsPath := ""
		var headers map[string]string
		if p := aiPresetByID(c.ProviderID); p != nil {
			modelsPath = p.ModelsPath
			headers = p.Headers
		}
		if modelsPath == "" {
			// derive from the chat path: .../chat/completions -> .../models
			if strings.HasSuffix(c.ChatPath, "/chat/completions") {
				modelsPath = strings.TrimSuffix(c.ChatPath, "/chat/completions") + "/models"
			} else {
				modelsPath = "/v1/models"
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		ids, err := ai.FetchModels(ctx, cfg, modelsPath, headers)
		if err != nil {
			fail(w, 502, "拉取模型失败: "+err.Error())
			return
		}
		// Pro/ tiers bill cash only (no voucher coverage on e.g.
		// SiliconFlow) and look identical to the regular models in the
		// picker — never let them into the list.
		kept := make([]string, 0, len(ids))
		for _, id := range ids {
			if !strings.HasPrefix(id, "Pro/") {
				kept = append(kept, id)
			}
		}
		ids = kept
		kinds := s.aiClassifyKinds(ctx, c, cfg, modelsPath, headers, ids)
		kb, _ := json.Marshal(kinds)
		s.DB.UpdateAIConfigKinds(id, string(kb))
		mb, _ := json.Marshal(ids)
		s.DB.UpdateAIConfigModels(id, string(mb))
		ok(w, map[string]interface{}{"models": ids, "modelKinds": kinds})
	case sub == "balance" && r.Method == http.MethodPost:
		p := aiPresetByID(c.ProviderID)
		if p == nil || p.Balance == nil {
			ok(w, map[string]interface{}{"supported": false})
			return
		}
		key := ""
		if c.KeyEnc != "" {
			b, err := crypto.Decrypt(s.EncKey, c.KeyEnc)
			if err != nil {
				fail(w, 500, "Key 解密失败")
				return
			}
			key = string(b)
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, p.Balance.Method, p.Balance.URL, nil)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		if c.Format == "anthropic" {
			req.Header.Set("x-api-key", key)
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fail(w, 502, "查询余额失败: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode != 200 {
			fail(w, 502, fmt.Sprintf("查询余额失败: HTTP %d", resp.StatusCode))
			return
		}
		amount := extractJSONPath(body, p.Balance.BalancePath)
		res := map[string]interface{}{"supported": true, "amount": amount, "at": time.Now().Unix()}
		rb, _ := json.Marshal(res)
		s.DB.UpdateAIConfigBalance(id, string(rb))
		ok(w, res)
	case sub == "" && r.Method == http.MethodPut:
		var in struct {
			Name         string   `json:"name"`
			ProviderID   string   `json:"providerId"`
			Format       string   `json:"format"`
			BaseURL      string   `json:"baseUrl"`
			ChatPath     string   `json:"chatPath"`
			Key          string   `json:"key"` // empty = keep
			Models       []string `json:"models"`
			DefaultModel string   `json:"defaultModel"`
			NativeSearch bool     `json:"nativeSearch"`
			PriceIn      float64  `json:"priceIn"`
			PriceOut     float64  `json:"priceOut"`
		}
		if err := readJSON(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		c.Name = in.Name
		c.ProviderID = in.ProviderID
		if in.Format == "anthropic" {
			c.Format = "anthropic"
		} else {
			c.Format = "openai"
		}
		c.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
		if in.ChatPath != "" {
			c.ChatPath = in.ChatPath
		}
		if in.Key != "" {
			e, err := crypto.Encrypt(s.EncKey, []byte(in.Key))
			if err != nil {
				fail(w, 500, "加密失败")
				return
			}
			c.KeyEnc = e
		}
		mb, _ := json.Marshal(in.Models)
		c.Models = string(mb)
		c.DefaultModel = in.DefaultModel
		c.NativeSearch = in.NativeSearch
		c.PriceIn, c.PriceOut = in.PriceIn, in.PriceOut
		if err := s.DB.UpdateAIConfig(c); err != nil {
			fail(w, 500, err.Error())
			return
		}
		// Keep stored kinds for models still listed (a models fetch may
		// have classified them via the provider); dictionary-fill the
		// rest and drop kinds of removed models.
		kinds := map[string]string{}
		if c.ModelKinds != "" {
			_ = json.Unmarshal([]byte(c.ModelKinds), &kinds)
		}
		live := map[string]string{}
		for _, m := range in.Models {
			if k, ok := kinds[m]; ok && k != "" {
				live[m] = k
			} else if k := aiDictKind(m); k != "chat" {
				live[m] = k
			}
		}
		kb, _ := json.Marshal(live)
		c.ModelKinds = string(kb)
		s.DB.UpdateAIConfigKinds(id, c.ModelKinds)
		ok(w, aiConfigView(c, s.keyLast4(c)))
	case sub == "" && r.Method == http.MethodDelete:
		if err := s.DB.DeleteAIConfig(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 404, "not found")
	}
}

// extractJSONPath walks a dotted path ("data.balance_infos.0.total_balance").
func extractJSONPath(body []byte, path string) interface{} {
	var v interface{}
	if json.Unmarshal(body, &v) != nil {
		return nil
	}
	// preset paths are written without the root wrapper used in docs
	path = strings.TrimPrefix(path, "data.")
	cur := v
	if m, ok := cur.(map[string]interface{}); ok {
		if d, ok := m["data"]; ok {
			cur = d
		}
	}
	for _, part := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]interface{}:
			cur = node[part]
		case []interface{}:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			cur = node[i]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// ---------------- threads ----------------

func (s *Server) handleAIThreads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var list []*db.AIThread
		var err error
		if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
			list, err = s.DB.SearchAIThreads(q)
		} else {
			list, err = s.DB.ListAIThreads(r.URL.Query().Get("archived") == "1")
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		if list == nil {
			list = []*db.AIThread{}
		}
		ok(w, list)
	case http.MethodPost:
		var in struct {
			ConfigID int64  `json:"configId"`
			Model    string `json:"model"`
		}
		if err := readJSON(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		id, err := s.DB.CreateAIThread(in.ConfigID, in.Model)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		t, _ := s.DB.GetAIThread(id)
		ok(w, t)
	default:
		fail(w, 405, "method")
	}
}

func (s *Server) handleAIThreadOne(w http.ResponseWriter, r *http.Request) {
	id, sub := parseAIID(r.URL.Path, "/api/ai/threads/")
	if id == 0 {
		fail(w, 404, "not found")
		return
	}
	switch {
	case sub == "messages" && r.Method == http.MethodGet:
		t, err := s.DB.GetAIThread(id)
		if err != nil {
			fail(w, 404, "会话不存在")
			return
		}
		msgs, err := s.DB.ThreadMessages(id)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		leaf, _ := s.DB.ThreadLeaf(id)
		ok(w, map[string]interface{}{"thread": t, "messages": msgs, "leafId": leaf})
	case sub == "fork" && r.Method == http.MethodPost:
		var in struct {
			MessageID int64 `json:"messageId"`
		}
		if err := readJSON(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		src, err := s.DB.GetAIThread(id)
		if err != nil {
			fail(w, 404, "会话不存在")
			return
		}
		path, err := s.DB.ThreadPath(id, in.MessageID)
		if err != nil || len(path) == 0 {
			fail(w, 400, "分叉点无效")
			return
		}
		newID, err := s.DB.CreateAIThread(src.ConfigID, src.Model)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		var parent int64
		for _, m := range path {
			nm := &db.AIMessage{ThreadID: newID, ParentID: parent, Role: m.Role,
				Content: m.Content, Reasoning: m.Reasoning, Attachments: m.Attachments, Usage: m.Usage}
			nid, err := s.DB.CreateAIMessage(nm)
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			parent = nid
		}
		title := src.Title
		if title == "新会话" || title == "" {
			title = "分支会话"
		} else if !strings.HasSuffix(title, "（分支）") {
			title += "（分支）"
		}
		s.DB.SetAIThreadTitle(newID, title)
		t, _ := s.DB.GetAIThread(newID)
		ok(w, t)
	case sub == "" && r.Method == http.MethodPut:
		var in struct {
			Title    *string `json:"title"`
			Pinned   *bool   `json:"pinned"`
			Archived *bool   `json:"archived"`
			SearchOn *bool   `json:"searchOn"`
			ConfigID *int64  `json:"configId"`
			Model    *string `json:"model"`
		}
		if err := readJSON(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		fields := map[string]interface{}{}
		if in.Title != nil {
			fields["title"] = *in.Title
		}
		if in.Pinned != nil {
			fields["pinned"] = boolInt(*in.Pinned)
		}
		if in.Archived != nil {
			fields["archived"] = boolInt(*in.Archived)
		}
		if in.SearchOn != nil {
			fields["search_on"] = boolInt(*in.SearchOn)
		}
		if in.ConfigID != nil {
			fields["config_id"] = *in.ConfigID
		}
		if in.Model != nil {
			fields["model"] = *in.Model
		}
		if err := s.DB.UpdateAIThread(id, fields); err != nil {
			fail(w, 500, err.Error())
			return
		}
		t, _ := s.DB.GetAIThread(id)
		ok(w, t)
	case sub == "" && r.Method == http.MethodDelete:
		msgs, _ := s.DB.ThreadMessages(id)
		for _, m := range msgs {
			s.deleteMsgAttachments(m)
		}
		if err := s.DB.DeleteAIThread(id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 404, "not found")
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---------------- attachments ----------------

var aiFileIDRe = regexp.MustCompile(`^[0-9]+_[a-f0-9]{10}\.[a-z0-9]{1,8}$`)

func (s *Server) aiFilesDir() string {
	d := filepath.Join(s.DataDir, "ai-files")
	_ = os.MkdirAll(d, 0o700)
	return d
}

type aiAttachmentMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // image | audio | text | pdf | file
	Mime string `json:"mime"`
	Size int64  `json:"size"`
}

func aiKindOf(name, mime string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case ext == ".pdf":
		return "pdf"
	case strings.HasPrefix(mime, "text/") || ext == ".txt" || ext == ".md" || ext == ".json" ||
		ext == ".csv" || ext == ".log" || ext == ".go" || ext == ".js" || ext == ".py" ||
		ext == ".html" || ext == ".css" || ext == ".xml" || ext == ".yaml" || ext == ".yml" ||
		ext == ".sh" || ext == ".sql":
		return "text"
	}
	return "file"
}

func (s *Server) handleAIUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "method")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		fail(w, 400, "上传解析失败")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "没有文件")
		return
	}
	defer f.Close()
	if hdr.Size > 20<<20 {
		fail(w, 400, "文件超过 20MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if len(ext) > 9 {
		ext = ""
	}
	id := fmt.Sprintf("%d_%s%s", time.Now().Unix(), randHex(10), ext)
	dst, err := os.Create(filepath.Join(s.aiFilesDir(), id))
	if err != nil {
		fail(w, 500, "保存失败")
		return
	}
	if _, err := io.Copy(dst, f); err != nil {
		dst.Close()
		fail(w, 500, "保存失败")
		return
	}
	dst.Close()
	mime := hdr.Header.Get("Content-Type")
	meta := aiAttachmentMeta{ID: id, Name: hdr.Filename, Kind: aiKindOf(hdr.Filename, mime), Mime: mime, Size: hdr.Size}
	ok(w, meta)
}

func randHex(n int) string {
	const hexd = "0123456789abcdef"
	b := make([]byte, n)
	f, err := os.Open("/dev/urandom")
	if err == nil {
		_, _ = io.ReadFull(f, b)
		f.Close()
	}
	for i := range b {
		b[i] = hexd[b[i]%16]
	}
	return string(b)
}

func (s *Server) handleAIFile(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/ai/files/")
	if !aiFileIDRe.MatchString(id) {
		fail(w, 404, "not found")
		return
	}
	p := filepath.Join(s.aiFilesDir(), id)
	data, err := os.ReadFile(p)
	if err != nil {
		fail(w, 404, "文件不存在")
		return
	}
	ct := http.DetectContentType(data)
	if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment")
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}

// resolveAttachments reads stored files and converts them for the relay.
func (s *Server) resolveAttachments(metas []aiAttachmentMeta) []ai.Attachment {
	var out []ai.Attachment
	for _, m := range metas {
		if !aiFileIDRe.MatchString(m.ID) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.aiFilesDir(), m.ID))
		if err != nil {
			continue
		}
		a := ai.Attachment{Name: m.Name, Mime: m.Mime}
		switch m.Kind {
		case "image":
			a.Kind = "image"
			a.Data = data
		case "text":
			a.Kind = "text"
			a.Text = string(data)
		case "pdf":
			a.Kind = "text"
			a.Text = extractPDFText(data)
		default:
			a.Kind = "file"
		}
		out = append(out, a)
	}
	return out
}

// extractPDFText pulls plain text out of a PDF (best-effort, capped).
func extractPDFText(data []byte) string {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	rd, err := r.GetPlainText()
	if err != nil {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(rd, 120<<10))
	text := strings.TrimSpace(string(b))
	if len(text) > 60000 {
		text = text[:60000]
	}
	return text
}

func (s *Server) deleteMsgAttachments(m *db.AIMessage) {
	var metas []aiAttachmentMeta
	_ = json.Unmarshal([]byte(m.Attachments), &metas)
	for _, meta := range metas {
		if aiFileIDRe.MatchString(meta.ID) {
			// only delete the file when no other message references it
			refs := 0
			rows, _ := s.DB.Query(`SELECT COUNT(*) FROM ai_messages WHERE attachments LIKE ?`, "%"+meta.ID+"%")
			if rows != nil {
				if rows.Next() {
					_ = rows.Scan(&refs)
				}
				rows.Close()
			}
			if refs <= 1 {
				_ = os.Remove(filepath.Join(s.aiFilesDir(), meta.ID))
			}
		}
	}
}

// ---------------- meta (profile / prompts / search settings) ----------------

func (s *Server) handleAIMeta(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ok(w, map[string]interface{}{
			"profile":        s.DB.GetSetting("ai_profile"),
			"prompts":        jsonRawOr(s.DB.GetSetting("ai_prompts"), "[]"),
			"searchBackend":  s.DB.GetSetting("ai_search_backend"),
			"searxngUrl":     s.DB.GetSetting("ai_searxng_url"),
			"tavilyKeySet":   len(s.aiTavilyKeys()) > 0,
			"tavilyKeyCount": len(s.aiTavilyKeys()),
			"tavilyKeyHints": s.aiTavilyKeyHints(),
			"defaultConfigId": atoi64(s.DB.GetSetting("ai_default_config")),
		})
	case http.MethodPut:
		var in struct {
			Profile         *string          `json:"profile"`
			Prompts         *json.RawMessage `json:"prompts"`
			SearchBackend   *string          `json:"searchBackend"`
			SearxngURL      *string          `json:"searxngUrl"`
			TavilyKey       *string          `json:"tavilyKey"`
			TavilyKeys      *[]string        `json:"tavilyKeys"`
			DefaultConfigID *int64           `json:"defaultConfigId"`
		}
		if err := readJSON(r, &in); err != nil {
			fail(w, 400, "bad json")
			return
		}
		if in.Profile != nil {
			_ = s.DB.SetSetting("ai_profile", *in.Profile)
		}
		if in.Prompts != nil {
			_ = s.DB.SetSetting("ai_prompts", string(*in.Prompts))
		}
		if in.SearchBackend != nil {
			_ = s.DB.SetSetting("ai_search_backend", *in.SearchBackend)
		}
		if in.SearxngURL != nil {
			_ = s.DB.SetSetting("ai_searxng_url", *in.SearxngURL)
		}
		if in.TavilyKeys != nil {
			s.storeTavilyKeys(*in.TavilyKeys)
		}
		if in.TavilyKey != nil && *in.TavilyKey != "" {
			s.storeTavilyKeys([]string{*in.TavilyKey})
		}
		if in.DefaultConfigID != nil {
			_ = s.DB.SetSetting("ai_default_config", strconv.FormatInt(*in.DefaultConfigID, 10))
		}
		ok(w, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method")
	}
}

func jsonRawOr(s, def string) json.RawMessage {
	if strings.TrimSpace(s) == "" {
		return json.RawMessage(def)
	}
	return json.RawMessage(s)
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// aiTavilyKeys returns all stored Tavily keys (multi-key list setting,
// falling back to the legacy single-key setting).
func (s *Server) aiTavilyKeys() []string {
	var out []string
	if enc := s.DB.GetSetting("ai_tavily_keys_enc"); enc != "" {
		if b, err := crypto.Decrypt(s.EncKey, enc); err == nil {
			var keys []string
			if json.Unmarshal(b, &keys) == nil {
				for _, k := range keys {
					if k = strings.TrimSpace(k); k != "" {
						out = append(out, k)
					}
				}
			}
		}
	}
	if len(out) == 0 {
		if enc := s.DB.GetSetting("ai_tavily_key_enc"); enc != "" {
			if b, err := crypto.Decrypt(s.EncKey, enc); err == nil && len(b) > 0 {
				out = []string{string(b)}
			}
		}
	}
	return out
}

// aiTavilyKeyHints returns masked views of the stored keys (prefix + last 4)
// so the UI can prove keys are still saved without ever exposing them.
func (s *Server) aiTavilyKeyHints() []string {
	keys := s.aiTavilyKeys()
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		prefix := k
		if i := strings.Index(k, "-"); i > 0 && i < 8 {
			prefix = k[:i+1]
		} else if len(k) > 4 {
			prefix = k[:4]
		}
		last := k
		if len(k) > 4 {
			last = k[len(k)-4:]
		}
		out = append(out, prefix+"••••"+last)
	}
	return out
}

func (s *Server) storeTavilyKeys(keys []string) {
	clean := []string{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			clean = append(clean, k)
		}
	}
	raw, _ := json.Marshal(clean)
	if enc, err := crypto.Encrypt(s.EncKey, raw); err == nil {
		_ = s.DB.SetSetting("ai_tavily_keys_enc", enc)
	}
	_ = s.DB.SetSetting("ai_tavily_key_enc", "")
}

func (s *Server) aiSearchSettings(enabled bool) ai.SearchSettings {
	ss := ai.SearchSettings{Enabled: enabled, Backend: s.DB.GetSetting("ai_search_backend")}
	if ss.Backend == "" {
		ss.Backend = "tavily"
	}
	ss.SearxngURL = s.DB.GetSetting("ai_searxng_url")
	ss.TavilyKeys = s.aiTavilyKeys()
	if len(ss.TavilyKeys) > 0 {
		ss.TavilyKey = ss.TavilyKeys[0]
	}
	return ss
}

// ---------------- chat (streaming) ----------------

const aiSystemBase = "你是一个得力的 AI 助手，回答准确、简洁、有条理。使用简体中文回答（除非用户使用其他语言）。引用联网搜索结果时，在句末标注来源编号，如 [1]。"

const aiSearchDirective = "联网搜索已开启：当问题涉及最新信息、具体数据、实时状态或站外事实时，必须先调用 web_search 工具查证，再依据搜索结果回答并标注来源编号；没有查证的内容不要冒充已经搜索过，更不要编造来源链接。"

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "method")
		return
	}
	var in struct {
		ThreadID    int64              `json:"threadId"`
		ParentID    int64              `json:"parentId"`    // parent for new user msg (0 = current leaf)
		Content     string             `json:"content"`
		Attachments []aiAttachmentMeta `json:"attachments"`
		Regenerate  int64              `json:"regenerate"`  // ai message id to regenerate
		EditFrom    int64              `json:"editFrom"`    // user message id being edited
	}
	if err := readJSON(r, &in); err != nil {
		fail(w, 400, "bad json")
		return
	}
	thread, err := s.DB.GetAIThread(in.ThreadID)
	if err != nil {
		fail(w, 404, "会话不存在")
		return
	}
	cfgRow, err := s.DB.GetAIConfig(thread.ConfigID)
	if err != nil {
		fail(w, 400, "这个会话还没有选择 AI 商家")
		return
	}
	cfg, err := s.resolveAIConfig(cfgRow)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if thread.Model != "" {
		cfg.Model = thread.Model
	}
	if cfg.Model == "" {
		fail(w, 400, "这个商家还没有选择模型")
		return
	}

	// Determine the user message this turn hangs from + the history path.
	var userMsg *db.AIMessage
	if in.Regenerate != 0 {
		old, err := s.DB.GetAIMessage(in.Regenerate)
		if err != nil || old.Role != "ai" {
			fail(w, 400, "重新生成的目标无效")
			return
		}
		userMsg, err = s.DB.GetAIMessage(old.ParentID)
		if err != nil {
			fail(w, 400, "找不到对应的问题")
			return
		}
	} else if in.EditFrom != 0 {
		old, err := s.DB.GetAIMessage(in.EditFrom)
		if err != nil || old.Role != "user" {
			fail(w, 400, "编辑目标无效")
			return
		}
		attJSON, _ := json.Marshal(metasOrEmpty(in.Attachments))
		userMsg = &db.AIMessage{ThreadID: thread.ID, ParentID: old.ParentID, Role: "user",
			Content: in.Content, Attachments: string(attJSON)}
		id, err := s.DB.CreateAIMessage(userMsg)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		userMsg.ID = id
	} else {
		parent := in.ParentID
		if parent == 0 {
			parent, _ = s.DB.ThreadLeaf(thread.ID)
		}
		attJSON, _ := json.Marshal(metasOrEmpty(in.Attachments))
		userMsg = &db.AIMessage{ThreadID: thread.ID, ParentID: parent, Role: "user",
			Content: in.Content, Attachments: string(attJSON)}
		id, err := s.DB.CreateAIMessage(userMsg)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		userMsg.ID = id
	}

	// Media models (image / video generation) take a completely
	// different provider path than chat — route them before any
	// history building.
	if kind := aiModelKind(cfgRow, cfg.Model); kind == "image" || kind == "video" {
		s.aiChatMedia(w, r, thread, cfgRow, cfg, userMsg, kind)
		return
	}

	// Build provider history: stable prefix (system) + path root..userMsg.
	pathMsgs, err := s.DB.ThreadPath(thread.ID, userMsg.ID)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if len(pathMsgs) > 41 { // keep the newest 40 + always include the first exchange
		pathMsgs = append([]*db.AIMessage{pathMsgs[0], pathMsgs[1]}, pathMsgs[len(pathMsgs)-39:]...)
	}
	search := s.aiSearchSettings(thread.SearchOn)
	sys := aiSystemBase
	// Always tell the model today's date (Asia/Shanghai): without it,
	// models guess "today" from stale search results and get it wrong.
	if loc, lerr := time.LoadLocation("Asia/Shanghai"); lerr == nil {
		now := time.Now().In(loc)
		weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
		sys += fmt.Sprintf("\n\n当前日期：%d年%d月%d日，星期%s（北京时间）。回答涉及日期、新闻、价格等时效内容时以这个日期为准。",
			now.Year(), int(now.Month()), now.Day(), weekdays[int(now.Weekday())])
	}
	if search.Ready() {
		sys += "\n\n" + aiSearchDirective
	}
	if p := strings.TrimSpace(s.DB.GetSetting("ai_profile")); p != "" {
		sys += "\n\n关于用户（长期档案）：\n" + p
	}
	providerMsgs := []ai.Msg{{Role: "system", Content: sys}}
	for _, m := range pathMsgs {
		if m.Role == "ai" && strings.TrimSpace(m.Content) == "" {
			// Failed/empty generations carry no information and some
			// providers reject empty assistant content — keep them out
			// of the context sent upstream.
			continue
		}
		pm := ai.Msg{Role: m.Role, Content: m.Content}
		if m.Role == "ai" {
			pm.Role = "assistant"
		}
		if m.Role == "user" && m.Attachments != "" && m.Attachments != "[]" {
			var metas []aiAttachmentMeta
			_ = json.Unmarshal([]byte(m.Attachments), &metas)
			pm.Attachments = s.resolveAttachments(metas)
		}
		providerMsgs = append(providerMsgs, pm)
	}

	// SSE stream.
	fl, okF := w.(http.Flusher)
	if !okF {
		fail(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	send := func(v interface{}) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	send(map[string]interface{}{"t": "user", "id": userMsg.ID})

	var textBuf, reasonBuf strings.Builder
	ev := ai.Events{
		OnText: func(d string) {
			textBuf.WriteString(d)
			send(map[string]interface{}{"t": "text", "d": d})
		},
		OnReasoning: func(d string) {
			reasonBuf.WriteString(d)
			send(map[string]interface{}{"t": "reason", "d": d})
		},
		OnTool: func(name, detail string) {
			send(map[string]interface{}{"t": "tool", "name": name, "d": detail})
		},
	}
	usage, streamErr := ai.StreamChat(r.Context(), cfg, providerMsgs, search, ev)

	usageJSON := ""
	if usage.In > 0 || usage.Out > 0 {
		ub, _ := json.Marshal(usage)
		usageJSON = string(ub)
	}
	if streamErr != nil {
		// Persist the failure on the message itself, so a reloaded
		// thread shows why the reply is empty instead of a blank bubble.
		um := map[string]interface{}{}
		if usageJSON != "" {
			_ = json.Unmarshal([]byte(usageJSON), &um)
		}
		um["error"] = streamErr.Error()
		if ub, err := json.Marshal(um); err == nil {
			usageJSON = string(ub)
		}
	}
	aiMsg := &db.AIMessage{ThreadID: thread.ID, ParentID: userMsg.ID, Role: "ai",
		Content: textBuf.String(), Reasoning: reasonBuf.String(), Attachments: "[]", Usage: usageJSON}
	aiID, _ := s.DB.CreateAIMessage(aiMsg)
	s.DB.TouchAIThread(thread.ID)

	if streamErr != nil {
		send(map[string]interface{}{"t": "error", "d": streamErr.Error(), "aiId": aiID})
		return
	}
	send(map[string]interface{}{"t": "done", "aiId": aiID, "usage": usage})

	// Auto-title after the first exchange.
	if thread.Title == "新会话" || thread.Title == "" {
		go s.aiAutoTitle(thread.ID, cfg, userMsg.Content, textBuf.String())
	}
}

// ---------------- media generation (image / video models) ----------------

func mediaNameFromPrompt(prompt string) string {
	name := strings.Join(strings.Fields(prompt), " ")
	r := []rune(name)
	if len(r) > 18 {
		name = string(r[:18])
	}
	if name == "" {
		name = "AI生成"
	}
	return name
}

// saveAIMediaBytes stores generated media in the shared AI files dir so
// it is served, counted and cleaned up exactly like uploads. Provider
// URLs expire within hours, so generated media is ALWAYS re-hosted
// here first — the user only ever sees our own copy.
func (s *Server) saveAIMediaBytes(data []byte, kind, name string) (*aiAttachmentMeta, error) {
	mime := http.DetectContentType(data)
	ext := ""
	switch {
	case strings.HasPrefix(mime, "image/png"):
		ext = ".png"
	case strings.HasPrefix(mime, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(mime, "image/webp"):
		ext = ".webp"
	case strings.HasPrefix(mime, "image/gif"):
		ext = ".gif"
	case strings.HasPrefix(mime, "video/mp4"):
		ext = ".mp4"
	case strings.HasPrefix(mime, "video/"):
		ext = ".webm"
	}
	if ext == "" {
		if kind == "video" {
			ext = ".mp4"
		} else {
			ext = ".png"
		}
	}
	id := fmt.Sprintf("%d_%s%s", time.Now().Unix(), randHex(10), ext)
	if err := os.WriteFile(filepath.Join(s.aiFilesDir(), id), data, 0o600); err != nil {
		return nil, err
	}
	k := kind
	if k != "image" && k != "video" {
		k = aiKindOf(name+ext, mime)
	}
	return &aiAttachmentMeta{ID: id, Name: name + ext, Kind: k, Mime: mime, Size: int64(len(data))}, nil
}

func (s *Server) downloadAIMedia(rawURL, kind, name string) (*aiAttachmentMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("下载生成文件失败 HTTP %d", resp.StatusCode)
	}
	limit := int64(32 << 20)
	if kind == "video" {
		limit = 256 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("生成文件超过大小上限")
	}
	return s.saveAIMediaBytes(data, kind, name)
}

// aiChatMedia handles one send on an image/video model: same SSE
// envelope as chat, but the "reply" is generated media stored as an
// attachment on the AI message.
func (s *Server) aiChatMedia(w http.ResponseWriter, r *http.Request, thread *db.AIThread, cfgRow *db.AIConfig, cfg ai.Config, userMsg *db.AIMessage, kind string) {
	fl, okF := w.(http.Flusher)
	if !okF {
		fail(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	send := func(v interface{}) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	send(map[string]interface{}{"t": "user", "id": userMsg.ID})
	failMsg := func(msg string) {
		uj, _ := json.Marshal(map[string]interface{}{"error": msg})
		aiMsg := &db.AIMessage{ThreadID: thread.ID, ParentID: userMsg.ID, Role: "ai",
			Content: "", Attachments: "[]", Usage: string(uj)}
		aiID, _ := s.DB.CreateAIMessage(aiMsg)
		s.DB.TouchAIThread(thread.ID)
		send(map[string]interface{}{"t": "error", "d": msg, "aiId": aiID})
	}
	prompt := strings.TrimSpace(userMsg.Content)
	if prompt == "" {
		failMsg("先描述你想生成的内容，再发送")
		return
	}
	// Edit / image-to-video models need a source image: take the first
	// image attached to the user's message.
	var inputImage []byte
	if userMsg.Attachments != "" && userMsg.Attachments != "[]" {
		var metas []aiAttachmentMeta
		if json.Unmarshal([]byte(userMsg.Attachments), &metas) == nil {
			for _, a := range s.resolveAttachments(metas) {
				if a.Kind == "image" && len(a.Data) > 0 {
					inputImage = a.Data
					break
				}
			}
		}
	}
	lowerModel := strings.ToLower(cfg.Model)
	if kind == "image" {
		if strings.Contains(lowerModel, "edit") && len(inputImage) == 0 {
			failMsg("这个模型是用来改图的：先点 ⊕ 附上一张图片，再写你想怎么改")
			return
		}
		send(map[string]interface{}{"t": "tool", "name": "image", "d": "正在生成图片"})
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()
		url, raw, err := ai.GenerateImage(ctx, cfg, prompt, inputImage)
		var meta *aiAttachmentMeta
		if err == nil {
			name := mediaNameFromPrompt(prompt)
			if len(raw) > 0 {
				meta, err = s.saveAIMediaBytes(raw, "image", name)
			} else {
				meta, err = s.downloadAIMedia(url, "image", name)
			}
		}
		if err != nil {
			failMsg("图片生成失败：" + err.Error())
			return
		}
		att, _ := json.Marshal([]aiAttachmentMeta{*meta})
		aiMsg := &db.AIMessage{ThreadID: thread.ID, ParentID: userMsg.ID, Role: "ai",
			Content: "", Attachments: string(att)}
		aiID, _ := s.DB.CreateAIMessage(aiMsg)
		s.DB.TouchAIThread(thread.ID)
		send(map[string]interface{}{"t": "done", "aiId": aiID, "files": []aiAttachmentMeta{*meta}})
		return
	}
	// video: submit now, poll in the background, the message updates
	// itself when the provider finishes.
	if strings.Contains(lowerModel, "i2v") && len(inputImage) == 0 {
		failMsg("这个模型是图生视频：先点 ⊕ 附上一张图片当首帧，再写你想让它怎么动")
		return
	}
	send(map[string]interface{}{"t": "tool", "name": "video", "d": "正在提交视频生成"})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	reqID, err := ai.SubmitVideo(ctx, cfg, prompt, inputImage)
	cancel()
	if err != nil {
		failMsg("视频提交失败：" + err.Error())
		return
	}
	uj, _ := json.Marshal(map[string]interface{}{"video": map[string]interface{}{
		"req": reqID, "status": "pending", "configId": cfgRow.ID}})
	aiMsg := &db.AIMessage{ThreadID: thread.ID, ParentID: userMsg.ID, Role: "ai",
		Content: "", Attachments: "[]", Usage: string(uj)}
	aiID, _ := s.DB.CreateAIMessage(aiMsg)
	s.DB.TouchAIThread(thread.ID)
	send(map[string]interface{}{"t": "done", "aiId": aiID, "videoPending": true})
	go s.pollAIVideo(aiID)
}

func (s *Server) failAIVideo(msgID int64, msg string) {
	uj, _ := json.Marshal(map[string]interface{}{
		"video": map[string]string{"status": "failed"}, "error": msg})
	_ = s.DB.UpdateAIMessageMedia(msgID, "[]", string(uj))
}

func (s *Server) pollAIVideo(msgID int64) {
	for i := 0; i < 90; i++ {
		time.Sleep(20 * time.Second)
		msg, err := s.DB.GetAIMessage(msgID)
		if err != nil {
			return // message deleted meanwhile
		}
		var u struct {
			Video *struct {
				Req      string `json:"req"`
				Status   string `json:"status"`
				ConfigID int64  `json:"configId"`
			} `json:"video"`
		}
		if json.Unmarshal([]byte(msg.Usage), &u) != nil || u.Video == nil || u.Video.Status != "pending" {
			return
		}
		cfgRow, err := s.DB.GetAIConfig(u.Video.ConfigID)
		if err != nil {
			s.failAIVideo(msgID, "商家配置已被删除，视频无法取回")
			return
		}
		cfg, err := s.resolveAIConfig(cfgRow)
		if err != nil {
			s.failAIVideo(msgID, "商家配置不可用："+err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		status, url, reason, err := ai.VideoStatus(ctx, cfg, u.Video.Req)
		cancel()
		if err != nil {
			continue // transient provider/network error: keep polling
		}
		switch status {
		case "Succeed":
			if url == "" {
				s.failAIVideo(msgID, "视频生成完成但商家没有返回文件")
				return
			}
			meta, derr := s.downloadAIMedia(url, "video", "AI生成视频")
			if derr != nil {
				s.failAIVideo(msgID, "视频下载失败："+derr.Error())
				return
			}
			att, _ := json.Marshal([]aiAttachmentMeta{*meta})
			uj, _ := json.Marshal(map[string]interface{}{"video": map[string]string{"status": "done"}})
			_ = s.DB.UpdateAIMessageMedia(msgID, string(att), string(uj))
			return
		case "Failed":
			if reason == "" {
				reason = "商家未说明原因"
			}
			s.failAIVideo(msgID, "视频生成失败："+reason)
			return
		}
	}
	s.failAIVideo(msgID, "视频生成超时（等了 30 分钟）")
}

// resumeAIVideos re-arms pollers for video jobs left pending by a
// restart, so a deploy never orphans a paid generation.
func (s *Server) resumeAIVideos() {
	msgs, err := s.DB.PendingVideoMessages()
	if err != nil {
		return
	}
	for _, m := range msgs {
		go s.pollAIVideo(m.ID)
	}
}

func metasOrEmpty(m []aiAttachmentMeta) []aiAttachmentMeta {
	if m == nil {
		return []aiAttachmentMeta{}
	}
	return m
}

func (s *Server) aiAutoTitle(threadID int64, cfg ai.Config, userText, aiText string) {
	title := ""
	if len([]rune(userText)) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		t, err := ai.Complete(ctx, cfg, "", "用不超过12个字概括下面这段对话的主题，只输出标题本身，不要标点和引号：\n用户："+truncateRunes(userText, 200)+"\n助手："+truncateRunes(aiText, 200))
		if err == nil {
			t = strings.Trim(strings.TrimSpace(t), "\"'「」")
			if len([]rune(t)) >= 2 && len([]rune(t)) <= 20 {
				title = t
			}
		}
	}
	if title == "" {
		title = truncateRunes(userText, 18)
		if title == "" {
			title = "新会话"
		}
	}
	s.DB.SetAIThreadTitle(threadID, title)
}

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

// ---------------- stats & cleanup ----------------

func (s *Server) handleAIStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.DB.AIStats()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	// attachment files by kind
	type kindStat struct {
		Count int   `json:"count"`
		Bytes int64 `json:"bytes"`
	}
	byKind := map[string]*kindStat{"image": {}, "video": {}, "audio": {}, "text": {}, "pdf": {}, "file": {}}
	entries, _ := os.ReadDir(s.aiFilesDir())
	fileSizes := map[string]int64{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil {
			fileSizes[e.Name()] = info.Size()
		}
	}
	msgs, _ := s.DB.ThreadMessages(0) // unused; per-kind scan below
	_ = msgs
	rows, _ := s.DB.Query(`SELECT attachments FROM ai_messages WHERE attachments != '[]'`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var att string
			if rows.Scan(&att) != nil {
				continue
			}
			var metas []aiAttachmentMeta
			if json.Unmarshal([]byte(att), &metas) != nil {
				continue
			}
			for _, m := range metas {
				ks := byKind[m.Kind]
				if ks == nil {
					ks = &kindStat{}
					byKind[m.Kind] = ks
				}
				ks.Count++
				ks.Bytes += fileSizes[m.ID]
			}
		}
	}
	// usage totals
	var totIn, totOut, totCache int
	var totCost float64
	urows, _ := s.DB.Query(`SELECT usage_json FROM ai_messages WHERE usage_json != ''`)
	if urows != nil {
		defer urows.Close()
		for urows.Next() {
			var uj string
			if urows.Scan(&uj) != nil {
				continue
			}
			var u ai.Usage
			if json.Unmarshal([]byte(uj), &u) == nil {
				totIn += u.In
				totOut += u.Out
				totCache += u.CacheHit
				totCost += u.Cost
			}
		}
	}
	ok(w, map[string]interface{}{
		"threads": st.Threads, "messages": st.Messages,
		"textBytes": st.TextBytes, "reasonBytes": st.ReasonBytes,
		"byKind": byKind,
		"usage":  map[string]interface{}{"in": totIn, "out": totOut, "cacheHit": totCache, "cost": totCost},
	})
}

func (s *Server) handleAICleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "method")
		return
	}
	var in struct {
		Type string `json:"type"` // image|audio|text|pdf|file|reasoning|all
	}
	if err := readJSON(r, &in); err != nil {
		fail(w, 400, "bad json")
		return
	}
	switch in.Type {
	case "reasoning":
		if err := s.DB.AIClearType("reasoning"); err != nil {
			fail(w, 500, err.Error())
			return
		}
	case "all":
		rows, _ := s.DB.Query(`SELECT id FROM ai_threads`)
		var ids []int64
		if rows != nil {
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
		}
		for _, id := range ids {
			msgs, _ := s.DB.ThreadMessages(id)
			for _, m := range msgs {
				s.deleteMsgAttachments(m)
			}
			_ = s.DB.DeleteAIThread(id)
		}
	case "image", "video", "audio", "text", "pdf", "file":
		rows, _ := s.DB.Query(`SELECT id, attachments FROM ai_messages WHERE attachments != '[]'`)
		type upd struct {
			id  int64
			att string
		}
		var updates []upd
		var toDelete []string
		if rows != nil {
			for rows.Next() {
				var mid int64
				var att string
				if rows.Scan(&mid, &att) != nil {
					continue
				}
				var metas []aiAttachmentMeta
				if json.Unmarshal([]byte(att), &metas) != nil {
					continue
				}
				kept := []aiAttachmentMeta{}
				changed := false
				for _, m := range metas {
					if m.Kind == in.Type {
						toDelete = append(toDelete, m.ID)
						changed = true
					} else {
						kept = append(kept, m)
					}
				}
				if changed {
					nb, _ := json.Marshal(kept)
					updates = append(updates, upd{mid, string(nb)})
				}
			}
			rows.Close()
		}
		for _, u := range updates {
			_, _ = s.DB.Exec(`UPDATE ai_messages SET attachments = ? WHERE id = ?`, u.att, u.id)
		}
		for _, fid := range toDelete {
			if aiFileIDRe.MatchString(fid) {
				_ = os.Remove(filepath.Join(s.aiFilesDir(), fid))
			}
		}
	default:
		fail(w, 400, "未知类型")
		return
	}
	ok(w, map[string]bool{"ok": true})
}
