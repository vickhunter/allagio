package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/outbound"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

// allowLoopbackNotify is false in production. Tests set it so a httptest
// server on 127.0.0.1 or localhost can be saved and can receive a signed
// webhook. Private, link-local, and metadata addresses stay blocked either way.
var allowLoopbackNotify bool

// webhookGuard is the process resolver. Tests replace LookupIP and DialContext.
var webhookGuard outbound.Resolver

// notifyWaiting fires the owner's deal-waiting webhook when an effect just
// stored a waiting marker — a new deal's waiting, or an updated one from a
// counter, share hold, or revoke hold. Delivery is best-effort and async:
// one retry, then it is dropped; the escalation url always works by hand.
func (h *ORPHost) notifyWaiting(c *gin.Context, effect database.ORPEffect) {
	var raw json.RawMessage
	var dealID string
	switch {
	case len(effect.UpdateWaiting) > 0:
		raw = effect.UpdateWaiting
		dealID = effect.UpdateDealID
	case effect.Deal != nil && len(effect.Deal.Waiting) > 0:
		raw = effect.Deal.Waiting
		dealID = effect.Deal.ID
	}
	if len(raw) == 0 {
		return
	}
	var waiting map[string]any
	if json.Unmarshal(raw, &waiting) != nil {
		return
	}
	party, _ := waiting["party"].(string)
	if party == "" {
		return
	}
	_, dns := h.origin(c)
	ownerID, ok := orp.OwnerFromParty(dns, party)
	if !ok || ownerID == "" {
		return
	}
	url, secret, err := h.Entry.DB.OwnerNotify(c.Request.Context(), ownerID)
	if err == nil && url != "" && secret != "" {
		if payload, err := json.Marshal(map[string]any{
			"type":    "deal.waiting",
			"deal":    dealID,
			"waiting": waiting,
		}); err == nil {
			go postSignedWebhook(url, secret, payload)
		}
	}
	go telegramDM(h.Entry.DB, h.Entry.Telegram, ownerID, fmt.Sprintf("A deal (%s) is waiting on your party — act at %s/owner", dealID, strings.TrimRight(h.Entry.BaseURL, "/")))
}

// telegramDM pushes the same owner-facing events the webhook channel gets to
// a linked Telegram chat. It always runs on a detached context: callers invoke
// it from request handlers whose context dies with the response. Nil-safe and
// best-effort: an unlinked or unconfigured owner simply gets nothing.
func telegramDM(db *database.DB, tg *services.TelegramClient, ownerID, text string) {
	if tg == nil || !tg.Enabled() || db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	chatID, _, err := db.OwnerTelegram(ctx, ownerID)
	if err != nil || chatID == 0 {
		return
	}
	if err := tg.SendMessage(ctx, chatID, text); err != nil {
		log.Printf("telegram: dm to owner %s failed: %v", ownerID, err)
	}
}

func postSignedWebhook(rawURL, secret string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = deliverSignedWebhook(ctx, rawURL, secret, payload)
}

func deliverSignedWebhook(ctx context.Context, rawURL, secret string, payload []byte) error {
	guard := guardFor(rawURL)
	if err := notifyTargetAllowed(ctx, rawURL, allowLoopbackNotify); err != nil {
		return err
	}
	ts := fmt.Sprint(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write(payload)
	sig := fmt.Sprintf("t=%s,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
	header := http.Header{
		"Content-Type":        []string{"application/json"},
		"X-Allagio-Signature": []string{sig},
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		// Check again so a DNS change between attempts cannot retarget the post.
		if err := guard.Check(ctx, rawURL); err != nil {
			return err
		}
		resp, err := sendOwnerWebhook(ctx, guard, rawURL, payload, header)
		if err != nil {
			if errors.Is(err, outbound.ErrBlocked) || errors.Is(err, outbound.ErrInvalid) {
				return err
			}
			last = err
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			// Redirects are not followed. A 307 or 308 would resend the signed body.
			return fmt.Errorf("webhook redirect %d", resp.StatusCode)
		}
		if resp.StatusCode < 500 {
			return nil
		}
		last = fmt.Errorf("status %d", resp.StatusCode)
	}
	if last == nil {
		last = errors.New("webhook failed")
	}
	return last
}

// sendOwnerWebhook is the guarded poster. Redirects are checked with the same
// blocked-address rule as the first hop, and the dial uses the pinned IP.
func sendOwnerWebhook(ctx context.Context, guard outbound.Resolver, rawURL string, payload []byte, header http.Header) (*http.Response, error) {
	return guard.Do(ctx, http.MethodPost, rawURL, payload, outbound.Options{
		Timeout:    4 * time.Second,
		Header:     header,
		NoRedirect: true,
	})
}

func guardFor(raw string) outbound.Resolver {
	guard := webhookGuard
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err == nil && allowLoopbackNotify && parsed.Scheme == "http" && loopbackName(parsed.Hostname()) {
		guard.AllowLoopback = true
	}
	return guard
}

// notifyTargetAllowed is the save-time and send-time URL policy.
// Production accepts only https URLs whose addresses are public.
// allowLoopback additionally accepts http://127.0.0.1 and http://localhost,
// which is how tests receive a webhook on httptest. It does not accept
// private, link-local, or metadata addresses.
func notifyTargetAllowed(ctx context.Context, raw string, allowLoopback bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return outbound.ErrInvalid
	}
	loopback := allowLoopback && parsed.Scheme == "http" && loopbackName(parsed.Hostname())
	if parsed.Scheme != "https" && !loopback {
		return outbound.ErrInvalid
	}
	guard := webhookGuard
	guard.AllowLoopback = loopback
	return guard.Check(ctx, raw)
}

func loopbackName(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "localhost" || host == "127.0.0.1"
}

func newNotifySecret() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (h *HostedHandler) notifySection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	url, _, err := h.DB.OwnerNotify(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<section id="notify"><h2>Deal-waiting webhook</h2>`)
	b.WriteString(`<p>Allagio POSTs signed events here: <code>deal.waiting</code> when a deal waits on your party, <code>account.registered</code> when an agent opens an account for you, and <code>assessment.finished</code> when a product readiness check completes (score, status, fix_hints). Leave it empty to get no calls.</p><form method="post" action="/owner/notify">`)
	b.WriteString(`<input type="hidden" name="csrf" value="`)
	b.WriteString(template.HTMLEscapeString(o.CSRF))
	b.WriteString(`"><label>Webhook url<input type="url" name="url" maxlength="500" value="`)
	b.WriteString(template.HTMLEscapeString(url))
	b.WriteString(`" placeholder="https://your-server.example/orp-waiting"></label><button>Save</button></form>`)
	if url != "" {
		b.WriteString(`<p class="caption">A signing secret is set. Verify <code>X-Allagio-Signature</code> as <code>t=&lt;unix&gt;,v1=&lt;hmac-sha256 hex of t+body&gt;</code>.</p>`)
	}
	b.WriteString(`</section>`)
	optOut, _, err := h.DB.OwnerDigestPrefs(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	chatID, tgUser, err := h.DB.OwnerTelegram(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	botEnabled := h.Telegram != nil && h.Telegram.Enabled()
	b.WriteString(`<section id="telegram"><h2>Telegram</h2>`)
	if chatID != 0 {
		label := "chat " + fmt.Sprint(chatID)
		if tgUser != "" {
			label = "@" + template.HTMLEscapeString(tgUser)
		}
		fmt.Fprintf(&b, `<p>Linked to <strong>%s</strong>. Deal-waiting alerts, assessment results and registration notices arrive as direct messages; /status and /earnings answer from the bot.</p>`, label)
		b.WriteString(`<form method="post" action="/owner/telegram/unlink"><input type="hidden" name="csrf" value="`)
		b.WriteString(template.HTMLEscapeString(o.CSRF))
		b.WriteString(`"><button>Unlink Telegram</button></form>`)
	} else if botEnabled {
		bot := "the Allagio bot"
		if h.Telegram.Username != "" {
			bot = "@" + template.HTMLEscapeString(h.Telegram.Username)
		}
		fmt.Fprintf(&b, `<p>Talk to your account from Telegram: deal alerts and /status / /earnings as direct messages. Generate a link code, then send <code>/link CODE</code> to %s.</p>`, bot)
		if code := strings.TrimSpace(c.Query("tg")); code != "" {
			fmt.Fprintf(&b, `<p>Your one-time code (valid ~15 min): <code>%s</code> — send <code>/link %s</code> to %s.</p>`, template.HTMLEscapeString(code), template.HTMLEscapeString(code), bot)
		}
		b.WriteString(`<form method="post" action="/owner/telegram/code"><input type="hidden" name="csrf" value="`)
		b.WriteString(template.HTMLEscapeString(o.CSRF))
		b.WriteString(`"><button>Generate link code</button></form>`)
	} else {
		b.WriteString(`<p class="caption">The Telegram channel is not enabled on this deployment.</p>`)
	}
	b.WriteString(`</section>`)
	b.WriteString(`<section id="digest"><h2>Weekly digest email</h2>`)
	b.WriteString(`<p>A short summary each week: what went live, clicks, and test-mode earnings, mailed to your sign-in address.</p><form method="post" action="/owner/digest">`)
	b.WriteString(`<input type="hidden" name="csrf" value="`)
	b.WriteString(template.HTMLEscapeString(o.CSRF))
	b.WriteString(`"><label><input type="checkbox" name="digest" value="on"`)
	if !optOut {
		b.WriteString(` checked`)
	}
	b.WriteString(`> Send me the weekly digest</label><button>Save</button></form></section>`)
	return template.HTML(b.String()), nil
}

// CreateTelegramCodeForm mints the one-time code an owner sends to the bot as
// /link CODE; the code shows once in the settings page flash (?tg=).
func (h *HostedHandler) CreateTelegramCodeForm(c *gin.Context) {
	o := ownerFrom(c)
	if h.Telegram == nil || !h.Telegram.Enabled() {
		// No bot is polling this deployment: a minted code could never be
		// claimed, so don't offer one.
		c.Redirect(http.StatusSeeOther, "/owner/settings")
		return
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	code := strings.ToUpper(hex.EncodeToString(b[:]))
	if err := h.DB.CreateTelegramLinkCode(c.Request.Context(), o.ID, services.HashAPIKey(code), time.Now().Add(15*time.Minute)); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings?tg="+code)
}

func (h *HostedHandler) UnlinkTelegramForm(c *gin.Context) {
	o := ownerFrom(c)
	if err := h.DB.UnlinkTelegram(c.Request.Context(), o.ID); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}

func (h *HostedHandler) SaveDigestForm(c *gin.Context) {
	o := ownerFrom(c)
	if err := h.DB.SetOwnerDigestOptOut(c.Request.Context(), o.ID, c.PostForm("digest") != "on"); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}

func (h *HostedHandler) SaveNotifyForm(c *gin.Context) {
	o := ownerFrom(c)
	rawURL := strings.TrimSpace(c.PostForm("url"))
	if err := notifyTargetAllowed(c.Request.Context(), rawURL, allowLoopbackNotify); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	_, secret, err := h.DB.OwnerNotify(c.Request.Context(), o.ID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if rawURL != "" && secret == "" {
		secret = newNotifySecret()
	}
	if rawURL == "" {
		secret = ""
	}
	if err := h.DB.SetOwnerNotify(c.Request.Context(), o.ID, rawURL, secret); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}
