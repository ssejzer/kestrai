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
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/store"
)

// workflowService implements WorkflowService CRUD over the state store.
// Workflow is project-scoped, so every key carries metadata.project.
type workflowService struct {
	apiv1.UnimplementedWorkflowServiceServer

	store store.Store
}

func (s *workflowService) CreateWorkflow(ctx context.Context, req *apiv1.CreateWorkflowRequest) (*apiv1.CreateWorkflowResponse, error) {
	w := req.GetWorkflow()
	if err := requireName(w.GetMetadata()); err != nil {
		return nil, err
	}
	if w.GetMetadata().GetProject() == "" {
		return nil, status.Error(codes.InvalidArgument, "metadata.project is required for a Workflow")
	}
	tenant := resolveTenant(ctx)

	// The owning Project must exist (Workflow is project-scoped).
	if _, err := s.store.Get(ctx, store.Key{TenantID: tenant, Kind: kindProject, Name: w.Metadata.GetProject()}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.FailedPrecondition, "project %q does not exist", w.Metadata.GetProject())
		}
		return nil, storeErr(err)
	}

	w.Metadata.TenantId = tenant
	w.Metadata.Generation = 1
	w.Status = nil

	rec, err := marshalForStore(tenant, kindWorkflow, w.Metadata, w)
	if err != nil {
		return nil, err
	}
	stored, err := s.store.Create(ctx, rec)
	if err != nil {
		return nil, storeErr(err)
	}
	overlayStoredMeta(w.Metadata, stored)
	return &apiv1.CreateWorkflowResponse{Workflow: w}, nil
}

func (s *workflowService) GetWorkflow(ctx context.Context, req *apiv1.GetWorkflowRequest) (*apiv1.GetWorkflowResponse, error) {
	if req.GetProject() == "" || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "project and name are required")
	}
	rec, err := s.store.Get(ctx, store.Key{TenantID: resolveTenant(ctx), Kind: kindWorkflow, Project: req.GetProject(), Name: req.GetName()})
	if err != nil {
		return nil, storeErr(err)
	}
	w, err := decodeWorkflow(rec)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetWorkflowResponse{Workflow: w}, nil
}

func (s *workflowService) ListWorkflows(ctx context.Context, req *apiv1.ListWorkflowsRequest) (*apiv1.ListWorkflowsResponse, error) {
	selector, err := parseLabelSelector(req.GetLabelSelector())
	if err != nil {
		return nil, err
	}
	recs, token, err := s.store.List(ctx, store.ListOptions{
		TenantID:      resolveTenant(ctx),
		Kind:          kindWorkflow,
		Project:       req.GetProject(), // "" lists across all projects in the tenant
		LabelSelector: selector,
		Limit:         int(req.GetPageSize()),
		Continue:      req.GetContinueToken(),
	})
	if err != nil {
		return nil, storeErr(err)
	}
	items := make([]*apiv1.Workflow, 0, len(recs))
	for _, rec := range recs {
		w, err := decodeWorkflow(rec)
		if err != nil {
			return nil, err
		}
		items = append(items, w)
	}
	return &apiv1.ListWorkflowsResponse{
		Items:    items,
		Metadata: &apiv1.ListMeta{ContinueToken: token},
	}, nil
}

func (s *workflowService) UpdateWorkflow(ctx context.Context, req *apiv1.UpdateWorkflowRequest) (*apiv1.UpdateWorkflowResponse, error) {
	w := req.GetWorkflow()
	if err := requireName(w.GetMetadata()); err != nil {
		return nil, err
	}
	if w.GetMetadata().GetProject() == "" {
		return nil, status.Error(codes.InvalidArgument, "metadata.project is required for a Workflow")
	}
	if w.GetMetadata().GetResourceVersion() == "" {
		return nil, status.Error(codes.InvalidArgument, "metadata.resource_version is required for update")
	}
	tenant := resolveTenant(ctx)
	w.Metadata.TenantId = tenant

	key := store.Key{TenantID: tenant, Kind: kindWorkflow, Project: w.Metadata.GetProject(), Name: w.Metadata.GetName()}
	current, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, storeErr(err)
	}
	cur, err := decodeWorkflow(current)
	if err != nil {
		return nil, err
	}

	w.Metadata.Generation = current.Generation
	if !proto.Equal(cur.GetSpec(), w.GetSpec()) {
		w.Metadata.Generation = current.Generation + 1
	}

	rec, err := marshalForStore(tenant, kindWorkflow, w.Metadata, w)
	if err != nil {
		return nil, err
	}
	rec.ResourceVersion = w.Metadata.GetResourceVersion()

	stored, err := s.store.Update(ctx, rec)
	if err != nil {
		return nil, storeErr(err)
	}
	overlayStoredMeta(w.Metadata, stored)
	return &apiv1.UpdateWorkflowResponse{Workflow: w}, nil
}

func (s *workflowService) DeleteWorkflow(ctx context.Context, req *apiv1.DeleteWorkflowRequest) (*apiv1.DeleteWorkflowResponse, error) {
	if req.GetProject() == "" || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "project and name are required")
	}
	rec, err := s.store.Delete(ctx, store.Key{TenantID: resolveTenant(ctx), Kind: kindWorkflow, Project: req.GetProject(), Name: req.GetName()})
	if err != nil {
		return nil, storeErr(err)
	}
	w, err := decodeWorkflow(rec)
	if err != nil {
		return nil, err
	}
	return &apiv1.DeleteWorkflowResponse{Workflow: w}, nil
}

// decodeWorkflow unmarshals a stored record into a Workflow and overlays the
// store-authoritative metadata.
func decodeWorkflow(rec store.Record) (*apiv1.Workflow, error) {
	w := &apiv1.Workflow{}
	if err := proto.Unmarshal(rec.Data, w); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal Workflow: %v", err)
	}
	if w.Metadata == nil {
		w.Metadata = &apiv1.ObjectMeta{}
	}
	overlayStoredMeta(w.Metadata, rec)
	return w, nil
}
