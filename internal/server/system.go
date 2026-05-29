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

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

// systemService implements SystemService: liveness and build metadata. It
// holds no external dependencies so the CLI's `version` and `doctor` verbs
// can reach it before the store or reconciler are up.
type systemService struct {
	apiv1.UnimplementedSystemServiceServer

	version    string
	apiVersion string
}

// GetHealth reports SERVING once the gRPC server is accepting requests.
// Phase 0 has no degraded states; readiness gains depth (store, NATS) later.
func (s *systemService) GetHealth(context.Context, *apiv1.GetHealthRequest) (*apiv1.GetHealthResponse, error) {
	return &apiv1.GetHealthResponse{
		Status: apiv1.GetHealthResponse_SERVING_STATUS_SERVING,
	}, nil
}

// GetVersion returns the control plane build and API version.
func (s *systemService) GetVersion(context.Context, *apiv1.GetVersionRequest) (*apiv1.GetVersionResponse, error) {
	return &apiv1.GetVersionResponse{
		Version:    s.version,
		ApiVersion: s.apiVersion,
	}, nil
}
