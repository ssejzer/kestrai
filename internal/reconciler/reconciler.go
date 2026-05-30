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

// Package reconciler is the control-plane controller loop. In Phase 0 it
// reconciles Workflow desired state (spec) into actual state (status): it
// validates each Workflow and stamps Validated/Ready conditions plus
// observed_generation, the K8s-style convergence pattern.
//
// It is a skeleton in two deliberate ways (ADR-004): it polls the store on a
// resync interval rather than reacting to a NATS event stream (the event log
// lands in Phase 1), and it converges status only — Run execution arrives with
// the scheduler and agent runtime. The poll/reconcile/write shape is the final
// one; only the trigger source changes.
package reconciler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/auth"
	"github.com/kestrai/kestrai/internal/store"
)

// kindWorkflow matches the kind the API server stores Workflows under.
const kindWorkflow = "Workflow"

// defaultResyncInterval is how often the loop re-lists and reconciles. Short,
// because Phase 0 has no event stream to react to and the local dataset is tiny.
const defaultResyncInterval = time.Second

// Reconciler converges Workflow status against spec for one tenant.
type Reconciler struct {
	store    store.Store
	tenant   string
	interval time.Duration
	log      *slog.Logger
}

// Option configures a Reconciler.
type Option func(*Reconciler)

// WithInterval overrides the resync interval.
func WithInterval(d time.Duration) Option { return func(r *Reconciler) { r.interval = d } }

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option { return func(r *Reconciler) { r.log = l } }

// New returns a Reconciler over st. Phase 0 reconciles the default tenant only;
// multi-tenant fan-out arrives with tenant enumeration in a later phase.
func New(st store.Store, opts ...Option) *Reconciler {
	r := &Reconciler{
		store:    st,
		tenant:   auth.DefaultTenant,
		interval: defaultResyncInterval,
		log:      slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run reconciles once immediately, then on every resync tick, until ctx is
// cancelled. It returns ctx.Err() on shutdown.
func (r *Reconciler) Run(ctx context.Context) error {
	t := time.NewTicker(r.interval)
	defer t.Stop()

	r.reconcileAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			r.reconcileAll(ctx)
		}
	}
}

// reconcileAll lists and reconciles every Workflow in the tenant. Per-item
// errors are logged and swallowed so one bad object cannot stall the loop; the
// next resync retries.
func (r *Reconciler) reconcileAll(ctx context.Context) {
	recs, _, err := r.store.List(ctx, store.ListOptions{TenantID: r.tenant, Kind: kindWorkflow})
	if err != nil {
		r.log.ErrorContext(ctx, "reconciler: list workflows", "error", err)
		return
	}
	for _, rec := range recs {
		if err := r.reconcileWorkflow(ctx, rec); err != nil {
			r.log.ErrorContext(ctx, "reconciler: reconcile workflow",
				"workflow", rec.Project+"/"+rec.Name, "error", err)
		}
	}
}

// reconcileWorkflow computes the desired status for one Workflow and writes it
// back only when it changed, so a steady state produces no store churn.
func (r *Reconciler) reconcileWorkflow(ctx context.Context, rec store.Record) error {
	wf := &apiv1.Workflow{}
	if err := proto.Unmarshal(rec.Data, wf); err != nil {
		return err
	}
	if wf.Metadata == nil {
		wf.Metadata = &apiv1.ObjectMeta{}
	}

	desired := computeWorkflowStatus(wf.GetSpec(), wf.GetStatus(), rec.Generation)
	if proto.Equal(wf.GetStatus(), desired) {
		return nil // already converged
	}
	wf.Status = desired

	data, err := proto.Marshal(wf)
	if err != nil {
		return err
	}
	upd := rec // preserves Key, UID, Generation, ResourceVersion, Labels
	upd.Data = data

	_, err = r.store.Update(ctx, upd)
	switch {
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrNotFound):
		// Raced with an API write or a delete; the next resync reconciles the
		// fresh state. Not an error worth surfacing.
		return nil
	default:
		return err
	}
}

// computeWorkflowStatus derives the desired WorkflowStatus from the spec,
// carrying forward run counters and condition transition times from prev so a
// converged object compares equal across resyncs.
func computeWorkflowStatus(spec *apiv1.WorkflowSpec, prev *apiv1.WorkflowStatus, generation int64) *apiv1.WorkflowStatus {
	st := &apiv1.WorkflowStatus{ObservedGeneration: generation}
	if prev != nil {
		st.ActiveRunCount = prev.GetActiveRunCount()
		st.LastRunRef = prev.GetLastRunRef()
	}

	valid, reason, msg := validateWorkflowSpec(spec)
	st.Conditions = []*apiv1.Condition{
		condition(prev, "Validated", valid, reason, msg, generation),
	}
	if valid {
		st.Conditions = append(st.Conditions,
			condition(prev, "Ready", true, "WorkflowReady", "Workflow is valid and ready to run.", generation))
	} else {
		st.Conditions = append(st.Conditions,
			condition(prev, "Ready", false, "ValidationFailed", msg, generation))
	}
	return st
}

// validateWorkflowSpec reports whether a Workflow spec is runnable, with a
// CamelCase reason and human message when it is not.
func validateWorkflowSpec(spec *apiv1.WorkflowSpec) (ok bool, reason, message string) {
	switch {
	case spec == nil:
		return false, "MissingSpec", "Workflow has no spec."
	case spec.GetGoal() == "":
		return false, "MissingGoal", "spec.goal is required."
	case len(spec.GetPipeline()) == 0:
		return false, "EmptyPipeline", "spec.pipeline must declare at least one phase."
	}
	for _, phase := range spec.GetPipeline() {
		if phase.GetName() == "" {
			return false, "UnnamedPhase", "every pipeline phase requires a name."
		}
	}
	return true, "SpecValid", "Workflow spec validated."
}

// condition builds a status Condition, reusing the prior LastTransitionTime
// when the boolean state is unchanged (so transition times reflect real flips).
func condition(prev *apiv1.WorkflowStatus, condType string, ok bool, reason, message string, generation int64) *apiv1.Condition {
	status := "False"
	if ok {
		status = "True"
	}
	transition := timestamppb.Now()
	if prior := findCondition(prev, condType); prior != nil && prior.GetStatus() == status {
		transition = prior.GetLastTransitionTime()
	}
	return &apiv1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: transition,
		ObservedGeneration: generation,
	}
}

func findCondition(st *apiv1.WorkflowStatus, condType string) *apiv1.Condition {
	for _, c := range st.GetConditions() {
		if c.GetType() == condType {
			return c
		}
	}
	return nil
}
