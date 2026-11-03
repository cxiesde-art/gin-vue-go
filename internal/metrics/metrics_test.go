package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRegistryCountersAndEndpoint(t *testing.T) {
	registry := New()
	registry.ObserveHTTP(http.StatusOK, time.Millisecond)
	registry.ObserveHTTP(http.StatusInternalServerError, time.Millisecond)
	registry.ObserveGRPC("OK")
	registry.ObserveGRPC("Unavailable")
	registry.ObserveAudit()
	if snapshot := registry.Snapshot(); snapshot.HTTPRequests != 2 || snapshot.GRPCRequests != 2 || snapshot.AuditEvents != 1 || snapshot.RequestErrors != 2 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", registry.Handler)
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("metrics endpoint failed: %d %s", response.Code, response.Body.String())
	}
}
