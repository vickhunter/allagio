package orp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeOrdersKeys(t *testing.T) {
	got := Canonicalize(map[string]any{"b": 1, "a": "x"})
	if got != `{"a":"x","b":1}` {
		t.Fatalf("canonical form %s", got)
	}
	if Canonicalize("a&b<c>") != `"a&b<c>"` {
		t.Fatalf("string escapes %s", Canonicalize("a&b<c>"))
	}
}

func TestOfficialExamplesMatchSchemas(t *testing.T) {
	root := schemaRootForTest(t)
	examples := filepath.Join(filepath.Dir(root), "examples")
	checkFile(t, filepath.Join(root, "discovery.schema.json"), filepath.Join(examples, "discovery.json"))
	for _, name := range []string{"suggest-request", "suggest-response", "join-request", "join-response"} {
		checkFile(t, filepath.Join(root, "http", name+".schema.json"), filepath.Join(examples, "http", name+".json"))
	}
	messages, err := filepath.Glob(filepath.Join(examples, "messages", "*.json"))
	if err != nil || len(messages) < 10 {
		t.Fatalf("message examples: %v %d", err, len(messages))
	}
	messageSchema := filepath.Join(root, "message.schema.json")
	for _, file := range messages {
		checkFile(t, messageSchema, file)
	}
	var wrapped struct {
		Instance any `json:"instance"`
	}
	raw, err := os.ReadFile(filepath.Join(examples, "invalid", "envelope-68-shape.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAgainst(messageSchema, wrapped.Instance); err == nil {
		t.Fatal("envelope #68 validated")
	}
}

func TestDiscoveryDocumentValidates(t *testing.T) {
	doc := DiscoveryDocument("http://127.0.0.1:8080", "127.0.0.1:8080", "stub")
	if _, ok := doc["conformance"]; ok {
		t.Fatal("discovery claims a conformance level")
	}
	if doc["name"] != TestingName {
		t.Fatalf("name %v", doc["name"])
	}
	if doc["docs"] != "http://127.0.0.1:8080"+DocsPath {
		t.Fatalf("docs %v", doc["docs"])
	}
	desc, _ := doc["description"].(string)
	if !strings.Contains(desc, ProofPageTemplate) || !strings.Contains(desc, DocsPath) || !strings.Contains(desc, TestingName) {
		t.Fatalf("description %s", desc)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var wire any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAgainst(filepath.Join(filepath.Dir(schemaRootForTest(t)), "0.4", "discovery.schema.json"), wire); err != nil {
		t.Fatal(err)
	}
}

func checkFile(t *testing.T, schemaFile, instanceFile string) {
	t.Helper()
	raw, err := os.ReadFile(instanceFile)
	if err != nil {
		t.Fatal(err)
	}
	var instance any
	if err := json.Unmarshal(raw, &instance); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAgainst(schemaFile, instance); err != nil {
		t.Fatalf("%s: %v", instanceFile, err)
	}
}

func schemaRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "docs", "protocol", "schema", "0.3", "defs.schema.json")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Dir(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("schema directory not found")
		}
		dir = parent
	}
}
