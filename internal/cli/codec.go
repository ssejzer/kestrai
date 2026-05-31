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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	apiv1 "github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1"
	"github.com/kestrai/kestrai/internal/version"
)

// Recognized resource kinds. These match the YAML `kind:` and the store's
// kind discriminator.
const (
	kindProject  = "Project"
	kindWorkflow = "Workflow"
)

// apiVersionV1Alpha1 is the only apiVersion the CLI accepts on input.
var apiVersionV1Alpha1 = version.APIVersion // "kestrai.dev/v1alpha1"

// resource is one decoded YAML document: the typed proto message plus the
// metadata the verbs route on.
type resource struct {
	Kind    string
	Project string // metadata.project ("" for tenant-global kinds)
	Name    string
	Message proto.Message
}

// decodeResources reads one or more YAML documents (separated by `---`) and
// decodes each into a typed resource. The TypeMeta envelope (apiVersion, kind)
// is consumed here and stripped before protojson sees the body, so unknown-field
// checking still catches typos in the actual spec.
func decodeResources(r io.Reader) ([]resource, error) {
	dec := yaml.NewDecoder(r)
	var out []resource
	for i := 0; ; i++ {
		var doc map[string]any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse YAML document %d: %w", i+1, err)
		}
		if len(doc) == 0 {
			continue // empty document (e.g. trailing `---`)
		}
		res, err := decodeDocument(doc)
		if err != nil {
			return nil, fmt.Errorf("document %d: %w", i+1, err)
		}
		out = append(out, res)
	}
	if len(out) == 0 {
		return nil, errors.New("no resources found in input")
	}
	return out, nil
}

// decodeDocument turns one parsed YAML mapping into a typed resource.
func decodeDocument(doc map[string]any) (resource, error) {
	apiVersion, _ := doc["apiVersion"].(string)
	kind, _ := doc["kind"].(string)
	if apiVersion == "" || kind == "" {
		return resource{}, errors.New("both apiVersion and kind are required")
	}
	if apiVersion != apiVersionV1Alpha1 {
		return resource{}, fmt.Errorf("unsupported apiVersion %q (want %q)", apiVersion, apiVersionV1Alpha1)
	}

	msg, err := newMessageForKind(kind)
	if err != nil {
		return resource{}, err
	}

	// Strip the envelope; protojson only sees {metadata, spec, status}.
	delete(doc, "apiVersion")
	delete(doc, "kind")

	body, err := json.Marshal(doc)
	if err != nil {
		return resource{}, fmt.Errorf("re-encode %s body: %w", kind, err)
	}
	if err := protojson.Unmarshal(body, msg); err != nil {
		return resource{}, fmt.Errorf("decode %s: %w", kind, err)
	}

	meta := metadataOf(msg)
	if meta.GetName() == "" {
		return resource{}, errors.New("metadata.name is required")
	}
	return resource{Kind: kind, Project: meta.GetProject(), Name: meta.GetName(), Message: msg}, nil
}

// normalizeKind maps a user-supplied kind argument — singular or plural, any
// case ("project", "Projects", "workflows") — onto the canonical Kind name.
func normalizeKind(arg string) (string, error) {
	switch strings.ToLower(strings.TrimSuffix(arg, "s")) {
	case "project":
		return kindProject, nil
	case "workflow":
		return kindWorkflow, nil
	default:
		return "", fmt.Errorf("unknown kind %q (supported: project, workflow)", arg)
	}
}

// newMessageForKind returns an empty typed message for a kind.
func newMessageForKind(kind string) (proto.Message, error) {
	switch kind {
	case kindProject:
		return &apiv1.Project{}, nil
	case kindWorkflow:
		return &apiv1.Workflow{}, nil
	default:
		return nil, fmt.Errorf("unknown kind %q (supported: %s, %s)", kind, kindProject, kindWorkflow)
	}
}

// metadataOf returns the ObjectMeta of a known resource message, or an empty
// one if the message has none.
func metadataOf(msg proto.Message) *apiv1.ObjectMeta {
	switch m := msg.(type) {
	case *apiv1.Project:
		return m.GetMetadata()
	case *apiv1.Workflow:
		return m.GetMetadata()
	default:
		return &apiv1.ObjectMeta{}
	}
}

// encodeResource marshals a resource message back to YAML or JSON with the
// TypeMeta envelope stamped on. format is "yaml" or "json".
func encodeResource(kind string, msg proto.Message, format string) ([]byte, error) {
	body, err := protojson.MarshalOptions{UseProtoNames: false}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// Decode into generic values (not json.RawMessage) so yaml.Marshal renders
	// real maps/scalars rather than raw JSON byte slices.
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}

	// Stamp the envelope so output round-trips back through apply.
	envelope := map[string]any{"apiVersion": apiVersionV1Alpha1, "kind": kind}
	for k, v := range fields {
		envelope[k] = v
	}

	switch format {
	case outputJSON:
		return json.MarshalIndent(envelope, "", "  ")
	case outputYAML:
		return yaml.Marshal(envelope)
	default:
		return nil, fmt.Errorf("unsupported output format %q (want yaml or json)", format)
	}
}
