// Package auth handles env-based single-user login: Argon2id password
// verification, session cookies, and per-IP brute-force lockout.
//
// There is no registration: SECRETDOCK_USER / SECRETDOCK_PASSWORD must be
// set. 3 failed attempts from one IP within 24h lock that IP out until the
// process restarts (in-memory counters).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const sessionCookieName = "secretdock_session"

// Argon2id parameters (OWASP recommended).
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// Lockout policy.
const (
	maxFails      = 3
	failWindow    = 24 * time.Hour
	sessionTTL    = 12 * time.Hour
	rememberTTL   = 30 * 24 * time.Hour
	cleanupEvery  = 10 * time.Minute
)

// Auth holds the single user's credential hash, sessions and fail counters.
type Auth struct {
	username string
	passHash []byte
	passSalt []byte

	secureCookies bool

	mu       sync.Mutex
	sessions map[string]session // token -> session
	pending  map[string]pendingAuth // 2fa tmp token -> pending
	fails    map[string]*failRec // ip -> failures
	locked   map[string]time.Time
}

type session struct {
	expires time.Time
	ip      string // session is bound to the login IP; IP change drops it
}

// pendingAuth is a password-verified but 2FA-pending login attempt.
type pendingAuth struct {
	ip       string
	remember bool
	expires  time.Time
}

type failRec struct {
	count int
	first time.Time
}

// New creates an Auth for session management. Username/password are verified against DB.
func New(username, password string, secureCookies bool) (*Auth, error) {
	a := &Auth{
		secureCookies: secureCookies,
		sessions:      make(map[string]session),
		pending:       make(map[string]pendingAuth),
		fails:         make(map[string]*failRec),
		locked:        make(map[string]time.Time),
	}
	go a.cleanupLoop()
	return a, nil
}



func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Locked reports whether the request's IP is currently locked out.
func (a *Auth) Locked(r *http.Request) bool {
	ip := clientIP(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.locked[ip]
	return ok
}

// RecordFail records a failed login; returns true if this attempt locks the IP.
func (a *Auth) RecordFail(r *http.Request) bool {
	ip := clientIP(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	rec, ok := a.fails[ip]
	if !ok || now.Sub(rec.first) > failWindow {
		rec = &failRec{first: now}
		a.fails[ip] = rec
	}
	rec.count++
	if rec.count >= maxFails {
		a.locked[ip] = now
		delete(a.fails, ip)
		return true
	}
	return false
}

// RecordSuccess clears fail state for the IP.
func (a *Auth) RecordSuccess(r *http.Request) {
	ip := clientIP(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.fails, ip)
	delete(a.locked, ip)
}

// NewSession creates a session token bound to ip; remember extends TTL to 30 days.
func (a *Auth) NewSession(remember bool, ip string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	ttl := sessionTTL
	if remember {
		ttl = rememberTTL
	}
	a.mu.Lock()
	a.sessions[tok] = session{expires: time.Now().Add(ttl), ip: ip}
	a.mu.Unlock()
	return tok
}

// Valid checks a session token and that the request still comes from the
// login IP. An IP change invalidates the session immediately.
func (a *Auth) Valid(tok, ip string) bool {
	if tok == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[tok]
	if !ok || time.Now().After(s.expires) {
		delete(a.sessions, tok)
		return false
	}
	if s.ip != "" && s.ip != ip {
		delete(a.sessions, tok)
		return false
	}
	return true
}

// NewPending creates a short-lived pre-2FA token bound to ip (5 minutes).
func (a *Auth) NewPending(ip string, remember bool) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	a.mu.Lock()
	a.pending[tok] = pendingAuth{ip: ip, remember: remember, expires: time.Now().Add(5 * time.Minute)}
	a.mu.Unlock()
	return tok
}

// ConsumePending validates and single-uses a pre-2FA token for ip,
// returning the remember-me flag carried from the password step.
func (a *Auth) ConsumePending(tok, ip string) (bool, bool) {
	if tok == "" {
		return false, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.pending[tok]
	if !ok || time.Now().After(p.expires) || p.ip != ip {
		delete(a.pending, tok)
		return false, false
	}
	delete(a.pending, tok)
	return true, p.remember
}

// DropPendingForIP revokes all pending 2FA tokens for ip (after too many
// wrong codes the user must redo the password step).
func (a *Auth) DropPendingForIP(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for tok, p := range a.pending {
		if p.ip == ip {
			delete(a.pending, tok)
		}
	}
}

// Revoke deletes a session token.
func (a *Auth) RevokeAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = make(map[string]session)
}

func (a *Auth) Revoke(tok string) {
	a.mu.Lock()
	delete(a.sessions, tok)
	a.mu.Unlock()
}

// RevokeByIP revokes all sessions bound to ip (e.g. after a one-click ban).
func (a *Auth) RevokeByIP(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for tok, s := range a.sessions {
		if s.ip == ip {
			delete(a.sessions, tok)
		}
	}
}

// SetCookie writes the session cookie.
func (a *Auth) SetCookie(w http.ResponseWriter, tok string, remember bool) {
	maxAge := int(sessionTTL.Seconds())
	if remember {
		maxAge = int(rememberTTL.Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    tok,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie removes the session cookie.
func (a *Auth) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// CookieToken extracts the session token from the request.
func CookieToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func (a *Auth) cleanupLoop() {
	t := time.NewTicker(cleanupEvery)
	defer t.Stop()
	for range t.C {
		a.mu.Lock()
		now := time.Now()
		for tok, s := range a.sessions {
			if now.After(s.expires) {
				delete(a.sessions, tok)
			}
		}
		for ip, rec := range a.fails {
			if now.Sub(rec.first) > failWindow {
				delete(a.fails, ip)
			}
		}
		for tok, p := range a.pending {
			if now.After(p.expires) {
				delete(a.pending, tok)
			}
		}
		a.mu.Unlock()
	}
}

// HashSecret hashes an arbitrary secret (e.g. the TG query password)
// with Argon2id and no length/complexity requirements.
func HashSecret(secret string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// NormalizeSecret trims whitespace and converts full-width characters
// (common with Chinese input methods) to half-width, so what the user
// sees is what gets hashed on both set and verify paths.
func NormalizeSecret(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			b.WriteRune(r - 0xFEE0)
		case r == 0x3000:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// VerifySecret checks a secret against a HashSecret hash.
func VerifySecret(encodedHash, secret string) bool {
	// Format: $argon2id$v=19$m=<m>,t=<t>,p=<p>$<saltB64>$<hashB64>
	// NOTE: do not use fmt.Sscanf here: %s does not stop at '$', so the
	// salt/hash split silently fails and verification always returns false.
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t, p int
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(secret), salt, uint32(t), uint32(m), uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
