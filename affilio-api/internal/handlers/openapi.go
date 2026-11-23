package handlers

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.json
var openapiDocument []byte

// ServeOpenAPI is GET /openapi.json. It returns the OpenAPI 3.1 document for
// the public HTTP surface with {PUBLIC_BASE_URL} swapped for the configured
// origin (or the request origin on a bare dev box). Public doc CORS and a
// short cache match the other discovery documents.
func ServeOpenAPI(publicBase string) gin.HandlerFunc {
	return func(c *gin.Context) {
		base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
		if base == "" {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			base = scheme + "://" + c.Request.Host
		}
		body := strings.ReplaceAll(string(openapiDocument), "{PUBLIC_BASE_URL}", base)
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Cache-Control", "public, max-age=300")
		c.Header("X-Content-Type-Options", "nosniff")
		if c.Request.Method == http.MethodHead {
			writeMethodBody(c, http.StatusOK, "application/json; charset=utf-8", []byte(body))
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(body))
	}
}

// scalarPage renders the API reference UI. Scalar loads /openapi.json in the
// browser, so the page is a fixed document and needs no templating.
const scalarPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Allagio REST API Reference · ORP &amp; Platform</title>
<meta name="description" content="Interactive OpenAPI 3.1 REST API reference for Allagio and the Open Rails Protocol (ORP).">
<style>
  :root {
    --scalar-background-1: #f7f7ef;
    --scalar-background-2: #fdfdf7;
    --scalar-background-3: #ecefe2;
    --scalar-color-1: #232720;
    --scalar-color-2: #5c6257;
    --scalar-color-accent: #3f5c35;
    --scalar-border-color: #dde0d3;
  }
  body { margin: 0; background: #f7f7ef; }
  .allagio-docs-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 20px;
    background: #232720;
    color: #f7f7ef;
    font: 500 13px/1.4 ui-monospace, SFMono-Regular, Consolas, monospace;
  }
  .allagio-docs-bar a {
    color: #ced3c2;
    text-decoration: none;
    margin-left: 14px;
  }
  .allagio-docs-bar a:hover { color: #f7f7ef; text-decoration: underline; }
  .allagio-docs-brand {
    font-weight: 700;
    color: #f7f7ef !important;
    margin-left: 0 !important;
    letter-spacing: -0.02em;
  }
</style>
</head>
<body>
<header class="allagio-docs-bar">
  <div>
    <a class="allagio-docs-brand" href="/">ALLAGIO</a>
    <span>· REST API Reference</span>
  </div>
  <nav aria-label="API documentation links">
    <a href="/protocol/">ORP Protocol</a>
    <a href="/use-cases/">Deal Templates</a>
    <a href="/agents">Agents Hub</a>
    <a href="/reference">MCP Cheat-Sheet</a>
    <a href="/openapi.json">openapi.json</a>
  </nav>
</header>
<script
  id="api-reference"
  data-url="/openapi.json"
  data-configuration='{"theme":"default","layout":"modern","showDeveloperTools":"never","defaultHttpClient":{"targetKey":"shell","clientKey":"curl"},"hideClientButton":true,"metaData":{"title":"Allagio REST API Reference"}}'></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>
`

// ServeScalarDocs is GET /docs. It renders the Scalar API reference against
// the OpenAPI document this server serves at /openapi.json.
func ServeScalarDocs() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.Header("X-Content-Type-Options", "nosniff")
		if c.Request.Method == http.MethodHead {
			writeMethodBody(c, http.StatusOK, "text/html; charset=utf-8", []byte(scalarPage))
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(scalarPage))
	}
}
