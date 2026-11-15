package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
)

// TelegramRunner is the owner-facing chat channel: it long-polls the Bot API
// and answers commands. Owners link a chat with a one-time code generated in
// /owner/settings; a linked chat also receives the same deal-waiting,
// assessment.finished and account.registered events the webhook channel gets.
// Disabled entirely when no bot token is configured.
type TelegramRunner struct {
	DB       *database.DB
	TG       *services.TelegramClient
	BaseURL  string
	PollSecs int
}

func (r *TelegramRunner) RunWorker(ctx context.Context) {
	if r.TG == nil || !r.TG.Enabled() {
		return
	}
	poll := r.PollSecs
	if poll <= 0 {
		poll = 25
	}
	var offset int64
	for {
		updates, err := r.TG.GetUpdates(ctx, offset, poll)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram: getUpdates failed: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if u.Message != nil && strings.TrimSpace(u.Message.Text) != "" {
				r.handle(ctx, u.Message)
			}
		}
	}
}

func (r *TelegramRunner) handle(ctx context.Context, m *services.TelegramMessage) {
	chatID := m.Chat.ID
	fields := strings.Fields(m.Text)
	cmd := strings.ToLower(fields[0])
	if i := strings.IndexByte(cmd, '@'); i >= 0 {
		cmd = cmd[:i] // /cmd@BotName in groups
	}
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m.Text), fields[0]))

	reply := func(text string) {
		if err := r.TG.SendMessage(ctx, chatID, text); err != nil && ctx.Err() == nil {
			log.Printf("telegram: send to chat %d failed: %v", chatID, err)
		}
	}

	switch cmd {
	case "/start", "/help":
		reply(r.helpText(m.From.FirstName))
	case "/link":
		if arg == "" {
			reply("Usage: /link CODE — generate the code in your Allagio workspace under Settings.")
			return
		}
		ownerID, ok, err := r.DB.ClaimTelegramCode(ctx, services.HashAPIKey(strings.ToUpper(arg)))
		if err != nil {
			log.Printf("telegram: claim code failed: %v", err)
			reply("Something went wrong, try again in a moment.")
			return
		}
		if !ok {
			reply("That code is unknown or expired — generate a fresh one at " + r.base() + "/owner/settings.")
			return
		}
		if err := r.DB.BindTelegramChat(ctx, ownerID, chatID, m.From.Username); err != nil {
			log.Printf("telegram: bind chat failed: %v", err)
			reply("Something went wrong, try again in a moment.")
			return
		}
		o, _ := r.DB.OwnerByTelegramChat(ctx, chatID)
		name := ""
		if o != nil {
			name = " (" + o.Email + ")"
		}
		reply("Linked" + name + ". You'll now get deal-waiting alerts, assessment results and registration notices here. /status for your account, /unlink to disconnect.")
	case "/status":
		o := r.requireLinked(ctx, chatID, reply)
		if o == nil {
			return
		}
		digest, err := r.DB.AccountDigest(ctx, o.ID)
		if err != nil {
			log.Printf("telegram: digest for owner %s failed: %v", o.ID, err)
			reply("Couldn't load your account right now.")
			return
		}
		reply(digestTelegramText(digest, r.base()))
	case "/earnings":
		o := r.requireLinked(ctx, chatID, reply)
		if o == nil {
			return
		}
		digest, err := r.DB.AccountDigest(ctx, o.ID)
		if err != nil {
			log.Printf("telegram: digest for owner %s failed: %v", o.ID, err)
			reply("Couldn't load your account right now.")
			return
		}
		reply(earningsTelegramText(digest))
	case "/unlink":
		o := r.requireLinked(ctx, chatID, reply)
		if o == nil {
			return
		}
		if err := r.DB.UnlinkTelegram(ctx, o.ID); err != nil {
			log.Printf("telegram: unlink owner %s failed: %v", o.ID, err)
			reply("Couldn't unlink right now.")
			return
		}
		reply("Unlinked. No more Allagio messages here — relink any time from " + r.base() + "/owner/settings.")
	default:
		reply("Commands: /link CODE, /status, /earnings, /unlink, /help")
	}
}

func (r *TelegramRunner) requireLinked(ctx context.Context, chatID int64, reply func(string)) *database.Owner {
	o, err := r.DB.OwnerByTelegramChat(ctx, chatID)
	if err != nil {
		log.Printf("telegram: owner lookup failed: %v", err)
		reply("Something went wrong, try again in a moment.")
		return nil
	}
	if o == nil {
		reply("This chat isn't linked to an Allagio account — generate a link code at " + r.base() + "/owner/settings, then send /link CODE here.")
		return nil
	}
	return o
}

func (r *TelegramRunner) helpText(firstName string) string {
	name := strings.TrimSpace(firstName)
	if name != "" {
		name = " " + name
	}
	return fmt.Sprintf("Hi%s — this is the Allagio agent channel. Your agents negotiate deals on ORP and report here.\n\nCommands:\n/link CODE — connect this chat to your account (code from %s/owner/settings)\n/status — what's live, clicks, earnings\n/earnings — earnings only\n/unlink — disconnect\n\nEverything also lives at %s/owner.", name, r.base(), r.base())
}

func (r *TelegramRunner) base() string {
	return strings.TrimRight(r.BaseURL, "/")
}

func digestTelegramText(d database.AccountDigest, base string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Live now: %d item(s) · Clicks: %d\n", d.WentLive, d.Clicks)
	writeTelegramAmounts(&b, "Publisher earnings", d.PublisherEarnings)
	writeTelegramAmounts(&b, "Maker earnings", d.MakerEarnings)
	for _, p := range d.Projects {
		fmt.Fprintf(&b, "• %s — %d live, %d clicks\n", p.Name, p.WentLive, p.Clicks)
	}
	fmt.Fprintf(&b, "\n%s/owner", base)
	return b.String()
}

func earningsTelegramText(d database.AccountDigest) string {
	var b strings.Builder
	writeTelegramAmounts(&b, "Publisher earnings", d.PublisherEarnings)
	writeTelegramAmounts(&b, "Maker earnings", d.MakerEarnings)
	if b.Len() == 0 {
		return "No earnings yet (test-mode payments only)."
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func writeTelegramAmounts(b *strings.Builder, label string, amounts []database.MoneyAmount) {
	if len(amounts) == 0 {
		return
	}
	var parts []string
	for _, e := range amounts {
		parts = append(parts, fmt.Sprintf("%.2f %s", e.Amount, e.Currency))
	}
	fmt.Fprintf(b, "%s: %s\n", label, strings.Join(parts, ", "))
}
