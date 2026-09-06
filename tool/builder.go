// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"
	"errors"

	"github.com/zeroroot-ai/sdk/types"
	"google.golang.org/protobuf/proto"
)

// Config holds the configuration for building a Tool.
type Config struct {
	name              string
	version           string
	description       string
	tags              []string
	inputMessageType  string
	outputMessageType string
	executeProtoFunc  func(ctx context.Context, input proto.Message) (proto.Message, error)
}

// NewConfig creates a new Config with default values.
func NewConfig() *Config {
	return &Config{
		version: "1.0.0",
		tags:    []string{},
	}
}

// SetName sets the tool name.
func (c *Config) SetName(name string) *Config {
	c.name = name
	return c
}

// SetVersion sets the tool version.
func (c *Config) SetVersion(version string) *Config {
	c.version = version
	return c
}

// SetDescription sets the tool description.
func (c *Config) SetDescription(desc string) *Config {
	c.description = desc
	return c
}

// SetTags sets the tool tags.
func (c *Config) SetTags(tags []string) *Config {
	c.tags = tags
	return c
}

// SetInputMessageType sets the proto input message type.
func (c *Config) SetInputMessageType(messageType string) *Config {
	c.inputMessageType = messageType
	return c
}

// SetOutputMessageType sets the proto output message type.
func (c *Config) SetOutputMessageType(messageType string) *Config {
	c.outputMessageType = messageType
	return c
}

// SetExecuteProtoFunc sets the proto execution function.
func (c *Config) SetExecuteProtoFunc(fn func(ctx context.Context, input proto.Message) (proto.Message, error)) *Config {
	c.executeProtoFunc = fn
	return c
}

// sdkTool is the internal implementation of the Tool interface.
type sdkTool struct {
	name              string
	version           string
	description       string
	tags              []string
	inputMessageType  string
	outputMessageType string
	executeProtoFunc  func(ctx context.Context, input proto.Message) (proto.Message, error)
}

// New creates a new Tool from the provided Config.
// Returns an error if required fields (name) are missing.
func New(cfg *Config) (Tool, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	if cfg.name == "" {
		return nil, errors.New("tool name is required")
	}

	return &sdkTool{
		name:              cfg.name,
		version:           cfg.version,
		description:       cfg.description,
		tags:              cfg.tags,
		inputMessageType:  cfg.inputMessageType,
		outputMessageType: cfg.outputMessageType,
		executeProtoFunc:  cfg.executeProtoFunc,
	}, nil
}

// Name returns the tool name.
func (t *sdkTool) Name() string {
	return t.name
}

// Version returns the tool version.
func (t *sdkTool) Version() string {
	return t.version
}

// Description returns the tool description.
func (t *sdkTool) Description() string {
	return t.description
}

// Tags returns the tool tags.
func (t *sdkTool) Tags() []string {
	return t.tags
}

// InputMessageType returns the proto message type name for input.
func (t *sdkTool) InputMessageType() string {
	return t.inputMessageType
}

// OutputMessageType returns the proto message type name for output.
func (t *sdkTool) OutputMessageType() string {
	return t.outputMessageType
}

// ExecuteProto runs the tool with proto message input/output.
func (t *sdkTool) ExecuteProto(ctx context.Context, input proto.Message) (proto.Message, error) {
	if t.executeProtoFunc == nil {
		return nil, errors.New("proto execution not configured for this tool")
	}
	return t.executeProtoFunc(ctx, input)
}

// Health returns the health status of the tool.
// By default, tools are always healthy unless they implement custom health checks.
func (t *sdkTool) Health(ctx context.Context) types.HealthStatus {
	return types.NewHealthyStatus("tool is operational")
}
