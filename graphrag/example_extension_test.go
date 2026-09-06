// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package graphrag_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/graphrag"
)

// ExampleTaxonomyIntrospector_extensionQuery demonstrates how to query
// taxonomy extensions and trace node types to their source.
func ExampleTaxonomyIntrospector_extensionQuery() {
	// Create core taxonomy
	core := graphrag.NewSimpleTaxonomy()

	// Create registry with core taxonomy
	registry := graphrag.NewTaxonomyRegistry(core)

	// Register a custom extension from an agent
	agentExtension := graphrag.TaxonomyExtension{
		NodeTypes: []graphrag.NodeTypeDefinition{
			{
				Name:        "kubernetes_pod",
				Category:    "asset",
				Description: "A Kubernetes pod in a cluster",
				Properties: []graphrag.PropertyInfo{
					{Name: "namespace", Type: "string", Required: true},
					{Name: "name", Type: "string", Required: true},
				},
			},
			{
				Name:        "container",
				Category:    "asset",
				Description: "A container within a pod",
			},
		},
		Relationships: []graphrag.RelationshipDefinition{
			{
				Name:        "RUNS_IN_POD",
				Category:    "execution",
				Description: "Container runs in pod",
				FromTypes:   []string{"container"},
				ToTypes:     []string{"kubernetes_pod"},
			},
		},
	}

	registry.RegisterExtension("k8s-scanner", agentExtension)

	// Query all registered extensions
	extensions := registry.ExtensionNames()
	fmt.Printf("Registered extensions: %v\n", extensions)

	// Get extension details
	ext := registry.ExtensionInfo("k8s-scanner")
	if ext != nil {
		fmt.Printf("Extension 'k8s-scanner' has %d node types\n", len(ext.NodeTypes))
	}

	// Trace node type sources
	fmt.Printf("Source of 'host': %s\n", registry.NodeTypeSource("host"))
	fmt.Printf("Source of 'kubernetes_pod': %s\n", registry.NodeTypeSource("kubernetes_pod"))
	fmt.Printf("Source of 'unknown_type': %s\n", registry.NodeTypeSource("unknown_type"))

	// Output:
	// Registered extensions: [k8s-scanner]
	// Extension 'k8s-scanner' has 2 node types
	// Source of 'host': core
	// Source of 'kubernetes_pod': k8s-scanner
	// Source of 'unknown_type': unknown
}

// ExampleTaxonomyIntrospector_multipleExtensions demonstrates managing
// multiple taxonomy extensions from different agents.
func ExampleTaxonomyIntrospector_multipleExtensions() {
	core := graphrag.NewSimpleTaxonomy()
	registry := graphrag.NewTaxonomyRegistry(core)

	// Register extension from cloud scanner
	cloudExt := graphrag.TaxonomyExtension{
		NodeTypes: []graphrag.NodeTypeDefinition{
			{Name: "s3_bucket", Category: "asset"},
			{Name: "iam_role", Category: "asset"},
		},
	}
	registry.RegisterExtension("cloud-scanner", cloudExt)

	// Register extension from network scanner
	networkExt := graphrag.TaxonomyExtension{
		NodeTypes: []graphrag.NodeTypeDefinition{
			{Name: "vlan", Category: "asset"},
			{Name: "network_segment", Category: "asset"},
		},
	}
	registry.RegisterExtension("network-scanner", networkExt)

	// Count total extensions
	extensions := registry.ExtensionNames()
	fmt.Printf("Total extensions: %d\n", len(extensions))

	// Check specific extensions exist
	cloudInfo := registry.ExtensionInfo("cloud-scanner")
	if cloudInfo != nil {
		fmt.Printf("cloud-scanner has %d node types\n", len(cloudInfo.NodeTypes))
	}

	networkInfo := registry.ExtensionInfo("network-scanner")
	if networkInfo != nil {
		fmt.Printf("network-scanner has %d node types\n", len(networkInfo.NodeTypes))
	}

	// Trace different node types to their sources
	nodeTypes := []string{"host", "s3_bucket", "vlan", "unknown"}
	for _, nt := range nodeTypes {
		source := registry.NodeTypeSource(nt)
		fmt.Printf("%s -> %s\n", nt, source)
	}

	// Output:
	// Total extensions: 2
	// cloud-scanner has 2 node types
	// network-scanner has 2 node types
	// host -> core
	// s3_bucket -> cloud-scanner
	// vlan -> network-scanner
	// unknown -> unknown
}

// ExampleSimpleTaxonomy_extensionMethods demonstrates that SimpleTaxonomy
// always returns empty results for extension queries since it has no extensions.
func ExampleSimpleTaxonomy_extensionMethods() {
	taxonomy := graphrag.NewSimpleTaxonomy()

	// SimpleTaxonomy has no extensions
	extensions := taxonomy.ExtensionNames()
	fmt.Printf("Extensions: %v\n", extensions)

	// Extension info always returns nil
	ext := taxonomy.ExtensionInfo("any-name")
	fmt.Printf("Extension info: %v\n", ext)

	// Node types are all core or unknown
	fmt.Printf("Source of 'host': %s\n", taxonomy.NodeTypeSource("host"))
	fmt.Printf("Source of 'custom': %s\n", taxonomy.NodeTypeSource("custom"))

	// Output:
	// Extensions: []
	// Extension info: <nil>
	// Source of 'host': core
	// Source of 'custom': unknown
}
