package auth

import "testing"

// Regression test: VerifySecret must accept the secret that HashSecret hashed.
// (2026-10-18: fmt.Sscanf with %s did not stop at '$', so verification
// always returned false and no TG query password could ever succeed.)
func TestHashVerifySecretRoundtrip(t *testing.T) {
	for _, pw := range []string{"626389", "hello-world_123", "p@ss w0rd!"} {
		h, err := HashSecret(pw)
		if err != nil {
			t.Fatalf("HashSecret(%q): %v", pw, err)
		}
		if !VerifySecret(h, pw) {
			t.Errorf("VerifySecret rejected correct password %q", pw)
		}
		if VerifySecret(h, pw+"x") {
			t.Errorf("VerifySecret accepted wrong password for %q", pw)
		}
	}
	if VerifySecret("garbage", "x") {
		t.Error("VerifySecret accepted malformed hash")
	}
}

func TestNormalizeSecret(t *testing.T) {
	if got := NormalizeSecret("６２６３８９"); got != "626389" {
		t.Errorf("full-width not normalized: %q", got)
	}
	if got := NormalizeSecret("  abc  "); got != "abc" {
		t.Errorf("trim failed: %q", got)
	}
}
