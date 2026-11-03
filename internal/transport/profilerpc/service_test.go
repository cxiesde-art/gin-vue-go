package profilerpc

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
	user := &model.User{Email: "reader@example.com", DisplayName: "Reader", Role: "reader", PasswordHash: "hash"}
	if err := repo.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("profile-test"))
	RegisterServer(server, NewServer(repo))
	go server.Serve(listener)
	t.Cleanup(func() { server.Stop(); listener.Close() })
	connection, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	client := &Client{connection: connection}
	found, err := client.Get(context.Background(), user.ID)
	if err != nil || found.DisplayName != "Reader" {
		t.Fatalf("get profile failed: %#v error=%v", found, err)
	}
	updated, err := client.Update(context.Background(), user.ID, "Vue Reader")
	if err != nil || updated.DisplayName != "Vue Reader" {
		t.Fatalf("update profile failed: %#v error=%v", updated, err)
	}
	if err := client.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}
