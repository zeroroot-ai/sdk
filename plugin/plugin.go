// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package plugin is the Gibson plugin SDK.
//
// Plugin authors call [Serve] from their main() to register typed Go method
// handlers and start the dispatch loop the daemon drives. Authoring is
// Go-first (ADR-0065 R4): the author writes one handler.go with plain typed Go
// request/response structs and registers each handler with [WithHandler]; the
// SDK derives the method's tool schema/descriptor from those Go types. There is
// no hand-written .proto and no per-method codegen. The plugin declares itself
// in code and reports the declaration at start (ADR-0097). No manifest file
// exists.
//
// Example:
//
//	func main() {
//	    if err := plugin.Serve(ctx,
//	        plugin.WithName("incidents"),
//	        plugin.WithVersion("1.0.0"),
//	        plugin.WithHandler("CreateIncident", "opens an incident from a finding", createIncident),
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

// Descriptor is a resolved, harness-visible snapshot of a plugin registration.
// The daemon builds it from the plugin's RegisterComponent call, and
// agent.Harness.ListPlugins returns it. Agents use it for method discovery without
// invoking the plugin.
type Descriptor struct {
	// Name is the plugin name, set by [WithName].
	Name string `json:"name"`
	// Version is the plugin version, set by [WithVersion].
	Version string `json:"version"`
	// Description is the plugin description.
	Description string `json:"description,omitempty"`
	// Methods is the ordered list of method descriptors, one per [WithHandler].
	Methods []MethodDescriptor `json:"methods"`
}

// MethodDescriptor carries the per-method contract surfaced to agents.
//
// Under the Go-first model the request/response schema is derived from the
// author's typed Go structs at registration (see [WithHandler]) and travels to
// the daemon as a JSON-Schema document. [InputSchema] holds that derived
// request schema when known.
type MethodDescriptor struct {
	// Name is the method name given to [WithHandler].
	Name string `json:"name"`
	// Description is the method description given to [WithHandler].
	Description string `json:"description,omitempty"`
	// InputSchema is the JSON-Schema document describing the method's request,
	// derived from the Go request struct registered with [WithHandler]. Empty
	// when unknown.
	InputSchema string `json:"input_schema,omitempty"`
	// OutputSchema is the JSON-Schema document describing the method's response,
	// derived from the Go response struct registered with [WithHandler]. Empty
	// when unknown.
	OutputSchema string `json:"output_schema,omitempty"`
	// Capabilities is the list of declared capability strings for the method
	// (e.g. "cache", "rate_limit:tier1").
	Capabilities []string `json:"capabilities,omitempty"`
}
