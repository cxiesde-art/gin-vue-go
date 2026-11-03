package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterRejectsAfterLimitAndResets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewRateLimiter(2, 10*time.Millisecond).Handler())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("request %d was unexpectedly rejected: %d", attempt, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("third request was accepted: %d", response.Code)
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response did not include Retry-After")
	}
	time.Sleep(12 * time.Millisecond)
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("rate window did not reset: %d", response.Code)
	}
}

func TestRateLimiterIgnoresSpoofedForwardedAddressWithoutTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewRateLimiterWithTrustedProxies(1, time.Minute, nil).Handler())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, forwarded := range []string{"198.51.100.1", "198.51.100.2"} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent && forwarded == "198.51.100.1" {
			t.Fatalf("first request was unexpectedly rejected: %d", response.Code)
		}
		if response.Code != http.StatusTooManyRequests && forwarded == "198.51.100.2" {
			t.Fatalf("spoofed address bypassed rate limit: %d", response.Code)
		}
	}
}

func TestRateLimiterUsesForwardedAddressFromTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewRateLimiterWithTrustedProxies(1, time.Minute, []string{"10.0.0.0/8"}).Handler())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, forwarded := range []string{"198.51.100.1", "198.51.100.2"} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "10.0.0.5:1234"
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("trusted client %s was unexpectedly rejected: %d", forwarded, response.Code)
		}
	}
}
