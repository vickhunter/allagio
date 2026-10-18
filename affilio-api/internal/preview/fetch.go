// Package preview fetches a public HTML page without storing it.
// The dialer uses the IP that passed the address check, so a DNS record
// that changes between the check and the connection cannot retarget the fetch.
package preview

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	fetchTimeout = 5 * time.Second
	maxBody      = 2 << 20
	maxRedirects = 3
	maxURLBytes  = 2048
)

var (
	// ErrInvalidURL is a URL the preview will not request.
	ErrInvalidURL = errors.New("invalid url")
	// ErrBlocked is a host that resolves to a non-public address.
	ErrBlocked = errors.New("blocked address")
	// ErrFetch is a page that could not be read as HTML.
	ErrFetch = errors.New("fetch failed")
	// ErrUnreadable is a private post, a login wall, or a page with no text.
	ErrUnreadable = errors.New("unreadable or private")
)

const (
	kindHTML    = "html"
	kindTwitter = "twitter"
	kindYouTube = "youtube"
)

// Page is the title and visible text of a public HTML page.
type Page struct {
	Title string
	Text  string
}

// Fetcher GETs one public page. A zero Fetcher uses the process resolver and dialer.
// TLSClientConfig is for tests that terminate TLS locally. Leave it nil so the
// live client keeps verifying certificates.
type Fetcher struct {
	LookupIP        func(ctx context.Context, host string) ([]net.IP, error)
	DialContext     func(ctx context.Context, network, address string) (net.Conn, error)
	TLSClientConfig *tls.Config
}

// Fetch downloads rawURL and returns its title and main text.
// An X or Twitter post is read from the public oEmbed endpoint. A YouTube
// URL is read from YouTube's oEmbed endpoint. Any other URL is the HTML page,
// including its Open Graph and description tags. It does not log the URL or the body.
func (f Fetcher) Fetch(ctx context.Context, rawURL string) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	pageURL, err := parsePublicURL(rawURL)
	if err != nil {
		return Page{}, err
	}
	target, kind := embedTarget(pageURL)
	pins := &pinSet{hosts: map[string]net.IP{}}
	if err = f.admit(ctx, pins, target); err != nil {
		return Page{}, err
	}

	client := f.client(pins)
	defer client.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Page{}, ErrInvalidURL
	}
	req.Header.Set("User-Agent", "OfferRails-Preview/1.0")
	if kind == kindHTML {
		req.Header.Set("Accept", "text/html")
	} else {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
			resp.Body.Close()
		}
		return Page{}, classify(err)
	}
	defer resp.Body.Close()
	if unreadableStatus(kind, resp.StatusCode) {
		return Page{}, ErrUnreadable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Page{}, ErrFetch
	}
	if resp.ContentLength > maxBody {
		return Page{}, ErrFetch
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return Page{}, ErrFetch
	}
	if kind == kindHTML {
		if !htmlContent(resp.Header.Get("Content-Type")) {
			return Page{}, ErrFetch
		}
		return pageFromHTML(string(body))
	}
	if !jsonContent(resp.Header.Get("Content-Type")) {
		if htmlContent(resp.Header.Get("Content-Type")) {
			return Page{}, ErrUnreadable
		}
		return Page{}, ErrFetch
	}
	return pageFromOEmbed(kind, body)
}

// client speaks HTTP/1.1 on the pinned address. Clearing TLSNextProto only
// disables the HTTP/2 transport. The cloned client still offers h2, and a
// server that accepts that offer answers with a preface the HTTP/1 reader
// rejects. NextProtos keeps the handshake on HTTP/1.1.
func (f Fetcher) client(pins *pinSet) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	tlsConfig := &tls.Config{NextProtos: []string{"http/1.1"}}
	if f.TLSClientConfig != nil {
		tlsConfig = f.TLSClientConfig.Clone()
	}
	tlsConfig.NextProtos = []string{"http/1.1"}
	transport.TLSClientConfig = tlsConfig
	transport.TLSHandshakeTimeout = fetchTimeout
	transport.ResponseHeaderTimeout = fetchTimeout
	transport.DisableKeepAlives = true
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return nil, ErrInvalidURL
		}
		ip, ok := pins.get(host)
		if !ok || blockedIP(ip) {
			return nil, ErrBlocked
		}
		target := net.JoinHostPort(ip.String(), port)
		if f.DialContext != nil {
			return f.DialContext(ctx, network, target)
		}
		return (&net.Dialer{Timeout: fetchTimeout}).DialContext(ctx, network, target)
	}
	return &http.Client{
		Timeout:   fetchTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects+1 {
				return ErrFetch
			}
			return f.admit(req.Context(), pins, req.URL.String())
		},
	}
}

// screen is the address check Fetch runs before it dials.
func (f Fetcher) screen(ctx context.Context, rawURL string) error {
	if _, err := parsePublicURL(rawURL); err != nil {
		return err
	}
	return f.admit(ctx, &pinSet{hosts: map[string]net.IP{}}, rawURL)
}

func (f Fetcher) admit(ctx context.Context, pins *pinSet, rawURL string) error {
	parsed, err := parseFetchURL(rawURL)
	if err != nil {
		return err
	}
	ip, err := f.resolve(ctx, parsed.Hostname())
	if err != nil {
		return err
	}
	pins.set(parsed.Hostname(), ip)
	return nil
}

func parsePublicURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > maxURLBytes {
		return nil, ErrInvalidURL
	}
	return parseFetchURL(raw)
}

func parseFetchURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 8192 || strings.ContainsAny(raw, "\r\n\t \\") {
		return nil, ErrInvalidURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil {
		return nil, ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if !opaquePath(parsed) {
		return nil, ErrInvalidURL
	}
	port := parsed.Port()
	if port != "" && port != "80" && port != "443" {
		return nil, ErrInvalidURL
	}
	return parsed, nil
}

func opaquePath(parsed *url.URL) bool {
	return parsed.Opaque == ""
}

func (f Fetcher) resolve(ctx context.Context, host string) (net.IP, error) {
	host = strings.Trim(host, "[]")
	host = strings.TrimSuffix(host, ".")
	if ip := net.ParseIP(host); ip != nil {
		if blockedIP(ip) {
			return nil, ErrBlocked
		}
		return ip, nil
	}
	lookup := f.LookupIP
	if lookup == nil {
		lookup = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := lookup(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, ErrFetch
	}
	var chosen net.IP
	for _, ip := range ips {
		if ip == nil || blockedIP(ip) {
			return nil, ErrBlocked
		}
		if chosen == nil {
			chosen = ip
		}
	}
	if chosen == nil {
		return nil, ErrFetch
	}
	return chosen, nil
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrBlocked) {
		return ErrBlocked
	}
	if errors.Is(err, ErrInvalidURL) {
		return ErrInvalidURL
	}
	if errors.Is(err, ErrUnreadable) {
		return ErrUnreadable
	}
	return ErrFetch
}

func htmlContent(header string) bool {
	media, _, err := mime.ParseMediaType(header)
	return err == nil && strings.EqualFold(media, "text/html")
}

func jsonContent(header string) bool {
	media, _, err := mime.ParseMediaType(header)
	return err == nil && strings.EqualFold(media, "application/json")
}

type pinSet struct {
	mu    sync.Mutex
	hosts map[string]net.IP
}

func (p *pinSet) set(host string, ip net.IP) {
	p.mu.Lock()
	p.hosts[hostKey(host)] = append(net.IP(nil), ip...)
	p.mu.Unlock()
}

func (p *pinSet) get(host string) (net.IP, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ip, ok := p.hosts[hostKey(host)]
	if !ok {
		return nil, false
	}
	return append(net.IP(nil), ip...), true
}

func hostKey(host string) string {
	host = strings.Trim(host, "[]")
	host = strings.TrimSuffix(host, ".")
	return strings.ToLower(host)
}

var (
	cgnatNet = mustCIDR("100.64.0.0/10")
)

func mustCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

// blockedIP reports addresses a preview must not dial.
// IPv4-mapped IPv6 is unwrapped and judged as IPv4.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && cgnatNet.Contains(v4) {
		return true
	}
	return false
}

var (
	titlePattern   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	headPattern    = regexp.MustCompile(`(?is)<head\b[^>]*>.*?</head>`)
	mainPattern    = regexp.MustCompile(`(?is)<main\b[^>]*>(.*?)</main>`)
	articlePattern = regexp.MustCompile(`(?is)<article\b[^>]*>(.*?)</article>`)
	commentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)
	dropPattern    = regexp.MustCompile(`(?is)<(script|style|noscript|nav|header|footer|form|svg|iframe|template)\b[^>]*>.*?</(script|style|noscript|nav|header|footer|form|svg|iframe|template)>`)
	breakPattern   = regexp.MustCompile(`(?i)</\s*(p|div|h[1-6]|li|section|article|main|blockquote|tr)\b[^>]*>|<\s*br\s*/?\s*>`)
	tagPattern     = regexp.MustCompile(`(?s)<[^>]+>`)
)

func extractPage(raw string) (string, string) {
	raw = commentPattern.ReplaceAllString(raw, " ")
	title := ""
	if match := titlePattern.FindStringSubmatch(raw); match != nil {
		title = cleanLine(match[1])
	}
	source := headPattern.ReplaceAllString(raw, " ")
	if match := mainPattern.FindStringSubmatch(raw); match != nil {
		source = match[1]
	} else if match := articlePattern.FindStringSubmatch(raw); match != nil {
		source = match[1]
	}
	source = dropPattern.ReplaceAllString(source, " ")
	source = breakPattern.ReplaceAllString(source, "\n\n")
	source = tagPattern.ReplaceAllString(source, " ")
	source = html.UnescapeString(source)
	return title, normalizeParagraphs(source)
}

func cleanLine(raw string) string {
	raw = tagPattern.ReplaceAllString(raw, " ")
	raw = html.UnescapeString(raw)
	return strings.Join(strings.Fields(raw), " ")
}

func normalizeParagraphs(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	var paragraphs []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, " "))
		current = nil
	}
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	text := strings.Join(paragraphs, "\n\n")
	runes := []rune(text)
	if len(runes) > 20000 {
		text = string(runes[:20000])
	}
	return text
}

func embedTarget(page *url.URL) (string, string) {
	host := page.Hostname()
	switch {
	case hostIs(host, "twitter.com", "x.com"):
		return "https://publish.twitter.com/oembed?url=" + url.QueryEscape(page.String()), kindTwitter
	case hostIs(host, "youtube.com", "youtu.be"):
		return "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(page.String()), kindYouTube
	default:
		return page.String(), kindHTML
	}
}

func hostIs(host string, names ...string) bool {
	host = hostKey(host)
	for _, name := range names {
		if host == name || strings.HasSuffix(host, "."+name) {
			return true
		}
	}
	return false
}

func unreadableStatus(kind string, code int) bool {
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return true
	}
	return kind != kindHTML && code == http.StatusNotFound
}

func pageFromHTML(raw string) (Page, error) {
	title, text := extractPage(raw)
	title, text = applyMeta(raw, title, text)
	if strings.TrimSpace(text) == "" || loginWall(raw, title, text) {
		return Page{}, ErrUnreadable
	}
	return Page{Title: title, Text: text}, nil
}

func pageFromOEmbed(kind string, body []byte) (Page, error) {
	var doc struct {
		Title      string `json:"title"`
		AuthorName string `json:"author_name"`
		HTML       string `json:"html"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return Page{}, ErrFetch
	}
	if kind == kindYouTube {
		title := cleanLine(doc.Title)
		text := strings.TrimSpace(title + "\n\n" + cleanLine(doc.AuthorName))
		if title == "" || strings.TrimSpace(text) == "" {
			return Page{}, ErrUnreadable
		}
		return Page{Title: title, Text: text}, nil
	}
	_, text := extractPage(doc.HTML)
	text = strings.TrimSpace(text)
	if text == "" {
		return Page{}, ErrUnreadable
	}
	title := cleanLine(doc.Title)
	if title == "" {
		title = text
		if i := strings.Index(title, "\n"); i >= 0 {
			title = strings.TrimSpace(title[:i])
		}
	}
	if title == "" {
		title = cleanLine(doc.AuthorName)
	}
	return Page{Title: title, Text: text}, nil
}

var (
	metaPattern        = regexp.MustCompile(`(?i)<meta\b[^>]*>`)
	attrPattern        = regexp.MustCompile(`(?i)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
	passwordPattern    = regexp.MustCompile(`(?i)\btype\s*=\s*['"]?password\b`)
	loginPhrasePattern = regexp.MustCompile(`(?i)(?:\blog[\s-]?in\b|\bsign[\s-]?in\b|protected account|this account is private|tweets are protected|only subscribers can)`)
)

func applyMeta(raw, title, text string) (string, string) {
	metas := readMeta(raw)
	if og := firstMeta(metas, "og:title"); og != "" {
		title = og
	} else if title == "" {
		if tw := firstMeta(metas, "twitter:title"); tw != "" {
			title = tw
		}
	}
	var extra []string
	seen := map[string]struct{}{}
	for _, key := range []string{"og:description", "twitter:description", "description"} {
		value := firstMeta(metas, key)
		if value == "" || strings.Contains(text, value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		extra = append(extra, value)
	}
	if tags := metas["article:tag"]; len(tags) > 0 {
		joined := strings.Join(tags, " ")
		if _, ok := seen[joined]; !ok && !strings.Contains(text, joined) {
			extra = append(extra, joined)
		}
	}
	if len(extra) == 0 {
		return title, text
	}
	block := strings.Join(extra, "\n\n")
	if strings.TrimSpace(text) == "" {
		return title, block
	}
	return title, text + "\n\n" + block
}

func readMeta(raw string) map[string][]string {
	out := map[string][]string{}
	for _, tag := range metaPattern.FindAllString(raw, -1) {
		attrs := map[string]string{}
		for _, match := range attrPattern.FindAllStringSubmatch(tag, -1) {
			value := match[3]
			if value == "" {
				value = match[4]
			}
			if value == "" {
				value = match[5]
			}
			attrs[strings.ToLower(match[1])] = strings.Join(strings.Fields(html.UnescapeString(value)), " ")
		}
		key := attrs["property"]
		if key == "" {
			key = attrs["name"]
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" || attrs["content"] == "" {
			continue
		}
		out[key] = append(out[key], attrs["content"])
	}
	return out
}

func firstMeta(metas map[string][]string, key string) string {
	values := metas[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func loginWall(raw, title, text string) bool {
	words := len(strings.Fields(text))
	if words >= 80 {
		return false
	}
	if passwordPattern.MatchString(raw) {
		return true
	}
	if words >= 40 {
		return false
	}
	return loginPhrase(title + " " + text)
}

func loginPhrase(text string) bool {
	return loginPhrasePattern.MatchString(text)
}
