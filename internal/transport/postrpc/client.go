package postrpc

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

func (c *Client) List(ctx context.Context, page, pageSize int) ([]model.Post, int64, error) {
	response := new(ListResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/List", &ListRequest{Page: page, PageSize: pageSize}, response); err != nil {
		return nil, 0, err
	}
	return response.Items, response.Total, nil
}

func (c *Client) Get(ctx context.Context, identifier string) (model.Post, error) {
	response := new(PostResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Get", &GetRequest{Identifier: identifier}, response); err != nil {
		return model.Post{}, err
	}
	return response.Post, nil
}

func (c *Client) Save(ctx context.Context, post *model.Post) error {
	if post == nil {
		return fmt.Errorf("post is required")
	}
	response := new(SaveResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Save", &SaveRequest{Post: *post}, response); err != nil {
		return err
	}
	*post = response.Post
	return nil
}

func (c *Client) Delete(ctx context.Context, postID uint) error {
	return c.connection.Invoke(ctx, "/"+serviceName+"/Delete", &DeleteRequest{ID: postID}, new(struct{}))
}

func (c *Client) ListRevisions(ctx context.Context, postID uint) ([]model.PostRevision, error) {
	response := new(RevisionsResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/ListRevisions", &RevisionsRequest{PostID: postID}, response); err != nil {
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
		return fmt.Errorf("post service status is %q", response.Status)
	}
	return nil
}
