package taxonomyrpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
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

func (c *Client) ListCategories(ctx context.Context) ([]model.Category, error) {
	response := new(ListCategoriesResponse)
	if err := c.invoke(ctx, "ListCategories", response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) ListTags(ctx context.Context) ([]model.Tag, error) {
	response := new(ListTagsResponse)
	if err := c.invoke(ctx, "ListTags", response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	if err := c.invoke(ctx, "Health", response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("taxonomy service status is %q", response.Status)
	}
	return nil
}

func (c *Client) invoke(ctx context.Context, method string, response any) error {
	if logging.RequestID(ctx) == "" {
		ctx = logging.WithIDs(ctx, logging.NewID(), logging.NewID())
	}
	return c.connection.Invoke(ctx, "/"+serviceName+"/"+method, &struct{}{}, response)
}
