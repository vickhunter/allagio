package services

import "testing"

func TestConnectionCipherBindsSecretsToProduct(t *testing.T) {
	c, err := NewConnectionCipher("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := c.Encrypt("product-a", "whsec_secret")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := c.Decrypt("product-a", encrypted)
	if err != nil || secret != "whsec_secret" {
		t.Fatalf("decrypt: %q %v", secret, err)
	}
	if _, err := c.Decrypt("product-b", encrypted); err == nil {
		t.Fatal("secret transferable to another product")
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err := c.Decrypt("product-a", encrypted); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
