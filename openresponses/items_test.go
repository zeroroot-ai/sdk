// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageHelpers(t *testing.T) {
	assert.Equal(t, "s", NewSystemMessage("s").Text())
	assert.Equal(t, RoleSystem, NewSystemMessage("s").Role)
	m := Message{Content: []ContentPart{InputText{Text: "x"}, Refusal{Refusal: "no"}, OutputText{Text: "y"}}}
	assert.Equal(t, "xy", m.Text(), "only text parts count")
}

func TestMarshalPartVariants(t *testing.T) {
	m := Message{Role: RoleUser, Content: []ContentPart{
		&InputText{Text: "a"}, &OutputText{Text: "b"}, Refusal{Refusal: "no"},
		RawPart{Type: "input_image", JSON: json.RawMessage(`{"type":"input_image","image_url":"u"}`)},
	}}
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"refusal":"no"`)
	assert.Contains(t, string(raw), `"image_url":"u"`)
	assert.Contains(t, string(raw), `"annotations":[]`)

	_, err = json.Marshal(Message{Content: []ContentPart{badPart{}}})
	require.ErrorContains(t, err, "unknown content part")
	_, err = json.Marshal(Message{Content: []ContentPart{RawPart{Type: "x"}}})
	require.ErrorContains(t, err, "no JSON")
	_, err = json.Marshal(RawItem{Type: "x"})
	require.ErrorContains(t, err, "no JSON")
}

type badPart struct{}

func (badPart) isPart() {}

func TestUnmarshalItemErrors(t *testing.T) {
	for name, in := range map[string]string{
		"not json":      `{`,
		"bad call":      `{"type":"function_call","arguments":1}`,
		"bad output":    `{"type":"function_call_output","call_id":1}`,
		"bad message":   `{"type":"message","role":1}`,
		"bad content":   `{"type":"message","role":"user","content":5}`,
		"bad part":      `{"type":"message","role":"user","content":[5]}`,
		"bad reasoning": `{"type":"reasoning","summary":5}`,
	} {
		_, err := UnmarshalItem([]byte(in))
		require.Error(t, err, name)
	}
	it, err := UnmarshalItem([]byte(`{"type":"message","role":"assistant"}`))
	require.NoError(t, err)
	assert.Empty(t, it.(Message).Content)
	it, err = UnmarshalItem([]byte(`{"type":"message","role":"assistant","content":"hi"}`))
	require.NoError(t, err)
	assert.IsType(t, OutputText{}, it.(Message).Content[0])
	it, err = UnmarshalItem([]byte(`{"type":"message","role":"user","content":[{"type":"input_image"}]}`))
	require.NoError(t, err)
	assert.IsType(t, RawPart{}, it.(Message).Content[0])
}

func TestResponseUnmarshalError(t *testing.T) {
	var r Response
	require.Error(t, json.Unmarshal([]byte(`{"output":[5]}`), &r))
	require.Error(t, json.Unmarshal([]byte(`{"output":"x"}`), &r))
}

func TestRequestMarshalNilInput(t *testing.T) {
	raw, err := json.Marshal(Request{Model: "m"})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"input":[]`)
	_, err = json.Marshal(Request{Model: "m", Input: []Item{Message{Content: []ContentPart{badPart{}}}}})
	require.Error(t, err)
}
