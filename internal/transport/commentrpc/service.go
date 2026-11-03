package commentrpc

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

const serviceName = "blog.comments.v1.CommentService"

type ListRequest struct {
	PostID         uint `json:"postId"`
	IncludePending bool `json:"includePending"`
}

type ListResponse struct {
	Items []model.Comment `json:"items"`
}

type CreateRequest struct {
	PostID uint   `json:"postId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Body   string `json:"body"`
}

type CreateResponse struct {
	ID      uint   `json:"id"`
	Message string `json:"message"`
}

type ModerateRequest struct {
	ID       uint `json:"id"`
	Approved bool `json:"approved"`
}

type ModerateResponse struct {
	Approved bool `json:"approved"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type CommentServiceServer interface {
	List(context.Context, *ListRequest) (*ListResponse, error)
	Create(context.Context, *CreateRequest) (*CreateResponse, error)
	Moderate(context.Context, *ModerateRequest) (*ModerateResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	repo *repository.Repository
}

func NewServer(repo *repository.Repository) *Server { return &Server{repo: repo} }

func (s *Server) List(ctx context.Context, request *ListRequest) (*ListResponse, error) {
	comments, err := s.repo.ListComments(ctx, request.PostID, request.IncludePending)
	if err != nil {
		return nil, status.Error(codes.Internal, "list comments failed")
	}
	return &ListResponse{Items: comments}, nil
}

func (s *Server) Create(ctx context.Context, request *CreateRequest) (*CreateResponse, error) {
	if err := service.ValidateComment(request.Name, request.Email, request.Body); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	comment := &model.Comment{PostID: request.PostID, Name: request.Name, Email: request.Email, Body: request.Body}
	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return nil, status.Error(codes.Internal, "create comment failed")
	}
	return &CreateResponse{ID: comment.ID, Message: "comment submitted for moderation"}, nil
}

func (s *Server) Moderate(ctx context.Context, request *ModerateRequest) (*ModerateResponse, error) {
	if err := s.repo.SetCommentApproval(ctx, request.ID, request.Approved); err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Error(codes.NotFound, "comment not found")
		}
		return nil, status.Error(codes.Internal, "moderate comment failed")
	}
	return &ModerateResponse{Approved: request.Approved}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-comments"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation CommentServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*CommentServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "List", Handler: listHandler},
			{MethodName: "Create", Handler: createHandler},
			{MethodName: "Moderate", Handler: moderateHandler},
			{MethodName: "Health", Handler: healthHandler},
		},
	}, implementation)
}

func NewGRPCServer(logger *slog.Logger) *grpc.Server {
	return grpc.NewServer(grpc.ForceServerCodec(grpcjson.Codec{}), grpc.UnaryInterceptor(grpcmiddleware.UnaryServerLogging(logger)))
}

func listHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ListRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(CommentServiceServer).List(ctx, request.(*ListRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/List", call, interceptor)
}

func createHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(CreateRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(CommentServiceServer).Create(ctx, request.(*CreateRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Create", call, interceptor)
}

func moderateHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ModerateRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(CommentServiceServer).Moderate(ctx, request.(*ModerateRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Moderate", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(CommentServiceServer).Health(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Health", call, interceptor)
}

func invoke(ctx context.Context, request, server any, method string, call grpc.UnaryHandler, interceptor grpc.UnaryServerInterceptor) (any, error) {
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: method}, call)
}
