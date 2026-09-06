// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package types

// Capabilities represents the runtime privileges and features available to a tool.
// It captures what operations the tool can perform based on the execution environment,
// including privilege levels, network capabilities, and feature availability.
type Capabilities struct {
	// HasRoot indicates the tool is running as uid 0 (root user).
	HasRoot bool `json:"has_root"`

	// HasSudo indicates passwordless sudo access is available.
	// This allows privilege escalation without user interaction.
	HasSudo bool `json:"has_sudo"`

	// CanRawSocket indicates the ability to create raw network sockets.
	// This requires CAP_NET_RAW capability on Linux or equivalent privileges.
	CanRawSocket bool `json:"can_raw_socket"`

	// Features contains tool-specific feature availability flags.
	// Keys are feature names, values indicate if the feature is available.
	// Example: {"stealth_scan": true, "os_detection": false}
	Features map[string]bool `json:"features,omitempty"`

	// BlockedArgs lists command-line arguments that cannot be used
	// due to insufficient privileges or missing capabilities.
	// Example: ["-sS", "-O"] for mytool without raw socket access
	BlockedArgs []string `json:"blocked_args,omitempty"`

	// ArgAlternatives maps blocked arguments to their safer alternatives.
	// Allows graceful degradation by suggesting equivalent commands.
	// Example: {"-sS": "-sT"} maps SYN scan to TCP connect scan
	ArgAlternatives map[string]string `json:"arg_alternatives,omitempty"`
}

// IsArgBlocked checks if a specific argument is in the BlockedArgs list.
// Returns true if the argument cannot be used with current privileges.
func (c *Capabilities) IsArgBlocked(arg string) bool {
	for _, blocked := range c.BlockedArgs {
		if blocked == arg {
			return true
		}
	}
	return false
}

// GetAlternative returns the suggested alternative for a blocked argument.
// Returns the alternative and true if one exists, empty string and false otherwise.
// This allows tools to automatically substitute safer equivalent operations.
func (c *Capabilities) GetAlternative(arg string) (string, bool) {
	if c.ArgAlternatives == nil {
		return "", false
	}
	alt, exists := c.ArgAlternatives[arg]
	return alt, exists
}

// HasFeature checks if a specific tool feature is available.
// Returns false if the Features map is nil or the feature is not present.
func (c *Capabilities) HasFeature(feature string) bool {
	if c.Features == nil {
		return false
	}
	enabled, exists := c.Features[feature]
	return exists && enabled
}

// HasPrivilegedAccess returns true if the tool has any form of elevated privileges.
// This includes root access, sudo access, or raw socket capability.
func (c *Capabilities) HasPrivilegedAccess() bool {
	return c.HasRoot || c.HasSudo || c.CanRawSocket
}

// NewCapabilities creates a new Capabilities struct with empty collections initialized.
// This prevents nil pointer dereferences when adding features or blocked arguments.
func NewCapabilities() *Capabilities {
	return &Capabilities{
		Features:        make(map[string]bool),
		BlockedArgs:     make([]string, 0),
		ArgAlternatives: make(map[string]string),
	}
}
