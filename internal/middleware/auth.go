package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/gin-gonic/gin"
)

const ClaimsKey = "blogClaims"

type TokenValidator interface {
	Validate(context.Context, string) (uint, string, error)
}

type localTokenValidator struct {
	secret string
}

func (v localTokenValidator) Validate(_ context.Context, token string) (uint, string, error) {
	claims, err := auth.ParseToken(v.secret, token)
	if err != nil {
		return 0, "", err
	}
	return claims.UserID, claims.Role, nil
}

func RequireAuth(secret string) gin.HandlerFunc {
	return RequireTokenValidator(localTokenValidator{secret: secret})
}

func RequireTokenValidator(validator TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		userID, role, err := validator.Validate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(ClaimsKey, auth.Claims{UserID: userID, Role: role})
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
