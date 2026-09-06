// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"github.com/zeroroot-ai/sdk/graphrag"
)

// OntologyContributor is the optional interface a component implements to
// contribute an ontology extension at enrollment time. `serve.Tool`,
// `serve.Agent`, and the SPIFFE variants type-assert against this interface
// when constructing the `ComponentInfo` passed to `PlatformClient.Register`;
// components that do not implement it are unaffected.
//
// The recommended implementation pattern is to delegate to a generated
// helper produced by the ADK:
//
//	import "github.com/zeroroot-ai/<your-component>/gen"
//
//	func (t *myTool) OntologyExtension() graphrag.OntologyExtension {
//	    return gen.OntologyExtension()
//	}
//
// Components that author an `ontology.yaml` in their repo get the generated
// helper automatically via `gibson component build`. Components without an
// `ontology.yaml` should not implement this interface (or should return a
// zero value) — the SDK skips the wire field when `OntologyExtension.IsZero`
// is true, so an empty implementation is harmless but wasteful.
type OntologyContributor interface {
	// OntologyExtension returns the component's contribution to the daemon's
	// ontology reasoner. The zero value is treated as "no contribution" and
	// is skipped on the wire.
	OntologyExtension() graphrag.OntologyExtension
}

// ontologyExtensionFrom returns the OntologyExtension a component contributes,
// or the zero value if the component does not implement OntologyContributor.
// Internal helper used by the serve.Tool / serve.Agent code paths to keep the
// type-assertion in one place.
func ontologyExtensionFrom(component interface{}) graphrag.OntologyExtension {
	if oc, ok := component.(OntologyContributor); ok {
		return oc.OntologyExtension()
	}
	return graphrag.OntologyExtension{}
}
