package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ReadinessEngine answers "is this product actually distributable?" before any
// publisher wastes traffic on it. Deterministic probes always run; the AI rubric
// runs only when Claude is configured and degrades to neutral defaults otherwise,
// matching the AIClassifier convention.
type ReadinessEngine struct {
	Client     *http.Client
	APIKey     string
	Model      string
	APIURL     string
	APIVersion string
	// AllowPrivateHosts permits probing loopback/private addresses (tests only).
	AllowPrivateHosts bool
	// PublicBaseURL is the only extra origin readiness may probe when it is not a public host.
	// Behavior is covered by TestReadinessAllowsOnlyTheConfiguredPublicOrigin.
	PublicBaseURL string
}

type ReadinessProduct struct {
	Name            string
	Tagline         string
	WebsiteURL      string
	DocsURL         string
	PricingURL      string
	PaymentProvider string
	RefParam        string
	CheckoutURL     string
}

type ReadinessCheck struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	Gate        bool   `json:"gate"`
	Status      string `json:"status"`
	Score       int    `json:"score"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation,omitempty"`
}

type ReadinessResult struct {
	Score     int
	Status    string
	Checks    []ReadinessCheck
	AISummary string
	Category  string
	Keywords  []string
	Tags      []string
}

var checkoutMarkers = map[string][]string{
	"stripe":       {"js.stripe.com", "checkout.stripe.com", "buy.stripe.com"},
	"lemonsqueezy": {"lemonsqueezy.com", "lemon squeezy"},
	"polar":        {"polar.sh", "buy.polar.sh"},
}

var (
	pricingMarkerPattern    = regexp.MustCompile(`(?i)[$€£]\s*\d|/\s*(month|mo|year|yr)|price|pricing|free trial`)
	onboardingMarkerPattern = regexp.MustCompile(`(?i)sign\s*up|get started|start (for )?free|create (an )?account|register|try (it )?free`)
	privacyMarkerPattern    = regexp.MustCompile(`(?i)href="[^"]*privacy[^"]*"`)
	pricingLinkPattern      = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']*(?:pricing|price)[^"']*)["']`)
	docsLinkPattern         = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']*(?:docs|documentation)[^"']*)["']`)
	termsMarkerPattern      = regexp.MustCompile(`(?i)href="[^"]*terms[^"]*"`)
	checkoutActionText      = regexp.MustCompile(`(?i)\b(buy|subscribe|checkout|start|sign[\s-]*up|get\s+started)\b`)
	checkoutHrefMarker      = regexp.MustCompile(`(?i)(?:^|[/?#=_-])(buy|subscribe|checkout|signup|sign-up|get-started|start)(?:[/?#=_./-]|$)`)
	actionTagPattern        = regexp.MustCompile(`(?is)<(a|button)\b([^>]*)>(.*?)</(a|button)>`)
	hrefAttrPattern         = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']*)["']`)
	htmlTagPattern          = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]*>`)
)

func (e *ReadinessEngine) Assess(ctx context.Context, product ReadinessProduct) ReadinessResult {
	refParam := strings.TrimSpace(product.RefParam)
	if refParam == "" {
		refParam = "via"
	}

	site := e.fetchPage(ctx, product.WebsiteURL)
	checkout, providerWarning := e.checkCheckoutPresent(ctx, product, site)
	pricingURL := strings.TrimSpace(product.PricingURL)
	if pricingURL == "" {
		pricingURL = linkedPageURL(site, product.WebsiteURL, pricingLinkPattern)
	}
	docsURL := strings.TrimSpace(product.DocsURL)
	if docsURL == "" {
		docsURL = linkedPageURL(site, product.WebsiteURL, docsLinkPattern)
	}
	checks := []ReadinessCheck{
		e.checkSiteReachable(site),
		checkout,
		e.checkRefParamSurvives(ctx, product.WebsiteURL, product.PricingURL, refParam),
		e.checkPricingPage(ctx, pricingURL),
		e.checkDocsPresent(ctx, docsURL),
		checkBodyMarker(site, "privacy_policy", privacyMarkerPattern, true,
			"Privacy Policy link found on the landing page.",
			"Add a discoverable Privacy Policy link to the landing page."),
		checkBodyMarker(site, "terms", termsMarkerPattern, true,
			"Terms link found on the landing page.",
			"Add a discoverable Terms link to the landing page."),
		checkBodyMarker(site, "onboarding_flow", onboardingMarkerPattern, false,
			"Signup call-to-action found on the landing page.",
			"Add a clear signup / get-started call-to-action to the landing page."),
	}
	if providerWarning != nil {
		checks = append(checks, *providerWarning)
	}

	result := ReadinessResult{}
	if rubric := e.assessWithClaude(ctx, product, site.Body); rubric != nil {
		checks = append(checks, rubric.Checks...)
		result.AISummary = rubric.Summary
		result.Category = rubric.Category
		result.Keywords = rubric.Keywords
		result.Tags = rubric.Tags
	}

	result.Checks = checks
	result.Score = scoreChecks(checks)
	result.Status = "ready"
	for _, check := range checks {
		if check.Gate && check.Status == "fail" {
			result.Status = "failing"
			break
		}
	}
	return result
}

type fetchedPage struct {
	OK       bool
	Status   int
	FinalURL *url.URL
	Body     string
	Err      string
}

func (e *ReadinessEngine) httpClient() *http.Client {
	client := &http.Client{Timeout: 10 * time.Second}
	if e.Client != nil {
		copy := *e.Client
		client = &copy
	}
	if e.AllowPrivateHosts {
		return client
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if existing, ok := client.Transport.(*http.Transport); ok {
		transport = existing.Clone()
	}
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if e.publicOriginHost(host) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, address)
		}
		return dialPublicProbe(ctx, network, address)
	}
	client.Transport = transport
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return e.hostAllowed(req.Context(), req.URL.String())
	}
	return client
}

func dialPublicProbe(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := publicProbeAddresses(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	for _, address := range addresses {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	return nil, err
}

func publicProbeAddresses(ctx context.Context, host string) ([]net.IPAddr, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("host did not resolve")
	}
	for _, address := range addresses {
		ip := address.IP
		if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, errors.New("host resolves to a private address")
		}
	}
	return addresses, nil
}

func (e *ReadinessEngine) fetchPage(ctx context.Context, rawURL string) fetchedPage {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fetchedPage{Err: "no url"}
	}
	if err := e.hostAllowed(ctx, rawURL); err != nil {
		return fetchedPage{Err: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fetchedPage{Err: "invalid url"}
	}
	req.Header.Set("User-Agent", "Distributoor-Readiness/1.0")
	client := e.httpClient()
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return fetchedPage{Err: "request failed: " + err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return fetchedPage{
		OK:       resp.StatusCode >= 200 && resp.StatusCode < 300,
		Status:   resp.StatusCode,
		FinalURL: resp.Request.URL,
		Body:     string(body),
	}
}

// hostAllowed blocks probes of loopback/private/link-local addresses so a maker
// cannot point readiness checks at internal infrastructure.
func (e *ReadinessEngine) hostAllowed(ctx context.Context, rawURL string) error {
	if e.AllowPrivateHosts || e.originAllowed(rawURL) {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("invalid url")
	}
	_, err = publicProbeAddresses(ctx, parsed.Hostname())
	return err
}

func (e *ReadinessEngine) checkSiteReachable(site fetchedPage) ReadinessCheck {
	check := ReadinessCheck{Key: "site_reachable", Kind: "probe", Gate: true}
	if site.OK {
		check.Status = "pass"
		check.Score = 100
		check.Evidence = fmt.Sprintf("landing page returned HTTP %d", site.Status)
		return check
	}
	check.Status = "fail"
	check.Evidence = site.Err
	if site.Status > 0 {
		check.Evidence = fmt.Sprintf("landing page returned HTTP %d", site.Status)
	}
	check.Remediation = "Deploy the product and make the landing page publicly reachable over HTTPS with a 2xx response."
	return check
}

// originAllowed is the only SSRF exception: probes of PUBLIC_BASE_URL, so this
// server can assess its own demo product. Every other private host stays blocked.
func (e *ReadinessEngine) originAllowed(rawURL string) bool {
	base := strings.TrimSpace(e.PublicBaseURL)
	if base == "" {
		return false
	}
	allowed, err := url.Parse(base)
	if err != nil || allowed.Hostname() == "" || (allowed.Scheme != "http" && allowed.Scheme != "https") {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, allowed.Scheme) && e.publicOriginHost(parsed.Hostname()) && parsed.Port() == allowed.Port()
}

func (e *ReadinessEngine) publicOriginHost(host string) bool {
	allowed, err := url.Parse(strings.TrimSpace(e.PublicBaseURL))
	if err != nil || allowed.Hostname() == "" {
		return false
	}
	return strings.EqualFold(host, allowed.Hostname())
}

func pageHasCheckoutAction(html string) bool {
	for _, match := range actionTagPattern.FindAllStringSubmatch(html, -1) {
		href := ""
		if found := hrefAttrPattern.FindStringSubmatch(match[2]); found != nil {
			href = found[1]
		}
		if checkoutActionText.MatchString(stripHTML(match[3])) || checkoutHrefMarker.MatchString(href) {
			return true
		}
	}
	return false
}

func detectCheckoutProvider(haystack string) (string, string) {
	for provider, markers := range checkoutMarkers {
		for _, marker := range markers {
			if strings.Contains(haystack, marker) {
				return provider, marker
			}
		}
	}
	return "", ""
}

func (e *ReadinessEngine) checkCheckoutPresent(ctx context.Context, product ReadinessProduct, site fetchedPage) (ReadinessCheck, *ReadinessCheck) {
	check := ReadinessCheck{Key: "checkout_present", Kind: "probe", Gate: true}
	bodies := site.Body
	if product.PricingURL != "" && product.PricingURL != product.WebsiteURL {
		pricing := e.fetchPage(ctx, product.PricingURL)
		bodies += "\n" + pricing.Body
	}
	haystack := strings.ToLower(bodies + "\n" + product.CheckoutURL)
	provider, marker := detectCheckoutProvider(haystack)
	if strings.TrimSpace(product.CheckoutURL) == "" && !pageHasCheckoutAction(bodies) {
		check.Status = "fail"
		check.Evidence = "no checkout, buy, or sign-up link found on the landing or pricing page"
		check.Remediation = "Add a buy, subscribe, checkout, or sign-up link on the landing or pricing page, or set a checkout URL. Offer Rails does not require a particular payment provider."
		return check, nil
	}
	check.Status = "pass"
	check.Score = 100
	if provider != "" {
		check.Evidence = fmt.Sprintf("detected %s checkout marker %q", provider, marker)
		if product.PaymentProvider != "" && product.PaymentProvider != "none" && product.PaymentProvider != provider {
			check.Status = "warn"
			check.Score = 60
			check.Evidence += fmt.Sprintf(" but product declares payment_provider=%s", product.PaymentProvider)
		}
		return check, nil
	}
	if strings.TrimSpace(product.CheckoutURL) != "" {
		check.Evidence = "declared checkout URL"
	} else {
		check.Evidence = "checkout or sign-up link found"
	}
	return check, &ReadinessCheck{
		Key:         "payment_provider",
		Kind:        "probe",
		Status:      "warn",
		Score:       60,
		Evidence:    "no known payment provider",
		Remediation: "No known payment provider was detected. Offer Rails does not hold money and does not require Stripe, Lemon Squeezy, or Polar.",
	}
}

var ctaLinkPattern = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)

func attributionLinkDrops(body, pageURL, refParam, probeValue string) bool {
	base, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	for _, match := range ctaLinkPattern.FindAllStringSubmatch(body, -1) {
		href := html.UnescapeString(strings.TrimSpace(match[1]))
		text := strings.ToLower(strings.TrimSpace(stripHTML(match[2])))
		ref, err := url.Parse(href)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(ref)
		path := strings.TrimRight(abs.EscapedPath(), "/")
		if text != "get started" && !strings.HasSuffix(path, "/start") && !strings.HasSuffix(path, "/pricing") {
			continue
		}
		if abs.Query().Get(refParam) != probeValue {
			return true
		}
	}
	return false
}

func (e *ReadinessEngine) checkRefParamSurvives(ctx context.Context, websiteURL, pricingURL, refParam string) ReadinessCheck {
	check := ReadinessCheck{Key: "ref_param_survives", Kind: "probe", Gate: true}
	probeValue := "dtr-probe"
	probeURL, err := url.Parse(websiteURL)
	if err != nil {
		check.Status = "fail"
		check.Evidence = "invalid website url"
		check.Remediation = "Fix the product website URL."
		return check
	}
	q := probeURL.Query()
	q.Set(refParam, probeValue)
	probeURL.RawQuery = q.Encode()

	page := e.fetchPage(ctx, probeURL.String())
	if !page.OK {
		check.Status = "fail"
		check.Evidence = "probe request failed: " + page.Err
		check.Remediation = "Make the landing page reachable so attribution can be verified."
		return check
	}
	if page.FinalURL != nil && page.FinalURL.Query().Get(refParam) == probeValue {
		if attributionLinkDrops(page.Body, page.FinalURL.String(), refParam, probeValue) || e.pricingLinkDrops(ctx, pricingURL, websiteURL, refParam, probeValue) {
			check.Status = "fail"
			check.Evidence = fmt.Sprintf("a get-started or pricing link dropped ?%s=", refParam)
			check.Remediation = fmt.Sprintf("Keep ?%s= on get-started and pricing links as well as on the landing URL. It carries the click id.", refParam)
			return check
		}
		check.Status = "pass"
		check.Score = 100
		check.Evidence = fmt.Sprintf("?%s= parameter survived to the final URL", refParam)
		return check
	}
	check.Status = "fail"
	check.Evidence = fmt.Sprintf("?%s= parameter was stripped by a redirect before the final URL", refParam)
	check.Remediation = fmt.Sprintf("Preserve the ?%s= query parameter through redirects (or capture it before redirecting); it carries the click id that attribution depends on.", refParam)
	return check
}

func (e *ReadinessEngine) pricingLinkDrops(ctx context.Context, pricingURL, websiteURL, refParam, probeValue string) bool {
	pricingURL = strings.TrimSpace(pricingURL)
	if pricingURL == "" || pricingURL == websiteURL {
		return false
	}
	probe, err := url.Parse(pricingURL)
	if err != nil {
		return false
	}
	q := probe.Query()
	q.Set(refParam, probeValue)
	probe.RawQuery = q.Encode()
	page := e.fetchPage(ctx, probe.String())
	if !page.OK || page.FinalURL == nil {
		return false
	}
	return attributionLinkDrops(page.Body, page.FinalURL.String(), refParam, probeValue)
}

func linkedPageURL(site fetchedPage, websiteURL string, pattern *regexp.Regexp) string {
	if !site.OK {
		return ""
	}
	match := pattern.FindStringSubmatch(site.Body)
	if len(match) < 2 {
		return ""
	}
	ref, err := url.Parse(strings.TrimSpace(html.UnescapeString(match[1])))
	if err != nil || ref.Scheme == "javascript" || ref.Scheme == "data" {
		return ""
	}
	base := site.FinalURL
	if base == nil {
		parsed, parseErr := url.Parse(websiteURL)
		if parseErr != nil {
			return ""
		}
		base = parsed
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func (e *ReadinessEngine) checkPricingPage(ctx context.Context, pricingURL string) ReadinessCheck {
	check := ReadinessCheck{Key: "pricing_page", Kind: "probe", Gate: true}
	if strings.TrimSpace(pricingURL) == "" {
		check.Status = "fail"
		check.Evidence = "No pricing page found."
		check.Remediation = "Add a pricing page that states the price, and link it from the landing page."
		return check
	}
	page := e.fetchPage(ctx, pricingURL)
	if page.OK && pricingMarkerPattern.MatchString(page.Body) {
		check.Status = "pass"
		check.Score = 100
		check.Evidence = "pricing page resolves and states a price"
		return check
	}
	check.Status = "fail"
	if !page.OK {
		check.Evidence = "pricing page did not resolve: " + page.Err
	} else {
		check.Evidence = "pricing page resolves but no price found"
	}
	check.Remediation = "Make the pricing page reachable and state an actual price."
	return check
}

func (e *ReadinessEngine) checkDocsPresent(ctx context.Context, docsURL string) ReadinessCheck {
	check := ReadinessCheck{Key: "docs_present", Kind: "probe", Gate: true}
	if strings.TrimSpace(docsURL) == "" {
		check.Status = "fail"
		check.Evidence = "No documentation page found."
		check.Remediation = "Add a documentation page, and link it from the landing page."
		return check
	}
	page := e.fetchPage(ctx, docsURL)
	if page.OK && len(strings.TrimSpace(stripHTML(page.Body))) > 500 {
		check.Status = "pass"
		check.Score = 100
		check.Evidence = "docs resolve with non-trivial content"
		return check
	}
	check.Status = "fail"
	if !page.OK {
		check.Evidence = "docs page did not resolve: " + page.Err
	} else {
		check.Evidence = "docs page resolves but has little content"
	}
	check.Remediation = "Make the documentation reachable with real content."
	return check
}

func checkBodyMarker(site fetchedPage, key string, pattern *regexp.Regexp, gate bool, passEvidence, remediation string) ReadinessCheck {
	check := ReadinessCheck{Key: key, Kind: "probe", Gate: gate}
	if site.OK && pattern.MatchString(site.Body) {
		check.Status = "pass"
		check.Score = 100
		check.Evidence = passEvidence
		return check
	}
	check.Evidence = "marker not found on the landing page"
	check.Remediation = remediation
	if gate {
		check.Status = "fail"
		return check
	}
	check.Status = "warn"
	check.Score = 40
	return check
}

func scoreChecks(checks []ReadinessCheck) int {
	if len(checks) == 0 {
		return 0
	}
	total := 0
	for _, check := range checks {
		total += check.Score
	}
	return total / len(checks)
}

type readinessRubric struct {
	Checks   []ReadinessCheck
	Summary  string
	Category string
	Keywords []string
	Tags     []string
}

type claudeRubricResult struct {
	OnboardingClarity int      `json:"onboarding_clarity"`
	DocsQuality       int      `json:"docs_quality"`
	ValuePropClarity  int      `json:"value_prop_clarity"`
	PrimaryCategory   string   `json:"primary_category"`
	Keywords          []string `json:"keywords"`
	Tags              []string `json:"tags"`
	Summary           string   `json:"summary"`
}

// assessWithClaude scores fetched content against a distribution-readiness rubric
// and extracts the taxonomy fields the matcher scores on. Returns nil when Claude
// isn't configured or fails, so only deterministic probes gate readiness.
func (e *ReadinessEngine) assessWithClaude(ctx context.Context, product ReadinessProduct, siteBody string) *readinessRubric {
	if e.APIKey == "" || e.Model == "" {
		return nil
	}
	apiURL := strings.TrimSpace(e.APIURL)
	if apiURL == "" {
		apiURL = "https://api.anthropic.com/v1/messages"
	}
	apiVersion := strings.TrimSpace(e.APIVersion)
	if apiVersion == "" {
		apiVersion = "2023-06-01"
	}
	content := stripHTML(siteBody)
	if len(content) > 4000 {
		content = content[:4000]
	}
	prompt := fmt.Sprintf(`Assess this product's distribution readiness for an affiliate network of bloggers.
Product: %s — %s (website %s)

Landing page text:
%s

Return JSON only: {"onboarding_clarity":0-100,"docs_quality":0-100,"value_prop_clarity":0-100,"primary_category":"one lowercase word","keywords":["..."],"tags":["..."],"summary":"2 sentences for a human reviewer"}`,
		product.Name, product.Tagline, product.WebsiteURL, content)
	payload := claudeMessageRequest{
		Model:       e.Model,
		MaxTokens:   700,
		Temperature: 0,
		System:      "You assess products for distribution readiness. Return JSON only. Do not add any extra text.",
		Messages:    []claudeMessage{{Role: "user", Content: prompt}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.APIKey)
	req.Header.Set("anthropic-version", apiVersion)
	resp, err := e.httpClient().Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var parsed claudeMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil
	}
	text := ""
	for _, block := range parsed.Content {
		if block.Type == "text" && block.Text != "" {
			text = block.Text
			break
		}
	}
	var rubric claudeRubricResult
	if err := json.Unmarshal([]byte(text), &rubric); err != nil {
		return nil
	}
	return &readinessRubric{
		Checks: []ReadinessCheck{
			aiCheck("onboarding_clarity", rubric.OnboardingClarity, "Simplify the signup flow so a first-time visitor can start without guidance."),
			aiCheck("docs_quality", rubric.DocsQuality, "Expand the documentation so publishers can reference real usage."),
			aiCheck("value_prop_clarity", rubric.ValuePropClarity, "State on the landing page what the product does and who it is for."),
		},
		Summary:  strings.TrimSpace(rubric.Summary),
		Category: strings.TrimSpace(strings.ToLower(rubric.PrimaryCategory)),
		Keywords: rubric.Keywords,
		Tags:     rubric.Tags,
	}
}

func aiCheck(key string, score int, remediation string) ReadinessCheck {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	check := ReadinessCheck{Key: key, Kind: "ai", Score: score}
	switch {
	case score >= 70:
		check.Status = "pass"
	case score >= 40:
		check.Status = "warn"
		check.Remediation = remediation
	default:
		check.Status = "fail"
		check.Remediation = remediation
	}
	check.Evidence = fmt.Sprintf("rubric score %d/100", score)
	return check
}

func stripHTML(body string) string {
	text := htmlTagPattern.ReplaceAllString(body, " ")
	return strings.Join(strings.Fields(text), " ")
}
