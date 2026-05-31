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

// Package cli is the `kestrai` command tree (Cobra). Phase 0 wires the
// process-lifecycle verbs — `up`, `server`, `version`; the resource verbs
// (`apply`, `get`, …) land in the next CLI PR.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/kestrai/kestrai/internal/version"
)

const (
	// DefaultAPIAddress is where `kestrai up` serves gRPC and the resource
	// verbs will dial by default.
	DefaultAPIAddress = "127.0.0.1:8585"

	// EnvAPIAddress overrides the API address without a flag.
	EnvAPIAddress = "KESTRAI_ADDRESS"
	// EnvDBPath overrides the SQLite path without a flag.
	EnvDBPath = "KESTRAI_DB_PATH"
	// EnvToken supplies a bearer token for a static-token server.
	EnvToken = "KESTRAI_TOKEN"
)

// newRootCmd assembles the command tree.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kestrai",
		Short:         "Kestrai — the Kubernetes of AI agents",
		Long:          "Kestrai is a declarative control plane that reconciles YAML specs (Project, Workflow, …) into running AI-agent work.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cfg := &clientConfig{}
	cfg.bindFlags(root.PersistentFlags())
	root.AddCommand(newUpCmd(), newServerCmd(), newVersionCmd(), newApplyCmd(cfg), newGetCmd(cfg))
	return root
}

// Execute runs the command tree and returns the process exit code. main keeps
// the os.Exit call so deferred cleanup in subcommands still runs.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		printError(err)
		return 1
	}
	return 0
}
