// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"encoding/json"
	"fmt"
)

// Item type names of the specification.
const (
	TypeMessage            = "message"
	TypeFunctionCall       = "function_call"
	TypeFunctionCallOutput = "function_call_output"
	TypeReasoning          = "reasoning"
)

// Content part type names of the specification.
const (
	PartInputText  = "input_text"
	PartOutputText = "output_text"
	PartRefusal    = "refusal"
)

// Role names of a message.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Status names of an item and of a response.
const (
	StatusIncomplete = "incomplete"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Item is one element of the conversation. The concrete types are Message,
// FunctionCall, FunctionCallOutput, Reasoning and RawItem. The set is closed:
// RawItem holds any other item type.
type Item interface {
	isItem()
}

// ContentPart is one part of a message. The concrete types are InputText,
// OutputText, Refusal and RawPart. The set is closed: RawPart holds any other
// part type.
type ContentPart interface {
	isPart()
}

// InputText is a text part of an input message.
type InputText struct {
	Text string
}

// OutputText is a text part of a message of the model.
type OutputText struct {
	Text string
	// Annotations holds the annotations of the model, unchanged.
	Annotations []json.RawMessage
}

// Refusal is a part in which the model refuses to answer.
type Refusal struct {
	Refusal string
}

// RawPart holds a part of a type that this package has no struct for, for
// example an image. It keeps the JSON as it came, so a round trip loses
// nothing.
type RawPart struct {
	Type string
	JSON json.RawMessage
}

func (InputText) isPart() {}

func (OutputText) isPart() {}

func (Refusal) isPart() {}

func (RawPart) isPart() {}

// Message is a message of a user, the system, or the model.
type Message struct {
	// ID is set on a message of the model. It is empty on an input message.
	ID     string
	Role   string
	Status string
	// Content holds the parts of the message.
	Content []ContentPart
}

// FunctionCall is a request of the model to run a function.
type FunctionCall struct {
	ID     string
	Status string
	// CallID links the call to its FunctionCallOutput.
	CallID string
	Name   string
	// Arguments is a JSON string.
	Arguments string
}

// FunctionCallOutput is the result of a function call. The agent sends it
// back to the model.
type FunctionCallOutput struct {
	ID     string
	CallID string
	// Output is a string. A structured result is JSON text.
	Output string
}

// Reasoning is a reasoning record of the model.
type Reasoning struct {
	ID     string
	Status string
	// Summary holds the text of each summary_text part.
	Summary []string
	// Content holds the text of each reasoning text part.
	Content []string
	// EncryptedContent is opaque. The agent sends it back unchanged.
	EncryptedContent string
}

// RawItem holds an item of a type that this package has no struct for. It
// keeps the JSON as it came.
type RawItem struct {
	Type string
	JSON json.RawMessage
}

// The marker methods close the sets of items and parts.
func (Message) isItem() {}

func (FunctionCall) isItem() {}

func (FunctionCallOutput) isItem() {}

func (Reasoning) isItem() {}

func (RawItem) isItem() {}

// NewUserMessage returns a user message with one text part.
func NewUserMessage(text string) Message {
	return Message{Role: RoleUser, Content: []ContentPart{InputText{Text: text}}}
}

// NewSystemMessage returns a system message with one text part.
func NewSystemMessage(text string) Message {
	return Message{Role: RoleSystem, Content: []ContentPart{InputText{Text: text}}}
}

// Text returns the text of each text part of the message, joined in order.
func (m Message) Text() string {
	var out string
	for _, p := range m.Content {
		switch v := p.(type) {
		case InputText:
			out += v.Text
		case OutputText:
			out += v.Text
		}
	}
	return out
}

// MarshalJSON writes the message with its type field.
func (m Message) MarshalJSON() ([]byte, error) {
	parts := make([]json.RawMessage, 0, len(m.Content))
	for _, p := range m.Content {
		raw, err := marshalPart(p)
		if err != nil {
			return nil, err
		}
		parts = append(parts, raw)
	}
	return json.Marshal(struct {
		Type    string            `json:"type"`
		ID      string            `json:"id,omitempty"`
		Role    string            `json:"role"`
		Status  string            `json:"status,omitempty"`
		Content []json.RawMessage `json:"content"`
	}{TypeMessage, m.ID, m.Role, m.Status, parts})
}

// MarshalJSON writes the call with its type field.
func (f FunctionCall) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type      string `json:"type"`
		ID        string `json:"id,omitempty"`
		Status    string `json:"status,omitempty"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{TypeFunctionCall, f.ID, f.Status, f.CallID, f.Name, f.Arguments})
}

// MarshalJSON writes the output with its type field.
func (o FunctionCallOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string `json:"type"`
		ID     string `json:"id,omitempty"`
		CallID string `json:"call_id"`
		Output string `json:"output"`
	}{TypeFunctionCallOutput, o.ID, o.CallID, o.Output})
}

type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MarshalJSON writes the record with its type field.
func (r Reasoning) MarshalJSON() ([]byte, error) {
	summary := make([]textPart, 0, len(r.Summary))
	for _, s := range r.Summary {
		summary = append(summary, textPart{"summary_text", s})
	}
	content := make([]textPart, 0, len(r.Content))
	for _, c := range r.Content {
		content = append(content, textPart{PartOutputText, c})
	}
	out := struct {
		Type             string     `json:"type"`
		ID               string     `json:"id,omitempty"`
		Status           string     `json:"status,omitempty"`
		Summary          []textPart `json:"summary"`
		Content          []textPart `json:"content,omitempty"`
		EncryptedContent string     `json:"encrypted_content,omitempty"`
	}{TypeReasoning, r.ID, r.Status, summary, content, r.EncryptedContent}
	return json.Marshal(out)
}

// MarshalJSON writes the JSON that the item held.
func (r RawItem) MarshalJSON() ([]byte, error) { return rawOrError(r.JSON, "item") }

func rawOrError(raw json.RawMessage, what string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("openresponses: raw %s has no JSON", what)
	}
	return raw, nil
}

func marshalPart(p ContentPart) (json.RawMessage, error) {
	switch v := p.(type) {
	case InputText:
		return json.Marshal(textPart{PartInputText, v.Text})
	case *InputText:
		return json.Marshal(textPart{PartInputText, v.Text})
	case OutputText:
		return marshalOutputText(v)
	case *OutputText:
		return marshalOutputText(*v)
	case Refusal:
		return json.Marshal(struct {
			Type    string `json:"type"`
			Refusal string `json:"refusal"`
		}{PartRefusal, v.Refusal})
	case RawPart:
		return rawOrError(v.JSON, "part")
	default:
		return nil, fmt.Errorf("openresponses: unknown content part %T", p)
	}
}

func marshalOutputText(v OutputText) (json.RawMessage, error) {
	ann := v.Annotations
	if ann == nil {
		ann = []json.RawMessage{}
	}
	return json.Marshal(struct {
		Type        string            `json:"type"`
		Text        string            `json:"text"`
		Annotations []json.RawMessage `json:"annotations"`
	}{PartOutputText, v.Text, ann})
}

// UnmarshalItem decodes one item. An item of an unknown type becomes a
// RawItem.
func UnmarshalItem(data []byte) (Item, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("openresponses: decode item: %w", err)
	}
	switch head.Type {
	case TypeMessage:
		return unmarshalMessage(data)
	case TypeFunctionCall:
		var w struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}
		if err := json.Unmarshal(data, &w); err != nil {
			return nil, fmt.Errorf("openresponses: decode function_call: %w", err)
		}
		return FunctionCall{ID: w.ID, Status: w.Status, CallID: w.CallID, Name: w.Name, Arguments: w.Arguments}, nil
	case TypeFunctionCallOutput:
		var w struct {
			ID     string          `json:"id"`
			CallID string          `json:"call_id"`
			Output json.RawMessage `json:"output"`
		}
		if err := json.Unmarshal(data, &w); err != nil {
			return nil, fmt.Errorf("openresponses: decode function_call_output: %w", err)
		}
		return FunctionCallOutput{ID: w.ID, CallID: w.CallID, Output: stringOrJSON(w.Output)}, nil
	case TypeReasoning:
		return unmarshalReasoning(data)
	default:
		return RawItem{Type: head.Type, JSON: append(json.RawMessage(nil), data...)}, nil
	}
}

// stringOrJSON returns the value of a JSON string, or the JSON text of any
// other value.
func stringOrJSON(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func unmarshalMessage(data []byte) (Item, error) {
	var w struct {
		ID      string          `json:"id"`
		Role    string          `json:"role"`
		Status  string          `json:"status"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("openresponses: decode message: %w", err)
	}
	m := Message{ID: w.ID, Role: w.Role, Status: w.Status}
	if len(w.Content) == 0 || string(w.Content) == "null" {
		return m, nil
	}
	// The specification allows a plain string as the content of an input
	// message.
	var s string
	if json.Unmarshal(w.Content, &s) == nil {
		part := ContentPart(InputText{Text: s})
		if w.Role == RoleAssistant {
			part = OutputText{Text: s}
		}
		m.Content = []ContentPart{part}
		return m, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(w.Content, &raws); err != nil {
		return nil, fmt.Errorf("openresponses: decode message content: %w", err)
	}
	for _, raw := range raws {
		p, err := unmarshalPart(raw)
		if err != nil {
			return nil, err
		}
		m.Content = append(m.Content, p)
	}
	return m, nil
}

func unmarshalPart(raw json.RawMessage) (ContentPart, error) {
	var w struct {
		Type        string            `json:"type"`
		Text        string            `json:"text"`
		Refusal     string            `json:"refusal"`
		Annotations []json.RawMessage `json:"annotations"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("openresponses: decode content part: %w", err)
	}
	switch w.Type {
	case PartInputText:
		return InputText{Text: w.Text}, nil
	case PartOutputText:
		return OutputText{Text: w.Text, Annotations: w.Annotations}, nil
	case PartRefusal:
		return Refusal{Refusal: w.Refusal}, nil
	default:
		return RawPart{Type: w.Type, JSON: append(json.RawMessage(nil), raw...)}, nil
	}
}

func unmarshalReasoning(data []byte) (Item, error) {
	var w struct {
		ID               string     `json:"id"`
		Status           string     `json:"status"`
		Summary          []textPart `json:"summary"`
		Content          []textPart `json:"content"`
		EncryptedContent *string    `json:"encrypted_content"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("openresponses: decode reasoning: %w", err)
	}
	r := Reasoning{ID: w.ID, Status: w.Status}
	for _, s := range w.Summary {
		r.Summary = append(r.Summary, s.Text)
	}
	for _, c := range w.Content {
		r.Content = append(r.Content, c.Text)
	}
	if w.EncryptedContent != nil {
		r.EncryptedContent = *w.EncryptedContent
	}
	return r, nil
}
