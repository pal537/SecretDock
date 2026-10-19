package handlers

import (
	"path/filepath"
	"testing"

	"github.com/example/secretdock/internal/auth"
	"github.com/example/secretdock/internal/db"
)

func newBanTestServer(t *testing.T) *Server {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := auth.New("", "", false)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{DB: d, Auth: a}
}

func TestBanUnban(t *testing.T) {
	s := newBanTestServer(t)

	if err := s.BanIP("1.2.3.4", "test"); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := s.BanIP("1.2.3.4", "test"); err != nil {
		t.Fatal(err)
	}
	// IPv6 normalization
	if err := s.BanIP("2001:0db8::1", "test"); err != nil {
		t.Fatal(err)
	}
	rows := s.ListBans()
	if len(rows) != 2 {
		t.Fatalf("want 2 bans, got %d", len(rows))
	}
	found := false
	for _, r := range rows {
		if r.IP == "2001:db8::1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("IPv6 not normalized, rows=%v", rows)
	}
	// invalid IP
	if err := s.BanIP("999.1.1.1", "test"); err == nil {
		t.Fatal("want error for invalid IP")
	}
	// unban
	if err := s.UnbanIP("1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if len(s.ListBans()) != 1 {
		t.Fatal("unban did not remove")
	}
	if err := s.UnbanIP("9.9.9.9"); err == nil {
		t.Fatal("want error unbanning unknown IP")
	}
}

func TestBanRevokesSessions(t *testing.T) {
	s := newBanTestServer(t)
	tok := s.Auth.NewSession(false, "1.2.3.4")
	if !s.Auth.Valid(tok, "1.2.3.4") {
		t.Fatal("session should be valid")
	}
	if err := s.BanIP("1.2.3.4", "test"); err != nil {
		t.Fatal(err)
	}
	if s.Auth.Valid(tok, "1.2.3.4") {
		t.Fatal("session should be revoked after ban")
	}
}

func TestIsNewIP(t *testing.T) {
	s := newBanTestServer(t)
	if !s.isNewIP("1.2.3.4") {
		t.Fatal("first sight should be new")
	}
	if s.isNewIP("1.2.3.4") {
		t.Fatal("second sight should not be new")
	}
	if !s.isNewIP("5.6.7.8") {
		t.Fatal("different IP should be new")
	}
}
