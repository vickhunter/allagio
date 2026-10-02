package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

type ConnectionCipher struct{ aead cipher.AEAD }

func NewConnectionCipher(encodedKey string) (*ConnectionCipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("CONNECTION_ENCRYPTION_KEY must encode 32 bytes in base64")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &ConnectionCipher{aead: aead}, nil
}

func (c *ConnectionCipher) Encrypt(productID, secret string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(secret), []byte(productID)), nil
}

func (c *ConnectionCipher) Decrypt(productID string, ciphertext []byte) (string, error) {
	size := c.aead.NonceSize()
	if len(ciphertext) < size {
		return "", errors.New("invalid encrypted connection")
	}
	plain, err := c.aead.Open(nil, ciphertext[:size], ciphertext[size:], []byte(productID))
	return string(plain), err
}
