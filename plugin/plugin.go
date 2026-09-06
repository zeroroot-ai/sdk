// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package plugin is the Gibson plugin SDK.
//
// Plugin authors call [Serve] from their main() to register typed Go method
// handlers and start the dispatch loop the daemon drives. Authoring is
// Go-first (ADR-0065 R4): the author writes one handler.go with plain typed Go
// request/response structs and registers each handler with [WithHandler]; the
// SDK derives the method's tool schema/descriptor from those Go types. There is
// no hand-written .proto and no per-method codegen.
//
// Example:
//
//	func main() {
//	    if err := plugin.Serve(ctx,
//	        plugin.WithManifest("plugin.yaml"),
//	        plugin.WithHandler("CreateIncident", createIncident),
//	    ); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
//	type CreateIncidentRequest struct {
//	    Title    string `json:"title"`
//	    Severity int    `json:"severity"`
//	}
//	type CreateIncidentResponse struct {
//	    ID string `json:"id"`
//	}
//
//	func createIncident(ctx context.Context, req CreateIncidentRequest) (CreateIncidentResponse, error) {
//	    // ... call the vendor SDK ...
//	    return CreateIncidentResponse{ID: "INC-1"}, nil
//	}
package plugin

import "github.com/zeroroot-ai/sdk/plugin/manifest"

// Descriptor is a resolved, harness-visible snapshot of a plugin registration.
// It is constructed from the plugin's manifest at registration time and returned
// by agent.Harness.ListPlugins. Agents use it for method discovery without
// invoking the plugin.
type Descriptor struct {
	// Name is manifest.metadata.name.
	Name string `json:"name"`
	// Version is manifest.metadata.version.
	Version string `json:"version"`
	// Description is manifest.metadata.description.
	Description string `json:"description,omitempty"`
	// Methods is the ordered list of method descriptors derived from
	// manifest.spec.methods[].
	Methods []MethodDescriptor `json:"methods"`
}

// MethodDescriptor carries the per-method contract surfaced to agents.
//
// Under the Go-first model the request/response schema is derived from the
// author's typed Go structs at registration (see [WithHandler]) and travels to
// the daemon as a JSON-Schema document. [InputSchema] holds that derived
// request schema when known; [FromManifest] — which sees only the manifest —
// populates identity and description, and leaves the schema empty.
type MethodDescriptor struct {
	// Name is manifest.spec.methods[i].name.
	Name string `json:"name"`
	// Description is manifest.spec.methods[i].description.
	Description string `json:"description,omitempty"`
	// InputSchema is the JSON-Schema document describing the method's request,
	// derived from the Go request struct registered with [WithHandler]. Empty
	// when the descriptor was built from the manifest alone.
	InputSchema string `json:"input_schema,omitempty"`
	// OutputSchema is the JSON-Schema document describing the method's response,
	// derived from the Go response struct registered with [WithHandler]. Empty
	// when the descriptor was built from the manifest alone.
	OutputSchema string `json:"output_schema,omitempty"`
	// Capabilities is the list of declared capability strings for the method
	// (e.g. "cache", "rate_limit:tier1").
	Capabilities []string `json:"capabilities,omitempty"`
}

// FromManifest constructs a Descriptor from a loaded manifest.Manifest.
// It is called by the daemon at registration time and by the SDK framework
// registry when building the harness-visible list.
//
// FromManifest does not call manifest.Load — it expects an already-loaded
// and validated *manifest.Manifest. It sets each method's Name and Description;
// the per-method request/response schema is derived from the registered Go
// handlers (see [WithHandler]), not from the manifest, so InputSchema and
// OutputSchema are left empty here.
//
// The returned Descriptor.Methods is always non-nil; it is an empty slice
// when the manifest declares no methods.
func FromManifest(m *manifest.Manifest) Descriptor {
	methods := make([]MethodDescriptor, 0, len(m.Spec.Methods))
	for _, meth := range m.Spec.Methods {
		methods = append(methods, MethodDescriptor{
			Name:        meth.Name,
			Description: meth.Description,
		})
	}
	return Descriptor{
		Name:        m.Metadata.Name,
		Version:     m.Metadata.Version,
		Description: m.Metadata.Description,
		Methods:     methods,
	}
}
