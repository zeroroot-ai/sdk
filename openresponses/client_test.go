// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capture is a test gateway. It keeps the last request and sends a fixed reply.
type capture struct {
	body   []byte
	header http.Header
	path   string
}

func newGateway(t *testing.T, status int, contentType, reply string) (*httptest.Server, *capture) {
	t.Helper()
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.body, _ = io.ReadAll(r.Body)
		c.header = r.Header.Clone()
		c.path = r.URL.Path
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

// assertGolden compares a request body with a file in testdata. Set
// UPDATE_GOLDEN=1 to write the file.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		var v any
		require.NoError(t, json.Unmarshal(got, &v))
		pretty, err := json.MarshalIndent(v, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, append(pretty, '\n'), 0o600))
	}
	want, err := os.ReadFile(path) //nolint:gosec // reads a golden file under testdata
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got))
}

func testClient(srv *httptest.Server) *Client {
	return &Client{
		BaseURL:      srv.URL + "/",
		Model:        "claude-test-1",
		RouteHeaders: map[string]string{"x-gibson-tenant": "t1", "x-gibson-provider": "p1"},
		Headers: func(_ context.Context, route string) (map[string]string, error) {
			return map[string]string{"x-capability-grant": "grant-for-" + route}, nil
		},
	}
}

const plainReply = `{"id":"resp_1","object":"response","status":"completed","model":"claude-test-1",
"output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed",
"content":[{"type":"output_text","text":"hello","annotations":[]}]}],
"usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13,
"input_tokens_details":{"cached_tokens":4},"output_tokens_details":{"reasoning_tokens":1}}}`

func TestCreatePlain(t *testing.T) {
	srv, c := newGateway(t, 200, "application/json", plainReply)
	temp := 0.2
	maxTokens := 100
	resp, err := testClient(srv).Create(context.Background(), &Request{
		Instructions:    "Be brief.",
		Input:           []Item{NewUserMessage("say hello")},
		Temperature:     &temp,
		MaxOutputTokens: &maxTokens,
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Text())
	assert.Equal(t, 13, resp.Usage.TotalTokens)

	assert.Equal(t, "/v1/responses", c.path)
	assert.Equal(t, "grant-for-/v1/responses", c.header.Get("x-capability-grant"))
	assert.Equal(t, "t1", c.header.Get("x-gibson-tenant"))
	assert.Equal(t, "p1", c.header.Get("x-gibson-provider"))
	assertGolden(t, "request_plain.json", c.body)
}

func TestCreateToolCallRoundTrip(t *testing.T) {
	reply := `{"id":"resp_2","status":"completed","model":"claude-test-1","output":[
{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"I need the weather."}],"encrypted_content":"abc"},
{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"Oslo\"}","status":"completed"}]}`
	srv, c := newGateway(t, 200, "application/json", reply)
	cl := testClient(srv)

	// First turn: the model asks for a tool.
	req := &Request{
		Input: []Item{NewUserMessage("weather in Oslo?")},
		Tools: []Tool{{
			Name: "get_weather", Description: "Get the weather.",
			Parameters: map[string]any{
				"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}},
				"required": []string{"city"},
			},
		}},
		ToolChoice: "auto",
	}
	resp, err := cl.Create(context.Background(), req)
	require.NoError(t, err)
	calls := resp.FunctionCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "get_weather", calls[0].Name)
	assertGolden(t, "request_tools_first.json", c.body)

	// Second turn: the agent sends all items back, with the tool result.
	req.Input = append(req.Input, resp.Output...)
	req.Input = append(req.Input, FunctionCallOutput{CallID: calls[0].CallID, Output: `{"temp_c":3}`})
	_, err = cl.Create(context.Background(), req)
	require.NoError(t, err)
	assertGolden(t, "request_tools_second.json", c.body)
}

func TestCreateStructured(t *testing.T) {
	reply := `{"id":"resp_3","status":"completed","model":"claude-test-1","output":[
{"type":"message","id":"m","role":"assistant","status":"completed",
"content":[{"type":"output_text","text":"{\"name\":\"Ada\",\"age\":36}","annotations":[]}]}]}`
	srv, c := newGateway(t, 200, "application/json", reply)
	var out struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	schema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"name": map[string]any{"type": "string"}, "age": map[string]any{"type": "integer"}},
		"required":   []string{"name", "age"},
	}
	_, err := testClient(srv).CreateStructured(context.Background(),
		&Request{Input: []Item{NewUserMessage("who?")}}, "person", schema, &out)
	require.NoError(t, err)
	assert.Equal(t, "Ada", out.Name)
	assert.Equal(t, 36, out.Age)
	assertGolden(t, "request_structured.json", c.body)
}

func TestCreateStructuredBadJSON(t *testing.T) {
	reply := `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"nope"}]}]}`
	srv, _ := newGateway(t, 200, "application/json", reply)
	var out map[string]any
	_, err := testClient(srv).CreateStructured(context.Background(),
		&Request{Input: []Item{NewUserMessage("x")}}, "n", map[string]any{"type": "object"}, &out)
	require.Error(t, err)
}

const streamReply = `event: response.created
data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_4","status":"in_progress","output":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":2,"item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hel"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":3,"item_id":"msg_1","output_index":0,"content_index":0,"delta":"lo"}

event: response.completed
data: {"type":"response.completed","sequence_number":4,"response":{"id":"resp_4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello","annotations":[]}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}

`

func TestStream(t *testing.T) {
	srv, c := newGateway(t, 200, "text/event-stream", streamReply)
	s, err := testClient(srv).Stream(context.Background(), &Request{Input: []Item{NewUserMessage("hi")}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	var deltas []string
	resp, err := s.Collect(func(d string) { deltas = append(deltas, d) })
	require.NoError(t, err)
	assert.Equal(t, []string{"Hel", "lo"}, deltas)
	assert.Equal(t, "Hello", resp.Text())
	assert.Equal(t, 7, resp.Usage.TotalTokens)
	assert.Equal(t, "text/event-stream", c.header.Get("Accept"))
	assertGolden(t, "request_stream.json", c.body)
}

func TestStreamEvents(t *testing.T) {
	srv, _ := newGateway(t, 200, "text/event-stream", streamReply)
	s, err := testClient(srv).Stream(context.Background(), &Request{Input: []Item{NewUserMessage("hi")}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	var types []string
	for {
		ev, err := s.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		types = append(types, ev.Type)
	}
	assert.Equal(t, []string{
		"response.created", "response.output_item.added", EventOutputTextDelta, EventOutputTextDelta, EventCompleted,
	}, types)
}

func TestStreamFunctionCallArguments(t *testing.T) {
	reply := `event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","sequence_number":1,"item_id":"fc_1","output_index":0,"delta":"{\"a\":"}

event: response.function_call_arguments.done
data: {"type":"response.function_call_arguments.done","sequence_number":2,"item_id":"fc_1","output_index":0,"arguments":"{\"a\":1}"}

event: response.completed
data: {"type":"response.completed","sequence_number":3,"response":{"status":"completed","output":[{"type":"function_call","call_id":"c1","name":"f","arguments":"{\"a\":1}"}]}}

`
	srv, _ := newGateway(t, 200, "text/event-stream", reply)
	s, err := testClient(srv).Stream(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	resp, err := s.Collect(nil)
	require.NoError(t, err)
	require.Len(t, resp.FunctionCalls(), 1)
	assert.Equal(t, `{"a":1}`, resp.FunctionCalls()[0].Arguments)
}

func TestStreamFailed(t *testing.T) {
	reply := "event: response.failed\n" +
		`data: {"type":"response.failed","sequence_number":1,"response":{"status":"failed","error":{"code":"server_error","message":"boom"}}}` +
		"\n\n"
	srv, _ := newGateway(t, 200, "text/event-stream", reply)
	s, err := testClient(srv).Stream(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	_, err = s.Collect(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestAPIError(t *testing.T) {
	srv, _ := newGateway(t, 403, "application/json",
		`{"error":{"type":"invalid_request","code":"model_forbidden","message":"model not allowed","param":"model"}}`)
	_, err := testClient(srv).Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	var ae *APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.StatusCode)
	assert.Equal(t, "model_forbidden", ae.Code)
}

func TestAPIErrorPlainBody(t *testing.T) {
	srv, _ := newGateway(t, 502, "text/plain", "bad gateway")
	_, err := testClient(srv).Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	var ae *APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "bad gateway", ae.Message)
}

func TestFailedResponse(t *testing.T) {
	srv, _ := newGateway(t, 200, "application/json",
		`{"status":"failed","error":{"code":"model_error","message":"overloaded"}}`)
	_, err := testClient(srv).Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overloaded")
}

func TestRequestNeedsModelAndURL(t *testing.T) {
	_, err := (&Client{BaseURL: "http://x"}).Create(context.Background(), &Request{})
	require.Error(t, err)
	_, err = (&Client{Model: "m"}).Create(context.Background(), &Request{})
	require.Error(t, err)
}

func TestHeaderFuncError(t *testing.T) {
	srv, _ := newGateway(t, 200, "application/json", plainReply)
	cl := testClient(srv)
	cl.Headers = func(context.Context, string) (map[string]string, error) { return nil, errors.New("no grant") }
	_, err := cl.Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.ErrorContains(t, err, "no grant")
}

func TestItemRoundTrip(t *testing.T) {
	items := []string{
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"a"}]}`,
		`{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text","text":"b","annotations":[]},{"type":"refusal","refusal":"no"}]}`,
		`{"type":"function_call","id":"f","status":"completed","call_id":"c","name":"n","arguments":"{}"}`,
		`{"type":"function_call_output","call_id":"c","output":"ok"}`,
		`{"type":"reasoning","id":"r","status":"completed","summary":[{"type":"summary_text","text":"s"}],"content":[{"type":"output_text","text":"c"}],"encrypted_content":"e"}`,
		`{"type":"web_search_call","id":"w","status":"completed"}`,
	}
	for _, in := range items {
		it, err := UnmarshalItem([]byte(in))
		require.NoError(t, err, in)
		out, err := json.Marshal(it)
		require.NoError(t, err)
		assert.JSONEq(t, in, string(out))
	}
}

func TestMessageStringContent(t *testing.T) {
	it, err := UnmarshalItem([]byte(`{"type":"message","role":"user","content":"plain"}`))
	require.NoError(t, err)
	assert.Equal(t, "plain", it.(Message).Text())
}

func TestFunctionCallOutputNonString(t *testing.T) {
	it, err := UnmarshalItem([]byte(`{"type":"function_call_output","call_id":"c","output":{"a":1}}`))
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, it.(FunctionCallOutput).Output)
}

func TestToolChoiceFunction(t *testing.T) {
	b, err := json.Marshal(Request{Model: "m", ToolChoice: ToolChoiceFunction("f")})
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(b), `"tool_choice":{"name":"f","type":"function"}`), string(b))
}

func TestOnUsage(t *testing.T) {
	var got []Usage
	srv, _ := newGateway(t, 200, "application/json", plainReply)
	cl := testClient(srv)
	cl.OnUsage = func(u Usage) { got = append(got, u) }
	_, err := cl.Create(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.NoError(t, err)

	ssrv, _ := newGateway(t, 200, "text/event-stream", streamReply)
	cl2 := testClient(ssrv)
	cl2.OnUsage = func(u Usage) { got = append(got, u) }
	s, err := cl2.Stream(context.Background(), &Request{Input: []Item{NewUserMessage("x")}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	_, err = s.Collect(nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, 13, got[0].TotalTokens)
	assert.Equal(t, 7, got[1].TotalTokens)
}

func newTLSGateway(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plainReply)
	}))
	t.Cleanup(srv.Close)
	der := srv.Certificate().Raw
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return srv, string(pemBytes)
}

func TestCAPEMTrustsOnlyTheBundle(t *testing.T) {
	srv, ca := newTLSGateway(t)
	req := &Request{Input: []Item{NewUserMessage("x")}}

	// The bundle of the gateway: the call works.
	good := &Client{BaseURL: srv.URL, Model: "m", CAPEM: ca}
	_, err := good.Create(context.Background(), req)
	require.NoError(t, err)

	// No bundle: the system roots do not trust the test CA, so the call fails.
	bad := &Client{BaseURL: srv.URL, Model: "m"}
	_, err = bad.Create(context.Background(), req)
	require.Error(t, err)

	// Another bundle: the call fails.
	wrong := &Client{BaseURL: srv.URL, Model: "m", CAPEM: selfSignedPEM(t)}
	_, err = wrong.Create(context.Background(), req)
	require.Error(t, err)
}

func TestCAPEMNeedsHTTPS(t *testing.T) {
	_, ca := newTLSGateway(t)
	cl := &Client{BaseURL: "http://gateway.example", Model: "m", CAPEM: ca}
	_, err := cl.Create(context.Background(), &Request{})
	require.ErrorContains(t, err, "https")
}

func TestCAPEMBadBundleAndConflict(t *testing.T) {
	cl := &Client{BaseURL: "https://gateway.example", Model: "m", CAPEM: "not pem"}
	_, err := cl.Create(context.Background(), &Request{})
	require.ErrorContains(t, err, "no certificate")

	_, ca := newTLSGateway(t)
	cl = &Client{BaseURL: "https://gateway.example", Model: "m", CAPEM: ca, HTTPClient: http.DefaultClient}
	_, err = cl.Create(context.Background(), &Request{})
	require.ErrorContains(t, err, "not both")
}

// selfSignedPEM returns a CA certificate that did not sign the test gateway.
func selfSignedPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "other-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
