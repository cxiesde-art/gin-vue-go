package commentrpc

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
	connection, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})), grpc.WithUnaryInterceptor(grpcmiddleware.UnaryClientLogging(logger)))
	if err != nil {
		return nil, err
	}
	return &Client{connection: connection}, nil
}

func (c *Client) Close() error { return c.connection.Close() }

func (c *Client) List(ctx context.Context, postID uint, includePending bool) ([]model.Comment, error) {
	response := new(ListResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/List", &ListRequest{PostID: postID, IncludePending: includePending}, response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) Create(ctx context.Context, postID uint, name, email, body string) (uint, string, error) {
	response := new(CreateResponse)
	if err := c.connection.Invoke(ctx, "/"+serviceName+"/Create", &CreateRequest{PostID: postID, Name: name, Email: email, Body: body}, response); err != nil {
		return 0, "", err
	}
	return response.ID, response.Message, nil
}

func (c *Client) Moderate(ctx context.Context, id uint, approved bool) error {
	response := new(ModerateResponse)
	return c.connection.Invoke(ctx, "/"+serviceName+"/Moderate", &ModerateRequest{ID: id, Approved: approved}, response)
}

func (c *Client) Health(ctx context.Context) error {
	return c.connection.Invoke(ctx, "/"+serviceName+"/Health", &struct{}{}, new(HealthResponse))
}
