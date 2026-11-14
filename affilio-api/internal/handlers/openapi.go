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
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(body))
	}
}
