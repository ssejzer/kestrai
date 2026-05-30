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
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/auth"
	"github.com/kestrai/kestrai/internal/store"
)

// Kind names used as the store's resource discriminator. They match the YAML
// `kind:` and are stable across the v1alpha1 lifetime.
const (
	kindProject  = "Project"
	kindWorkflow = "Workflow"
)

// resolveTenant returns the tenant the request operates in. The authenticated
// Identity is authoritative (local-dev fills "default"); the tenant_id on the
// request/object is advisory and deliberately ignored, since the CLI exposes
// no --tenant flag in v1 (§12 tension #1).
func resolveTenant(ctx context.Context) string {
	if id, ok := auth.IdentityFromContext(ctx); ok && id.TenantID != "" {
		return id.TenantID
	}
	return auth.DefaultTenant
}

// storeErr maps a store sentinel error onto the matching gRPC status.
func storeErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, store.ErrConflict):
		// K8s returns 409 Conflict for a stale resourceVersion; Aborted is the
		// gRPC code clients should retry-after-reread.
		return status.Error(codes.Aborted, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// parseLabelSelector parses a "k1=v1,k2=v2" selector into an equality map.
// Empty input matches everything. Phase 0 supports equality only.
func parseLabelSelector(sel string) (map[string]string, error) {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil, nil
	}
	out := map[string]string{}
	for _, term := range strings.Split(sel, ",") {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		k, v, ok := strings.Cut(term, "=")
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "invalid label selector term %q: want key=value", term)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

// marshalForStore builds a store.Record for a resource message. meta must be
// the message's own *ObjectMeta (so its fields are already stamped); the
// returned record carries the marshaled body plus the indexed columns. The
// store assigns UID/ResourceVersion/CreatedAt.
func marshalForStore(tenantID, kind string, meta *apiv1.ObjectMeta, msg proto.Message) (store.Record, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return store.Record{}, status.Errorf(codes.Internal, "marshal %s: %v", kind, err)
	}
	return store.Record{
		Key: store.Key{
			TenantID: tenantID,
			Kind:     kind,
			Project:  meta.GetProject(),
			Name:     meta.GetName(),
		},
		Generation: meta.GetGeneration(),
		Labels:     meta.GetLabels(),
		Data:       data,
	}, nil
}

// overlayStoredMeta stamps the store-authoritative metadata from rec onto meta.
// Called after a write (to return assigned values) and after a read (the body
// blob's copy of these fields may be stale, e.g. resource_version).
func overlayStoredMeta(meta *apiv1.ObjectMeta, rec store.Record) {
	meta.TenantId = rec.TenantID
	meta.Project = rec.Key.Project
	meta.Name = rec.Name
	meta.Uid = rec.UID
	meta.ResourceVersion = rec.ResourceVersion
	meta.Generation = rec.Generation
	if !rec.CreatedAt.IsZero() {
		meta.CreationTimestamp = timestamppb.New(rec.CreatedAt)
	}
}

// requireName validates the metadata carries a name, returning InvalidArgument
// otherwise. It is the shared precondition for every write/lookup.
func requireName(meta *apiv1.ObjectMeta) error {
	if meta == nil || meta.GetName() == "" {
		return status.Error(codes.InvalidArgument, "metadata.name is required")
	}
	return nil
}
