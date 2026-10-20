// Package db wraps SQLite (modernc.org/sqlite, pure Go) for SecretDock.
package db

import (
	"crypto/rand"
	"encoding/base64"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Merchant is a named group of secrets (e.g. "Cloudflare").
type Merchant struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Aliases   []string `json:"aliases"`
	LogoURL   string   `json:"logoUrl"`
	LogoSVG   string   `json:"logoSvg"`
	CreatedAt int64    `json:"createdAt"`
	Count     int      `json:"count"` // number of secrets (filled by list queries)
	IsPinned  bool     `json:"isPinned"`
}

// Secret is one stored credential. Value is plaintext in memory;
// it is encrypted at rest via crypto.Encrypt before insert.
type Secret struct {
	ID         int64  `json:"id"`
	MerchantID int64  `json:"merchantId"`
	Kind       string `json:"kind"` // "secret" | "account"
	Label      string `json:"label"`
	Username   string `json:"username"` // account kind
	Value      string `json:"value"`    // plaintext in memory, ciphertext in DB
	Note       string `json:"note"`
	ExpiresAt  int64  `json:"expiresAt"` // unix seconds, 0 = none
	CreatedAt  int64  `json:"createdAt"`
}

// AuditEntry is one security log row.
type AuditEntry struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
	CreatedAt int64  `json:"createdAt"`
}

// DB is a thin wrapper around *sql.DB.
type DB struct {
	*sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS merchants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  aliases TEXT NOT NULL DEFAULT '[]',
  logo_url TEXT NOT NULL DEFAULT '',
  logo_svg TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS secrets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  merchant_id INTEGER NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  kind TEXT NOT NULL DEFAULT 'secret',
  label TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  expires_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_secrets_merchant ON secrets(merchant_id);
CREATE INDEX IF NOT EXISTS idx_secrets_expires ON secrets(expires_at);
CREATE TABLE IF NOT EXISTS custom_labels (
  label TEXT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
CREATE TABLE IF NOT EXISTS tg_messages (
  chat_id INTEGER NOT NULL,
  msg_id INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (chat_id, msg_id)
);
CREATE TABLE IF NOT EXISTS image_categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS images (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category_id INTEGER NOT NULL REFERENCES image_categories(id) ON DELETE CASCADE,
  caption TEXT NOT NULL DEFAULT '',
  file_path TEXT NOT NULL DEFAULT '',
  thumb_path TEXT NOT NULL DEFAULT '',
  file_size INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_images_category ON images(category_id);
CREATE TABLE IF NOT EXISTS notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL DEFAULT '',
  content_enc TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS link_categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category_id INTEGER NOT NULL REFERENCES link_categories(id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_links_category ON links(category_id);
CREATE TABLE IF NOT EXISTS scripts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  content_enc TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
`

// Open opens (creating if needed) the SQLite database and applies the schema.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := sqldb.Exec(schema); err != nil {
		sqldb.Close()
		return nil, err
	}
	// migrate: images thumbnails s/m/l
	for _, col := range []string{"thumb_s", "thumb_m", "thumb_l"} {
		var n int
		_ = sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('images') WHERE name = ?`, col).Scan(&n)
		if n == 0 {
			if _, err := sqldb.Exec(`ALTER TABLE images ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`); err != nil {
				sqldb.Close()
				return nil, err
			}
		}
	}
	// links auto_name column
	{
		var n int
		_ = sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('links') WHERE name = 'auto_name'`).Scan(&n)
		if n == 0 {
			if _, err := sqldb.Exec(`ALTER TABLE links ADD COLUMN auto_name INTEGER NOT NULL DEFAULT 0`); err != nil {
				sqldb.Close()
				return nil, err
			}
		}
	}
	// pin columns
	for _, t := range []string{"merchants", "scripts", "links", "notes"} {
		var n int
		_ = sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('` + t + `') WHERE name = 'is_pinned'`).Scan(&n)
		if n == 0 {
			if _, err := sqldb.Exec(`ALTER TABLE ` + t + ` ADD COLUMN is_pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
				sqldb.Close()
				return nil, err
			}
		}
	}
	// file categories + files
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS file_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		sort INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER NOT NULL,
		filename TEXT NOT NULL,
		orig_name TEXT NOT NULL DEFAULT '',
		file_path TEXT NOT NULL,
		file_size INTEGER NOT NULL DEFAULT 0,
		mime TEXT NOT NULL DEFAULT '',
		is_text INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		FOREIGN KEY(category_id) REFERENCES file_categories(id) ON DELETE CASCADE
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`INSERT OR IGNORE INTO file_categories (name, sort, created_at) VALUES ('未分类', 0, ?)`, time.Now().Unix()); err != nil {
		sqldb.Close()
		return nil, err
	}
	// node categories + nodes (订阅节点)
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS node_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		sort INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL,
		kind TEXT NOT NULL DEFAULT 'sub',
		proto TEXT NOT NULL DEFAULT '',
		host TEXT NOT NULL DEFAULT '',
		port INTEGER NOT NULL DEFAULT 0,
		last_status TEXT NOT NULL DEFAULT '',
		last_latency INTEGER NOT NULL DEFAULT 0,
		last_check_at INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		FOREIGN KEY(category_id) REFERENCES node_categories(id) ON DELETE CASCADE
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`INSERT OR IGNORE INTO node_categories (name, sort, created_at) VALUES ('未分类', 0, ?)`, time.Now().Unix()); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS ssh_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		sort INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS ssh_hosts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		host TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 22,
		username TEXT NOT NULL DEFAULT 'root',
		password_enc TEXT NOT NULL DEFAULT '',
		country TEXT NOT NULL DEFAULT '',
		last_status TEXT NOT NULL DEFAULT '',
		last_latency INTEGER NOT NULL DEFAULT 0,
		last_check_at INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		FOREIGN KEY(category_id) REFERENCES ssh_categories(id) ON DELETE CASCADE
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	// ssh_hosts 加 region/os_id/os_version 列
	for _, col := range []string{"region", "os_id", "os_version"} {
		var n int
		_ = sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('ssh_hosts') WHERE name = '`+col+`'`).Scan(&n)
		if n == 0 {
			if _, err := sqldb.Exec(`ALTER TABLE ssh_hosts ADD COLUMN `+col+` TEXT NOT NULL DEFAULT ''`); err != nil {
				sqldb.Close()
				return nil, err
			}
		}
	}
	// images 加 orig_name 列
	{
		var n int
		_ = sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('images') WHERE name = 'orig_name'`).Scan(&n)
		if n == 0 {
			if _, err := sqldb.Exec(`ALTER TABLE images ADD COLUMN orig_name TEXT NOT NULL DEFAULT ''`); err != nil {
				sqldb.Close()
				return nil, err
			}
		}
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		pass_hash TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS share_tokens (
		token TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		ref_id INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`INSERT OR IGNORE INTO ssh_categories (name, sort, created_at) VALUES ('未分类', 0, ?)`, time.Now().Unix()); err != nil {
		sqldb.Close()
		return nil, err
	}
	// script categories + note categories
	for _, tbl := range []string{"script_categories", "note_categories"} {
		if _, err := sqldb.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			sort INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`, tbl)); err != nil {
			sqldb.Close()
			return nil, err
		}
		if _, err := sqldb.Exec(fmt.Sprintf(`INSERT OR IGNORE INTO %s (name, sort, created_at) VALUES ('未分类', 0, ?)`, tbl), time.Now().Unix()); err != nil {
			sqldb.Close()
			return nil, err
		}
	}
	// scripts/note 加 category_id 列
	for _, tbl := range []string{"scripts", "notes"} {
		var n int
		_ = sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('`+tbl+`') WHERE name = 'category_id'`).Scan(&n)
		if n == 0 {
			if _, err := sqldb.Exec(`ALTER TABLE `+tbl+` ADD COLUMN category_id INTEGER NOT NULL DEFAULT 0`); err != nil {
				sqldb.Close()
				return nil, err
			}
		}
	}
	// 旧数据 category_id=0 的，指向未分类
	for _, tbl := range []string{"scripts", "notes"} {
		catTbl := strings.Replace(tbl, "s", "_categories", 1)
		if tbl == "notes" {
			catTbl = "note_categories"
		} else {
			catTbl = "script_categories"
		}
		var defID int64
		_ = sqldb.QueryRow(fmt.Sprintf(`SELECT id FROM %s WHERE name = '未分类'`, catTbl)).Scan(&defID)
		if defID != 0 {
			sqldb.Exec(fmt.Sprintf(`UPDATE %s SET category_id = ? WHERE category_id = 0`, tbl), defID)
		}
	}
	// reminder categories + reminders
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS reminder_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		sort INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		remind_at INTEGER NOT NULL,
		repeat_type TEXT NOT NULL DEFAULT 'none',
		repeat_detail TEXT NOT NULL DEFAULT '',
		times INTEGER NOT NULL DEFAULT 3,
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL,
		FOREIGN KEY(category_id) REFERENCES reminder_categories(id) ON DELETE CASCADE
	)`); err != nil {
		sqldb.Close()
		return nil, err
	}
	if _, err := sqldb.Exec(`INSERT OR IGNORE INTO reminder_categories (name, sort, created_at) VALUES ('未分类', 0, ?)`, time.Now().Unix()); err != nil {
		sqldb.Close()
		return nil, err
	}
	// ensure default image category
	if _, err := sqldb.Exec(`INSERT OR IGNORE INTO image_categories (name, sort, created_at) VALUES ('未分类', 0, ?)`, now()); err != nil {
		sqldb.Close()
		return nil, err
	}
	// ensure default link category
	if _, err := sqldb.Exec(`INSERT OR IGNORE INTO link_categories (name, sort, created_at) VALUES ('未分类', 0, ?)`, now()); err != nil {
		sqldb.Close()
		return nil, err
	}
	// AI module tables (configs / threads / tree messages)
	if err := ensureAITables(&DB{sqldb}); err != nil {
		sqldb.Close()
		return nil, err
	}
	// migrate: scripts used to have merchant_id FK; scripts are top-level now
	var fkCount int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_list('scripts')`).Scan(&fkCount); err == nil && fkCount > 0 {
		if _, err := sqldb.Exec(`DROP TABLE scripts`); err != nil {
			sqldb.Close()
			return nil, err
		}
		if _, err := sqldb.Exec(schema); err != nil {
			sqldb.Close()
			return nil, err
		}
	}
	return &DB{sqldb}, nil
}

func now() int64 { return time.Now().Unix() }

// ---- merchants ----



// ListMerchants returns all merchants with their secret counts, ordered by name.
func (d *DB) ListMerchants() ([]Merchant, error) {
	rows, err := d.Query(`SELECT m.id, m.name, m.aliases, m.logo_url, m.logo_svg, m.created_at, m.is_pinned,
		(SELECT COUNT(*) FROM secrets s WHERE s.merchant_id = m.id)
		FROM merchants m ORDER BY m.is_pinned DESC, m.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Merchant
	for rows.Next() {
		var m Merchant
		var aliases string
		if err := rows.Scan(&m.ID, &m.Name, &aliases, &m.LogoURL, &m.LogoSVG, &m.CreatedAt, &m.IsPinned, &m.Count); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(aliases), &m.Aliases)
		if m.Aliases == nil {
			m.Aliases = []string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMerchant fetches one merchant by id.
func (d *DB) GetMerchant(id int64) (Merchant, error) {
	var m Merchant
	var aliases string
	err := d.QueryRow(`SELECT id, name, aliases, logo_url, logo_svg, created_at, is_pinned FROM merchants WHERE id = ?`, id).
		Scan(&m.ID, &m.Name, &aliases, &m.LogoURL, &m.LogoSVG, &m.CreatedAt, &m.IsPinned)
	if err != nil {
		return m, err
	}
	_ = json.Unmarshal([]byte(aliases), &m.Aliases)
	if m.Aliases == nil {
		m.Aliases = []string{}
	}
	return m, nil
}

// CreateMerchant inserts a merchant and returns its id.
func (d *DB) CreateMerchant(name string, aliases []string, logoURL, logoSVG string) (int64, error) {
	if aliases == nil {
		aliases = []string{}
	}
	ab, _ := json.Marshal(aliases)
	res, err := d.Exec(`INSERT INTO merchants (name, aliases, logo_url, logo_svg, created_at) VALUES (?, ?, ?, ?, ?)`,
		name, string(ab), logoURL, logoSVG, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateMerchant updates name/aliases/logo.
func (d *DB) UpdateMerchant(id int64, name string, aliases []string, logoURL, logoSVG string) error {
	if aliases == nil {
		aliases = []string{}
	}
	ab, _ := json.Marshal(aliases)
	_, err := d.Exec(`UPDATE merchants SET name = ?, aliases = ?, logo_url = ?, logo_svg = ? WHERE id = ?`,
		name, string(ab), logoURL, logoSVG, id)
	return err
}

// DeleteMerchant removes a merchant and its secrets (FK cascade).
func (d *DB) DeleteMerchant(id int64) error {
	_, err := d.Exec(`DELETE FROM merchants WHERE id = ?`, id)
	return err
}

// ---- secrets ----

// CreateSecret inserts a secret; value must already be encrypted.
func (d *DB) CreateSecret(s Secret) (int64, error) {
	res, err := d.Exec(`INSERT INTO secrets (merchant_id, kind, label, username, value, note, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.MerchantID, s.Kind, s.Label, s.Username, s.Value, s.Note, s.ExpiresAt, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetSecret fetches one secret by id.
func (d *DB) GetSecret(id int64) (Secret, error) {
	var s Secret
	err := d.QueryRow(`SELECT id, merchant_id, kind, label, username, value, note, expires_at, created_at
		FROM secrets WHERE id = ?`, id).
		Scan(&s.ID, &s.MerchantID, &s.Kind, &s.Label, &s.Username, &s.Value, &s.Note, &s.ExpiresAt, &s.CreatedAt)
	return s, err
}

// ListSecrets returns secrets for a merchant, newest first.
func (d *DB) ListSecrets(merchantID int64) ([]Secret, error) {
	rows, err := d.Query(`SELECT id, merchant_id, kind, label, username, value, note, expires_at, created_at
		FROM secrets WHERE merchant_id = ? ORDER BY created_at DESC, id DESC`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.MerchantID, &s.Kind, &s.Label, &s.Username, &s.Value, &s.Note, &s.ExpiresAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateSecret updates label/username/value(note)/expires of a secret; value must be encrypted.
func (d *DB) UpdateSecret(s Secret) error {
	_, err := d.Exec(`UPDATE secrets SET kind = ?, label = ?, username = ?, value = ?, note = ?, expires_at = ? WHERE id = ?`,
		s.Kind, s.Label, s.Username, s.Value, s.Note, s.ExpiresAt, s.ID)
	return err
}

// DeleteSecret removes one secret.
func (d *DB) DeleteSecret(id int64) error {
	_, err := d.Exec(`DELETE FROM secrets WHERE id = ?`, id)
	return err
}

// SearchSecrets searches label/username/note/merchant name (LIKE, case-insensitive).
func (d *DB) SearchSecrets(q string) ([]Secret, error) {
	like := "%" + q + "%"
	rows, err := d.Query(`SELECT s.id, s.merchant_id, s.kind, s.label, s.username, s.value, s.note, s.expires_at, s.created_at
		FROM secrets s LEFT JOIN merchants m ON m.id = s.merchant_id
		WHERE s.label LIKE ? ESCAPE '\' OR s.username LIKE ? ESCAPE '\' OR s.note LIKE ? ESCAPE '\' OR m.name LIKE ? ESCAPE '\'
		ORDER BY s.created_at DESC, s.id DESC LIMIT 200`, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.MerchantID, &s.Kind, &s.Label, &s.Username, &s.Value, &s.Note, &s.ExpiresAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ExpiringSecrets returns secrets expiring within days (expires_at > 0).
func (d *DB) ExpiringSecrets(withinDays int) ([]Secret, error) {
	cutoff := now() + int64(withinDays)*86400
	rows, err := d.Query(`SELECT id, merchant_id, kind, label, username, value, note, expires_at, created_at
		FROM secrets WHERE expires_at > 0 AND expires_at <= ? ORDER BY expires_at ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.MerchantID, &s.Kind, &s.Label, &s.Username, &s.Value, &s.Note, &s.ExpiresAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---- custom labels ----

// CustomLabels returns user-taught labels.
func (d *DB) CustomLabels() ([]string, error) {
	rows, err := d.Query(`SELECT label FROM custom_labels ORDER BY label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// AddCustomLabel remembers a user-corrected label (idempotent).
func (d *DB) AddCustomLabel(label string) error {
	if label == "" {
		return nil
	}
	_, err := d.Exec(`INSERT OR IGNORE INTO custom_labels (label) VALUES (?)`, label)
	return err
}

// ---- settings ----

// GetSetting returns a setting value or "" if unset.
func (d *DB) GetSetting(key string) string {
	var v string
	_ = d.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	return v
}

// SetSetting upserts a setting.
func (d *DB) ListSettings() (map[string]string, error) {
	rows, err := d.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ---- audit ----

// Audit writes a security log row.
// ClearData 清空指定分类的数据
func (d *DB) ClearData(key string) error {
	switch key {
	case "merchants":
		_, err := d.Exec(`DELETE FROM secrets`)
		if err != nil { return err }
		_, err = d.Exec(`DELETE FROM merchants`)
		return err
	case "scripts":
		_, err := d.Exec(`DELETE FROM scripts`)
		return err
	case "links":
		_, err := d.Exec(`DELETE FROM links`)
		return err
	case "notes":
		_, err := d.Exec(`DELETE FROM notes`)
		return err
	case "images":
		_, err := d.Exec(`DELETE FROM images`)
		return err
	case "files":
		_, err := d.Exec(`DELETE FROM files`)
		return err
	case "nodes":
		_, err := d.Exec(`DELETE FROM nodes`)
		return err
	case "reminders":
		_, err := d.Exec(`DELETE FROM reminders`)
		return err
	case "ssh":
		_, err := d.Exec(`DELETE FROM ssh_hosts`)
		return err
	case "audit":
		_, err := d.Exec(`DELETE FROM audit_log`)
		return err
	case "tg":
		// 清 TG 相关设置
		tgKeys := []string{"bot_token_enc", "admin_chat_id", "query_pwd_hash", "query_pwd_max_tries", "auto_delete_minutes", "tg_idle_wipe_minutes", "tg_ban_count", "tg_banned_until", "reminder_default_times"}
		for _, k := range tgKeys {
			if _, err := d.Exec(`DELETE FROM settings WHERE key = ?`, k); err != nil {
				return err
			}
		}
		_, err := d.Exec(`DELETE FROM tg_messages`)
		return err
	case "all":
		tables := []string{"secrets", "merchants", "scripts", "links", "notes", "images", "files", "nodes", "reminders", "ssh_hosts", "tg_messages"}
		// TG 设置也清
		tgKeys := []string{"bot_token_enc", "admin_chat_id", "query_pwd_hash", "query_pwd_max_tries", "auto_delete_minutes", "tg_idle_wipe_minutes", "tg_ban_count", "tg_banned_until", "reminder_default_times"}
		for _, k := range tgKeys {
			d.Exec(`DELETE FROM settings WHERE key = ?`, k)
		}
		for _, t := range tables {
			if _, err := d.Exec(`DELETE FROM ` + t); err != nil {
				return err
			}
		}
		// 设置保留（不清空，避免把自己锁外面）
		return nil
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func (d *DB) CreateShareToken(kind string, refID int64) (string, error) {
	token := randomToken(16)
	_, err := d.Exec(`INSERT INTO share_tokens (token, kind, ref_id, created_at) VALUES (?, ?, ?, ?)`,
		token, kind, refID, time.Now().Unix())
	return token, err
}

func (d *DB) GetShareToken(token string) (string, int64, error) {
	var kind string
	var refID int64
	err := d.QueryRow(`SELECT kind, ref_id FROM share_tokens WHERE token = ?`, token).Scan(&kind, &refID)
	return kind, refID, err
}

func (d *DB) DeleteShareToken(token string) error {
	_, err := d.Exec(`DELETE FROM share_tokens WHERE token = ?`, token)
	return err
}

func (d *DB) HasUser() bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n > 0
}

func (d *DB) CreateUser(username, passHash string) error {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if n > 0 {
		return fmt.Errorf("user already exists")
	}
	_, err := d.Exec(`INSERT INTO users (username, pass_hash, created_at) VALUES (?, ?, ?)`,
		username, passHash, time.Now().Unix())
	return err
}

func (d *DB) GetUser() (username, passHash string, err error) {
	err = d.QueryRow(`SELECT username, pass_hash FROM users LIMIT 1`).Scan(&username, &passHash)
	return
}

func (d *DB) UpdatePassword(passHash string) error {
	_, err := d.Exec(`UPDATE users SET pass_hash = ?`, passHash)
	return err
}

func (d *DB) Audit(kind, detail, ip string) {
	_, _ = d.Exec(`INSERT INTO audit_log (kind, detail, ip, created_at) VALUES (?, ?, ?, ?)`,
		kind, detail, ip, now())
}

// AuditList returns recent audit rows, newest first.
func (d *DB) AuditList(limit int) ([]AuditEntry, error) {
	rows, err := d.Query(`SELECT id, kind, detail, ip, created_at FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(&a.ID, &a.Kind, &a.Detail, &a.IP, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---- TG message tracking (for idle auto-wipe) ----

// TrackTgMessage records a message ID (user or bot) for later deletion.
func (d *DB) TrackTgMessage(chatID, msgID int64) {
	_, _ = d.Exec(`INSERT OR IGNORE INTO tg_messages (chat_id, msg_id, created_at) VALUES (?, ?, ?)`,
		chatID, msgID, time.Now().Unix())
}

// UntrackTgMessage removes a single message ID from tracking.
func (d *DB) UntrackTgMessage(chatID, msgID int64) {
	_, _ = d.Exec(`DELETE FROM tg_messages WHERE chat_id = ? AND msg_id = ?`, chatID, msgID)
}

// ListTgMessages returns all tracked message IDs for a chat.
func (d *DB) ListTgMessages(chatID int64) []int64 {
	rows, err := d.Query(`SELECT msg_id FROM tg_messages WHERE chat_id = ? ORDER BY msg_id`, chatID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// ListTgChats returns chat IDs that have tracked messages.
func (d *DB) ListTgChats() []int64 {
	rows, err := d.Query(`SELECT DISTINCT chat_id FROM tg_messages`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// ClearTgMessages removes all tracking rows for a chat.
func (d *DB) ClearTgMessages(chatID int64) {
	_, _ = d.Exec(`DELETE FROM tg_messages WHERE chat_id = ?`, chatID)
}

// ---- userscripts ----

// Script is a stored userscript (top-level, not tied to a merchant).
type Script struct {
	ID          int64
	Name        string
	Version     string
	Description string
	Author      string
	ContentEnc  string
	CategoryID  int64
	CreatedAt   int64
	UpdatedAt   int64
	IsPinned    bool
}

func (d *DB) CreateScript(name, version, desc, author, contentEnc string, catID int64) (int64, error) {
	now := time.Now().Unix()
	r, err := d.Exec(`INSERT INTO scripts (name, version, description, author, content_enc, category_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, name, version, desc, author, contentEnc, catID, now, now)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func scanScript(rows *sql.Rows) (*Script, error) {
	var sc Script
	err := rows.Scan(&sc.ID, &sc.Name, &sc.Version, &sc.Description, &sc.Author, &sc.ContentEnc, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt, &sc.IsPinned)
	return &sc, err
}

func (d *DB) ListScripts() ([]*Script, error) {
	rows, err := d.Query(`SELECT id, name, version, description, author, content_enc, category_id, created_at, updated_at, is_pinned
		FROM scripts ORDER BY is_pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Script
	for rows.Next() {
		sc, err := scanScript(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, nil
}

func (d *DB) GetScript(id int64) (*Script, error) {
	row := d.QueryRow(`SELECT id, name, version, description, author, content_enc, category_id, created_at, updated_at, is_pinned
		FROM scripts WHERE id = ?`, id)
	var sc Script
	err := row.Scan(&sc.ID, &sc.Name, &sc.Version, &sc.Description, &sc.Author, &sc.ContentEnc, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt, &sc.IsPinned)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (d *DB) DeleteScript(id int64) error {
	_, err := d.Exec(`DELETE FROM scripts WHERE id = ?`, id)
	return err
}


// ---- links ----

type LinkCategory struct {
	ID        int64
	Name      string
	Sort      int
	CreatedAt int64
}

type Link struct {
	ID         int64
	CategoryID int64
	Name       string
	URL        string
	CreatedAt  int64
	UpdatedAt  int64
	IsPinned   bool
	AutoName   bool
}

func (d *DB) ListLinkCategories() ([]*LinkCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM link_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*LinkCategory
	for rows.Next() {
		var c LinkCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, nil
}

func (d *DB) FindOrCreateLinkCategory(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未分类"
	}
	var id int64
	err := d.QueryRow(`SELECT id FROM link_categories WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	now := time.Now().Unix()
	r, err := d.Exec(`INSERT INTO link_categories (name, sort, created_at) VALUES (?, (SELECT COALESCE(MAX(sort),0)+1 FROM link_categories), ?)`, name, now)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) CreateLinkCategory(name string) (int64, error) {
	return d.FindOrCreateLinkCategory(name)
}

func (d *DB) RenameLinkCategory(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty name")
	}
	_, err := d.Exec(`UPDATE link_categories SET name = ? WHERE id = ?`, name, id)
	return err
}

// DeleteLinkCategory moves its links to 未分类, then deletes it.
func (d *DB) DeleteLinkCategory(id int64) error {
	defID, err := d.FindOrCreateLinkCategory("未分类")
	if err != nil {
		return err
	}
	if defID == id {
		return fmt.Errorf("cannot delete default category")
	}
	if _, err := d.Exec(`UPDATE links SET category_id = ? WHERE category_id = ?`, defID, id); err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM link_categories WHERE id = ?`, id)
	return err
}

func (d *DB) CreateLink(catID int64, name, url string, autoName int) (int64, error) {
	now := time.Now().Unix()
	r, err := d.Exec(`INSERT INTO links (category_id, name, url, auto_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		catID, name, url, autoName, now, now)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListLinks(catID int64) ([]*Link, error) {
	rows, err := d.Query(`SELECT id, category_id, name, url, created_at, updated_at, is_pinned, auto_name FROM links
		WHERE category_id = ? ORDER BY is_pinned DESC, updated_at DESC`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.CategoryID, &l.Name, &l.URL, &l.CreatedAt, &l.UpdatedAt, &l.IsPinned, &l.AutoName); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, nil
}

func (d *DB) GetLink(id int64) (*Link, error) {
	var l Link
	err := d.QueryRow(`SELECT id, category_id, name, url, created_at, updated_at, is_pinned, auto_name FROM links WHERE id = ?`, id).
		Scan(&l.ID, &l.CategoryID, &l.Name, &l.URL, &l.CreatedAt, &l.UpdatedAt, &l.IsPinned, &l.AutoName)
	return &l, err
}

func (d *DB) UpdateLink(id, catID int64, name, url string) error {
	_, err := d.Exec(`UPDATE links SET category_id = ?, name = ?, url = ?, updated_at = ? WHERE id = ?`,
		catID, name, url, time.Now().Unix(), id)
	return err
}

func (d *DB) DeleteLink(id int64) error {
	_, err := d.Exec(`DELETE FROM links WHERE id = ?`, id)
	return err
}

func (d *DB) MoveLinks(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE links SET category_id = ?, updated_at = ? WHERE id IN (`
	args := []interface{}{catID, time.Now().Unix()}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// ---- notes ----

type Note struct {
	ID         int64
	Title      string
	ContentEnc string
	CategoryID int64
	CreatedAt  int64
	UpdatedAt  int64
	IsPinned   bool
}

func (d *DB) CreateNote(title, contentEnc string, catID int64) (int64, error) {
	now := time.Now().Unix()
	r, err := d.Exec(`INSERT INTO notes (title, content_enc, category_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		title, contentEnc, catID, now, now)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListNotes() ([]*Note, error) {
	return d.ListNotesByCategory(0)
}

func (d *DB) ListNotesByCategory(catID int64) ([]*Note, error) {
	q := `SELECT id, title, content_enc, category_id, created_at, updated_at, is_pinned FROM notes`
	var args []interface{}
	if catID != 0 {
		q += ` WHERE category_id = ?`
		args = append(args, catID)
	}
	q += ` ORDER BY is_pinned DESC, updated_at DESC`
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.ContentEnc, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt, &n.IsPinned); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, nil
}

func (d *DB) GetNote(id int64) (*Note, error) {
	var n Note
	err := d.QueryRow(`SELECT id, title, content_enc, category_id, created_at, updated_at FROM notes WHERE id = ?`, id).
		Scan(&n.ID, &n.Title, &n.ContentEnc, &n.CategoryID, &n.CreatedAt, &n.UpdatedAt)
	return &n, err
}

func (d *DB) UpdateNote(id int64, title, contentEnc string) error {
	_, err := d.Exec(`UPDATE notes SET title = ?, content_enc = ?, updated_at = ? WHERE id = ?`,
		title, contentEnc, time.Now().Unix(), id)
	return err
}

func (d *DB) DeleteNote(id int64) error {
	_, err := d.Exec(`DELETE FROM notes WHERE id = ?`, id)
	return err
}

// UpdateScript updates a script's metadata and content.
func (d *DB) UpdateScript(id int64, name, version, desc, author, contentEnc string) error {
	_, err := d.Exec(`UPDATE scripts SET name = ?, version = ?, description = ?, author = ?,
		content_enc = ?, updated_at = ? WHERE id = ?`,
		name, version, desc, author, contentEnc, time.Now().Unix(), id)
	return err
}

// ---- images ----

type ImageCategory struct {
	ID        int64
	Name      string
	Sort      int
	CreatedAt int64
}

type Image struct {
	ID         int64
	CategoryID int64
	Caption    string
	OrigName   string
	FilePath   string
	ThumbPath  string
	ThumbS     string
	ThumbM     string
	ThumbL     string
	FileSize   int64
	CreatedAt  int64
}

func (d *DB) ListImageCategories() ([]*ImageCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM image_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ImageCategory
	for rows.Next() {
		var c ImageCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, nil
}

func (d *DB) FindOrCreateImageCategory(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未分类"
	}
	var id int64
	err := d.QueryRow(`SELECT id FROM image_categories WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	now := time.Now().Unix()
	r, err := d.Exec(`INSERT INTO image_categories (name, sort, created_at) VALUES (?, (SELECT COALESCE(MAX(sort),0)+1 FROM image_categories), ?)`, name, now)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) CreateImageCategory(name string) (int64, error) {
	return d.FindOrCreateImageCategory(name)
}

func (d *DB) RenameImageCategory(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty name")
	}
	_, err := d.Exec(`UPDATE image_categories SET name = ? WHERE id = ?`, name, id)
	return err
}

func (d *DB) DeleteImageCategory(id int64) error {
	defID, err := d.FindOrCreateImageCategory("未分类")
	if err != nil {
		return err
	}
	if defID == id {
		return fmt.Errorf("cannot delete default category")
	}
	// delete files of moved images? No — move to default, keep files.
	if _, err := d.Exec(`UPDATE images SET category_id = ? WHERE category_id = ?`, defID, id); err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM image_categories WHERE id = ?`, id)
	return err
}

func (d *DB) CreateImage(catID int64, caption, origName, filePath, thumbS, thumbM, thumbL string, fileSize int64) (int64, error) {
	r, err := d.Exec(`INSERT INTO images (category_id, caption, orig_name, file_path, thumb_s, thumb_m, thumb_l, file_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, catID, caption, origName, filePath, thumbS, thumbM, thumbL, fileSize, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListImages(catID int64) ([]*Image, error) {
	rows, err := d.Query(`SELECT id, category_id, caption, orig_name, file_path, thumb_path, thumb_s, thumb_m, thumb_l, file_size, created_at
		FROM images WHERE category_id = ? ORDER BY created_at DESC`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Image
	for rows.Next() {
		var im Image
		if err := rows.Scan(&im.ID, &im.CategoryID, &im.Caption, &im.OrigName, &im.FilePath, &im.ThumbPath, &im.ThumbS, &im.ThumbM, &im.ThumbL, &im.FileSize, &im.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &im)
	}
	return out, nil
}

func (d *DB) GetImage(id int64) (*Image, error) {
	var im Image
	err := d.QueryRow(`SELECT id, category_id, caption, orig_name, file_path, thumb_path, thumb_s, thumb_m, thumb_l, file_size, created_at
		FROM images WHERE id = ?`, id).
		Scan(&im.ID, &im.CategoryID, &im.Caption, &im.OrigName, &im.FilePath, &im.ThumbPath, &im.ThumbS, &im.ThumbM, &im.ThumbL, &im.FileSize, &im.CreatedAt)
	return &im, err
}

func (d *DB) UpdateImage(id, catID int64, caption string) error {
	_, err := d.Exec(`UPDATE images SET category_id = ?, caption = ? WHERE id = ?`, catID, caption, id)
	return err
}

func (d *DB) DeleteImage(id int64) error {
	_, err := d.Exec(`DELETE FROM images WHERE id = ?`, id)
	return err
}

func (d *DB) MoveImages(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE images SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// ---- pin ----

func (d *DB) SetPinned(table string, id int64, pinned bool) error {
	v := 0
	if pinned {
		v = 1
	}
	allowed := map[string]bool{"merchants": true, "scripts": true, "links": true, "notes": true}
	if !allowed[table] {
		return fmt.Errorf("bad table")
	}
	_, err := d.Exec(`UPDATE `+table+` SET is_pinned = ? WHERE id = ?`, v, id)
	return err
}

// ---- file categories & files ----

type FileCategory struct {
	ID        int64
	Name      string
	Sort      int
	CreatedAt int64
}

type File struct {
	ID         int64
	CategoryID int64
	Filename   string
	OrigName   string
	FilePath   string
	FileSize   int64
	Mime       string
	IsText     bool
	CreatedAt  int64
}

func (d *DB) FindOrCreateFileCategory(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未分类"
	}
	var id int64
	err := d.QueryRow(`SELECT id FROM file_categories WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	var maxSort int
	_ = d.QueryRow(`SELECT COALESCE(MAX(sort), -1) FROM file_categories`).Scan(&maxSort)
	r, err := d.Exec(`INSERT INTO file_categories (name, sort, created_at) VALUES (?, ?, ?)`, name, maxSort+1, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListFileCategories() ([]FileCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM file_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileCategory
	for rows.Next() {
		var c FileCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) CreateFileCategory(name string) (int64, error) {
	return d.FindOrCreateFileCategory(name)
}

func (d *DB) RenameFileCategory(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty name")
	}
	_, err := d.Exec(`UPDATE file_categories SET name = ? WHERE id = ?`, name, id)
	return err
}

func (d *DB) DeleteFileCategory(id int64) error {
	var name string
	if err := d.QueryRow(`SELECT name FROM file_categories WHERE id = ?`, id).Scan(&name); err != nil {
		return err
	}
	if name == "未分类" {
		return fmt.Errorf("default category")
	}
	defID, err := d.FindOrCreateFileCategory("未分类")
	if err != nil {
		return err
	}
	if _, err := d.Exec(`UPDATE files SET category_id = ? WHERE category_id = ?`, defID, id); err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM file_categories WHERE id = ?`, id)
	return err
}

func (d *DB) CreateFile(catID int64, filename, origName, filePath, mime string, isText bool, fileSize int64) (int64, error) {
	t := 0
	if isText {
		t = 1
	}
	r, err := d.Exec(`INSERT INTO files (category_id, filename, orig_name, file_path, file_size, mime, is_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, catID, filename, origName, filePath, fileSize, mime, t, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListFiles(catID int64) ([]File, error) {
	rows, err := d.Query(`SELECT id, category_id, filename, orig_name, file_path, file_size, mime, is_text, created_at
		FROM files WHERE category_id = ? ORDER BY created_at DESC`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.CategoryID, &f.Filename, &f.OrigName, &f.FilePath, &f.FileSize, &f.Mime, &f.IsText, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (d *DB) GetFile(id int64) (File, error) {
	var f File
	err := d.QueryRow(`SELECT id, category_id, filename, orig_name, file_path, file_size, mime, is_text, created_at
		FROM files WHERE id = ?`, id).
		Scan(&f.ID, &f.CategoryID, &f.Filename, &f.OrigName, &f.FilePath, &f.FileSize, &f.Mime, &f.IsText, &f.CreatedAt)
	return f, err
}

func (d *DB) DeleteFile(id int64) error {
	_, err := d.Exec(`DELETE FROM files WHERE id = ?`, id)
	return err
}

func (d *DB) MoveFile(id, catID int64) error {
	_, err := d.Exec(`UPDATE files SET category_id = ? WHERE id = ?`, catID, id)
	return err
}

func (d *DB) RenameFile(id int64, origName string) error {
	origName = strings.TrimSpace(origName)
	if origName == "" {
		return fmt.Errorf("empty name")
	}
	_, err := d.Exec(`UPDATE files SET orig_name = ? WHERE id = ?`, origName, id)
	return err
}

// ---- script & note categories ----

type ScriptCategory struct {
	ID        int64
	Name      string
	Sort      int
	CreatedAt int64
}

type NoteCategory struct {
	ID        int64
	Name      string
	Sort      int
	CreatedAt int64
}

func (d *DB) findOrCreateCat(tbl, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未分类"
	}
	var id int64
	err := d.QueryRow(fmt.Sprintf(`SELECT id FROM %s WHERE name = ?`, tbl), name).Scan(&id)
	if err == nil {
		return id, nil
	}
	var maxSort int
	_ = d.QueryRow(fmt.Sprintf(`SELECT COALESCE(MAX(sort), -1) FROM %s`, tbl)).Scan(&maxSort)
	r, err := d.Exec(fmt.Sprintf(`INSERT INTO %s (name, sort, created_at) VALUES (?, ?, ?)`, tbl), name, maxSort+1, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) listCats(tbl string) ([]ScriptCategory, error) {
	rows, err := d.Query(fmt.Sprintf(`SELECT id, name, sort, created_at FROM %s ORDER BY sort, id`, tbl))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScriptCategory
	for rows.Next() {
		var c ScriptCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) renameCat(tbl string, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty name")
	}
	_, err := d.Exec(fmt.Sprintf(`UPDATE %s SET name = ? WHERE id = ?`, tbl), name, id)
	return err
}

func (d *DB) deleteCat(tbl, itemTbl string, id int64) error {
	var name string
	if err := d.QueryRow(fmt.Sprintf(`SELECT name FROM %s WHERE id = ?`, tbl), id).Scan(&name); err != nil {
		return err
	}
	if name == "未分类" {
		return fmt.Errorf("default category")
	}
	defID, err := d.findOrCreateCat(tbl, "未分类")
	if err != nil {
		return err
	}
	if _, err := d.Exec(fmt.Sprintf(`UPDATE %s SET category_id = ? WHERE category_id = ?`, itemTbl), defID, id); err != nil {
		return err
	}
	_, err = d.Exec(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tbl), id)
	return err
}

func (d *DB) FindOrCreateScriptCategory(name string) (int64, error) {
	return d.findOrCreateCat("script_categories", name)
}
func (d *DB) ListScriptCategories() ([]ScriptCategory, error) { return d.listCats("script_categories") }

func (d *DB) ListScriptsByCategory(catID int64) ([]Script, error) {
	rows, err := d.Query(`SELECT id, name, version, description, author, content_enc, category_id, created_at, updated_at, is_pinned
		FROM scripts WHERE category_id = ? ORDER BY is_pinned DESC, updated_at DESC`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Script
	for rows.Next() {
		var sc Script
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Version, &sc.Description, &sc.Author, &sc.ContentEnc, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt, &sc.IsPinned); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (d *DB) ListAllScripts() ([]Script, error) {
	rows, err := d.Query(`SELECT id, name, version, description, author, content_enc, category_id, created_at, updated_at, is_pinned
		FROM scripts ORDER BY is_pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Script
	for rows.Next() {
		var sc Script
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Version, &sc.Description, &sc.Author, &sc.ContentEnc, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt, &sc.IsPinned); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}
func (d *DB) RenameScriptCategory(id int64, name string) error {
	return d.renameCat("script_categories", id, name)
}
func (d *DB) DeleteScriptCategory(id int64) error {
	return d.deleteCat("script_categories", "scripts", id)
}
func (d *DB) FindOrCreateNoteCategory(name string) (int64, error) {
	return d.findOrCreateCat("note_categories", name)
}
func (d *DB) ListNoteCategories() ([]NoteCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM note_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteCategory
	for rows.Next() {
		var c NoteCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (d *DB) RenameNoteCategory(id int64, name string) error {
	return d.renameCat("note_categories", id, name)
}
func (d *DB) DeleteNoteCategory(id int64) error {
	return d.deleteCat("note_categories", "notes", id)
}

func (d *DB) MoveScript(id, catID int64) error {
	_, err := d.Exec(`UPDATE scripts SET category_id = ? WHERE id = ?`, catID, id)
	return err
}
func (d *DB) MoveNote(id, catID int64) error {
	_, err := d.Exec(`UPDATE notes SET category_id = ? WHERE id = ?`, catID, id)
	return err
}

func (d *DB) ListAllLinks() ([]*Link, error) {
	rows, err := d.Query(`SELECT id, category_id, name, url, created_at, updated_at, is_pinned, auto_name
		FROM links ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.CategoryID, &l.Name, &l.URL, &l.CreatedAt, &l.UpdatedAt, &l.IsPinned, &l.AutoName); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

func (d *DB) ListAllImages() ([]*Image, error) {
	rows, err := d.Query(`SELECT id, category_id, caption, orig_name, file_path, thumb_path, thumb_s, thumb_m, thumb_l, file_size, created_at
		FROM images ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Image
	for rows.Next() {
		var im Image
		if err := rows.Scan(&im.ID, &im.CategoryID, &im.Caption, &im.OrigName, &im.FilePath, &im.ThumbPath, &im.ThumbS, &im.ThumbM, &im.ThumbL, &im.FileSize, &im.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &im)
	}
	return out, rows.Err()
}

func (d *DB) ListAllFiles() ([]File, error) {
	rows, err := d.Query(`SELECT id, category_id, filename, orig_name, file_path, file_size, mime, is_text, created_at
		FROM files ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.CategoryID, &f.Filename, &f.OrigName, &f.FilePath, &f.FileSize, &f.Mime, &f.IsText, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ---- reminders ----

type ReminderCategory struct {
	ID        int64
	Name      string
	Sort      int
	CreatedAt int64
}

type Reminder struct {
	ID           int64
	CategoryID   int64
	Title        string
	Note         string
	RemindAt     int64
	RepeatType   string // none|daily|weekly|monthly|yearly
	RepeatDetail string // weekly: 0-6 (周日=0); monthly: 1-31
	Times        int
	IsActive     bool
	CreatedAt    int64
}

func (d *DB) FindOrCreateReminderCategory(name string) (int64, error) {
	return d.findOrCreateCat("reminder_categories", name)
}
func (d *DB) ListReminderCategories() ([]ReminderCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM reminder_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReminderCategory
	for rows.Next() {
		var c ReminderCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (d *DB) RenameReminderCategory(id int64, name string) error {
	return d.renameCat("reminder_categories", id, name)
}
func (d *DB) DeleteReminderCategory(id int64) error {
	return d.deleteCat("reminder_categories", "reminders", id)
}

func (d *DB) CreateReminder(r Reminder) (int64, error) {
	res, err := d.Exec(`INSERT INTO reminders (category_id, title, note, remind_at, repeat_type, repeat_detail, times, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.CategoryID, r.Title, r.Note, r.RemindAt, r.RepeatType, r.RepeatDetail, r.Times, boolToInt(r.IsActive), time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetReminder(id int64) (Reminder, error) {
	var r Reminder
	var active int
	err := d.QueryRow(`SELECT id, category_id, title, note, remind_at, repeat_type, repeat_detail, times, is_active, created_at
		FROM reminders WHERE id = ?`, id).
		Scan(&r.ID, &r.CategoryID, &r.Title, &r.Note, &r.RemindAt, &r.RepeatType, &r.RepeatDetail, &r.Times, &active, &r.CreatedAt)
	r.IsActive = active != 0
	return r, err
}

func (d *DB) ListReminders(catID int64) ([]Reminder, error) {
	q := `SELECT id, category_id, title, note, remind_at, repeat_type, repeat_detail, times, is_active, created_at FROM reminders`
	var args []interface{}
	if catID != 0 {
		q += ` WHERE category_id = ?`
		args = append(args, catID)
	}
	q += ` ORDER BY is_active DESC, remind_at ASC`
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reminder
	for rows.Next() {
		var r Reminder
		var active int
		if err := rows.Scan(&r.ID, &r.CategoryID, &r.Title, &r.Note, &r.RemindAt, &r.RepeatType, &r.RepeatDetail, &r.Times, &active, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.IsActive = active != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) UpdateReminder(r Reminder) error {
	_, err := d.Exec(`UPDATE reminders SET category_id = ?, title = ?, note = ?, remind_at = ?,
		repeat_type = ?, repeat_detail = ?, times = ?, is_active = ? WHERE id = ?`,
		r.CategoryID, r.Title, r.Note, r.RemindAt, r.RepeatType, r.RepeatDetail, r.Times, boolToInt(r.IsActive), r.ID)
	return err
}

func (d *DB) DeleteReminder(id int64) error {
	_, err := d.Exec(`DELETE FROM reminders WHERE id = ?`, id)
	return err
}

func (d *DB) SetReminderActive(id int64, active bool) error {
	_, err := d.Exec(`UPDATE reminders SET is_active = ? WHERE id = ?`, boolToInt(active), id)
	return err
}

// DueReminders 返回到期的提醒
func (d *DB) DueReminders(now int64) ([]Reminder, error) {
	rows, err := d.Query(`SELECT id, category_id, title, note, remind_at, repeat_type, repeat_detail, times, is_active, created_at
		FROM reminders WHERE is_active = 1 AND remind_at <= ?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reminder
	for rows.Next() {
		var r Reminder
		var active int
		if err := rows.Scan(&r.ID, &r.CategoryID, &r.Title, &r.Note, &r.RemindAt, &r.RepeatType, &r.RepeatDetail, &r.Times, &active, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.IsActive = active != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- 订阅节点 ----

type Node struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"categoryId"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Kind       string `json:"kind"` // sub | node
	Proto      string `json:"proto"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	LastStatus string `json:"lastStatus"`
	LastLatency int64 `json:"lastLatency"`
	LastCheckAt int64 `json:"lastCheckAt"`
	CreatedAt  int64  `json:"createdAt"`
}

type NodeCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Sort      int    `json:"sort"`
	CreatedAt int64  `json:"createdAt"`
}

func (d *DB) FindOrCreateNodeCategory(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未分类"
	}
	var id int64
	err := d.QueryRow(`SELECT id FROM node_categories WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	r, err := d.Exec(`INSERT INTO node_categories (name, sort, created_at) VALUES (?, (SELECT COALESCE(MAX(sort),0)+1 FROM node_categories), ?)`, name, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListNodeCategories() ([]NodeCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM node_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NodeCategory{}
	for rows.Next() {
		var c NodeCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (d *DB) CreateNodeCategory(name string) (int64, error) {
	return d.FindOrCreateNodeCategory(name)
}

func (d *DB) RenameNodeCategory(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "未分类" {
		return fmt.Errorf("bad name")
	}
	_, err := d.Exec(`UPDATE node_categories SET name = ? WHERE id = ?`, name, id)
	return err
}

func (d *DB) DeleteNodeCategory(id int64) error {
	var name string
	if err := d.QueryRow(`SELECT name FROM node_categories WHERE id = ?`, id).Scan(&name); err != nil {
		return err
	}
	if name == "未分类" {
		return fmt.Errorf("cannot delete default")
	}
	uncatID, _ := d.FindOrCreateNodeCategory("未分类")
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE nodes SET category_id = ? WHERE category_id = ?`, uncatID, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM node_categories WHERE id = ?`, id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (d *DB) CreateNode(catID int64, name, url, kind, proto, host string, port int) (int64, error) {
	r, err := d.Exec(`INSERT INTO nodes (category_id, name, url, kind, proto, host, port, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, catID, name, url, kind, proto, host, port, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListNodes(catID int64) ([]Node, error) {
	var rows *sql.Rows
	var err error
	if catID == 0 {
		rows, err = d.Query(`SELECT id, category_id, name, url, kind, proto, host, port, last_status, last_latency, last_check_at, created_at
			FROM nodes ORDER BY created_at DESC`)
	} else {
		rows, err = d.Query(`SELECT id, category_id, name, url, kind, proto, host, port, last_status, last_latency, last_check_at, created_at
			FROM nodes WHERE category_id = ? ORDER BY created_at DESC`, catID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.CategoryID, &n.Name, &n.URL, &n.Kind, &n.Proto, &n.Host, &n.Port, &n.LastStatus, &n.LastLatency, &n.LastCheckAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (d *DB) GetNode(id int64) (Node, error) {
	var n Node
	err := d.QueryRow(`SELECT id, category_id, name, url, kind, proto, host, port, last_status, last_latency, last_check_at, created_at
		FROM nodes WHERE id = ?`, id).Scan(&n.ID, &n.CategoryID, &n.Name, &n.URL, &n.Kind, &n.Proto, &n.Host, &n.Port, &n.LastStatus, &n.LastLatency, &n.LastCheckAt, &n.CreatedAt)
	return n, err
}

func (d *DB) UpdateNode(id, catID int64, name string) error {
	_, err := d.Exec(`UPDATE nodes SET category_id = ?, name = ? WHERE id = ?`, catID, name, id)
	return err
}

func (d *DB) UpdateNodeCheck(id int64, status string, latency int64) error {
	_, err := d.Exec(`UPDATE nodes SET last_status = ?, last_latency = ?, last_check_at = ? WHERE id = ?`,
		status, latency, time.Now().Unix(), id)
	return err
}

func (d *DB) DeleteNode(id int64) error {
	_, err := d.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	return err
}

// MoveScriptsToCategory 批量移动脚本到分类
func (d *DB) MoveScriptsToCategory(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE scripts SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// MoveNotesToCategory 批量移动笔记到分类
func (d *DB) MoveNotesToCategory(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE notes SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// MoveNodesToCategory 批量移动节点到分类
func (d *DB) MoveNodesToCategory(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE nodes SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// MoveRemindersToCategory 批量移动提醒到分类
func (d *DB) MoveRemindersToCategory(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE reminders SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// MoveFilesToCategory 批量移动文件到分类
func (d *DB) MoveFilesToCategory(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE files SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}

// ============ SSH 主机 ============

type SSHHost struct {
	ID          int64  `json:"id"`
	CategoryID  int64  `json:"categoryId"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordEnc string `json:"-"` // 不返回前端
	Country     string `json:"country"`
	Region      string `json:"region"`
	OsID        string `json:"osId"`
	OsVersion   string `json:"osVersion"`
	LastStatus  string `json:"lastStatus"`
	LastLatency int64  `json:"lastLatency"`
	LastCheckAt int64  `json:"lastCheckAt"`
	CreatedAt   int64  `json:"createdAt"`
}

type SSHCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Sort      int    `json:"sort"`
	CreatedAt int64  `json:"createdAt"`
}

func (d *DB) ListSSHCategories() ([]SSHCategory, error) {
	rows, err := d.Query(`SELECT id, name, sort, created_at FROM ssh_categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SSHCategory{}
	for rows.Next() {
		var c SSHCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (d *DB) FindOrCreateSSHCategory(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未分类"
	}
	var id int64
	err := d.QueryRow(`SELECT id FROM ssh_categories WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	r, err := d.Exec(`INSERT INTO ssh_categories (name, sort, created_at) VALUES (?, 0, ?)`, name, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) CreateSSHCategory(name string) (int64, error) {
	return d.FindOrCreateSSHCategory(name)
}

func (d *DB) RenameSSHCategory(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "未分类" {
		return fmt.Errorf("分类名无效")
	}
	_, err := d.Exec(`UPDATE ssh_categories SET name = ? WHERE id = ?`, name, id)
	return err
}

func (d *DB) DeleteSSHCategory(id int64) error {
	var name string
	if err := d.QueryRow(`SELECT name FROM ssh_categories WHERE id = ?`, id).Scan(&name); err != nil {
		return err
	}
	if name == "未分类" {
		return fmt.Errorf("不能删除未分类")
	}
	var uncatID int64
	if err := d.QueryRow(`SELECT id FROM ssh_categories WHERE name = '未分类'`).Scan(&uncatID); err != nil {
		return err
	}
	if _, err := d.Exec(`UPDATE ssh_hosts SET category_id = ? WHERE category_id = ?`, uncatID, id); err != nil {
		return err
	}
	_, err := d.Exec(`DELETE FROM ssh_categories WHERE id = ?`, id)
	return err
}

func (d *DB) CreateSSHHost(catID int64, name, host string, port int, username, passwordEnc, country, region string) (int64, error) {
	r, err := d.Exec(`INSERT INTO ssh_hosts (category_id, name, host, port, username, password_enc, country, region, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, catID, name, host, port, username, passwordEnc, country, region, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) ListSSHHosts(catID int64) ([]SSHHost, error) {
	var rows *sql.Rows
	var err error
	q := `SELECT id, category_id, name, host, port, username, password_enc, country, region, os_id, os_version, last_status, last_latency, last_check_at, created_at FROM ssh_hosts`
	if catID == 0 {
		rows, err = d.Query(q + ` ORDER BY created_at DESC`)
	} else {
		rows, err = d.Query(q+` WHERE category_id = ? ORDER BY created_at DESC`, catID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SSHHost{}
	for rows.Next() {
		var h SSHHost
		if err := rows.Scan(&h.ID, &h.CategoryID, &h.Name, &h.Host, &h.Port, &h.Username, &h.PasswordEnc, &h.Country, &h.Region, &h.OsID, &h.OsVersion, &h.LastStatus, &h.LastLatency, &h.LastCheckAt, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

func (d *DB) GetSSHHost(id int64) (SSHHost, error) {
	var h SSHHost
	err := d.QueryRow(`SELECT id, category_id, name, host, port, username, password_enc, country, region, os_id, os_version, last_status, last_latency, last_check_at, created_at
		FROM ssh_hosts WHERE id = ?`, id).Scan(&h.ID, &h.CategoryID, &h.Name, &h.Host, &h.Port, &h.Username, &h.PasswordEnc, &h.Country, &h.Region, &h.OsID, &h.OsVersion, &h.LastStatus, &h.LastLatency, &h.LastCheckAt, &h.CreatedAt)
	return h, err
}

func (d *DB) UpdateSSHHost(id, catID int64, name, host string, port int, username, passwordEnc, country, region string) error {
	if passwordEnc == "" {
		// 不改密码
		_, err := d.Exec(`UPDATE ssh_hosts SET category_id=?, name=?, host=?, port=?, username=?, country=?, region=? WHERE id=?`,
			catID, name, host, port, username, country, region, id)
		return err
	}
	_, err := d.Exec(`UPDATE ssh_hosts SET category_id=?, name=?, host=?, port=?, username=?, password_enc=?, country=?, region=? WHERE id=?`,
		catID, name, host, port, username, passwordEnc, country, region, id)
	return err
}

func (d *DB) UpdateSSHHostOS(id int64, osID, osVersion string) error {
	_, err := d.Exec(`UPDATE ssh_hosts SET os_id=?, os_version=? WHERE id=?`, osID, osVersion, id)
	return err
}

func (d *DB) DeleteSSHHost(id int64) error {
	_, err := d.Exec(`DELETE FROM ssh_hosts WHERE id = ?`, id)
	return err
}

func (d *DB) UpdateSSHHostStatus(id int64, status string, latency int64) error {
	_, err := d.Exec(`UPDATE ssh_hosts SET last_status=?, last_latency=?, last_check_at=? WHERE id=?`,
		status, latency, time.Now().Unix(), id)
	return err
}

func (d *DB) MoveSSHHostsToCategory(ids []int64, catID int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := `UPDATE ssh_hosts SET category_id = ? WHERE id IN (`
	args := []interface{}{catID}
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += ")"
	_, err := d.Exec(q, args...)
	return err
}
