// CUE module for the gibson SDK protos.
//
// CUE definitions for gibson.mission.v1 and gibson.daemon.v1 are
// generated alongside the .proto sources via `make cue-defs`.
// They live next to the proto files (`*_proto_gen.cue`) following
// CUE's import-in-place convention.
//
// External proto deps (buf.build/bufbuild/protovalidate) are
// vendored under cue.mod/gen/.
//
// Spec: mission-authoring-cue Requirement 1.

module: "github.com/zeroroot-ai/sdk@v1"

language: {
	version: "v0.16.0"
}
