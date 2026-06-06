package middleware

import (
	"crypto/subtle"

	"pixez-sync/response"

	"github.com/gin-gonic/gin"
)

// BasicAuth creates a gin middleware for HTTP Basic Authentication.
// It uses constant-time comparison to prevent timing attacks.
func BasicAuth(expectedUser, expectedPass string) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(username), []byte(expectedUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(expectedPass)) != 1 {

			c.Header("WWW-Authenticate", `Basic realm="PixEz Sync"`)
			response.RespondUnauthorized(c, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
