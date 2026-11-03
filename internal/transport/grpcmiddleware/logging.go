package grpcmiddleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func UnaryServerLogging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = contextWithMetadata(ctx)
		started := time.Now()
		response, err := handler(ctx, request)
		logger.InfoContext(ctx, "grpc_request",
			"method", info.FullMethod,
			"code", status.Code(err).String(),
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", logging.RequestID(ctx),
			"trace_id", logging.TraceID(ctx),
		)
		return response, err
	}
}

func UnaryClientLogging(logger *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply any, conn *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		ctx = metadataContext(ctx)
		started := time.Now()
		err := invoker(ctx, method, request, reply, conn, options...)
		logger.InfoContext(ctx, "grpc_call",
			"method", method,
			"code", status.Code(err).String(),
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", logging.RequestID(ctx),
			"trace_id", logging.TraceID(ctx),
		)
		return err
	}
}

func contextWithMetadata(ctx context.Context) context.Context {
	requestID, traceID := metadataIDs(ctx)
	if requestID == "" {
		requestID = logging.NewID()
	}
	if traceID == "" {
		traceID = requestID
	}
	return logging.WithIDs(ctx, requestID, traceID)
}

func metadataContext(ctx context.Context) context.Context {
	requestID := logging.RequestID(ctx)
	traceID := logging.TraceID(ctx)
	if requestID == "" && traceID == "" {
		requestID = logging.NewID()
		traceID = requestID
	}
	if traceID == "" {
		traceID = requestID
	}
	if requestID == "" {
		requestID = traceID
	}
	return metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID, "x-trace-id", traceID)
}

func metadataIDs(ctx context.Context) (string, string) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ""
	}
	return first(values.Get("x-request-id")), first(values.Get("x-trace-id"))
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
