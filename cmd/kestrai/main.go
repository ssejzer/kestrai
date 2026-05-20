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

// Command kestrai is the entry point for the Kestrai control plane and CLI.
//
// In Phase 0 this binary only prints its version. The real subcommands
// (`up`, `apply`, `get`, `describe`, `delete`, `logs`, `init`, `doctor`,
// `version`, `explain`) are wired in subsequent commits.
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	fmt.Fprintf(os.Stderr, "kestrai %s — Phase 0 scaffold. Subcommands not wired yet.\n", version)
	os.Exit(0)
}
