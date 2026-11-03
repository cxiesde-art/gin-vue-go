package postrpc

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

const serviceName = "blog.posts.v1.PostService"

type ListRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type ListResponse struct {
	Items    []model.Post `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int64        `json:"total"`
	Pages    int          `json:"pages"`
}

type GetRequest struct {
	Identifier string `json:"identifier"`
}

type PostResponse struct {
	Post model.Post `json:"post"`
}

type SaveRequest struct {
	Post model.Post `json:"post"`
}

type SaveResponse struct {
	Post model.Post `json:"post"`
}

type DeleteRequest struct {
	ID uint `json:"id"`
}

type RevisionsRequest struct {
	PostID uint `json:"postId"`
}

type RevisionsResponse struct {
	Items []model.PostRevision `json:"items"`
}

type RevisionResponse struct {
	Revision model.PostRevision `json:"revision"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type PostServiceServer interface {
	List(context.Context, *ListRequest) (*ListResponse, error)
	Get(context.Context, *GetRequest) (*PostResponse, error)
	Save(context.Context, *SaveRequest) (*SaveResponse, error)
	Delete(context.Context, *DeleteRequest) (*struct{}, error)
	CreateRevision(context.Context, *model.PostRevision) (*RevisionResponse, error)
	ListRevisions(context.Context, *RevisionsRequest) (*RevisionsResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	service *service.BlogService
	repo    *repository.Repository
}

func NewServer(repo *repository.Repository) *Server {
	return &Server{service: service.NewBlogService(repo), repo: repo}
}

func (s *Server) List(ctx context.Context, request *ListRequest) (*ListResponse, error) {
	items, total, err := s.service.ListAllPosts(ctx, request.Page, request.PageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "list posts failed")
	}
	page, pageSize := normalizePagination(request.Page, request.PageSize)
	return &ListResponse{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: service.PageCount(total, pageSize)}, nil
}

func (s *Server) Get(ctx context.Context, request *GetRequest) (*PostResponse, error) {
	post, err := s.service.GetPost(ctx, request.Identifier, false)
	if err == repository.ErrNotFound {
		return nil, status.Error(codes.NotFound, "post not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "get post failed")
	}
	return &PostResponse{Post: post}, nil
}

func (s *Server) Save(ctx context.Context, request *SaveRequest) (*SaveResponse, error) {
	if err := s.service.SavePost(ctx, &request.Post); err != nil {
		if err == service.ErrInvalidPost || err == service.ErrInvalidStatus {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "save post failed")
	}
	return &SaveResponse{Post: request.Post}, nil
}

func (s *Server) Delete(ctx context.Context, request *DeleteRequest) (*struct{}, error) {
	if request.ID == 0 {
		return nil, status.Error(codes.InvalidArgument, "post id is required")
	}
	if err := s.service.DeletePost(ctx, request.ID); err == repository.ErrNotFound {
		return nil, status.Error(codes.NotFound, "post not found")
	} else if err != nil {
		return nil, status.Error(codes.Internal, "delete post failed")
	}
	return &struct{}{}, nil
}

func (s *Server) ListRevisions(ctx context.Context, request *RevisionsRequest) (*RevisionsResponse, error) {
	if request.PostID == 0 {
		return nil, status.Error(codes.InvalidArgument, "post id is required")
	}
	items, err := s.repo.ListPostRevisions(ctx, request.PostID)
	if err != nil {
		return nil, status.Error(codes.Internal, "list post revisions failed")
	}
	return &RevisionsResponse{Items: items}, nil
}

func (s *Server) CreateRevision(ctx context.Context, revision *model.PostRevision) (*RevisionResponse, error) {
	if revision.PostID == 0 || revision.Title == "" || revision.Content == "" || revision.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "post revision fields are required")
	}
	if err := s.repo.CreatePostRevision(ctx, revision); err != nil {
		return nil, status.Error(codes.Internal, "create post revision failed")
	}
	return &RevisionResponse{Revision: *revision}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-posts"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation PostServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*PostServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "List", Handler: listHandler},
			{MethodName: "Get", Handler: getHandler},
			{MethodName: "Save", Handler: saveHandler},
			{MethodName: "Delete", Handler: deleteHandler},
			{MethodName: "CreateRevision", Handler: createRevisionHandler},
			{MethodName: "ListRevisions", Handler: listRevisionsHandler},
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

func listHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ListRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).List(ctx, request.(*ListRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/List", call, interceptor)
}

func getHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(GetRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).Get(ctx, request.(*GetRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Get", call, interceptor)
}

func saveHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(SaveRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).Save(ctx, request.(*SaveRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Save", call, interceptor)
}

func deleteHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(DeleteRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).Delete(ctx, request.(*DeleteRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Delete", call, interceptor)
}

func listRevisionsHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(RevisionsRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).ListRevisions(ctx, request.(*RevisionsRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/ListRevisions", call, interceptor)
}

func createRevisionHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	revision := new(model.PostRevision)
	if err := decode(revision); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).CreateRevision(ctx, request.(*model.PostRevision))
	}
	return invoke(ctx, revision, server, "/"+serviceName+"/CreateRevision", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(PostServiceServer).Health(ctx, request.(*struct{}))
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
