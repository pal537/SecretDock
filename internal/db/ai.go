package db

import (
	"fmt"
	"time"
)

// ---------------- AI module schema ----------------

const aiSchema = `
CREATE TABLE IF NOT EXISTS ai_configs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	format TEXT NOT NULL DEFAULT 'openai',
	base_url TEXT NOT NULL DEFAULT '',
	chat_path TEXT NOT NULL DEFAULT '/v1/chat/completions',
	key_enc TEXT NOT NULL DEFAULT '',
	models TEXT NOT NULL DEFAULT '[]',
	default_model TEXT NOT NULL DEFAULT '',
	native_search INTEGER NOT NULL DEFAULT 0,
	price_in REAL NOT NULL DEFAULT 0,
	price_out REAL NOT NULL DEFAULT 0,
	last_latency INTEGER NOT NULL DEFAULT 0,
	last_status TEXT NOT NULL DEFAULT '',
	last_check_at INTEGER NOT NULL DEFAULT 0,
	balance_json TEXT NOT NULL DEFAULT '',
	balance_at INTEGER NOT NULL DEFAULT 0,
	model_kinds TEXT NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS ai_threads (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL DEFAULT '',
	config_id INTEGER NOT NULL DEFAULT 0,
	model TEXT NOT NULL DEFAULT '',
	pinned INTEGER NOT NULL DEFAULT 0,
	archived INTEGER NOT NULL DEFAULT 0,
	search_on INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL DEFAULT 0,
	updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS ai_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	thread_id INTEGER NOT NULL REFERENCES ai_threads(id) ON DELETE CASCADE,
	parent_id INTEGER NOT NULL DEFAULT 0,
	role TEXT NOT NULL DEFAULT 'user',
	content TEXT NOT NULL DEFAULT '',
	reasoning TEXT NOT NULL DEFAULT '',
	attachments TEXT NOT NULL DEFAULT '[]',
	usage_json TEXT NOT NULL DEFAULT '',
	hidden INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_ai_messages_thread ON ai_messages(thread_id, id);
CREATE INDEX IF NOT EXISTS idx_ai_messages_parent ON ai_messages(parent_id);
`

func ensureAITables(d *DB) error {
	if _, err := d.Exec(aiSchema); err != nil {
		return err
	}
	// migrate: ai_configs.model_kinds (JSON map model -> chat|image|video)
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('ai_configs') WHERE name = 'model_kinds'`).Scan(&n)
	if n == 0 {
		if _, err := d.Exec(`ALTER TABLE ai_configs ADD COLUMN model_kinds TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// ---------------- AI configs ----------------

type AIConfig struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ProviderID   string  `json:"providerId"`
	Format       string  `json:"format"`
	BaseURL      string  `json:"baseUrl"`
	ChatPath     string  `json:"chatPath"`
	KeyEnc       string  `json:"-"`
	Models       string  `json:"models"` // JSON array string
	DefaultModel string  `json:"defaultModel"`
	NativeSearch bool    `json:"nativeSearch"`
	PriceIn      float64 `json:"priceIn"`
	PriceOut     float64 `json:"priceOut"`
	LastLatency  int64   `json:"lastLatency"`
	LastStatus   string  `json:"lastStatus"`
	LastCheckAt  int64   `json:"lastCheckAt"`
	BalanceJSON  string  `json:"balanceJson"`
	BalanceAt    int64   `json:"balanceAt"`
	ModelKinds   string  `json:"-"` // JSON map: model id -> "image"|"video" (chat = absent)
	Sort         int64   `json:"sort"`
	CreatedAt    int64   `json:"createdAt"`
}

func scanAIConfig(rows interface{ Scan(...interface{}) error }) (*AIConfig, error) {
	var c AIConfig
	var ns int
	err := rows.Scan(&c.ID, &c.Name, &c.ProviderID, &c.Format, &c.BaseURL, &c.ChatPath,
		&c.KeyEnc, &c.Models, &c.DefaultModel, &ns, &c.PriceIn, &c.PriceOut,
		&c.LastLatency, &c.LastStatus, &c.LastCheckAt, &c.BalanceJSON, &c.BalanceAt,
		&c.Sort, &c.CreatedAt, &c.ModelKinds)
	c.NativeSearch = ns == 1
	return &c, err
}

const aiConfigCols = `id, name, provider_id, format, base_url, chat_path, key_enc, models, default_model, native_search, price_in, price_out, last_latency, last_status, last_check_at, balance_json, balance_at, sort, created_at, model_kinds`

func (d *DB) ListAIConfigs() ([]*AIConfig, error) {
	rows, err := d.Query(`SELECT ` + aiConfigCols + ` FROM ai_configs ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AIConfig
	for rows.Next() {
		c, err := scanAIConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) GetAIConfig(id int64) (*AIConfig, error) {
	row := d.QueryRow(`SELECT `+aiConfigCols+` FROM ai_configs WHERE id = ?`, id)
	return scanAIConfig(row)
}

func (d *DB) CreateAIConfig(c *AIConfig) (int64, error) {
	ns := 0
	if c.NativeSearch {
		ns = 1
	}
	r, err := d.Exec(`INSERT INTO ai_configs (name, provider_id, format, base_url, chat_path, key_enc, models, default_model, native_search, price_in, price_out, sort, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort),0)+1 FROM ai_configs), ?)`,
		c.Name, c.ProviderID, c.Format, c.BaseURL, c.ChatPath, c.KeyEnc, c.Models,
		c.DefaultModel, ns, c.PriceIn, c.PriceOut, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) UpdateAIConfig(c *AIConfig) error {
	ns := 0
	if c.NativeSearch {
		ns = 1
	}
	_, err := d.Exec(`UPDATE ai_configs SET name=?, provider_id=?, format=?, base_url=?, chat_path=?, key_enc=?, models=?, default_model=?, native_search=?, price_in=?, price_out=? WHERE id=?`,
		c.Name, c.ProviderID, c.Format, c.BaseURL, c.ChatPath, c.KeyEnc, c.Models,
		c.DefaultModel, ns, c.PriceIn, c.PriceOut, c.ID)
	return err
}

func (d *DB) UpdateAIConfigCheck(id, latency int64, status string) {
	_, _ = d.Exec(`UPDATE ai_configs SET last_latency=?, last_status=?, last_check_at=? WHERE id=?`,
		latency, status, time.Now().Unix(), id)
}

func (d *DB) UpdateAIConfigBalance(id int64, balanceJSON string) {
	_, _ = d.Exec(`UPDATE ai_configs SET balance_json=?, balance_at=? WHERE id=?`,
		balanceJSON, time.Now().Unix(), id)
}

func (d *DB) UpdateAIConfigModels(id int64, models string) {
	_, _ = d.Exec(`UPDATE ai_configs SET models=? WHERE id=?`, models, id)
}

func (d *DB) UpdateAIConfigKinds(id int64, kinds string) {
	_, _ = d.Exec(`UPDATE ai_configs SET model_kinds=? WHERE id=?`, kinds, id)
}

// UpdateAIMessageMedia replaces a message's attachments and usage in one
// go — used when an async media job (video generation) finishes.
func (d *DB) UpdateAIMessageMedia(id int64, attachments, usage string) error {
	_, err := d.Exec(`UPDATE ai_messages SET attachments=?, usage_json=? WHERE id=?`, attachments, usage, id)
	return err
}

// PendingVideoMessages returns AI messages whose usage marks a video job
// still pending (used to resume polling after a restart).
func (d *DB) PendingVideoMessages() ([]*AIMessage, error) {
	rows, err := d.Query(`SELECT id, thread_id, parent_id, role, content, reasoning, attachments, usage_json, hidden, created_at
		FROM ai_messages WHERE usage_json LIKE '%"video"%' AND usage_json LIKE '%"pending"%' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AIMessage
	for rows.Next() {
		var m AIMessage
		var h int
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.ParentID, &m.Role, &m.Content, &m.Reasoning, &m.Attachments, &m.Usage, &h, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Hidden = h == 1
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (d *DB) DeleteAIConfig(id int64) error {
	_, err := d.Exec(`DELETE FROM ai_configs WHERE id = ?`, id)
	return err
}

// ---------------- AI threads ----------------

type AIThread struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	ConfigID  int64  `json:"configId"`
	Model     string `json:"model"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	SearchOn  bool   `json:"searchOn"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	MsgCount  int    `json:"msgCount"`
}

func (d *DB) ListAIThreads(includeArchived bool) ([]*AIThread, error) {
	q := `SELECT t.id, t.title, t.config_id, t.model, t.pinned, t.archived, t.search_on, t.created_at, t.updated_at,
		(SELECT COUNT(*) FROM ai_messages m WHERE m.thread_id = t.id AND m.hidden = 0) FROM ai_threads t`
	if !includeArchived {
		q += ` WHERE t.archived = 0`
	}
	q += ` ORDER BY t.pinned DESC, t.updated_at DESC`
	rows, err := d.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AIThread
	for rows.Next() {
		var t AIThread
		var pn, ar, so int
		if err := rows.Scan(&t.ID, &t.Title, &t.ConfigID, &t.Model, &pn, &ar, &so,
			&t.CreatedAt, &t.UpdatedAt, &t.MsgCount); err != nil {
			return nil, err
		}
		t.Pinned, t.Archived, t.SearchOn = pn == 1, ar == 1, so == 1
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (d *DB) GetAIThread(id int64) (*AIThread, error) {
	var t AIThread
	var pn, ar, so int
	err := d.QueryRow(`SELECT id, title, config_id, model, pinned, archived, search_on, created_at, updated_at
		FROM ai_threads WHERE id = ?`, id).
		Scan(&t.ID, &t.Title, &t.ConfigID, &t.Model, &pn, &ar, &so, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Pinned, t.Archived, t.SearchOn = pn == 1, ar == 1, so == 1
	return &t, nil
}

// SearchAIThreads finds threads whose title or any visible message content
// matches the query substring.
func (d *DB) SearchAIThreads(query string) ([]*AIThread, error) {
	like := "%" + query + "%"
	rows, err := d.Query(`SELECT t.id, t.title, t.config_id, t.model, t.pinned, t.archived, t.search_on, t.created_at, t.updated_at,
		(SELECT COUNT(*) FROM ai_messages m WHERE m.thread_id = t.id AND m.hidden = 0) FROM ai_threads t
		WHERE t.title LIKE ? OR EXISTS (SELECT 1 FROM ai_messages m WHERE m.thread_id = t.id AND m.hidden = 0 AND m.content LIKE ?)
		ORDER BY t.pinned DESC, t.updated_at DESC LIMIT 100`, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AIThread
	for rows.Next() {
		var t AIThread
		var pn, ar, so int
		if err := rows.Scan(&t.ID, &t.Title, &t.ConfigID, &t.Model, &pn, &ar, &so,
			&t.CreatedAt, &t.UpdatedAt, &t.MsgCount); err != nil {
			return nil, err
		}
		t.Pinned, t.Archived, t.SearchOn = pn == 1, ar == 1, so == 1
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (d *DB) CreateAIThread(configID int64, model string) (int64, error) {
	now := time.Now().Unix()
	// Web search defaults ON for new threads (per user request); it can be
	// turned off per thread from the chat page.
	r, err := d.Exec(`INSERT INTO ai_threads (title, config_id, model, search_on, created_at, updated_at) VALUES ('新会话', ?, ?, 1, ?, ?)`,
		configID, model, now, now)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) UpdateAIThread(id int64, fields map[string]interface{}) error {
	allowed := map[string]bool{"title": true, "config_id": true, "model": true,
		"pinned": true, "archived": true, "search_on": true}
	q := `UPDATE ai_threads SET updated_at = ?`
	args := []interface{}{time.Now().Unix()}
	for k, v := range fields {
		if !allowed[k] {
			return fmt.Errorf("bad field %s", k)
		}
		q += `, ` + k + ` = ?`
		args = append(args, v)
	}
	q += ` WHERE id = ?`
	args = append(args, id)
	_, err := d.Exec(q, args...)
	return err
}

func (d *DB) TouchAIThread(id int64) {
	_, _ = d.Exec(`UPDATE ai_threads SET updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
}

func (d *DB) SetAIThreadTitle(id int64, title string) {
	_, _ = d.Exec(`UPDATE ai_threads SET title = ? WHERE id = ?`, title, id)
}

func (d *DB) DeleteAIThread(id int64) error {
	if _, err := d.Exec(`DELETE FROM ai_messages WHERE thread_id = ?`, id); err != nil {
		return err
	}
	_, err := d.Exec(`DELETE FROM ai_threads WHERE id = ?`, id)
	return err
}

// ---------------- AI messages (tree) ----------------

type AIMessage struct {
	ID          int64  `json:"id"`
	ThreadID    int64  `json:"threadId"`
	ParentID    int64  `json:"parentId"`
	Role        string `json:"role"` // user | ai
	Content     string `json:"content"`
	Reasoning   string `json:"reasoning,omitempty"`
	Attachments string `json:"attachments"` // JSON array
	Usage       string `json:"usageJson,omitempty"`
	Hidden      bool   `json:"hidden"`
	CreatedAt   int64  `json:"createdAt"`
	// computed by tree walkers
	BranchIndex int `json:"branchIndex,omitempty"` // which sibling (0-based)
	BranchCount int `json:"branchCount,omitempty"` // total siblings incl. self
}

func (d *DB) CreateAIMessage(m *AIMessage) (int64, error) {
	h := 0
	if m.Hidden {
		h = 1
	}
	r, err := d.Exec(`INSERT INTO ai_messages (thread_id, parent_id, role, content, reasoning, attachments, usage_json, hidden, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ThreadID, m.ParentID, m.Role, m.Content, m.Reasoning, m.Attachments, m.Usage, h, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) GetAIMessage(id int64) (*AIMessage, error) {
	var m AIMessage
	var h int
	err := d.QueryRow(`SELECT id, thread_id, parent_id, role, content, reasoning, attachments, usage_json, hidden, created_at
		FROM ai_messages WHERE id = ?`, id).
		Scan(&m.ID, &m.ThreadID, &m.ParentID, &m.Role, &m.Content, &m.Reasoning, &m.Attachments, &m.Usage, &h, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	m.Hidden = h == 1
	return &m, nil
}

// ThreadMessages returns every message of a thread ordered by id.
func (d *DB) ThreadMessages(threadID int64) ([]*AIMessage, error) {
	rows, err := d.Query(`SELECT id, thread_id, parent_id, role, content, reasoning, attachments, usage_json, hidden, created_at
		FROM ai_messages WHERE thread_id = ? ORDER BY id`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AIMessage
	for rows.Next() {
		var m AIMessage
		var h int
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.ParentID, &m.Role, &m.Content, &m.Reasoning, &m.Attachments, &m.Usage, &h, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Hidden = h == 1
		out = append(out, &m)
	}
	return out, rows.Err()
}

// ThreadPath returns the visible chain root..leafId by walking parent links.
func (d *DB) ThreadPath(threadID, leafID int64) ([]*AIMessage, error) {
	all, err := d.ThreadMessages(threadID)
	if err != nil {
		return nil, err
	}
	byID := map[int64]*AIMessage{}
	for _, m := range all {
		byID[m.ID] = m
	}
	var rev []*AIMessage
	cur := byID[leafID]
	for cur != nil {
		rev = append(rev, cur)
		if cur.ParentID == 0 {
			break
		}
		cur = byID[cur.ParentID]
	}
	out := make([]*AIMessage, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out, nil
}

// ThreadLeaf returns the newest visible message id of a thread (0 if empty).
func (d *DB) ThreadLeaf(threadID int64) (int64, error) {
	var id int64
	err := d.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM ai_messages WHERE thread_id = ? AND hidden = 0`, threadID).Scan(&id)
	return id, err
}

// AIStats returns aggregate counts/sizes for the data-management panel.
type AIStats struct {
	Threads     int   `json:"threads"`
	Messages    int   `json:"messages"`
	TextBytes   int64 `json:"textBytes"`
	ReasonBytes int64 `json:"reasonBytes"`
}

func (d *DB) AIStats() (*AIStats, error) {
	var st AIStats
	err := d.QueryRow(`SELECT
		(SELECT COUNT(*) FROM ai_threads),
		(SELECT COUNT(*) FROM ai_messages),
		(SELECT COALESCE(SUM(LENGTH(content)),0) FROM ai_messages),
		(SELECT COALESCE(SUM(LENGTH(reasoning)),0) FROM ai_messages)`).Scan(
		&st.Threads, &st.Messages, &st.TextBytes, &st.ReasonBytes)
	return &st, err
}

// AIClearType clears one attachment/data type across all messages.
func (d *DB) AIClearType(kind string) error {
	switch kind {
	case "reasoning":
		_, err := d.Exec(`UPDATE ai_messages SET reasoning = ''`)
		return err
	case "text":
		_, err := d.Exec(`DELETE FROM ai_messages`)
		return err
	}
	return fmt.Errorf("unknown type %s", kind)
}
