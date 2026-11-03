package auditrpc

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
	server := NewGRPCServer(logging.New("audit-test"))
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
	audit := &model.AuditEvent{RequestID: "request-1", TraceID: "trace-1", Service: "blog-api", Action: "post.created", ResourceType: "post", ResourceID: 7}
	outbox := &model.OutboxEvent{EventType: "post.created", AggregateType: "post", AggregateID: 7, Payload: "{}"}
	if err := client.Record(ctx, audit, outbox); err != nil {
		t.Fatal(err)
	}
	items, total, err := client.List(ctx, 1, 10)
	if err != nil || total != 1 || len(items) != 1 || items[0].Action != audit.Action {
		t.Fatalf("unexpected audit response: total=%d items=%d error=%v", total, len(items), err)
	}
	outboxItems, err := client.ListPendingOutbox(ctx, 10)
	if err != nil || len(outboxItems) != 1 || outboxItems[0].EventType != outbox.EventType {
		t.Fatalf("unexpected outbox response: items=%d error=%v", len(outboxItems), err)
	}
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health RPC failed: %v", err)
	}
}
