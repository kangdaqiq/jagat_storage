package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"storage_server/internal/utils"
)

// APIKeyAuth memvalidasi API Key lewat header X-API-Key atau Authorization Bearer
func APIKeyAuth(expectedKey string, isRequired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isRequired || expectedKey == "" {
			c.Next()
			return
		}

		clientKey := c.GetHeader("X-API-Key")
		if clientKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				clientKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Juga izinkan query param ?api_key=xxx jika dipanggil dari URL langsung (berguna untuk link preview tertentu jika diperlukan)
		if clientKey == "" {
			clientKey = c.Query("api_key")
		}

		if clientKey == "" || clientKey != expectedKey {
			utils.SendError(c, http.StatusUnauthorized, "Unauthorized: API Key tidak valid atau tidak disertakan", gin.H{
				"required_header": "X-API-Key atau Authorization: Bearer <key>",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
