package analyticsrpc

import (
	"context"
	"log/slog"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcmiddleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "blog.analytics.v1.AnalyticsService"

type RecordViewRequest struct {
	PostID    uint   `json:"postId"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

type RecordViewResponse struct {
	Accepted bool `json:"accepted"`
}

type StatsRequest struct {
	PostID uint `json:"postId"`
}

type StatsResponse struct {
	PostID uint  `json:"postId"`
	Views  int64 `json:"views"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type AnalyticsServiceServer interface {
	RecordView(context.Context, *RecordViewRequest) (*RecordViewResponse, error)
	Stats(context.Context, *StatsRequest) (*StatsResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	repo *repository.Repository
	now  func() time.Time
}

func NewServer(repo *repository.Repository) *Server { return &Server{repo: repo, now: time.Now().UTC} }

func (s *Server) RecordView(ctx context.Context, request *RecordViewRequest) (*RecordViewResponse, error) {
	if request.PostID == 0 {
		return nil, status.Error(codes.InvalidArgument, "post id is required")
	}
	event := &model.ViewEvent{PostID: request.PostID, RequestID: request.RequestID, TraceID: request.TraceID, ViewedAt: s.now()}
	if err := s.repo.CreateViewEvent(ctx, event); err != nil {
		return nil, status.Error(codes.Internal, "record view failed")
	}
	return &RecordViewResponse{Accepted: true}, nil
}

func (s *Server) Stats(ctx context.Context, request *StatsRequest) (*StatsResponse, error) {
	if request.PostID == 0 {
		return nil, status.Error(codes.InvalidArgument, "post id is required")
	}
	views, err := s.repo.CountPostViews(ctx, request.PostID)
	if err != nil {
		return nil, status.Error(codes.Internal, "read view stats failed")
	}
	return &StatsResponse{PostID: request.PostID, Views: views}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-analytics"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation AnalyticsServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*AnalyticsServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "RecordView", Handler: recordViewHandler},
			{MethodName: "Stats", Handler: statsHandler},
			{MethodName: "Health", Handler: healthHandler},
		},
	}, implementation)
}

func NewGRPCServer(logger *slog.Logger) *grpc.Server {
	return grpc.NewServer(grpc.ForceServerCodec(grpcjson.Codec{}), grpc.UnaryInterceptor(grpcmiddleware.UnaryServerLogging(logger)))
}

func recordViewHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(RecordViewRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AnalyticsServiceServer).RecordView(ctx, request.(*RecordViewRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/RecordView", call, interceptor)
}

func statsHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(StatsRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AnalyticsServiceServer).Stats(ctx, request.(*StatsRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Stats", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(AnalyticsServiceServer).Health(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Health", call, interceptor)
}

func invoke(ctx context.Context, request, server any, method string, call grpc.UnaryHandler, interceptor grpc.UnaryServerInterceptor) (any, error) {
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: method}, call)
}
