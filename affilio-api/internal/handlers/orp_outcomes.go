package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

// OutcomeStats is one sweep's tally for the worker's log line.
type OutcomeStats struct {
	Deals  int
	Sent   int
	Finals int
}

// SweepORPOutcomes runs one outcome pass (SPEC 14.1). Every active deal
// with tracking or a pay obligation whose period is at least a week old,
// and every ended deal still missing its final, gets an outcome message per
// party in that party owner's inbox. The period marker and the inbox copies
// commit in one transaction, so a repeated or crashed sweep sends nothing
// twice.
func SweepORPOutcomes(ctx context.Context, db *database.DB, publicBaseURL string, now time.Time) (OutcomeStats, error) {
	var stats OutcomeStats
	_, dns := orp.Origin(publicBaseURL, "", "")
	due, err := db.ListORPOutcomeDue(ctx, now, 200)
	if err != nil {
		return stats, err
	}
	for _, item := range due {
		stats.Deals++
		sent, err := sendOutcome(ctx, db, dns, item, now)
		if err != nil {
			log.Printf("orp outcome deal %s: %v", item.Deal.ID, err)
			continue
		}
		if sent {
			stats.Sent++
			if item.Final {
				stats.Finals++
			}
		}
	}
	return stats, nil
}

// sendOutcome builds one party's rows for the due period and commits them.
// A 0.3 deal reads its tracking column; a 0.4 deal reads the same document
// inside parties. A deal whose built message fails the envelope check is
// skipped rather than sent malformed.
func sendOutcome(ctx context.Context, db *database.DB, dns string, item database.ORPOutcomeDue, now time.Time) (bool, error) {
	deal := item.Deal
	raw := deal.Tracking
	version := orp.Version
	if len(deal.Parties) > 0 && string(deal.Parties) != "null" {
		raw = deal.Parties
		version = orp.Version04
	}
	doc, err := orp.DecodeMany(raw)
	if err != nil {
		return false, err
	}
	earned, err := db.ORPOutcomeEarned(ctx, deal.ID, item.PeriodFrom, item.PeriodTo)
	if err != nil {
		return false, err
	}
	rows := orp.PlanOutcomeRows(doc, item.PeriodFrom, item.PeriodTo, earned)
	body := orp.OutcomeBody(item.PeriodFrom, item.PeriodTo, rows, item.Final)
	targets := outcomeParties(deal, doc, version)
	inbox := make([]database.ORPInboxItem, 0, len(targets))
	seenOwner := map[string]bool{}
	for _, target := range targets {
		if target.owner == "" || seenOwner[target.owner] {
			continue
		}
		seenOwner[target.owner] = true
		msg := orp.OutcomeMessage(dns, target.party, deal.ID, version,
			orp.OutcomeID(deal.ID, item.PeriodTo, target.party), body, now)
		raw, err := json.Marshal(msg)
		if err != nil {
			return false, err
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return false, err
		}
		if _, problem, ok := orp.ParseEnvelope(value); !ok {
			log.Printf("orp outcome deal %s party %s skipped: %s", deal.ID, target.party, problem.Message)
			return false, nil
		}
		inbox = append(inbox, database.ORPInboxItem{OwnerID: target.owner, Message: raw})
	}
	if len(inbox) == 0 {
		return false, nil
	}
	return db.RecordORPOutcome(ctx, deal.ID, item.PeriodFrom, item.PeriodTo, item.Final, inbox)
}

type outcomeParty struct {
	party string
	owner string
}

// outcomeParties names each party's owner on the deal: offerer and acceptor
// on a 0.3 row, seats with owners on a 0.4 row.
func outcomeParties(deal database.ORPDeal, doc orp.Many, version string) []outcomeParty {
	if version == orp.Version04 {
		out := make([]outcomeParty, 0, len(doc.Seats))
		for _, seat := range doc.Seats {
			if seat.ID != "" && seat.Owner != "" {
				out = append(out, outcomeParty{party: seat.ID, owner: seat.Owner})
			}
		}
		return out
	}
	return []outcomeParty{
		{party: deal.OffererParty, owner: deal.OffererOwner},
		{party: deal.AcceptorParty, owner: deal.AcceptorOwner},
	}
}
