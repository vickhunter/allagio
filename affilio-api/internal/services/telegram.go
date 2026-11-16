package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TelegramClient is a thin Bot API client: long-poll getUpdates plus
// sendMessage. It is enabled only when a bot token is configured; every
// caller treats a nil or disabled client as "no Telegram channel".
type TelegramClient struct {
	Token    string
	API      string // base URL, default https://api.telegram.org (overridden in tests)
	HTTP     *http.Client
	Username string // bot username from getMe, for display
}

type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

type TelegramMessage struct {
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"` // "private", "group", "supergroup", "channel"
	} `json:"chat"`
	From struct {
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	} `json:"from"`
	Text string `json:"text"`
}

func (c *TelegramClient) Enabled() bool {
	return c != nil && strings.TrimSpace(c.Token) != ""
}

func (c *TelegramClient) apiURL(method string) string {
	api := strings.TrimRight(c.API, "/")
	if api == "" {
		api = "https://api.telegram.org"
	}
	return api + "/bot" + c.Token + "/" + method
}

func (c *TelegramClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	//allagio:outbound fixed-host Telegram Bot API
	return &http.Client{Timeout: 45 * time.Second}
}

// GetUpdates long-polls the Bot API: the server holds the request up to
// timeoutSec for new updates. offset is the next update id to confirm.
func (c *TelegramClient) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]TelegramUpdate, error) {
	body, err := json.Marshal(map[string]any{"offset": offset, "timeout": timeoutSec, "allowed_updates": []string{"message"}})
	if err != nil {
		return nil, err
	}
	var out struct {
		OK          bool             `json:"ok"`
		Description string           `json:"description"`
		Result      []TelegramUpdate `json:"result"`
	}
	if err := c.call(ctx, "/getUpdates", body, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, errors.New("getUpdates failed: " + out.Description)
	}
	return out.Result, nil
}

func (c *TelegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	body, err := json.Marshal(map[string]any{"chat_id": chatID, "text": text, "disable_web_page_preview": true})
	if err != nil {
		return err
	}
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := c.call(ctx, "/sendMessage", body, &out); err != nil {
		return err
	}
	if !out.OK {
		return errors.New("sendMessage failed: " + out.Description)
	}
	return nil
}

// GetMe caches the bot's username for display (settings page, welcome text).
func (c *TelegramClient) GetMe(ctx context.Context) error {
	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := c.call(ctx, "/getMe", nil, &out); err != nil {
		return err
	}
	if !out.OK {
		return errors.New("getMe failed")
	}
	c.Username = out.Result.Username
	return nil
}

func (c *TelegramClient) call(ctx context.Context, methodPath string, body []byte, out any) error {
	method := strings.TrimPrefix(methodPath, "/")
	url := c.apiURL(method)
	var req *http.Request
	var err error
	if body == nil {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	}
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("telegram: another poller is running getUpdates for this token")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram %s: status %d", method, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
