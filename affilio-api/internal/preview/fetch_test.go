package preview

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGuardBlocksEachRange(t *testing.T) {
	blocked := []struct {
		name string
		ip   string
	}{
		{"loopback v4", "127.0.0.1"},
		{"loopback v4 upper", "127.255.255.255"},
		{"loopback v6", "::1"},
		{"private 10", "10.0.0.1"},
		{"private 10 upper", "10.255.255.255"},
		{"private 172.16", "172.16.0.1"},
		{"private 172.31", "172.31.255.255"},
		{"private 192.168", "192.168.1.1"},
		{"unique local fc00", "fc00::1"},
		{"unique local fd00", "fd00::1"},
		{"link-local v4", "169.254.169.254"},
		{"link-local v4 low", "169.254.0.1"},
		{"link-local v6", "fe80::1"},
		{"cgnat low", "100.64.0.1"},
		{"cgnat high", "100.127.255.254"},
		{"multicast v4", "224.0.0.1"},
		{"multicast v4 high", "239.255.255.255"},
		{"multicast v6", "ff00::1"},
		{"link-local multicast v6", "ff02::1"},
		{"unspecified v4", "0.0.0.0"},
		{"unspecified v6", "::"},
		{"ipv4-mapped loopback", "::ffff:127.0.0.1"},
		{"ipv4-mapped private", "::ffff:10.1.2.3"},
		{"ipv4-mapped 172.16", "::ffff:172.16.5.5"},
		{"ipv4-mapped 192.168", "::ffff:192.168.0.1"},
		{"ipv4-mapped link-local", "::ffff:169.254.169.254"},
		{"ipv4-mapped cgnat", "::ffff:100.64.1.1"},
		{"ipv4-mapped multicast", "::ffff:224.0.0.1"},
		{"ipv4-mapped unspecified", "::ffff:0.0.0.0"},
	}
	allowed := []string{
		"8.8.8.8",
		"1.1.1.1",
		"172.15.255.255",
		"172.32.0.1",
		"100.63.255.255",
		"100.128.0.1",
		"11.0.0.1",
		"192.167.0.1",
		"169.255.0.1",
		"223.255.255.255",
		"203.0.113.10",
		"::ffff:8.8.8.8",
		"2001:4860:4860::8888",
	}
	for _, tc := range blocked {
		t.Run(tc.name, func(t *testing.T) {
			if !blockedIP(net.ParseIP(tc.ip)) {
				t.Fatalf("%s was allowed", tc.ip)
			}
			f := Fetcher{LookupIP: func(context.Context, string) ([]net.IP, error) {
				return []net.IP{net.ParseIP(tc.ip)}, nil
			}, DialContext: func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("blocked address was dialed")
				return nil, errors.New("dialed")
			}}
			if err := f.screen(context.Background(), "http://"+literalHost(tc.ip)+"/secret"); !errors.Is(err, ErrBlocked) {
				t.Fatalf("literal %s: %v", tc.ip, err)
			}
			if err := f.screen(context.Background(), "http://blocked.example/secret"); !errors.Is(err, ErrBlocked) {
				t.Fatalf("dns %s: %v", tc.ip, err)
			}
		})
	}
	for _, ip := range allowed {
		if blockedIP(net.ParseIP(ip)) {
			t.Fatalf("public address blocked: %s", ip)
		}
	}
}

func TestGuardRejectsMixedPublicAndPrivateRecords(t *testing.T) {
	dialed := false
	f := Fetcher{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("127.0.0.1")}, nil
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("dialed")
		},
	}
	err := f.screen(context.Background(), "http://mixed.example/page")
	if !errors.Is(err, ErrBlocked) || dialed {
		t.Fatalf("err=%v dialed=%v", err, dialed)
	}
}

func TestGuardRejectsBadURLs(t *testing.T) {
	f := Fetcher{}
	for _, raw := range []string{
		"",
		"not a url",
		"file:///etc/passwd",
		"ftp://example.com/",
		"http://user:pass@example.com/",
		"http://example.com:8080/",
		"https://example.com:8443/",
		"http://example.com:22/",
		"gopher://example.com/",
		"http://",
	} {
		if err := f.screen(context.Background(), raw); !errors.Is(err, ErrInvalidURL) {
			t.Fatalf("%q: %v", raw, err)
		}
	}
}

func TestGuardRedirectToPrivate(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/literal":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
		case "/dns":
			http.Redirect(w, r, "http://inside.example/secret", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(page.Close)

	var dialed []string
	f := Fetcher{
		LookupIP: func(_ context.Context, host string) ([]net.IP, error) {
			switch host {
			case "preview.example":
				return []net.IP{net.ParseIP("203.0.113.10")}, nil
			case "inside.example":
				return []net.IP{net.ParseIP("10.0.0.1")}, nil
			default:
				return nil, errors.New("nxdomain")
			}
		},
		DialContext: dialPage(page, &dialed),
	}
	for _, path := range []string{"/literal", "/dns"} {
		dialed = nil
		_, err := f.Fetch(context.Background(), "http://preview.example"+path)
		if !errors.Is(err, ErrBlocked) {
			t.Fatalf("%s: %v", path, err)
		}
		if len(dialed) != 1 || dialed[0] != "203.0.113.10:80" {
			t.Fatalf("%s dialed %v", path, dialed)
		}
	}
}

func TestGuardPinsCheckedIPAgainstRebinding(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><title>Pinned</title><main><p>Public page text.</p></main></html>`)
	}))
	t.Cleanup(page.Close)

	var lookups int
	var dialed []string
	f := Fetcher{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			lookups++
			if lookups == 1 {
				return []net.IP{net.ParseIP("203.0.113.77")}, nil
			}
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		},
		DialContext: dialPage(page, &dialed),
	}
	pageOut, err := f.Fetch(context.Background(), "http://rebind.example/article")
	if err != nil {
		t.Fatal(err)
	}
	if lookups != 1 {
		t.Fatalf("lookup count = %d, dialed %v", lookups, dialed)
	}
	if len(dialed) != 1 || dialed[0] != "203.0.113.77:80" {
		t.Fatalf("dialed %v", dialed)
	}
	if pageOut.Title != "Pinned" || !strings.Contains(pageOut.Text, "Public page text") {
		t.Fatalf("page = %+v", pageOut)
	}
}

func TestGuardRechecksEveryRedirectAndStopsAfterThree(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "http://preview.example/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "http://second.example/c", http.StatusFound)
		case "/c":
			http.Redirect(w, r, "http://preview.example/ok", http.StatusFound)
		case "/d":
			http.Redirect(w, r, "http://preview.example/e", http.StatusFound)
		case "/e":
			http.Redirect(w, r, "http://preview.example/f", http.StatusFound)
		case "/f":
			http.Redirect(w, r, "http://preview.example/g", http.StatusFound)
		case "/g":
			http.Redirect(w, r, "http://preview.example/too-far", http.StatusFound)
		case "/ok":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<html><title>Landed</title><main><p>Hello from the last page.</p></main></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(page.Close)

	var dialed []string
	f := Fetcher{
		LookupIP: func(_ context.Context, host string) ([]net.IP, error) {
			switch host {
			case "preview.example":
				return []net.IP{net.ParseIP("203.0.113.10")}, nil
			case "second.example":
				return []net.IP{net.ParseIP("203.0.113.88")}, nil
			default:
				return nil, errors.New("nxdomain")
			}
		},
		DialContext: dialPage(page, &dialed),
	}
	got, err := f.Fetch(context.Background(), "http://preview.example/a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Landed" {
		t.Fatalf("title = %q", got.Title)
	}
	want := []string{"203.0.113.10:80", "203.0.113.10:80", "203.0.113.88:80", "203.0.113.10:80"}
	if strings.Join(dialed, ",") != strings.Join(want, ",") {
		t.Fatalf("dialed %v", dialed)
	}

	dialed = nil
	_, err = f.Fetch(context.Background(), "http://preview.example/d")
	if !errors.Is(err, ErrFetch) {
		t.Fatalf("fourth redirect: %v dialed %v", err, dialed)
	}
	if len(dialed) != 4 {
		t.Fatalf("fourth redirect dialed %v", dialed)
	}
}

func TestFetchRejectsNonHTMLAndHugeBody(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true}`)
		case "/big":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(make([]byte, maxBody+1))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(page.Close)
	var dialed []string
	f := Fetcher{
		LookupIP:    func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.10")}, nil },
		DialContext: dialPage(page, &dialed),
	}
	if _, err := f.Fetch(context.Background(), "http://preview.example/json"); !errors.Is(err, ErrFetch) {
		t.Fatalf("json: %v", err)
	}
	if _, err := f.Fetch(context.Background(), "http://preview.example/big"); !errors.Is(err, ErrFetch) {
		t.Fatalf("big: %v", err)
	}
}

func TestExtractTitleAndMainText(t *testing.T) {
	raw := `<html><head><title>Inbox &amp; Mail</title><style>body{color:red}</style></head>
<body><nav>skip me</nav><main><p>First paragraph.</p><script>secret()</script><p>Second paragraph.</p></main></body></html>`
	title, text := extractPage(raw)
	if title != "Inbox & Mail" {
		t.Fatalf("title = %q", title)
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "skip me") || strings.Contains(text, "color") {
		t.Fatalf("text = %q", text)
	}
	if !strings.Contains(text, "First paragraph.") || !strings.Contains(text, "Second paragraph.") {
		t.Fatalf("text = %q", text)
	}
	if !strings.Contains(text, "\n\n") {
		t.Fatalf("paragraphs were collapsed: %q", text)
	}
}

func TestFetchDocumentReadsPlainTextAndRefusesCrossHost(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/offer-rails.txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "token-line\n")
		case "/away":
			http.Redirect(w, r, "http://other.example/token", http.StatusFound)
		case "/scheme":
			http.Redirect(w, r, "https://preview.example/.well-known/offer-rails.txt", http.StatusFound)
		case "/port":
			http.Redirect(w, r, "http://preview.example:443/token", http.StatusFound)
		case "/same":
			http.Redirect(w, r, "http://preview.example/.well-known/offer-rails.txt", http.StatusFound)
		case "/token":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "stolen\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(page.Close)
	var dialed []string
	f := Fetcher{
		LookupIP: func(_ context.Context, host string) ([]net.IP, error) {
			switch host {
			case "preview.example":
				return []net.IP{net.ParseIP("203.0.113.10")}, nil
			case "other.example":
				return []net.IP{net.ParseIP("203.0.113.88")}, nil
			default:
				return nil, errors.New("nxdomain")
			}
		},
		DialContext: dialPage(page, &dialed),
	}
	doc, err := f.FetchDocument(context.Background(), "http://preview.example/.well-known/offer-rails.txt")
	if err != nil || string(doc.Body) != "token-line\n" || !strings.Contains(doc.ContentType, "text/plain") {
		t.Fatalf("document: %v %q %q", err, doc.ContentType, doc.Body)
	}
	if len(dialed) != 1 || dialed[0] != "203.0.113.10:80" {
		t.Fatalf("dialed %v", dialed)
	}
	dialed = nil
	_, err = f.FetchDocument(context.Background(), "http://preview.example/away")
	if !errors.Is(err, ErrFetch) {
		t.Fatalf("cross-host redirect: %v", err)
	}
	for _, address := range dialed {
		if strings.HasPrefix(address, "203.0.113.88") {
			t.Fatalf("cross-host redirect was dialed: %v", dialed)
		}
	}
	dialed = nil
	_, err = f.FetchDocument(context.Background(), "http://preview.example/scheme")
	if !errors.Is(err, ErrFetch) || strings.Contains(strings.Join(dialed, ","), ":443") {
		t.Fatalf("scheme change: %v dialed %v", err, dialed)
	}
	dialed = nil
	_, err = f.FetchDocument(context.Background(), "http://preview.example/port")
	if !errors.Is(err, ErrFetch) || strings.Contains(strings.Join(dialed, ","), ":443") {
		t.Fatalf("port change: %v dialed %v", err, dialed)
	}
	dialed = nil
	doc, err = f.FetchDocument(context.Background(), "http://preview.example/same")
	if err != nil || string(doc.Body) != "token-line\n" {
		t.Fatalf("same-host redirect: %v %q dialed %v", err, doc.Body, dialed)
	}

	blocked := Fetcher{
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("blocked address was dialed")
			return nil, errors.New("dialed")
		},
	}
	if _, err := blocked.FetchDocument(context.Background(), "http://preview.example/secret"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("blocked: %v", err)
	}
}

func TestTransportSpeaksHTTP1(t *testing.T) {
	client := (Fetcher{}).client(&pinSet{hosts: map[string]net.IP{}})
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("preview client has no http transport")
	}
	if transport.ForceAttemptHTTP2 {
		t.Fatal("preview transport offers http2")
	}
	if _, offered := transport.TLSNextProto["h2"]; offered || len(transport.TLSNextProto) != 0 {
		t.Fatalf("TLSNextProto = %#v", transport.TLSNextProto)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("preview transport has no tls config")
	}
	got := transport.TLSClientConfig.NextProtos
	if len(got) != 1 || got[0] != "http/1.1" {
		t.Fatalf("NextProtos = %v", got)
	}
}

func literalHost(ip string) string {
	if strings.Contains(ip, ":") {
		return "[" + ip + "]"
	}
	return ip
}

func dialPage(page *httptest.Server, dialed *[]string) func(context.Context, string, string) (net.Conn, error) {
	var mu sync.Mutex
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		mu.Lock()
		*dialed = append(*dialed, address)
		mu.Unlock()
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if blockedIP(net.ParseIP(host)) {
			return nil, errors.New("dialed blocked " + address)
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", page.Listener.Addr().String())
	}
}
