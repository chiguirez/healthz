package healthz

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	proto "github.com/chiguirez/healthz/proto/health/v1"
)

var _checker *serverCheckers //nolint:gochecknoglobals

type checker struct {
	hc         HealthChecker
	lastResult *int32
}

type serverCheckers struct {
	srv  *grpc.Server
	port struct {
		http uint16
		grpc uint16
	}
	list []checker
}

func (c serverCheckers) GetGRPCPort() uint16 {
	const defaultGRPCPort = 8080

	if c.port.grpc == 0 {
		return uint16(defaultGRPCPort)
	}

	return c.port.grpc
}

func (c serverCheckers) GetHTTPPort() uint16 {
	const defaultHTTPPort = 8081

	if c.port.http == 0 {
		return uint16(defaultHTTPPort)
	}

	return c.port.http
}

func deferChecking(c checker) chan bool {
	bChan := make(chan bool, 1)

	tBool := map[bool]int32{
		true:  1,
		false: 0,
	}

	go func() {
		result := c.hc.HealthCheck(context.Background())

		atomic.StoreInt32(c.lastResult, tBool[result])

		bChan <- result
		defer close(bChan)
	}()

	return bChan
}

func (hc checker) healthCheck(ctx context.Context) bool {
	const timeout = time.Second * 5

	ctx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()

	select {
	case <-deferChecking(hc):
		return atomic.LoadInt32(hc.lastResult) == int32(1)
	case <-ctx.Done():
		return atomic.LoadInt32(hc.lastResult) == int32(1)
	}
}

func (c serverCheckers) Check(ctx context.Context, _ *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	g, _ctx := errgroup.WithContext(ctx)

	for _, chkr := range c.list {
		g.Go(
			func(chkr checker) func() error {
				return func() error {
					if chkr.healthCheck(_ctx) {
						return fmt.Errorf("%w for dependency %v", ErrUnsuccessful, reflect.TypeOf(chkr).String())
					}

					return nil
				}
			}(chkr),
		)
	}

	if err := g.Wait(); err != nil {
		return nil, status.New(codes.Unavailable, err.Error()).Err()
	}

	return &proto.HealthCheckResponse{
		Status: proto.HealthCheckResponse_SERVING,
	}, nil
}

func (c serverCheckers) Watch(*proto.HealthCheckRequest, proto.Health_WatchServer) error {
	panic("implement me")
}

var ErrUnsuccessful = errors.New("unsuccessful health check")

func (c serverCheckers) Ping(_ context.Context, _ *proto.PingRequest) (*proto.PongResponse, error) {
	return &proto.PongResponse{
		Pong: true,
	}, nil
}

type HealthChecker interface {
	HealthCheck(ctx context.Context) bool
}

type HealthCheckOptions func(c *serverCheckers)

func WithGRPCServer(srv *grpc.Server) HealthCheckOptions {
	return func(c *serverCheckers) {
		c.srv = srv
	}
}

func WithChecker(chkr HealthChecker) HealthCheckOptions {
	return func(c *serverCheckers) {
		c.list = append(c.list, checker{
			hc:         chkr,
			lastResult: new(int32),
		})
	}
}

func WithGRPCPort(port uint16) HealthCheckOptions {
	return func(c *serverCheckers) {
		c.port.grpc = port
	}
}

func WithHTTPPort(port uint16) HealthCheckOptions {
	return func(c *serverCheckers) {
		c.port.http = port
	}
}

func Register(opts ...HealthCheckOptions) {
	var deferredStartFn func()

	if _checker == nil {
		_checker = &serverCheckers{}
	}

	for _, opt := range opts {
		opt(_checker)
	}

	if _checker.srv == nil {
		deferredStartFn = createServer(_checker)
	}

	proto.RegisterHealthServer(_checker.srv, _checker)

	if deferredStartFn != nil {
		deferredStartFn()
	}

	registerHTTPServer(_checker)
}

func registerHTTPServer(c *serverCheckers) {
	httpAddress := fmt.Sprintf(":%d", c.GetHTTPPort())
	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	_ = proto.RegisterHealthHandlerServer(ctx, mux, c)

	go func() {
		_ = http.ListenAndServe(httpAddress, mux)
	}()
}

func Unregister() {
	_checker.srv.GracefulStop()
	_checker = nil
}

func createServer(c *serverCheckers) func() {
	address := fmt.Sprintf("127.0.0.1:%d", c.GetGRPCPort())

	lis, err := net.Listen("tcp", address)
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
