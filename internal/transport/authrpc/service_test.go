package authrpc

import (
	"context"
	"net"
	"testing"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/grpcjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestIdentityClientServerRoundTrip(t *testing.T) {
	repo, err := repository.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateUser(context.Background(), &model.User{Email: "reader@example.com", PasswordHash: auth.HashPassword("secret"), Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := NewGRPCServer(logging.New("identity-test"))
	RegisterServer(server, NewServer(repo, "secret-key"))
	go server.Serve(listener)
	t.Cleanup(func() { server.Stop(); listener.Close() })
	connection, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcjson.Codec{})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	client := &Client{connection: connection}
	user, token, err := client.Login(context.Background(), "reader@example.com", "secret")
	if err != nil || user.Email != "reader@example.com" || token == "" {
		t.Fatalf("login failed: %#v token=%q error=%v", user, token, err)
	}
	userID, role, err := client.Validate(context.Background(), token)
	if err != nil || userID != user.ID || role != "admin" {
		t.Fatalf("validation failed: id=%d role=%q error=%v", userID, role, err)
	}
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("health failed: %v", err)
	}
}
