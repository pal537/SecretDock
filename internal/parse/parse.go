// Package parse implements the Telegram free-text parser for SecretDock.
//
// Message format (add flow):
//
//	<merchant name>            <- first line
//	<Label>[:：]<Value>[ <N>天]  <- one entry per line(s)
//	...
//
// Rules (from the product spec):
//   - label/value split: first Chinese or ASCII colon wins; without a colon,
//     the longest known-label prefix wins; a bare label line means the next
//     line is its value.
//   - mixed labels: base label + parenthesized alias count as one label,
//     e.g. SecretID(访问密钥) or 访问密钥(Secret ID).
//   - trailing "<digits>天" is stripped as expiry-in-days; everything between
//     the label and the expiry is the value, never truncated (cookies with
//     arbitrary symbols survive).
//   - unknown labels are kept as empty label so the confirm preview flags them.
package parse

import (
	"regexp"
	"strings"
	"unicode"
)

// Entry is one parsed label/value pair.
type Entry struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	ExpiresDays int    `json:"expiresDays"` // 0 = none
}

// Parsed is the result of ParseMessage.
type Parsed struct {
	Merchant string  `json:"merchant"`
	Entries  []Entry `json:"entries"`
}

// seedLabels are built-in known base labels (canonical forms).
var seedLabels = []string{
	// Chinese
	"访问密钥", "机密访问密钥", "密钥ID", "密钥", "令牌", "令牌值",
	"密码", "用户名", "账号", "账户", "终结点", "终端节点", "区域",
	"存储桶", "备注", "公钥", "私钥", "证书", "客户端ID", "客户端密钥",
	"应用ID", "应用密钥", "有效期", "过期时间",
	// English
	"SecretID", "SecretKey", "AccessKey", "AccessKeyID", "SecretAccessKey",
	"APIKey", "API Key", "APISecret", "API Secret", "Token", "AccessToken",
	"Password", "Username", "User", "Account", "Endpoint", "Region",
	"Bucket", "Note", "Remark", "ClientID", "ClientSecret", "AppID",
	"AppSecret", "AppKey", "PrivateKey", "PublicKey", "Certificate",
	"Cookie", "Session",
}

var expiryRe = regexp.MustCompile(`(\d+)\s*天\s*$`)

// fullWidthDigits normalizes full-width digits to ASCII.
func fullWidthDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		return r
	}, s)
}

// stripExpiry removes a trailing "<digits>天" and returns the rest + days.
func stripExpiry(line string) (string, int) {
	line = fullWidthDigits(line)
	m := expiryRe.FindStringSubmatch(line)
	if m == nil {
		return line, 0
	}
	days := 0
	for _, c := range m[1] {
		days = days*10 + int(c-'0')
	}
	return strings.TrimSpace(line[:len(line)-len(m[0])]), days
}

// labelIndex holds known labels sorted for longest-prefix matching.
type labelIndex struct {
	lower []string // lowercased label
	orig  []string // canonical form
}

// buildIndex merges seed labels with user-taught ones.
func buildIndex(extra []string) *labelIndex {
	seen := map[string]bool{}
	var lower, orig []string
	add := func(l string) {
		l = strings.TrimSpace(l)
		if l == "" {
			return
		}
		k := strings.ToLower(l)
		if seen[k] {
			return
		}
		seen[k] = true
		lower = append(lower, k)
		orig = append(orig, l)
	}
	for _, l := range seedLabels {
		add(l)
	}
	for _, l := range extra {
		add(l)
	}
	// longest first so longest-prefix matching is trivial
	for i := 0; i < len(lower); i++ {
		for j := i + 1; j < len(lower); j++ {
			if len(lower[j]) > len(lower[i]) {
				lower[i], lower[j] = lower[j], lower[i]
				orig[i], orig[j] = orig[j], orig[i]
			}
		}
	}
	return &labelIndex{lower: lower, orig: orig}
}

// matchPrefix returns the canonical label if line starts with a known label.
func (li *labelIndex) matchPrefix(line string) (label, rest string, ok bool) {
	ll := strings.ToLower(line)
	for i, k := range li.lower {
		if strings.HasPrefix(ll, k) {
			return li.orig[i], line[len(k):], true
		}
	}
	return "", "", false
}

// consumeParen consumes a leading parenthesized group like "(访问密钥)" or
// "（Secret ID）" and returns it plus the remainder.
func consumeParen(s string) (group, rest string) {
	if s == "" {
		return "", s
	}
	r := []rune(s)
	var open, close rune
	switch r[0] {
	case '(':
		open, close = '(', ')'
	case '（':
		open, close = '（', '）'
	default:
		return "", s
	}
	_ = open
	for i := 1; i < len(r); i++ {
		if r[i] == close {
			return string(r[:i+1]), string(r[i+1:])
		}
	}
	return "", s // unbalanced: not a group
}

// firstSepColon returns the byte index of the first colon that acts as a
// label/value separator. Colons that are part of "://" (URLs) are skipped.
func firstSepColon(line string) int {
	for i := 0; i < len(line); {
		j := strings.IndexAny(line[i:], ":：")
		if j < 0 {
			return -1
		}
		j += i
		if line[j] == ':' && strings.HasPrefix(line[j:], "://") {
			i = j + 1
			continue
		}
		return j
	}
	return -1
}

// splitLabelValue splits one line into label and value.
// Returns label == "" when nothing recognizable; pending=true when the line
// is a bare label whose value comes on the next line.
func splitLabelValue(li *labelIndex, line string) (label, value string, pending bool) {
	// 0. space before colon: "PPPID abc:def" -> label "PPPID", not "PPPID abc"
	// (English labels must use space; value may contain colons)
	// 注意：括号内的空格不算（如 "访问密钥(Secret ID)abc123"）
	if si := firstSpaceOutsideParen(line); si > 0 {
		if ci := firstSepColon(line); ci < 0 || si < ci {
			l := strings.TrimSpace(line[:si])
			v := strings.TrimSpace(line[si+1:])
			if l != "" && v != "" && isPlausibleLabel(l) {
				return l, v, false
			}
		}
	}
	// 1. colon (Chinese or ASCII): split on the first separator colon
	if i := firstSepColon(line); i >= 0 {
		l := strings.TrimSpace(line[:i])
		var v string
		if line[i] == ':' { // ASCII colon: 1 byte
			v = strings.TrimSpace(line[i+1:])
		} else { // '：' full-width colon: 3 bytes in UTF-8
			v = strings.TrimSpace(line[i+3:])
		}
		if v == "" {
			return l, "", true // bare label, value on next line
		}
		return l, v, false
	}
	// 1.5 URL split: if the line contains a URL, everything before it is
	// the label (e.g. "终结点默认https://…" -> label "终结点默认").
	// This is deterministic and beats guessing unknown labels.
	if i := strings.Index(line, "https://"); i > 0 {
		l := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i:])
		if l != "" && v != "" {
			return l, v, false
		}
	} else if i := strings.Index(line, "http://"); i > 0 {
		l := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i:])
		if l != "" && v != "" {
			return l, v, false
		}
	}
	// 2. known-label prefix
	if lab, rest, ok := li.matchPrefix(line); ok {
		// consume optional parenthesized alias: SecretID(访问密钥)
		if g, r := consumeParen(strings.TrimSpace(rest)); g != "" {
			lab += g
			rest = r
		}
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return lab, "", true // bare label
		}
		// If the remainder starts with a CJK character, the matched prefix
		// is probably only the head of a longer unknown label (e.g.
		// "终结点" inside "终结点默认https://…"). Splitting here would
		// corrupt the value, so flag the whole line for review instead.
		// (Teach it once via custom labels and it parses next time.)
		if r, _ := firstRune(rest); unicode.Is(unicode.Han, r) {
			return "", line, false
		}
		return lab, rest, false
	}
	// 4. unknown label: smart split
	// 4b. paren separator: "PPPID（备注）value" -> label "PPPID（备注）"
	if i := strings.Index(line, "）"); i > 0 {
		l := strings.TrimSpace(line[:i+len("）")])
		v := strings.TrimSpace(line[i+len("）"):])
		if l != "" && v != "" {
			return l, v, false
		}
	}
	if i := strings.Index(line, ")"); i > 0 {
		l := strings.TrimSpace(line[:i+1])
		v := strings.TrimSpace(line[i+1:])
		// ) 前面不能有空格（避免把 "foo (bar) baz" 误切）
		if l != "" && v != "" && !strings.Contains(strings.TrimSpace(line[:i]), " ") {
			return l, v, false
		}
	}
	// 4c. Chinese leading run: "屁屁屁账号xxx" -> label "屁屁屁账号"
	if label, value, ok := splitChineseLabel(line); ok {
		return label, value, false
	}
	// 5. unrecognized: whole line is the value
	return "", line, false
}

// firstSpaceOutsideParen 找括号外的第一个空格
func firstSpaceOutsideParen(s string) int {
	depth := 0
	for i, r := range s {
		switch r {
		case '(', '（':
			depth++
		case ')', '）':
			if depth > 0 {
				depth--
			}
		case ' ', '\t':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// isPlausibleLabel 标签是否合理（1-20字，含字母）
func isPlausibleLabel(s string) bool {
	r := []rune(s)
	if len(r) == 0 || len(r) > 20 {
		return false
	}
	hasLetter := false
	for _, c := range r {
		if unicode.Is(unicode.Han, c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
		// 允许中文、英文字母、数字、常见符号
		if !(unicode.Is(unicode.Han, c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' || c == '（' || c == '）' || c == '(' || c == ')') {
			return false
		}
	}
	return hasLetter
}

var chineseLabelRe = regexp.MustCompile(`^([\p{Han}]{2,8}[A-Z]{0,4})([A-Za-z0-9].*)$`)

// splitChineseLabel 切分中文开头的未知标签
// "屁屁屁账号secret123" -> 标签"屁屁屁账号", 值"secret123"
// "屁屁屁IDsecret123" -> 标签"屁屁屁ID", 值"secret123"
func splitChineseLabel(line string) (string, string, bool) {
	m := chineseLabelRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	label, value := m[1], strings.TrimSpace(m[2])
	// 值太短不切（避免误伤）
	if len([]rune(value)) < 3 {
		return "", "", false
	}
	return label, value, true
}

// ParseMessage parses a TG add-message. The first line is the merchant name.
func ParseMessage(text string, extraLabels []string) Parsed {
	li := buildIndex(extraLabels)
	var p Parsed
	var pendingLabel string
	var pendingExp int
	first := true
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if first {
			p.Merchant = line
			first = false
			continue
		}
		body, expDays := stripExpiry(line)
		if pendingLabel != "" {
			// previous line was a bare label: this line is its value
			p.Entries = append(p.Entries, Entry{Label: pendingLabel, Value: body, ExpiresDays: expDays})
			pendingLabel, pendingExp = "", 0
			continue
		}
		label, value, pending := splitLabelValue(li, body)
		if pending {
			pendingLabel, pendingExp = label, expDays
			continue
		}
		p.Entries = append(p.Entries, Entry{Label: label, Value: value, ExpiresDays: expDays})
	}
	// dangling bare label at the end: keep it with empty value so the
	// confirm preview shows it instead of silently dropping it
	if pendingLabel != "" {
		p.Entries = append(p.Entries, Entry{Label: pendingLabel, Value: "", ExpiresDays: pendingExp})
	}
	return p
}

// MerchantInfo is the minimal merchant view needed for matching.
type MerchantInfo struct {
	ID      int64
	Name    string
	Aliases []string
}

func normName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// firstRune returns the first rune of s.
func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

// commonPrefixLen counts the common rune prefix length of a and b.
func commonPrefixLen(a, b string) int {
	ar, br := []rune(a), []rune(b)
	n := 0
	for n < len(ar) && n < len(br) && ar[n] == br[n] {
		n++
	}
	return n
}

// MatchMerchant resolves a merchant name: exact (name or alias,
// case-insensitive) wins; otherwise substring candidates either direction.
func MatchMerchant(name string, merchants []MerchantInfo) (exact *MerchantInfo, candidates []MerchantInfo) {
	n := normName(name)
	for i := range merchants {
		m := &merchants[i]
		if normName(m.Name) == n {
			return m, nil
		}
		for _, a := range m.Aliases {
			if normName(a) == n {
				return m, nil
			}
		}
	}
	for i := range merchants {
		m := &merchants[i]
		mn := normName(m.Name)
		hit := strings.Contains(mn, n) || strings.Contains(n, mn) ||
			commonPrefixLen(mn, n) >= 4 // "backblaze" vs "backblaze大号"
		if !hit {
			for _, a := range m.Aliases {
				an := normName(a)
				if strings.Contains(an, n) || strings.Contains(n, an) ||
					commonPrefixLen(an, n) >= 4 {
					hit = true
					break
				}
			}
		}
		if hit {
			candidates = append(candidates, *m)
		}
	}
	return nil, candidates
}
