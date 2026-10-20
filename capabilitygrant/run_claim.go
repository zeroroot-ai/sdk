// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// MissionRunOf reads the mission run claim from the payload of a grant without
// checking the signature. The daemon verifies the grant on every call. The
// client only needs the run to name it in the context of each callback. A token
// that does not parse, and a grant minted before the claim existed, has no run.
func MissionRunOf(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	run, _ := payload[MissionRunClaim].(string)
	return run
}
