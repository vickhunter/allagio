package handlers

import (
	"context"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/dcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHostedMCPServerInfoName(t *testing.T) {
	ctx := context.Background()
	server := newHostedMCPServer(&database.AgentGrant{}, "http://127.0.0.1:1", "https://offer-rails.example", "token")
	client := mcp.NewClient(&mcp.Implementation{Name: "review", Version: "0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := clientSession.InitializeResult().ServerInfo
	if info == nil || info.Name != dcp.ServerName || info.Title != dcp.ServerTitle || info.Version != "0.3.0" {
		t.Fatalf("serverInfo %+v", info)
	}
	if err := clientSession.Close(); err != nil {
		t.Fatal(err)
	}
	if err := serverSession.Wait(); err != nil {
		t.Fatal(err)
	}
}
