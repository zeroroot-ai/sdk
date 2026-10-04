// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package secretenv is the one rule for naming the environment variable a
// declared secret arrives in.
//
// A mission declares which named tenant secrets its components may receive
// (gibson#485). The daemon resolves each name as itself at dispatch and hands
// the VALUE to the component in its environment. The environment is the channel
// on purpose: a tool's input JSON is captured with the tool call, so a secret
// placed there would be stored and displayed, while the mission definition
// carries only the name.
//
// Two programs therefore need the same name: the daemon that writes the variable
// and the component that reads it. The rule lives here so there is one copy of
// it. A component resolves the secret the mission handed it with Lookup, and
// never reconstructs the name itself.
package secretenv

import (
	"os"
	"strings"
)

// Prefix is the environment namespace a component receives its declared
// secrets under. A secret named "goat-kubeconfig" arrives as
// GIBSON_SECRET_GOAT_KUBECONFIG.
const Prefix = "GIBSON_SECRET_"

// Key is a secret's name as an environment variable: upper-cased, with every
// character outside A-Z, 0-9 and underscore folded to an underscore, behind
// Prefix.
//
// The fold is not injective. Two different names can produce one key ("a-b" and
// "a.b" both become A_B). The daemon refuses that collision at dispatch rather
// than letting one name silently win, because a component handed the wrong
// credential under the right name is worse than a mission that fails to start.
func Key(name string) string {
	var b strings.Builder
	b.Grow(len(Prefix) + len(name))
	b.WriteString(Prefix)
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Lookup reads the value of the named secret from this process's environment.
//
// The second result is false when the mission did not declare the secret for
// this component, or declared it for a different one. It is also false for a
// variable that is present but empty: the daemon refuses to dispatch with a
// blank credential, so an empty value means something other than the daemon set
// it, and treating it as a credential would be worse than reporting it missing.
//
// A component that cannot work without the secret reports the name it wanted
// and stops. It must not fall back to an ambient credential — the mission's
// declaration is the whole authorization record for the hand-over, and a
// fallback would run work nobody declared.
func Lookup(name string) (string, bool) {
	v := os.Getenv(Key(name))
	if v == "" {
		return "", false
	}
	return v, true
}
