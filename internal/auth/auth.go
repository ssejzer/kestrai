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

// Package auth defines the control-plane authentication seam. A Provider
// turns the credentials carried on an incoming request into an Identity; the
// gRPC interceptors in this package run that provider and stash the resulting
// Identity on the request context.
//
// Phase 0 ships two reference providers — local-dev (no auth) and
// static-token (one shared bearer token). The interface is deliberately wide
// enough that OIDC/SAML providers (Phase 4) drop in without changing it: they
// read JWTs/assertions from Credentials.Token or Credentials.Metadata. See
// §12 design tension #2 of the canonical spec.
package auth

import (
	"context"
	"errors"
)

// DefaultTenant is the tenant every request resolves to in single-tenant
// deployments. local-dev always uses it; the CLI exposes no --tenant flag in
// v1. (Pattern: the Kubernetes "default" namespace.)
const DefaultTenant = "default"

// ErrUnauthenticated is returned (or wrapped) by a Provider when credentials
// are missing or invalid. The interceptor maps it to codes.Unauthenticated.
var ErrUnauthenticated = errors.New("authentication failed")

// Identity is the authenticated principal for a request. It is attached to
// the request context by the interceptor and read by handlers (e.g. to scope
// queries to TenantID).
type Identity struct {
	// Subject identifies the caller: a user id, service account, or a fixed
	// label for the no-auth providers ("local-dev", "static-token").
	Subject string

	// TenantID scopes every tenant-scoped resource the request touches.
	// Resolves to DefaultTenant in single-tenant deployments.
	TenantID string

	// Groups carries role/group memberships for the Phase 4 RBAC layer.
	// Empty for the Phase 0 providers.
	Groups []string

	// Extra holds provider-specific claims (e.g. OIDC token claims) that do
	// not map onto a typed field. Never nil after Authenticate.
	Extra map[string]string
}

// Credentials is the raw authentication material the interceptor extracts
// from a request, in a transport-neutral form so out-of-process Provider
// plugins (Phase 4) can receive the same shape.
type Credentials struct {
	// Token is the bearer token from "authorization: Bearer <token>", if any.
	Token string

	// Metadata is the full set of request headers with lowercased keys, for
	// providers that need more than a bearer token (mTLS peer headers, custom
	// schemes). May be nil.
	Metadata map[string][]string
}

// Provider authenticates request credentials into an Identity.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns a stable identifier: "local-dev", "static-token", "oidc", …
	Name() string

	// Authenticate validates creds and returns the caller's Identity. On
	// missing or invalid credentials it returns an error wrapping
	// ErrUnauthenticated.
	Authenticate(ctx context.Context, creds Credentials) (*Identity, error)
}
