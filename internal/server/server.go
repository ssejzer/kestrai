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

// Package server hosts the Kestrai control-plane gRPC API. In Phase 0 it
// serves SystemService (health + version) fully; ProjectService and
// WorkflowService are registered but return Unimplemented until the SQLite
// state store lands. The same Server runs inside `kestrai up` (in-process,
// alongside the reconciler) and `kestrai server api` (standalone).
package server

import (
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/store"
	"github.com/kestrai/kestrai/internal/version"
)

// Config tunes a Server. The zero value serves SystemService only; Project and
// Workflow CRUD require a non-nil Store.
type Config struct {
	// Version reported by SystemService.GetVersion. Defaults to the build
	// version injected via -ldflags.
	Version string

	// APIVersion reported by SystemService.GetVersion. Defaults to the
	// API group/version this build speaks.
	APIVersion string

	// Store backs Project/Workflow CRUD. When nil those services answer
	// codes.Unimplemented via the embedded Unimplemented* types.
	Store store.Store
}

// Server wraps a configured gRPC server with the Kestrai services registered.
type Server struct {
	grpc *grpc.Server
}

// New builds a Server with every v1alpha1 service registered. Extra
// grpc.ServerOptions (interceptors, credentials) are appended as-is, so the
// Phase 3 AuthProvider interceptor can be threaded through without changing
// this signature.
func New(cfg Config, opts ...grpc.ServerOption) *Server {
	if cfg.Version == "" {
		cfg.Version = version.Version
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = version.APIVersion
	}

	g := grpc.NewServer(opts...)
	apiv1.RegisterSystemServiceServer(g, &systemService{version: cfg.Version, apiVersion: cfg.APIVersion})
	if cfg.Store != nil {
		apiv1.RegisterProjectServiceServer(g, &projectService{store: cfg.Store})
		apiv1.RegisterWorkflowServiceServer(g, &workflowService{store: cfg.Store})
	} else {
		// No store wired (e.g. system-only deployments): CRUD answers
		// codes.Unimplemented rather than dereferencing a nil store.
		apiv1.RegisterProjectServiceServer(g, apiv1.UnimplementedProjectServiceServer{})
		apiv1.RegisterWorkflowServiceServer(g, apiv1.UnimplementedWorkflowServiceServer{})
	}

	// Reflection lets grpcurl and `kestrai doctor` introspect the surface
	// without a compiled descriptor set.
	reflection.Register(g)

	return &Server{grpc: g}
}

// GRPC exposes the underlying *grpc.Server for callers that need to multiplex
// it (e.g. cmux) or register additional services. Most callers use Serve.
func (s *Server) GRPC() *grpc.Server { return s.grpc }

// Serve blocks, accepting connections on lis until GracefulStop or Stop is
// called. It returns nil on a clean stop.
func (s *Server) Serve(lis net.Listener) error {
	return s.grpc.Serve(lis)
}

// GracefulStop stops the server after in-flight RPCs drain.
func (s *Server) GracefulStop() { s.grpc.GracefulStop() }

// Stop stops the server immediately, cancelling in-flight RPCs.
func (s *Server) Stop() { s.grpc.Stop() }
