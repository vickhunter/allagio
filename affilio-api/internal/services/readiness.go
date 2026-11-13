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
	// LookupIPAddr and DialContext are test seams. Production leaves them nil.
	LookupIPAddr func(context.Context, string) ([]net.IPAddr, error)
	DialContext  func(context.Context, string, string) (net.Conn, error)
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
	termsMarkerPattern      = regexp.MustCompile(`(?i)href="[^"]*terms[^"]*"`)
	checkoutActionText      = regexp.MustCompile(`(?i)\b(buy|subscribe|checkout|start|sign[\s-]*up|get\s+started)\b`)
	checkoutHrefMarker      = regexp.MustCompile(`(?i)(?:^|[/?#=_-])(buy|subscribe|checkout|signup|sign-up|get-started|start)(?:[/?#=_./-]|$)`)
	actionTagPattern        = regexp.MustCompile(`(?is)<(a|button)\b([^>]*)>(.*?)</(a|button)>`)
	hrefAttrPattern         = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']*)["']`)
	htmlTagPattern          = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]*>`)
	appShellScriptPattern   = regexp.MustCompile(`(?i)<script[^>]*\bsrc\s*=`)
)

func (e *ReadinessEngine) Assess(ctx context.Context, product ReadinessProduct) ReadinessResult {
	refParam := strings.TrimSpace(product.RefParam)
	if refParam == "" {
		refParam = "via"
	}

	site := e.fetchPage(ctx, product.WebsiteURL)
	shell := looksLikeAppShell(site)
	checkout, providerWarning := e.checkCheckoutPresent(ctx, product, site)
	checks := []ReadinessCheck{
		e.checkSiteReachable(site),
		checkout,
		e.checkRefParamSurvives(ctx, product.WebsiteURL, product.PricingURL, refParam),
		e.checkPricingPage(ctx, product.PricingURL),
		e.checkDocsPresent(ctx, product.DocsURL),
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
	if shell {
		// The probe saw an empty JavaScript shell, so marker checks fail for the
		// wrong reason: links exist in the browser DOM, not in the served HTML.
		// Say so directly — the fix is prerendering, not "add a privacy link".
		checks = append(checks, ReadinessCheck{
			Key:    "client_rendered_shell",
			Kind:   "probe",
			Gate:   false,
			Status: "warn",
			Score:  40,
			Evidence:    "the landing page is a client-rendered app shell: script bundles load but the served HTML has almost no visible text",
			Remediation: "Serve prerendered or static HTML for the landing page (SSR, SSG, or a prerender service) so crawlers and agents see the content without executing JavaScript.",
		})
		for i := range checks {
			if checks[i].Status != "fail" {
				continue
			}
			switch checks[i].Key {
			case "privacy_policy", "terms", "onboarding_flow", "checkout_present":
				checks[i].Evidence += "; the page looks like a client-rendered app shell, so the marker may exist in the browser DOM but not in the served HTML"
				checks[i].Remediation = "Serve prerendered or static HTML so crawlers read the links without executing JavaScript. " + checks[i].Remediation
			}
		}
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
		return e.dialPublic(ctx, network, address)
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
	return (&ReadinessEngine{}).dialPublic(ctx, network, address)
}

func (e *ReadinessEngine) dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := e.resolvePublic(ctx, host)
	if err != nil {
		return nil, err
	}
	var dialErr error
	for _, resolved := range addresses {
		target := net.JoinHostPort(resolved.IP.String(), port)
		var conn net.Conn
		if e != nil && e.DialContext != nil {
			conn, dialErr = e.DialContext(ctx, network, target)
		} else {
			conn, dialErr = (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, target)
		}
		if dialErr == nil {
			return conn, nil
		}
	}
	if dialErr == nil {
		dialErr = errors.New("host did not resolve")
	}
	return nil, dialErr
}

func publicProbeAddresses(ctx context.Context, host string) ([]net.IPAddr, error) {
	return (&ReadinessEngine{}).resolvePublic(ctx, host)
}

func (e *ReadinessEngine) resolvePublic(ctx context.Context, host string) ([]net.IPAddr, error) {
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if blockedProbeIP(ip) {
			return nil, errors.New("host resolves to a private address")
		}
		return []net.IPAddr{{IP: ip}}, nil
	}
	lookup := net.DefaultResolver.LookupIPAddr
	if e != nil && e.LookupIPAddr != nil {
		lookup = e.LookupIPAddr
	}
	addresses, err := lookup(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("host did not resolve")
	}
	for _, address := range addresses {
		if blockedProbeIP(address.IP) {
			return nil, errors.New("host resolves to a private address")
		}
	}
	return addresses, nil
}

var probeCGNAT = mustProbeCIDR("100.64.0.0/10")

func mustProbeCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

// blockedProbeIP matches the preview fetcher. IPv4-mapped IPv6 is judged as IPv4.
// The carrier-grade NAT range is not covered by net.IP.IsPrivate.
func blockedProbeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && probeCGNAT.Contains(v4) {
		return true
	}
	return false
}

// FetchPublicPage GETs a public page with the readiness timeout and the
// 512 KiB body cap. ok is true only for an HTTP 2xx response.
func (e *ReadinessEngine) FetchPublicPage(ctx context.Context, rawURL string) (body string, status int, ok bool) {
	page := e.fetchPage(ctx, rawURL)
	return page.Body, page.Status, page.OK
}

// PostPublicJSON POSTs a JSON body to a public URL under the same SSRF rules
// as FetchPublicPage: public addresses only, redirects re-checked, the body
// cap applies to the reply. ok is true for a 2xx status.
func (e *ReadinessEngine) PostPublicJSON(ctx context.Context, rawURL string, payload []byte) (body string, status int, ok bool) {
	if err := e.hostAllowed(ctx, rawURL); err != nil {
		return "", 0, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(string(payload)))
	if err != nil {
		return "", 0, false
	}
	req.Header.Set("User-Agent", "Allagio-Readiness/1.0")
	req.Header.Set("Content-Type", "application/json")
	client := e.httpClient()
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return string(raw), resp.StatusCode, resp.StatusCode >= 200 && resp.StatusCode < 300
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
	req.Header.Set("User-Agent", "Allagio-Readiness/1.0")
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
	_, err = e.resolvePublic(ctx, parsed.Hostname())
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
		check.Remediation = "Add a buy, subscribe, checkout, or sign-up link on the landing or pricing page, or set a checkout URL. Allagio does not require a particular payment provider."
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
		Remediation: "No known payment provider was detected. Allagio does not hold money and does not require Stripe, Lemon Squeezy, or Polar.",
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

func (e *ReadinessEngine) checkPricingPage(ctx context.Context, pricingURL string) ReadinessCheck {
	check := ReadinessCheck{Key: "pricing_page", Kind: "probe", Gate: true}
	if strings.TrimSpace(pricingURL) == "" {
		check.Status = "fail"
		check.Evidence = "no pricing_url provided"
		check.Remediation = "Publish a pricing page and set pricing_url; publishers convert better when the price is visible."
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
		check.Evidence = "no docs_url provided"
		check.Remediation = "Publish documentation and set docs_url; publishers link to docs to support their recommendation."
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

var shellLinkPattern = regexp.MustCompile(`(?i)<a\b[^>]*\bhref\s*=`)

// looksLikeAppShell reports whether the fetch returned a JavaScript app shell:
// a bundle <script src> loads, the stripped text is thin, and there are almost
// no anchor links — a real landing page always links somewhere. Crawlers that
// do not execute JavaScript then see a blank page.
func looksLikeAppShell(page fetchedPage) bool {
	if !page.OK || !appShellScriptPattern.MatchString(page.Body) {
		return false
	}
	return len(stripHTML(page.Body)) < 250 && len(shellLinkPattern.FindAllString(page.Body, -1)) < 3
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
