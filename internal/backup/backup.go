// Package backup implements encrypted JSON export/import and scheduled
// local backups of all SecretDock data.
package backup

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/internal/img"
	"golang.org/x/crypto/argon2"
)

// Export format version. v1 = merchants/secrets/labels only.
// v2 adds scripts, links, notes, nodes, reminders, ssh hosts, settings,
// and optional images/files. applyImport accepts both.
const exportVersion = 2

// BackupOptions controls what goes into a backup.
type BackupOptions struct {
	IncludeImages bool
	IncludeFiles  bool
	DataDir       string // needed to read/write images/files
}

// SecretData is one secret with its plaintext value (inside the encrypted envelope).
type SecretData struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Username  string `json:"username,omitempty"`
	Value     string `json:"value"`
	Note      string `json:"note,omitempty"`
	ExpiresAt int64  `json:"expiresAt"`
	CreatedAt int64  `json:"createdAt"`
}

// MerchantData groups secrets under a merchant.
type MerchantData struct {
	Name    string       `json:"name"`
	Aliases []string     `json:"aliases"`
	LogoURL string       `json:"logoUrl"`
	Secrets []SecretData `json:"secrets"`
}

// ScriptData is one script with decrypted content; category stored by name.
type ScriptData struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Content     string `json:"content"`
	Category    string `json:"category"`
	IsPinned    bool   `json:"isPinned,omitempty"`
	CreatedAt   int64  `json:"createdAt,omitempty"`
	UpdatedAt   int64  `json:"updatedAt,omitempty"`
}

// LinkData is one bookmark.
type LinkData struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Category  string `json:"category"`
	AutoName  bool   `json:"autoName,omitempty"`
	IsPinned  bool   `json:"isPinned,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// NoteData is one note with decrypted content.
type NoteData struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	IsPinned  bool   `json:"isPinned,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// NodeData is one subscription node or subscription URL.
type NodeData struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	Proto     string `json:"proto,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Category  string `json:"category"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// ReminderData is one reminder.
type ReminderData struct {
	Title        string `json:"title"`
	Note         string `json:"note,omitempty"`
	RemindAt     int64  `json:"remindAt"`
	RepeatType   string `json:"repeatType,omitempty"`
	RepeatDetail string `json:"repeatDetail,omitempty"`
	Times        int    `json:"times,omitempty"`
	IsActive     bool   `json:"isActive"`
	Category     string `json:"category"`
	CreatedAt    int64  `json:"createdAt,omitempty"`
}

// SSHHostData is one SSH host with decrypted password.
type SSHHostData struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	PrivKey   string `json:"privKey,omitempty"`
	KeyPass   string `json:"keyPass,omitempty"`
	Country   string `json:"country,omitempty"`
	Region    string `json:"region,omitempty"`
	ExpiryDate string `json:"expiryDate,omitempty"`
	VendorURL string `json:"vendorUrl,omitempty"`
	OsID      string `json:"osId,omitempty"`
	OsVersion string `json:"osVersion,omitempty"`
	Category  string `json:"category"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// ImageData is one image; Data is base64 of the original file (optional).
type ImageData struct {
	Caption  string `json:"caption"`
	OrigName string `json:"origName,omitempty"`
	FileName string `json:"fileName"` // on-disk name under images/
	Data     string `json:"data,omitempty"`
	Category string `json:"category"`
	CreatedAt int64 `json:"createdAt,omitempty"`
}

// FileData is one file; Data is base64 of the file (optional).
type FileData struct {
	Name     string `json:"name"`     // display/original name
	FileName string `json:"fileName"` // on-disk name under files/
	Mime     string `json:"mime,omitempty"`
	IsText   bool   `json:"isText,omitempty"`
	Data     string `json:"data,omitempty"`
	Category string `json:"category"`
	CreatedAt int64 `json:"createdAt,omitempty"`
}

// ExportFile is the plaintext payload before envelope encryption.
type ExportFile struct {
	Version    int               `json:"version"`
	App        string            `json:"app"`
	ExportedAt int64             `json:"exportedAt"`
	Merchants  []MerchantData    `json:"merchants"`
	Labels     []string          `json:"labels"`
	Scripts    []ScriptData      `json:"scripts,omitempty"`
	Links      []LinkData        `json:"links,omitempty"`
	Notes      []NoteData        `json:"notes,omitempty"`
	Nodes      []NodeData        `json:"nodes,omitempty"`
	Reminders  []ReminderData    `json:"reminders,omitempty"`
	SSHHosts   []SSHHostData     `json:"sshHosts,omitempty"`
	Settings   map[string]string `json:"settings,omitempty"`
	Images     []ImageData       `json:"images,omitempty"`
	Files      []FileData        `json:"files,omitempty"`
}

// envelope wraps the encrypted payload.
type envelope struct {
	V    int    `json:"v"`
	KDF  string `json:"kdf"`
	Salt string `json:"salt"`
	Data string `json:"data"`
}

// deriveKey derives a 32-byte key from a password with Argon2id.
func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 1, 32*1024, 4, 32)
}

func validVersion(v int) bool { return v == 1 || v == exportVersion }

// catName builds an id->name lookup for categories.
func catNames[K any](cats []K, id func(K) int64, name func(K) string) map[int64]string {
	m := map[int64]string{}
	for _, c := range cats {
		m[id(c)] = name(c)
	}
	return m
}

// Collect gathers all data with decrypted values.
func Collect(database *db.DB, encKey []byte, opts BackupOptions) (*ExportFile, error) {
	dec := func(enc string, what string) (string, error) {
		raw, err := crypto.Decrypt(encKey, enc)
		if err != nil {
			return "", fmt.Errorf("decrypt %s: %w", what, err)
		}
		return string(raw), nil
	}

	merchants, err := database.ListMerchants()
	if err != nil {
		return nil, err
	}
	ef := &ExportFile{Version: exportVersion, App: "secretdock", ExportedAt: time.Now().Unix()}
	for _, m := range merchants {
		md := MerchantData{Name: m.Name, Aliases: m.Aliases, LogoURL: m.LogoURL}
		secrets, err := database.ListSecrets(m.ID)
		if err != nil {
			return nil, err
		}
		for _, s := range secrets {
			v, err := dec(s.Value, fmt.Sprintf("secret %d", s.ID))
			if err != nil {
				return nil, err
			}
			md.Secrets = append(md.Secrets, SecretData{
				Kind: s.Kind, Label: s.Label, Username: s.Username,
				Value: v, Note: s.Note,
				ExpiresAt: s.ExpiresAt, CreatedAt: s.CreatedAt,
			})
		}
		ef.Merchants = append(ef.Merchants, md)
	}
	labels, _ := database.CustomLabels()
	ef.Labels = labels

	// scripts
	if scats, err := database.ListScriptCategories(); err == nil {
		names := catNames(scats, func(c db.ScriptCategory) int64 { return c.ID }, func(c db.ScriptCategory) string { return c.Name })
		if scripts, err := database.ListAllScripts(); err == nil {
			for _, sc := range scripts {
				content, err := dec(sc.ContentEnc, fmt.Sprintf("script %d", sc.ID))
				if err != nil {
					return nil, err
				}
				ef.Scripts = append(ef.Scripts, ScriptData{
					Name: sc.Name, Version: sc.Version, Description: sc.Description,
					Author: sc.Author, Content: content, Category: names[sc.CategoryID],
					IsPinned: sc.IsPinned, CreatedAt: sc.CreatedAt, UpdatedAt: sc.UpdatedAt,
				})
			}
		}
	}

	// links
	if lcats, err := database.ListLinkCategories(); err == nil {
		names := catNames(lcats, func(c *db.LinkCategory) int64 { return c.ID }, func(c *db.LinkCategory) string { return c.Name })
		if links, err := database.ListAllLinks(); err == nil {
			for _, l := range links {
				ef.Links = append(ef.Links, LinkData{
					Name: l.Name, URL: l.URL, Category: names[l.CategoryID],
					AutoName: l.AutoName, IsPinned: l.IsPinned, CreatedAt: l.CreatedAt,
				})
			}
		}
	}

	// notes
	if ncats, err := database.ListNoteCategories(); err == nil {
		names := catNames(ncats, func(c db.NoteCategory) int64 { return c.ID }, func(c db.NoteCategory) string { return c.Name })
		if notes, err := database.ListNotes(); err == nil {
			for _, n := range notes {
				content, err := dec(n.ContentEnc, fmt.Sprintf("note %d", n.ID))
				if err != nil {
					return nil, err
				}
				ef.Notes = append(ef.Notes, NoteData{
					Title: n.Title, Content: content, Category: names[n.CategoryID],
					IsPinned: n.IsPinned, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
				})
			}
		}
	}

	// nodes
	if ncats, err := database.ListNodeCategories(); err == nil {
		names := catNames(ncats, func(c db.NodeCategory) int64 { return c.ID }, func(c db.NodeCategory) string { return c.Name })
		if nodes, err := database.ListNodes(0); err == nil {
			for _, nd := range nodes {
				ef.Nodes = append(ef.Nodes, NodeData{
					Name: nd.Name, URL: nd.URL, Kind: nd.Kind, Proto: nd.Proto,
					Host: nd.Host, Port: nd.Port, Category: names[nd.CategoryID],
					CreatedAt: nd.CreatedAt,
				})
			}
		}
	}

	// reminders
	if rcats, err := database.ListReminderCategories(); err == nil {
		names := catNames(rcats, func(c db.ReminderCategory) int64 { return c.ID }, func(c db.ReminderCategory) string { return c.Name })
		if rems, err := database.ListReminders(0); err == nil {
			for _, r := range rems {
				ef.Reminders = append(ef.Reminders, ReminderData{
					Title: r.Title, Note: r.Note, RemindAt: r.RemindAt,
					RepeatType: r.RepeatType, RepeatDetail: r.RepeatDetail,
					Times: r.Times, IsActive: r.IsActive, Category: names[r.CategoryID],
					CreatedAt: r.CreatedAt,
				})
			}
		}
	}

	// ssh hosts
	if scats, err := database.ListSSHCategories(); err == nil {
		names := catNames(scats, func(c db.SSHCategory) int64 { return c.ID }, func(c db.SSHCategory) string { return c.Name })
		if hosts, err := database.ListSSHHosts(0); err == nil {
			for _, h := range hosts {
				pw, err := dec(h.PasswordEnc, fmt.Sprintf("ssh host %d", h.ID))
				if err != nil {
					return nil, err
				}
				var privKey, keyPass string
				if h.PrivKeyEnc != "" {
					k, err := dec(h.PrivKeyEnc, fmt.Sprintf("ssh host %d key", h.ID))
					if err != nil {
						return nil, err
					}
					privKey = k
					if h.KeyPassEnc != "" {
						kp, err := dec(h.KeyPassEnc, fmt.Sprintf("ssh host %d keypass", h.ID))
						if err != nil {
							return nil, err
						}
						keyPass = kp
					}
				}
				ef.SSHHosts = append(ef.SSHHosts, SSHHostData{
					Name: h.Name, Host: h.Host, Port: h.Port, Username: h.Username,
					Password: pw, PrivKey: privKey, KeyPass: keyPass, Country: h.Country, Region: h.Region,
					OsID: h.OsID, OsVersion: h.OsVersion, Category: names[h.CategoryID],
					ExpiryDate: h.ExpiryDate, VendorURL: h.VendorURL,
					CreatedAt: h.CreatedAt,
				})
			}
		}
	}

	// settings (TG config lives here)
	if settings, err := database.ListSettings(); err == nil {
		ef.Settings = settings
	}

	// images (optional, with file data)
	if opts.IncludeImages && opts.DataDir != "" {
		if icats, err := database.ListImageCategories(); err == nil {
			names := catNames(icats, func(c *db.ImageCategory) int64 { return c.ID }, func(c *db.ImageCategory) string { return c.Name })
			for _, c := range icats {
				images, err := database.ListImages(c.ID)
				if err != nil {
					continue
				}
				for _, im := range images {
					data, err := os.ReadFile(filepath.Join(opts.DataDir, "images", im.FilePath))
					if err != nil {
						continue // orphaned record; skip
					}
					ef.Images = append(ef.Images, ImageData{
						Caption: im.Caption, OrigName: im.OrigName, FileName: im.FilePath,
						Data: base64.StdEncoding.EncodeToString(data),
						Category: names[im.CategoryID], CreatedAt: im.CreatedAt,
					})
				}
			}
		}
	}

	// files (optional, with file data)
	if opts.IncludeFiles && opts.DataDir != "" {
		if fcats, err := database.ListFileCategories(); err == nil {
			names := catNames(fcats, func(c db.FileCategory) int64 { return c.ID }, func(c db.FileCategory) string { return c.Name })
			for _, c := range fcats {
				files, err := database.ListFiles(c.ID)
				if err != nil {
					continue
				}
				for _, f := range files {
					data, err := os.ReadFile(filepath.Join(opts.DataDir, "files", f.FilePath))
					if err != nil {
						continue // orphaned record; skip
					}
					ef.Files = append(ef.Files, FileData{
						Name: f.OrigName, FileName: f.FilePath, Mime: f.Mime,
						IsText: f.IsText,
						Data: base64.StdEncoding.EncodeToString(data),
						Category: names[f.CategoryID], CreatedAt: f.CreatedAt,
					})
				}
			}
		}
	}

	return ef, nil
}

// seal encrypts the payload with a raw 32-byte key.
func seal(payload []byte, key []byte) ([]byte, error) {
	ct, err := crypto.Encrypt(key, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{V: exportVersion, KDF: "raw32", Data: ct})
}

// open decrypts a raw32 envelope (accepts v1 and v2).
func openEnvelope(raw, key []byte) ([]byte, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, errors.New("backup: not a SecretDock backup file")
	}
	if !validVersion(env.V) || env.KDF != "raw32" {
		return nil, errors.New("backup: unsupported backup version")
	}
	pt, err := crypto.Decrypt(key, env.Data)
	if err != nil {
		return nil, errors.New("backup: wrong key or corrupted file")
	}
	return pt, nil
}

// ExportWithPassword creates a password-encrypted backup file.
func ExportWithPassword(database *db.DB, encKey []byte, password string, opts BackupOptions) ([]byte, error) {
	if password == "" {
		return nil, errors.New("backup: password required")
	}
	ef, err := Collect(database, encKey, opts)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(ef)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	ct, err := crypto.Encrypt(deriveKey(password, salt), payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{
		V: exportVersion, KDF: "argon2id",
		Salt: base64.StdEncoding.EncodeToString(salt), Data: ct,
	})
}

// ExportWithKey creates a master-key-encrypted backup (for scheduled backups).
func ExportWithKey(database *db.DB, encKey []byte, opts BackupOptions) ([]byte, error) {
	ef, err := Collect(database, encKey, opts)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(ef)
	if err != nil {
		return nil, err
	}
	return seal(payload, encKey)
}

// ImportWithPassword restores a password-encrypted backup (merge: append).
func ImportWithPassword(database *db.DB, encKey []byte, raw []byte, password string, opts BackupOptions) (int, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, errors.New("backup: not a SecretDock backup file")
	}
	if !validVersion(env.V) || env.KDF != "argon2id" {
		return 0, errors.New("backup: unsupported backup version")
	}
	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return 0, errors.New("backup: corrupted file")
	}
	pt, err := crypto.Decrypt(deriveKey(password, salt), env.Data)
	if err != nil {
		return 0, errors.New("backup: wrong password or corrupted file")
	}
	return applyImport(database, encKey, pt, opts)
}

// ImportWithKey restores a master-key-encrypted backup.
func ImportWithKey(database *db.DB, encKey, raw []byte, opts BackupOptions) (int, error) {
	pt, err := openEnvelope(raw, encKey)
	if err != nil {
		return 0, err
	}
	return applyImport(database, encKey, pt, opts)
}

func encOrDie(encKey []byte, plain string) (string, error) {
	return crypto.Encrypt(encKey, []byte(plain))
}

// applyImport merges the payload into the database. v1 payloads (merchants
// and labels only) import exactly as before; newer sections are appended
// when present.
func applyImport(database *db.DB, encKey, payload []byte, opts BackupOptions) (int, error) {
	var ef ExportFile
	if err := json.Unmarshal(payload, &ef); err != nil {
		return 0, errors.New("backup: corrupted payload")
	}
	if ef.App != "" && ef.App != "secretdock" {
		return 0, errors.New("backup: not a SecretDock backup")
	}
	// index existing merchants by lowercase name
	existing, _ := database.ListMerchants()
	byName := map[string]int64{}
	for _, m := range existing {
		byName[toLower(m.Name)] = m.ID
	}
	count := 0
	for _, md := range ef.Merchants {
		id, ok := byName[toLower(md.Name)]
		if !ok {
			nid, err := database.CreateMerchant(md.Name, md.Aliases, md.LogoURL, "")
			if err != nil {
				return count, err
			}
			id = nid
			byName[toLower(md.Name)] = id
		}
		for _, sd := range md.Secrets {
			enc, err := encOrDie(encKey, sd.Value)
			if err != nil {
				return count, err
			}
			if _, err := database.CreateSecret(db.Secret{
				MerchantID: id, Kind: sd.Kind, Label: sd.Label,
				Username: sd.Username, Value: enc, Note: sd.Note,
				ExpiresAt: sd.ExpiresAt,
			}); err != nil {
				return count, err
			}
			count++
		}
	}
	for _, l := range ef.Labels {
		_ = database.AddCustomLabel(l)
	}

	// scripts
	for _, sd := range ef.Scripts {
		catID, err := database.FindOrCreateScriptCategory(sd.Category)
		if err != nil {
			return count, err
		}
		enc, err := encOrDie(encKey, sd.Content)
		if err != nil {
			return count, err
		}
		id, err := database.CreateScript(sd.Name, sd.Version, sd.Description, sd.Author, enc, catID)
		if err != nil {
			return count, err
		}
		if sd.IsPinned {
			_ = database.SetPinned("scripts", id, true)
		}
		count++
	}

	// links
	for _, ld := range ef.Links {
		catID, err := database.FindOrCreateLinkCategory(ld.Category)
		if err != nil {
			return count, err
		}
		autoName := 0
		if ld.AutoName {
			autoName = 1
		}
		id, err := database.CreateLink(catID, ld.Name, ld.URL, autoName)
		if err != nil {
			return count, err
		}
		if ld.IsPinned {
			_ = database.SetPinned("links", id, true)
		}
		count++
	}

	// notes
	for _, nd := range ef.Notes {
		catID, err := database.FindOrCreateNoteCategory(nd.Category)
		if err != nil {
			return count, err
		}
		enc, err := encOrDie(encKey, nd.Content)
		if err != nil {
			return count, err
		}
		id, err := database.CreateNote(nd.Title, enc, catID)
		if err != nil {
			return count, err
		}
		if nd.IsPinned {
			_ = database.SetPinned("notes", id, true)
		}
		count++
	}

	// nodes
	for _, nd := range ef.Nodes {
		catID, err := database.FindOrCreateNodeCategory(nd.Category)
		if err != nil {
			return count, err
		}
		if _, err := database.CreateNode(catID, nd.Name, nd.URL, nd.Kind, nd.Proto, nd.Host, nd.Port); err != nil {
			return count, err
		}
		count++
	}

	// reminders
	for _, rd := range ef.Reminders {
		catID, err := database.FindOrCreateReminderCategory(rd.Category)
		if err != nil {
			return count, err
		}
		r := db.Reminder{
			CategoryID: catID, Title: rd.Title, Note: rd.Note,
			RemindAt: rd.RemindAt, RepeatType: rd.RepeatType,
			RepeatDetail: rd.RepeatDetail, Times: rd.Times, IsActive: rd.IsActive,
		}
		if _, err := database.CreateReminder(r); err != nil {
			return count, err
		}
		count++
	}

	// ssh hosts
	for _, hd := range ef.SSHHosts {
		catID, err := database.FindOrCreateSSHCategory(hd.Category)
		if err != nil {
			return count, err
		}
		enc, err := encOrDie(encKey, hd.Password)
		if err != nil {
			return count, err
		}
		keyEnc, keyPassEnc := "", ""
		if hd.PrivKey != "" {
			k, err := encOrDie(encKey, hd.PrivKey)
			if err != nil {
				return count, err
			}
			keyEnc = k
			if hd.KeyPass != "" {
				kp, err := encOrDie(encKey, hd.KeyPass)
				if err != nil {
					return count, err
				}
				keyPassEnc = kp
			}
		}
		id, err := database.CreateSSHHost(catID, hd.Name, hd.Host, hd.Port, hd.Username, enc, keyEnc, keyPassEnc, hd.Country, hd.Region, hd.ExpiryDate, hd.VendorURL)
		if err != nil {
			return count, err
		}
		if hd.OsID != "" {
			_ = database.UpdateSSHHostOS(id, hd.OsID, hd.OsVersion)
		}
		count++
	}

	// settings: merge only, never overwrite existing keys
	for k, v := range ef.Settings {
		if database.GetSetting(k) == "" {
			_ = database.SetSetting(k, v)
		}
	}

	// images (optional; needs a data dir)
	if opts.DataDir != "" {
		imgDir := filepath.Join(opts.DataDir, "images")
		for _, im := range ef.Images {
			if im.Data == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(im.Data)
			if err != nil || len(data) == 0 {
				continue
			}
			if err := os.MkdirAll(imgDir, 0o755); err != nil {
				return count, err
			}
			base := fmt.Sprintf("imp-%d", time.Now().UnixNano())
			name := base + "_orig" + filepath.Ext(im.FileName)
			if err := os.WriteFile(filepath.Join(imgDir, name), data, 0o644); err != nil {
				return count, err
			}
			thumbS, thumbM, thumbL, err := img.MakeThumbs(data, imgDir, base)
			if err != nil {
				_ = os.Remove(filepath.Join(imgDir, name))
				continue
			}
			catID, err := database.FindOrCreateImageCategory(im.Category)
			if err != nil {
				return count, err
			}
			if _, err := database.CreateImage(catID, im.Caption, im.OrigName, name, thumbS, thumbM, thumbL, int64(len(data))); err != nil {
				return count, err
			}
			count++
		}

		// files (optional; needs a data dir)
		fileDir := filepath.Join(opts.DataDir, "files")
		for _, fd := range ef.Files {
			if fd.Data == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(fd.Data)
			if err != nil || len(data) == 0 {
				continue
			}
			if err := os.MkdirAll(fileDir, 0o755); err != nil {
				return count, err
			}
			name := fmt.Sprintf("imp-%d%s", time.Now().UnixNano(), filepath.Ext(fd.FileName))
			if err := os.WriteFile(filepath.Join(fileDir, name), data, 0o644); err != nil {
				return count, err
			}
			catID, err := database.FindOrCreateFileCategory(fd.Category)
			if err != nil {
				return count, err
			}
			if _, err := database.CreateFile(catID, name, fd.Name, name, fd.Mime, fd.IsText, int64(len(data))); err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// ---- scheduled local backups ----

// WriteScheduled writes a master-key-encrypted backup into dir and prunes
// old ones, keeping the newest `keep` files.
func WriteScheduled(database *db.DB, encKey []byte, dir string, keep int, opts BackupOptions) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	raw, err := ExportWithKey(database, encKey, opts)
	if err != nil {
		return "", err
	}
	ts := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("secretdock-backup-%s.enc", ts)
	path := filepath.Join(dir, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("secretdock-backup-%s-%d.enc", ts, i))
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	// prune
	entries, err := filepath.Glob(filepath.Join(dir, "secretdock-backup-*.enc"))
	if err == nil && len(entries) > keep && keep > 0 {
		sort.Strings(entries)
		for _, old := range entries[:len(entries)-keep] {
			_ = os.Remove(old)
		}
	}
	return path, nil
}

// RunScheduler runs a daily backup at hh:mm (local time) until stopCh closes.
func RunScheduler(database *db.DB, encKey []byte, dir, hhmm string, keep int, opts BackupOptions, stopCh <-chan struct{}, logf func(string, ...interface{})) {
	if dir == "" {
		return
	}
	for {
		next := nextRun(hhmm)
		select {
		case <-stopCh:
			return
		case <-time.After(time.Until(next)):
		}
		path, err := WriteScheduled(database, encKey, dir, keep, opts)
		if err != nil {
			logf("backup: scheduled backup failed: %v", err)
		} else {
			logf("backup: wrote %s", path)
		}
	}
}

func nextRun(hhmm string) time.Time {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		h, m = 3, 0
	}
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
