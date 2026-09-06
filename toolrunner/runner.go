// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package toolrunner provides the in-sandbox harness for executing a single
// SDK tool inside a Setec microVM (or any container with the same env-in /
// stdout-marker-out ABI).
//
// ABI summary:
//
//	Input  (env):    GIBSON_TOOL_INPUT_B64=<base64(protojson(input))>
//	                 GIBSON_TRACE_ID=<hex>            (optional)
//	                 GIBSON_SPAN_ID=<hex>             (optional)
//	Output (stdout): <arbitrary tool log lines, may be empty>
//	                 ===GIBSON_TOOL_OUTPUT===<base64(protojson(response))>\n
//	Error  (stdout): ===GIBSON_TOOL_ERROR===<message>\n
//	                 (and exit code: 1=input parse, 2=execute, 3=output marshal)
//
// The executor on the Gibson side scans the captured stdout buffer for the
// LAST line beginning with ===GIBSON_TOOL_OUTPUT===, allowing tools to log
// freely beforehand.
package toolrunner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/zeroroot-ai/sdk/tool"
)

const (
	envInputB64  = "GIBSON_TOOL_INPUT_B64"
	envTraceID   = "GIBSON_TRACE_ID"
	envSpanID    = "GIBSON_SPAN_ID"
	markerOutput = "===GIBSON_TOOL_OUTPUT==="
	markerError  = "===GIBSON_TOOL_ERROR==="

	exitInputParse    = 1
	exitExecute       = 2
	exitOutputMarshal = 3
)

// Run is the entry point a per-tool main() invokes. It blocks until the tool
// returns and exits the process with the appropriate code. It does not return
// to the caller in the success path.
//
// The tool's input is constructed by reflective proto allocation of the
// message type named by t.InputMessageType(). The caller is therefore
// responsible for ensuring that input message type is registered with the
// proto runtime (i.e. its generated package is imported transitively).
func Run(t tool.Tool) {
	exit, err := run(os.Stdout, os.Stderr, os.Environ(), t)
	if err != nil {
		fmt.Fprint(os.Stdout, errorLine(err))
	}
	os.Exit(exit)
}

// run is the testable core. Returns (exit code, error).
//   - On success: writes the output marker line to stdout, exit 0.
//   - On failure: writes nothing to stdout (caller writes the error marker
//     line based on the returned error). Returns the exit code.
func run(stdout, stderr io.Writer, env []string, t tool.Tool) (int, error) {
	envMap := envSlice(env)

	// Decode input
	b64, ok := envMap[envInputB64]
	if !ok || b64 == "" {
		return exitInputParse, fmt.Errorf("missing required env %s", envInputB64)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return exitInputParse, fmt.Errorf("decode %s: %w", envInputB64, err)
	}
	input, err := newMessage(t.InputMessageType())
	if err != nil {
		return exitInputParse, fmt.Errorf("alloc input %q: %w", t.InputMessageType(), err)
	}
	if err := protojson.Unmarshal(raw, input); err != nil {
		return exitInputParse, fmt.Errorf("unmarshal input: %w", err)
	}

	// Execute
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	output, err := t.ExecuteProto(ctx, input)
	if err != nil {
		return exitExecute, fmt.Errorf("execute: %w", err)
	}
	if output == nil {
		return exitExecute, errors.New("tool returned nil output without error")
	}

	// Marshal + emit
	outBytes, err := protojson.Marshal(output)
	if err != nil {
		return exitOutputMarshal, fmt.Errorf("marshal output: %w", err)
	}
	fmt.Fprint(stdout, markerOutput+base64.StdEncoding.EncodeToString(outBytes)+"\n")
	return 0, nil
}

func envSlice(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		i := strings.IndexByte(e, '=')
		if i < 0 {
			continue
		}
		m[e[:i]] = e[i+1:]
	}
	return m
}

func errorLine(err error) string {
	return markerError + err.Error() + "\n"
}

// newMessage allocates a proto.Message instance for the given fully-qualified
// type name. The type must be registered with the proto runtime (achieved by
// importing the generating package somewhere in the binary's transitive
// dependency graph).
func newMessage(typeName string) (proto.Message, error) {
	mt, err := protoTypeFor(typeName)
	if err != nil {
		return nil, err
	}
	return mt.New().Interface(), nil
}
