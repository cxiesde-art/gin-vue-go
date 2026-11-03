package middleware

import (
	"net/http"
	"strings"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/gin-gonic/gin"
)

const ClaimsKey = "blogClaims"

func RequireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := auth.ParseToken(secret, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(ClaimsKey)
		claims, ok := value.(auth.Claims)
		if !exists || !ok || claims.Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}
