package orp

import (
	"math"
	"time"
)

// RepEvent is one reputation input: a verified receipt (positive weight from
// the §13 method table) or a failure (weight −1) between a subject and one
// counterparty at a point in time.
type RepEvent struct {
	Counterparty string
	Weight       float64
	At           time.Time
}

// RepComponents is the counts block of a reputation attestation.
type RepComponents struct {
	Verified       float64
	Failed         float64
	DisputesLost   int
	Counterparties int
	EvidenceWeight float64
}

const repHalfLifeDays = 180.0
const repMinWeight = 3.0
const wilsonZ = 1.96

// MethodWeight is the §13 reputation weight per receipt method.
var MethodWeight = map[string]float64{
	"provider": 1, "platform_check": 1, "link": 0.9,
	"witness": 0.8, "proof_url": 0.7, "attestation": 0,
}

func repWeighted(events []RepEvent, now time.Time, standing func(string) float64) (pos, neg float64) {
	seen := map[string]int{}
	for _, ev := range events {
		seen[ev.Counterparty]++
		w := ev.Weight / float64(seen[ev.Counterparty])
		if !ev.At.IsZero() {
			w *= math.Pow(0.5, now.Sub(ev.At).Hours()/(24*repHalfLifeDays))
		}
		if standing != nil {
			w *= standing(ev.Counterparty)
		}
		if w >= 0 {
			pos += w
		} else {
			neg -= w
		}
	}
	return pos, neg
}

func wilsonLower(pos, total float64) float64 {
	if total <= 0 {
		return 0
	}
	p := pos / total
	z2 := wilsonZ * wilsonZ
	return (p + z2/(2*total) - wilsonZ*math.Sqrt(p*(1-p)/total+z2/(4*total*total))) / (1 + z2/total)
}

// baseScore is a party's Wilson score with every counterparty standing taken
// as unknown (factor 0.25), the depth-1 approximation SPEC §15.3 step 4 needs.
func baseScore(events []RepEvent, now time.Time) float64 {
	pos, neg := repWeighted(events, now, func(string) float64 { return 0.25 })
	if pos+neg < repMinWeight {
		return 0
	}
	return math.Round(1000 * wilsonLower(pos, pos+neg))
}

// Reputation computes an orp-rep-1 score and components for a subject's
// events. score is nil until total evidence weight reaches 3. Counterparty
// standing uses each counterparty's own depth-1 score: their events weighted
// with every one of their counterparties unknown.
func Reputation(subject string, events []RepEvent, counterpartyEvents func(party string) []RepEvent, now time.Time) (score *int, components RepComponents) {
	cpSet := map[string]bool{}
	for _, ev := range events {
		cpSet[ev.Counterparty] = true
	}
	standing := func(cp string) float64 {
		s := 0.0
		if counterpartyEvents != nil {
			s = baseScore(counterpartyEvents(cp), now)
		}
		return 0.25 + 0.75*math.Min(1, s/1000)
	}
	pos, neg := repWeighted(events, now, standing)
	comp := RepComponents{
		Verified:       math.Round(pos*100) / 100,
		Failed:         math.Round(neg*100) / 100,
		Counterparties: len(cpSet),
		EvidenceWeight: math.Round((pos+neg)*100) / 100,
	}
	if pos+neg < repMinWeight {
		return nil, comp
	}
	s := int(math.Round(1000 * wilsonLower(pos, pos+neg)))
	return &s, comp
}
