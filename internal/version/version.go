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

// Package version holds build metadata shared by the CLI and the control
// plane. Version is injected at build time via -ldflags; everything else is
// a compile-time constant.
package version

// Version is the build version of the binary. It is overridden at build time
// via -ldflags "-X github.com/kestrai/kestrai/internal/version.Version=...".
// The default applies to `go run`/`go build` without the Makefile.
var Version = "0.0.0-dev"

// APIVersion is the API group/version this build speaks. It tracks the
// protobuf package (kestrai.v1alpha1) and is reported by SystemService.
const APIVersion = "kestrai.dev/v1alpha1"
