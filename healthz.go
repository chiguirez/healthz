package healthz

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	proto "github.com/chiguirez/healthz/proto/health/v1"
)

var _checker *checker

type checker struct {
	srv  *grpc.Server
	list []HealthChecker
}

var ErrUnsucessful = errors.New("unsuccessful health check")

func (c checker) Check(ctx context.Context, _ *proto.CheckRequest) (*proto.CheckResponse, error) {
	g, _ctx := errgroup.WithContext(ctx)

	for _, chkr := range c.list {
		g.Go(
			func(chkr HealthChecker) func() error {
				return func() error {
					if !chkr.HealthCheck(_ctx) {
						return fmt.Errorf("%w for dependency %v", ErrUnsucessful, chkr)
					}

					return nil
				}
			}(chkr),
		)
	}

	if g.Wait() != nil {
		return &proto.CheckResponse{
			Status: proto.CheckResponse_SERVING_STATUS_NOT_SERVING,
		}, nil
	}

	return &proto.CheckResponse{
		Status: proto.CheckResponse_SERVING_STATUS_SERVING,
	}, nil
}

func (c checker) Ping(_ context.Context, _ *proto.PingRequest) (*proto.PongResponse, error) {
	return &proto.PongResponse{
		Pong: true,
	}, nil
}

type HealthChecker interface {
	HealthCheck(ctx context.Context) bool
}

type HealthCheckOptions func(c *checker)

func WithGRPCServer(srv *grpc.Server) HealthCheckOptions {
	return func(c *checker) {
		c.srv = srv
	}
}

func WithChecker(chkr HealthChecker) HealthCheckOptions {
	return func(c *checker) {
		c.list = append(c.list, chkr)
	}
}

func Register(opts ...HealthCheckOptions) {
	var (
		deferredStartFn func()
	)

	if _checker == nil {
		_checker = &checker{}
	}

	for _, opt := range opts {
		opt(_checker)
	}

	if _checker.srv == nil {
		deferredStartFn = createServer(_checker)
	}

	proto.RegisterHealthServiceServer(_checker.srv, _checker)

	if deferredStartFn != nil {
		deferredStartFn()
	}

	registerHTTPServer(_checker)
}

func registerHTTPServer(c *checker) {
	const httpAddress = ":8081"

	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()

	_ = proto.RegisterHealthServiceHandlerServer(ctx, mux, c)

	go func() {
		_ = http.ListenAndServe(httpAddress, mux)
	}()
}

func Unregister() {
	_checker.srv.GracefulStop()
	_checker = nil
}

func createServer(c *checker) func() {
	lis, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	c.srv = grpc.NewServer()

	return func() {
		reflection.Register(c.srv)

		go func() {
			_ = c.srv.Serve(lis)
		}()
	}
}
