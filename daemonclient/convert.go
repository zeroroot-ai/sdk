// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package daemonclient provides proto-to-domain type conversions for daemon gRPC responses.
//
// This file contains conversion functions that transform protocol buffer types into
// domain types used by the client and CLI commands. These conversions ensure a clean
// separation between the gRPC layer and the application layer.
//
// It also contains TypedMap/TypedValue helper functions for converting between
// proto types and Go native maps/values.
package daemonclient

import (
	"fmt"
	"time"

	commonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/common/v1"
	daemonpb "github.com/zeroroot-ai/sdk/api/gen/gibson/daemon/v1"
)

// convertProtoStatus converts a gRPC StatusResponse to a domain DaemonStatus.
//
// Parameters:
//   - resp: The gRPC status response
//
// Returns:
//   - *DaemonStatus: The converted daemon status
func convertProtoStatus(resp *daemonpb.StatusResponse) *DaemonStatus {
	if resp == nil {
		return nil
	}

	return &DaemonStatus{
		Running:      resp.Running,
		PID:          int(resp.Pid),
		StartTime:    time.Unix(resp.StartTime, 0),
		Uptime:       resp.Uptime,
		GRPCAddress:  resp.GrpcAddress,
		RegistryType: resp.RegistryType,
		RegistryAddr: resp.RegistryAddr,
		CallbackAddr: resp.CallbackAddr,
		AgentCount:   int(resp.AgentCount),
	}
}

// convertProtoAgents converts a slice of gRPC AgentInfo messages to domain AgentInfo structs.
//
// Parameters:
//   - agents: Slice of gRPC agent info messages
//
// Returns:
//   - []AgentInfo: Converted agent information slice (never nil)
func convertProtoAgents(agents []*daemonpb.AgentInfo) []AgentInfo {
	if agents == nil {
		return []AgentInfo{}
	}

	result := make([]AgentInfo, 0, len(agents))
	for _, a := range agents {
		if a == nil {
			continue
		}

		result = append(result, AgentInfo{
			Name:        a.Name,
			Version:     a.Version,
			Description: "", // Description not in proto AgentInfo
			Address:     a.Endpoint,
			Status:      a.Health,
		})
	}

	return result
}

// convertProtoTools converts a slice of gRPC ToolInfo messages to domain ToolInfo structs.
//
// Parameters:
//   - tools: Slice of gRPC tool info messages
//
// Returns:
//   - []ToolInfo: Converted tool information slice (never nil)
func convertProtoTools(tools []*daemonpb.ToolInfo) []ToolInfo {
	if tools == nil {
		return []ToolInfo{}
	}

	result := make([]ToolInfo, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}

		var caps *Capabilities
		if t.Capabilities != nil {
			caps = &Capabilities{
				HasRoot:         t.Capabilities.HasRoot,
				HasSudo:         t.Capabilities.HasSudo,
				CanRawSocket:    t.Capabilities.CanRawSocket,
				Features:        t.Capabilities.Features,
				BlockedArgs:     t.Capabilities.BlockedArgs,
				ArgAlternatives: t.Capabilities.ArgAlternatives,
			}
		}

		result = append(result, ToolInfo{
			Name:         t.Name,
			Version:      t.Version,
			Description:  t.Description,
			Address:      t.Endpoint,
			Status:       t.Health,
			Capabilities: caps,
		})
	}

	return result
}

// convertProtoPlugins converts a slice of gRPC PluginInfo messages to domain PluginInfo structs.
//
// Parameters:
//   - plugins: Slice of gRPC plugin info messages
//
// Returns:
//   - []PluginInfo: Converted plugin information slice (never nil)
func convertProtoPlugins(plugins []*daemonpb.PluginInfo) []PluginInfo {
	if plugins == nil {
		return []PluginInfo{}
	}

	result := make([]PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		if p == nil {
			continue
		}

		result = append(result, PluginInfo{
			Name:        p.Name,
			Version:     p.Version,
			Description: p.Description,
			Address:     p.Endpoint,
			Status:      p.Health,
		})
	}

	return result
}

// convertProtoMissionEvent converts a gRPC MissionEvent to a domain MissionEvent.
//
// Parameters:
//   - event: The gRPC mission event
//
// Returns:
//   - MissionEvent: The converted mission event
func convertProtoMissionEvent(event *daemonpb.RunMissionResponse) MissionEvent {
	if event == nil {
		return MissionEvent{}
	}

	// Convert TypedMap data to map[string]interface{}
	var data map[string]interface{}
	if event.Data != nil {
		data = typedMapToMap(event.Data)
	}

	return MissionEvent{
		Type:      event.EventType,
		Timestamp: time.Unix(event.Timestamp, 0),
		Message:   event.Message,
		Data:      data,
	}
}

// convertProtoEvent converts a gRPC Event to a domain Event.
//
// Parameters:
//   - event: The gRPC event
//
// Returns:
//   - Event: The converted event
func convertProtoEvent(event *daemonpb.SubscribeResponse) Event {
	if event == nil {
		return Event{}
	}

	// Convert TypedMap data to map[string]interface{}
	var data map[string]interface{}
	if event.Data != nil {
		data = typedMapToMap(event.Data)
	}

	return Event{
		Type:      event.EventType,
		Source:    event.Source,
		Timestamp: time.Unix(event.Timestamp, 0),
		Data:      data,
	}
}

// mapToTypedMap converts map[string]any to *commonpb.TypedMap.
func mapToTypedMap(m map[string]any) *commonpb.TypedMap {
	if m == nil {
		return nil
	}
	entries := make(map[string]*commonpb.TypedValue)
	for k, v := range m {
		entries[k] = anyToTypedValue(v)
	}
	return &commonpb.TypedMap{Entries: entries}
}

// anyToTypedValue converts any Go value to *commonpb.TypedValue.
func anyToTypedValue(v any) *commonpb.TypedValue {
	if v == nil {
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_NullValue{}}
	}
	switch val := v.(type) {
	case string:
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_StringValue{StringValue: val}}
	case float64:
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_DoubleValue{DoubleValue: val}}
	case bool:
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_BoolValue{BoolValue: val}}
	case int:
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_IntValue{IntValue: int64(val)}}
	case int64:
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_IntValue{IntValue: val}}
	case []any:
		items := make([]*commonpb.TypedValue, len(val))
		for i, item := range val {
			items[i] = anyToTypedValue(item)
		}
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_ArrayValue{ArrayValue: &commonpb.TypedArray{Items: items}}}
	case map[string]any:
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_MapValue{MapValue: mapToTypedMap(val)}}
	default:
		// Fallback: convert to string
		return &commonpb.TypedValue{Kind: &commonpb.TypedValue_StringValue{StringValue: fmt.Sprintf("%v", v)}}
	}
}

// typedMapToMap converts *commonpb.TypedMap back to map[string]any.
func typedMapToMap(tm *commonpb.TypedMap) map[string]any {
	if tm == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range tm.Entries {
		result[k] = typedValueToAny(v)
	}
	return result
}

// typedValueToAny converts *commonpb.TypedValue to any.
func typedValueToAny(tv *commonpb.TypedValue) any {
	if tv == nil {
		return nil
	}
	switch v := tv.Kind.(type) {
	case *commonpb.TypedValue_NullValue:
		return nil
	case *commonpb.TypedValue_StringValue:
		return v.StringValue
	case *commonpb.TypedValue_IntValue:
		return v.IntValue
	case *commonpb.TypedValue_DoubleValue:
		return v.DoubleValue
	case *commonpb.TypedValue_BoolValue:
		return v.BoolValue
	case *commonpb.TypedValue_BytesValue:
		return v.BytesValue
	case *commonpb.TypedValue_ArrayValue:
		if v.ArrayValue == nil {
			return nil
		}
		result := make([]any, len(v.ArrayValue.Items))
		for i, item := range v.ArrayValue.Items {
			result[i] = typedValueToAny(item)
		}
		return result
	case *commonpb.TypedValue_MapValue:
		return typedMapToMap(v.MapValue)
	default:
		return nil
	}
}
