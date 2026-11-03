package searchrpc

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

const serviceName = "blog.search.v1.SearchService"

type SearchRequest struct {
	Query    string `json:"query"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type SearchResponse struct {
	Items    []model.Post `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int64        `json:"total"`
	Pages    int          `json:"pages"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type SearchServiceServer interface {
	Search(context.Context, *SearchRequest) (*SearchResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	service *service.BlogService
}

func NewServer(repo *repository.Repository) *Server {
	return &Server{service: service.NewBlogService(repo)}
}

func (s *Server) Search(ctx context.Context, request *SearchRequest) (*SearchResponse, error) {
	if len([]rune(request.Query)) < 2 {
		return nil, status.Error(codes.InvalidArgument, "query must contain at least 2 characters")
	}
	items, total, err := s.service.SearchPosts(ctx, request.Query, request.Page, request.PageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "search posts failed")
	}
	page, pageSize := normalizePagination(request.Page, request.PageSize)
	return &SearchResponse{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: service.PageCount(total, pageSize)}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-search"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation SearchServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*SearchServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Search", Handler: searchHandler},
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

func searchHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(SearchRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(SearchServiceServer).Search(ctx, request.(*SearchRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Search", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(SearchServiceServer).Health(ctx, request.(*struct{}))
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
