// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package taxonomy

import (
	"embed"
	"io/fs"
)

//go:embed ontology
var embeddedOntologyFS embed.FS

// EmbeddedOntology returns an fs.FS rooted at the taxonomy/ontology/
// directory that is compiled into the SDK binary.
//
// The returned FS contains all files under taxonomy/ontology/ including
// README.md, THIRD-PARTY-LICENSES.md, and any *.yaml or *.ttl files that
// contributors have placed there. Example files (*.yaml.example, *.ttl.example)
// are present in the FS but the daemon loader skips them by convention.
//
// Usage in the daemon:
//
//	fsys := taxonomy.EmbeddedOntology()
//	entries, _ := fs.ReadDir(fsys, "ontology")
//	for _, e := range entries {
//	    if strings.HasSuffix(e.Name(), ".yaml") {
//	        data, _ := fs.ReadFile(fsys, "ontology/"+e.Name())
//	        o, _ := taxonomy.Parse(data)
//	        // ... convert and register
//	    }
//	}
func EmbeddedOntology() fs.FS {
	return embeddedOntologyFS
}
