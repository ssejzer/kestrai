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

package cli

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

// execute runs the root command with args, capturing stdout/stderr.
func execute(args ...string) (string, error) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestVersionCommand(t *testing.T) {
	out, err := execute("version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "kestrai ") {
		t.Errorf("version output = %q, want it to contain 'kestrai '", out)
	}
}

func TestServerRoleValidation(t *testing.T) {
	if _, err := execute("server", "bogus"); err == nil {
		t.Error("server bogus: want error, got nil")
	}
	if _, err := execute("server", "api"); err != nil {
		t.Errorf("server api (stub): want nil, got %v", err)
	}
}

// rpc runs fn with a short per-call deadline so a stuck call cannot hang the
// test loop. The cancel is always invoked, satisfying go vet's lostcancel.
func rpc[T any](t *testing.T, fn func(context.Context) (T, error)) (T, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return fn(ctx)
}

// TestUpServesAndReconciles brings up the all-in-one process on a free port and
// drives the full path: health, a CRUD write through the local-dev auth
// interceptor, and reconciler convergence on the created Workflow.
func TestUpServesAndReconciles(t *testing.T) {
	addr := freePort(t)
	dbPath := filepath.Join(t.TempDir(), "state.db")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runUp(ctx, addr, dbPath) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("runUp returned %v, want nil on clean shutdown", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("runUp did not shut down after context cancel")
		}
	}()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer conn.Close()

	sys := apiv1.NewSystemServiceClient(conn)
	projects := apiv1.NewProjectServiceClient(conn)
	workflows := apiv1.NewWorkflowServiceClient(conn)

	// Wait for the server to accept requests.
	if !eventually(t, 5*time.Second, func() bool {
		resp, err := rpc(t, func(c context.Context) (*apiv1.GetHealthResponse, error) {
			return sys.GetHealth(c, &apiv1.GetHealthRequest{})
		})
		return err == nil && resp.GetStatus() == apiv1.GetHealthResponse_SERVING_STATUS_SERVING
	}) {
		t.Fatal("server did not become healthy")
	}

	// CRUD writes flow through the auth interceptor → store.
	if _, err := rpc(t, func(c context.Context) (*apiv1.CreateProjectResponse, error) {
		return projects.CreateProject(c, &apiv1.CreateProjectRequest{
			Project: &apiv1.Project{Metadata: &apiv1.ObjectMeta{Name: "app"}},
		})
	}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := rpc(t, func(c context.Context) (*apiv1.CreateWorkflowResponse, error) {
		return workflows.CreateWorkflow(c, &apiv1.CreateWorkflowRequest{
			Workflow: &apiv1.Workflow{
				Metadata: &apiv1.ObjectMeta{Name: "build", Project: "app"},
				Spec:     &apiv1.WorkflowSpec{Goal: "ship it", Pipeline: []*apiv1.WorkflowPhase{{Name: "execute"}}},
			},
		})
	}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	// The reconciler should stamp status within a few resync ticks.
	if !eventually(t, 5*time.Second, func() bool {
		got, err := rpc(t, func(c context.Context) (*apiv1.GetWorkflowResponse, error) {
			return workflows.GetWorkflow(c, &apiv1.GetWorkflowRequest{Project: "app", Name: "build"})
		})
		return err == nil && len(got.GetWorkflow().GetStatus().GetConditions()) > 0
	}) {
		t.Fatal("reconciler did not set Workflow status within deadline")
	}
}

// eventually polls cond every 50ms until it is true or timeout elapses.
func eventually(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// freePort returns a currently-free 127.0.0.1 address. There is a small race
// between closing the probe listener and runUp re-binding, tolerated in tests.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}
