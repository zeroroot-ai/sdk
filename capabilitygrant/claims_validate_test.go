// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package capabilitygrant

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/zeroroot-ai/sdk/auth"
)

// Validate refuses a claim set with each required field missing or wrong,
// and accepts a complete one.
func TestClaims_Validate_EachRule(t *testing.T) {
	now := time.Now().UTC()
	valid := func() Claims {
		return Claims{
			Issuer:    "iss",
			Audience:  "aud",
			Subject:   "sub",
			Tenant:    auth.MustNewTenantID("acme"),
			MissionID: "m",
			TaskID:    "t",
			JTI:       "j",
			IssuedAt:  now,
			ExpiresAt: now.Add(time.Minute),
		}
	}
	if err := valid().Validate(now, "iss", "aud"); err != nil {
		t.Fatalf("a complete claim set was refused: %v", err)
	}
	cases := map[string]struct {
		edit func(*Claims)
		want error
	}{
		"no issuer":         {func(c *Claims) { c.Issuer = "" }, ErrClaimsInvalid},
		"other issuer":      {func(c *Claims) { c.Issuer = "other" }, ErrClaimsInvalid},
		"no audience":       {func(c *Claims) { c.Audience = "" }, ErrClaimsInvalid},
		"other audience":    {func(c *Claims) { c.Audience = "other" }, ErrClaimsInvalid},
		"no subject":        {func(c *Claims) { c.Subject = "" }, ErrClaimsInvalid},
		"no tenant":         {func(c *Claims) { c.Tenant = auth.TenantID{} }, ErrClaimsInvalid},
		"no mission":        {func(c *Claims) { c.MissionID = "" }, ErrClaimsInvalid},
		"no task":           {func(c *Claims) { c.TaskID = "" }, ErrClaimsInvalid},
		"no jti":            {func(c *Claims) { c.JTI = "" }, ErrClaimsInvalid},
		"no iat":            {func(c *Claims) { c.IssuedAt = time.Time{} }, ErrClaimsInvalid},
		"no exp":            {func(c *Claims) { c.ExpiresAt = time.Time{} }, ErrClaimsInvalid},
		"exp before iat":    {func(c *Claims) { c.ExpiresAt = now.Add(-time.Minute) }, ErrClaimsInvalid},
		"lifetime too long": {func(c *Claims) { c.ExpiresAt = now.Add(MaxLifetime + time.Minute) }, ErrClaimsInvalid},
		"expired": {func(c *Claims) {
			c.IssuedAt = now.Add(-2 * time.Minute)
			c.ExpiresAt = now.Add(-time.Minute)
		}, ErrExpired},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid()
			tc.edit(&c)
			if err := c.Validate(now, "iss", "aud"); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// audienceClaim reads a string aud and the first string of a list aud. Any
// other form reads as no audience, which Validate then refuses.
func TestAudienceClaim_Forms(t *testing.T) {
	cases := map[string]struct {
		v    any
		want string
	}{
		"string":           {"aud", "aud"},
		"list":             {[]any{"aud", "other"}, "aud"},
		"empty list":       {[]any{}, ""},
		"list of a number": {[]any{42}, ""},
		"number":           {42, ""},
		"absent":           {nil, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mc := jwt.MapClaims{}
			if tc.v != nil {
				mc["aud"] = tc.v
			}
			if got := audienceClaim(mc, "aud"); got != tc.want {
				t.Fatalf("audienceClaim = %q, want %q", got, tc.want)
			}
		})
	}
}

// GibsonDir reads GIBSON_HOME first, and the install path of a component
// with no name is default.runtime.json.
func TestGibsonDir_AndTheDefaultInstallName(t *testing.T) {
	t.Setenv(EnvGibsonHome, "/opt/gibson")
	dir, err := GibsonDir()
	if err != nil || dir != "/opt/gibson" {
		t.Fatalf("GibsonDir = %q, %v; want /opt/gibson", dir, err)
	}
	p, err := RuntimeInstallPath("agent", "")
	if err != nil || p != "/opt/gibson/agent/default.runtime.json" {
		t.Fatalf("RuntimeInstallPath = %q, %v", p, err)
	}
	t.Setenv(EnvGibsonHome, "")
	t.Setenv("HOME", "/home/someone")
	if dir, err := GibsonDir(); err != nil || dir != "/home/someone/.gibson" {
		t.Fatalf("GibsonDir with no GIBSON_HOME = %q, %v", dir, err)
	}
}
