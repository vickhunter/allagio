package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
)

// fakeTelegram records sendMessage calls and answers getUpdates/getMe the way
// the real Bot API shapes them.
type fakeTelegram struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeTelegram) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /bottest-token/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.sent = append(f.sent, body.Text)
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /bottest-token/getMe", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"allagio_test_bot"}}`))
	})
	return mux
}

func (f *fakeTelegram) last(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("no telegram messages sent")
	}
	return f.sent[len(f.sent)-1]
}

func telegramTestClient(api string) *services.TelegramClient {
	return &services.TelegramClient{Token: "test-token", API: api}
}

func telegramMsg(chatID int64, text string) *services.TelegramMessage {
	var m services.TelegramMessage
	m.Chat.ID = chatID
	m.Chat.Type = "private"
	m.From.Username = "vick"
	m.From.FirstName = "Vic"
	m.Text = text
	return &m
}

func telegramRunner(db *database.DB, tg *services.TelegramClient) *TelegramRunner {
	return &TelegramRunner{DB: db, TG: tg, BaseURL: "https://allagio.dev"}
}

func TestTelegramLinkFlow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	r := telegramRunner(db, telegramTestClient(srv.URL))

	ownerID := insertDigestOwner(t, db, "tg@example.com", false, nil)
	code := "ABCD1234"
	if err := db.CreateTelegramLinkCode(ctx, ownerID, services.HashAPIKey(code), time.Now().Add(15*time.Minute)); err != nil {
		t.Fatalf("create code: %v", err)
	}

	r.handle(ctx, telegramMsg(777, "/link "+code))

	got := fake.last(t)
	if !strings.Contains(got, "Linked") || !strings.Contains(got, "tg@example.com") {
		t.Fatalf("unexpected link reply: %q", got)
	}
	chatID, user, err := db.OwnerTelegram(ctx, ownerID)
	if err != nil || chatID != 777 || user != "vick" {
		t.Fatalf("bind mismatch: chat=%d user=%q err=%v", chatID, user, err)
	}

	// The code is one-time: replaying it now fails.
	r.handle(ctx, telegramMsg(888, "/link "+code))
	if !strings.Contains(fake.last(t), "unknown or expired") {
		t.Fatalf("replayed code should be rejected: %q", fake.last(t))
	}
}

func TestTelegramStatusRequiresLink(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	r := telegramRunner(db, telegramTestClient(srv.URL))

	r.handle(ctx, telegramMsg(555, "/status"))
	if got := fake.last(t); !strings.Contains(got, "isn't linked") {
		t.Fatalf("unlinked /status should point at linking: %q", got)
	}

	ownerID := insertDigestOwner(t, db, "linked@example.com", false, nil)
	if err := db.BindTelegramChat(ctx, ownerID, 555, "vick"); err != nil {
		t.Fatalf("bind: %v", err)
	}
	r.handle(ctx, telegramMsg(555, "/status"))
	got := fake.last(t)
	if !strings.Contains(got, "Live now:") || !strings.Contains(got, "allagio.dev/owner") {
		t.Fatalf("/status should report the digest summary: %q", got)
	}

	r.handle(ctx, telegramMsg(555, "/unlink"))
	r.handle(ctx, telegramMsg(555, "/status"))
	if got := fake.last(t); !strings.Contains(got, "isn't linked") {
		t.Fatalf("/status after unlink should refuse: %q", got)
	}
}

func TestTelegramDMNotifiesLinkedOwner(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	tg := telegramTestClient(srv.URL)

	ownerID := insertDigestOwner(t, db, "dm@example.com", false, nil)

	// Unlinked owner gets nothing.
	telegramDM(db, tg, ownerID, "deal waiting")
	fake.mu.Lock()
	if len(fake.sent) != 0 {
		t.Fatalf("expected no dm before link, got %v", fake.sent)
	}
	fake.mu.Unlock()

	if err := db.BindTelegramChat(ctx, ownerID, 4242, "vick"); err != nil {
		t.Fatalf("bind: %v", err)
	}
	telegramDM(db, tg, ownerID, "deal waiting")
	if got := fake.last(t); got != "deal waiting" {
		t.Fatalf("dm text mismatch: %q", got)
	}

	// A disabled client is a no-op.
	telegramDM(db, &services.TelegramClient{}, ownerID, "never sent")
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.sent) != 1 {
		t.Fatalf("disabled client should not send: %v", fake.sent)
	}
}

func TestTelegramGroupChatIsRefused(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	r := telegramRunner(db, telegramTestClient(srv.URL))

	ownerID := insertDigestOwner(t, db, "grp@example.com", false, nil)
	code := "GROUPCD1"
	if err := db.CreateTelegramLinkCode(ctx, ownerID, services.HashAPIKey(code), time.Now().Add(15*time.Minute)); err != nil {
		t.Fatalf("create code: %v", err)
	}
	m := telegramMsg(-999, "/link "+code)
	m.Chat.Type = "group"
	r.handle(ctx, m)

	if got := fake.last(t); !strings.Contains(got, "direct messages") {
		t.Fatalf("group /link should be refused: %q", got)
	}
	chatID, _, _ := db.OwnerTelegram(ctx, ownerID)
	if chatID != 0 {
		t.Fatalf("group chat must not bind: chat=%d", chatID)
	}
}

func TestTelegramRelinkStealsChatFromPreviousOwner(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	first := insertDigestOwner(t, db, "first@example.com", false, nil)
	second := insertDigestOwner(t, db, "second@example.com", false, nil)
	if err := db.BindTelegramChat(ctx, first, 9090, "one"); err != nil {
		t.Fatalf("bind first: %v", err)
	}
	if err := db.BindTelegramChat(ctx, second, 9090, "two"); err != nil {
		t.Fatalf("bind second: %v", err)
	}

	chatID, _, _ := db.OwnerTelegram(ctx, first)
	if chatID != 0 {
		t.Fatalf("previous owner must lose the chat: chat=%d", chatID)
	}
	o, err := db.OwnerByTelegramChat(ctx, 9090)
	if err != nil || o == nil || o.ID != second {
		t.Fatalf("chat must resolve to the new owner only: %+v err=%v", o, err)
	}
}

func TestTelegramUnknownCommandGetsHelp(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	r := telegramRunner(db, telegramTestClient(srv.URL))

	r.handle(ctx, telegramMsg(1, "hello there"))
	if got := fake.last(t); !strings.Contains(got, "/link") {
		t.Fatalf("unknown text should get the command list: %q", got)
	}
}
