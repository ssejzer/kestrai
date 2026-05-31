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
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

func newGetCmd(cfg *clientConfig) *cobra.Command {
	var projectScope string
	cmd := &cobra.Command{
		Use:   "get KIND [NAME]",
		Short: "List resources, or show one by name",
		Long: "get lists resources of a kind (project, workflow), or shows a single resource when a name is given. " +
			"Default output is a table; use -o yaml or -o json for the full object.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := normalizeKind(args[0])
			if err != nil {
				return err
			}
			var name string
			if len(args) == 2 {
				name = args[1]
			}
			return runGet(cmd.Context(), cfg, kind, name, projectScope)
		},
	}
	cmd.Flags().StringVarP(&projectScope, "project", "p", "", "project scope (for workflows)")
	return cmd
}

func runGet(ctx context.Context, cfg *clientConfig, kind, name, projectScope string) error {
	c, err := cfg.connect()
	if err != nil {
		return err
	}
	defer c.close()

	switch kind {
	case kindProject:
		return getProjects(ctx, c, cfg.output, name)
	case kindWorkflow:
		return getWorkflows(ctx, c, cfg.output, name, projectScope)
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}
}

func getProjects(ctx context.Context, c *client, output, name string) error {
	cc, cancel := callContext(ctx)
	defer cancel()

	if name != "" {
		resp, err := c.projects.GetProject(cc, &apiv1.GetProjectRequest{Name: name})
		if err != nil {
			return err
		}
		return renderOne(kindProject, resp.GetProject(), output, projectRow, projectHeader)
	}

	resp, err := c.projects.ListProjects(cc, &apiv1.ListProjectsRequest{})
	if err != nil {
		return err
	}
	items := make([]proto.Message, 0, len(resp.GetItems()))
	for _, p := range resp.GetItems() {
		items = append(items, p)
	}
	return renderList(kindProject, items, output, projectRow, projectHeader)
}

func getWorkflows(ctx context.Context, c *client, output, name, projectScope string) error {
	cc, cancel := callContext(ctx)
	defer cancel()

	if name != "" {
		if projectScope == "" {
			return fmt.Errorf("-p/--project is required to get a single workflow")
		}
		resp, err := c.workflows.GetWorkflow(cc, &apiv1.GetWorkflowRequest{Project: projectScope, Name: name})
		if err != nil {
			return err
		}
		return renderOne(kindWorkflow, resp.GetWorkflow(), output, workflowRow, workflowHeader)
	}

	resp, err := c.workflows.ListWorkflows(cc, &apiv1.ListWorkflowsRequest{Project: projectScope})
	if err != nil {
		return err
	}
	items := make([]proto.Message, 0, len(resp.GetItems()))
	for _, w := range resp.GetItems() {
		items = append(items, w)
	}
	return renderList(kindWorkflow, items, output, workflowRow, workflowHeader)
}

// renderOne prints a single resource: encoded (yaml/json) when output is set,
// otherwise a one-row table.
func renderOne(kind string, msg proto.Message, output string, row func(proto.Message) []string, header []string) error {
	if output != "" {
		return encodeAndPrint(kind, msg, output)
	}
	return printTable(header, [][]string{row(msg)})
}

// renderList prints many resources: encoded list when output is set, otherwise
// a table. An empty table still prints a "No resources found." note.
func renderList(kind string, msgs []proto.Message, output string, row func(proto.Message) []string, header []string) error {
	if output != "" {
		for _, m := range msgs {
			if err := encodeAndPrint(kind, m, output); err != nil {
				return err
			}
		}
		return nil
	}
	if len(msgs) == 0 {
		dimf("No resources found.")
		return nil
	}
	rows := make([][]string, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, row(m))
	}
	return printTable(header, rows)
}

func encodeAndPrint(kind string, msg proto.Message, output string) error {
	out, err := encodeResource(kind, msg, output)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, strings.TrimRight(string(out), "\n"))
	return nil
}

var projectHeader = []string{"NAME", "DISPLAY", "TENANT", "AGE"}

func projectRow(msg proto.Message) []string {
	p := msg.(*apiv1.Project)
	m := p.GetMetadata()
	return []string{m.GetName(), p.GetSpec().GetDisplayName(), m.GetTenantId(), age(m)}
}

var workflowHeader = []string{"PROJECT", "NAME", "READY", "GOAL", "AGE"}

func workflowRow(msg proto.Message) []string {
	w := msg.(*apiv1.Workflow)
	m := w.GetMetadata()
	return []string{m.GetProject(), m.GetName(), conditionState(w.GetStatus()), truncate(w.GetSpec().GetGoal(), 40), age(m)}
}

// conditionState returns the Ready condition's status, or "-" if unset.
func conditionState(st *apiv1.WorkflowStatus) string {
	for _, c := range st.GetConditions() {
		if c.GetType() == "Ready" {
			return c.GetStatus()
		}
	}
	return "-"
}

func age(m *apiv1.ObjectMeta) string {
	if m.GetCreationTimestamp() == nil {
		return "-"
	}
	return durationShort(m.GetCreationTimestamp().AsTime())
}

func printTable(header []string, rows [][]string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	return w.Flush()
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func lower(s string) string { return strings.ToLower(s) }
