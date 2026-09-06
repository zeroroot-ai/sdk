// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import "embed"

// CueSchemas is the embedded CUE module tree for the SDK.
//
// It contains a self-contained CUE module (github.com/zeroroot-ai/sdk@v1)
// with the mission-definition proto bindings and their transitive imports.
// Consumers should use the [cueschemas] package for the stable public API:
//
//	import "github.com/zeroroot-ai/sdk/cueschemas"
//	f, _ := cueschemas.Schemas.Open("cue.mod/module.cue")
//
// This variable is unexported-via-naming convention in the root package;
// the cueschemas package re-exports it as cueschemas.Schemas.
//
// Spec: sdk#147.
//
//go:embed all:cue.mod
//go:embed api/proto/gibson/mission/v1/mission_definition_proto_gen.cue
//go:embed api/proto/gibson/types/v1/types_proto_gen.cue
//go:embed api/proto/gibson/job/v1/job_proto_gen.cue
//go:embed api/proto/gibson/common/v1/gibson_common_proto_gen.cue
var CueSchemas embed.FS
