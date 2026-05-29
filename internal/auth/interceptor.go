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
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authorizationHeader is the gRPC metadata key carrying the bearer token.
const authorizationHeader = "authorization"

// Interceptor authenticates incoming gRPC calls with a Provider and attaches
// the resulting Identity to the handler context. Methods listed as public
// skip authentication (e.g. health/version probes that must answer without a
// token).
type Interceptor struct {
	provider Provider
	public   map[string]bool
}

// NewInterceptor builds an Interceptor for provider. publicMethods are
// fully-qualified gRPC method names ("/kestrai.v1alpha1.SystemService/GetHealth")
// that bypass authentication.
func NewInterceptor(provider Provider, publicMethods ...string) *Interceptor {
	public := make(map[string]bool, len(publicMethods))
	for _, m := range publicMethods {
		public[m] = true
	}
	return &Interceptor{provider: provider, public: public}
}

// ServerOptions returns the gRPC server options that install the interceptor
// for both unary and streaming RPCs. Wire it as:
//
//	server.New(cfg, auth.NewInterceptor(p, publicMethods...).ServerOptions()...)
func (i *Interceptor) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(i.Unary()),
		grpc.ChainStreamInterceptor(i.Stream()),
	}
}

// Unary returns the unary server interceptor.
func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if i.public[info.FullMethod] {
			return handler(ctx, req)
		}
		ctx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// Stream returns the streaming server interceptor.
func (i *Interceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if i.public[info.FullMethod] {
			return handler(srv, ss)
		}
		ctx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &authStream{ServerStream: ss, ctx: ctx})
	}
}

// authenticate extracts credentials from ctx, runs the provider, and returns a
// child context carrying the Identity. On failure it returns a
// codes.Unauthenticated status error.
func (i *Interceptor) authenticate(ctx context.Context) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	creds := Credentials{
		Token:    bearerToken(md),
		Metadata: md,
	}
	id, err := i.provider.Authenticate(ctx, creds)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return ContextWithIdentity(ctx, id), nil
}

// bearerToken returns the token from an "authorization: Bearer <token>"
// header, or "" if absent. The scheme match is case-insensitive.
func bearerToken(md metadata.MD) string {
	vals := md.Get(authorizationHeader)
	if len(vals) == 0 {
		return ""
	}
	const prefix = "bearer "
	v := vals[0]
	if len(v) >= len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
		return strings.TrimSpace(v[len(prefix):])
	}
	return ""
}

// authStream wraps a grpc.ServerStream to override its context with one
// carrying the authenticated Identity.
type authStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authStream) Context() context.Context { return s.ctx }
