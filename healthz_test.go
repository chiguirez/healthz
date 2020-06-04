package healthz

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"

	"google.golang.org/grpc"

	proto "github.com/chiguirez/healthz/proto/health/v1"
)

func TestHealthCheck(t *testing.T) {

	t.Run("Given a service and a HealthCheck GRPC Client", func(t *testing.T) {
		Register()
		defer Unregister()
		// Set up a connection to the server.
		conn, err := grpc.Dial(":8080", grpc.WithInsecure())
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		client := proto.NewHealthServiceClient(conn)

		t.Run("When Checked via GRPC", func(t *testing.T) {
			check, err := client.Check(context.Background(), &proto.CheckRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			t.Run("Then is serving", func(t *testing.T) {
				if check.Status.String() != "SERVING_STATUS_SERVING" {
					t.Fatalf("status supposed to be SERVING, got: %v instead", check)
				}
			})
		})

		t.Run("When Pinged via GRPC Then Pong is always true", func(t *testing.T) {
			pong, err := client.Ping(context.Background(), &proto.PingRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			if pong.GetPong() != true {
				t.Fatalf("Pong expected to be true got: %v instead", err)
			}
		})
	})

	t.Run("Given a service and a fault dependency", func(t *testing.T) {
		Register(WithChecker(faultInMemoryChecker{}))
		defer Unregister()
		// Set up a connection to the server.
		conn, err := grpc.Dial(":8080", grpc.WithInsecure())
		if err != nil {
			t.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		client := proto.NewHealthServiceClient(conn)

		t.Run("When Checked", func(t *testing.T) {
			check, err := client.Check(context.Background(), &proto.CheckRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			t.Run("Then is not serving", func(t *testing.T) {
				if check.Status.String() != "SERVING_STATUS_NOT_SERVING" {
					t.Fatalf("status supposed to be NOT_SERVING, got: %v instead", check)
				}
			})
		})
	})

	t.Run("Given a GRPCServer", func(t *testing.T) {
		lis, err := net.Listen("tcp", ":8080")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}

		s := grpc.NewServer()
		defer s.GracefulStop()

		Register(WithGRPCServer(s))
		defer Unregister()

		go func() {
			_ = s.Serve(lis)
		}()

		// Set up a connection to the server.
		conn, err := grpc.Dial(":8080", grpc.WithInsecure())
		if err != nil {
			t.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		client := proto.NewHealthServiceClient(conn)

		t.Run("When Checked", func(t *testing.T) {
			check, err := client.Check(context.Background(), &proto.CheckRequest{})
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			t.Run("Then is serving", func(t *testing.T) {
				if check.Status.String() != "SERVING_STATUS_SERVING" {
					t.Fatalf("status supposed to be SERVING, got: %v instead", check)
				}
			})
		})
	})

	t.Run("Given a service and a HealthCheck HTTP Client", func(t *testing.T) {
		Register()
		defer Unregister()

		t.Run("When Checked via HTTP", func(t *testing.T) {
			check, err := http.Get("http://localhost:8081/v1/check")
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
			body, err := ioutil.ReadAll(check.Body)
			if err != nil {
				t.Fatalf("error fetching data: %v", err)
			}
			v := make(map[string]interface{}, 1)
			err = json.Unmarshal(body, &v)
			if err != nil {
				t.Fatalf("unmarshalling error : %v", err)
			}
			t.Run("Then is serving", func(t *testing.T) {
				if v["status"] != "SERVING_STATUS_SERVING" {
					t.Fatalf("status supposed to be SERVING, got: %v instead", check)
				}
			})
		})

		t.Run("When Pinged via HTTP Then Pong is always true", func(t *testing.T) {
			check, err := http.Get("http://localhost:8081/v1/ping")
			if err != nil {
				t.Fatalf("did not connect: %v", err)
			}
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
