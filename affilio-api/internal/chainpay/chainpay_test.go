package chainpay

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testTx    = "0x1111111111111111111111111111111111111111111111111111111111111111"
	testPayee = "0x00000000000000000000000000000000000000aa"
)

// fakeChain answers the three reads Verify makes.
type fakeChain struct {
	receipt map[string]any
	head    int64
	stamp   int64
	fail    bool
}

func (f *fakeChain) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.fail {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "eth_getTransactionReceipt":
			if f.receipt != nil {
				result = f.receipt
			}
		case "eth_blockNumber":
			result = "0x" + big.NewInt(f.head).Text(16)
		case "eth_getBlockByNumber":
			result = map[string]any{"timestamp": "0x" + big.NewInt(f.stamp).Text(16)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func transferReceipt(token, to string, units int64, status string) map[string]any {
	return map[string]any{
		"status":      status,
		"blockNumber": "0x64",
		"logs": []any{map[string]any{
			"address": token,
			"topics": []any{
				transferTopic,
				"0x000000000000000000000000" + strings.Repeat("b", 40),
				"0x000000000000000000000000" + strings.TrimPrefix(to, "0x"),
			},
			"data": "0x" + big.NewInt(units).Text(16),
		}},
	}
}

func TestParseRef(t *testing.T) {
	cases := []struct {
		rail, ref, network string
		ok                 bool
	}{
		{"usdc.base", testTx, "eip155:8453", true},
		{"usdc.base-sepolia", strings.ToUpper(testTx[:2]) + testTx[2:], "", false},
		{"usdc.base-sepolia", testTx, "eip155:84532", true},
		{"usdc.base", "0x1234", "", false},
		{"x402", "eip155:84532:" + testTx, "eip155:84532", true},
		{"x402", "eip155:1:" + testTx, "", false},
		{"x402", testTx, "", false},
		{"stripe", testTx, "", false},
	}
	for _, tc := range cases {
		network, tx, ok := ParseRef(tc.rail, tc.ref)
		if ok != tc.ok || network != tc.network || (ok && tx != testTx) {
			t.Fatalf("ParseRef(%q, %q) = %q %q %v", tc.rail, tc.ref, network, tx, ok)
		}
	}
	if !IsChainRail("x402") || !IsChainRail("usdc.base") || IsChainRail("manual") {
		t.Fatal("IsChainRail")
	}
	if got := RailNetworks("x402"); len(got) != 2 {
		t.Fatalf("x402 networks %v", got)
	}
}

func TestUnits(t *testing.T) {
	units, ok := Units("eip155:8453", 1500, "USD")
	if !ok || units.String() != "15000000" {
		t.Fatalf("units %v %v", units, ok)
	}
	if _, ok := Units("eip155:8453", 1500, "EUR"); ok {
		t.Fatal("EUR must not map to USDC")
	}
}

func TestVerify(t *testing.T) {
	token := Networks["eip155:84532"].Token
	min, _ := Units("eip155:84532", 1500, "USD")
	created := time.Unix(1_000, 0)
	check := Check{Network: "eip155:84532", Tx: testTx, PayTo: testPayee, MinUnits: min, NotBefore: created}
	cases := []struct {
		name   string
		chain  fakeChain
		status Status
		reason string
	}{
		{"verified", fakeChain{receipt: transferReceipt(token, testPayee, 15_000_000, "0x1"), head: 0x64 + 4, stamp: 2_000}, Verified, ""},
		{"overpay", fakeChain{receipt: transferReceipt(token, testPayee, 20_000_000, "0x1"), head: 0x64 + 4, stamp: 2_000}, Verified, ""},
		{"not found", fakeChain{head: 200}, Pending, "not_found"},
		{"confirming", fakeChain{receipt: transferReceipt(token, testPayee, 15_000_000, "0x1"), head: 0x64 + 1, stamp: 2_000}, Pending, "confirming"},
		{"reverted", fakeChain{receipt: transferReceipt(token, testPayee, 15_000_000, "0x0"), head: 200, stamp: 2_000}, Mismatch, "reverted"},
		{"short", fakeChain{receipt: transferReceipt(token, testPayee, 14_999_999, "0x1"), head: 200, stamp: 2_000}, Mismatch, "no_matching_transfer"},
		{"wrong payee", fakeChain{receipt: transferReceipt(token, "0x"+strings.Repeat("c", 40), 15_000_000, "0x1"), head: 200, stamp: 2_000}, Mismatch, "no_matching_transfer"},
		{"wrong token", fakeChain{receipt: transferReceipt(Networks["eip155:8453"].Token, testPayee, 15_000_000, "0x1"), head: 200, stamp: 2_000}, Mismatch, "no_matching_transfer"},
		{"old transfer", fakeChain{receipt: transferReceipt(token, testPayee, 15_000_000, "0x1"), head: 200, stamp: 500}, Mismatch, "before_payout"},
		{"rpc down", fakeChain{fail: true}, Unavailable, "rpc_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain := tc.chain
			srv := chain.server(t)
			v := &Verifier{RPC: map[string]string{"eip155:84532": srv.URL}, Confirmations: 3}
			got := v.Verify(context.Background(), check)
			if got.Status != tc.status || got.Reason != tc.reason {
				t.Fatalf("got %+v want %s %s", got, tc.status, tc.reason)
			}
		})
	}
	if got := (&Verifier{}).Verify(context.Background(), check); got.Status != Unavailable || got.Reason != "no_rpc" {
		t.Fatalf("no rpc %+v", got)
	}
}
