package main

import (
	"testing"

	"affilio-api/internal/config"
)

func TestLoopbackMCPResource(t *testing.T) {
	got, ok := loopbackMCPResource(config.Config{Host: "127.0.0.1", Port: "8097"})
	if !ok || got != "http://127.0.0.1:8097/mcp" {
		t.Fatalf("loopback resource %q ok=%v", got, ok)
	}
	if _, ok = loopbackMCPResource(config.Config{Host: "127.0.0.1", Port: "8097", GinMode: "release"}); ok {
		t.Fatal("release mode registered a loopback resource")
	}
	if _, ok = loopbackMCPResource(config.Config{Host: "0.0.0.0", Port: "8080"}); ok {
		t.Fatal("a public bind registered a loopback resource")
	}
	got, ok = loopbackMCPResource(config.Config{Host: "", Port: "8080"})
	if !ok || got != "http://127.0.0.1:8080/mcp" {
		t.Fatalf("empty host %q ok=%v", got, ok)
	}
}
