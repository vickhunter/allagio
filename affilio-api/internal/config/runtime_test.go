package config

import "testing"

func TestProductionRefusesDefaultSecretAndWildcardOrigins(t *testing.T) {
	ready := Config{
		GinMode:        "release",
		APIKeySecret:   "not-the-dev-secret",
		AllowedOrigins: "https://pilot.example",
	}
	if err := ready.ValidateRuntime(); err != nil {
		t.Fatalf("explicit production config should start: %v", err)
	}

	refused := []Config{
		{GinMode: "release", APIKeySecret: "", AllowedOrigins: "https://pilot.example"},
		{GinMode: "release", APIKeySecret: "dev-secret", AllowedOrigins: "https://pilot.example"},
		{GinMode: "release", APIKeySecret: "  dev-secret  ", AllowedOrigins: "https://pilot.example"},
		{GinMode: "release", APIKeySecret: "not-the-dev-secret", AllowedOrigins: "*"},
		{GinMode: "release", APIKeySecret: "not-the-dev-secret", AllowedOrigins: " * "},
		{GinMode: "release", APIKeySecret: "not-the-dev-secret", AllowedOrigins: ""},
	}
	for _, cfg := range refused {
		if err := cfg.ValidateRuntime(); err == nil {
			t.Fatalf("production config %#v must refuse to start", cfg)
		}
	}

	dev := []Config{
		{GinMode: "", APIKeySecret: "dev-secret", AllowedOrigins: "*"},
		{GinMode: "debug", APIKeySecret: "", AllowedOrigins: "*"},
		{GinMode: "test", APIKeySecret: "dev-secret", AllowedOrigins: ""},
	}
	for _, cfg := range dev {
		if err := cfg.ValidateRuntime(); err != nil {
			t.Fatalf("non-production config %#v must still start: %v", cfg, err)
		}
	}
}

func TestSecretRefusedAndLoadHasNoDevDefault(t *testing.T) {
	if err := SecretRefused(""); err == nil || SecretRefused("dev-secret") == nil || SecretRefused("  dev-secret  ") == nil {
		t.Fatal("empty and dev-secret must be refused")
	}
	if err := SecretRefused("real-secret"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("API_KEY_SECRET", "")
	cfg := Load()
	if cfg.APIKeySecret != "" {
		t.Fatalf("API_KEY_SECRET defaulted to %q", cfg.APIKeySecret)
	}
	if err := cfg.ValidateSecrets(); err != nil {
		t.Fatal(err)
	}
}
