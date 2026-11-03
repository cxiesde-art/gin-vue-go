package taxonomyrpc

import (
	"context"
	"net"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestClientServerRoundTrip(t *testing.T) {
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateCategory(context.Background(), &model.Category{Name: "Go", Slug: "go"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTag(context.Background(), &model.Tag{Name: "Gin", Slug: "gin"}); err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("taxonomy-test"))
	RegisterServer(server, NewServer(repo))
	go server.Serve(listener)
	t.Cleanup(func() {
		server.Stop()
		listener.Close()
	})

	connection, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	client := &Client{connection: connection}
	ctx := logging.WithIDs(context.Background(), "request-1", "trace-1")
	categories, err := client.ListCategories(ctx)
	if err != nil || len(categories) != 1 || categories[0].Slug != "go" {
		t.Fatalf("unexpected categories: %#v error=%v", categories, err)
	}
	tags, err := client.ListTags(ctx)
	if err != nil || len(tags) != 1 || tags[0].Slug != "gin" {
		t.Fatalf("unexpected tags: %#v error=%v", tags, err)
	}
	category, err := client.CreateCategory(ctx, "Rust", "")
	if err != nil || category.Slug != "rust" {
		t.Fatalf("unexpected created category: %#v error=%v", category, err)
	}
	tag, err := client.CreateTag(ctx, "Axum", "")
	if err != nil || tag.Slug != "axum" {
		t.Fatalf("unexpected created tag: %#v error=%v", tag, err)
	}
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health RPC failed: %v", err)
	}
}
