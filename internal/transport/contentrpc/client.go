package contentrpc

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

func (c *Client) ListPosts(ctx context.Context, page, pageSize int) ([]model.Post, int64, error) {
	request := &ListPostsRequest{Page: page, PageSize: pageSize}
	response := new(ListPostsResponse)
	if err := c.invoke(ctx, "ListPosts", request, response); err != nil {
		return nil, 0, err
	}
	return response.Items, response.Total, nil
}

func (c *Client) SearchPosts(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error) {
	request := &SearchPostsRequest{Query: query, Page: page, PageSize: pageSize}
	response := new(ListPostsResponse)
	if err := c.invoke(ctx, "SearchPosts", request, response); err != nil {
		return nil, 0, err
	}
	return response.Items, response.Total, nil
}

func (c *Client) GetPost(ctx context.Context, identifier string, _ bool) (model.Post, error) {
	request := &GetPostRequest{Identifier: identifier}
	response := new(PostResponse)
	if err := c.invoke(ctx, "GetPost", request, response); err != nil {
		return model.Post{}, err
	}
	return response.Post, nil
}

func (c *Client) GetPostWithoutView(ctx context.Context, identifier string, publicOnly bool) (model.Post, error) {
	if !publicOnly {
		return c.GetPost(ctx, identifier, false)
	}
	return c.GetPost(ctx, identifier, true)
}

func (c *Client) ListCategories(ctx context.Context) ([]model.Category, error) {
	response, err := c.taxonomy(ctx)
	if err != nil {
		return nil, err
	}
	return response.Categories, nil
}

func (c *Client) ListTags(ctx context.Context) ([]model.Tag, error) {
	response, err := c.taxonomy(ctx)
	if err != nil {
		return nil, err
	}
	return response.Tags, nil
}

func (c *Client) Health(ctx context.Context) error {
	response := new(HealthResponse)
	if err := c.invoke(ctx, "Health", &struct{}{}, response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("content service status is %q", response.Status)
	}
	return nil
}

func (c *Client) taxonomy(ctx context.Context) (*TaxonomyResponse, error) {
	response := new(TaxonomyResponse)
	if err := c.invoke(ctx, "ListTaxonomy", &struct{}{}, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) invoke(ctx context.Context, method string, request, response any) error {
	if logging.RequestID(ctx) == "" {
		ctx = logging.WithIDs(ctx, logging.NewID(), logging.NewID())
	}
	return c.connection.Invoke(ctx, "/"+serviceName+"/"+method, request, response)
}
