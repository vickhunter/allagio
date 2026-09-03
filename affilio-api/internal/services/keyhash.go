package services

import (
	"crypto/sha256"
	"encoding/hex"
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
