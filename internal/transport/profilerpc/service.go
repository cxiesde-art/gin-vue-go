package profilerpc

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

const serviceName = "blog.profile.v1.ProfileService"

type GetRequest struct {
	UserID uint `json:"userId"`
}

type UpdateRequest struct {
	UserID      uint   `json:"userId"`
	DisplayName string `json:"displayName"`
}

type UserResponse struct {
	User model.User `json:"user"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type ProfileServiceServer interface {
	Get(context.Context, *GetRequest) (*UserResponse, error)
	Update(context.Context, *UpdateRequest) (*UserResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type Server struct {
	repo *repository.Repository
}

func NewServer(repo *repository.Repository) *Server { return &Server{repo: repo} }

func (s *Server) Get(ctx context.Context, request *GetRequest) (*UserResponse, error) {
	if request == nil || request.UserID == 0 {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}
	user, err := s.repo.FindUserByID(ctx, request.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "get profile failed")
	}
	return &UserResponse{User: user}, nil
}

func (s *Server) Update(ctx context.Context, request *UpdateRequest) (*UserResponse, error) {
	if request == nil || request.UserID == 0 {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}
	displayName, err := service.NormalizeDisplayName(request.DisplayName)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	user, err := s.repo.UpdateUserDisplayName(ctx, request.UserID, displayName)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "update profile failed")
	}
	return &UserResponse{User: user}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-profile"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation ProfileServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*ProfileServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Get", Handler: getHandler},
			{MethodName: "Update", Handler: updateHandler},
			{MethodName: "Health", Handler: healthHandler},
		},
	}, implementation)
}

func NewGRPCServer(logger *slog.Logger) *grpc.Server {
	return grpc.NewServer(grpc.ForceServerCodec(grpcjson.Codec{}), grpc.UnaryInterceptor(grpcmiddleware.UnaryServerLogging(logger)))
}

func getHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(GetRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ProfileServiceServer).Get(ctx, request.(*GetRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Get", call, interceptor)
}

func updateHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(UpdateRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ProfileServiceServer).Update(ctx, request.(*UpdateRequest))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Update", call, interceptor)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(ProfileServiceServer).Health(ctx, request.(*struct{}))
	}
	return invoke(ctx, request, server, "/"+serviceName+"/Health", call, interceptor)
}

func invoke(ctx context.Context, request, server any, method string, call grpc.UnaryHandler, interceptor grpc.UnaryServerInterceptor) (any, error) {
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: method}, call)
}
