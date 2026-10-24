package orp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Canonicalize is RFC 8785 (JCS) for the strings, booleans, integers, arrays
// and objects ORP hashes. Whole numbers are written without a fraction.
func Canonicalize(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return canonicalizeString(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if !math.IsInf(v, 0) && !math.IsNaN(v) && v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
			return strconv.FormatInt(int64(v), 10)
		}
		raw, _ := json.Marshal(v)
		return string(raw)
	case json.Number:
		return v.String()
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = Canonicalize(item)
		}
		out := "["
		for i, part := range parts {
			if i > 0 {
				out += ","
			}
			out += part
		}
		return out + "]"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := "{"
		first := true
		for _, key := range keys {
			item, ok := v[key]
			if !ok {
				continue
			}
			if !first {
				out += ","
			}
			first = false
			out += canonicalizeString(key) + ":" + Canonicalize(item)
		}
		return out + "}"
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		var decoded any
		if json.Unmarshal(raw, &decoded) != nil {
			return "null"
		}
		return Canonicalize(decoded)
	}
}

// canonicalizeString is the RFC 8785 string form. It escapes quotes, reverse
// solidi, and the C0 controls. It leaves &, <, and > as characters.
func canonicalizeString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// TermsHash is sha256- plus the unpadded base64url SHA-256 of the JCS form.
func TermsHash(terms map[string]any) string {
	return ContentHash(terms)
}

// ContentHash is the JCS hash of one JSON value. The same bytes are the
// idempotency key for a stored envelope.
func ContentHash(value any) string {
	sum := sha256.Sum256([]byte(Canonicalize(value)))
	return "sha256-" + base64.RawURLEncoding.EncodeToString(sum[:])
}
