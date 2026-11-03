package auditrpc

import (
	"context"
	"log/slog"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/service"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcmiddleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "blog.audit.v1.AuditService"

type RecordRequest struct {
	Audit  model.AuditEvent  `json:"audit"`
	Outbox model.OutboxEvent `json:"outbox"`
}

type RecordResponse struct {
	Recorded bool `json:"recorded"`
}

type ListRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type ListResponse struct {
	Items    []model.AuditEvent `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
	Pages    int                `json:"pages"`
}

type ListOutboxRequest struct {
	Limit int `json:"limit"`
}

type ListOutboxResponse struct {
	Items []model.OutboxEvent `json:"items"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type AuditServiceServer interface {
	Record(context.Context, *RecordRequest) (*RecordResponse, error)
	List(context.Context, *ListRequest) (*ListResponse, error)
	ListOutbox(context.Context, *ListOutboxRequest) (*ListOutboxResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	repo *repository.Repository
}

func NewServer(repo *repository.Repository) *Server { return &Server{repo: repo} }

func (s *Server) Record(ctx context.Context, request *RecordRequest) (*RecordResponse, error) {
	if request.Audit.Action == "" || request.Outbox.EventType == "" {
		return nil, status.Error(codes.InvalidArgument, "audit and outbox event types are required")
	}
	if err := s.repo.RecordAuditAndOutbox(ctx, &request.Audit, &request.Outbox); err != nil {
		return nil, status.Error(codes.Internal, "record audit event failed")
	}
	return &RecordResponse{Recorded: true}, nil
}

func (s *Server) List(ctx context.Context, request *ListRequest) (*ListResponse, error) {
	page, pageSize := normalizePagination(request.Page, request.PageSize)
	items, total, err := s.repo.ListAuditEvents(ctx, page, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "list audit events failed")
	}
	return &ListResponse{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: service.PageCount(total, pageSize)}, nil
}

func (s *Server) ListOutbox(ctx context.Context, request *ListOutboxRequest) (*ListOutboxResponse, error) {
	items, err := s.repo.ListPendingOutboxEvents(ctx, request.Limit)
	if err != nil {
		return nil, status.Error(codes.Internal, "list pending outbox events failed")
	}
	return &ListOutboxResponse{Items: items}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-audit"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation AuditServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*AuditServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Record", Handler: recordHandler},
			{MethodName: "List", Handler: listHandler},
			{MethodName: "ListOutbox", Handler: listOutboxHandler},
			{MethodName: "Health", Handler: healthHandler},
		},
	}, implementation)
}

func NewGRPCServer(logger *slog.Logger) *grpc.Server {
	return grpc.NewServer(
		grpc.ForceServerCodec(grpcjson.Codec{}),
		grpc.UnaryInterceptor(grpcmiddleware.UnaryServerLogging(logger)),
	)
}

func recordHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(RecordRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AuditServiceServer).Record(ctx, request.(*RecordRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Record", call, interceptor)
}

func listHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ListRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AuditServiceServer).List(ctx, request.(*ListRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/List", call, interceptor)
}

func listOutboxHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ListOutboxRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AuditServiceServer).ListOutbox(ctx, request.(*ListOutboxRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/ListOutbox", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AuditServiceServer).Health(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Health", call, interceptor)
}

func invoke(ctx context.Context, request, server any, method string, call grpc.UnaryHandler, interceptor grpc.UnaryServerInterceptor) (any, error) {
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: method}, call)
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return page, pageSize
}
