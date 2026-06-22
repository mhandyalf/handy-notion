package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const UserIDKey = "user_id"

type validationResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"user_id"`
}

func HandyAuth(authServiceURL string, client *http.Client) gin.HandlerFunc {
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}
	validateURL := strings.TrimRight(authServiceURL, "/") + "/api/auth/validate"
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, validateURL, nil)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authentication configuration error"})
			return
		}
		req.Header.Set("Authorization", token)
		response, err := client.Do(req)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authentication service unavailable"})
			return
		}
		defer response.Body.Close()
		var result validationResponse
		if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&result) != nil || !result.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
			return
		}
		userID, err := uuid.Parse(result.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return
		}
		c.Set(UserIDKey, userID)
		c.Next()
	}
}
