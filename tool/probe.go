// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package tool

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ProbeRoot returns true if the current process is running with root privileges.
// This check is performed by verifying if the effective user ID is 0.
//
// Example:
//
//	if tool.ProbeRoot() {
//		// Running as root, can perform privileged operations
//	}
func ProbeRoot() bool {
	return os.Geteuid() == 0
}

// ProbeSudo returns true if passwordless sudo is available for the current user.
// This is determined by attempting to run 'sudo -n true' with a 2-second timeout.
// The -n flag prevents sudo from prompting for a password.
//
// This function is fast and safe - it does not attempt privilege escalation,
// only checks if sudo access is already configured.
//
// Example:
//
//	if tool.ProbeSudo() {
//		// Passwordless sudo available, can elevate privileges
//	}
func ProbeSudo() bool {
	// Create a context with timeout to ensure this check doesn't hang
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use sudo -n (non-interactive) to check if passwordless sudo is available
	// The 'true' command is a no-op that always succeeds
	cmd := exec.CommandContext(ctx, "sudo", "-n", "true")

	// Run the command and check if it succeeded
	err := cmd.Run()
	return err == nil
}

// ProbeRawSocket returns true if the process has the capability to create raw sockets.
// Raw socket capability is required for low-level network operations such as:
//   - Custom protocol implementations
//   - ICMP ping operations
//   - Packet crafting and injection
//   - Network scanning tools (e.g., mytool, masscan)
//
// The check is performed by:
//  1. Returning true if running as root (CAP_NET_RAW is implicitly available)
//  2. Attempting to create an ICMP raw socket and checking for success
//
// This function is safe - it cleans up any created sockets and does not
// send any network traffic.
//
// Example:
//
//	if !tool.ProbeRawSocket() {
//		return fmt.Errorf("raw socket capability required (run as root or with CAP_NET_RAW)")
//	}
func ProbeRawSocket() bool {
	// Root users have all capabilities including CAP_NET_RAW
	if ProbeRoot() {
		return true
	}

	// Try to create a raw ICMP socket
	// This will succeed if the process has CAP_NET_RAW capability
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		return false
	}

	// Clean up the socket immediately
	_ = syscall.Close(fd)
	return true
}
