// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// This is a simple example demonstrating the logger functionality
// Run with: go run main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zeroroot-ai/sdk/agent"
	"github.com/zeroroot-ai/sdk/eval"
)

func main() {
	// Create logger
	logger, err := eval.NewJSONLLogger("example.jsonl")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Create sample
	sample := eval.Sample{
		ID: "example-001",
		Task: agent.Task{
			ID:      "task-001",
			Context: map[string]any{"objective": "Demonstrate logger functionality"},
		},
		Tags: []string{"example", "demo"},
	}

	// Create result with multiple scorers
	result := eval.Result{
		SampleID: "example-001",
		Scores: map[string]eval.ScoreResult{
			"accuracy": {
				Score: 0.92,
				Details: map[string]any{
					"true_positives":  23,
					"false_positives": 2,
					"false_negatives": 1,
				},
			},
			"completeness": {
				Score: 0.88,
				Details: map[string]any{
					"tasks_completed": 15,
					"tasks_total":     17,
				},
			},
		},
		OverallScore: 0.90,
		Duration:     350 * time.Millisecond,
		Timestamp:    time.Now(),
	}

	// Log it
	fmt.Println("Logging evaluation result...")
	if err := logger.Log(sample, result); err != nil {
		fmt.Printf("Failed to log: %v\n", err)
		os.Exit(1)
	}

	// Close logger
	if err := logger.Close(); err != nil {
		fmt.Printf("Failed to close logger: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully logged result to example.jsonl")

	// Read it back and display
	data, err := os.ReadFile("example.jsonl")
	if err != nil {
		fmt.Printf("Failed to read log file: %v\n", err)
		os.Exit(1)
	}

	var entry eval.LogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		fmt.Printf("Failed to parse log entry: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nLogged entry:")
	fmt.Printf("  Sample ID: %s\n", entry.SampleID)
	fmt.Printf("  Task ID: %s\n", entry.TaskID)
	fmt.Printf("  Overall Score: %.2f\n", entry.OverallScore)
	fmt.Printf("  Duration: %dms\n", entry.Duration)
	fmt.Printf("  Scores:\n")
	for name, score := range entry.Scores {
		fmt.Printf("    %s: %.2f\n", name, score)
	}

	// Clean up
	os.Remove("example.jsonl")
	fmt.Println("\nExample completed successfully!")
}
