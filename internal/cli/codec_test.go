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
	"strings"
	"testing"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
)

const sampleYAML = `
apiVersion: kestrai.dev/v1alpha1
kind: Project
metadata:
  name: demo
  labels:
    team: core
spec:
  displayName: Demo Project
---
apiVersion: kestrai.dev/v1alpha1
kind: Workflow
metadata:
  name: build
  project: demo
spec:
  goal: ship it
  pipeline:
    - name: execute
      approval: APPROVAL_POLICY_HUMAN
`

func TestDecodeResourcesMultiDoc(t *testing.T) {
	res, err := decodeResources(strings.NewReader(sampleYAML))
	if err != nil {
		t.Fatalf("decodeResources: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d resources, want 2", len(res))
	}

	p, ok := res[0].Message.(*apiv1.Project)
	if !ok {
		t.Fatalf("first resource is %T, want *Project", res[0].Message)
	}
	if p.GetMetadata().GetName() != "demo" {
		t.Errorf("project name = %q, want demo", p.GetMetadata().GetName())
	}
	if p.GetSpec().GetDisplayName() != "Demo Project" {
		t.Errorf("displayName = %q, want 'Demo Project'", p.GetSpec().GetDisplayName())
	}
	if p.GetMetadata().GetLabels()["team"] != "core" {
		t.Errorf("label team = %q, want core", p.GetMetadata().GetLabels()["team"])
	}

	w, ok := res[1].Message.(*apiv1.Workflow)
	if !ok {
		t.Fatalf("second resource is %T, want *Workflow", res[1].Message)
	}
	if w.GetMetadata().GetProject() != "demo" {
		t.Errorf("workflow project = %q, want demo", w.GetMetadata().GetProject())
	}
	if got := w.GetSpec().GetPipeline(); len(got) != 1 || got[0].GetApproval() != apiv1.ApprovalPolicy_APPROVAL_POLICY_HUMAN {
		t.Errorf("pipeline = %+v, want one HUMAN-approval phase", got)
	}
}

func TestDecodeResourcesErrors(t *testing.T) {
	cases := map[string]string{
		"missing kind":       "apiVersion: kestrai.dev/v1alpha1\nmetadata:\n  name: x\n",
		"wrong apiVersion":   "apiVersion: v2\nkind: Project\nmetadata:\n  name: x\n",
		"unknown kind":       "apiVersion: kestrai.dev/v1alpha1\nkind: Banana\nmetadata:\n  name: x\n",
		"missing name":       "apiVersion: kestrai.dev/v1alpha1\nkind: Project\nspec: {}\n",
		"unknown spec field": "apiVersion: kestrai.dev/v1alpha1\nkind: Project\nmetadata:\n  name: x\nspec:\n  bogusField: 1\n",
		"empty input":        "",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeResources(strings.NewReader(in)); err == nil {
				t.Errorf("expected error for %q, got nil", name)
			}
		})
	}
}

// TestEncodeRoundTrip verifies that encoding a resource and decoding it again
// preserves the spec, including the stamped TypeMeta envelope.
func TestEncodeRoundTrip(t *testing.T) {
	orig := &apiv1.Project{
		Metadata: &apiv1.ObjectMeta{Name: "demo"},
		Spec:     &apiv1.ProjectSpec{DisplayName: "Demo"},
	}
	for _, format := range []string{outputYAML, outputJSON} {
		t.Run(format, func(t *testing.T) {
			out, err := encodeResource(kindProject, orig, format)
			if err != nil {
				t.Fatalf("encodeResource: %v", err)
			}
			if !strings.Contains(string(out), apiVersionV1Alpha1) {
				t.Errorf("encoded output missing apiVersion:\n%s", out)
			}
			res, err := decodeResources(strings.NewReader(string(out)))
			if err != nil {
				t.Fatalf("decode round-trip: %v", err)
			}
			got := res[0].Message.(*apiv1.Project)
			if got.GetSpec().GetDisplayName() != "Demo" {
				t.Errorf("round-trip displayName = %q, want Demo", got.GetSpec().GetDisplayName())
			}
		})
	}
}
