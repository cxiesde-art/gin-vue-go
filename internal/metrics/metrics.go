package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type Registry struct {
	httpRequests  atomic.Uint64
	grpcRequests  atomic.Uint64
	auditEvents   atomic.Uint64
	requestErrors atomic.Uint64
}

type Snapshot struct {
	HTTPRequests  uint64
	GRPCRequests  uint64
	AuditEvents   uint64
	RequestErrors uint64
}

func New() *Registry { return &Registry{} }

func (r *Registry) ObserveHTTP(status int, _ time.Duration) {
	r.httpRequests.Add(1)
	if status >= http.StatusInternalServerError {
		r.requestErrors.Add(1)
	}
}

func (r *Registry) ObserveGRPC(code string) {
	r.grpcRequests.Add(1)
	if code != "OK" {
		r.requestErrors.Add(1)
	}
}

func (r *Registry) ObserveAudit() { r.auditEvents.Add(1) }

func (r *Registry) Snapshot() Snapshot {
	return Snapshot{
		HTTPRequests:  r.httpRequests.Load(),
		GRPCRequests:  r.grpcRequests.Load(),
		AuditEvents:   r.auditEvents.Load(),
		RequestErrors: r.requestErrors.Load(),
	}
}

func (r *Registry) Handler(c *gin.Context) {
	snapshot := r.Snapshot()
	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(http.StatusOK, "# TYPE blog_http_requests_total counter\nblog_http_requests_total %d\n# TYPE blog_grpc_requests_total counter\nblog_grpc_requests_total %d\n# TYPE blog_audit_events_total counter\nblog_audit_events_total %d\n# TYPE blog_request_errors_total counter\nblog_request_errors_total %d\n", snapshot.HTTPRequests, snapshot.GRPCRequests, snapshot.AuditEvents, snapshot.RequestErrors)
}

func (r *Registry) String() string {
	snapshot := r.Snapshot()
	return fmt.Sprintf("http=%d grpc=%d audit=%d errors=%d", snapshot.HTTPRequests, snapshot.GRPCRequests, snapshot.AuditEvents, snapshot.RequestErrors)
}
