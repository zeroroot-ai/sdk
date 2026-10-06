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
