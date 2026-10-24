package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrORPReplay is a stored message id reused with different content.
var ErrORPReplay = errors.New("orp replay")

// ErrORPState is a conditional deal update that matched no row.
// The message transaction rolls back, so the accept is not stored.
var ErrORPState = errors.New("orp state")

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
	UpdatedAt     time.Time
	CreatedAt     time.Time
}

// ORPInboxItem is one stored host message for a party.
type ORPInboxItem struct {
	OwnerID string
	Message json.RawMessage
}

// ORPEffect is the state written with one message. Empty fields write nothing.
type ORPEffect struct {
	Offer             *ORPOffer
	Deal              *ORPDeal
	UpdateObligations json.RawMessage
	UpdateDealID      string
	// UpdateState "awaiting" moves that deal from proposed to awaiting.
	// Any other value, or a row that is no longer proposed, is an error.
	UpdateState string
	UpdatedAt   time.Time
	Inbox       []ORPInboxItem
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
    terms, terms_hash, state, waiting, obligations, bound, updated_at, created_at
) VALUES(
    $1, NULLIF($2,''), NULLIF($3,'')::uuid, $4::uuid, $5, $6,
    $7, $8, $9, $10, $11, $12, $13, $14
)`, deal.ID, deal.OfferID, deal.OffererOwner, deal.AcceptorOwner, deal.OffererParty, deal.AcceptorParty,
			deal.Terms, deal.TermsHash, deal.State, nullJSON(deal.Waiting), deal.Obligations, nullJSON(deal.Bound), deal.UpdatedAt, deal.CreatedAt); err != nil {
			return err
		}
	}
	if effect.UpdateDealID != "" && len(effect.UpdateObligations) > 0 {
		if _, err := tx.Exec(ctx, `
UPDATE orp_deals SET obligations=$2, updated_at=$3 WHERE id=$1`, effect.UpdateDealID, effect.UpdateObligations, effect.UpdatedAt); err != nil {
			return err
		}
	}
	if effect.UpdateState != "" {
		if effect.UpdateDealID == "" || effect.UpdateState != "awaiting" {
			return errors.New("orp state effect")
		}
		tag, err := tx.Exec(ctx, `
UPDATE orp_deals SET state=$2, updated_at=$3
WHERE id=$1 AND state='proposed'`, effect.UpdateDealID, effect.UpdateState, effect.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrORPState
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
WHERE state IN ('proposed','awaiting','active')
  AND (offerer_owner=$1::uuid OR acceptor_owner=$1::uuid)`, ownerID).Scan(&n)
	return n, err
}

// OpenORPDealForOffer reports an open deal this acceptor already has for the offer.
func (d *DB) OpenORPDealForOffer(ctx context.Context, offerID, acceptorOwner string) (string, bool, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
SELECT id FROM orp_deals
WHERE offer_id=$1 AND acceptor_owner=$2::uuid AND state IN ('proposed','awaiting','active')
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
func (d *DB) ActivateORPDeal(ctx context.Context, id string, bound json.RawMessage, updated time.Time, inbox []ORPInboxItem) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
UPDATE orp_deals
SET state='active', waiting=NULL, bound=$2, updated_at=$3
WHERE id=$1 AND state='awaiting'`, id, bound, updated)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
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

// GetORPDeal reads one deal.
func (d *DB) GetORPDeal(ctx context.Context, id string) (ORPDeal, bool, error) {
	var deal ORPDeal
	var offerID, offererOwner *string
	err := d.Pool.QueryRow(ctx, `
SELECT id, offer_id, offerer_owner::text, acceptor_owner::text, offerer_party, acceptor_party,
       terms, terms_hash, state, waiting, obligations, bound, updated_at, created_at
FROM orp_deals WHERE id=$1`, id).Scan(
		&deal.ID, &offerID, &offererOwner, &deal.AcceptorOwner, &deal.OffererParty, &deal.AcceptorParty,
		&deal.Terms, &deal.TermsHash, &deal.State, &deal.Waiting, &deal.Obligations, &deal.Bound, &deal.UpdatedAt, &deal.CreatedAt,
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
