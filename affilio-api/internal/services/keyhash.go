package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashAPIKey returns the SHA-256 hex digest of a plaintext API key.
func HashAPIKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

// KeyPrefix returns the first 8 characters of a plaintext API key for indexed lookup.
func KeyPrefix(plaintext string) string {
	if len(plaintext) <= 8 {
		return plaintext
	}
	return plaintext[:8]
}

// HashClientAddress is the lowercase hex HMAC-SHA256 of a client address.
// The key is API_KEY_SECRET itself. It is not the daily go-link salt.
func HashClientAddress(key []byte, address string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(address))
	return hex.EncodeToString(mac.Sum(nil))
}

// ClientAddressHMACKey returns API_KEY_SECRET as the HMAC key. An empty
// secret returns a random 32-byte key and a warning the process should log.
// The caller keeps the returned key for the life of the process.
func ClientAddressHMACKey(secret string) (key []byte, warning string) {
	secret = strings.TrimSpace(secret)
	if secret != "" {
		return []byte(secret), ""
	}
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return buf[:], "warning: API_KEY_SECRET is unset; client-address hashes use a random per-process key"
}
