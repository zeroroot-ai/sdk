// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package cueschemas exports the SDK's CUE module tree as an [embed.FS].
//
// The embedded tree is a self-contained CUE module rooted at
// github.com/zeroroot-ai/sdk@v1. It contains:
//
//   - cue.mod/module.cue — module declaration and language version
//   - cue.mod/gen/       — vendored CUE bindings for buf.build deps
//     (currently buf.build/bufbuild/protovalidate)
//   - api/proto/gibson/mission/v1/mission_definition_proto_gen.cue
//   - api/proto/gibson/types/v1/types_proto_gen.cue
//     (imported by mission_definition_proto_gen.cue)
//   - api/proto/gibson/common/v1/gibson_common_proto_gen.cue
//     (imported by types_proto_gen.cue)
//
// Consumers (e.g. the gibson daemon) use [Schemas] as an [fs.FS]
// overlay when calling cuelang.org/go/cue/load to validate mission
// definitions without a subprocess or runtime download.
//
// The embed directive lives in the root sdk package (cue_schemas.go)
// because go:embed paths are resolved relative to the declaring
// source file, and cue.mod/ and api/proto/ are siblings of the
// module root — not of this subdirectory. Schemas re-exports the
// root package variable so callers use the stable sub-package path.
//
// Spec: sdk#147.
package cueschemas

import (
	"embed"
	"io/fs"

	sdk "github.com/zeroroot-ai/sdk"
)

// Schemas is the embedded CUE module tree.
// Access files via the standard [fs.FS] interface:
//
//	f, err := cueschemas.Schemas.Open("cue.mod/module.cue")
//
// The underlying embed.FS is initialized from the module root where
// cue.mod/ and api/proto/ reside. See cue_schemas.go in the root package.
var Schemas embed.FS = sdk.CueSchemas

// Open is a convenience wrapper around [Schemas.Open].
func Open(name string) (fs.File, error) {
	return Schemas.Open(name)
}

// ReadFile is a convenience wrapper around [fs.ReadFile] on [Schemas].
func ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(Schemas, name)
}
