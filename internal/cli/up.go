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
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/auth"
	"github.com/kestrai/kestrai/internal/reconciler"
	"github.com/kestrai/kestrai/internal/server"
	"github.com/kestrai/kestrai/internal/store"
)

func newUpCmd() *cobra.Command {
	var address, dbPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Run the full control plane in one process (SQLite, local-dev auth)",
		Long: "up starts the API server, reconciler, and SQLite state store in a single process " +
			"with no auth required — the zero-config path for local development.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUp(cmd.Context(), address, dbPath)
		},
	}
	cmd.Flags().StringVar(&address, "address", envOr(EnvAPIAddress, DefaultAPIAddress), "gRPC listen address")
	cmd.Flags().StringVar(&dbPath, "db-path", envOr(EnvDBPath, defaultDBPath()), "SQLite database path")
	return cmd
}

func runUp(ctx context.Context, address, dbPath string) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}
	st, err := store.Open(ctx, "file:"+dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	statusf("state store ready at %s", dbPath)

	// local-dev auth: no token required; health/version stay public anyway.
	interceptor := auth.NewInterceptor(auth.NewLocalDev(), systemMethods()...)
	srv := server.New(server.Config{Store: st}, interceptor.ServerOptions()...)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}

	rec := reconciler.New(st)
	go rec.Run(ctx) //nolint:errcheck // returns ctx.Err() on shutdown

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	statusf("API serving on %s (auth: local-dev)", address)
	statusf("reconciler running")
	dimf("press Ctrl-C to stop")

	select {
	case <-ctx.Done():
		dimf("shutting down…")
		srv.GracefulStop()
		return nil
	case err := <-serveErr:
		return fmt.Errorf("api server: %w", err)
	}
}

// systemMethods returns the fully-qualified SystemService method names so the
// auth interceptor can leave health/version unauthenticated.
func systemMethods() []string {
	desc := apiv1.SystemService_ServiceDesc
	out := make([]string, 0, len(desc.Methods))
	for _, m := range desc.Methods {
		out = append(out, "/"+desc.ServiceName+"/"+m.MethodName)
	}
	return out
}

// defaultDBPath is ~/.kestrai/state.db, falling back to the working directory
// when the home directory cannot be resolved.
func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "kestrai-state.db"
	}
	return filepath.Join(home, ".kestrai", "state.db")
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
