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
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/store"
)

// newStoreConn starts a Server backed by a fresh temp-file store and returns
// clients for it.
func newStoreConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	st, err := store.Open(context.Background(), "file:"+filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return newTestConn(t, Config{Store: st})
}

func project(name, display string) *apiv1.Project {
	return &apiv1.Project{
		Metadata: &apiv1.ObjectMeta{Name: name, Labels: map[string]string{"team": "core"}},
		Spec:     &apiv1.ProjectSpec{DisplayName: display},
	}
}

func TestProjectCRUD(t *testing.T) {
	ctx := context.Background()
	client := apiv1.NewProjectServiceClient(newStoreConn(t))

	created, err := client.CreateProject(ctx, &apiv1.CreateProjectRequest{Project: project("demo", "Demo")})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	meta := created.GetProject().GetMetadata()
	if meta.GetUid() == "" || meta.GetResourceVersion() != "1" || meta.GetGeneration() != 1 {
		t.Fatalf("unexpected assigned metadata: %+v", meta)
	}
	if meta.GetTenantId() != "default" {
		t.Errorf("tenant = %q, want default", meta.GetTenantId())
	}

	got, err := client.GetProject(ctx, &apiv1.GetProjectRequest{Name: "demo"})
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.GetProject().GetSpec().GetDisplayName() != "Demo" {
		t.Errorf("display name = %q, want Demo", got.GetProject().GetSpec().GetDisplayName())
	}

	list, err := client.ListProjects(ctx, &apiv1.ListProjectsRequest{LabelSelector: "team=core"})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list.GetItems()) != 1 {
		t.Errorf("list returned %d, want 1", len(list.GetItems()))
	}

	// Spec change bumps generation and resource_version.
	updatedSpec := got.GetProject()
	updatedSpec.Spec.DisplayName = "Demo 2"
	upd, err := client.UpdateProject(ctx, &apiv1.UpdateProjectRequest{Project: updatedSpec})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if g := upd.GetProject().GetMetadata().GetGeneration(); g != 2 {
		t.Errorf("generation = %d, want 2 after spec change", g)
	}
	if rv := upd.GetProject().GetMetadata().GetResourceVersion(); rv != "2" {
		t.Errorf("resource_version = %q, want 2", rv)
	}

	if _, err := client.DeleteProject(ctx, &apiv1.DeleteProjectRequest{Name: "demo"}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := client.GetProject(ctx, &apiv1.GetProjectRequest{Name: "demo"}); status.Code(err) != codes.NotFound {
		t.Errorf("Get after delete: code = %v, want NotFound", status.Code(err))
	}
}

func TestProjectCreateValidation(t *testing.T) {
	ctx := context.Background()
	client := apiv1.NewProjectServiceClient(newStoreConn(t))

	if _, err := client.CreateProject(ctx, &apiv1.CreateProjectRequest{Project: &apiv1.Project{}}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("missing name: code = %v, want InvalidArgument", status.Code(err))
	}

	if _, err := client.CreateProject(ctx, &apiv1.CreateProjectRequest{Project: project("dup", "")}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := client.CreateProject(ctx, &apiv1.CreateProjectRequest{Project: project("dup", "")}); status.Code(err) != codes.AlreadyExists {
		t.Errorf("duplicate: code = %v, want AlreadyExists", status.Code(err))
	}
}

func TestProjectUpdateConflict(t *testing.T) {
	ctx := context.Background()
	client := apiv1.NewProjectServiceClient(newStoreConn(t))

	created, err := client.CreateProject(ctx, &apiv1.CreateProjectRequest{Project: project("c", "C")})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	p := created.GetProject()
	p.Metadata.ResourceVersion = "1"
	if _, err := client.UpdateProject(ctx, &apiv1.UpdateProjectRequest{Project: p}); err != nil {
		t.Fatalf("first update: %v", err)
	}
	// p still carries the now-stale version "1".
	if _, err := client.UpdateProject(ctx, &apiv1.UpdateProjectRequest{Project: p}); status.Code(err) != codes.Aborted {
		t.Errorf("stale update: code = %v, want Aborted", status.Code(err))
	}
}

func TestWorkflowRequiresExistingProject(t *testing.T) {
	ctx := context.Background()
	client := apiv1.NewWorkflowServiceClient(newStoreConn(t))

	wf := &apiv1.Workflow{
		Metadata: &apiv1.ObjectMeta{Name: "build", Project: "ghost"},
		Spec:     &apiv1.WorkflowSpec{Goal: "ship it"},
	}
	if _, err := client.CreateWorkflow(ctx, &apiv1.CreateWorkflowRequest{Workflow: wf}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("workflow under missing project: code = %v, want FailedPrecondition", status.Code(err))
	}
}

func TestWorkflowCRUD(t *testing.T) {
	ctx := context.Background()
	conn := newStoreConn(t)
	projects := apiv1.NewProjectServiceClient(conn)
	workflows := apiv1.NewWorkflowServiceClient(conn)

	if _, err := projects.CreateProject(ctx, &apiv1.CreateProjectRequest{Project: project("app", "App")}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	wf := &apiv1.Workflow{
		Metadata: &apiv1.ObjectMeta{Name: "build", Project: "app"},
		Spec:     &apiv1.WorkflowSpec{Goal: "ship it"},
	}
	if _, err := workflows.CreateWorkflow(ctx, &apiv1.CreateWorkflowRequest{Workflow: wf}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	got, err := workflows.GetWorkflow(ctx, &apiv1.GetWorkflowRequest{Project: "app", Name: "build"})
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.GetWorkflow().GetSpec().GetGoal() != "ship it" {
		t.Errorf("goal = %q, want 'ship it'", got.GetWorkflow().GetSpec().GetGoal())
	}

	list, err := workflows.ListWorkflows(ctx, &apiv1.ListWorkflowsRequest{Project: "app"})
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(list.GetItems()) != 1 {
		t.Errorf("list returned %d, want 1", len(list.GetItems()))
	}

	if _, err := workflows.DeleteWorkflow(ctx, &apiv1.DeleteWorkflowRequest{Project: "app", Name: "build"}); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}
	if _, err := workflows.GetWorkflow(ctx, &apiv1.GetWorkflowRequest{Project: "app", Name: "build"}); status.Code(err) != codes.NotFound {
		t.Errorf("Get after delete: code = %v, want NotFound", status.Code(err))
	}
}
