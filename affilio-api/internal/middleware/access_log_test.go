package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestApprovalAccessLogRedactsEscalationID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orp/approve/esc_secretvalue?ref=1", nil)
	req.RemoteAddr = "203.0.113.77:1000"
	line := AccessLogFormatter(gin.LogFormatterParams{
		TimeStamp:    time.Date(2026, 10, 10, 15, 4, 5, 0, time.UTC),
		StatusCode:   http.StatusOK,
		Method:       http.MethodGet,
		Request:      req,
		Latency:      time.Millisecond,
		ErrorMessage: "lookup failed from 203.0.113.77",
		ClientIP:     "203.0.113.77",
	})
	for _, secret := range []string{"esc_secretvalue", "203.0.113.77", "ref=1"} {
		if strings.Contains(line, secret) {
			t.Fatalf("access log kept %s: %s", secret, line)
		}
	}
	if !strings.Contains(line, "/orp/approve/[redacted]") || !strings.Contains(line, "200") || !strings.Contains(line, "GET") {
		t.Fatalf("access log shape: %s", line)
	}

	post := httptest.NewRequest(http.MethodPost, "/orp/approve/esc_other", nil)
	postLine := AccessLogFormatter(gin.LogFormatterParams{
		TimeStamp:  time.Date(2026, 10, 10, 15, 4, 5, 0, time.UTC),
		StatusCode: http.StatusForbidden,
		Method:     http.MethodPost,
		Request:    post,
		Latency:    time.Millisecond,
	})
	if strings.Contains(postLine, "esc_other") || !strings.Contains(postLine, "/orp/approve/[redacted]") {
		t.Fatalf("post access log: %s", postLine)
	}

	if got, ok := approvalLogPath("/orp/approve/"); ok || got != "/orp/approve/" {
		t.Fatalf("empty id %q %v", got, ok)
	}
	if got, ok := approvalLogPath("/orp/deals/deal_1"); ok || got != "/orp/deals/deal_1" {
		t.Fatalf("other path %q %v", got, ok)
	}
}
