package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxRequestBody caps request bodies under the group's routes. A body over the
// cap still binds when JSON decoding stops early; handlers should treat a
// capped body as malformed.
func MaxRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPatch || c.Request.Method == http.MethodPut {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
