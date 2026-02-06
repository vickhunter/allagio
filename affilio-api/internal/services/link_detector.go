package services

import (
	"net/url"
	"regexp"
	"strings"
)

type LinkDetection struct {
	URL          string  `json:"url"`
	Domain       string  `json:"domain"`
	IsAffiliate  bool    `json:"is_affiliate"`
	Confidence   float64 `json:"confidence"`
	Method       string  `json:"method"`
	ProgramGuess string  `json:"program_guess"`
}

var (
	urlRegex              = regexp.MustCompile(`https?://[^\s\)\]\"'>]+`)
	affParams             = []string{"ref", "aff", "affiliate", "via", "partner", "utm_medium"}
	knownAffiliateDomains = map[string]bool{
		"amazon.com":       true,
		"impact.com":       true,
		"partnerstack.com": true,
		"shareasale.com":   true,
		"cj.com":           true,
	}
)

func ExtractLinks(content string) []string {
	matches := urlRegex.FindAllString(content, -1)
	return uniqueStrings(matches)
}

func DetectAffiliateLinks(content string) []LinkDetection {
	links := ExtractLinks(content)
	results := make([]LinkDetection, 0, len(links))
	for _, raw := range links {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		domain := strings.ToLower(u.Host)
		isAff, conf, method := ruleDetect(u)
		guess := guessProgram(u)
		results = append(results, LinkDetection{
			URL:          raw,
			Domain:       domain,
			IsAffiliate:  isAff,
			Confidence:   conf,
			Method:       method,
			ProgramGuess: guess,
		})
	}
	return results
}

func ruleDetect(u *url.URL) (bool, float64, string) {
	q := u.Query()
	for _, p := range affParams {
		if q.Get(p) != "" {
			return true, 0.85, "rules"
		}
	}
	if knownAffiliateDomains[stripSubdomain(u.Host)] {
		return true, 0.9, "rules"
	}
	return false, 0.0, "rules"
}

func guessProgram(u *url.URL) string {
	host := stripSubdomain(u.Host)
	if host == "" {
		return ""
	}
	parts := strings.Split(host, ".")
	return strings.Title(parts[0])
}

func stripSubdomain(host string) string {
	host = strings.ToLower(host)
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
