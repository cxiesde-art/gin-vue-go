package searchrpc

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
	policy     RetryPolicy
}

func Dial(ctx context.Context, address string, logger *slog.Logger) (*Client, error) {
	return DialWithPolicy(ctx, address, logger, DefaultRetryPolicy)
}

func DialWithPolicy(ctx context.Context, address string, logger *slog.Logger, policy RetryPolicy) (*Client, error) {
	connection, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})),
		grpc.WithUnaryInterceptor(grpcmiddleware.UnaryClientLogging(logger)),
	)
	if err != nil {
		return nil, err
	}
	return &Client{connection: connection, policy: policy}, nil
}

func (c *Client) Close() error { return c.connection.Close() }

func (c *Client) SearchPosts(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error) {
	response := new(SearchResponse)
	if err := c.invoke(ctx, "Search", &SearchRequest{Query: query, Page: page, PageSize: pageSize}, response); err != nil {
		return nil, 0, err
	}
	return response.Items, response.Total, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	if err := c.invoke(ctx, "Health", &struct{}{}, response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("search service status is %q", response.Status)
	}
	return nil
}

func (c *Client) invoke(ctx context.Context, method string, request, response any) error {
	if logging.RequestID(ctx) == "" {
		ctx = logging.WithIDs(ctx, logging.NewID(), logging.NewID())
	}
	return retryRPC(ctx, c.policy, func() error {
		return c.connection.Invoke(ctx, "/"+serviceName+"/"+method, request, response)
	})
}
