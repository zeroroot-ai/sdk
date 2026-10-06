// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema

import (
	"reflect"
	"testing"
	"time"
)

func TestBuilders(t *testing.T) {
	if got := Any(); !reflect.DeepEqual(got, JSON{}) {
		t.Errorf("Any() = %+v, want an empty schema", got)
	}
	if got := StringWithDesc("a name"); got.Type != "string" || got.Description != "a name" {
		t.Errorf("StringWithDesc() = %+v", got)
	}
	if got := Int(); got.Type != "integer" {
		t.Errorf("Int() = %+v", got)
	}
	arr := Array(Int())
	if arr.Type != "array" || arr.Items == nil || arr.Items.Type != "integer" {
		t.Errorf("Array(Int()) = %+v", arr)
	}
	obj := Object(map[string]JSON{"n": Int()}, "n")
	if obj.Type != "object" || obj.Properties["n"].Type != "integer" || !reflect.DeepEqual(obj.Required, []string{"n"}) {
		t.Errorf("Object() = %+v", obj)
	}
}

type sample struct {
	Name     string            `json:"name" description:"the name"`
	Count    int               `json:"count,omitempty"`
	Ratio    float64           `json:"ratio"`
	On       bool              `json:"on"`
	When     time.Time         `json:"when"`
	Tags     []string          `json:"tags"`
	Labels   map[string]string `json:"labels"`
	Inner    *inner            `json:"inner,omitempty"`
	Untagged uint8
	Anything any    `json:"anything"`
	Skipped  string `json:"-"`
	hidden   string
	Fn       func() `json:"fn"`
}

type inner struct {
	Value string `json:"value"`
}

func TestFromType(t *testing.T) {
	if got := FromType(nil); !reflect.DeepEqual(got, JSON{}) {
		t.Fatalf("FromType(nil) = %+v", got)
	}
	s := FromType(&sample{hidden: "x"})
	if s.Type != "object" {
		t.Fatalf("type = %q, want object", s.Type)
	}
	want := map[string]string{
		"name": "string", "count": "integer", "ratio": "number", "on": "boolean",
		"when": "string", "tags": "array", "labels": "object", "inner": "object",
		"Untagged": "integer", "anything": "", "fn": "",
	}
	for name, typ := range want {
		p, ok := s.Properties[name]
		if !ok {
			t.Errorf("property %q missing", name)
			continue
		}
		if p.Type != typ {
			t.Errorf("property %q type = %q, want %q", name, p.Type, typ)
		}
	}
	for _, gone := range []string{"Skipped", "-", "hidden"} {
		if _, ok := s.Properties[gone]; ok {
			t.Errorf("property %q must not appear", gone)
		}
	}
	if s.Properties["when"].Format != "date-time" {
		t.Errorf("time.Time format = %q", s.Properties["when"].Format)
	}
	if s.Properties["name"].Description != "the name" {
		t.Errorf("description tag not applied: %+v", s.Properties["name"])
	}
	if s.Properties["tags"].Items == nil || s.Properties["tags"].Items.Type != "string" {
		t.Errorf("tags items = %+v", s.Properties["tags"].Items)
	}
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	if required["count"] || required["inner"] {
		t.Errorf("omitempty fields must not be required: %v", s.Required)
	}
	if !required["name"] || !required["Untagged"] {
		t.Errorf("required = %v, want name and Untagged", s.Required)
	}
}
