package authrpc

import (
	"context"
	"log/slog"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcmiddleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "blog.identity.v1.IdentityService"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

type ValidateRequest struct {
	Token string `json:"token"`
}

type ValidateResponse struct {
	UserID uint   `json:"userId"`
	Role   string `json:"role"`
}

type IdentityServiceServer interface {
	Login(context.Context, *LoginRequest) (*LoginResponse, error)
	Validate(context.Context, *ValidateRequest) (*ValidateResponse, error)
	Health(context.Context, *struct{}) (*HealthResponse, error)
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type Server struct {
	repo      *repository.Repository
	jwtSecret string
	now       func() time.Time
}

func NewServer(repo *repository.Repository, jwtSecret string) *Server {
	return &Server{repo: repo, jwtSecret: jwtSecret, now: time.Now().UTC}
}

func (s *Server) Login(ctx context.Context, request *LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindUserByEmail(ctx, request.Email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, request.Password) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := auth.IssueToken(s.jwtSecret, user.ID, user.Role, s.now())
	if err != nil {
		return nil, status.Error(codes.Internal, "issue token")
	}
	return &LoginResponse{Token: token, User: user}, nil
}

func (s *Server) Validate(ctx context.Context, request *ValidateRequest) (*ValidateResponse, error) {
	claims, err := auth.ParseToken(s.jwtSecret, request.Token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &ValidateResponse{UserID: claims.UserID, Role: claims.Role}, nil
}

func (s *Server) Health(context.Context, *struct{}) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "blog-identity"}, nil
}

func RegisterServer(server grpc.ServiceRegistrar, implementation IdentityServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*IdentityServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Login", Handler: loginHandler},
			{MethodName: "Validate", Handler: validateHandler},
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

func loginHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(LoginRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(IdentityServiceServer).Login(ctx, request.(*LoginRequest))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/Login"}, call)
}

func validateHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(ValidateRequest)
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(IdentityServiceServer).Validate(ctx, request.(*ValidateRequest))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/Validate"}, call)
}

func healthHandler(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(struct{})
	if err := decode(request); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, request any) (any, error) {
		return server.(IdentityServiceServer).Health(ctx, request.(*struct{}))
	}
	if interceptor == nil {
		return call(ctx, request)
	}
	return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/Health"}, call)
}
