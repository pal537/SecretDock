// Package ai implements SecretDock's AI relay: it forwards chat requests to
// user-configured providers (OpenAI-compatible or Anthropic native), streams
// the response back as events, and optionally gives the model web tools
// (search / fetch) executed server-side.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// Config is one resolved provider configuration (key already decrypted).
type Config struct {
	Format       string // "openai" | "anthropic"
	BaseURL      string
	ChatPath     string
	APIKey       string
	Model        string
	NativeSearch bool
	PriceIn      float64 // per 1M input tokens
	PriceOut     float64 // per 1M output tokens
}

// Attachment is one resolved message attachment.
type Attachment struct {
	Kind string // image | text | file
	Name string
	Mime string
	Data []byte // raw bytes (images)
	Text string // extracted text (text/pdf/doc)
}

// Msg is one conversation message fed to the provider.
type Msg struct {
	Role        string // system | user | assistant
	Content     string
	Attachments []Attachment
}

// Usage is normalized token accounting for one completion.
type Usage struct {
	In       int     `json:"in"`       // total input tokens (incl. cached where provider counts them)
	CacheHit int     `json:"cacheHit"` // input tokens served from provider cache
	Out      int     `json:"out"`
	Cost     float64 `json:"cost"` // in provider currency units, 0 when prices unset
}

// Events receives streaming callbacks.
type Events struct {
	OnText      func(delta string)
	OnReasoning func(delta string)
	OnUsage     func(u Usage)
	OnTool      func(name, detail string) // e.g. ("search", "正在联网搜索「…」")
}

// SearchSettings configures the app-level web tools.
type SearchSettings struct {
	Enabled    bool
	Backend    string // "tavily" | "searxng"
	TavilyKey  string   // first key, kept for convenience
	TavilyKeys []string // all keys; rotated per search, next key on failure
	SearxngURL string
}

func (s SearchSettings) Ready() bool {
	if !s.Enabled {
		return false
	}
	switch s.Backend {
	case "tavily":
		return len(s.TavilyKeys) > 0 || s.TavilyKey != ""
	case "searxng":
		return s.SearxngURL != ""
	}
	return false
}

// keys returns the effective Tavily key list.
func (s SearchSettings) keys() []string {
	if len(s.TavilyKeys) > 0 {
		return s.TavilyKeys
	}
	if s.TavilyKey != "" {
		return []string{s.TavilyKey}
	}
	return nil
}

var httpTransport = &http.Transport{
	DialContext:         (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
	ResponseHeaderTimeout: 90 * time.Second,
}

func streamClient() *http.Client { return &http.Client{Transport: httpTransport} }
func plainClient() *http.Client {
	return &http.Client{Transport: httpTransport, Timeout: 60 * time.Second}
}

// JoinURL joins base + path tolerating trailing slashes on base and an
// absolute path in chat path.
func JoinURL(base, path string) string {
	b := strings.TrimRight(base, "/")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return b + path
}

// StreamChat runs one full completion (including tool-call rounds) and
// streams text/reasoning through ev. It returns the final usage.
func StreamChat(ctx context.Context, cfg Config, msgs []Msg, search SearchSettings, ev Events) (Usage, error) {
	switch cfg.Format {
	case "anthropic":
		return streamAnthropic(ctx, cfg, msgs, search, ev)
	default:
		return streamOpenAI(ctx, cfg, msgs, search, ev)
	}
}

// ---------- shared helpers ----------

type oaiMsg struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func buildOpenAIContent(m Msg) interface{} {
	hasImg := false
	for _, a := range m.Attachments {
		if a.Kind == "image" && len(a.Data) > 0 {
			hasImg = true
		}
	}
	text := m.Content
	for _, a := range m.Attachments {
		if a.Kind == "text" && a.Text != "" {
			text += "\n\n[附件 " + a.Name + "]\n" + a.Text
		} else if a.Kind == "file" {
			text += "\n\n[附件 " + a.Name + "：该文件无法提取文本内容]"
		}
	}
	if !hasImg {
		return text
	}
	parts := []map[string]interface{}{}
	if text != "" {
		parts = append(parts, map[string]interface{}{"type": "text", "text": text})
	}
	for _, a := range m.Attachments {
		if a.Kind == "image" && len(a.Data) > 0 {
			mime := a.Mime
			if mime == "" {
				mime = "image/png"
			}
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(a.Data),
				},
			})
		}
	}
	return parts
}

func costOf(cfg Config, u Usage) float64 {
	if cfg.PriceIn == 0 && cfg.PriceOut == 0 {
		return 0
	}
	nonCached := u.In - u.CacheHit
	if nonCached < 0 {
		nonCached = 0
	}
	return (float64(nonCached)*cfg.PriceIn + float64(u.CacheHit)*cfg.PriceIn*0.25 + float64(u.Out)*cfg.PriceOut) / 1e6
}

// ---------- OpenAI-compatible adapter ----------

var searchTools = []map[string]interface{}{
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "web_search",
			"description": "联网搜索。当需要最新信息、事实核查或你不确定的知识时调用。返回若干条带标题、链接和摘要的结果。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "搜索关键词"},
				},
				"required": []string{"query"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "fetch_url",
			"description": "打开一个网页并读取其正文文本。当搜索摘要不够、需要页面详情时调用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "要打开的网页 URL"},
				},
				"required": []string{"url"},
			},
		},
	},
}

// noToolModel reports whether a model is known to never emit tool calls on
// OpenAI-compatible providers (DeepSeek-R1 family, QwQ): with search on, the
// relay searches on the model's behalf instead of offering tools it can't use.
func noToolModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "deepseek-r1") || strings.Contains(m, "/r1") ||
		strings.Contains(m, "-r1") || strings.Contains(m, "qwq")
}

func streamOpenAI(ctx context.Context, cfg Config, msgs []Msg, search SearchSettings, ev Events) (Usage, error) {
	toolsOn := search.Ready()
	if toolsOn && noToolModel(cfg.Model) {
		// Pre-search: fetch results for the latest user question and hand
		// them to the model as context; it answers from real sources.
		q := ""
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "user" {
				q = strings.TrimSpace(msgs[i].Content)
				break
			}
		}
		if q != "" {
			if ev.OnTool != nil {
				ev.OnTool("search", "正在联网搜索「"+q+"」")
			}
			if res, err := doSearch(ctx, search, q); err == nil && len(res) > 0 {
				var b strings.Builder
				for i, r := range res {
					fmt.Fprintf(&b, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, r.Content)
				}
				msgs = append(msgs, Msg{Role: "user", Content: "【联网搜索结果】\n" + b.String() +
					"请依据以上搜索结果回答我上面的问题，引用处标注来源编号（如 [1]），不要编造不存在的来源。"})
			}
		}
		toolsOn = false
	}
	conv := make([]oaiMsg, 0, len(msgs))
	for _, m := range msgs {
		conv = append(conv, oaiMsg{Role: m.Role, Content: buildOpenAIContent(m)})
	}
	var total Usage
	for round := 0; round <= 5; round++ {
		reqBody := map[string]interface{}{
			"model":  cfg.Model,
			"stream": true,
			"stream_options": map[string]bool{"include_usage": true},
			"messages": conv,
		}
		if toolsOn {
			reqBody["tools"] = searchTools
			reqBody["tool_choice"] = "auto"
		}
		usage, calls, err := openAIOnce(ctx, cfg, reqBody, ev)
		total.In += usage.In
		total.CacheHit += usage.CacheHit
		total.Out += usage.Out
		if err != nil {
			total.Cost = costOf(cfg, total)
			return total, err
		}
		if len(calls) == 0 {
			break
		}
		// assistant tool-call turn, then tool results
		conv = append(conv, oaiMsg{Role: "assistant", ToolCalls: calls})
		for _, c := range calls {
			out := runTool(ctx, search, c.Function.Name, c.Function.Arguments, ev)
			conv = append(conv, oaiMsg{Role: "tool", ToolCallID: c.ID, Name: c.Function.Name, Content: out})
		}
	}
	total.Cost = costOf(cfg, total)
	return total, nil
}

// openAIOnce performs one streaming request; returns usage + any tool calls.
func openAIOnce(ctx context.Context, cfg Config, reqBody map[string]interface{}, ev Events) (Usage, []oaiToolCall, error) {
	var u Usage
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", JoinURL(cfg.BaseURL, cfg.ChatPath), bytes.NewReader(body))
	if err != nil {
		return u, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := streamClient().Do(req)
	if err != nil {
		return u, nil, fmt.Errorf("请求商家接口失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return u, nil, fmt.Errorf("商家接口返回 %d: %s", resp.StatusCode, snippetErr(string(b)))
	}
	toolAcc := map[int]*oaiToolCall{}
	var toolOrder []int
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens        int `json:"prompt_tokens"`
				CompletionTokens    int `json:"completion_tokens"`
				PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
				PromptTokensDetails *struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.ReasoningContent != "" && ev.OnReasoning != nil {
				ev.OnReasoning(ch.Delta.ReasoningContent)
			}
			if ch.Delta.Content != "" && ev.OnText != nil {
				ev.OnText(ch.Delta.Content)
			}
			for _, tc := range ch.Delta.ToolCalls {
				acc, ok := toolAcc[tc.Index]
				if !ok {
					acc = &oaiToolCall{Type: "function"}
					toolAcc[tc.Index] = acc
					toolOrder = append(toolOrder, tc.Index)
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					acc.Function.Name = tc.Function.Name
				}
				acc.Function.Arguments += tc.Function.Arguments
			}
		}
		if chunk.Usage != nil {
			u.In = chunk.Usage.PromptTokens
			u.Out = chunk.Usage.CompletionTokens
			u.CacheHit = chunk.Usage.PromptCacheHitTokens
			if chunk.Usage.PromptTokensDetails != nil && chunk.Usage.PromptTokensDetails.CachedTokens > u.CacheHit {
				u.CacheHit = chunk.Usage.PromptTokensDetails.CachedTokens
			}
		}
	}
	if err := sc.Err(); err != nil {
		return u, nil, fmt.Errorf("读取商家响应中断: %w", err)
	}
	var calls []oaiToolCall
	for _, idx := range toolOrder {
		if c := toolAcc[idx]; c != nil && c.Function.Name != "" {
			calls = append(calls, *c)
		}
	}
	return u, calls, nil
}

// ---------- web tools ----------

type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

func runTool(ctx context.Context, search SearchSettings, name, argsJSON string, ev Events) string {
	var args map[string]string
	_ = json.Unmarshal([]byte(argsJSON), &args)
	switch name {
	case "web_search":
		q := args["query"]
		if ev.OnTool != nil {
			ev.OnTool("search", "正在联网搜索「"+q+"」")
		}
		res, err := doSearch(ctx, search, q)
		if err != nil {
			return "搜索失败: " + err.Error()
		}
		var b strings.Builder
		for i, r := range res {
			fmt.Fprintf(&b, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, r.Content)
		}
		if b.Len() == 0 {
			return "没有搜索到结果。"
		}
		return b.String()
	case "fetch_url":
		u := args["url"]
		if ev.OnTool != nil {
			ev.OnTool("fetch", "正在打开网页 "+u)
		}
		text, err := fetchPageText(ctx, u)
		if err != nil {
			return "打开网页失败: " + err.Error()
		}
		return text
	}
	return "未知工具: " + name
}

var tavilyKeyCursor uint64

func doSearch(ctx context.Context, s SearchSettings, query string) ([]searchResult, error) {
	switch s.Backend {
	case "tavily":
		keys := s.keys()
		if len(keys) == 0 {
			return nil, fmt.Errorf("未配置 Tavily Key")
		}
		// Rotate: start from a rotating key, fall through to the next key
		// whenever one fails (invalid key, rate limit, quota exhausted…).
		start := int(atomic.AddUint64(&tavilyKeyCursor, 1)-1) % len(keys)
		var lastErr error
		for i := 0; i < len(keys); i++ {
			res, err := tavilySearchOnce(ctx, keys[(start+i)%len(keys)], query)
			if err == nil {
				return res, nil
			}
			lastErr = err
		}
		return nil, lastErr
	case "searxng":
		base := strings.TrimRight(s.SearxngURL, "/")
		req, err := http.NewRequestWithContext(ctx, "GET", base+"/search?format=json&q="+strings.ReplaceAll(query, " ", "+"), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "SecretDock")
		resp, err := plainClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("searxng %d", resp.StatusCode)
		}
		var out struct {
			Results []searchResult `json:"results"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
			return nil, err
		}
		if len(out.Results) > 5 {
			out.Results = out.Results[:5]
		}
		return out.Results, nil
	}
	return nil, fmt.Errorf("未配置搜索引擎")
}

func tavilySearchOnce(ctx context.Context, apiKey, query string) ([]searchResult, error) {
	payload := map[string]interface{}{
		"api_key": apiKey, "query": query,
		"max_results": 5, "search_depth": "basic", "include_answer": false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := plainClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tavily %d", resp.StatusCode)
	}
	var out struct {
		Results []searchResult `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

var (
	reScript = regexp.MustCompile(`(?is)<script.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style.*?</style>`)
	reTag    = regexp.MustCompile(`(?s)<[^>]+>`)
	reSpace  = regexp.MustCompile(`\s+`)
)

func fetchPageText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36")
	resp, err := plainClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	s := reScript.ReplaceAllString(string(b), " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reTag.ReplaceAllString(s, " ")
	s = reSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if len(s) > 8000 {
		s = s[:8000]
	}
	return s, nil
}

func snippetErr(s string) string {
	s = strings.TrimSpace(s)
	var v struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(s), &v) == nil {
		if v.Error.Message != "" {
			return v.Error.Message
		}
		if v.Message != "" {
			return v.Message
		}
	}
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// ---------- Anthropic adapter ----------

func streamAnthropic(ctx context.Context, cfg Config, msgs []Msg, search SearchSettings, ev Events) (Usage, error) {
	var u Usage
	var system string
	var conv []map[string]interface{}
	for _, m := range msgs {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		blocks := []map[string]interface{}{}
		text := m.Content
		for _, a := range m.Attachments {
			if a.Kind == "text" && a.Text != "" {
				text += "\n\n[附件 " + a.Name + "]\n" + a.Text
			} else if a.Kind == "file" {
				text += "\n\n[附件 " + a.Name + "：该文件无法提取文本内容]"
			}
		}
		if m.Role == "user" {
			for _, a := range m.Attachments {
				if a.Kind == "image" && len(a.Data) > 0 {
					mime := a.Mime
					if mime == "" {
						mime = "image/png"
					}
					blocks = append(blocks, map[string]interface{}{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": mime,
							"data":       base64.StdEncoding.EncodeToString(a.Data),
						},
					})
				}
			}
		}
		if text != "" {
			blocks = append(blocks, map[string]interface{}{"type": "text", "text": text})
		}
		if len(blocks) == 0 {
			continue
		}
		conv = append(conv, map[string]interface{}{"role": m.Role, "content": blocks})
	}
	// cache-friendly: stable system block + cache marker on last user turn
	body := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": 8192,
		"stream":     true,
		"messages":   conv,
	}
	if system != "" {
		body["system"] = []map[string]interface{}{
			{"type": "text", "text": system, "cache_control": map[string]string{"type": "ephemeral"}},
		}
	}
	if cfg.NativeSearch && search.Enabled {
		body["tools"] = []map[string]interface{}{
			{"type": "web_search_20250305", "name": "web_search", "max_uses": 5},
		}
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", JoinURL(cfg.BaseURL, cfg.ChatPath), bytes.NewReader(raw))
	if err != nil {
		return u, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := streamClient().Do(req)
	if err != nil {
		return u, fmt.Errorf("请求商家接口失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return u, fmt.Errorf("商家接口返回 %d: %s", resp.StatusCode, snippetErr(string(b)))
	}
	var inTok, cacheRead, cacheCreate, outTok int
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var evt struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Message struct {
				Usage struct {
					InputTokens              int `json:"input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			ContentBlock struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &evt) != nil {
			continue
		}
		switch evt.Type {
		case "message_start":
			inTok = evt.Message.Usage.InputTokens
			cacheRead = evt.Message.Usage.CacheReadInputTokens
			cacheCreate = evt.Message.Usage.CacheCreationInputTokens
		case "content_block_start":
			if (evt.ContentBlock.Type == "server_tool_use" || evt.ContentBlock.Type == "tool_use") && ev.OnTool != nil {
				ev.OnTool("search", "正在联网搜索")
			}
		case "content_block_delta":
			if evt.Delta.Type == "text_delta" && evt.Delta.Text != "" && ev.OnText != nil {
				ev.OnText(evt.Delta.Text)
			}
			if evt.Delta.Type == "thinking_delta" && evt.Delta.Thinking != "" && ev.OnReasoning != nil {
				ev.OnReasoning(evt.Delta.Thinking)
			}
		case "message_delta":
			outTok = evt.Usage.OutputTokens
		}
	}
	if err := sc.Err(); err != nil {
		return u, fmt.Errorf("读取商家响应中断: %w", err)
	}
	u.In = inTok + cacheCreate + cacheRead
	u.CacheHit = cacheRead
	u.Out = outTok
	u.Cost = costOf(cfg, u)
	return u, nil
}

// ---------- non-streaming helpers (health check / models / title) ----------

// TestConfig performs a minimal real request to verify a config works.
// Returns latency in ms.
func TestConfig(ctx context.Context, cfg Config) (int64, error) {
	start := time.Now()
	if cfg.Format == "anthropic" {
		body := map[string]interface{}{
			"model": cfg.Model, "max_tokens": 1,
			"messages": []map[string]string{{"role": "user", "content": "hi"}},
		}
		raw, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, "POST", JoinURL(cfg.BaseURL, cfg.ChatPath), bytes.NewReader(raw))
		if err != nil {
			return 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		resp, err := plainClient().Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8192))
		if resp.StatusCode != 200 {
			return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return time.Since(start).Milliseconds(), nil
	}
	body := map[string]interface{}{
		"model": cfg.Model, "max_tokens": 1, "stream": false,
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", JoinURL(cfg.BaseURL, cfg.ChatPath), bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := plainClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippetErr(string(b)))
	}
	return time.Since(start).Milliseconds(), nil
}

// FetchModels lists model ids from the provider's models endpoint.
func FetchModels(ctx context.Context, cfg Config, modelsPath string, headers map[string]string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", JoinURL(cfg.BaseURL, modelsPath), nil)
	if err != nil {
		return nil, err
	}
	if cfg.Format == "anthropic" {
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := plainClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippetErr(string(b)))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// Complete runs a tiny non-streaming completion (used for auto titles).
func Complete(ctx context.Context, cfg Config, system, prompt string) (string, error) {
	msgs := []Msg{{Role: "user", Content: prompt}}
	if system != "" {
		msgs = append([]Msg{{Role: "system", Content: system}}, msgs...)
	}
	var sb strings.Builder
	ev := Events{OnText: func(d string) { sb.WriteString(d) }}
	// reuse streaming adapters with search off
	_, err := StreamChat(ctx, cfg, msgs, SearchSettings{}, ev)
	return strings.TrimSpace(sb.String()), err
}
