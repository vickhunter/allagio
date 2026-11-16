package good

import (
	"net"
	"net/http"
)

func allowed() {
	//allagio:outbound fixed-host example API
	_ = &http.Client{Timeout: 1}
	//allagio:outbound operator-config EXAMPLE_URL
	_, _ = http.Get("https://example.com")
	d := &net.Dialer{}
	//allagio:outbound operator-config DATABASE_URL host
	_, _ = d.Dial("tcp", "db.internal:5432")
}
