package services

import (
	"strings"
)

type ScoreResult struct {
	Score     int
	Potential string
}

func ComputeMonetizationScore(wordCount int, mentioned int, existing int, topOpportunity int, opportunities int) ScoreResult {
	coverage := 0
	if mentioned > 0 {
		coverage = int(float64(existing) / float64(mentioned) * 100)
	}
	quality := 20
	switch {
	case wordCount > 1500:
		quality = 80
	case wordCount > 800:
		quality = 60
	case wordCount > 400:
		quality = 40
	}
	score := int(float64(coverage)*0.5 + float64(topOpportunity)*0.3 + float64(quality)*0.2)
	if score > 100 {
		score = 100
	}
	potential := "low"
	if opportunities >= 2 && topOpportunity >= 70 {
		potential = "high"
	} else if opportunities >= 1 {
		potential = "medium"
	}
	return ScoreResult{Score: score, Potential: potential}
}

func WordCount(content string) int {
	fields := strings.Fields(content)
	return len(fields)
}
