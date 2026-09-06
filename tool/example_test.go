// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	protolib "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/zeroroot-ai/sdk/tool"
)

// Example demonstrates creating and using a simple calculator tool with proto messages.
func Example() {
	// Create a calculator tool configuration
	cfg := tool.NewConfig().
		SetName("calculator").
		SetVersion("1.0.0").
		SetDescription("Performs basic arithmetic operations").
		SetTags([]string{"math", "utility"}).
		SetInputMessageType("google.protobuf.Struct").
		SetOutputMessageType("google.protobuf.Struct").
		SetExecuteProtoFunc(func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
			st := input.(*structpb.Struct)
			op := st.Fields["operation"].GetStringValue()
			a := st.Fields["a"].GetNumberValue()
			b := st.Fields["b"].GetNumberValue()

			var result float64
			switch op {
			case "add":
				result = a + b
			case "subtract":
				result = a - b
			case "multiply":
				result = a * b
			case "divide":
				if b == 0 {
					return nil, errors.New("division by zero")
				}
				result = a / b
			default:
				return nil, fmt.Errorf("unknown operation: %s", op)
			}

			output, _ := structpb.NewStruct(map[string]any{"result": result})
			return output, nil
		})

	// Create the tool
	calculator, err := tool.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Execute the tool
	inputMsg, _ := structpb.NewStruct(map[string]any{
		"operation": "add",
		"a":         5.0,
		"b":         3.0,
	})
	result, err := calculator.ExecuteProto(context.Background(), inputMsg)
	if err != nil {
		log.Fatal(err)
	}

	resultStruct := result.(*structpb.Struct)
	fmt.Printf("Result: %.0f\n", resultStruct.Fields["result"].GetNumberValue())

	// Output:
	// Result: 8
}

// ExampleToDescriptor demonstrates converting a tool to its descriptor.
func ExampleToDescriptor() {
	// Create a simple tool
	cfg := tool.NewConfig().
		SetName("greeter").
		SetVersion("1.0.0").
		SetDescription("Greets users by name").
		SetTags([]string{"greeting", "example"})

	greeter, err := tool.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Convert to descriptor
	desc := tool.ToDescriptor(greeter)

	fmt.Printf("Tool: %s v%s\n", desc.Name, desc.Version)
	fmt.Printf("Description: %s\n", desc.Description)
	fmt.Printf("Tags: %v\n", desc.Tags)

	// Output:
	// Tool: greeter v1.0.0
	// Description: Greets users by name
	// Tags: [greeting example]
}

// ExampleTool_Health demonstrates checking tool health.
func ExampleTool_Health() {
	cfg := tool.NewConfig().SetName("health-check-example")

	t, err := tool.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	status := t.Health(context.Background())
	if status.IsHealthy() {
		fmt.Println("Tool is operational")
	}

	// Output:
	// Tool is operational
}

// ExampleNew demonstrates creating a tool with proto execution.
func ExampleNew() {
	// Create a tool with proto-based execution
	cfg := tool.NewConfig().
		SetName("string-reverser").
		SetVersion("1.0.0").
		SetDescription("Reverses a string").
		SetInputMessageType("google.protobuf.Struct").
		SetOutputMessageType("google.protobuf.Struct").
		SetExecuteProtoFunc(func(ctx context.Context, input protolib.Message) (protolib.Message, error) {
			st := input.(*structpb.Struct)
			text := st.Fields["text"].GetStringValue()
			runes := []rune(text)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			output, _ := structpb.NewStruct(map[string]any{"reversed": string(runes)})
			return output, nil
		})

	reverser, err := tool.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	inputMsg, _ := structpb.NewStruct(map[string]any{"text": "hello"})
	result, err := reverser.ExecuteProto(context.Background(), inputMsg)
	if err != nil {
		log.Fatal(err)
	}

	resultStruct := result.(*structpb.Struct)
	fmt.Println(resultStruct.Fields["reversed"].GetStringValue())

	// Output:
	// olleh
}

// ExampleConfig_SetTags demonstrates setting tool tags for categorization.
func ExampleConfig_SetTags() {
	cfg := tool.NewConfig().
		SetName("tagged-tool").
		SetTags([]string{"data", "transformation", "utility"})

	t, err := tool.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Tags: %v\n", t.Tags())

	// Output:
	// Tags: [data transformation utility]
}
