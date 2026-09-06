// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package tool provides interfaces and builders for creating executable SDK tools.
//
// The tool package defines the Tool interface, which represents executable components
// with well-defined inputs, outputs, and metadata. Tools are the building blocks for
// creating composable, testable, and discoverable functionality within the Gibson Framework.
//
// # Core Concepts
//
// Tool: An executable component with:
//   - Unique name and version
//   - Human-readable description
//   - Tags for categorization and discovery
//   - JSON Schema definitions for inputs and outputs
//   - Execute function for performing operations
//   - Health check capability
//
// Config: A builder pattern configuration for constructing tools with fluent API.
//
// Descriptor: A metadata snapshot of a tool without execution logic, useful for
// tool discovery, documentation, and API responses.
//
// # Usage
//
// Creating a simple tool:
//
//	cfg := tool.NewConfig().
//		SetName("calculator").
//		SetVersion("1.0.0").
//		SetDescription("Performs basic arithmetic operations").
//		SetTags([]string{"math", "utility"}).
//		SetInputSchema(schema.Object(map[string]schema.JSON{
//			"operation": schema.String(),
//			"a":         schema.Number(),
//			"b":         schema.Number(),
//		}, "operation", "a", "b")).
//		SetOutputSchema(schema.Object(map[string]schema.JSON{
//			"result": schema.Number(),
//		}, "result")).
//		SetExecuteFunc(func(ctx context.Context, input map[string]any) (map[string]any, error) {
//			op := input["operation"].(string)
//			a := input["a"].(float64)
//			b := input["b"].(float64)
//
//			var result float64
//			switch op {
//			case "add":
//				result = a + b
//			case "subtract":
//				result = a - b
//			case "multiply":
//				result = a * b
//			case "divide":
//				if b == 0 {
//					return nil, errors.New("division by zero")
//				}
//				result = a / b
//			default:
//				return nil, fmt.Errorf("unknown operation: %s", op)
//			}
//
//			return map[string]any{"result": result}, nil
//		})
//
//	calculator, err := tool.New(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Executing a tool:
//
//	result, err := calculator.Execute(ctx, map[string]any{
//		"operation": "add",
//		"a":         5.0,
//		"b":         3.0,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Result: %v\n", result["result"]) // Output: Result: 8
//
// Getting tool metadata:
//
//	desc := tool.ToDescriptor(calculator)
//	fmt.Printf("Tool: %s v%s\n", desc.Name, desc.Version)
//	fmt.Printf("Description: %s\n", desc.Description)
//	fmt.Printf("Tags: %v\n", desc.Tags)
//
// Checking tool health:
//
//	status := calculator.Health(ctx)
//	if status.IsHealthy() {
//		fmt.Println("Tool is operational")
//	}
//
// # Input and Output Validation
//
// The tool package automatically validates inputs and outputs against their schemas.
// This ensures type safety and early error detection:
//
//   - Input validation occurs before execute function is called
//   - Output validation occurs after execute function completes
//   - Validation errors are returned to the caller
//
// # Context Support
//
// All tool operations accept a context.Context parameter, enabling:
//   - Request cancellation
//   - Timeout handling
//   - Request-scoped values
//   - Distributed tracing integration
//
// # Thread Safety
//
// Tool instances are immutable after creation and safe for concurrent use.
// Multiple goroutines can safely call Execute on the same tool instance.
package tool
