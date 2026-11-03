package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type tokenValidator struct{}

func (tokenValidator) Validate(context.Context, string) (uint, string, error) { return 7, "admin", nil }

func TestRequireTokenValidatorStoresRemoteClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireTokenValidator(tokenValidator{}), RequireRole("admin"))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer remote-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("remote validator claims were rejected: %d", response.Code)
	}
}
