package config

import "testing"

func TestResolveClaimsCheck(t *testing.T) {
	cases := []struct {
		name    string
		ginMode string
		raw     string
		want    string
		fail    bool
	}{
		{name: "hosted empty", ginMode: "release", raw: "", want: ClaimsStub},
		{name: "hosted mixed case", ginMode: " Release ", raw: "  ", want: ClaimsStub},
		{name: "hosted explicit stub", ginMode: "release", raw: " STUB ", want: ClaimsStub},
		{name: "hosted real", ginMode: "release", raw: "real", fail: true},
		{name: "hosted off-local", ginMode: "RELEASE", raw: "off-local", fail: true},
		{name: "local empty", ginMode: "", raw: "", want: ClaimsOffLocal},
		{name: "debug empty", ginMode: "debug", raw: "", want: ClaimsOffLocal},
		{name: "test empty", ginMode: "test", raw: "", want: ClaimsOffLocal},
		{name: "local real", ginMode: "", raw: "real", want: ClaimsReal},
		{name: "local off-local", ginMode: "debug", raw: "off-local", want: ClaimsOffLocal},
		{name: "local stub", ginMode: "debug", raw: "stub", want: ClaimsStub},
		{name: "unknown", ginMode: "", raw: "live", fail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveClaimsCheck(tc.ginMode, tc.raw)
			if tc.fail {
				if err == nil {
					t.Fatalf("ResolveClaimsCheck(%q, %q) = %q, want an error", tc.ginMode, tc.raw, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ResolveClaimsCheck(%q, %q) = %q, %v; want %q", tc.ginMode, tc.raw, got, err, tc.want)
			}
		})
	}
}

func TestLoadResolvesClaimsCheck(t *testing.T) {
	t.Setenv("GIN_MODE", "")
	t.Setenv("CLAIMS_CHECK", "")
	cfg := Load()
	if cfg.ClaimsCheck != ClaimsOffLocal || cfg.ValidateClaims() != nil {
		t.Fatalf("local empty: %q %v", cfg.ClaimsCheck, cfg.ValidateClaims())
	}

	t.Setenv("GIN_MODE", "release")
	cfg = Load()
	if cfg.ClaimsCheck != ClaimsStub || cfg.ValidateClaims() != nil {
		t.Fatalf("hosted empty: %q %v", cfg.ClaimsCheck, cfg.ValidateClaims())
	}

	t.Setenv("CLAIMS_CHECK", "real")
	cfg = Load()
	if cfg.ValidateClaims() == nil {
		t.Fatal("hosted real must refuse to start")
	}

	t.Setenv("CLAIMS_CHECK", "off-local")
	cfg = Load()
	if cfg.ValidateClaims() == nil {
		t.Fatal("hosted off-local must refuse to start")
	}

	t.Setenv("GIN_MODE", "debug")
	t.Setenv("CLAIMS_CHECK", "real")
	cfg = Load()
	if cfg.ClaimsCheck != ClaimsReal || cfg.ValidateClaims() != nil {
		t.Fatalf("local real: %q %v", cfg.ClaimsCheck, cfg.ValidateClaims())
	}

	t.Setenv("CLAIMS_CHECK", "nope")
	cfg = Load()
	if cfg.ValidateClaims() == nil {
		t.Fatal("unknown CLAIMS_CHECK must refuse to start")
	}
}
