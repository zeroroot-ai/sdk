// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package schema derives a JSON-Schema descriptor from a plain Go type by
// reflection. It is the heart of the Gibson plugin SDK's Go-first authoring
// model (ADR-0065 R4): a plugin author writes ONE handler.go with typed Go
// request/response structs, and the build derives the tool schema/descriptor
// gibson's dispatch consumes — no hand-written .proto, no per-method codegen.
//
// The derived document is a JSON-Schema draft-2020-12 subset. It is the exact
// format placed in RegisterComponentRequest.method_descriptors[].input_schema_json
// so the daemon's tool catalog can surface the method's argument contract to an
// agent without invoking the plugin.
//
// # Supported Go shapes (v1)
//
// Derivation walks the following and nothing else, keeping the surface small,
// total, and predictable:
//
//   - scalars: string, bool, all signed/unsigned integers, float32/float64;
//   - structs of the above, nested to any depth;
//   - slices and arrays (→ JSON array with a derived item schema);
//   - maps with a string key (→ JSON object with additionalProperties);
//   - pointers (unwrapped; a pointer field is optional);
//   - time.Time (→ string, format "date-time"); and
//   - json.RawMessage / []byte (→ string; []byte is base64 in encoding/json).
//
// A struct field is REQUIRED unless it is a pointer or carries the
// `,omitempty` json tag; a field tagged `json:"-"` is skipped. Field names
// follow the `json` tag when present, else the Go field name verbatim.
//
// # Unsupported shapes (v1) — these return an error, they do not silently pass
//
//   - interface types, including any/interface{} — a union has no single
//     JSON-Schema shape; model the alternatives as distinct methods or an
//     explicit tagged struct instead;
//   - channels, functions, and complex numbers — not serialisable;
//   - maps with a non-string key — JSON object keys are strings only.
//
// Streaming request/response bodies are out of scope for v1 entirely: a
// handler is one request struct in, one response struct out. These edge cases
// are called out in the package doc deliberately so an author hits a clear
// build-time error rather than a surprising runtime one.
package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Schema is a JSON-Schema (draft-2020-12 subset) node. Only the fields the
// derivation emits are modelled; the zero value marshals to an empty object.
//
// The struct marshals to canonical JSON-Schema. Property order within an
// object is preserved via the ordered Properties list so the emitted document
// is deterministic for a given Go type (important for golden tests and stable
// registration payloads).
type Schema struct {
	// Type is the JSON-Schema type keyword: "object", "array", "string",
	// "integer", "number", or "boolean".
	Type string

	// Description is an optional human-readable annotation. It is populated
	// from a field's `desc:"..."` struct tag when present.
	Description string

	// Format is an optional JSON-Schema format annotation, e.g. "date-time"
	// for time.Time.
	Format string

	// Properties is the ordered set of an object's properties. Non-nil only
	// when Type == "object" and the source is a struct.
	Properties []Property

	// Required lists the required property names of an object, in declaration
	// order. Non-nil only when Type == "object".
	Required []string

	// Items is the element schema of an array. Non-nil only when
	// Type == "array".
	Items *Schema

	// AdditionalProperties is the value schema of a string-keyed map. Non-nil
	// only when Type == "object" and the source is a map.
	AdditionalProperties *Schema
}

// Property is one named property of an object schema, retaining declaration
// order.
type Property struct {
	Name   string
	Schema *Schema
}

// MarshalJSON renders the Schema as canonical JSON-Schema. Object properties
// are emitted in declaration order.
func (s *Schema) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Items != nil {
		m["items"] = s.Items
	}
	if s.AdditionalProperties != nil {
		m["additionalProperties"] = s.AdditionalProperties
	}
	if s.Properties != nil {
		// json.Marshal sorts map keys, which would lose declaration order, so
		// build the properties object as ordered raw JSON.
		var b strings.Builder
		b.WriteByte('{')
		for i, p := range s.Properties {
			if i > 0 {
				b.WriteByte(',')
			}
			key, err := json.Marshal(p.Name)
			if err != nil {
				return nil, err
			}
			val, err := json.Marshal(p.Schema)
			if err != nil {
				return nil, err
			}
			b.Write(key)
			b.WriteByte(':')
			b.Write(val)
		}
		b.WriteByte('}')
		m["properties"] = json.RawMessage(b.String())
	}
	if s.Required != nil {
		m["required"] = s.Required
	}
	return json.Marshal(m)
}

// timeType is compared by identity to special-case time.Time.
var timeType = reflect.TypeOf(time.Time{})

// rawMessageType is compared by identity to special-case json.RawMessage.
var rawMessageType = reflect.TypeOf(json.RawMessage(nil))

// Derive builds a [Schema] from a Go type. It returns an error for any shape
// the v1 model does not support (see the package doc).
//
// The top-level type is typically a struct (an object schema), but any
// supported shape is accepted.
func Derive(t reflect.Type) (*Schema, error) {
	return derive(t, make(map[reflect.Type]bool))
}

// DeriveJSON is Derive followed by json.Marshal of the result. It returns the
// canonical JSON-Schema document as bytes.
func DeriveJSON(t reflect.Type) ([]byte, error) {
	s, err := Derive(t)
	if err != nil {
		return nil, err
	}
	return json.Marshal(s)
}

// derive is the recursive worker. seen guards against recursive struct types
// (e.g. a tree node that points at itself), which JSON-Schema cannot express as
// a finite inlined document; such a type is rejected with a clear error.
func derive(t reflect.Type, seen map[reflect.Type]bool) (*Schema, error) {
	// Unwrap pointers: a *T serialises exactly as its T (possibly null).
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Identity special-cases, checked before the Kind switch.
	switch t {
	case timeType:
		return &Schema{Type: "string", Format: "date-time"}, nil
	case rawMessageType:
		return &Schema{Type: "string"}, nil
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}, nil
	case reflect.Bool:
		return &Schema{Type: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}, nil

	case reflect.Slice, reflect.Array:
		// []byte marshals as a base64 string in encoding/json, not an array.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: "string"}, nil
		}
		item, err := derive(t.Elem(), seen)
		if err != nil {
			return nil, fmt.Errorf("array element: %w", err)
		}
		return &Schema{Type: "array", Items: item}, nil

	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type %s is not a string; JSON object keys must be strings", t.Key())
		}
		val, err := derive(t.Elem(), seen)
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		return &Schema{Type: "object", AdditionalProperties: val}, nil

	case reflect.Struct:
		return deriveStruct(t, seen)

	default:
		return nil, fmt.Errorf("unsupported Go kind %s for type %s: only structs of scalars, "+
			"slices, string-keyed maps, and nested structs are supported (no interfaces/unions, "+
			"channels, functions, or complex numbers)", t.Kind(), t)
	}
}

// deriveStruct builds an object schema from an exported struct's fields,
// honouring json tags for names and omitempty, and `desc` tags for
// descriptions.
func deriveStruct(t reflect.Type, seen map[reflect.Type]bool) (*Schema, error) {
	if seen[t] {
		return nil, fmt.Errorf("recursive struct type %s cannot be expressed as a finite JSON schema", t)
	}
	seen[t] = true
	defer delete(seen, t)

	s := &Schema{Type: "object", Properties: []Property{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			// Unexported field; encoding/json ignores it, so skip.
			continue
		}
		name, omitempty, skip := jsonName(f)
		if skip {
			continue
		}

		// A pointer or ,omitempty field is optional; every other field is
		// required (mirrors what a JSON consumer can rely on being present).
		optional := omitempty || f.Type.Kind() == reflect.Pointer

		fieldSchema, err := derive(f.Type, seen)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		if d := f.Tag.Get("desc"); d != "" {
			fieldSchema.Description = d
		}
		s.Properties = append(s.Properties, Property{Name: name, Schema: fieldSchema})
		if !optional {
			s.Required = append(s.Required, name)
		}
	}
	return s, nil
}

// jsonName resolves a struct field's JSON property name and options from its
// `json` tag. skip is true for `json:"-"`.
func jsonName(f reflect.StructField) (name string, omitempty, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	name = f.Name
	if tag == "" {
		return name, false, false
	}
	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		name = parts[0]
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty, false
}
