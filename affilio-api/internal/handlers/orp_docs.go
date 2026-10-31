package handlers

import (
	"net/http"
	"strings"

	"affilio-api/internal/config"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// Spec is GET /orp/spec, the protocol text.
func (h *ORPHost) Spec(c *gin.Context) {
	h.publicFile(c, "SPEC.md", "text/markdown; charset=utf-8")
}

// Gaps is GET /orp/gaps, the ranked hosted pitfalls.
func (h *ORPHost) Gaps(c *gin.Context) {
	h.publicFile(c, "GAPS.md", "text/markdown; charset=utf-8")
}

// SchemaIndex is GET /orp/schemas.
func (h *ORPHost) SchemaIndex(c *gin.Context) {
	h.publicIndex(c, "schemas")
}

// ExampleIndex is GET /orp/examples.
func (h *ORPHost) ExampleIndex(c *gin.Context) {
	h.publicIndex(c, "examples")
}

// Schema is GET /orp/schemas/*name.
func (h *ORPHost) Schema(c *gin.Context) {
	h.publicTree(c, "schemas", c.Param("name"))
}

// Example is GET /orp/examples/*name.
func (h *ORPHost) Example(c *gin.Context) {
	h.publicTree(c, "examples", c.Param("name"))
}

func (h *ORPHost) publicTree(c *gin.Context, dir, name string) {
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		h.publicIndex(c, dir)
		return
	}
	h.publicFile(c, dir+"/"+name, "application/json; charset=utf-8")
}

func (h *ORPHost) publicIndex(c *gin.Context, dir string) {
	body, ok := orp.PublicIndex(dir)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	writePublic(c, body, "application/json; charset=utf-8")
}

func (h *ORPHost) publicFile(c *gin.Context, name, contentType string) {
	body, ok := orp.ReadPublic(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	if strings.HasSuffix(name, ".md") {
		base := ""
		if h.Entry != nil {
			base = h.Entry.BaseURL
		}
		body = []byte(config.ExpandPublicBase(string(body), base))
	}
	writePublic(c, body, contentType)
}

func writePublic(c *gin.Context, body []byte, contentType string) {
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, body)
}
