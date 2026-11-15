package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

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
	if err != nil || url == "" || secret == "" {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type":    "deal.waiting",
		"deal":    dealID,
		"waiting": waiting,
	})
	if err != nil {
		return
	}
	go postSignedWebhook(url, secret, payload)
}

func postSignedWebhook(url, secret string, payload []byte) {
	ts := fmt.Sprint(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write(payload)
	sig := fmt.Sprintf("t=%s,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
	client := &http.Client{Timeout: 4 * time.Second}
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			time.Sleep(2 * time.Second)
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Allagio-Signature", sig)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return
			}
			continue
		}
	}
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
	url := strings.TrimSpace(c.PostForm("url"))
	if url != "" && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://127.0.0.1") && !strings.HasPrefix(url, "http://localhost") {
		c.Status(http.StatusBadRequest)
		return
	}
	_, secret, err := h.DB.OwnerNotify(c.Request.Context(), o.ID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if url != "" && secret == "" {
		secret = newNotifySecret()
	}
	if url == "" {
		secret = ""
	}
	if err := h.DB.SetOwnerNotify(c.Request.Context(), o.ID, url, secret); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}
