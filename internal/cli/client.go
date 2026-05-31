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

package cli

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

// client bundles the gRPC connection and the per-service stubs the resource
// verbs use. Phase 0 dials plaintext (local-dev); TLS and a real credential
// flow arrive with the static-token/OIDC client work.
type client struct {
	conn      *grpc.ClientConn
	system    apiv1.SystemServiceClient
	projects  apiv1.ProjectServiceClient
	workflows apiv1.WorkflowServiceClient
}

// dial connects to the control plane at address. When token is non-empty it is
// attached as a bearer token on every call (for a static-token server); local
// dev needs no token.
func dial(address, token string) (*client, error) {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearerCreds{token: token}))
	}
	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}
	return &client{
		conn:      conn,
		system:    apiv1.NewSystemServiceClient(conn),
		projects:  apiv1.NewProjectServiceClient(conn),
		workflows: apiv1.NewWorkflowServiceClient(conn),
	}, nil
}

func (c *client) close() error { return c.conn.Close() }

// callContext returns a context with the default per-call deadline.
func callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 10*time.Second)
}

// bearerCreds attaches a bearer token to outgoing requests over an insecure
// transport. RequireTransportSecurity is false because Phase 0 dials plaintext
// against a local server; this tightens up with TLS later.
type bearerCreds struct{ token string }

func (b bearerCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

func (bearerCreds) RequireTransportSecurity() bool { return false }
