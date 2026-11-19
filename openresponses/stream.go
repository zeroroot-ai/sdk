// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Event type names of the stream that the SDK reads.
const (
	EventOutputTextDelta = "response.output_text.delta"
	EventCompleted       = "response.completed"
	EventFailed          = "response.failed"
	EventIncomplete      = "response.incomplete"
	EventError           = "error"
)

// Event is one server-sent event. The fields that an event type does not have
// stay empty.
type Event struct {
	Type string
	// Delta is the text of a delta event.
	Delta string
	// Response is set on the response.* events.
	Response *Response
	// Err is set on an error event.
	Err *APIError
}

// Stream reads the server-sent events of one call.
type Stream struct {
	body    io.Closer
	sc      *bufio.Scanner
	final   *Response
	termErr error
	onDone  func(*Response)
	done    bool
}

func newStream(body io.ReadCloser) *Stream {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	return &Stream{body: body, sc: sc}
}

// Close releases the connection.
func (s *Stream) Close() error {
	if err := s.body.Close(); err != nil {
		return fmt.Errorf("openresponses: close stream: %w", err)
	}
	return nil
}

// Recv returns the next event. It returns io.EOF after the last event. A
// terminal response.failed or error event is returned as an event, and the
// next call returns an error that says why the stream failed.
func (s *Stream) Recv() (*Event, error) {
	if s.termErr != nil {
		return nil, s.termErr
	}
	var name string
	var data bytes.Buffer
	for s.sc.Scan() {
		line := s.sc.Text()
		switch {
		case line == "":
			if data.Len() == 0 {
				name = ""
				continue
			}
			return s.decode(name, data.Bytes())
		case strings.HasPrefix(line, ":"):
			// A comment line. Ignore it.
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := s.sc.Err(); err != nil {
		return nil, fmt.Errorf("openresponses: read stream: %w", err)
	}
	if data.Len() > 0 {
		return s.decode(name, data.Bytes())
	}
	return nil, io.EOF
}

func (s *Stream) decode(name string, data []byte) (*Event, error) {
	if string(data) == "[DONE]" {
		return nil, io.EOF
	}
	var w struct {
		Type     string          `json:"type"`
		Delta    string          `json:"delta"`
		Response json.RawMessage `json:"response"`
		Error    *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("openresponses: decode event: %w", err)
	}
	ev := &Event{Type: w.Type, Delta: w.Delta}
	if ev.Type == "" {
		ev.Type = name
	}
	if len(w.Response) > 0 {
		var r Response
		if err := json.Unmarshal(w.Response, &r); err != nil {
			return nil, fmt.Errorf("openresponses: decode response of event: %w", err)
		}
		ev.Response = &r
	}
	if w.Error != nil {
		ev.Err = &APIError{Code: w.Error.Code, Message: w.Error.Message}
	}
	defer func() {
		if s.final != nil && !s.done {
			s.done = true
			if s.onDone != nil {
				s.onDone(s.final)
			}
		}
	}()
	switch ev.Type {
	case EventCompleted, EventIncomplete:
		s.final = ev.Response
	case EventFailed:
		s.final = ev.Response
		if ev.Response != nil {
			s.termErr = responseFailure(ev.Response)
		} else {
			s.termErr = errors.New("openresponses: response failed")
		}
	case EventError:
		if ev.Err != nil {
			s.termErr = ev.Err
		} else {
			s.termErr = errors.New("openresponses: stream error")
		}
	}
	return ev, nil
}

// Collect reads the stream to its end and returns the final response. It
// calls on for each text delta when on is not nil.
func (s *Stream) Collect(on func(delta string)) (*Response, error) {
	for {
		ev, err := s.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return s.final, err
		}
		if on != nil && ev.Type == EventOutputTextDelta {
			on(ev.Delta)
		}
	}
	if s.final == nil {
		return nil, errors.New("openresponses: stream ended with no terminal event")
	}
	return s.final, nil
}
