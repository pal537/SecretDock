package secpolicy

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"strings"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

// totpIssuer is shown in authenticator apps.
const totpIssuer = "SecretDock"

// GenerateTOTP creates a new TOTP credential. Returns the base32 secret
// (for manual entry) and the otpauth:// URI (for the QR code).
func GenerateTOTP(account string) (secret, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: account,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks a 6-digit code, allowing ±1 time step of clock skew.
func ValidateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	return totp.Validate(code, secret)
}

// QRCodeDataURI renders the otpauth URI as a PNG data URI for <img>.
func QRCodeDataURI(uri string) (string, error) {
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// NewRecoveryCodes generates n printable single-use recovery codes
// in the form XXXX-XXXX.
func NewRecoveryCodes(n int) ([]string, error) {
	codes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
		s = strings.TrimRight(s, "=")
		if len(s) < 8 {
			return nil, errShortCode
		}
		codes = append(codes, s[:4]+"-"+s[4:8])
	}
	return codes, nil
}

// NormalizeRecoveryCode trims spaces/dashes so "xxxx-xxxx" and
// "xxxxxxxx" both match.
func NormalizeRecoveryCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

type codeErr string

func (e codeErr) Error() string { return string(e) }

const errShortCode codeErr = "random source produced short code"
