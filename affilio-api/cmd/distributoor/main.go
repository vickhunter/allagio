// distributoor is the legacy maker CLI name for Allagio: register a
// product, get it distribution-ready, publish an offer, track earnings.
// The same verbs are available to agents through cmd/distributoor-mcp.
//
//	distributoor init    --email you@example.com --product "Name" --website https://...
//	distributoor assess  [--ref-param via]
//	distributoor offer   create --type percentage --value 30
//	distributoor offer   submit
//	distributoor snippet
//	distributoor status
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"affilio-api/internal/dcp"
)

const configFile = ".distributoor.json"

type cliConfig struct {
	APIURL    string `json:"api_url"`
	MakerID   string `json:"maker_id"`
	APIKey    string `json:"api_key"`
	ProductID string `json:"product_id"`
	OfferID   string `json:"offer_id"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "assess":
		err = cmdAssess(os.Args[2:])
	case "offer":
		err = cmdOffer(os.Args[2:])
	case "listings":
		err = cmdListings()
	case "snippet":
		err = cmdSnippet()
	case "status":
		err = cmdStatus()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: distributoor <command>

  init     register maker + product (writes .distributoor.json)
  assess   run the readiness assessment and print the checklist
  offer    create | submit an offer
  listings list offers with review_reason (null when listed)
  snippet  print the attribution snippet + provider checkout wiring
  status   offers, earnings, and readiness at a glance`)
}

func loadConfig() (*cliConfig, error) {
	raw, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("no %s found; run 'distributoor init' first", configFile)
	}
	var cfg cliConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *cliConfig) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, append(raw, '\n'), 0o600)
}

func clientFor(cfg *cliConfig) *dcp.Client {
	return dcp.NewClient(cfg.APIURL, cfg.APIKey)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	apiURL := fs.String("api-url", "http://localhost:8080", "Allagio API base URL")
	email := fs.String("email", "", "maker email (required unless already initialized)")
	name := fs.String("name", "", "maker display name")
	product := fs.String("product", "", "product name (required)")
	website := fs.String("website", "", "product landing page URL (required)")
	docs := fs.String("docs", "", "docs URL")
	pricing := fs.String("pricing", "", "pricing page URL")
	productType := fs.String("type", "saas", "product type: saas|game|shopify_app|dev_tool|other")
	provider := fs.String("provider", "", "payment provider: stripe|lemonsqueezy|polar|none")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *product == "" || *website == "" {
		return fmt.Errorf("--product and --website are required")
	}
	cfg, _ := loadConfig()
	if cfg == nil {
		cfg = &cliConfig{}
	}
	cfg.APIURL = *apiURL
	if cfg.APIKey == "" {
		if *email == "" {
			return fmt.Errorf("--email is required to register a maker")
		}
		result, err := clientFor(cfg).Do(http.MethodPost, "/v1/makers/register", map[string]interface{}{
			"email": *email, "name": *name,
		})
		if err != nil {
			return err
		}
		cfg.MakerID, _ = result["maker_id"].(string)
		cfg.APIKey, _ = result["api_key"].(string)
		fmt.Printf("registered maker %s\n", cfg.MakerID)
	}
	payload := map[string]interface{}{
		"name": *product, "website_url": *website, "product_type": *productType,
	}
	if *docs != "" {
		payload["docs_url"] = *docs
	}
	if *pricing != "" {
		payload["pricing_url"] = *pricing
	}
	if *provider != "" {
		payload["payment_provider"] = *provider
	}
	result, err := clientFor(cfg).Do(http.MethodPost, "/v1/maker/products", payload)
	if err != nil {
		return err
	}
	cfg.ProductID, _ = result["id"].(string)
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("registered product %s\nnext: distributoor assess\n", cfg.ProductID)
	return nil
}

func cmdAssess(args []string) error {
	fs := flag.NewFlagSet("assess", flag.ExitOnError)
	refParam := fs.String("ref-param", "", "attribution query parameter the product preserves (default via)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	payload := map[string]interface{}{}
	if *refParam != "" {
		payload["ref_param"] = *refParam
	}
	result, err := clientFor(cfg).Do(http.MethodPost, "/v1/maker/products/"+cfg.ProductID+"/assess", payload)
	if err != nil {
		return err
	}
	fmt.Printf("readiness: %v (score %v)\n\n", result["readiness_status"], result["readiness_score"])
	checks, _ := result["checks"].([]interface{})
	for _, item := range checks {
		check, _ := item.(map[string]interface{})
		gate := ""
		if isGate, _ := check["gate"].(bool); isGate {
			gate = " [gate]"
		}
		fmt.Printf("%-6s %s%s — %v\n", fmt.Sprintf("[%v]", check["status"]), check["key"], gate, check["evidence"])
		if remediation, _ := check["remediation"].(string); remediation != "" {
			fmt.Printf("       fix: %s\n", remediation)
		}
	}
	if summary, _ := result["ai_summary"].(string); summary != "" {
		fmt.Printf("\nreview summary: %s\n", summary)
	}
	return nil
}

func cmdOffer(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: distributoor offer create|submit")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("offer create", flag.ExitOnError)
		commissionType := fs.String("type", "percentage", "commission type: percentage|flat|recurring")
		value := fs.Float64("value", 0, "commission value (required)")
		cookie := fs.Int("cookie-days", 30, "attribution window in days")
		refund := fs.Int("refund-days", 30, "refund hold window in days")
		terms := fs.String("terms", "", "offer terms")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *value <= 0 {
			return fmt.Errorf("--value is required and must be positive")
		}
		payload := map[string]interface{}{
			"product_id":           cfg.ProductID,
			"commission_type":      *commissionType,
			"commission_value":     *value,
			"cookie_duration_days": *cookie,
			"refund_window_days":   *refund,
		}
		if *terms != "" {
			payload["terms"] = *terms
		}
		result, err := clientFor(cfg).Do(http.MethodPost, "/v1/maker/offers", payload)
		if err != nil {
			return err
		}
		cfg.OfferID, _ = result["id"].(string)
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("created offer %s (draft)\nnext: distributoor offer submit\n", cfg.OfferID)
		return nil
	case "submit":
		if cfg.OfferID == "" {
			return fmt.Errorf("no offer in %s; run 'distributoor offer create' first", configFile)
		}
		result, err := clientFor(cfg).Do(http.MethodPost, "/v1/maker/offers/"+cfg.OfferID+"/submit", nil)
		if err != nil {
			return err
		}
		if result["status"] == "listed" {
			fmt.Printf("offer %s is listed\n", cfg.OfferID)
			return nil
		}
		fmt.Printf("offer %s is in review (%v): %v\n", cfg.OfferID, result["review_reason"], result["message"])
		return nil
	default:
		return fmt.Errorf("usage: distributoor offer create|submit")
	}
}

func cmdListings() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	result, err := clientFor(cfg).Do(http.MethodGet, "/v1/listings", nil)
	if err != nil {
		return err
	}
	items, _ := result["listings"].([]interface{})
	if len(items) == 0 {
		fmt.Println("no listings yet — run 'distributoor offer create'")
		return nil
	}
	for _, item := range items {
		listing, _ := item.(map[string]interface{})
		fmt.Printf("listing %v [%v] review_reason=%v message=%v\n", listing["id"], listing["status"], listing["review_reason"], listing["message"])
	}
	return nil
}

func cmdSnippet() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	product, err := clientFor(cfg).Do(http.MethodGet, "/v1/maker/products/"+cfg.ProductID, nil)
	if err != nil {
		return err
	}
	refParam := "via"
	if cfg.OfferID != "" {
		if offer, err := clientFor(cfg).Do(http.MethodGet, "/v1/maker/offers/"+cfg.OfferID, nil); err == nil {
			if rp, _ := offer["ref_param"].(string); rp != "" {
				refParam = rp
			}
		}
	}
	fmt.Printf(`<!-- Allagio attribution for %v: persist the click id, pass it into checkout -->
<script>
(function () {
  var ref = new URLSearchParams(location.search).get(%q);
  if (ref) {
    try { localStorage.setItem("dtr", ref); } catch (e) {}
    document.cookie = "dtr=" + ref + ";max-age=" + 90*24*3600 + ";path=/;SameSite=Lax";
  }
})();
// Read it back at checkout time:
//   var dtr = localStorage.getItem("dtr") || (document.cookie.match(/(?:^|; )dtr=([^;]*)/)||[])[1];
</script>

Wire it into your checkout:
  Stripe:        client_reference_id: dtr   (or metadata: {dtr: dtr})
  Lemon Squeezy: checkout[custom][dtr] = dtr
  Polar:         metadata: {dtr: dtr}

Then point your provider's webhooks at:  <api>/v1/webhooks/<provider>
`, product["name"], refParam)
	return nil
}

func cmdStatus() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	product, err := clientFor(cfg).Do(http.MethodGet, "/v1/maker/products/"+cfg.ProductID, nil)
	if err != nil {
		return err
	}
	fmt.Printf("product: %v — readiness %v (score %v)\n", product["name"], product["readiness_status"], product["readiness_score"])
	earnings, err := clientFor(cfg).Do(http.MethodGet, "/v1/maker/earnings", nil)
	if err != nil {
		return err
	}
	offers, _ := earnings["offers"].([]interface{})
	if len(offers) == 0 {
		fmt.Println("no offers yet — run 'distributoor offer create'")
		return nil
	}
	for _, item := range offers {
		offer, _ := item.(map[string]interface{})
		fmt.Printf("offer %v [%v]: %v clicks, %v sales (%v refunds), revenue %v, commission owed %v %v\n",
			offer["offer_id"], offer["status"], offer["clicks"], offer["sales"], offer["refunds"], offer["revenue"], offer["commission"], offer["currency"])
	}
	return nil
}
