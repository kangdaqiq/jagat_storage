package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"storage_server/internal/model"
	"storage_server/internal/storage"
	"storage_server/internal/utils"
)

const CurrentUserKey = "currentUser"

// UserAuth memvalidasi API Key pengguna lewat header X-API-Key, Authorization Bearer, atau Query Param ?api_key=
func UserAuth(userStore *storage.UserStore, isRequired bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientKey := extractAPIKey(c)

		if clientKey == "" {
			if isRequired {
				utils.SendError(c, http.StatusUnauthorized, "Unauthorized: API Key diperlukan untuk mengakses endpoint ini", gin.H{
					"header_key": "X-API-Key atau Authorization: Bearer <token>",
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		user, err := userStore.GetUserByAPIKey(clientKey)
		if err != nil {
			utils.SendError(c, http.StatusUnauthorized, "Unauthorized: API Key tidak valid atau telah dicabut", gin.H{
				"provided_key": clientKey,
			})
			c.Abort()
			return
		}

		// Simpan data user ke dalam context request
		c.Set(CurrentUserKey, user)
		c.Next()
	}
}

// AdminOnly membatasi akses endpoint khusus untuk pengguna dengan role admin
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		if user == nil || user.Role != "admin" {
			utils.SendError(c, http.StatusForbidden, "Forbidden: Aksi ini memerlukan hak akses Administrator", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetCurrentUser mengambil model User dari context Gin
func GetCurrentUser(c *gin.Context) *model.User {
	val, exists := c.Get(CurrentUserKey)
	if !exists {
		return nil
	}
	user, ok := val.(*model.User)
	if !ok {
		return nil
	}
	return user
}

// extractAPIKey mengekstrak API Key dari request
func extractAPIKey(c *gin.Context) string {
	clientKey := c.GetHeader("X-API-Key")
	if clientKey == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if clientKey == "" {
		clientKey = c.Query("api_key")
	}
	return strings.TrimSpace(clientKey)
}
