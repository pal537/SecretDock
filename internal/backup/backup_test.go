package backup

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
)

func testDB(t *testing.T) (*db.DB, []byte) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := crypto.GenerateKey()
	return d, key
}

func seed(t *testing.T, d *db.DB, key []byte) {
	t.Helper()
	enc := func(s string) string {
		c, err := crypto.Encrypt(key, []byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	mid, err := d.CreateMerchant("Cloudflare", []string{"cf"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateSecret(db.Secret{MerchantID: mid, Kind: "secret", Label: "API Token", Value: enc("cfat_xxx"), ExpiresAt: 123}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateSecret(db.Secret{MerchantID: mid, Kind: "account", Label: "登录", Username: "a@b.c", Value: enc("p@ss:word/带中文"), Note: "n"}); err != nil {
		t.Fatal(err)
	}
	_ = d.AddCustomLabel("API Token")
}

// TestExportImportRoundtrip covers the full backup->wipe->restore cycle.
func TestExportImportRoundtrip(t *testing.T) {
	d, key := testDB(t)
	seed(t, d, key)

	raw, err := ExportWithPassword(d, key, "pw-123", BackupOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// wrong password must fail
	if _, err := ImportWithPassword(d, key, raw, "wrong", BackupOptions{}); err == nil {
		t.Fatal("expected wrong-password error")
	}

	// wipe
	for _, m := range mustList(t, d) {
		if err := d.DeleteMerchant(m.ID); err != nil {
			t.Fatal(err)
		}
	}

	n, err := ImportWithPassword(d, key, raw, "pw-123", BackupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("imported %d secrets, want 2", n)
	}
	merchants := mustList(t, d)
	if len(merchants) != 1 || merchants[0].Name != "Cloudflare" {
		t.Fatalf("merchants after import: %+v", merchants)
	}
	secrets, err := d.ListSecrets(merchants[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range secrets {
		raw2, err := crypto.Decrypt(key, s.Value)
		if err != nil {
			t.Fatal(err)
		}
		got[s.Label] = string(raw2)
	}
	if got["API Token"] != "cfat_xxx" || got["登录"] != "p@ss:word/带中文" {
		t.Fatalf("values corrupted: %v", got)
	}
	labels, _ := d.CustomLabels()
	if len(labels) != 1 || labels[0] != "API Token" {
		t.Fatalf("labels: %v", labels)
	}
}

// TestScheduledBackupWritePrune verifies WriteScheduled + keep-N pruning.
func TestScheduledBackupWritePrune(t *testing.T) {
	d, key := testDB(t)
	seed(t, d, key)
	dir := t.TempDir()

	for i := 0; i < 4; i++ {
		if _, err := WriteScheduled(d, key, dir, 3, BackupOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "secretdock-backup-*.enc"))
	if len(entries) != 3 {
		t.Fatalf("kept %d files, want 3", len(entries))
	}
	// newest file must be restorable with the master key
	raw, err := os.ReadFile(entries[len(entries)-1])
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := testDB(t)
	n, err := ImportWithKey(d2, key, raw, BackupOptions{})
	if err != nil {
		t.Fatalf("scheduled backup restore failed: %v", err)
	}
	if n != 2 {
		t.Fatalf("restored %d secrets, want 2", n)
	}
}

func mustList(t *testing.T, d *db.DB) []db.Merchant {
	t.Helper()
	l, err := d.ListMerchants()
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestExportImportV2Roundtrip covers scripts/links/notes/nodes/reminders/ssh/settings.
func TestExportImportV2Roundtrip(t *testing.T) {
	d, key := testDB(t)
	enc := func(s string) string {
		c, err := crypto.Encrypt(key, []byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	scat, _ := d.FindOrCreateScriptCategory("运维")
	if _, err := d.CreateScript("test.sh", "1.0", "desc", "me", enc("echo hi"), scat); err != nil {
		t.Fatal(err)
	}
	lcat, _ := d.FindOrCreateLinkCategory("常用")
	lid, err := d.CreateLink(lcat, "Google", "https://google.com", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetPinned("links", lid, true); err != nil {
		t.Fatal(err)
	}
	ncat, _ := d.FindOrCreateNoteCategory("笔记")
	if _, err := d.CreateNote("标题", enc("正文"), ncat); err != nil {
		t.Fatal(err)
	}
	nocat, _ := d.FindOrCreateNodeCategory("节点")
	if _, err := d.CreateNode(nocat, "n1", "vless://xxx", "node", "vless", "1.2.3.4", 443); err != nil {
		t.Fatal(err)
	}
	rcat, _ := d.FindOrCreateReminderCategory("提醒")
	if _, err := d.CreateReminder(db.Reminder{CategoryID: rcat, Title: "t", RemindAt: 123, IsActive: true}); err != nil {
		t.Fatal(err)
	}
	hcat, _ := d.FindOrCreateSSHCategory("服务器")
	if _, err := d.CreateSSHHost(hcat, "srv", "1.2.3.4", 22, "root", enc("pw"), "", "", "美国", "西雅图"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting("tg_bot_token", "tok123"); err != nil {
		t.Fatal(err)
	}

	raw, err := ExportWithPassword(d, key, "pw", BackupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// pre-set a setting in target: import must not overwrite it
	d2, _ := testDB(t)
	if err := d2.SetSetting("tg_bot_token", "existing"); err != nil {
		t.Fatal(err)
	}
	n, err := ImportWithPassword(d2, key, raw, "pw", BackupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Fatalf("imported %d items, want 6", n)
	}
	if got := d2.GetSetting("tg_bot_token"); got != "existing" {
		t.Fatalf("setting overwritten: %q", got)
	}
	scripts, _ := d2.ListAllScripts()
	if len(scripts) != 1 || scripts[0].Name != "test.sh" {
		t.Fatalf("scripts: %+v", scripts)
	}
	c, _ := crypto.Decrypt(key, scripts[0].ContentEnc)
	if string(c) != "echo hi" {
		t.Fatalf("script content: %q", c)
	}
	links, _ := d2.ListAllLinks()
	if len(links) != 1 || !links[0].IsPinned {
		t.Fatalf("links: %+v", links)
	}
	notes, _ := d2.ListNotes()
	if len(notes) != 1 || notes[0].Title != "标题" {
		t.Fatalf("notes: %+v", notes)
	}
	nodes, _ := d2.ListNodes(0)
	if len(nodes) != 1 || nodes[0].Proto != "vless" {
		t.Fatalf("nodes: %+v", nodes)
	}
	rems, _ := d2.ListReminders(0)
	if len(rems) != 1 || !rems[0].IsActive {
		t.Fatalf("reminders: %+v", rems)
	}
	hosts, _ := d2.ListSSHHosts(0)
	if len(hosts) != 1 {
		t.Fatalf("ssh hosts: %+v", hosts)
	}
	pw, _ := crypto.Decrypt(key, hosts[0].PasswordEnc)
	if string(pw) != "pw" || hosts[0].Region != "西雅图" {
		t.Fatalf("ssh host data: %+v", hosts[0])
	}
}

// TestImportV1Compat verifies old v1 backups (merchants/labels only) still import.
func TestImportV1Compat(t *testing.T) {
	d, key := testDB(t)
	payload, _ := json.Marshal(map[string]interface{}{
		"version": 1, "app": "secretdock", "exportedAt": 123,
		"merchants": []interface{}{map[string]interface{}{
			"name": "M", "secrets": []interface{}{
				map[string]interface{}{"kind": "secret", "label": "L", "value": "V"},
			},
		}},
		"labels": []string{"L"},
	})
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		t.Fatal(err)
	}
	ct, err := crypto.Encrypt(deriveKey("pw", salt), payload)
	if err != nil {
		t.Fatal(err)
	}
	envRaw, _ := json.Marshal(envelope{V: 1, KDF: "argon2id",
		Salt: base64.StdEncoding.EncodeToString(salt), Data: ct})
	n, err := ImportWithPassword(d, key, envRaw, "pw", BackupOptions{})
	if err != nil {
		t.Fatalf("v1 import failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported %d, want 1", n)
	}
}

// TestExportImportImagesFiles covers optional image/file backup with data dir.
func TestExportImportImagesFiles(t *testing.T) {
	d, key := testDB(t)
	dataDir := t.TempDir()
	imgDir := filepath.Join(dataDir, "images")
	fileDir := filepath.Join(dataDir, "files")
	os.MkdirAll(imgDir, 0o755)
	os.MkdirAll(fileDir, 0o755)

	// 10x10 red PNG
	png := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		for y := 0; y < 10; y++ {
			for x := 0; x < 10; x++ {
				img.Set(x, y, color.RGBA{255, 0, 0, 255})
			}
		}
		var buf bytes.Buffer
		png.Encode(&buf, img)
		return buf.Bytes()
	}()
	os.WriteFile(filepath.Join(imgDir, "a.png"), png, 0o644)
	icat, _ := d.FindOrCreateImageCategory("相册")
	if _, err := d.CreateImage(icat, "cap", "a.png", "a_orig.png", "a_s.jpg", "a_m.jpg", "a_l.jpg", int64(len(png))); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(fileDir, "b.txt"), []byte("hello"), 0o644)
	fcat, _ := d.FindOrCreateFileCategory("文档")
	if _, err := d.CreateFile(fcat, "b.txt", "b.txt", "b.txt", "text/plain", true, 5); err != nil {
		t.Fatal(err)
	}

	// without options: no images/files
	raw, err := ExportWithPassword(d, key, "pw", BackupOptions{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := testDB(t)
	n, err := ImportWithPassword(d2, key, raw, "pw", BackupOptions{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("imported %d, want 0", n)
	}

	// with options: images + files roundtrip
	raw, err = ExportWithPassword(d, key, "pw", BackupOptions{IncludeImages: true, IncludeFiles: true, DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	d3, _ := testDB(t)
	dd3 := t.TempDir()
	n, err = ImportWithPassword(d3, key, raw, "pw", BackupOptions{IncludeImages: true, IncludeFiles: true, DataDir: dd3})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("imported %d, want 2", n)
	}
	images, _ := d3.ListImages(icat)
	// note: imported image lands in its own category (by name "相册"), check by listing all cats
	found := false
	for _, c := range mustImageCats(t, d3) {
		ims, _ := d3.ListImages(c.ID)
		for _, im := range ims {
			if im.Caption == "cap" {
				found = true
				if _, err := os.Stat(filepath.Join(dd3, "images", im.FilePath)); err != nil {
					t.Fatalf("image file missing: %v", err)
				}
			}
		}
	}
	if !found {
		t.Fatal("image not restored")
	}
	_ = images
	filesFound := false
	for _, c := range mustFileCats(t, d3) {
		fs, _ := d3.ListFiles(c.ID)
		for _, f := range fs {
			if f.OrigName == "b.txt" {
				filesFound = true
				got, _ := os.ReadFile(filepath.Join(dd3, "files", f.FilePath))
				if string(got) != "hello" {
					t.Fatalf("file content: %q", got)
				}
			}
		}
	}
	if !filesFound {
		t.Fatal("file not restored")
	}
}

func mustImageCats(t *testing.T, d *db.DB) []*db.ImageCategory {
	t.Helper()
	l, err := d.ListImageCategories()
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func mustFileCats(t *testing.T, d *db.DB) []db.FileCategory {
	t.Helper()
	l, err := d.ListFileCategories()
	if err != nil {
		t.Fatal(err)
	}
	return l
}
