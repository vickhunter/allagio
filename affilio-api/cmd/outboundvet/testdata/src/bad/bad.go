package bad

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
)

func raw() {
	_ = &http.Client{}           // want `raw http.Client`
	_, _ = http.Get("https://example.com") // want `raw outbound call net/http.Get`
	_ = http.DefaultClient       // want `raw http.DefaultClient`
	_ = new(http.Client)         // want `raw http.Client`
	var zero http.Client         // want `raw http.Client`
	_ = zero
	_ = &http.Transport{}        // want `raw http.Transport`
	_ = http.DefaultTransport    // want `raw http.DefaultTransport`
	_ = &tls.Dialer{}            // want `raw tls.Dialer`
	_, _ = net.Dial("tcp", "example.com:80") // want `raw outbound call net.Dial`
	_, _ = net.DialTCP("tcp", nil, nil)      // want `raw outbound call net.DialTCP`
	_, _ = net.DialUDP("udp", nil, nil)      // want `raw outbound call net.DialUDP`
	_, _ = net.DialIP("ip4", nil, nil)       // want `raw outbound call net.DialIP`
	_ = &httputil.ReverseProxy{}             // want `raw httputil.ReverseProxy`
	_ = httputil.NewSingleHostReverseProxy(nil) // want `raw outbound call net/http/httputil.NewSingleHostReverseProxy`
}
