package taxonomyrpc

import (
	"context"
	"log/slog"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcmiddleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "blog.taxonomy.v1.TaxonomyService"

type ListCategoriesResponse struct {
	Items []model.Category `json:"items"`
}

type ListTagsResponse struct {
	Items []model.Tag `json:"items"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type TaxonomyServiceServer interface {
	ListCategories(context.Context, *struct{}) (*ListCategoriesResponse, error)
	ListTags(context.Context, *struct{}) (*ListTagsResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	repo *repository.Repository
}

func NewServer(repo *repository.Repository) *Server { return &Server{repo: repo} }

func (s *Server) ListCategories(ctx context.Context, _ *struct{}) (*ListCategoriesResponse, error) {
	items, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "list categories failed")
	}
	return &ListCategoriesResponse{Items: items}, nil
}

func (s *Server) ListTags(ctx context.Context, _ *struct{}) (*ListTagsResponse, error) {
	items, err := s.repo.ListTags(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "list tags failed")
	}
	return &ListTagsResponse{Items: items}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-taxonomy"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation TaxonomyServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*TaxonomyServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "ListCategories", Handler: listCategoriesHandler},
			{MethodName: "ListTags", Handler: listTagsHandler},
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

func listCategoriesHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(TaxonomyServiceServer).ListCategories(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/ListCategories", call, interceptor)
}

func listTagsHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(TaxonomyServiceServer).ListTags(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/ListTags", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(TaxonomyServiceServer).Health(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Health", call, interceptor)
}

func invoke(ctx context.Context, request, server any, method string, call grpc.UnaryHandler, interceptor grpc.UnaryServerInterceptor) (any, error) {
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: method}, call)
}
