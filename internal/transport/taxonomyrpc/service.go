package taxonomyrpc

import (
	"context"
	"errors"
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

const serviceName = "blog.taxonomy.v1.TaxonomyService"

type ListCategoriesResponse struct {
	Items []model.Category `json:"items"`
}

type ListTagsResponse struct {
	Items []model.Tag `json:"items"`
}

type CreateRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type CategoryResponse struct {
	Category model.Category `json:"category"`
}

type TagResponse struct {
	Tag model.Tag `json:"tag"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type TaxonomyServiceServer interface {
	ListCategories(context.Context, *struct{}) (*ListCategoriesResponse, error)
	ListTags(context.Context, *struct{}) (*ListTagsResponse, error)
	CreateCategory(context.Context, *CreateRequest) (*CategoryResponse, error)
	CreateTag(context.Context, *CreateRequest) (*TagResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	repo    *repository.Repository
	service *service.BlogService
}

func NewServer(repo *repository.Repository) *Server {
	return &Server{repo: repo, service: service.NewBlogService(repo)}
}

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

func (s *Server) CreateCategory(ctx context.Context, request *CreateRequest) (*CategoryResponse, error) {
	category, err := s.service.CreateCategory(ctx, request.Name, request.Slug)
	if errors.Is(err, service.ErrInvalidTaxonomy) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "create category failed")
	}
	return &CategoryResponse{Category: category}, nil
}

func (s *Server) CreateTag(ctx context.Context, request *CreateRequest) (*TagResponse, error) {
	tag, err := s.service.CreateTag(ctx, request.Name, request.Slug)
	if errors.Is(err, service.ErrInvalidTaxonomy) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "create tag failed")
	}
	return &TagResponse{Tag: tag}, nil
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
			{MethodName: "CreateCategory", Handler: createCategoryHandler},
			{MethodName: "CreateTag", Handler: createTagHandler},
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

func createCategoryHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(CreateRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(TaxonomyServiceServer).CreateCategory(ctx, request.(*CreateRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/CreateCategory", call, interceptor)
}

func createTagHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(CreateRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(TaxonomyServiceServer).CreateTag(ctx, request.(*CreateRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/CreateTag", call, interceptor)
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
