// Copyright 2026 The Kestrai Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

// newTestConn starts a Server on an in-memory bufconn listener and returns a
// client connection to it. The server is stopped and the listener closed via
// t.Cleanup.
func newTestConn(t *testing.T, cfg Config) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := New(cfg)
	go func() {
		if err := srv.Serve(lis); err != nil {
			// Serve returns nil on a clean Stop; anything else is a test bug.
			t.Errorf("Serve: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	})
	return conn
}

func TestSystemServiceGetHealth(t *testing.T) {
	conn := newTestConn(t, Config{})
	client := apiv1.NewSystemServiceClient(conn)

	resp, err := client.GetHealth(context.Background(), &apiv1.GetHealthRequest{})
	if err != nil {
		t.Fatalf("GetHealth: %v", err)
	}
	if got := resp.GetStatus(); got != apiv1.GetHealthResponse_SERVING_STATUS_SERVING {
		t.Errorf("status = %v, want SERVING", got)
	}
}

func TestSystemServiceGetVersion(t *testing.T) {
	conn := newTestConn(t, Config{Version: "1.2.3-test", APIVersion: "kestrai.dev/vtest"})
	client := apiv1.NewSystemServiceClient(conn)

	resp, err := client.GetVersion(context.Background(), &apiv1.GetVersionRequest{})
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got, want := resp.GetVersion(), "1.2.3-test"; got != want {
		t.Errorf("version = %q, want %q", got, want)
	}
	if got, want := resp.GetApiVersion(), "kestrai.dev/vtest"; got != want {
		t.Errorf("api_version = %q, want %q", got, want)
	}
}

// TestVersionDefaults verifies the empty Config falls back to compiled-in
// build metadata rather than serving empty strings.
func TestVersionDefaults(t *testing.T) {
	conn := newTestConn(t, Config{})
	client := apiv1.NewSystemServiceClient(conn)

	resp, err := client.GetVersion(context.Background(), &apiv1.GetVersionRequest{})
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if resp.GetVersion() == "" {
		t.Error("version is empty, want compiled-in default")
	}
	if resp.GetApiVersion() == "" {
		t.Error("api_version is empty, want compiled-in default")
	}
}

// TestCRUDUnimplemented locks in that the Project/Workflow surfaces are
// registered (reachable) but not yet backed by a store. When the store lands,
// these expectations flip to real assertions.
func TestCRUDUnimplemented(t *testing.T) {
	conn := newTestConn(t, Config{})

	projects := apiv1.NewProjectServiceClient(conn)
	if _, err := projects.CreateProject(context.Background(), &apiv1.CreateProjectRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("CreateProject code = %v, want Unimplemented", status.Code(err))
	}

	workflows := apiv1.NewWorkflowServiceClient(conn)
	if _, err := workflows.GetWorkflow(context.Background(), &apiv1.GetWorkflowRequest{}); status.Code(err) != codes.Unimplemented {
		t.Errorf("GetWorkflow code = %v, want Unimplemented", status.Code(err))
	}
}
