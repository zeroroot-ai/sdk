// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// MissionRunOf reads the mission run claim from the payload of a grant without
// checking the signature. The daemon verifies the grant on every call. The
// client only needs the run to name it in the context of each callback. A token
// that does not parse, and a grant minted before the claim existed, has no run.
func MissionRunOf(token string) string { return addressingClaims(token).MissionRunID }

// addressingClaims reads the claims that address a call from the payload of a
// grant, with no signature check. It leaves every other field empty.
func addressingClaims(token string) Claims {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}
	}
	var payload jwt.MapClaims
	if json.Unmarshal(raw, &payload) != nil {
		return Claims{}
	}
	return Claims{
		Subject:      stringClaim(payload, "sub"),
		MissionID:    stringClaim(payload, "mission_id"),
		TaskID:       stringClaim(payload, "task_id"),
		MissionRunID: stringClaim(payload, MissionRunClaim),
	}
}
