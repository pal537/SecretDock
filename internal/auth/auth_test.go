package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLockoutAfterThreeFails(t *testing.T) {
	a, err := New("admin", "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	mkReq := func(ip string) *http.Request {
		r := httptest.NewRequest("POST", "/api/login", nil)
		r.RemoteAddr = ip + ":5555"
		return r
	}
	r := mkReq("1.2.3.4")

	if a.Locked(r) {
		t.Fatal("should not start locked")
	}
	for i := 0; i < 3; i++ {
		locked := a.RecordFail(r)
		if i < 2 && locked {
			t.Fatalf("locked too early at fail %d", i+1)
		}
	}
	if !a.Locked(r) {
		t.Fatal("expected locked after 3 fails")
	}
	// another IP unaffected
	if a.Locked(mkReq("9.9.9.9")) {
		t.Fatal("other IP should not be locked")
	}
	// success clears failures
	a.RecordSuccess(r)
	if a.Locked(r) {
		t.Fatal("lock should clear after success")
	}
}

func TestSessionRoundtrip(t *testing.T) {
	a, _ := New("admin", "secret", true)
	tok := a.NewSession(true, "1.2.3.4")
	if !a.Valid(tok, "1.2.3.4") {
		t.Fatal("session should be valid")
	}
	// IP change drops the session
	if a.Valid(tok, "5.6.7.8") {
		t.Fatal("session must be invalid after IP change")
	}
	if a.Valid(tok, "1.2.3.4") {
		t.Fatal("session should have been revoked on IP mismatch")
	}
	tok2 := a.NewSession(false, "9.9.9.9")
	a.Revoke(tok2)
	if a.Valid(tok2, "9.9.9.9") {
		t.Fatal("session should be revoked")
	}
	// secure flag honored
	w := httptest.NewRecorder()
	a.SetCookie(w, "abc", true)
	if c := w.Header().Get("Set-Cookie"); !contains(c, "Secure") {
		t.Fatalf("Secure flag missing: %s", c)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
