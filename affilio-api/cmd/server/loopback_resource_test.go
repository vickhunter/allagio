package main

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"affilio-api/internal/config"
	"affilio-api/internal/handlers"

	"github.com/gin-gonic/gin"
)

func TestLoopbackMCPResource(t *testing.T) {
	got, ok := loopbackMCPResource(config.Config{Host: "127.0.0.1", Port: "8097"})
	if !ok || got != "http://127.0.0.1:8097/mcp" {
		t.Fatalf("loopback resource %q ok=%v", got, ok)
	}
	got, ok = loopbackMCPResource(config.Config{Host: "localhost", Port: "8097"})
	if !ok || got != "http://localhost:8097/mcp" {
		t.Fatalf("localhost resource %q ok=%v", got, ok)
	}
	got, ok = loopbackMCPResource(config.Config{Host: "::1", Port: "8097"})
	if !ok || got != "http://[::1]:8097/mcp" {
		t.Fatalf("ipv6 resource %q ok=%v", got, ok)
	}
	if _, ok = loopbackMCPResource(config.Config{Host: "127.0.0.1", Port: "8097", GinMode: "release"}); ok {
		t.Fatal("release mode registered a loopback resource")
	}
	if _, ok = loopbackMCPResource(config.Config{Host: "0.0.0.0", Port: "8080"}); ok {
		t.Fatal("0.0.0.0 registered a loopback resource")
	}
	if _, ok = loopbackMCPResource(config.Config{Host: "", Port: "8080"}); ok {
		t.Fatal("empty HOST registered a loopback resource while the listen address is every interface")
	}
	if net.JoinHostPort("", "8080") != ":8080" {
		t.Fatal("empty HOST does not bind every interface")
	}
}

// TestRemoteSpoofedLoopbackHostCannotMint binds the way an empty HOST does,
// on every interface, and refuses a non-loopback peer that sends
// Host: 127.0.0.1:$PORT.
func TestRemoteSpoofedLoopbackHostCannotMint(t *testing.T) {
	if _, ok := loopbackMCPResource(config.Config{Host: "", Port: "8080"}); ok {
		t.Fatal("empty HOST registered a loopback audience")
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("", "0"))
	if err != nil {
		t.Fatal(err)
	}
	bound, ok := ln.Addr().(*net.TCPAddr)
	if !ok || bound.IP == nil || !bound.IP.IsUnspecified() {
		t.Fatalf("listen %v, want every interface", ln.Addr())
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	entry := &handlers.AgentEntry{ListenHost: ""}
	r.POST("/v1/agent-tokens", entry.CreateAgentToken)
	srv := &http.Server{Handler: r}
	remote := &net.TCPAddr{IP: net.ParseIP("203.0.113.50"), Port: 1000}
	go func() { _ = srv.Serve(addrListener{Listener: ln, remote: remote}) }()
	t.Cleanup(func() { _ = srv.Close() })

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+port+"/v1/agent-tokens", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "127.0.0.1:" + port
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("mint status %d body %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "access_token") || !strings.Contains(string(body), "loopback host is not accepted from this client") {
		t.Fatalf("mint body %s", body)
	}
}

type addrListener struct {
	net.Listener
	remote net.Addr
}

func (l addrListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return addrConn{Conn: c, remote: l.remote}, nil
}

type addrConn struct {
	net.Conn
	remote net.Addr
}

func (c addrConn) RemoteAddr() net.Addr { return c.remote }
