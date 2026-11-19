// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"encoding/json"
	"fmt"
)

// Tool is a function that the model may call.
type Tool struct {
	// Name is the name of the function.
	Name        string
	Description string
	// Parameters is the JSON Schema of the arguments. A map, a struct or a
	// json.RawMessage all work.
	Parameters any
	Strict     bool
}

// MarshalJSON writes the tool with type "function".
func (t Tool) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type        string `json:"type"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
		Strict      bool   `json:"strict,omitempty"`
	}{"function", t.Name, t.Description, t.Parameters, t.Strict})
}

// ToolChoiceFunction forces the model to call the function with this name.
func ToolChoiceFunction(name string) any {
	return map[string]string{"type": "function", "name": name}
}

// TextFormat is the text.format field. It asks for output that matches a
// JSON schema.
type TextFormat struct {
	Name        string
	Description string
	// Schema is the JSON Schema of the output.
	Schema any
	Strict bool
}

// MarshalJSON writes the format with type "json_schema".
func (f TextFormat) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type        string `json:"type"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Schema      any    `json:"schema"`
		Strict      bool   `json:"strict,omitempty"`
	}{"json_schema", f.Name, f.Description, f.Schema, f.Strict})
}

// TextParam is the text field of a request.
type TextParam struct {
	Format *TextFormat `json:"format,omitempty"`
}

// Request is the body of POST /v1/responses.
type Request struct {
	// Model is the exact model name. The Client fills it when it is empty.
	Model        string
	Input        []Item
	Instructions string
	Tools        []Tool
	// ToolChoice is the string "auto", "required" or "none", or the value of
	// ToolChoiceFunction.
	ToolChoice      any
	Text            *TextParam
	Stream          bool
	MaxOutputTokens *int
	Temperature     *float64
	TopP            *float64
	// PreviousResponseID continues an earlier response on the server.
	PreviousResponseID string
}

// MarshalJSON writes the request. A zero field is left out.
func (r Request) MarshalJSON() ([]byte, error) {
	input := r.Input
	if input == nil {
		input = []Item{}
	}
	return json.Marshal(struct {
		Model              string     `json:"model"`
		Input              []Item     `json:"input"`
		Instructions       string     `json:"instructions,omitempty"`
		Tools              []Tool     `json:"tools,omitempty"`
		ToolChoice         any        `json:"tool_choice,omitempty"`
		Text               *TextParam `json:"text,omitempty"`
		Stream             bool       `json:"stream,omitempty"`
		MaxOutputTokens    *int       `json:"max_output_tokens,omitempty"`
		Temperature        *float64   `json:"temperature,omitempty"`
		TopP               *float64   `json:"top_p,omitempty"`
		PreviousResponseID string     `json:"previous_response_id,omitempty"`
	}{r.Model, input, r.Instructions, r.Tools, r.ToolChoice, r.Text, r.Stream,
		r.MaxOutputTokens, r.Temperature, r.TopP, r.PreviousResponseID})
}

// Usage holds the token counts of a response.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// IncompleteDetails says why a response is incomplete.
type IncompleteDetails struct {
	Reason string `json:"reason"`
}

// ResponseError is the error object of a failed response.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Response is the response object of POST /v1/responses.
type Response struct {
	Status            string
	Output            []Item
	Usage             *Usage
	IncompleteDetails *IncompleteDetails
	Error             *ResponseError
}

// UnmarshalJSON reads the response and decodes each output item.
func (r *Response) UnmarshalJSON(data []byte) error {
	var w struct {
		Status            string             `json:"status"`
		Output            []json.RawMessage  `json:"output"`
		Usage             *Usage             `json:"usage"`
		IncompleteDetails *IncompleteDetails `json:"incomplete_details"`
		Error             *ResponseError     `json:"error"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return fmt.Errorf("openresponses: decode response: %w", err)
	}
	out := Response{Status: w.Status, Usage: w.Usage,
		IncompleteDetails: w.IncompleteDetails, Error: w.Error}
	for _, raw := range w.Output {
		it, err := UnmarshalItem(raw)
		if err != nil {
			return err
		}
		out.Output = append(out.Output, it)
	}
	*r = out
	return nil
}

// Text returns the text of each message of the model, joined in order.
func (r *Response) Text() string {
	var out string
	for _, it := range r.Output {
		if m, ok := it.(Message); ok {
			out += m.Text()
		}
	}
	return out
}

// FunctionCalls returns the function calls of the response in order.
func (r *Response) FunctionCalls() []FunctionCall {
	out := make([]FunctionCall, 0, len(r.Output))
	for _, it := range r.Output {
		if c, ok := it.(FunctionCall); ok {
			out = append(out, c)
		}
	}
	return out
}
