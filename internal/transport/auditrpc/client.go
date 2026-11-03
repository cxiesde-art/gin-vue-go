package auditrpc

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

func (c *Client) Record(ctx context.Context, audit *model.AuditEvent, outbox *model.OutboxEvent) error {
	if audit == nil || outbox == nil {
		return fmt.Errorf("audit and outbox events are required")
	}
	return c.connection.Invoke(ctx, "/"+serviceName+"/Record", &RecordRequest{Audit: *audit, Outbox: *outbox}, new(RecordResponse))
}

func (c *Client) List(ctx context.Context, page, pageSize int) ([]model.AuditEvent, int64, error) {
	response := new(ListResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/List", &ListRequest{Page: page, PageSize: pageSize}, response); err != nil {
		return nil, 0, err
	}
	return response.Items, response.Total, nil
}

func (c *Client) ListPendingOutbox(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	response := new(ListOutboxResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/ListOutbox", &ListOutboxRequest{Limit: limit}, response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Health", &struct{}{}, response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("audit service status is %q", response.Status)
	}
	return nil
}
