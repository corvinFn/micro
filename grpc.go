package micro

import (
	"context"
	"time"

	"micro/tracer"

	"github.com/pkg/errors"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
)

// default timeout: 1s
func DialGRPC(url string) (*grpc.ClientConn, error) {
	return DialGRPCWithTimeout(time.Second, url)
}

func DialGRPCWithTimeout(timeout time.Duration, url string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DialGRPCContext(ctx, url)
}

func DialGRPCContext(ctx context.Context, url string) (*grpc.ClientConn, error) {
	conn, err := grpc.DialContext(
		ctx,
		url,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: time.Millisecond * 100,
		}),
		tracer.GrpcDialOption(),
	)
	if err != nil {
		return nil, errors.Errorf("fail to dail:%s", url)
	}
	return conn, nil
}
