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

// Package gen is the root of generated code in the Kestrai monorepo.
//
// Subpackages under this module hold the Go bindings produced from the
// Protobuf sources in //proto. They live in their own Go module so that
// regeneration churn does not pollute the main module's go.mod and so that
// downstream consumers can import the bindings without pulling in the rest
// of the control plane.
package gen
