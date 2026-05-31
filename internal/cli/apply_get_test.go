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
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/auth"
	"github.com/kestrai/kestrai/internal/server"
	"github.com/kestrai/kestrai/internal/store"
)

// writeFile writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// startServer brings up a real gRPC server (local-dev auth) on a free port and
// returns its address, for the resource-verb round-trip.
func startServer(t *testing.T) string {
	t.Helper()
	st, err := store.Open(context.Background(), "file:"+filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	interceptor := auth.NewInterceptor(auth.NewLocalDev(), systemMethods()...)
	srv := server.New(server.Config{Store: st}, interceptor.ServerOptions()...)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(lis) //nolint:errcheck // stopped via cleanup
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func TestApplyThenGet(t *testing.T) {
	addr := startServer(t)
	cfg := &clientConfig{address: addr}
	ctx := context.Background()

	// Write a Project and a Workflow from one multi-doc file.
	file := filepath.Join(t.TempDir(), "resources.yaml")
	writeFile(t, file, sampleYAML)
	if err := runApply(ctx, cfg, file); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// get project demo -o json should round-trip the spec.
	jsonCfg := &clientConfig{address: addr, output: outputJSON}
	if err := runGet(ctx, jsonCfg, kindProject, "demo", ""); err != nil {
		t.Fatalf("get project: %v", err)
	}

	// get workflow build -p demo (single object needs the project scope).
	if err := runGet(ctx, cfg, kindWorkflow, "build", "demo"); err != nil {
		t.Fatalf("get workflow: %v", err)
	}

	// Table-form list of both kinds.
	if err := runGet(ctx, cfg, kindProject, "", ""); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if err := runGet(ctx, cfg, kindWorkflow, "", ""); err != nil {
		t.Fatalf("list workflows: %v", err)
	}
}

// TestApplyIsIdempotent verifies a second apply of the same file succeeds
// (update path), not an AlreadyExists error.
func TestApplyIsIdempotent(t *testing.T) {
	addr := startServer(t)
	cfg := &clientConfig{address: addr}
	ctx := context.Background()

	file := filepath.Join(t.TempDir(), "p.yaml")
	writeFile(t, file, "apiVersion: kestrai.dev/v1alpha1\nkind: Project\nmetadata:\n  name: demo\nspec:\n  displayName: One\n")
	if err := runApply(ctx, cfg, file); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	writeFile(t, file, "apiVersion: kestrai.dev/v1alpha1\nkind: Project\nmetadata:\n  name: demo\nspec:\n  displayName: Two\n")
	if err := runApply(ctx, cfg, file); err != nil {
		t.Fatalf("second apply (update): %v", err)
	}

	// Confirm the update took: fetch and check displayName.
	c, err := cfg.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.close()
	cc, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.projects.GetProject(cc, &apiv1.GetProjectRequest{Name: "demo"})
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got := resp.GetProject().GetSpec().GetDisplayName(); got != "Two" {
		t.Errorf("displayName = %q, want Two after re-apply", got)
	}
}

func TestApplyMissingProjectFails(t *testing.T) {
	addr := startServer(t)
	cfg := &clientConfig{address: addr}

	file := filepath.Join(t.TempDir(), "wf.yaml")
	writeFile(t, file, "apiVersion: kestrai.dev/v1alpha1\nkind: Workflow\nmetadata:\n  name: build\n  project: ghost\nspec:\n  goal: x\n  pipeline:\n    - name: run\n")
	if err := runApply(context.Background(), cfg, file); err == nil {
		t.Error("apply workflow under missing project: want error, got nil")
	}
}
