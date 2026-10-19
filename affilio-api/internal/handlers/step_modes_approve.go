package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *OfferReviewHandler) ApproveStep(c *gin.Context) {
	row, err := h.DB.GetStepApproval(c.Request.Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval unavailable"})
		return
	}
	if row.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "approval is not waiting", "status": row.Status})
		return
	}
	result, err := executeStepApproval(c, h.DB, row)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval failed"})
		return
	}
	raw, _ := json.Marshal(result)
	ok, err := h.DB.FinishStepApproval(c.Request.Context(), row.ID, raw)
	if err != nil || !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "approval failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "approved", "approval_id": row.ID, "step": row.Step, "result": result})
}

func executeStepApproval(c *gin.Context, db *database.DB, row database.StepApproval) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return nil, err
	}
	switch row.Step {
	case database.StepSiteSignup:
		return approveSignup(c, db, payload)
	case database.StepOfferMatch:
		return approveMatch(c, db, row.SiteID, payload)
	case database.StepLinkChange:
		return approveLink(c, db, row.SiteID, payload)
	case database.StepPayout:
		return approvePayout(c, db, row.SiteID, payload)
	default:
		return nil, errors.New("unsupported step")
	}
}

func approveSignup(c *gin.Context, db *database.DB, payload map[string]any) (map[string]any, error) {
	kind, _ := payload["kind"].(string)
	if kind == "agent_account" {
		return approveAgentAccount(c, db, payload)
	}
	domain, _ := payload["domain"].(string)
	name, _ := payload["name"].(string)
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	siteID, err := db.CreateSite(c.Request.Context(), services.HashAPIKey(apiKey), services.KeyPrefix(apiKey), domain, name)
	if err != nil {
		return nil, err
	}
	token, err := db.EnsureSiteVerificationToken(c.Request.Context(), siteID)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"site_id": siteID, "api_key": apiKey, "verification_token": token, "verified": false}
	pluginKey, issued, err := mintPluginKey(c.Request.Context(), db, siteID)
	if err != nil {
		return nil, err
	}
	if issued {
		body["plugin_key"] = pluginKey
	}
	return body, nil
}

func approveAgentAccount(c *gin.Context, db *database.DB, payload map[string]any) (map[string]any, error) {
	role, _ := payload["role"].(string)
	name, _ := payload["name"].(string)
	email, _ := payload["email"].(string)
	domain, _ := payload["domain"].(string)
	source, _ := payload["source"].(string)
	subject, err := randomToken()
	if err != nil {
		return nil, err
	}
	siteKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	makerKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	pluginKey, pluginHash, pluginPrefix := "", "", ""
	if role == "publisher" {
		pluginKey, err = generateAPIKey()
		if err != nil {
			return nil, err
		}
		pluginHash = services.HashAPIKey(pluginKey)
		pluginPrefix = services.KeyPrefix(pluginKey)
	}
	account, err := db.CreateAgentOnlyAccount(c.Request.Context(), "agent:"+subject, email, name, database.NormalizeAttributionSource(source), role, domain, services.HashAPIKey(siteKey), services.KeyPrefix(siteKey), makerKey, pluginHash, pluginPrefix)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"owner_id": account.OwnerID, "role": role}
	if role == "publisher" {
		out["site_id"] = account.SiteID
		out["api_key"] = siteKey
		out["plugin_key"] = pluginKey
		out["verification_token"] = account.VerificationToken
	} else {
		out["maker_id"] = account.MakerID
		out["api_key"] = makerKey
	}
	return out, nil
}

func approveMatch(c *gin.Context, db *database.DB, siteID string, payload map[string]any) (map[string]any, error) {
	text, _ := payload["context"].(string)
	c.Set("site_id", siteID)
	found, err := (&PlacementsHandler{DB: db}).opportunities(c, text)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "matched", "opportunities": found.items, "reason": found.reason}, nil
}

func approveLink(c *gin.Context, db *database.DB, siteID string, payload map[string]any) (map[string]any, error) {
	offerID, _ := payload["offer_id"].(string)
	code, err := generateTrackingCode()
	if err != nil {
		return nil, err
	}
	campaignID, code, err := db.JoinOffer(c.Request.Context(), offerID, siteID, code)
	if err != nil {
		return nil, err
	}
	return map[string]any{"campaign_id": campaignID, "offer_id": offerID, "tracking_code": code, "tracked_link": "/v1/go/" + code}, nil
}

func approvePayout(c *gin.Context, db *database.DB, siteID string, payload map[string]any) (map[string]any, error) {
	action, _ := payload["action"].(string)
	switch action {
	case "confirm_received":
		payoutID, _ := payload["payout_id"].(string)
		ok, err := db.ConfirmPayoutReceived(c.Request.Context(), siteID, payoutID)
		if err != nil || !ok {
			return nil, errors.New("payout not awaiting receipt confirmation")
		}
		return map[string]any{"status": "received", "payout_id": payoutID}, nil
	case "mark_paid":
		payoutID, _ := payload["payout_id"].(string)
		ref, _ := payload["provider_reference"].(string)
		ok, err := db.MarkPayoutPaid(c.Request.Context(), payoutID, ref)
		if err != nil || !ok {
			return nil, errors.New("payout not pending")
		}
		return map[string]any{"status": "paid", "payout_id": payoutID}, nil
	case "save_profile":
		instructions, _ := payload["instructions"].(string)
		if err := db.SavePayoutInstructions(c.Request.Context(), siteID, instructions); err != nil {
			return nil, err
		}
		return map[string]any{"saved": true}, nil
	case "report_payment":
		payoutID, _ := payload["payout_id"].(string)
		makerID, _ := payload["maker_id"].(string)
		ref, _ := payload["reference"].(string)
		ok, err := db.ReportMerchantPayment(c.Request.Context(), makerID, payoutID, ref)
		if err != nil || !ok {
			return nil, errors.New("payout not pending")
		}
		return map[string]any{"status": "payment_reported", "payout_id": payoutID}, nil
	case "batch":
		currency, _ := payload["currency"].(string)
		makerID, _ := payload["maker_id"].(string)
		paymentMode, _ := payload["payment_mode"].(string)
		if siteID == "" {
			siteID, _ = payload["site_id"].(string)
		}
		payoutID, created, err := db.FulfillBatchPayout(c.Request.Context(), siteID, currency, makerID, paymentMode)
		if err != nil {
			return nil, err
		}
		return map[string]any{"payout_id": payoutID, "created": created}, nil
	default:
		return nil, errors.New("unsupported payout action")
	}
}
