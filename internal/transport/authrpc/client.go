package authrpc

import (
	"context"
	"log/slog"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcmiddleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	connection *grpc.ClientConn
}

func Dial(ctx context.Context, address string, logger *slog.Logger) (*Client, error) {
	connection, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})),
		grpc.WithUnaryInterceptor(grpcmiddleware.UnaryClientLogging(logger)),
	)
	if err != nil {
		return nil, err
	}
	return &Client{connection: connection}, nil
}

func (c *Client) Close() error { return c.connection.Close() }

func (c *Client) Login(ctx context.Context, email, password string) (model.User, string, error) {
	response := new(LoginResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Login", &LoginRequest{Email: email, Password: password}, response); err != nil {
		return model.User{}, "", err
	}
	return response.User, response.Token, nil
}

func (c *Client) Validate(ctx context.Context, token string) (uint, string, error) {
	response := new(ValidateResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Validate", &ValidateRequest{Token: token}, response); err != nil {
		return 0, "", err
	}
	return response.UserID, response.Role, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	return c.connection.Invoke(ctx, "/"+serviceName+"/Health", &struct{}{}, response)
}
