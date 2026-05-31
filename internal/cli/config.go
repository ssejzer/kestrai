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

import "github.com/spf13/pflag"

// Output formats for the resource verbs.
const (
	outputYAML = "yaml"
	outputJSON = "json"
)

// clientConfig holds the connection settings shared by the resource verbs,
// populated from persistent flags (with KESTRAI_* env defaults). It is created
// once on the root command and passed to each verb's constructor.
type clientConfig struct {
	address string
	token   string
	output  string
}

// bindFlags registers the persistent flags onto fs. Env vars supply the
// defaults so the common local-dev case needs no flags at all.
func (c *clientConfig) bindFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.address, "address", envOr(EnvAPIAddress, DefaultAPIAddress), "control-plane gRPC address")
	fs.StringVar(&c.token, "token", envOr(EnvToken, ""), "bearer token (static-token auth; unset for local-dev)")
	fs.StringVarP(&c.output, "output", "o", "", "output format: table (default), yaml, or json")
}

// connect dials the control plane using the resolved settings.
func (c *clientConfig) connect() (*client, error) {
	return dial(c.address, c.token)
}
