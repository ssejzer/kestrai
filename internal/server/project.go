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

// projectService implements ProjectService CRUD. The embedded Unimplemented
// type makes every RPC return codes.Unimplemented until the SQLite state
// store is wired in (Phase 0 backlog: state store + migrations). Handler
// methods are filled in then; the type exists now so the surface is stable
// and registration is exercised by tests.
type projectService struct {
	apiv1.UnimplementedProjectServiceServer
}
