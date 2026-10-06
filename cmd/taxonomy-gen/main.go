// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// taxonomy-gen generates the graphrag node and relationship type constants
// from the taxonomy YAML.
//
// Usage:
//
//	taxonomy-gen --base taxonomy/core.yaml --output-constants graphrag/constants_generated.go --package graphrag
//
// Flags:
//
//	--base              Path to base taxonomy YAML (required)
//	--output-constants  Output path for constants Go file (required)
//	--package           Go package name for generated code (default: graphrag)
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/generator"
	"github.com/zeroroot-ai/sdk/cmd/taxonomy-gen/parser"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run parses the flags in args, reads the taxonomy and writes the constants.
func run(args []string) error {
	fs := flag.NewFlagSet("taxonomy-gen", flag.ContinueOnError)
	basePath := fs.String("base", "", "Path to base taxonomy YAML (required)")
	outputConstants := fs.String("output-constants", "", "Output path for constants Go file (required)")
	goPackage := fs.String("package", "graphrag", "Go package name for generated code")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	if *basePath == "" || *outputConstants == "" {
		return errors.New("--base and --output-constants are required")
	}

	fmt.Printf("Parsing base taxonomy: %s\n", *basePath)
	taxonomy, err := parser.ParseYAML(*basePath)
	if err != nil {
		return fmt.Errorf("parsing base taxonomy: %w", err)
	}
	fmt.Printf("  Version: %s\n", taxonomy.Version)
	fmt.Printf("  Node types: %d\n", len(taxonomy.NodeTypes))
	fmt.Printf("  Relationship types: %d\n", len(taxonomy.RelationshipTypes))

	// Retired entries leave the taxonomy here, before the generator sees it.
	taxonomy, _, _ = taxonomy.Retire()

	fmt.Printf("Generating constants: %s\n", *outputConstants)
	if err := generator.GenerateConstants(taxonomy, *outputConstants, *goPackage); err != nil {
		return fmt.Errorf("generating constants: %w", err)
	}
	return nil
}
