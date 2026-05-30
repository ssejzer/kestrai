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
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// serverRoles are the single-responsibility processes the split deployment
// mode will run. Phase 0 recognizes them but does not wire them — split mode
// (external NATS + Postgres) lands in Phase 1; see §12 tension #3.
var serverRoles = map[string]string{
	"api":           "gRPC API server only",
	"reconciler":    "controller loop only",
	"scheduler":     "task scheduler only",
	"agent-runtime": "agent execution workers only",
}

func newServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "server <role>",
		Short:     "Run a single control-plane role (split-process mode)",
		Long:      "server runs one role of the control plane as its own process. Phase 0 stub: use `kestrai up` for the all-in-one process; split mode arrives in Phase 1.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: roleNames(),
		RunE: func(cmd *cobra.Command, args []string) error {
			role := args[0]
			if _, ok := serverRoles[role]; !ok {
				return fmt.Errorf("unknown role %q; valid roles: %s", role, strings.Join(roleNames(), ", "))
			}
			dimf("split-process mode (server %s) is not wired until Phase 1.", role)
			dimf("for local development run: kestrai up")
			return nil
		},
	}
}

func roleNames() []string {
	names := make([]string, 0, len(serverRoles))
	for r := range serverRoles {
		names = append(names, r)
	}
	sort.Strings(names)
	return names
}
