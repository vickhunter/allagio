package services

import (
	"sort"
	"strings"
	"unicode"
)

type ProgramMatch struct {
	ProgramID     string
	Name          string
	Domain        string
	Relevance     int
	Reason        string
	Insertion     string
	Position      string
	HasLink       bool
	IsUserProgram bool
}

type UserProgramInput struct {
	ID     string
	Name   string
	Domain string
}

type CuratedProgramInput struct {
	ID       string
	Name     string
	Company  string
	Slug     string
	Keywords []string
	Category string
}

func MatchUserPrograms(content string, programs []UserProgramInput, detections []LinkDetection) []ProgramMatch {
	lower := strings.ToLower(content)
	tokens := tokenize(content)
	hasDomain := map[string]bool{}
	for _, d := range detections {
		hasDomain[strings.ToLower(d.Domain)] = true
	}
	var out []ProgramMatch
	for _, p := range programs {
		if p.Domain == "" && p.Name == "" {
			continue
		}
		mentioned := containsAny(lower, []string{p.Domain, p.Name})
		if !mentioned {
			if score, _ := semanticOverlapScore(tokens, []string{p.Domain, p.Name}); score >= 55 {
				mentioned = true
			}
		}
		if !mentioned {
			continue
		}
		hasLink := false
		if p.Domain != "" && hasDomain[strings.ToLower(p.Domain)] {
			hasLink = true
		}
		if hasLink {
			continue
		}
		out = append(out, ProgramMatch{
			ProgramID:     p.ID,
			Name:          p.Name,
			Domain:        p.Domain,
			Relevance:     85,
			Reason:        "Mentioned product with no affiliate link detected.",
			Insertion:     "inline_mention",
			Position:      "end_of_article",
			HasLink:       false,
			IsUserProgram: true,
		})
	}
	return out
}

func MatchCuratedPrograms(content string, programs []CuratedProgramInput, detections []LinkDetection) []ProgramMatch {
	lower := strings.ToLower(content)
	tokens := tokenize(content)
	hasDomain := map[string]bool{}
	for _, d := range detections {
		hasDomain[strings.ToLower(d.Domain)] = true
	}
	var out []ProgramMatch
	for _, p := range programs {
		if p.Name == "" && p.Company == "" && p.Slug == "" {
			continue
		}
		terms := []string{p.Name, p.Company, p.Slug}
		terms = append(terms, p.Keywords...)
		score, reason := scoreMatch(lower, tokens, terms, p.Category)
		if score < 50 {
			continue
		}
		if p.Slug != "" && hasDomain[strings.ToLower(p.Slug)+".com"] {
			continue
		}
		out = append(out, ProgramMatch{
			ProgramID: p.ID,
			Name:      p.Name,
			Domain:    p.Slug,
			Relevance: score,
			Reason:    reason,
			Insertion: "inline_mention",
			Position:  "end_of_article",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Relevance > out[j].Relevance })
	return out
}

func DedupeMatches(matches []ProgramMatch) []ProgramMatch {
	if len(matches) == 0 {
		return matches
	}
	userKeys := map[string]bool{}
	for _, m := range matches {
		if !m.IsUserProgram {
			continue
		}
		key := normalizeMatchKey(m)
		if key != "" {
			userKeys[key] = true
		}
	}
	best := map[string]ProgramMatch{}
	for _, m := range matches {
		key := normalizeMatchKey(m)
		if key == "" {
			key = m.ProgramID
		}
		if key == "" {
			continue
		}
		if !m.IsUserProgram && userKeys[key] {
			continue
		}
		existing, ok := best[key]
		if !ok {
			best[key] = m
			continue
		}
		if compareMatch(m, existing) {
			best[key] = m
		}
	}
	out := make([]ProgramMatch, 0, len(best))
	for _, m := range best {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Relevance > out[j].Relevance })
	return out
}

func compareMatch(a, b ProgramMatch) bool {
	if a.IsUserProgram != b.IsUserProgram {
		return a.IsUserProgram
	}
	if a.Relevance != b.Relevance {
		return a.Relevance > b.Relevance
	}
	return a.ProgramID < b.ProgramID
}

func normalizeMatchKey(m ProgramMatch) string {
	if m.IsUserProgram {
		if key := normalizeKey(m.Domain); key != "" {
			return key
		}
		return normalizeKey(m.Name)
	}
	if key := normalizeKey(m.Domain); key != "" {
		return key
	}
	return normalizeKey(m.Name)
}

func normalizeKey(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimPrefix(input, "www.")
	input = strings.Split(input, "/")[0]
	input = strings.TrimSuffix(input, ".com")
	return input
}

func scoreMatch(content string, tokens []string, terms []string, category string) (int, string) {
	max := 0
	best := ""
	semanticScore, semanticTerm := semanticOverlapScore(tokens, terms)
	if semanticScore > 0 {
		max = semanticScore
		best = semanticTerm
	}
	for _, t := range terms {
		term := strings.ToLower(strings.TrimSpace(t))
		if term == "" {
			continue
		}
		if strings.Contains(content, term) {
			score := 60
			if len(term) > 8 {
				score = 80
			}
			if score > max {
				max = score
				best = term
			}
		}
	}
	if max == 0 {
		return 0, ""
	}
	reason := "The article discusses \u201c" + best + "\u201d, making it a strong contextual fit for this recommendation."
	if category != "" && categoryMatch(tokens, category) {
		reason = "The article discusses \u201c" + best + "\u201d within the " + category + " space, making it a strong contextual fit."
		max += 5
	}
	if max > 100 {
		max = 100
	}
	return max, reason
}

func semanticOverlapScore(tokens []string, terms []string) (int, string) {
	if len(tokens) == 0 || len(terms) == 0 {
		return 0, ""
	}
	tokenSet := map[string]struct{}{}
	for _, tok := range tokens {
		tokenSet[tok] = struct{}{}
	}
	bestScore := 0
	bestTerm := ""
	for _, t := range terms {
		termTokens := tokenize(t)
		if len(termTokens) == 0 {
			continue
		}
		matches := 0
		for _, tt := range termTokens {
			if _, ok := tokenSet[tt]; ok {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		score := 45 + matches*10
		if len(termTokens) >= 3 {
			score += 5
		}
		if score > bestScore {
			bestScore = score
			bestTerm = strings.Join(termTokens, " ")
		}
	}
	if bestScore > 95 {
		bestScore = 95
	}
	return bestScore, bestTerm
}

func tokenize(input string) []string {
	input = strings.ToLower(input)
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = normalizeToken(p)
		if p == "" || isStopword(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func normalizeToken(token string) string {
	if len(token) > 4 {
		if strings.HasSuffix(token, "ing") {
			return strings.TrimSuffix(token, "ing")
		}
		if strings.HasSuffix(token, "ed") {
			return strings.TrimSuffix(token, "ed")
		}
	}
	if len(token) > 3 && strings.HasSuffix(token, "s") {
		return strings.TrimSuffix(token, "s")
	}
	return token
}

func isStopword(token string) bool {
	switch token {
	case "the", "and", "for", "with", "from", "this", "that", "are", "was", "were", "has", "have", "your", "you", "our", "their", "its", "about", "into", "but", "not", "can", "use", "using", "used", "how":
		return true
	default:
		return false
	}
}

func categoryMatch(tokens []string, category string) bool {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return false
	}
	for _, tok := range tokens {
		if tok == category {
			return true
		}
	}
	return false
}

func containsAny(content string, terms []string) bool {
	for _, t := range terms {
		term := strings.ToLower(strings.TrimSpace(t))
		if term == "" {
			continue
		}
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}
