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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/store"
)

// projectService implements ProjectService CRUD over the state store. Project
// is tenant-global, so its store key uses an empty project segment.
type projectService struct {
	apiv1.UnimplementedProjectServiceServer

	store store.Store
}

func (s *projectService) CreateProject(ctx context.Context, req *apiv1.CreateProjectRequest) (*apiv1.CreateProjectResponse, error) {
	p := req.GetProject()
	if err := requireName(p.GetMetadata()); err != nil {
		return nil, err
	}
	tenant := resolveTenant(ctx)

	// Server owns identity and lifecycle fields: a Project is tenant-global,
	// starts at generation 1, and ignores any client-supplied status.
	p.Metadata.TenantId = tenant
	p.Metadata.Project = ""
	p.Metadata.Generation = 1
	p.Status = nil

	rec, err := marshalForStore(tenant, kindProject, p.Metadata, p)
	if err != nil {
		return nil, err
	}
	stored, err := s.store.Create(ctx, rec)
	if err != nil {
		return nil, storeErr(err)
	}
	overlayStoredMeta(p.Metadata, stored)
	return &apiv1.CreateProjectResponse{Project: p}, nil
}

func (s *projectService) GetProject(ctx context.Context, req *apiv1.GetProjectRequest) (*apiv1.GetProjectResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	rec, err := s.store.Get(ctx, store.Key{TenantID: resolveTenant(ctx), Kind: kindProject, Name: req.GetName()})
	if err != nil {
		return nil, storeErr(err)
	}
	p, err := decodeProject(rec)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetProjectResponse{Project: p}, nil
}

func (s *projectService) ListProjects(ctx context.Context, req *apiv1.ListProjectsRequest) (*apiv1.ListProjectsResponse, error) {
	selector, err := parseLabelSelector(req.GetLabelSelector())
	if err != nil {
		return nil, err
	}
	recs, token, err := s.store.List(ctx, store.ListOptions{
		TenantID:      resolveTenant(ctx),
		Kind:          kindProject,
		LabelSelector: selector,
		Limit:         int(req.GetPageSize()),
		Continue:      req.GetContinueToken(),
	})
	if err != nil {
		return nil, storeErr(err)
	}
	items := make([]*apiv1.Project, 0, len(recs))
	for _, rec := range recs {
		p, err := decodeProject(rec)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return &apiv1.ListProjectsResponse{
		Items:    items,
		Metadata: &apiv1.ListMeta{ContinueToken: token},
	}, nil
}

func (s *projectService) UpdateProject(ctx context.Context, req *apiv1.UpdateProjectRequest) (*apiv1.UpdateProjectResponse, error) {
	p := req.GetProject()
	if err := requireName(p.GetMetadata()); err != nil {
		return nil, err
	}
	if p.GetMetadata().GetResourceVersion() == "" {
		return nil, status.Error(codes.InvalidArgument, "metadata.resource_version is required for update")
	}
	tenant := resolveTenant(ctx)
	p.Metadata.TenantId = tenant
	p.Metadata.Project = ""

	key := store.Key{TenantID: tenant, Kind: kindProject, Name: p.Metadata.GetName()}
	current, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, storeErr(err)
	}
	cur, err := decodeProject(current)
	if err != nil {
		return nil, err
	}

	// generation bumps only when the spec changes (status-only writes do not).
	p.Metadata.Generation = current.Generation
	if !proto.Equal(cur.GetSpec(), p.GetSpec()) {
		p.Metadata.Generation = current.Generation + 1
	}

	rec, err := marshalForStore(tenant, kindProject, p.Metadata, p)
	if err != nil {
		return nil, err
	}
	rec.ResourceVersion = p.Metadata.GetResourceVersion()

	stored, err := s.store.Update(ctx, rec)
	if err != nil {
		return nil, storeErr(err)
	}
	overlayStoredMeta(p.Metadata, stored)
	return &apiv1.UpdateProjectResponse{Project: p}, nil
}

func (s *projectService) DeleteProject(ctx context.Context, req *apiv1.DeleteProjectRequest) (*apiv1.DeleteProjectResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	rec, err := s.store.Delete(ctx, store.Key{TenantID: resolveTenant(ctx), Kind: kindProject, Name: req.GetName()})
	if err != nil {
		return nil, storeErr(err)
	}
	p, err := decodeProject(rec)
	if err != nil {
		return nil, err
	}
	return &apiv1.DeleteProjectResponse{Project: p}, nil
}

// decodeProject unmarshals a stored record into a Project and overlays the
// store-authoritative metadata.
func decodeProject(rec store.Record) (*apiv1.Project, error) {
	p := &apiv1.Project{}
	if err := proto.Unmarshal(rec.Data, p); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal Project: %v", err)
	}
	if p.Metadata == nil {
		p.Metadata = &apiv1.ObjectMeta{}
	}
	overlayStoredMeta(p.Metadata, rec)
	return p, nil
}
