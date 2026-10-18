package preview

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

func TestSocialHostsChooseTheOEmbedEndpoint(t *testing.T) {
	cases := []struct {
		raw  string
		kind string
		host string
	}{
		{"https://twitter.com/jack/status/20", kindTwitter, "publish.twitter.com"},
		{"https://x.com/jack/status/20", kindTwitter, "publish.twitter.com"},
		{"https://mobile.twitter.com/jack/status/20", kindTwitter, "publish.twitter.com"},
		{"https://Mobile.X.com/jack/status/20", kindTwitter, "publish.twitter.com"},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", kindYouTube, "www.youtube.com"},
		{"https://youtu.be/dQw4w9WgXcQ", kindYouTube, "www.youtube.com"},
		{"https://m.youtube.com/watch?v=abc", kindYouTube, "www.youtube.com"},
		{"https://music.youtube.com/watch?v=abc", kindYouTube, "www.youtube.com"},
		{"https://nottwitter.com/jack/status/20", kindHTML, "nottwitter.com"},
		{"https://twitter.com.evil.com/jack/status/20", kindHTML, "twitter.com.evil.com"},
		{"http://example.com/post", kindHTML, "example.com"},
	}
	for _, tc := range cases {
		parsed, err := parsePublicURL(tc.raw)
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		target, kind := embedTarget(parsed)
		if kind != tc.kind {
			t.Fatalf("%s kind = %s", tc.raw, kind)
		}
		got, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		if got.Hostname() != tc.host {
			t.Fatalf("%s host = %s", tc.raw, got.Hostname())
		}
		if kind != kindHTML && got.Query().Get("url") != parsed.String() {
			t.Fatalf("%s oembed url = %s", tc.raw, got.Query().Get("url"))
		}
	}
}

func TestFetchXPostFixtureYouTubeAndUnreadablePages(t *testing.T) {
	fixture, err := os.ReadFile("testdata/x_post_oembed.json")
	if err != nil {
		t.Fatal(err)
	}
	var dialed []string
	tlsPage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "publish.twitter.com":
			if strings.Contains(r.URL.Query().Get("url"), "/status/404") {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "https://publish.x.com/oembed?"+r.URL.RawQuery, http.StatusMovedPermanently)
		case "publish.x.com":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write(fixture)
		case "www.youtube.com":
			if r.URL.Query().Get("format") != "json" {
				t.Errorf("youtube format = %q", r.URL.Query().Get("format"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"title":"Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)","author_name":"Rick Astley","html":"<iframe></iframe>"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(tlsPage.Close)
	secure := Fetcher{
		LookupIP:        func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.10")}, nil },
		DialContext:     dialPage(tlsPage, &dialed),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	dialed = nil
	got, err := secure.Fetch(context.Background(), "https://twitter.com/jack/status/20")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "just setting up my twttr" || !strings.Contains(got.Text, "just setting up my twttr") {
		t.Fatalf("x post = %+v", got)
	}
	if strings.Contains(got.Text, "widgets.js") {
		t.Fatalf("x post kept the widget script: %q", got.Text)
	}
	if len(dialed) != 2 || !strings.HasSuffix(dialed[0], ":443") {
		t.Fatalf("x dialed %v", dialed)
	}

	dialed = nil
	if _, err = secure.Fetch(context.Background(), "https://x.com/jack/status/404"); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("private oembed: %v", err)
	}

	dialed = nil
	video, err := secure.Fetch(context.Background(), "https://youtu.be/dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	if video.Title != "Rick Astley - Never Gonna Give You Up (Official Video) (4K Remaster)" || !strings.Contains(video.Text, "Rick Astley") {
		t.Fatalf("youtube = %+v", video)
	}

	htmlPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/og":
			_, _ = io.WriteString(w, `<html><head><title>Browser title</title><meta property="og:title" content="OG Title"><meta property="og:description" content="A kiln for ceramic glaze tests."><meta name="description" content="A kiln for ceramic glaze tests."><meta property="article:tag" content="ceramics"></head><body><main><p>Visible kiln notes.</p></main></body></html>`)
		case "/empty":
			_, _ = io.WriteString(w, `<html><head><title>Empty</title></head><body> </body></html>`)
		case "/login":
			_, _ = io.WriteString(w, `<html><head><title>Members</title></head><body><main><p>This account is private. Log in to continue.</p></main></body></html>`)
		case "/password":
			_, _ = io.WriteString(w, `<html><head><title>Members</title></head><body><main><p>Welcome back.</p><input type="password"></main></body></html>`)
		case "/long":
			_, _ = io.WriteString(w, `<html><head><title>Notes</title></head><body><main><p>`+strings.Repeat("word ", 90)+`This article mentions login once.</p></main></body></html>`)
		case "/bare":
			_, _ = io.WriteString(w, `<html><head><title>InboxPilot email inbox</title></head><body><p>A short public note.</p></body></html>`)
		case "/gone":
			http.NotFound(w, r)
		case "/denied":
			http.Error(w, "no", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(htmlPage.Close)
	plain := Fetcher{
		LookupIP:    func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.10")}, nil },
		DialContext: dialPage(htmlPage, &dialed),
	}
	dialed = nil
	og, err := plain.Fetch(context.Background(), "http://nottwitter.com/og")
	if err != nil {
		t.Fatal(err)
	}
	if og.Title != "OG Title" || !strings.Contains(og.Text, "Visible kiln notes.") || !strings.Contains(og.Text, "A kiln for ceramic glaze tests.") || !strings.Contains(og.Text, "ceramics") {
		t.Fatalf("og = %+v", og)
	}
	if strings.Count(og.Text, "A kiln for ceramic glaze tests.") != 1 {
		t.Fatalf("description repeated: %q", og.Text)
	}
	if len(dialed) != 1 || dialed[0] != "203.0.113.10:80" {
		t.Fatalf("html dialed %v", dialed)
	}
	for _, path := range []string{"/empty", "/login", "/password", "/denied"} {
		if _, err = plain.Fetch(context.Background(), "http://preview.example"+path); !errors.Is(err, ErrUnreadable) {
			t.Fatalf("%s: %v", path, err)
		}
	}
	long, err := plain.Fetch(context.Background(), "http://preview.example/long")
	if err != nil || !strings.Contains(long.Text, "login") {
		t.Fatalf("long article: %v %+v", err, long)
	}
	if _, err = plain.Fetch(context.Background(), "http://preview.example/gone"); !errors.Is(err, ErrFetch) {
		t.Fatalf("html 404: %v", err)
	}
	evil, err := plain.Fetch(context.Background(), "http://twitter.com.evil.com/og")
	if err != nil || evil.Title != "OG Title" {
		t.Fatalf("lookalike host: %v %+v", err, evil)
	}
	bare, err := plain.Fetch(context.Background(), "http://preview.example/bare")
	if err != nil || !strings.Contains(bare.Text, "InboxPilot email inbox") || !strings.Contains(bare.Text, "A short public note.") {
		t.Fatalf("title was dropped from the match text: %v %+v", err, bare)
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
