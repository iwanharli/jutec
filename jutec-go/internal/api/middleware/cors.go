package middleware

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// LocalOrigin: dashboard lokal di port berapa pun (dev server bisa pindah ke 3001 dst).
// Aman karena backend hanya bind ke 127.0.0.1 secara default.
var LocalOrigin = regexp.MustCompile(`^https?://(localhost|127\.0\.0\.1)(:\d+)?$`)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); LocalOrigin.MatchString(origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Jutec-User")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, X-Content-SHA256")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
