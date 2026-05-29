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
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestLocalDevAuthenticatesWithoutCredentials(t *testing.T) {
	p := NewLocalDev()
	if p.Name() != ProviderLocalDev {
		t.Errorf("Name = %q, want %q", p.Name(), ProviderLocalDev)
	}

	id, err := p.Authenticate(context.Background(), Credentials{})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if id.TenantID != DefaultTenant {
		t.Errorf("TenantID = %q, want %q", id.TenantID, DefaultTenant)
	}
	if id.Subject != ProviderLocalDev {
		t.Errorf("Subject = %q, want %q", id.Subject, ProviderLocalDev)
	}
	if id.Extra == nil {
		t.Error("Extra is nil, want non-nil")
	}
}

func TestNewStaticTokenRejectsEmptyToken(t *testing.T) {
	if _, err := NewStaticToken(""); err == nil {
		t.Fatal("NewStaticToken(\"\") = nil error, want error")
	}
}

func TestStaticTokenAuthenticate(t *testing.T) {
	p, err := NewStaticToken("s3cret")
	if err != nil {
		t.Fatalf("NewStaticToken: %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		id, err := p.Authenticate(context.Background(), Credentials{Token: "s3cret"})
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if id.TenantID != DefaultTenant {
			t.Errorf("TenantID = %q, want %q", id.TenantID, DefaultTenant)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		_, err := p.Authenticate(context.Background(), Credentials{Token: "nope"})
		if !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("err = %v, want wrapping ErrUnauthenticated", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := p.Authenticate(context.Background(), Credentials{})
		if !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("err = %v, want wrapping ErrUnauthenticated", err)
		}
	})
}

func TestStaticTokenWithTenant(t *testing.T) {
	p, err := NewStaticToken("tok", WithTenant("acme"))
	if err != nil {
		t.Fatalf("NewStaticToken: %v", err)
	}
	id, err := p.Authenticate(context.Background(), Credentials{Token: "tok"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if id.TenantID != "acme" {
		t.Errorf("TenantID = %q, want %q", id.TenantID, "acme")
	}
}

// identityHandler is a unary handler that returns the Identity from its
// context (or nil if absent), for asserting interceptor behavior.
func identityHandler(ctx context.Context, _ any) (any, error) {
	id, _ := IdentityFromContext(ctx)
	return id, nil
}

func TestInterceptorUnary(t *testing.T) {
	p, _ := NewStaticToken("s3cret")
	const private = "/kestrai.v1alpha1.ProjectService/GetProject"
	const public = "/kestrai.v1alpha1.SystemService/GetHealth"
	i := NewInterceptor(p, public)
	unary := i.Unary()

	bearer := func(tok string) context.Context {
		return metadata.NewIncomingContext(context.Background(),
			metadata.Pairs(authorizationHeader, "Bearer "+tok))
	}

	t.Run("valid token injects identity", func(t *testing.T) {
		resp, err := unary(bearer("s3cret"), nil, &grpc.UnaryServerInfo{FullMethod: private}, identityHandler)
		if err != nil {
			t.Fatalf("interceptor: %v", err)
		}
		id, ok := resp.(*Identity)
		if !ok || id == nil {
			t.Fatalf("handler got no identity, resp = %#v", resp)
		}
		if id.TenantID != DefaultTenant {
			t.Errorf("TenantID = %q, want %q", id.TenantID, DefaultTenant)
		}
	})

	t.Run("invalid token is Unauthenticated", func(t *testing.T) {
		_, err := unary(bearer("wrong"), nil, &grpc.UnaryServerInfo{FullMethod: private}, identityHandler)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", status.Code(err))
		}
	})

	t.Run("no metadata is Unauthenticated", func(t *testing.T) {
		_, err := unary(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: private}, identityHandler)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", status.Code(err))
		}
	})

	t.Run("public method bypasses auth", func(t *testing.T) {
		resp, err := unary(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: public}, identityHandler)
		if err != nil {
			t.Fatalf("interceptor: %v", err)
		}
		if resp != (*Identity)(nil) {
			t.Errorf("public method got identity %#v, want none", resp)
		}
	})

	t.Run("case-insensitive bearer scheme", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(),
			metadata.Pairs(authorizationHeader, "bEaReR s3cret"))
		if _, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: private}, identityHandler); err != nil {
			t.Errorf("interceptor: %v", err)
		}
	})
}

func TestInterceptorServerOptions(t *testing.T) {
	if got := len(NewInterceptor(NewLocalDev()).ServerOptions()); got != 2 {
		t.Errorf("ServerOptions len = %d, want 2", got)
	}
}
