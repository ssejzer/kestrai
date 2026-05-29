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

package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
)

// ProviderStaticToken is the Name of the shared-bearer-token provider.
const ProviderStaticToken = "static-token"

// StaticToken authenticates against one shared bearer token. It is the
// reference provider for self-hosted, multi-user-but-single-tenant
// deployments: everyone presents the same token and shares the default
// tenant. It is not a substitute for per-user auth (OIDC, Phase 4).
type StaticToken struct {
	token    []byte
	tenantID string
}

// StaticTokenOption configures a StaticToken provider.
type StaticTokenOption func(*StaticToken)

// WithTenant overrides the tenant identities are scoped to. Defaults to
// DefaultTenant.
func WithTenant(tenantID string) StaticTokenOption {
	return func(s *StaticToken) { s.tenantID = tenantID }
}

// NewStaticToken returns a provider that accepts exactly token. It returns an
// error if token is empty, so a misconfigured server fails fast at startup
// rather than silently accepting empty bearer tokens.
func NewStaticToken(token string, opts ...StaticTokenOption) (*StaticToken, error) {
	if token == "" {
		return nil, fmt.Errorf("auth: static-token provider requires a non-empty token")
	}
	s := &StaticToken{token: []byte(token), tenantID: DefaultTenant}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Name implements Provider.
func (*StaticToken) Name() string { return ProviderStaticToken }

// Authenticate accepts the request when creds.Token matches the configured
// token (compared in constant time) and rejects it otherwise.
func (s *StaticToken) Authenticate(_ context.Context, creds Credentials) (*Identity, error) {
	if creds.Token == "" {
		return nil, fmt.Errorf("%w: missing bearer token", ErrUnauthenticated)
	}
	if subtle.ConstantTimeCompare([]byte(creds.Token), s.token) != 1 {
		return nil, fmt.Errorf("%w: invalid bearer token", ErrUnauthenticated)
	}
	return &Identity{
		Subject:  ProviderStaticToken,
		TenantID: s.tenantID,
		Extra:    map[string]string{},
	}, nil
}
