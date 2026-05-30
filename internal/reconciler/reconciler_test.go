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

package reconciler

import (
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/auth"
	"github.com/kestrai/kestrai/internal/store"
)

func openStore(t *testing.T) *store.SQLite {
	t.Helper()
	st, err := store.Open(context.Background(), "file:"+filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// seedWorkflow stores a Workflow (marshaled like the API server would) and
// returns its store key.
func seedWorkflow(t *testing.T, st *store.SQLite, name string, spec *apiv1.WorkflowSpec) store.Key {
	t.Helper()
	wf := &apiv1.Workflow{
		Metadata: &apiv1.ObjectMeta{Name: name, Project: "app", TenantId: auth.DefaultTenant},
		Spec:     spec,
	}
	data, err := proto.Marshal(wf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	key := store.Key{TenantID: auth.DefaultTenant, Kind: kindWorkflow, Project: "app", Name: name}
	if _, err := st.Create(context.Background(), store.Record{Key: key, Generation: 1, Data: data}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	return key
}

func getWorkflow(t *testing.T, st *store.SQLite, key store.Key) *apiv1.Workflow {
	t.Helper()
	rec, err := st.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	wf := &apiv1.Workflow{}
	if err := proto.Unmarshal(rec.Data, wf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return wf
}

func conditionStatus(wf *apiv1.Workflow, condType string) string {
	if c := findCondition(wf.GetStatus(), condType); c != nil {
		return c.GetStatus()
	}
	return ""
}

func TestReconcileValidWorkflow(t *testing.T) {
	st := openStore(t)
	key := seedWorkflow(t, st, "build", &apiv1.WorkflowSpec{
		Goal:     "ship it",
		Pipeline: []*apiv1.WorkflowPhase{{Name: "execute"}},
	})

	New(st).reconcileAll(context.Background())

	wf := getWorkflow(t, st, key)
	if got := wf.GetStatus().GetObservedGeneration(); got != 1 {
		t.Errorf("observed_generation = %d, want 1", got)
	}
	if s := conditionStatus(wf, "Validated"); s != "True" {
		t.Errorf("Validated = %q, want True", s)
	}
	if s := conditionStatus(wf, "Ready"); s != "True" {
		t.Errorf("Ready = %q, want True", s)
	}
}

func TestReconcileInvalidWorkflow(t *testing.T) {
	st := openStore(t)
	key := seedWorkflow(t, st, "broken", &apiv1.WorkflowSpec{Goal: ""}) // missing goal + pipeline

	New(st).reconcileAll(context.Background())

	wf := getWorkflow(t, st, key)
	if s := conditionStatus(wf, "Validated"); s != "False" {
		t.Errorf("Validated = %q, want False", s)
	}
	if s := conditionStatus(wf, "Ready"); s != "False" {
		t.Errorf("Ready = %q, want False", s)
	}
	if r := findCondition(wf.GetStatus(), "Validated").GetReason(); r != "MissingGoal" {
		t.Errorf("reason = %q, want MissingGoal", r)
	}
}

// TestReconcileIsIdempotent verifies a converged Workflow is not rewritten on
// the next pass (no resource_version churn).
func TestReconcileIsIdempotent(t *testing.T) {
	st := openStore(t)
	key := seedWorkflow(t, st, "build", &apiv1.WorkflowSpec{
		Goal:     "ship it",
		Pipeline: []*apiv1.WorkflowPhase{{Name: "execute"}},
	})
	r := New(st)

	r.reconcileAll(context.Background())
	first, err := st.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	r.reconcileAll(context.Background())
	second, err := st.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if first.ResourceVersion != second.ResourceVersion {
		t.Errorf("resource_version changed on no-op reconcile: %s -> %s",
			first.ResourceVersion, second.ResourceVersion)
	}
}
