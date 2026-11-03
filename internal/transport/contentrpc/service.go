package contentrpc

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

const serviceName = "blog.content.v1.ContentService"

type ListPostsRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type ListPostsResponse struct {
	Items    []model.Post `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int64        `json:"total"`
	Pages    int          `json:"pages"`
}

type SearchPostsRequest struct {
	Query    string `json:"query"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type GetPostRequest struct {
	Identifier string `json:"identifier"`
}

type PostResponse struct {
	Post model.Post `json:"post"`
}

type TaxonomyResponse struct {
	Categories []model.Category `json:"categories,omitempty"`
	Tags       []model.Tag      `json:"tags,omitempty"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type ContentServiceServer interface {
	ListPosts(context.Context, *ListPostsRequest) (*ListPostsResponse, error)
	SearchPosts(context.Context, *SearchPostsRequest) (*ListPostsResponse, error)
	GetPost(context.Context, *GetPostRequest) (*PostResponse, error)
	ListTaxonomy(context.Context, *struct{}) (*TaxonomyResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	service *service.BlogService
}

func NewServer(repo *repository.Repository) *Server {
	return &Server{service: service.NewBlogService(repo)}
}

func (s *Server) ListPosts(ctx context.Context, request *ListPostsRequest) (*ListPostsResponse, error) {
	items, total, err := s.service.ListPosts(ctx, request.Page, request.PageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "list posts failed")
	}
	page, pageSize := normalizePagination(request.Page, request.PageSize)
	return &ListPostsResponse{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: service.PageCount(total, pageSize)}, nil
}

func (s *Server) SearchPosts(ctx context.Context, request *SearchPostsRequest) (*ListPostsResponse, error) {
	items, total, err := s.service.SearchPosts(ctx, request.Query, request.Page, request.PageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "search posts failed")
	}
	page, pageSize := normalizePagination(request.Page, request.PageSize)
	return &ListPostsResponse{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: service.PageCount(total, pageSize)}, nil
}

func (s *Server) GetPost(ctx context.Context, request *GetPostRequest) (*PostResponse, error) {
	post, err := s.service.GetPost(ctx, request.Identifier, true)
	if err == repository.ErrNotFound {
		return nil, status.Error(codes.NotFound, "post not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "get post failed")
	}
	return &PostResponse{Post: post}, nil
}

func (s *Server) ListTaxonomy(ctx context.Context, _ *struct{}) (*TaxonomyResponse, error) {
	categories, err := s.service.ListCategories(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "list categories failed")
	}
	tags, err := s.service.ListTags(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "list tags failed")
	}
	return &TaxonomyResponse{Categories: categories, Tags: tags}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-content"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation ContentServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*ContentServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "ListPosts", Handler: listPostsHandler},
			{MethodName: "SearchPosts", Handler: searchPostsHandler},
			{MethodName: "GetPost", Handler: getPostHandler},
			{MethodName: "ListTaxonomy", Handler: listTaxonomyHandler},
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

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return page, pageSize
}

func listPostsHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ListPostsRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ContentServiceServer).ListPosts(ctx, request.(*ListPostsRequest))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/ListPosts"}, call)
}

func searchPostsHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(SearchPostsRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ContentServiceServer).SearchPosts(ctx, request.(*SearchPostsRequest))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/SearchPosts"}, call)
}

func getPostHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(GetPostRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ContentServiceServer).GetPost(ctx, request.(*GetPostRequest))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/GetPost"}, call)
}

func listTaxonomyHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ContentServiceServer).ListTaxonomy(ctx, request.(*struct{}))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/ListTaxonomy"}, call)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ContentServiceServer).Health(ctx, request.(*struct{}))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/Health"}, call)
}
