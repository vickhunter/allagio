package handlers

import "testing"

func TestHeldResponseOmitsError(t *testing.T) {
	body := heldResponse("placement_publish", "domain_unverified", "This domain is not verified, so placement publish waits for approval.")
	requireHold(t, body, "domain_unverified")
	if body["step"] != "placement_publish" || body["message"] == "" {
		t.Fatalf("held fields: %v", body)
	}
	refused := stepBody("placement_publish", "step_off", "off")
	if refused["error"] != "off" || refused["hold_reason"] != nil {
		t.Fatalf("refusal: %v", refused)
	}
}

func requireHold(t *testing.T, body map[string]any, reason string) {
	t.Helper()
	if _, ok := body["error"]; ok {
		t.Fatalf("hold body has error: %v", body)
	}
	if body["status"] != "awaiting_approval" || body["hold_reason"] != reason || body["reason"] != reason {
		t.Fatalf("hold body: %v", body)
	}
}
