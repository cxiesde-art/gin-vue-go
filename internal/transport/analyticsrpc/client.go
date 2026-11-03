package analyticsrpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
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

func (c *Client) RecordView(ctx context.Context, postID uint) error {
	if postID == 0 {
		return fmt.Errorf("post id is required")
	}
	request := &RecordViewRequest{PostID: postID, RequestID: logging.RequestID(ctx), TraceID: logging.TraceID(ctx)}
	return c.connection.Invoke(ctx, "/"+serviceName+"/RecordView", request, new(RecordViewResponse))
}

func (c *Client) Stats(ctx context.Context, postID uint) (int64, error) {
	response := new(StatsResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Stats", &StatsRequest{PostID: postID}, response); err != nil {
		return 0, err
	}
	return response.Views, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Health", &struct{}{}, response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("analytics service status is %q", response.Status)
	}
	return nil
}
