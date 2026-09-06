// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"

	"github.com/zeroroot-ai/sdk/types"
)

// CapabilityProvider is an optional interface that tools can implement
// to advertise their runtime capabilities and privilege requirements.
// This allows the framework to understand what operations a tool can perform
// based on the execution environment (e.g., root access, raw socket capability).
//
// Tools that require specific privileges (like mytool needing raw sockets for SYN scans)
// should implement this interface to report available capabilities and suggest
// fallback options when operations are blocked.
//
// Example implementation:
//
//	type NmapTool struct{}
//
//	func (t *NmapTool) Capabilities(ctx context.Context) *types.Capabilities {
//	    caps := types.NewCapabilities()
//	    caps.HasRoot = os.Geteuid() == 0
//	    caps.CanRawSocket = checkRawSocketAccess()
//
//	    if !caps.CanRawSocket {
//	        caps.BlockedArgs = []string{"-sS", "-sU", "-O"}
//	        caps.ArgAlternatives = map[string]string{
//	            "-sS": "-sT", // SYN scan -> TCP connect scan
//	        }
//	        caps.Features = map[string]bool{
//	            "stealth_scan": false,
//	            "os_detection": false,
//	        }
//	    }
//	    return caps
//	}
type CapabilityProvider interface {
	// Capabilities returns the runtime privileges and features available to this tool.
	// The returned Capabilities struct describes what operations can be performed,
	// which arguments are blocked, and what alternatives are available.
	//
	// This method may perform runtime checks (e.g., testing raw socket creation,
	// checking uid, verifying sudo access) to accurately report current capabilities.
	//
	// Returns nil if the tool does not have specific capability requirements
	// or if capability detection is not applicable.
	Capabilities(ctx context.Context) *types.Capabilities
}

// GetCapabilities retrieves capabilities from a tool if it implements CapabilityProvider.
// This helper function safely type asserts and calls the Capabilities method.
//
// Returns the tool's capabilities if it implements CapabilityProvider, nil otherwise.
// A nil return indicates either:
//   - The tool does not implement CapabilityProvider
//   - The tool returned nil from its Capabilities method
//   - The tool has no specific capability requirements
//
// Usage:
//
//	caps := tool.GetCapabilities(ctx, myTool)
//	if caps != nil && caps.HasPrivilegedAccess() {
//	    // Tool has elevated privileges
//	}
func GetCapabilities(ctx context.Context, t Tool) *types.Capabilities {
	if provider, ok := t.(CapabilityProvider); ok {
		return provider.Capabilities(ctx)
	}
	return nil
}
