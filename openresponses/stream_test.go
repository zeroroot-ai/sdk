// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func streamOf(s string) *Stream { return newStream(io.NopCloser(strings.NewReader(s))) }

func TestStreamSkipsCommentsAndReadsDone(t *testing.T) {
	s := streamOf(": keep-alive\n\nevent: response.output_text.delta\ndata: {\"delta\":\"a\"}\n\ndata: [DONE]\n\n")
	ev, err := s.Recv()
	require.NoError(t, err)
	assert.Equal(t, EventOutputTextDelta, ev.Type, "the event line names the type when the data has none")
	_, err = s.Recv()
	require.ErrorIs(t, err, io.EOF)
	require.NoError(t, s.Close())
}

func TestStreamLastEventWithoutBlankLine(t *testing.T) {
	s := streamOf("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}")
	ev, err := s.Recv()
	require.NoError(t, err)
	assert.Equal(t, EventCompleted, ev.Type)
	assert.Equal(t, "completed", s.Response().Status)
	_, err = s.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestStreamErrorEvents(t *testing.T) {
	s := streamOf("data: {\"type\":\"error\",\"error\":{\"code\":\"c\",\"message\":\"bad\"}}\n\n")
	ev, err := s.Recv()
	require.NoError(t, err)
	require.NotNil(t, ev.Err)
	_, err = s.Recv()
	var ae *APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "bad", ae.Message)

	s = streamOf("data: {\"type\":\"error\"}\n\n")
	_, _ = s.Recv()
	_, err = s.Recv()
	require.ErrorContains(t, err, "stream error")

	s = streamOf("data: {\"type\":\"response.failed\"}\n\n")
	_, _ = s.Recv()
	_, err = s.Recv()
	require.ErrorContains(t, err, "response failed")
}

func TestStreamBadEvents(t *testing.T) {
	_, err := streamOf("data: {\n\n").Recv()
	require.ErrorContains(t, err, "decode event")
	_, err = streamOf("data: {\"item\":5}\n\n").Recv()
	require.Error(t, err)
	_, err = streamOf("data: {\"response\":5}\n\n").Recv()
	require.Error(t, err)
}

func TestStreamItemEvent(t *testing.T) {
	ev, err := streamOf("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"c\",\"name\":\"n\",\"arguments\":\"{}\"}}\n\n").Recv()
	require.NoError(t, err)
	assert.IsType(t, FunctionCall{}, ev.Item)
}

func TestCollectNeedsATerminalEvent(t *testing.T) {
	_, err := streamOf("data: {\"type\":\"response.created\"}\n\n").Collect(nil)
	require.ErrorContains(t, err, "no terminal event")
	_, err = streamOf("data: {\"type\":\"error\",\"error\":{\"message\":\"x\"}}\n\n").Collect(nil)
	require.Error(t, err)
}

func TestStreamAPIErrorOnOpen(t *testing.T) {
	srv, _ := newGateway(t, 429, "application/json", `{"error":{"code":"too_many_requests","message":"slow"}}`)
	_, err := testClient(srv).Stream(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	var ae *APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 429, ae.StatusCode)
	assert.Contains(t, ae.Error(), "slow")
}

func TestCreateBadReplyAndBadRequest(t *testing.T) {
	srv, _ := newGateway(t, 200, "application/json", `not json`)
	_, err := testClient(srv).Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.ErrorContains(t, err, "decode response")

	_, err = testClient(srv).Create(context.Background(), &Request{Input: []Item{Message{Content: []ContentPart{badPart{}}}}})
	require.ErrorContains(t, err, "encode request")

	_, err = testClient(srv).Stream(context.Background(), &Request{})
	_ = err
	_, err = (&Client{}).Stream(context.Background(), &Request{})
	require.Error(t, err)

	// A bundle with an http URL is refused for the stream too.
	_, err = (&Client{BaseURL: "http://x", Model: "m", CAPEM: "x"}).Stream(context.Background(), &Request{})
	require.Error(t, err)
}

func TestStructuredEmptyAndFailed(t *testing.T) {
	srv, _ := newGateway(t, 200, "application/json", `{"status":"completed","output":[]}`)
	var out map[string]any
	_, err := testClient(srv).CreateStructured(context.Background(), &Request{Input: []Item{NewUserMessage("x")}}, "n", map[string]any{}, &out)
	require.ErrorContains(t, err, "no text")

	srv2, _ := newGateway(t, 500, "application/json", `{}`)
	_, err = testClient(srv2).CreateStructured(context.Background(), &Request{Input: []Item{NewUserMessage("x")}}, "n", map[string]any{}, &out)
	require.Error(t, err)
	require.NotErrorIs(t, err, io.EOF)
}

func TestFailedResponseWithoutError(t *testing.T) {
	srv, _ := newGateway(t, 200, "application/json", `{"status":"failed"}`)
	_, err := testClient(srv).Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.ErrorContains(t, err, "response failed")
}
