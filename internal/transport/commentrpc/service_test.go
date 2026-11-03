package commentrpc

import (
	"context"
	"net"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestCommentClientServerRoundTrip(t *testing.T) {
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("comments-test"))
	RegisterServer(server, NewServer(repo))
	go server.Serve(listener)
	t.Cleanup(func() { server.Stop(); listener.Close() })
	connection, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	client := &Client{connection: connection}
	id, message, err := client.Create(context.Background(), 7, "Reader", "reader@example.com", "Useful post")
	if err != nil || id == 0 || message == "" {
		t.Fatalf("create failed: id=%d message=%q error=%v", id, message, err)
	}
	comments, err := client.List(context.Background(), 7, true)
	if err != nil || len(comments) != 1 || comments[0].ID != id {
		t.Fatalf("list failed: %#v error=%v", comments, err)
	}
	if err := client.Moderate(context.Background(), id, true); err != nil {
		t.Fatal(err)
	}
	comments, err = client.List(context.Background(), 7, false)
	if err != nil || len(comments) != 1 {
		t.Fatalf("approved comment missing: %#v error=%v", comments, err)
	}
	if err := client.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}
