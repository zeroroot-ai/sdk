// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import "github.com/golang-jwt/jwt/v5"

// MissionRunOf reads the mission run claim from the payload of a grant without
// checking the signature. The daemon verifies the grant on every call. The
// client only needs the run to name it in the context of each callback. A token
// that does not parse, and a grant minted before the claim existed, has no run.
func MissionRunOf(token string) string {
	mc := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, mc); err != nil {
		return ""
	}
	claims, err := claimsFromMap(mc)
	if err != nil {
		return ""
	}
	return claims.MissionRunID
}
