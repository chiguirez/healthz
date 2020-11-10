package healthz_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/chiguirez/healthz"
	proto "github.com/chiguirez/healthz/proto/health/v1"
)

//nolint:funlen
func TestHealthCheck(t *testing.T) {
	t.Run("Given a service and a HealthCheck GRPC Client", func(t *testing.T) {
		healthz.Register()
		defer healthz.Unregister()
		// Set up a connection to the server.
		conn, err := grpc.Dial(":8080", grpc.WithInsecure())
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		checkClient := proto.NewHealthClient(conn)

		t.Run("When Checked via GRPC", func(t *testing.T) {
			check, err := checkClient.Check(context.Background(), &proto.HealthCheckRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			t.Run("Then is serving", func(t *testing.T) {
				if check.Status.String() != "SERVING" {
					t.Fatalf("status supposed to be SERVING, got: %v instead", check)
				}
			})
		})

		t.Run("When Pinged via GRPC Then Pong is always true", func(t *testing.T) {
			pong, err := checkClient.Ping(context.Background(), &proto.PingRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			if pong.GetPong() != true {
				t.Fatalf("Pong expected to be true got: %v instead", err)
			}
		})
	})

	t.Run("Given a service and a fault dependency", func(t *testing.T) {
		healthz.Register(healthz.WithChecker(faultInMemoryChecker{}))
		defer healthz.Unregister()
		// Set up a connection to the server.
		conn, err := grpc.Dial(":8080", grpc.WithInsecure())
		if err != nil {
			t.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		checkClient := proto.NewHealthClient(conn)

		t.Run("When Checked", func(t *testing.T) {
			_, err := checkClient.Check(context.Background(), &proto.HealthCheckRequest{})

			t.Run("Then is not serving", func(t *testing.T) {
				if err != nil {
					fromError, ok := status.FromError(err)
					if !ok {
						t.Fatalf("did not connect: %v", err)
					}
					if fromError.Code() != codes.Unavailable {
						t.Fatalf("expect Unavailable got : %s instead", fromError.Code().String())
					}
				}
			})
		})
	})

	t.Run("Given a GRPCServer", func(t *testing.T) {
		lis, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}

		s := grpc.NewServer()
		defer s.GracefulStop()

		healthz.Register(healthz.WithGRPCServer(s))
		defer healthz.Unregister()

		go func() {
			_ = s.Serve(lis)
		}()

		// Set up a connection to the server.
		conn, err := grpc.Dial(":8080", grpc.WithInsecure())
		if err != nil {
			t.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		checkClient := proto.NewHealthClient(conn)

		t.Run("When Checked", func(t *testing.T) {
			check, err := checkClient.Check(context.Background(), &proto.HealthCheckRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			t.Run("Then is serving", func(t *testing.T) {
				if check.Status.String() != "SERVING" {
					t.Fatalf("status supposed to be SERVING, got: %v instead", check)
				}
			})
		})
	})

	t.Run("Given a service and a HealthCheck HTTP Client", func(t *testing.T) {
		healthz.Register()
		defer healthz.Unregister()

		t.Run("When Pinged via HTTP Then Pong is always true", func(t *testing.T) {
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8081/v1/ping", nil)
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			check, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			defer check.Body.Close()
			body, err := ioutil.ReadAll(check.Body)
			if err != nil {
				t.Fatalf("error fetching data: %v", err)
			}
			v := make(map[string]interface{}, 1)
			err = json.Unmarshal(body, &v)
			if err != nil {
				t.Fatalf("unmarshalling error : %v", err)
			}
			if v["pong"] != true {
				t.Fatalf("Pong expected to be true got: %v instead", err)
			}
		})
	})
}

type faultInMemoryChecker struct{}

func (i faultInMemoryChecker) HealthCheck(_ context.Context) bool {
	return false
}
