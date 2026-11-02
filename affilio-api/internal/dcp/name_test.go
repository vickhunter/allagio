package dcp

import "testing"

func TestGetenvPrefersNewName(t *testing.T) {
	t.Setenv(EnvAPIURL, "https://new.example")
	t.Setenv(LegacyEnvAPIURL, "https://old.example")
	if got := Getenv(EnvAPIURL, LegacyEnvAPIURL); got != "https://new.example" {
		t.Fatalf("primary %q", got)
	}

	t.Setenv(EnvAPIKey, "  ")
	t.Setenv(LegacyEnvAPIKey, "legacy-key")
	if got := Getenv(EnvAPIKey, LegacyEnvAPIKey); got != "legacy-key" {
		t.Fatalf("blank primary should use the legacy value, got %q", got)
	}

	t.Setenv(EnvSiteKey, "")
	t.Setenv(LegacyEnvSiteKey, "  site-key  ")
	if got := Getenv(EnvSiteKey, LegacyEnvSiteKey); got != "site-key" {
		t.Fatalf("legacy trim %q", got)
	}

	t.Setenv(EnvAPIURL, "")
	t.Setenv(LegacyEnvAPIURL, "")
	t.Setenv(OlderEnvAPIURL, "https://older.example")
	if got := Getenv(EnvAPIURL, LegacyEnvAPIURL, OlderEnvAPIURL); got != "https://older.example" {
		t.Fatalf("older %q", got)
	}
	t.Setenv(LegacyEnvAPIURL, "https://legacy.example")
	if got := Getenv(EnvAPIURL, LegacyEnvAPIURL, OlderEnvAPIURL); got != "https://legacy.example" {
		t.Fatalf("legacy before older %q", got)
	}
}
