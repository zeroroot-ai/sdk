// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package tool defines the Tool interface that a tool component implements.
//
// A tool is a type with a name, a version, a description, tags, a proto input
// and output message type, and ExecuteProto, which runs the tool on one input
// message. serve.Tool serves a value of that type to the platform:
//
//	func main() {
//		if err := serve.Tool(&myTool{}); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// The tool scaffold of the adk writes this
// shape. Tool instances must be safe for concurrent use: the platform can call
// ExecuteProto from several goroutines at once.
package tool
