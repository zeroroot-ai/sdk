// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

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

// ClaimsOf reads the claims of a grant WITHOUT checking the signature. The
// daemon verifies the grant on every call. The client uses the claims to know
// the subject, the mission, the task and the expiry, for example to renew the
// grant before it ends.
func ClaimsOf(token string) (*Claims, error) {
	mc := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, mc); err != nil {
		return nil, fmt.Errorf("capabilitygrant: read grant claims: %w", err)
	}
	claims, err := claimsFromMap(mc)
	if err != nil {
		return nil, err
	}
	return &claims, nil
}
