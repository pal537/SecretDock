package parse

import (
	"reflect"
	"testing"
)

func entriesOf(p Parsed) []Entry { return p.Entries }

func TestColonVariants(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantLabel string
		wantValue string
	}{
		{"cn colon spaced", "访问密钥：5ee255fe16bb", "访问密钥", "5ee255fe16bb"},
		{"ascii colon", "访问密钥:5ee255fe16bb", "访问密钥", "5ee255fe16bb"},
		{"ascii colon spaced", "Token: abc123", "Token", "abc123"},
		{"value with colons kept", "Endpoint: https://a.b:8080/x", "Endpoint", "https://a.b:8080/x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ParseMessage("m\n"+c.line, nil)
			if len(p.Entries) != 1 {
				t.Fatalf("got %d entries", len(p.Entries))
			}
			e := p.Entries[0]
			if e.Label != c.wantLabel || e.Value != c.wantValue {
				t.Fatalf("got label=%q value=%q", e.Label, e.Value)
			}
		})
	}
}

func TestNoSeparator(t *testing.T) {
	p := ParseMessage("backblaze B2\n访问密钥5ee255fe16bbc9da212766173ee9084f", nil)
	if p.Merchant != "backblaze B2" {
		t.Fatalf("merchant = %q", p.Merchant)
	}
	if len(p.Entries) != 1 {
		t.Fatalf("entries = %d", len(p.Entries))
	}
	e := p.Entries[0]
	if e.Label != "访问密钥" || e.Value != "5ee255fe16bbc9da212766173ee9084f" {
		t.Fatalf("got %+v", e)
	}
}

func TestBareLabelNextLine(t *testing.T) {
	p := ParseMessage("m\n访问密钥\n5ee255fe16bb 60天", nil)
	if len(p.Entries) != 1 {
		t.Fatalf("entries = %d", len(p.Entries))
	}
	e := p.Entries[0]
	if e.Label != "访问密钥" || e.Value != "5ee255fe16bb" || e.ExpiresDays != 60 {
		t.Fatalf("got %+v", e)
	}
}

func TestMixedLabels(t *testing.T) {
	cases := []struct{ line, wantLabel, wantValue string }{
		{"SecretID(访问密钥)RTERTRWETscfe4353g4", "SecretID(访问密钥)", "RTERTRWETscfe4353g4"},
		{"访问密钥(Secret ID)abc123", "访问密钥(Secret ID)", "abc123"},
		{"Token（令牌）xyz-789", "Token（令牌）", "xyz-789"},
	}
	for _, c := range cases {
		p := ParseMessage("m\n"+c.line, nil)
		if len(p.Entries) != 1 {
			t.Fatalf("%q: entries = %d", c.line, len(p.Entries))
		}
		e := p.Entries[0]
		if e.Label != c.wantLabel || e.Value != c.wantValue {
			t.Fatalf("%q: got label=%q value=%q", c.line, e.Label, e.Value)
		}
	}
}

func TestExpiryVariants(t *testing.T) {
	// All five variants from the product discussion.
	lines := []string{
		"访问密钥5ee255fe16bbc9da212766173ee9084f 60天",
		"访问密钥 5ee255fe16bbc9da212766173ee9084f 60天",
		"访问密钥：5ee255fe16bbc9da212766173ee9084f 60天",
		"访问密钥:5ee255fe16bbc9da212766173ee9084f 60天",
		"访问密钥\n5ee255fe16bbc9da212766173ee9084f 60天",
	}
	for i, l := range lines {
		p := ParseMessage("m\n"+l, nil)
		if len(p.Entries) != 1 {
			t.Fatalf("case %d: entries = %d", i, len(p.Entries))
		}
		e := p.Entries[0]
		if e.ExpiresDays != 60 {
			t.Fatalf("case %d: expires = %d, entry=%+v", i, e.ExpiresDays, e)
		}
		if e.Value != "5ee255fe16bbc9da212766173ee9084f" {
			t.Fatalf("case %d: value = %q", i, e.Value)
		}
		if e.Label != "访问密钥" {
			t.Fatalf("case %d: label = %q", i, e.Label)
		}
	}
}

func TestNoExpiryStaysNone(t *testing.T) {
	p := ParseMessage("m\n访问密钥:abc123\n备注:明天开会", nil)
	if len(p.Entries) != 2 {
		t.Fatalf("entries = %d", len(p.Entries))
	}
	for _, e := range p.Entries {
		if e.ExpiresDays != 0 {
			t.Fatalf("unexpected expiry in %+v", e)
		}
	}
	if p.Entries[1].Value != "明天开会" {
		t.Fatalf("value = %q", p.Entries[1].Value)
	}
}

func TestValueNotTruncated(t *testing.T) {
	// Cookie-style values with arbitrary symbols must survive intact.
	p := ParseMessage("m\nCookie: sessionid=abc123; Path=/; HttpOnly; Secure 30天", nil)
	if len(p.Entries) != 1 {
		t.Fatalf("entries = %d", len(p.Entries))
	}
	e := p.Entries[0]
	if e.Value != "sessionid=abc123; Path=/; HttpOnly; Secure" {
		t.Fatalf("truncated value = %q", e.Value)
	}
	if e.ExpiresDays != 30 {
		t.Fatalf("expires = %d", e.ExpiresDays)
	}

	p2 := ParseMessage("m\n令牌TEST_TOKEN_EXAMPLE_12345", nil)
	if p2.Entries[0].Value != "TEST_TOKEN_EXAMPLE_12345" {
		t.Fatalf("value = %q", p2.Entries[0].Value)
	}
}

func TestFullExample(t *testing.T) {
	msg := `backblaze B2
访问密钥5ee255fe16bbc9da212766173ee9084f
机密访问密钥c9033cdef37e52dda8f347fcf3c3d2f9f81bd9e44314b8eeb9103ee910b0271d
令牌值TEST_TOKEN_EXAMPLE_12345

终结点默认https://d8c0db8110756ab2ab80a75c12880453.r2.cloudflarestorage.com
美国https://d8c0db8110756ab2ab80a75c12880453.us.r2.cloudflarestorage.com`
	p := ParseMessage(msg, nil)
	if p.Merchant != "backblaze B2" {
		t.Fatalf("merchant = %q", p.Merchant)
	}
	want := []Entry{
		{Label: "访问密钥", Value: "5ee255fe16bbc9da212766173ee9084f"},
		{Label: "机密访问密钥", Value: "c9033cdef37e52dda8f347fcf3c3d2f9f81bd9e44314b8eeb9103ee910b0271d"},
		{Label: "令牌值", Value: "TEST_TOKEN_EXAMPLE_12345"},
		// URL split: text before the link is the label.
		{Label: "终结点默认", Value: "https://d8c0db8110756ab2ab80a75c12880453.r2.cloudflarestorage.com"},
		{Label: "美国", Value: "https://d8c0db8110756ab2ab80a75c12880453.us.r2.cloudflarestorage.com"},
	}
	if !reflect.DeepEqual(entriesOf(p), want) {
		t.Fatalf("got %+v", entriesOf(p))
	}
}

func TestCustomLabelsLearned(t *testing.T) {
	p := ParseMessage("m\n终结点默认https://x.example.com", []string{"终结点默认"})
	if len(p.Entries) != 1 {
		t.Fatalf("entries = %d", len(p.Entries))
	}
	e := p.Entries[0]
	if e.Label != "终结点默认" || e.Value != "https://x.example.com" {
		t.Fatalf("got %+v", e)
	}
}

func TestMatchMerchant(t *testing.T) {
	merchants := []MerchantInfo{
		{ID: 1, Name: "backblaze", Aliases: []string{}},
		{ID: 2, Name: "Cloudflare", Aliases: []string{"cf", "CF"}},
		{ID: 3, Name: "backblaze大号", Aliases: []string{}},
	}
	// exact
	if m, _ := MatchMerchant("backblaze", merchants); m == nil || m.ID != 1 {
		t.Fatalf("exact failed: %+v", m)
	}
	// alias, case-insensitive
	if m, _ := MatchMerchant("CF", merchants); m == nil || m.ID != 2 {
		t.Fatalf("alias failed: %+v", m)
	}
	if m, _ := MatchMerchant("cloudflare", merchants); m == nil || m.ID != 2 {
		t.Fatalf("case-insensitive failed")
	}
	// fuzzy: "backblaze B2" is not exact -> candidates backblaze + backblaze大号
	if m, cands := MatchMerchant("backblaze B2", merchants); m != nil {
		t.Fatalf("should not be exact: %+v", m)
	} else if len(cands) != 2 {
		t.Fatalf("candidates = %+v", cands)
	}
	// no match at all
	if m, cands := MatchMerchant("github", merchants); m != nil || len(cands) != 0 {
		t.Fatalf("unexpected: %+v %+v", m, cands)
	}
}
