package secpolicy

import (
	"testing"
)

func TestEvaluateBanned(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		Countries: map[string]bool{"CN": true},
		Banned:    map[string]bool{"1.2.3.4": true},
	}
	// banned IP is denied even though it would otherwise be evaluated
	ok, reason := Evaluate("1.2.3.4", cfg, nil)
	if ok {
		t.Fatalf("banned IP should be denied, got allow")
	}
	if reason != "该 IP 已被封禁" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	// ban beats whitelist
	cfg.Whitelist, _ = ParseIPList("1.2.3.0/24")
	ok, _ = Evaluate("1.2.3.4", cfg, nil)
	if ok {
		t.Fatalf("ban should beat whitelist")
	}
	// non-banned IP is not affected by the ban list
	ok, reason = Evaluate("5.6.7.8", cfg, nil)
	if ok || reason == "该 IP 已被封禁" {
		t.Fatalf("non-banned IP wrongly affected: ok=%v reason=%q", ok, reason)
	}
	// private IPs still bypass even if listed (intranet default whitelist)
	cfg.Banned["127.0.0.1"] = true
	ok, _ = Evaluate("127.0.0.1", cfg, nil)
	if !ok {
		t.Fatalf("private IP should always be allowed")
	}
	// IPv6 normalization: stored canonical form matches
	cfg2 := Config{Enabled: true, Banned: map[string]bool{"2001:db8::1": true}}
	ok, reason = Evaluate("2001:0db8:0000::0001", cfg2, nil)
	if ok || reason != "该 IP 已被封禁" {
		t.Fatalf("IPv6 canonical mismatch: ok=%v reason=%q", ok, reason)
	}
}
