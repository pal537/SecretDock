// Package crypto provides AES-256-GCM encryption for stored secrets.
// The master key is injected at runtime via SECRETDOCK_MASTER_KEY
// (base64-encoded 32 bytes) or auto-generated on first start.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKey is returned when the provided master key is malformed.
var ErrInvalidKey = errors.New("crypto: master key must be 32 bytes, base64-encoded")

// KeyFromEnv parses a base64-encoded 32-byte key from an environment value.
func KeyFromEnv(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	return key, nil
}

// GenerateKey creates a fresh random 32-byte key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt encrypts plaintext with AES-256-GCM and returns base64 output.
func Encrypt(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt.
func Decrypt(key []byte, encoded string) ([]byte, error) {
	ct, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ct) < nonceSize {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, ciphertext := ct[:nonceSize], ct[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
