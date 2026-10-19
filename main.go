// Command secretdock is a self-hosted secrets vault for developer
// credentials (tokens, API keys, IDs, account passwords).
//
// Configuration (environment variables):
//
//	SECRETDOCK_DATA_DIR    directory for secretdock.db (default ./data)
//	SECRETDOCK_ADDR        listen address (default 127.0.0.1:8080)
//	SECRETDOCK_USER        login username (required)
//	SECRETDOCK_PASSWORD    login password (required)
//	SECRETDOCK_MASTER_KEY  base64-encoded 32-byte master key for data encryption.
//	                       Optional: if unset, SecretDock generates one on first
//	                       run and saves it to <dataDir>/.master_key (0600).
//	                       Do NOT lose it — encrypted data becomes unreadable.
//	SECRETDOCK_SECURE_COOKIES  set to 1 when serving over HTTPS (default 0)
//
// Generate a key with:  secretdock genkey
package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"path/filepath"
	"strings"

	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/backup"
	"github.com/example/secretdock/internal/crypto"
	"github.com/example/secretdock/internal/db"
	"github.com/example/secretdock/internal/handlers"
	"github.com/example/secretdock/internal/secpolicy"
	"github.com/example/secretdock/internal/tg"
)

var appVersion = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "genkey" {
		key, err := crypto.GenerateKey()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(base64.StdEncoding.EncodeToString(key))
		return
	}

	user := os.Getenv("SECRETDOCK_USER")
	pass := os.Getenv("SECRETDOCK_PASSWORD")

	dataDir := os.Getenv("SECRETDOCK_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	masterStr := os.Getenv("SECRETDOCK_MASTER_KEY")
	if masterStr == "" {
		keyPath := filepath.Join(dataDir, ".master_key")
		if b, err := os.ReadFile(keyPath); err == nil && strings.TrimSpace(string(b)) != "" {
			masterStr = strings.TrimSpace(string(b))
			log.Printf("using persisted master key from %s", keyPath)
		} else {
			key, err := crypto.GenerateKey()
			if err != nil {
				log.Fatalf("cannot generate master key: %v", err)
			}
			masterStr = base64.StdEncoding.EncodeToString(key)
			if err := os.WriteFile(keyPath, []byte(masterStr+"\n"), 0o600); err != nil {
				log.Fatalf("cannot save master key: %v", err)
			}
			log.Printf("generated new master key, saved to %s — BACK THIS FILE UP, losing it loses all data", keyPath)
		}
	}
	encKey, err := crypto.KeyFromEnv(masterStr)
	if err != nil {
		log.Fatalf("bad master key: %v", err)
	}

	database, err := db.Open(filepath.Join(dataDir, "secretdock.db"))
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}

	// CLI: ban list management (escape hatch if you ban yourself).
	// Usage: secretdock bans | secretdock unban <ip>
	if len(os.Args) > 1 && (os.Args[1] == "bans" || os.Args[1] == "unban") {
		cli := &handlers.Server{DB: database}
		if os.Args[1] == "bans" {
			rows := cli.ListBans()
			if len(rows) == 0 {
				fmt.Println("封禁名单为空")
				return
			}
			for _, b := range rows {
				fmt.Printf("%s  %s  %s\n", b.IP, b.Reason,
					time.Unix(b.Time, 0).Format("2006-01-02 15:04:05"))
			}
			return
		}
		if len(os.Args) < 3 {
			log.Fatal("usage: secretdock unban <ip>")
		}
		if err := cli.UnbanIP(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		fmt.Println("已解封", os.Args[2])
		return
	}

	// 迁移：env 账号 -> DB（仅一次）
	if !database.HasUser() && user != "" && pass != "" {
		hash, err := auth.HashSecret(pass)
		if err != nil {
			log.Fatalf("cannot hash password: %v", err)
		}
		if err := database.CreateUser(user, hash); err != nil {
			log.Fatalf("cannot migrate user: %v", err)
		}
		log.Printf("migrated env user to database")
	}

	secureCookies := os.Getenv("SECRETDOCK_SECURE_COOKIES") == "1"
	authSvc, err2 := auth.New("", "", secureCookies)
	if err2 != nil {
		log.Fatalf("auth: %v", err2)
	}

	// IP country database: prefer the updatable copy under the data dir,
	// fall back to the copy baked into the image.
	geo := &secpolicy.GeoIP{}
	for _, p := range []string{
		filepath.Join(dataDir, "geoip", "dbip.mmdb"),
		"/geoip/dbip.mmdb",
	} {
		if err := geo.Reload(p); err == nil {
			log.Printf("geoip: loaded %s", p)
			break
		}
	}
	if !geo.Loaded() {
		log.Printf("geoip: no country database found; country allowlist unavailable until updated in settings")
	}

	tgSvc := tg.NewService(database, encKey, dataDir)
	go tgSvc.Run()

	// scheduled local backups (optional; configured in settings)
	go runBackupScheduler(database, encKey, dataDir)

	srv := &handlers.Server{
		DB:      database,
		Auth:    authSvc,
		EncKey:  encKey,
		DataDir: dataDir,
		TG:      tgSvc,
		Geo:     geo,
	}
	// One-click IP ban from TG security alerts.
	tgSvc.BanIPFunc = srv.BanIP
	mux := http.NewServeMux()
	srv.Routes(mux)

	addr := os.Getenv("SECRETDOCK_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("SecretDock %s listening on %s (data: %s)", appVersion, addr, dataDir)
	log.Fatal(http.ListenAndServe(addr, srv.Wrap(mux)))
}

func runBackupScheduler(database *db.DB, encKey []byte, dataDir string) {
	for {
		enabled := database.GetSetting("backup_enabled") == "1"
		dir := strings.TrimSpace(database.GetSetting("backup_dir"))
		hhmm := database.GetSetting("backup_time")
		if hhmm == "" {
			hhmm = "03:00"
		}
		if !enabled || dir == "" {
			time.Sleep(time.Minute) // settings may change; re-check
			continue
		}
		now := time.Now()
		var h, m int
		if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
			h, m = 3, 0
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		if database.GetSetting("backup_enabled") != "1" {
			continue
		}
		keep := 7
		if v := database.GetSetting("backup_keep"); v != "" {
			var k int
			if _, err := fmt.Sscanf(v, "%d", &k); err == nil && k > 0 {
				keep = k
			}
		}
		if path, err := backup.WriteScheduled(database, encKey, dir, keep, backup.BackupOptions{
			IncludeImages: database.GetSetting("backup_include_images") == "1",
			IncludeFiles:  database.GetSetting("backup_include_files") == "1",
			DataDir:       dataDir,
		}); err != nil {
			log.Printf("backup: scheduled backup failed: %v", err)
		} else {
			log.Printf("backup: wrote %s", path)
		}
	}
}
