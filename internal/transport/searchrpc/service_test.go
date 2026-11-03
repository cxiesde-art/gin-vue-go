package searchrpc

import (
	"context"
	"net"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestClientServerRoundTrip(t *testing.T) {
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	post := model.Post{Title: "Distributed Search", Slug: "distributed-search", Content: "searchable body", Status: "published"}
	if err := repo.SavePost(context.Background(), &post); err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("search-test"))
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
	client := &Client{connection: connection, policy: RetryPolicy{MaxAttempts: 1}}
	ctx := logging.WithIDs(context.Background(), "request-1", "trace-1")
	items, total, err := client.SearchPosts(ctx, "searchable", 1, 10)
	if err != nil || total != 1 || len(items) != 1 || items[0].Slug != post.Slug {
		t.Fatalf("unexpected search response: total=%d items=%d error=%v", total, len(items), err)
	}
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health RPC failed: %v", err)
	}
	if _, _, err := client.SearchPosts(ctx, "x", 1, 10); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid query, got %v", err)
	}
}
