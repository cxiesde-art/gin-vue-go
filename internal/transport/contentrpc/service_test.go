package contentrpc

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
	post := model.Post{Title: "Distributed Go", Slug: "distributed-go", Content: "RPC body", Status: "published"}
	if err := repo.SavePost(context.Background(), &post); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateCategory(context.Background(), &model.Category{Name: "Go", Slug: "go"}); err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("content-test"))
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
	items, total, err := client.SearchPosts(ctx, "Distributed", 1, 10)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("unexpected search response: total=%d items=%d error=%v", total, len(items), err)
	}
	found, err := client.GetPost(ctx, "distributed-go", true)
	if err != nil || found.Title != post.Title {
		t.Fatalf("unexpected post response: %#v error=%v", found, err)
	}
	categories, err := client.ListCategories(ctx)
	if err != nil || len(categories) != 1 || categories[0].Slug != "go" {
		t.Fatalf("unexpected taxonomy response: %#v error=%v", categories, err)
	}
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health RPC failed: %v", err)
	}
}
