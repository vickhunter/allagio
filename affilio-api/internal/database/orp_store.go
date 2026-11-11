package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrORPReplay is a stored message id reused with different content.
var ErrORPReplay = errors.New("orp replay")

// ErrORPState is a conditional deal update that matched no row.
// The message transaction rolls back, so the accept is not stored.
var ErrORPState = errors.New("orp state")

// ORPRepeatWindow is how soon a later human click follows the previous
// counted click on the same tracking entry before it is stored as repeat.
// A click at exactly this gap still counts. The click instant is stored in
// the parties JSON with sub-second precision, so the gap is exactly this
// long. The comparison uses that stored timestamp only. No user agent,
// address, or client identifier is stored.
const ORPRepeatWindow = 5 * time.Second

// ORPRecord is the idempotency row for one party message.
type ORPRecord struct {
	Party  string
	ID     string
	Hash   string
	Status int
	Reply  []byte
}

// ORPOffer is a standing test offer. Terms are the public offer text.
// Listed is false while the offerer's list step is ask. Discover skips it.
type ORPOffer struct {
	ID        string
	OwnerID   string
	Party     string
	Terms     json.RawMessage
	TermsHash string
	Limit     int
	CreatedAt time.Time
	Listed    bool
}

// ORPDeal is a test-pool deal. It is not a placement and it is not a payout.
type ORPDeal struct {
	ID            string
	OfferID       string
	OffererOwner  string
	AcceptorOwner string
	OffererParty  string
	AcceptorParty string
	Terms         json.RawMessage
	TermsHash     string
	State         string
	Waiting       json.RawMessage
	Obligations   json.RawMessage
	Bound         json.RawMessage
	// Parties is the 0.4 many-party document. Nil on an ORP 0.3 row.
	Parties json.RawMessage
	// Tracking is the 0.3 tracked-link document ({tracking, clicks,
	// link_receipts}) minted on first activate. Nil until then; a 0.4 row
	// keeps the same data inside Parties instead.
	Tracking  json.RawMessage
	UpdatedAt time.Time
	CreatedAt time.Time
	// WireApprovals is who approved each ask step for this 0.4 response.
	// It is not a column. The rows already live in agent_step_approvals
	// or in the parties document.
	WireApprovals []DealApproval
	// ApprovalsUnavailable is set on an ORP 0.4 read when the approval
	// query fails. The response stays HTTP 200 and names that failure.
	// A 0.3 body never copies this flag.
	ApprovalsUnavailable bool
}

// ORPInboxItem is one stored host message for a party.
type ORPInboxItem struct {
	OwnerID string
	Message json.RawMessage
}

// ORPSpace is one registered space a party offers for deal work.
// Space holds the full object including locator; hosts strip locator
// before showing the space to another party.
type ORPSpace struct {
	ID      string
	OwnerID string
	Party   string
	Space   json.RawMessage
	Listed  bool
}

// ORPSpaceRef names one space for a withdraw effect.
type ORPSpaceRef struct {
	OwnerID string
	ID      string
}

// ORPEffect is the state written with one message. Empty fields write nothing.
type ORPEffect struct {
	Offer             *ORPOffer
	Deal              *ORPDeal
	UpdateObligations json.RawMessage
	UpdateDealID      string
	// UpdateState moves that deal to a new state when its current state is
	// one of UpdateFrom. An empty UpdateFrom, or a row in another state,
	// is an error.
	UpdateState string
	UpdateFrom  []string
	// UpdateWaiting replaces the deal's waiting marker in the same update;
	// ClearWaiting removes it. Neither set leaves the marker as stored.
	UpdateWaiting json.RawMessage
	ClearWaiting  bool
	// UpdateTerms/UpdateTermsHash replace the deal's terms in the same update;
	// a counter uses them so a stale accept fails terms_mismatch.
	UpdateTerms     json.RawMessage
	UpdateTermsHash string
	UpdatedAt       time.Time
	Inbox           []ORPInboxItem
	// Space upserts one space; SpaceWithdraw marks one withdrawn.
	Space         *ORPSpace
	SpaceWithdraw *ORPSpaceRef
	// ManyUpdate rewrites one 0.4 deal. ManyExpectHash and ManyExpectAt must
	// still match, or the transaction rolls back.
	ManyUpdate     *ORPDeal
	ManyExpectHash string
	ManyExpectAt   time.Time
	// OfferUpdate refreshes a standing offer's terms and window.
	OfferUpdate *ORPOffer
}

// ORPSaved is the reply that should be written. Reused means an earlier
// commit already stored this party id and the same content.
type ORPSaved struct {
	Status int
	Reply  []byte
	Reused bool
}

// LookupORPMessage reads a stored reply. found is false when the id is new.
// A different hash returns ErrORPReplay.
func (d *DB) LookupORPMessage(ctx context.Context, party, id, hash string) (ORPSaved, bool, error) {
	var storedHash string
	var saved ORPSaved
	err := d.Pool.QueryRow(ctx, `
SELECT body_hash, status, reply
FROM orp_messages
WHERE party=$1 AND message_id=$2`, party, id).Scan(&storedHash, &saved.Status, &saved.Reply)
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPSaved{}, false, nil
	}
	if err != nil {
		return ORPSaved{}, false, err
	}
	if storedHash != hash {
		return ORPSaved{}, true, ErrORPReplay
	}
	saved.Reused = true
	return saved, true, nil
}

// CommitORP stores the reply and its effect in one transaction.
// A concurrent writer that already stored the same content returns that reply.
func (d *DB) CommitORP(ctx context.Context, rec ORPRecord, effect ORPEffect) (ORPSaved, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return ORPSaved{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
INSERT INTO orp_messages(party, message_id, body_hash, status, reply)
VALUES($1,$2,$3,$4,$5)
ON CONFLICT (party, message_id) DO NOTHING`, rec.Party, rec.ID, rec.Hash, rec.Status, rec.Reply)
	if err != nil {
		return ORPSaved{}, err
	}
	if tag.RowsAffected() == 0 {
		var storedHash string
		var saved ORPSaved
		err = tx.QueryRow(ctx, `
SELECT body_hash, status, reply
FROM orp_messages
WHERE party=$1 AND message_id=$2`, rec.Party, rec.ID).Scan(&storedHash, &saved.Status, &saved.Reply)
		if err != nil {
			return ORPSaved{}, err
		}
		if storedHash != rec.Hash {
			return ORPSaved{}, ErrORPReplay
		}
		saved.Reused = true
		return saved, nil
	}
	if err = applyORPEffect(ctx, tx, effect); err != nil {
		return ORPSaved{}, err
	}
	if d.ORPBeforeCommit != nil {
		if err = d.ORPBeforeCommit(ctx); err != nil {
			return ORPSaved{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ORPSaved{}, err
	}
	return ORPSaved{Status: rec.Status, Reply: rec.Reply}, nil
}

func applyORPEffect(ctx context.Context, tx pgx.Tx, effect ORPEffect) error {
	if effect.Offer != nil {
		offer := effect.Offer
		if _, err := tx.Exec(ctx, `
INSERT INTO orp_offers(id, owner_id, party, terms, terms_hash, deal_limit, created_at, listed)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, offer.ID, offer.OwnerID, offer.Party, offer.Terms, offer.TermsHash, offer.Limit, offer.CreatedAt, offer.Listed); err != nil {
			return err
		}
	}
	if effect.Deal != nil {
		deal := effect.Deal
		if _, err := tx.Exec(ctx, `
INSERT INTO orp_deals(
    id, offer_id, offerer_owner, acceptor_owner, offerer_party, acceptor_party,
    terms, terms_hash, state, waiting, obligations, bound, updated_at, created_at, parties
) VALUES(
    $1, NULLIF($2,''), NULLIF($3,'')::uuid, $4::uuid, $5, $6,
    $7, $8, $9, $10, $11, $12, $13, $14, $15
)`, deal.ID, deal.OfferID, deal.OffererOwner, deal.AcceptorOwner, deal.OffererParty, deal.AcceptorParty,
			deal.Terms, deal.TermsHash, deal.State, nullJSON(deal.Waiting), JSONList(deal.Obligations), JSONList(deal.Bound), deal.UpdatedAt, deal.CreatedAt, nullJSON(deal.Parties)); err != nil {
			return err
		}
	}
	if effect.UpdateDealID != "" && len(effect.UpdateObligations) > 0 {
		if _, err := tx.Exec(ctx, `
UPDATE orp_deals SET obligations=$2, updated_at=$3 WHERE id=$1`, effect.UpdateDealID, JSONList(effect.UpdateObligations), effect.UpdatedAt); err != nil {
			return err
		}
	}
	if effect.ManyUpdate != nil {
		deal := effect.ManyUpdate
		if effect.ManyExpectHash == "" {
			return errors.New("orp many effect")
		}
		tag, err := tx.Exec(ctx, `
UPDATE orp_deals SET
    offerer_owner = NULLIF($2, '')::uuid,
    acceptor_owner = $3::uuid,
    offerer_party = $4,
    acceptor_party = $5,
    terms = $6,
    terms_hash = $7,
    state = $8,
    waiting = $9,
    obligations = $10,
    bound = $11,
    parties = $12,
    updated_at = $13
WHERE id = $1 AND terms_hash = $14 AND updated_at = $15`,
			deal.ID, deal.OffererOwner, deal.AcceptorOwner, deal.OffererParty, deal.AcceptorParty,
			deal.Terms, deal.TermsHash, deal.State, nullJSON(deal.Waiting), JSONList(deal.Obligations), JSONList(deal.Bound),
			nullJSON(deal.Parties), deal.UpdatedAt, effect.ManyExpectHash, effect.ManyExpectAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrORPState
		}
	}
	if effect.OfferUpdate != nil {
		offer := effect.OfferUpdate
		tag, err := tx.Exec(ctx, `
UPDATE orp_offers SET terms=$2, terms_hash=$3, listed=$4, created_at=$5
WHERE id=$1 AND owner_id=$6::uuid`, offer.ID, offer.Terms, offer.TermsHash, offer.Listed, offer.CreatedAt, offer.OwnerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrORPState
		}
	}
	if effect.UpdateState != "" {
		if effect.UpdateDealID == "" || len(effect.UpdateFrom) == 0 {
			return errors.New("orp state effect")
		}
		tag, err := tx.Exec(ctx, `
UPDATE orp_deals SET state=$2, updated_at=$3,
  waiting = CASE WHEN $5 THEN NULL ELSE COALESCE($6, waiting) END,
  terms = COALESCE($7, terms), terms_hash = COALESCE($8, terms_hash)
WHERE id=$1 AND state=ANY($4)`, effect.UpdateDealID, effect.UpdateState, effect.UpdatedAt, effect.UpdateFrom, effect.ClearWaiting, nullJSON(effect.UpdateWaiting), nullJSON(effect.UpdateTerms), nilOrString(effect.UpdateTermsHash))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrORPState
		}
	}
	if effect.Space != nil {
		sp := effect.Space
		if _, err := tx.Exec(ctx, `
INSERT INTO orp_spaces(id, owner_id, party, space, listed, withdrawn_at, updated_at)
VALUES($1,$2::uuid,$3,$4,$5,NULL,$6)
ON CONFLICT (owner_id, id) DO UPDATE
SET space=$4, listed=$5, withdrawn_at=NULL, updated_at=$6`, sp.ID, sp.OwnerID, sp.Party, sp.Space, sp.Listed, effect.UpdatedAt); err != nil {
			return err
		}
	}
	if effect.SpaceWithdraw != nil {
		if _, err := tx.Exec(ctx, `
UPDATE orp_spaces SET withdrawn_at=$3, listed=false, updated_at=$3
WHERE owner_id=$1::uuid AND id=$2 AND withdrawn_at IS NULL`, effect.SpaceWithdraw.OwnerID, effect.SpaceWithdraw.ID, effect.UpdatedAt); err != nil {
			return err
		}
	}
	for _, item := range effect.Inbox {
		if item.OwnerID == "" || len(item.Message) == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO orp_inbox(owner_id, message) VALUES($1,$2)`, item.OwnerID, item.Message); err != nil {
			return err
		}
	}
	return nil
}

func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func nilOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// JSONList is the stored form of a NOT NULL JSONB list.
// json.Marshal of a nil slice is null, and nullJSON turns that into SQL NULL.
// bound and obligations are NOT NULL, so an empty list is stored as [].
// terms, reply, and inbox message are NOT NULL objects and are written as given.
// waiting and parties are nullable and stay on nullJSON, which keeps SQL NULL.
func JSONList(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`[]`)
	}
	return raw
}

// GetORPOffer reads one standing offer.
func (d *DB) GetORPOffer(ctx context.Context, id string) (ORPOffer, bool, error) {
	var offer ORPOffer
	err := d.Pool.QueryRow(ctx, `
SELECT id, owner_id::text, party, terms, terms_hash, deal_limit, created_at, listed
FROM orp_offers WHERE id=$1 AND listed`, id).Scan(&offer.ID, &offer.OwnerID, &offer.Party, &offer.Terms, &offer.TermsHash, &offer.Limit, &offer.CreatedAt, &offer.Listed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPOffer{}, false, nil
	}
	if err != nil {
		return ORPOffer{}, false, err
	}
	return offer, true, nil
}

// GetORPOfferOwned reads one offer for its maker, including an unlisted row.
func (d *DB) GetORPOfferOwned(ctx context.Context, id, ownerID string) (ORPOffer, bool, error) {
	var offer ORPOffer
	err := d.Pool.QueryRow(ctx, `
SELECT id, owner_id::text, party, terms, terms_hash, deal_limit, created_at, listed
FROM orp_offers WHERE id=$1 AND owner_id=$2::uuid`, id, ownerID).Scan(
		&offer.ID, &offer.OwnerID, &offer.Party, &offer.Terms, &offer.TermsHash, &offer.Limit, &offer.CreatedAt, &offer.Listed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPOffer{}, false, nil
	}
	if err != nil {
		return ORPOffer{}, false, err
	}
	return offer, true, nil
}

// RecordORPClick attributes one live tracked redirect and returns where it goes.
// found is false for an unknown token. The row stores the class and the split
// of that click. A later click records its own split and does not add those
// percents onto an earlier click. The first human click that meets a place
// obligation whose proof includes link marks that obligation verified and
// writes one receipt. A bot, preview, test, or repeat click does not.
// Verification runs only when the deal is active and the place obligation is
// pending or claimed. record is false for HEAD. A deal that is not active
// redirects and writes nothing. A human click less than ORPRepeatWindow after
// the previous counted click on the same entry is stored as repeat and is not
// counted. It does not store an address, a user agent, or a referrer. No new
// column is required: the obligation list and the inbox already hold the receipt.
func (d *DB) RecordORPClick(ctx context.Context, token, class, clickID string, at time.Time, record bool) (destination, param, alias string, found bool, err error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", "", "", false, err
	}
	defer tx.Rollback(ctx)
	var id, state string
	var parties, trackingRaw, obligationsRaw, termsRaw []byte
	var offererOwner, offererParty, acceptorOwner, acceptorParty *string
	err = tx.QueryRow(ctx, `
SELECT id, state, parties, tracking, obligations, terms,
       offerer_owner::text, offerer_party, acceptor_owner::text, acceptor_party
FROM orp_deals
WHERE parties->'tracking' @> jsonb_build_array(jsonb_build_object('token', $1::text))
   OR tracking->'tracking' @> jsonb_build_array(jsonb_build_object('token', $1::text))
FOR UPDATE`, token).Scan(&id, &state, &parties, &trackingRaw, &obligationsRaw, &termsRaw,
		&offererOwner, &offererParty, &acceptorOwner, &acceptorParty)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", false, err
	}
	// A 0.4 deal keeps tracking inside parties; a 0.3 deal in the tracking
	// column. Both hold the same document shape, so one pipeline serves both.
	var raw []byte
	column := ""
	for _, candidate := range []struct {
		col string
		doc []byte
	}{{"parties", parties}, {"tracking", trackingRaw}} {
		if len(candidate.doc) == 0 {
			continue
		}
		var probe struct {
			Tracking []struct {
				Token string `json:"token"`
			} `json:"tracking"`
		}
		if json.Unmarshal(candidate.doc, &probe) != nil {
			continue
		}
		for _, e := range probe.Tracking {
			if e.Token == token {
				raw, column = candidate.doc, candidate.col
			}
		}
		if column != "" {
			break
		}
	}
	if column == "" {
		return "", "", "", false, nil
	}
	var doc struct {
		Seats []struct {
			ID    string         `json:"id"`
			Owner string         `json:"owner"`
			Terms map[string]any `json:"terms"`
		} `json:"seats"`
		Tracking []struct {
			Obligation  string `json:"obligation"`
			Token       string `json:"token"`
			Destination string `json:"destination"`
			Status      string `json:"status"`
			Param       string `json:"param"`
			Alias       string `json:"alias"`
		} `json:"tracking"`
		Clicks []json.RawMessage `json:"clicks"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return "", "", "", false, err
	}
	var entry *struct {
		Obligation  string `json:"obligation"`
		Token       string `json:"token"`
		Destination string `json:"destination"`
		Status      string `json:"status"`
		Param       string `json:"param"`
		Alias       string `json:"alias"`
	}
	for i := range doc.Tracking {
		if doc.Tracking[i].Token == token {
			entry = &doc.Tracking[i]
			break
		}
	}
	if entry == nil {
		return "", "", "", false, nil
	}
	destination, param, alias = entry.Destination, entry.Param, entry.Alias
	if param == "" {
		param = "via"
	}
	if entry.Status != "live" {
		return destination, param, alias, true, nil
	}
	// HEAD is class preview. A deal that is not active, including one that
	// returned to pending after a required party left, redirects and writes nothing.
	if !record || state != "active" {
		return destination, param, alias, true, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	at = at.UTC()
	class = repeatClass(doc.Clicks, entry.Obligation, class, at)
	shares := make([]any, 0)
	for _, seat := range doc.Seats {
		if len(seat.Terms) == 0 {
			continue
		}
		share := map[string]any{"party": seat.ID}
		kept := false
		if raw, ok := seat.Terms["percent"]; ok {
			if n, ok := jsonWhole(raw); ok {
				share["percent"] = n
				kept = true
			}
		}
		if amount, ok := seat.Terms["amount"].(map[string]any); ok {
			if n, ok := jsonWhole(amount["amount_minor"]); ok {
				share["amount_minor"] = n
				kept = true
			}
			if currency, _ := amount["currency"].(string); currency != "" {
				share["currency"] = currency
			}
		}
		if kept {
			shares = append(shares, share)
		}
	}
	click, err := json.Marshal(map[string]any{
		"id": clickID, "obligation": entry.Obligation, "class": class, "shares": shares,
		"at": at.Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", "", "", false, err
	}
	doc.Clicks = append(doc.Clicks, click)
	var stored map[string]any
	if err = json.Unmarshal(raw, &stored); err != nil {
		return "", "", "", false, err
	}
	clicks := make([]any, 0, len(doc.Clicks))
	for _, item := range doc.Clicks {
		var one any
		if err = json.Unmarshal(item, &one); err != nil {
			return "", "", "", false, err
		}
		clicks = append(clicks, one)
	}
	stored["clicks"] = clicks
	receiptID, nextObligations, receiptBody := linkReceipt(termsRaw, obligationsRaw, clicks, entry.Obligation, class, state, at)
	if receiptID != "" {
		receipts, _ := stored["link_receipts"].([]any)
		stored["link_receipts"] = append(receipts, map[string]any{
			"obligation":  entry.Obligation,
			"id":          receiptID,
			"method":      "link",
			"verified_at": at.Format(time.RFC3339),
		})
	}
	raw, err = json.Marshal(stored)
	if err != nil {
		return "", "", "", false, err
	}
	if receiptID == "" {
		if _, err = tx.Exec(ctx, `UPDATE orp_deals SET `+column+`=$2 WHERE id=$1`, id, raw); err != nil {
			return "", "", "", false, err
		}
	} else {
		if at.IsZero() {
			at = time.Now().UTC()
		}
		if _, err = tx.Exec(ctx, `UPDATE orp_deals SET `+column+`=$2, obligations=$3, updated_at=$4 WHERE id=$1`, id, raw, JSONList(nextObligations), at.UTC()); err != nil {
			return "", "", "", false, err
		}
		host := ""
		if len(doc.Seats) > 0 {
			host = hostPartyFromSeat(doc.Seats[0].ID)
		}
		for _, seat := range doc.Seats {
			if seat.Owner == "" || host == "" {
				continue
			}
			message, err := json.Marshal(map[string]any{
				"orp": "0.4", "type": "receipt", "id": receiptID, "ts": at.UTC().Format(time.RFC3339),
				"from": host, "to": seat.ID, "deal": id, "body": receiptBody,
			})
			if err != nil {
				return "", "", "", false, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO orp_inbox(owner_id, message) VALUES($1,$2)`, seat.Owner, message); err != nil {
				return "", "", "", false, err
			}
		}
		// A 0.3 deal has no seats; the receipt reaches both owners directly.
		if len(doc.Seats) == 0 {
			host = hostPartyFromSeat(oneStr(offererParty))
			for _, dest := range []struct {
				owner, party *string
			}{{offererOwner, offererParty}, {acceptorOwner, acceptorParty}} {
				if dest.owner == nil || dest.party == nil || host == "" {
					continue
				}
				message, err := json.Marshal(map[string]any{
					"orp": "0.3", "type": "receipt", "id": receiptID, "ts": at.UTC().Format(time.RFC3339),
					"from": host, "to": *dest.party, "deal": id, "body": receiptBody,
				})
				if err != nil {
					return "", "", "", false, err
				}
				if _, err = tx.Exec(ctx, `INSERT INTO orp_inbox(owner_id, message) VALUES($1::uuid,$2)`, *dest.owner, message); err != nil {
					return "", "", "", false, err
				}
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", "", false, err
	}
	return destination, param, alias, true, nil
}

// repeatClass stores a human click as repeat when it falls inside
// ORPRepeatWindow of the previous counted click on the same obligation.
// Only class human anchors the window. A clock that moves backward counts.
func repeatClass(clicks []json.RawMessage, obligation, class string, at time.Time) string {
	if class != "human" {
		return class
	}
	var latest time.Time
	found := false
	for _, raw := range clicks {
		var row struct {
			Obligation string `json:"obligation"`
			Class      string `json:"class"`
			At         string `json:"at"`
		}
		if json.Unmarshal(raw, &row) != nil || row.Class != "human" || row.Obligation != obligation || row.At == "" {
			continue
		}
		prev, err := time.Parse(time.RFC3339Nano, row.At)
		if err != nil {
			continue
		}
		if !found || prev.After(latest) {
			latest = prev
			found = true
		}
	}
	if !found {
		return class
	}
	delta := at.Sub(latest)
	if delta >= 0 && delta < ORPRepeatWindow {
		return "repeat"
	}
	return class
}

// linkReceipt verifies a place obligation once the human clicks reach its
// target. The default target is the first human click. The deal must be
// active, and the stored obligation must be pending or claimed. failed,
// partial, verified, and any other state stay as they are, with no receipt.
func linkReceipt(termsRaw, obligationsRaw []byte, clicks []any, obligation, class, dealState string, at time.Time) (receiptID string, obligations json.RawMessage, body map[string]any) {
	if class != "human" || dealState != "active" {
		return "", nil, nil
	}
	var terms map[string]any
	if json.Unmarshal(termsRaw, &terms) != nil {
		return "", nil, nil
	}
	spec := obligationSpec(terms, obligation)
	if spec["action"] != "place" || !proofIncludes(spec, "link") {
		return "", nil, nil
	}
	humans := 0
	for _, item := range clicks {
		row, _ := item.(map[string]any)
		if row["class"] == "human" && row["obligation"] == obligation {
			humans++
		}
	}
	target := 1
	if n, ok := jsonWhole(spec["clicks"]); ok && n > 0 {
		target = n
	}
	if humans < target {
		return "", nil, nil
	}
	var rows []map[string]any
	if json.Unmarshal(obligationsRaw, &rows) != nil {
		return "", nil, nil
	}
	found := false
	for i := range rows {
		if rows[i]["id"] != obligation {
			continue
		}
		stored, _ := rows[i]["state"].(string)
		if stored != "pending" && stored != "claimed" {
			return "", nil, nil
		}
		found = true
		rows[i]["state"] = "verified"
		rows[i]["receipt"] = newReceiptID()
		receiptID, _ = rows[i]["receipt"].(string)
		break
	}
	if !found || receiptID == "" {
		return "", nil, nil
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return "", nil, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	body = map[string]any{
		"obligation": obligation,
		"result":     "verified",
		"method":     "link",
		"at":         at.UTC().Format(time.RFC3339),
		"found":      map[string]any{"link": true, "clicks": humans},
	}
	return receiptID, raw, body
}

func obligationSpec(terms map[string]any, id string) map[string]any {
	raw, _ := terms["obligations"].([]any)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if obj["id"] == id {
			return obj
		}
	}
	return nil
}

func proofIncludes(spec map[string]any, method string) bool {
	raw, _ := spec["proof"].([]any)
	for _, item := range raw {
		if item == method {
			return true
		}
	}
	return false
}

func newReceiptID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "m_host_fallbackrc"
	}
	return "m_host_" + hex.EncodeToString(buf[:])
}

func hostPartyFromSeat(seatID string) string {
	rest := strings.TrimPrefix(seatID, "orp:")
	i := strings.LastIndex(rest, "/")
	if i <= 0 {
		return ""
	}
	return "orp:" + rest[:i] + "/host"
}

func oneStr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func jsonWhole(value any) (int, bool) {
	switch n := value.(type) {
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	case int:
		return n, true
	case json.Number:
		v, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

// ListORPOffers returns the newest standing offers.
func (d *DB) ListORPOffers(ctx context.Context, limit int) ([]ORPOffer, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.Pool.Query(ctx, `
SELECT id, owner_id::text, party, terms, terms_hash, deal_limit, created_at, listed
FROM orp_offers
WHERE listed
ORDER BY created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ORPOffer{}
	for rows.Next() {
		var offer ORPOffer
		if err = rows.Scan(&offer.ID, &offer.OwnerID, &offer.Party, &offer.Terms, &offer.TermsHash, &offer.Limit, &offer.CreatedAt, &offer.Listed); err != nil {
			return nil, err
		}
		out = append(out, offer)
	}
	return out, rows.Err()
}

// CountOpenORPDeals counts deals that still occupy the party's open-deal limit.
func (d *DB) CountOpenORPDeals(ctx context.Context, ownerID string) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM orp_deals
WHERE state IN ('proposed','awaiting','active','pending')
  AND (
    (parties IS NULL AND (offerer_owner=$1::uuid OR acceptor_owner=$1::uuid))
    OR (parties IS NOT NULL AND parties->'seats' @> jsonb_build_array(jsonb_build_object('owner', $1::text)))
  )`, ownerID).Scan(&n)
	return n, err
}

// OpenORPDealForOffer reports an open deal this acceptor already has for the offer.
func (d *DB) OpenORPDealForOffer(ctx context.Context, offerID, acceptorOwner string) (string, bool, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
SELECT id FROM orp_deals
WHERE offer_id=$1 AND acceptor_owner=$2::uuid AND state IN ('proposed','awaiting','active','pending')
LIMIT 1`, offerID, acceptorOwner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// ORPAcceptStored reports a stored HTTP 200 deal reply from this party for the deal.
// That reply is the counterparty accept. Activate refuses the deal until it exists.
func (d *DB) ORPAcceptStored(ctx context.Context, party, dealID string) (bool, error) {
	var ok bool
	err := d.Pool.QueryRow(ctx, `
SELECT EXISTS(
    SELECT 1 FROM orp_messages
    WHERE party=$1 AND status=200 AND reply->>'type'='deal' AND reply->>'deal'=$2
)`, party, dealID).Scan(&ok)
	return ok, err
}

// ActivateORPDeal moves one awaiting deal to active and stores the snapshots.
// changed is false when the row is no longer awaiting.
// approval is written in this same transaction, and only after the update
// matches one row. A later inbox error rolls the approval back with the deal.
// ActivateORPDeal resolves one awaiting deal to state and stores the snapshots.
// obligations, when set, replaces the obligation rows in the same transaction —
// a held share claim or obligation revoke lands on approval.
// changed is false when the row is no longer awaiting.
func (d *DB) ActivateORPDeal(ctx context.Context, id, state string, bound, obligations json.RawMessage, updated time.Time, inbox []ORPInboxItem, approval *DealApproval, tracking json.RawMessage) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
UPDATE orp_deals
SET state=$2, waiting=NULL, bound=$3, updated_at=$4, obligations=COALESCE($5, obligations), tracking=COALESCE($6, tracking)
WHERE id=$1 AND state='awaiting'`, id, state, JSONList(bound), updated, nullJSON(obligations), nullJSON(tracking))
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() != 1 {
		return false, nil
	}
	if err = insertDealApproval(ctx, tx, id, approval); err != nil {
		return false, err
	}
	for _, item := range inbox {
		if item.OwnerID == "" || len(item.Message) == 0 {
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO orp_inbox(owner_id, message) VALUES($1,$2)`, item.OwnerID, item.Message); err != nil {
			return false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func insertDealApproval(ctx context.Context, tx pgx.Tx, dealID string, row *DealApproval) error {
	if row == nil || row.By == "" || row.OwnerID == "" {
		return nil
	}
	payload, err := json.Marshal(map[string]any{
		"kind": "deal_approval", "deal_id": dealID, "party": row.Party, "step": row.Step, "approved_by": row.By,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO agent_step_approvals(step, owner_id, status, payload, result, decided_at)
VALUES('placement_publish', $1::uuid, 'approved', $2::jsonb, '{"kind":"deal_approval"}'::jsonb, NOW())`, row.OwnerID, payload)
	return err
}

// GetORPDeal reads one deal.
func (d *DB) GetORPDeal(ctx context.Context, id string) (ORPDeal, bool, error) {
	var deal ORPDeal
	var offerID, offererOwner *string
	err := d.Pool.QueryRow(ctx, `
SELECT id, offer_id, offerer_owner::text, acceptor_owner::text, offerer_party, acceptor_party,
       terms, terms_hash, state, waiting, obligations, bound, parties, tracking, updated_at, created_at
FROM orp_deals WHERE id=$1`, id).Scan(
		&deal.ID, &offerID, &offererOwner, &deal.AcceptorOwner, &deal.OffererParty, &deal.AcceptorParty,
		&deal.Terms, &deal.TermsHash, &deal.State, &deal.Waiting, &deal.Obligations, &deal.Bound, &deal.Parties, &deal.Tracking, &deal.UpdatedAt, &deal.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPDeal{}, false, nil
	}
	if err != nil {
		return ORPDeal{}, false, err
	}
	if offerID != nil {
		deal.OfferID = *offerID
	}
	if offererOwner != nil {
		deal.OffererOwner = *offererOwner
	}
	return deal, true, nil
}

// GetORPDealByEscalation reads the deal waiting on this escalation id.
func (d *DB) GetORPDealByEscalation(ctx context.Context, escalation string) (ORPDeal, bool, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
SELECT id FROM orp_deals WHERE waiting->>'escalation'=$1`, escalation).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ORPDeal{}, false, nil
	}
	if err != nil {
		return ORPDeal{}, false, err
	}
	return d.GetORPDeal(ctx, id)
}

// OwnerExists reports whether an agent account id is on this host.
func (d *DB) OwnerExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM owners WHERE id=$1::uuid)`, id).Scan(&ok)
	return ok, err
}

// ListOwnerORPDeals returns deals where the owner holds a seat, newest first.
func (d *DB) ListOwnerORPDeals(ctx context.Context, ownerID string, limit int) ([]ORPDeal, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.Pool.Query(ctx, `
SELECT id, offer_id, offerer_owner::text, acceptor_owner::text, offerer_party, acceptor_party,
       terms, terms_hash, state, waiting, obligations, bound, parties, tracking, updated_at, created_at
FROM orp_deals
WHERE
    (parties IS NULL AND (offerer_owner=$1::uuid OR acceptor_owner=$1::uuid))
    OR (parties IS NOT NULL AND parties->'seats' @> jsonb_build_array(jsonb_build_object('owner', $1::text)))
ORDER BY updated_at DESC
LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ORPDeal{}
	for rows.Next() {
		var deal ORPDeal
		var offerID, offererOwner *string
		if err = rows.Scan(
			&deal.ID, &offerID, &offererOwner, &deal.AcceptorOwner, &deal.OffererParty, &deal.AcceptorParty,
			&deal.Terms, &deal.TermsHash, &deal.State, &deal.Waiting, &deal.Obligations, &deal.Bound, &deal.Parties, &deal.Tracking, &deal.UpdatedAt, &deal.CreatedAt,
		); err != nil {
			return nil, err
		}
		if offerID != nil {
			deal.OfferID = *offerID
		}
		if offererOwner != nil {
			deal.OffererOwner = *offererOwner
		}
		out = append(out, deal)
	}
	return out, rows.Err()
}

// ListORPDealsForParty lists deals where the party id sits, oldest first.
func (d *DB) ListORPDealsForParty(ctx context.Context, party string, limit int) ([]ORPDeal, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx, `
SELECT id, offer_id, offerer_owner::text, acceptor_owner::text, offerer_party, acceptor_party,
       terms, terms_hash, state, waiting, obligations, bound, parties, tracking, updated_at, created_at
FROM orp_deals
WHERE
    (parties IS NULL AND (offerer_party=$1 OR acceptor_party=$1))
    OR (parties IS NOT NULL AND parties->'seats' @> jsonb_build_array(jsonb_build_object('party', $1::text)))
ORDER BY created_at ASC
LIMIT $2`, party, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ORPDeal{}
	for rows.Next() {
		var deal ORPDeal
		var offerID, offererOwner *string
		if err = rows.Scan(
			&deal.ID, &offerID, &offererOwner, &deal.AcceptorOwner, &deal.OffererParty, &deal.AcceptorParty,
			&deal.Terms, &deal.TermsHash, &deal.State, &deal.Waiting, &deal.Obligations, &deal.Bound, &deal.Parties, &deal.Tracking, &deal.UpdatedAt, &deal.CreatedAt,
		); err != nil {
			return nil, err
		}
		if offerID != nil {
			deal.OfferID = *offerID
		}
		if offererOwner != nil {
			deal.OffererOwner = *offererOwner
		}
		out = append(out, deal)
	}
	return out, rows.Err()
}

// ListORPDealEvents returns this owner's inbox messages for one deal, oldest first.
func (d *DB) ListORPDealEvents(ctx context.Context, ownerID, dealID string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := d.Pool.Query(ctx, `
SELECT message
FROM orp_inbox
WHERE owner_id=$1 AND message->>'deal'=$2
ORDER BY id
LIMIT $3`, ownerID, dealID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var message json.RawMessage
		if err = rows.Scan(&message); err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, rows.Err()
}

// ListORPInbox returns messages newer than after, oldest first.
func (d *DB) ListORPInbox(ctx context.Context, ownerID string, after int64, limit int) ([]json.RawMessage, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := d.Pool.Query(ctx, `
SELECT id, message
FROM orp_inbox
WHERE owner_id=$1 AND id>$2
ORDER BY id
LIMIT $3`, ownerID, after, limit)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	next := after
	for rows.Next() {
		var id int64
		var message json.RawMessage
		if err = rows.Scan(&id, &message); err != nil {
			return nil, after, err
		}
		out = append(out, message)
		next = id
	}
	return out, next, rows.Err()
}

// ListORPSpaces returns listed, not-withdrawn spaces, newest first.
func (d *DB) ListORPSpaces(ctx context.Context, limit int) ([]ORPSpace, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, owner_id::text, party, space, listed FROM orp_spaces
WHERE listed AND withdrawn_at IS NULL ORDER BY updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ORPSpace
	for rows.Next() {
		var sp ORPSpace
		if err = rows.Scan(&sp.ID, &sp.OwnerID, &sp.Party, &sp.Space, &sp.Listed); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// GetORPSpace reads one space for its owner (any listing state).
func (d *DB) GetORPSpace(ctx context.Context, ownerID, id string) (ORPSpace, error) {
	var sp ORPSpace
	var withdrawn *time.Time
	err := d.Pool.QueryRow(ctx, `
SELECT id, owner_id::text, party, space, listed, withdrawn_at FROM orp_spaces
WHERE owner_id=$1::uuid AND id=$2`, ownerID, id).Scan(&sp.ID, &sp.OwnerID, &sp.Party, &sp.Space, &sp.Listed, &withdrawn)
	if withdrawn != nil {
		sp.Listed = false
	}
	return sp, err
}

// ListORPSpacesByOwner returns one owner's not-withdrawn spaces for the dashboard.
func (d *DB) ListORPSpacesByOwner(ctx context.Context, ownerID string) ([]ORPSpace, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, owner_id::text, party, space, listed FROM orp_spaces
WHERE owner_id=$1::uuid AND withdrawn_at IS NULL ORDER BY updated_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ORPSpace
	for rows.Next() {
		var sp ORPSpace
		if err = rows.Scan(&sp.ID, &sp.OwnerID, &sp.Party, &sp.Space, &sp.Listed); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// ListORPSpaceOwner marks one space listed after the owner's ask-step approval.
func (d *DB) ListORPSpaceOwner(ctx context.Context, ownerID, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `
UPDATE orp_spaces SET listed=true, updated_at=NOW()
WHERE owner_id=$1::uuid AND id=$2 AND withdrawn_at IS NULL`, ownerID, id)
	return tag.RowsAffected() == 1, err
}

// OwnerSiteDomain is the claimed site's bare domain for one owner.
func (d *DB) OwnerSiteDomain(ctx context.Context, ownerID string) (string, error) {
	var domain string
	err := d.Pool.QueryRow(ctx, `SELECT s.domain FROM sites s JOIN owners o ON o.site_id=s.id WHERE o.id=$1::uuid`, ownerID).Scan(&domain)
	return domain, err
}
