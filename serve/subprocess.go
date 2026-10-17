// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/zeroroot-ai/sdk/tool"
)

// OutputSchema outputs the tool's schema as JSON to stdout.
// This is called when the tool is invoked with the --schema flag.
//
// The schema output includes:
//   - name: Tool name
//   - version: Tool version
//   - description: Tool description
//   - tags: Tool tags
//   - input_message_type: Proto message type for input
//   - output_message_type: Proto message type for output
//   - enum_mappings: (optional) Registered enum mappings for the tool
//
// The enum_mappings field is only included if the tool has registered
// explicit mappings via enum.Register() or enum.RegisterBatch().
//
// Example:
//
//	tool := &MyTool{}
//	if err := serve.OutputSchema(tool); err != nil {
//	    // Error is already written to stderr
//	    os.Exit(1)
//	}
func OutputSchema(t tool.Tool) error {
	// Build schema output
	schema := map[string]any{
		"name":                t.Name(),
		"version":             t.Version(),
		"description":         t.Description(),
		"tags":                t.Tags(),
		"input_message_type":  t.InputMessageType(),
		"output_message_type": t.OutputMessageType(),
	}

	// Marshal schema to JSON
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		writeError("failed to marshal schema: %v", err)
		return err
	}

	// Write schema to stdout
	if _, err := os.Stdout.Write(schemaBytes); err != nil {
		writeError("failed to write schema: %v", err)
		return err
	}

	// Write newline for clean output
	if _, err := os.Stdout.WriteString("\n"); err != nil {
		writeError("failed to write newline: %v", err)
		return err
	}

	return nil
}

// writeError writes a formatted error message to stderr.
func writeError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", msg)
}
