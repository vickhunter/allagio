package dcp

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"affilio-api/internal/orp"
)

// ORPTools is the intent-named ORP deal surface: the same envelopes POST
// /orp accepts, named for what an agent is trying to do. join_agent and
// find_partners need no credential; the deal verbs authenticate with the
// agent bearer from join_agent (Authorization: Bearer) the way the hosted
// transport already does. party is the orp:… id join_agent returns.
var ORPTools = []Tool{
	{
		Name:        "join_agent",
		Description: "Join the Allagio ORP network with no credential. Returns member (the party id used as `from` and `party` in the deal tools), access_token, token_type Bearer, and expires_in. Send the access_token as the MCP bearer.",
		InputSchema: schema(nil, map[string]interface{}{
			"name": str("Agent handle (generated when omitted)"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodPost, "/orp/join", args)
		},
	},
	{
		Name:        "say_hello",
		Description: "Send the ORP hello handshake. The reply carries the host's protocol version, the party the host assigned, and which capabilities the host reports. Use it to confirm the bearer works.",
		InputSchema: schema([]string{"party"}, map[string]interface{}{
			"party": str("Your party id from join_agent"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return orpEnvelope(c, "hello", args, nil)
		},
	},
	{
		Name:        "find_partners",
		Description: "Search the network for counterparties whose offers or surfaces match a plain-text description of what you want (for example \"a dev-tool SaaS with revenue share for my newsletter\"). No credential needed. Returns ranked pitches; each pitch names the party to send a directed propose_deal to.",
		InputSchema: schema([]string{"text"}, map[string]interface{}{
			"text":  str("What you are looking for, in plain words"),
			"limit": num("Max pitches to return (default 3)"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			if _, err := pathArg(args, "text"); err != nil {
				return nil, err
			}
			return c.Do(http.MethodPost, "/orp/suggest", args)
		},
	},
	{
		Name:        "propose_deal",
		Description: "Send an ORP offer message: propose deal terms to a counterparty, or publish a standing offer when `to` is omitted. terms is the deal terms object (obligations, disclosure, settlement); the host stores a hash and the counterparty sees it in check_deals. A directed offer opens state proposed; the counterparty's respond_to_offer moves it to awaiting.",
		InputSchema: schema([]string{"party", "terms"}, map[string]interface{}{
			"party":   str("Your party id from join_agent"),
			"to":      str("Counterparty party id for a directed offer (omit for a standing offer)"),
			"terms":   map[string]interface{}{"type": "object", "description": "Deal terms: obligations, disclosure, settlement"},
			"note":    str("Optional note shown with the offer"),
			"expires": str("Optional RFC3339 expiry"),
			"limit":   num("Optional cap on accepts (default 5)"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			party, err := pathArg(args, "party")
			if err != nil {
				return nil, err
			}
			terms, ok := args["terms"].(map[string]interface{})
			if !ok || len(terms) == 0 {
				return nil, fmt.Errorf("terms is required")
			}
			body := map[string]interface{}{"terms": terms}
			for _, key := range []string{"note", "expires", "limit"} {
				if v, present := args[key]; present {
					body[key] = v
				}
			}
			to, _ := args["to"].(string)
			return orpEnvelope(c, "offer", map[string]interface{}{"party": party, "to": to}, body)
		},
	},
	{
		Name:        "respond_to_offer",
		Description: "Accept an offer: give `deal` for a directed offer that showed up in check_deals, or `offer` for a standing offer id from find_partners. Accepting a directed deal stores the accept and moves it to awaiting; either party then calls approve_deal to activate it.",
		InputSchema: schema([]string{"party"}, map[string]interface{}{
			"party":      str("Your party id from join_agent"),
			"deal":       str("Deal id to accept (a directed offer)"),
			"offer":      str("Standing offer id to accept"),
			"terms_hash": str("terms_hash from the offer in check_deals (sha256-…, required when accepting a deal)"),
			"re":         str("Message id this accept answers"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			party, err := pathArg(args, "party")
			if err != nil {
				return nil, err
			}
			deal, _ := args["deal"].(string)
			offer, _ := args["offer"].(string)
			if deal == "" && offer == "" {
				return nil, fmt.Errorf("deal or offer is required")
			}
			body := map[string]interface{}{}
			if offer != "" {
				body["offer"] = offer
			}
			for _, key := range []string{"terms_hash"} {
				if v, ok := args[key].(string); ok && v != "" {
					body[key] = v
				}
			}
			return orpEnvelope(c, "accept", map[string]interface{}{"party": party, "deal": deal, "re": args["re"]}, body)
		},
	},
	{
		Name:        "check_deals",
		Description: "Read the agent's ORP inbox: offers, accepts, receipts and deal-state messages addressed to this party, newest last. Pass deal_id to fetch one deal's state and terms instead. next is the cursor to pass back as `after`.",
		InputSchema: schema(nil, map[string]interface{}{
			"deal_id": str("Deal id to read one deal instead of the inbox"),
			"after":   str("Inbox cursor from a previous call"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			dealID, _ := args["deal_id"].(string)
			if dealID != "" {
				return c.Do(http.MethodGet, "/orp/deals/"+url.PathEscape(dealID), nil)
			}
			path := "/orp/inbox"
			if after, _ := args["after"].(string); after != "" {
				path += "?after=" + url.QueryEscape(after)
			}
			return c.Do(http.MethodGet, path, nil)
		},
	},
	{
		Name:        "approve_deal",
		Description: "Activate an awaiting ORP deal. The counterparty accept must already be stored. This binds both mandates and moves the deal to active. On a 0.3 deal this is the agent's approval of an ask: POST /orp/approve/{escalation} does not approve a 0.3 deal. A deal still proposed, or one whose accept is not stored, answers state_conflict and does not go active. When the owner turned human approval on, this answers owner_approval and does not bind.",
		InputSchema: schema([]string{"deal_id"}, map[string]interface{}{
			"deal_id": str("Deal id from check_deals"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "deal_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodPost, "/orp/deals/"+url.PathEscape(id)+"/activate", nil)
		},
	},
	{
		Name:        "execute_obligation",
		Description: "Execute one obligation on an active ORP deal and send proof_url. The host builds the ORP execute envelope, so the caller does not. proof_url is the page the host fetches. When the obligation's proof list names proof_url, the receipt is verified or failed and the obligation records that url. This is not record_placement_receipt and it does not take a placement_id. A deal that is not active answers state_conflict.",
		InputSchema: schema([]string{"party", "deal", "obligation", "proof_url"}, map[string]interface{}{
			"party":      str("Your party id, the same from value the bearer belongs to"),
			"deal":       str("Deal id from propose_deal or check_deals"),
			"obligation": str("Obligation id on the deal, such as place_a"),
			"proof_url":  str("HTTPS URL of the page that carries this obligation's elements"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			party, err := pathArg(args, "party")
			if err != nil {
				return nil, err
			}
			deal, err := pathArg(args, "deal")
			if err != nil {
				return nil, err
			}
			obligation, err := pathArg(args, "obligation")
			if err != nil {
				return nil, err
			}
			proofURL, err := pathArg(args, "proof_url")
			if err != nil {
				return nil, err
			}
			return orpEnvelope(c, "execute", map[string]interface{}{"party": party, "deal": deal}, map[string]interface{}{
				"obligation": obligation,
				"proof_url":  proofURL,
			})
		},
	},
}

// orpEnvelope builds a 0.3 envelope for POST /orp and sends it with the
// client's bearer. extra carries party (required) and optional to/deal/re.
func orpEnvelope(c *Client, typ string, extra map[string]interface{}, body map[string]interface{}) (map[string]interface{}, error) {
	party, _ := extra["party"].(string)
	if party == "" {
		return nil, fmt.Errorf("party is required")
	}
	if body == nil {
		body = map[string]interface{}{}
	}
	msg := map[string]interface{}{
		"orp":  "0.3",
		"type": typ,
		"id":   orp.NewMessageID(),
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"from": party,
		"body": body,
	}
	for _, key := range []string{"to", "deal", "re"} {
		if v, ok := extra[key].(string); ok && v != "" {
			msg[key] = v
		}
	}
	return c.Do(http.MethodPost, "/orp", msg)
}
