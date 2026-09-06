// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestNewTenantID_Valid(t *testing.T) {
	cases := []string{
		"acme",
		"acme-corp",
		"a1b2c3",
		"customer-42",
		"u_under_score",
		"a",
		"alpha-beta-gamma",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			tid, err := NewTenantID(in)
			if err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if tid.String() != in {
				t.Fatalf("got %q want %q", tid.String(), in)
			}
			if tid.IsZero() {
				t.Fatalf("constructed tenant should not be zero")
			}
			if tid.IsSystem() {
				t.Fatalf("constructed tenant should not be system tenant")
			}
		})
	}
}

func TestNewTenantID_Trims(t *testing.T) {
	tid, err := NewTenantID("  acme  \n")
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if tid.String() != "acme" {
		t.Fatalf("expected trim, got %q", tid.String())
	}
}

func TestNewTenantID_RejectEmpty(t *testing.T) {
	for _, in := range []string{"", " ", "\t", "\n", "   \t  "} {
		_, err := NewTenantID(in)
		if err == nil {
			t.Fatalf("expected rejection of %q", in)
		}
		if !errors.Is(err, ErrInvalidTenant) {
			t.Fatalf("expected ErrInvalidTenant, got %v", err)
		}
	}
}

func TestNewTenantID_RejectOversize(t *testing.T) {
	big := strings.Repeat("a", MaxTenantIDLen+1)
	_, err := NewTenantID(big)
	if err == nil {
		t.Fatalf("expected rejection of oversize")
	}
	if !errors.Is(err, ErrInvalidTenant) {
		t.Fatalf("expected ErrInvalidTenant, got %v", err)
	}
}

func TestNewTenantID_RejectInvalidPattern(t *testing.T) {
	cases := []string{
		"1leading-digit",
		"UPPERCASE",
		"-leading-hyphen",
		"trailing-",
		"_leading-underscore",
		"trailing_",
		"contains spaces",
		"contains/slash",
		"contains.dot",
		"contains:colon",
		"contains$dollar",
		"containséunicode",
		"--double-hyphen", // grammar requires single separator between segments
		"__double-under",
		"a..b",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			_, err := NewTenantID(in)
			if err == nil {
				t.Fatalf("expected rejection of %q", in)
			}
			if !errors.Is(err, ErrInvalidTenant) {
				t.Fatalf("expected ErrInvalidTenant, got %v", err)
			}
		})
	}
}

func TestNewTenantID_RejectReservedSystem(t *testing.T) {
	_, err := NewTenantID("_system")
	if err == nil {
		t.Fatalf("expected refusal of reserved system tenant via public path")
	}
	if !errors.Is(err, ErrInvalidTenant) {
		t.Fatalf("expected ErrInvalidTenant, got %v", err)
	}
	if !strings.Contains(err.Error(), "auth.SystemTenant") {
		t.Fatalf("expected error to point caller at auth.SystemTenant, got %v", err)
	}
}

func TestSystemTenant_IsSystem(t *testing.T) {
	if !SystemTenant.IsSystem() {
		t.Fatal("SystemTenant should be system")
	}
	if SystemTenant.IsZero() {
		t.Fatal("SystemTenant should not be zero")
	}
	if SystemTenant.String() != "_system" {
		t.Fatalf("SystemTenant string mismatch: %q", SystemTenant.String())
	}
}

func TestZeroTenantID(t *testing.T) {
	var zero TenantID
	if !zero.IsZero() {
		t.Fatal("zero value should report IsZero")
	}
	if zero.IsSystem() {
		t.Fatal("zero value should not report IsSystem")
	}
	if zero.String() != "" {
		t.Fatalf("zero string should be empty, got %q", zero.String())
	}
}

func TestTenantID_Equal(t *testing.T) {
	a := MustNewTenantID("acme")
	a2, _ := NewTenantID("acme")
	b := MustNewTenantID("bigcorp")
	if !a.Equal(a2) {
		t.Fatal("equal tenants should compare equal")
	}
	if a.Equal(b) {
		t.Fatal("different tenants should not compare equal")
	}
	if a.Equal(SystemTenant) {
		t.Fatal("user tenant should not equal system tenant")
	}
}

func TestTenantID_MarshalText(t *testing.T) {
	tid := MustNewTenantID("acme")
	b, err := tid.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "acme" {
		t.Fatalf("got %q, want acme", string(b))
	}
}

func TestTenantID_AsMapKey(t *testing.T) {
	a := MustNewTenantID("acme")
	a2 := MustNewTenantID("acme")
	m := map[TenantID]int{a: 1}
	if m[a2] != 1 {
		t.Fatal("equal TenantIDs should hit the same map entry")
	}
}

func TestMustNewTenantID_PanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on invalid input")
		}
	}()
	_ = MustNewTenantID("")
}

// TestTenantID_NoUnmarshalText documents the absence of UnmarshalText. If
// somebody adds UnmarshalText to TenantID later, this test fails to flag
// the regression. The TenantID type intentionally has no deserializer
// that bypasses NewTenantID.
func TestTenantID_NoUnmarshalText(t *testing.T) {
	type unmarshaler interface {
		UnmarshalText(text []byte) error
	}
	var tid TenantID
	if _, ok := any(&tid).(unmarshaler); ok {
		t.Fatal("TenantID must not implement UnmarshalText (would bypass NewTenantID validation)")
	}
}
