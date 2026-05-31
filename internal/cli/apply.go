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
	"io"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

func newApplyCmd(cfg *clientConfig) *cobra.Command {
	var filename string
	cmd := &cobra.Command{
		Use:   "apply -f FILE",
		Short: "Create or update resources from a YAML file",
		Long: "apply reads one or more YAML documents (separated by ---) and creates each resource, " +
			"or updates it in place if it already exists. Use - to read from stdin.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filename == "" {
				return fmt.Errorf("the -f/--file flag is required")
			}
			return runApply(cmd.Context(), cfg, filename)
		},
	}
	cmd.Flags().StringVarP(&filename, "file", "f", "", "path to a YAML file, or - for stdin")
	return cmd
}

func runApply(ctx context.Context, cfg *clientConfig, filename string) error {
	in, closeFn, err := openInput(filename)
	if err != nil {
		return err
	}
	defer closeFn()

	resources, err := decodeResources(in)
	if err != nil {
		return err
	}

	c, err := cfg.connect()
	if err != nil {
		return err
	}
	defer c.close()

	for _, res := range resources {
		verb, err := applyResource(ctx, c, res)
		if err != nil {
			return fmt.Errorf("apply %s/%s: %w", res.Kind, res.Name, err)
		}
		statusf("%s/%s %s", lower(res.Kind), res.Name, verb)
	}
	return nil
}

// applyResource creates the resource, or updates it if it already exists. It
// returns the verb performed ("created" or "configured") for status output.
func applyResource(ctx context.Context, c *client, res resource) (string, error) {
	switch res.Kind {
	case kindProject:
		return applyProject(ctx, c, res.Message.(*apiv1.Project))
	case kindWorkflow:
		return applyWorkflow(ctx, c, res.Message.(*apiv1.Workflow))
	default:
		return "", fmt.Errorf("unsupported kind %q", res.Kind)
	}
}

func applyProject(ctx context.Context, c *client, p *apiv1.Project) (string, error) {
	cc, cancel := callContext(ctx)
	defer cancel()

	current, err := c.projects.GetProject(cc, &apiv1.GetProjectRequest{Name: p.GetMetadata().GetName()})
	if status.Code(err) == codes.NotFound {
		if _, err := c.projects.CreateProject(cc, &apiv1.CreateProjectRequest{Project: p}); err != nil {
			return "", err
		}
		return "created", nil
	}
	if err != nil {
		return "", err
	}

	// Carry the stored resource_version so the optimistic-concurrency check passes.
	p.Metadata.ResourceVersion = current.GetProject().GetMetadata().GetResourceVersion()
	if _, err := c.projects.UpdateProject(cc, &apiv1.UpdateProjectRequest{Project: p}); err != nil {
		return "", err
	}
	return "configured", nil
}

func applyWorkflow(ctx context.Context, c *client, w *apiv1.Workflow) (string, error) {
	cc, cancel := callContext(ctx)
	defer cancel()

	current, err := c.workflows.GetWorkflow(cc, &apiv1.GetWorkflowRequest{
		Project: w.GetMetadata().GetProject(),
		Name:    w.GetMetadata().GetName(),
	})
	if status.Code(err) == codes.NotFound {
		if _, err := c.workflows.CreateWorkflow(cc, &apiv1.CreateWorkflowRequest{Workflow: w}); err != nil {
			return "", err
		}
		return "created", nil
	}
	if err != nil {
		return "", err
	}

	w.Metadata.ResourceVersion = current.GetWorkflow().GetMetadata().GetResourceVersion()
	if _, err := c.workflows.UpdateWorkflow(cc, &apiv1.UpdateWorkflowRequest{Workflow: w}); err != nil {
		return "", err
	}
	return "configured", nil
}

// openInput opens filename for reading, or stdin when filename is "-". The
// returned close func is always safe to call.
func openInput(filename string) (io.Reader, func(), error) {
	if filename == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open %s: %w", filename, err)
	}
	return f, func() { f.Close() }, nil
}
