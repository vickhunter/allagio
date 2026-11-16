// Package outbound is the only production dialer for a URL this process
// does not already know. It refuses loopback, private, link-local, and
// metadata addresses, pins the IP it checked, and checks again on every
// redirect so a DNS change cannot retarget the connection.
package outbound

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout   = 10 * time.Second
	defaultRedirects = 10
	maxURLBytes      = 2048
)

var (
	// ErrInvalid is a URL this client will not request.
	ErrInvalid = errors.New("invalid url")
	// ErrBlocked is a host that is not a public address.
	ErrBlocked = errors.New("blocked address")
)

// Resolver checks and dials public URLs. A zero Resolver uses the process
// resolver and dialer. AllowLoopback is for tests that post to httptest;
// production leaves it false. Private, link-local, and metadata addresses
// stay blocked either way.
type Resolver struct {
	LookupIP        func(ctx context.Context, host string) ([]net.IP, error)
	DialContext     func(ctx context.Context, network, address string) (net.Conn, error)
	TLSClientConfig *tls.Config
	AllowLoopback   bool
}

// Options controls one request. A zero Options follows at most ten redirects.
type Options struct {
	Timeout time.Duration
	Header  http.Header
	// RedirectLimit is the net/http via length that stops further redirects.
	// Zero means the standard ten. NoRedirect returns the first response.
	RedirectLimit int
	NoRedirect    bool
	// BeforeRedirect runs after the address check on each hop.
	BeforeRedirect func(*http.Request) error
}

// Check resolves rawURL and reports whether it is safe to dial. It does not dial.
func (r Resolver) Check(ctx context.Context, rawURL string) error {
	parsed, err := parseURL(rawURL)
	if err != nil {
		return err
	}
	_, err = r.resolve(ctx, parsed.Hostname())
	return err
}

// Do sends one request. The dial uses the IP Check accepted, and every
// redirect is checked again. The caller closes a non-nil response body.
// On error, Do has already closed any body.
func (r Resolver) Do(ctx context.Context, method, rawURL string, body []byte, opt Options) (*http.Response, error) {
	parsed, err := parseURL(rawURL)
	if err != nil {
		return nil, err
	}
	ip, err := r.resolve(ctx, parsed.Hostname())
	if err != nil {
		return nil, err
	}
	pins := &pinSet{hosts: map[string]net.IP{}}
	pins.set(parsed.Hostname(), ip)

	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	limit := opt.RedirectLimit
	if limit <= 0 {
		limit = defaultRedirects
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableKeepAlives = true
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return r.dialPinned(ctx, network, address, pins, timeout)
	}
	if r.TLSClientConfig != nil {
		transport.TLSClientConfig = r.TLSClientConfig.Clone()
	}
	client := NewClient(timeout, transport, func(req *http.Request, via []*http.Request) error {
		if opt.NoRedirect {
			return http.ErrUseLastResponse
		}
		if len(via) >= limit {
			return errors.New("too many redirects")
		}
		if _, hopErr := parseURL(req.URL.String()); hopErr != nil {
			return hopErr
		}
		hop, hopErr := r.resolve(req.Context(), req.URL.Hostname())
		if hopErr != nil {
			return hopErr
		}
		pins.set(req.URL.Hostname(), hop)
		if opt.BeforeRedirect != nil {
			return opt.BeforeRedirect(req)
		}
		return nil
	})
	defer client.CloseIdleConnections()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), reader)
	if err != nil {
		return nil, ErrInvalid
	}
	for key, values := range opt.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
			resp.Body.Close()
		}
		return nil, err
	}
	return resp, nil
}

func (r Resolver) dialPinned(ctx context.Context, network, address string, pins *pinSet, timeout time.Duration) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrInvalid
	}
	ip, ok := pins.get(host)
	if !ok || r.blocked(ip) {
		return nil, ErrBlocked
	}
	target := net.JoinHostPort(ip.String(), port)
	if r.DialContext != nil {
		return r.DialContext(ctx, network, target)
	}
	// AllowLoopback still has to open a socket. DialPublic refuses every
	// blocked address, including the loopback exception tests opt into.
	if BlockedIP(ip) {
		if r.AllowLoopback && loopback(ip) {
			return dialSocket(ctx, network, target, timeout)
		}
		return nil, ErrBlocked
	}
	return DialPublic(ctx, network, target, timeout)
}

func (r Resolver) blocked(ip net.IP) bool {
	if !BlockedIP(ip) {
		return false
	}
	return !(r.AllowLoopback && loopback(ip))
}

func loopback(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip.IsLoopback()
}

func (r Resolver) resolve(ctx context.Context, host string) (net.IP, error) {
	host = strings.Trim(host, "[]")
	host = strings.TrimSuffix(host, ".")
	if ip := net.ParseIP(host); ip != nil {
		if r.blocked(ip) {
			return nil, ErrBlocked
		}
		return ip, nil
	}
	lookup := r.LookupIP
	if lookup == nil {
		lookup = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := lookup(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, ErrInvalid
	}
	var chosen net.IP
	for _, ip := range ips {
		if ip == nil || r.blocked(ip) {
			return nil, ErrBlocked
		}
		if chosen == nil {
			chosen = ip
		}
	}
	if chosen == nil {
		return nil, ErrBlocked
	}
	return chosen, nil
}

func parseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLBytes || strings.ContainsAny(raw, "\r\n\t \\") {
		return nil, ErrInvalid
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil {
		return nil, ErrInvalid
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalid
	}
	if parsed.Opaque != "" {
		return nil, ErrInvalid
	}
	return parsed, nil
}

// NewClient is the production http.Client constructor. Callers outside this
// package build a transport and a redirect policy; they do not write the literal.
func NewClient(timeout time.Duration, transport http.RoundTripper, checkRedirect func(*http.Request, []*http.Request) error) *http.Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: checkRedirect}
}

// DialPublic dials address when its host is an IP that BlockedIP allows.
// Callers pass the pinned IP, not the name that was resolved.
func DialPublic(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil || BlockedIP(ip) {
		return nil, ErrBlocked
	}
	return dialSocket(ctx, network, address, timeout)
}

func dialSocket(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, address)
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

var cgnatNet = mustCIDR("100.64.0.0/10")

func mustCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

// BlockedIP reports addresses a public fetch must not dial.
// IPv4-mapped IPv6 is unwrapped and judged as IPv4.
// The preview fetcher and every other outbound caller share this check.
func BlockedIP(ip net.IP) bool {
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
