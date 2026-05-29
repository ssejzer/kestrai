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
	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

// workflowService implements WorkflowService CRUD. Like projectService, the
// embedded Unimplemented type returns codes.Unimplemented for every RPC until
// the state store lands; the type is registered now to keep the API surface
// and reflection output stable.
type workflowService struct {
	apiv1.UnimplementedWorkflowServiceServer
}
