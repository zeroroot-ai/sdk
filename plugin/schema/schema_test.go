// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package schema_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/plugin/schema"
)

// CreateIncidentRequest is a representative plugin request struct exercising
// every supported shape: scalars, a nested struct, a slice, a string-keyed
// map, an optional pointer, and json-tag renaming/omitempty.
type CreateIncidentRequest struct {
	Title     string            `json:"title"`
	Severity  int               `json:"severity" desc:"1 (highest) to 5 (lowest)"`
	Urgent    bool              `json:"urgent"`
	Score     float64           `json:"score"`
	Tags      []string          `json:"tags"`
	Labels    map[string]string `json:"labels"`
	Assignee  *Person           `json:"assignee,omitempty"`
	Reporter  Person            `json:"reporter"`
	Internal  string            `json:"-"`
	openField string            //nolint:unused // exercises unexported-field skip
}

type Person struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

func TestDerive_RepresentativeStruct(t *testing.T) {
	s, err := schema.Derive(reflect.TypeOf(CreateIncidentRequest{}))
	require.NoError(t, err)

	assert.Equal(t, "object", s.Type)

	// Property order is preserved and json:"-"/unexported fields are skipped.
	var gotNames []string
	byName := map[string]*schema.Schema{}
	for _, p := range s.Properties {
		gotNames = append(gotNames, p.Name)
		byName[p.Name] = p.Schema
	}
	assert.Equal(t, []string{"title", "severity", "urgent", "score", "tags", "labels", "assignee", "reporter"}, gotNames)

	assert.Equal(t, "string", byName["title"].Type)
	assert.Equal(t, "integer", byName["severity"].Type)
	assert.Equal(t, "1 (highest) to 5 (lowest)", byName["severity"].Description)
	assert.Equal(t, "boolean", byName["urgent"].Type)
	assert.Equal(t, "number", byName["score"].Type)

	// Slice → array with item schema.
	require.Equal(t, "array", byName["tags"].Type)
	require.NotNil(t, byName["tags"].Items)
	assert.Equal(t, "string", byName["tags"].Items.Type)

	// string-keyed map → object with additionalProperties.
	require.Equal(t, "object", byName["labels"].Type)
	require.NotNil(t, byName["labels"].AdditionalProperties)
	assert.Equal(t, "string", byName["labels"].AdditionalProperties.Type)

	// Nested struct → object; pointer field is optional.
	assert.Equal(t, "object", byName["assignee"].Type)
	assert.Equal(t, "object", byName["reporter"].Type)

	// Required = non-pointer, non-omitempty fields, in declaration order.
	assert.Equal(t, []string{"title", "severity", "urgent", "score", "tags", "labels", "reporter"}, s.Required)
}

func TestDerive_NestedRequiredAndOmitempty(t *testing.T) {
	s, err := schema.Derive(reflect.TypeOf(Person{}))
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, s.Required, "email is omitempty and must not be required")
}

func TestDerive_TimeIsDateTimeString(t *testing.T) {
	type withTime struct {
		At time.Time `json:"at"`
	}
	s, err := schema.Derive(reflect.TypeOf(withTime{}))
	require.NoError(t, err)
	at := s.Properties[0].Schema
	assert.Equal(t, "string", at.Type)
	assert.Equal(t, "date-time", at.Format)
}

func TestDerive_ByteSliceIsString(t *testing.T) {
	type withBytes struct {
		Blob []byte `json:"blob"`
	}
	s, err := schema.Derive(reflect.TypeOf(withBytes{}))
	require.NoError(t, err)
	assert.Equal(t, "string", s.Properties[0].Schema.Type)
}

func TestDerive_RejectsInterfaceUnion(t *testing.T) {
	type withAny struct {
		Payload any `json:"payload"`
	}
	_, err := schema.Derive(reflect.TypeOf(withAny{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interfaces/unions")
}

func TestDerive_RejectsNonStringMapKey(t *testing.T) {
	type withBadMap struct {
		M map[int]string `json:"m"`
	}
	_, err := schema.Derive(reflect.TypeOf(withBadMap{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "string")
}

func TestDerive_RejectsRecursiveStruct(t *testing.T) {
	_, err := schema.Derive(reflect.TypeOf(node{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recursive")
}

type node struct {
	Next *node `json:"next"`
}

func TestDerive_RejectsChannel(t *testing.T) {
	type withChan struct {
		C chan int `json:"c"`
	}
	_, err := schema.Derive(reflect.TypeOf(withChan{}))
	require.Error(t, err)
}

// TestDeriveJSON_CanonicalOutput asserts the emitted JSON-Schema is exactly the
// canonical document, with properties in declaration order (not sorted).
func TestDeriveJSON_CanonicalOutput(t *testing.T) {
	type req struct {
		Zebra string `json:"zebra"`
		Alpha int    `json:"alpha"`
	}
	got, err := schema.DeriveJSON(reflect.TypeOf(req{}))
	require.NoError(t, err)

	const want = `{"properties":{"zebra":{"type":"string"},"alpha":{"type":"integer"}},"required":["zebra","alpha"],"type":"object"}`
	assert.JSONEq(t, want, string(got))

	// And property order in the raw bytes is declaration order, not alphabetical.
	assert.Less(t, indexOf(string(got), `"zebra"`), indexOf(string(got), `"alpha"`))

	// Round-trips as valid JSON.
	var sink map[string]any
	require.NoError(t, json.Unmarshal(got, &sink))
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
