package orp

import (
	"testing"
	"time"
)

func at(base time.Time, d time.Duration) string {
	return base.Add(d).Format(time.RFC3339Nano)
}

func TestPlanOutcomeRowsCountsClassesInsidePeriod(t *testing.T) {
	from := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	to := from.Add(7 * 24 * time.Hour)
	doc := Many{
		Tracking: []Track{
			{Obligation: "place", Token: "tk_one"},
			{Obligation: "share", Token: "tk_two"},
		},
		Clicks: []Click{
			{Obligation: "place", Class: "human", At: at(from, time.Hour)},
			{Obligation: "place", Class: "human", At: at(to, -time.Second)},
			{Obligation: "place", Class: "bot", At: at(from, time.Hour)},
			{Obligation: "place", Class: "preview", At: at(from, time.Hour)},
			{Obligation: "place", Class: "test", At: at(from, time.Hour)},
			{Obligation: "place", Class: "repeat", At: at(from, 2*time.Hour)},
			{Obligation: "place", Class: "human", At: at(to, 0)},
			{Obligation: "place", Class: "human", At: at(from, -time.Second)},
			{Obligation: "place", Class: "human", At: "not-a-time"},
			{Obligation: "ghost", Class: "human", At: at(from, time.Hour)},
		},
	}
	rows := PlanOutcomeRows(doc, from, to, nil)
	if len(rows) != 2 {
		t.Fatalf("rows %v", rows)
	}
	first, _ := rows[0].(map[string]any)
	clicks, _ := first["clicks"].(map[string]any)
	if first["obligation"] != "place" || clicks["human"] != 2 || clicks["bot"] != 1 || clicks["preview"] != 1 || clicks["test"] != 1 {
		t.Fatalf("place row %v", first)
	}
	if _, ok := clicks["repeat"]; ok {
		t.Fatalf("repeat counted %v", clicks)
	}
	second, _ := rows[1].(map[string]any)
	shareClicks, _ := second["clicks"].(map[string]any)
	if second["obligation"] != "share" || shareClicks["human"] != 0 || len(shareClicks) != 1 {
		t.Fatalf("share row %v", second)
	}
}

func TestPlanOutcomeRowsEarned(t *testing.T) {
	from := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	to := from.Add(7 * 24 * time.Hour)
	doc := Many{
		Tracking: []Track{
			{Obligation: "place", Token: "tk_one"},
		},
	}
	rows := PlanOutcomeRows(doc, from, to, []OutcomeEarned{
		{Obligation: "pay", AmountMinor: 500, Currency: "EUR"},
		{Obligation: "place", AmountMinor: 200, Currency: "EUR"},
		{Obligation: "", AmountMinor: 900, Currency: "EUR"},
		{Obligation: "zero", AmountMinor: 0, Currency: "EUR"},
	})
	if len(rows) != 2 {
		t.Fatalf("rows %v", rows)
	}
	place, _ := rows[0].(map[string]any)
	earnedPlace, _ := place["earned"].(map[string]any)
	if place["obligation"] != "place" || earnedPlace["amount_minor"] != int64(200) || earnedPlace["currency"] != "EUR" {
		t.Fatalf("merged earned %v", place)
	}
	pay, _ := rows[1].(map[string]any)
	clicks, _ := pay["clicks"].(map[string]any)
	earned, _ := pay["earned"].(map[string]any)
	if pay["obligation"] != "pay" || clicks["human"] != 0 || earned["amount_minor"] != int64(500) || earned["currency"] != "EUR" {
		t.Fatalf("pay row %v", pay)
	}
}

func TestOutcomeIDDeterministic(t *testing.T) {
	period := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	a := OutcomeID("deal_one", period, "orp:hub.example/p_a")
	if a != OutcomeID("deal_one", period, "orp:hub.example/p_a") {
		t.Fatal("id not deterministic")
	}
	if a == OutcomeID("deal_one", period, "orp:hub.example/p_b") ||
		a == OutcomeID("deal_two", period, "orp:hub.example/p_a") ||
		a == OutcomeID("deal_one", period.Add(time.Hour), "orp:hub.example/p_a") {
		t.Fatal("id ignores deal, period, or party")
	}
	if !messagePattern.MatchString(a) {
		t.Fatalf("id %q fails the message id pattern", a)
	}
}
