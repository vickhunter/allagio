// Package chainpay verifies on-chain USDC payouts for the
// dev.allagio.settle-chain profile. It only reads the chain: no keys, no
// signing, no gas. A payout names a chain rail, the payer reports the
// transaction hash as ref, and Verify checks that the transaction moved at
// least the payout amount of the network's USDC to the payee's wallet.
package chainpay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Profile is the extension name a host advertises in capabilities.ext.
const Profile = "dev.allagio.settle-chain"

// Network is one EVM network the profile can verify.
type Network struct {
	ID       string
	Name     string
	Token    string
	Decimals int
}

// Networks are keyed by CAIP-2 id. Token is the network's native USDC
// contract, lowercase.
var Networks = map[string]Network{
	"eip155:8453":  {ID: "eip155:8453", Name: "Base", Token: "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913", Decimals: 6},
	"eip155:84532": {ID: "eip155:84532", Name: "Base Sepolia", Token: "0x036cbd53842c5426634e7929541ec2318f3dcf7e", Decimals: 6},
}

// NetworkIDs is the stable order networks are listed in.
var NetworkIDs = []string{"eip155:8453", "eip155:84532"}

var railNetworks = map[string]string{
	"usdc.base":         "eip155:8453",
	"usdc.base-sepolia": "eip155:84532",
}

// RailX402 is the x402 rail. Its ref carries the network: <caip2>:<tx hash>.
const RailX402 = "x402"

var (
	txPattern      = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
	addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	transferTopic  = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

// IsChainRail reports whether the host verifies this rail on-chain.
func IsChainRail(rail string) bool {
	_, ok := railNetworks[rail]
	return ok || rail == RailX402
}

// RailNetworks lists the networks a payee wallet can serve for this rail:
// one for a usdc.* rail, every known network for x402.
func RailNetworks(rail string) []string {
	if id, ok := railNetworks[rail]; ok {
		return []string{id}
	}
	if rail == RailX402 {
		return append([]string{}, NetworkIDs...)
	}
	return nil
}

// ParseRef reads a chain rail ref. A usdc.* rail takes a bare transaction
// hash; x402 takes <caip2>:<hash>, the network and transaction fields of an
// x402 settlement response. The hash comes back lowercase.
func ParseRef(rail, ref string) (network, tx string, ok bool) {
	ref = strings.TrimSpace(ref)
	if id, known := railNetworks[rail]; known {
		if !txPattern.MatchString(ref) {
			return "", "", false
		}
		return id, strings.ToLower(ref), true
	}
	if rail != RailX402 {
		return "", "", false
	}
	cut := strings.LastIndex(ref, ":")
	if cut < 0 {
		return "", "", false
	}
	id, hash := ref[:cut], ref[cut+1:]
	if _, known := Networks[id]; !known || !txPattern.MatchString(hash) {
		return "", "", false
	}
	return id, strings.ToLower(hash), true
}

// ValidAddress reports whether s is a 20-byte hex EVM address.
func ValidAddress(s string) bool {
	return addressPattern.MatchString(s)
}

// Units converts a payout amount to token units. Only USD maps to USDC.
func Units(network string, amountMinor int64, currency string) (*big.Int, bool) {
	n, ok := Networks[network]
	if !ok || currency != "USD" || amountMinor <= 0 || n.Decimals < 2 {
		return nil, false
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Decimals-2)), nil)
	return new(big.Int).Mul(big.NewInt(amountMinor), scale), true
}

// Status is the result of one verification.
type Status string

const (
	// Verified: the transfer is final enough and matches the payout.
	Verified Status = "verified"
	// Pending: not found yet or still confirming. Check again later.
	Pending Status = "pending"
	// Mismatch: the transaction exists but cannot settle this payout.
	Mismatch Status = "mismatch"
	// Unavailable: no RPC for the network or the RPC failed.
	Unavailable Status = "unavailable"
)

// Result is a verification outcome. Reason is a short machine code.
type Result struct {
	Status Status
	Reason string
}

// Check is what one payout expects on-chain.
type Check struct {
	Network   string
	Tx        string
	PayTo     string
	MinUnits  *big.Int
	NotBefore time.Time
}

// Verifier reads chain state over JSON-RPC.
type Verifier struct {
	// RPC maps a CAIP-2 network id to a JSON-RPC URL.
	RPC map[string]string
	// Confirmations is the block depth a transfer needs. Below 1 means 1.
	Confirmations int64
	Client        *http.Client
}

// Verify checks one transaction against one payout. It never signs and
// never sends a transaction.
func (v *Verifier) Verify(ctx context.Context, check Check) Result {
	network, ok := Networks[check.Network]
	if !ok {
		return Result{Status: Mismatch, Reason: "unknown_network"}
	}
	if v == nil || v.RPC[check.Network] == "" {
		return Result{Status: Unavailable, Reason: "no_rpc"}
	}
	if check.MinUnits == nil || !ValidAddress(check.PayTo) {
		return Result{Status: Mismatch, Reason: "no_wallet"}
	}
	url := v.RPC[check.Network]
	var receipt *struct {
		Status      string `json:"status"`
		BlockNumber string `json:"blockNumber"`
		Logs        []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Data    string   `json:"data"`
		} `json:"logs"`
	}
	if err := v.call(ctx, url, "eth_getTransactionReceipt", []any{check.Tx}, &receipt); err != nil {
		return Result{Status: Unavailable, Reason: "rpc_error"}
	}
	if receipt == nil {
		return Result{Status: Pending, Reason: "not_found"}
	}
	if receipt.Status != "0x1" {
		return Result{Status: Mismatch, Reason: "reverted"}
	}
	payTo := strings.ToLower(strings.TrimPrefix(check.PayTo, "0x"))
	matched := false
	for _, entry := range receipt.Logs {
		if !strings.EqualFold(entry.Address, network.Token) || len(entry.Topics) != 3 || !strings.EqualFold(entry.Topics[0], transferTopic) {
			continue
		}
		to := strings.ToLower(entry.Topics[2])
		if len(to) < 40 || to[len(to)-40:] != payTo {
			continue
		}
		value, ok := hexBig(entry.Data)
		if ok && value.Cmp(check.MinUnits) >= 0 {
			matched = true
			break
		}
	}
	if !matched {
		return Result{Status: Mismatch, Reason: "no_matching_transfer"}
	}
	block, ok := hexInt(receipt.BlockNumber)
	if !ok {
		return Result{Status: Unavailable, Reason: "rpc_error"}
	}
	var headHex string
	if err := v.call(ctx, url, "eth_blockNumber", []any{}, &headHex); err != nil {
		return Result{Status: Unavailable, Reason: "rpc_error"}
	}
	head, ok := hexInt(headHex)
	if !ok {
		return Result{Status: Unavailable, Reason: "rpc_error"}
	}
	need := v.Confirmations
	if need < 1 {
		need = 1
	}
	if head-block+1 < need {
		return Result{Status: Pending, Reason: "confirming"}
	}
	var header *struct {
		Timestamp string `json:"timestamp"`
	}
	if err := v.call(ctx, url, "eth_getBlockByNumber", []any{receipt.BlockNumber, false}, &header); err != nil || header == nil {
		return Result{Status: Unavailable, Reason: "rpc_error"}
	}
	stamp, ok := hexInt(header.Timestamp)
	if !ok {
		return Result{Status: Unavailable, Reason: "rpc_error"}
	}
	if !check.NotBefore.IsZero() && time.Unix(stamp, 0).Before(check.NotBefore) {
		return Result{Status: Mismatch, Reason: "before_payout"}
	}
	return Result{Status: Verified}
}

func (v *Verifier) call(ctx context.Context, url, method string, params []any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rpc %s: http %d", method, resp.StatusCode)
	}
	var reply struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reply); err != nil {
		return err
	}
	if reply.Error != nil {
		return errors.New("rpc " + method + ": " + reply.Error.Message)
	}
	if len(reply.Result) == 0 {
		return errors.New("rpc " + method + ": empty result")
	}
	return json.Unmarshal(reply.Result, out)
}

func hexInt(s string) (int64, bool) {
	if !strings.HasPrefix(s, "0x") {
		return 0, false
	}
	n, err := strconv.ParseInt(s[2:], 16, 64)
	return n, err == nil
}

func hexBig(s string) (*big.Int, bool) {
	if !strings.HasPrefix(s, "0x") || len(s) < 3 {
		return nil, false
	}
	return new(big.Int).SetString(s[2:], 16)
}
