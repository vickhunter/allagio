package services

import (
	"fmt"
	"math"
	"sort"
)

type RecommendationTuneProfile struct {
	ProgramID     string
	IsUserProgram bool
	Multiplier    float64
	SampleSize    int
}

func ApplyRecommendationTuning(matches []ProgramMatch, profiles []RecommendationTuneProfile) []ProgramMatch {
	if len(matches) == 0 || len(profiles) == 0 {
		return matches
	}
	weights := map[string]RecommendationTuneProfile{}
	for _, p := range profiles {
		if p.ProgramID == "" {
			continue
		}
		key := tuneKey(p.ProgramID, p.IsUserProgram)
		weights[key] = p
	}
	out := make([]ProgramMatch, 0, len(matches))
	for _, m := range matches {
		profile, ok := weights[tuneKey(m.ProgramID, m.IsUserProgram)]
		if !ok {
			out = append(out, m)
			continue
		}
		adjusted := m
		adjusted.Relevance = clampScore(float64(m.Relevance) * profile.Multiplier)
		delta := adjusted.Relevance - m.Relevance
		if delta != 0 {
			adjusted.Reason = fmt.Sprintf("%s Performance-adjusted (%+d from %d historical outcomes).", m.Reason, delta, profile.SampleSize)
		}
		out = append(out, adjusted)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Relevance > out[j].Relevance })
	return out
}

func clampScore(score float64) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return int(math.Round(score))
}

func tuneKey(id string, isUserProgram bool) string {
	if isUserProgram {
		return "u:" + id
	}
	return "c:" + id
}
