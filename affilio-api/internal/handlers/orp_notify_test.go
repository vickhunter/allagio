package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type notifyEvent struct {
	payload map[string]any
	raw     []byte
	sig     string
}

// A directed offer puts a waiting marker on the acceptor; the acceptor's owner
// webhook receives a signed deal.waiting event.
func TestORPNotifyWaitingWebhook(t *testing.T) {
	events := make(chan notifyEvent, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		events <- notifyEvent{payload: payload, raw: body, sig: r.Header.Get("X-Allagio-Signature")}
	}))
	defer srv.Close()

	allowLoopbackNotify = true
	t.Cleanup(func() { allowLoopbackNotify = false })

	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	_, publisher := joinParty(t, r)
	if err := entry.DB.SetOwnerNotify(context.Background(), ownerOfParty(publisher), srv.URL, "testsecret"); err != nil {
		t.Fatal(err)
	}

	dealID, _ := shareDeal(t, r, makerToken, maker, publisher)

	select {
	case ev := <-events:
		if ev.payload["type"] != "deal.waiting" || ev.payload["deal"] != dealID {
			t.Fatalf("webhook payload %v", ev.payload)
		}
		waiting, _ := ev.payload["waiting"].(map[string]any)
		if waiting["party"] != publisher {
			t.Fatalf("waiting.party %v want %s", waiting["party"], publisher)
		}
		parts := strings.Split(ev.sig, ",")
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "t=") || !strings.HasPrefix(parts[1], "v1=") {
			t.Fatalf("signature %q", ev.sig)
		}
		mac := hmac.New(sha256.New, []byte("testsecret"))
		mac.Write([]byte(strings.TrimPrefix(parts[0], "t=")))
		mac.Write(ev.raw)
		if parts[1] != "v1="+hex.EncodeToString(mac.Sum(nil)) {
			t.Fatalf("bad signature %q", ev.sig)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no webhook call within 5s")
	}
}

// An owner with no webhook configured generates no outbound call — the offer
// itself must still succeed.
func TestORPNotifyWaitingNoURL(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	_, publisher := joinParty(t, r)
	if dealID, _ := shareDeal(t, r, makerToken, maker, publisher); dealID == "" {
		t.Fatal("offer failed without webhook")
	}
}
