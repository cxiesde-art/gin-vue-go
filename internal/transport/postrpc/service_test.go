package postrpc

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
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("post-test"))
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
	post := &model.Post{Title: "Remote post", Content: "body", Status: "published"}
	if err := client.Save(ctx, post); err != nil {
		t.Fatal(err)
	}
	if post.ID == 0 || post.Slug != "remote-post" {
		t.Fatalf("post was not normalized: %#v", post)
	}
	found, err := client.Get(ctx, post.Slug)
	if err != nil || found.ID != post.ID {
		t.Fatalf("unexpected post response: %#v error=%v", found, err)
	}
	items, total, err := client.List(ctx, 1, 10)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("unexpected list response: total=%d items=%d error=%v", total, len(items), err)
	}
	if err := repo.CreatePostRevision(ctx, &model.PostRevision{PostID: post.ID, Title: post.Title, Content: post.Content, Status: post.Status}); err != nil {
		t.Fatal(err)
	}
	revisions, err := client.ListRevisions(ctx, post.ID)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("unexpected revisions: %#v error=%v", revisions, err)
	}
	if err := client.Delete(ctx, post.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health RPC failed: %v", err)
	}
}
