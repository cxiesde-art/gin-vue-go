package profilerpc

import (
	"context"
	"fmt"
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

func (c *Client) Get(ctx context.Context, userID uint) (model.User, error) {
	if userID == 0 {
		return model.User{}, fmt.Errorf("user id is required")
	}
	response := new(UserResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Get", &GetRequest{UserID: userID}, response); err != nil {
		return model.User{}, err
	}
	return response.User, nil
}

func (c *Client) Update(ctx context.Context, userID uint, displayName string) (model.User, error) {
	if userID == 0 {
		return model.User{}, fmt.Errorf("user id is required")
	}
	response := new(UserResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Update", &UpdateRequest{UserID: userID, DisplayName: displayName}, response); err != nil {
		return model.User{}, err
	}
	return response.User, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Health", &struct{}{}, response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("profile service status is %q", response.Status)
	}
	return nil
}
