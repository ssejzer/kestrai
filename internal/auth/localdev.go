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

import "context"

// ProviderLocalDev is the Name of the no-auth provider.
const ProviderLocalDev = "local-dev"

// LocalDev is the zero-config provider used by `kestrai up`. It requires no
// credentials and authenticates every request as a single local user in the
// default tenant, so hobbyists never see a token prompt.
type LocalDev struct{}

// NewLocalDev returns the no-auth provider.
func NewLocalDev() *LocalDev { return &LocalDev{} }

// Name implements Provider.
func (*LocalDev) Name() string { return ProviderLocalDev }

// Authenticate always succeeds, ignoring creds, and returns the fixed local
// identity scoped to the default tenant.
func (*LocalDev) Authenticate(context.Context, Credentials) (*Identity, error) {
	return &Identity{
		Subject:  ProviderLocalDev,
		TenantID: DefaultTenant,
		Extra:    map[string]string{},
	}, nil
}
