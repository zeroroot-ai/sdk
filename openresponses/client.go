// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package openresponses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/zeroroot-ai/sdk/internal/gwhttp"
)

// ResponsesPath is the route of a model call. It is also the method name that
// a task grant lists.
const ResponsesPath = "/v1/responses"

// HeaderFunc returns the headers to add to a call of the route. The agent
// uses it to send its task grant in the header x-capability-grant.
type HeaderFunc = gwhttp.HeaderFunc

// Client calls the model gateway. It holds no state that changes, so many
// goroutines can share it.
type Client struct {
	// BaseURL is the gateway URL, for example https://gateway.example.
	BaseURL string
	// Model is the exact model name. It fills Request.Model when that is empty.
	Model string
	// RouteHeaders are sent unchanged on each call. The gateway route matches
	// on them.
	RouteHeaders map[string]string
	// Headers adds the headers of each call, for example the task grant. It is
	// optional.
	Headers HeaderFunc
	// OnUsage is called with the token counts of each finished response. It is
	// optional. The agent uses it to keep a running total.
	OnUsage func(Usage)
	// Provider, ContextWindow and MaxOutputTokens describe the model. The
	// client does not read them. A value of 0 means unknown.
	Provider        string
	ContextWindow   int
	MaxOutputTokens int
	// CAPEM is the PEM bundle of the CA that signed the certificate of the
	// gateway. When it is set, the client trusts exactly this bundle for the
	// gateway, and BaseURL must use https. When it is empty, the client uses
	// the system roots. The client has no option to skip verification.
	CAPEM string
	// HTTPClient makes the call. The default is http.DefaultClient. Do not
	// set it together with CAPEM.
	HTTPClient *http.Client

	tlsOnce   sync.Once
	tlsClient *http.Client
	tlsErr    error
}

// APIError is an error reply of the gateway.
type APIError struct {
	StatusCode int
	Type       string
	Code       string
	Message    string
	Param      string
}

// Error describes the reply.
func (e *APIError) Error() string {
	return fmt.Sprintf("openresponses: gateway returned %d (%s): %s", e.StatusCode, e.Code, e.Message)
}

func (c *Client) newRequest(ctx context.Context, req *Request, stream bool) (*http.Request, error) {
	if c.BaseURL == "" {
		return nil, errors.New("openresponses: client has no base URL")
	}
	if err := gwhttp.CheckURL(c.BaseURL, c.CAPEM); err != nil {
		return nil, fmt.Errorf("openresponses: %w", err)
	}
	r := *req
	if r.Model == "" {
		r.Model = c.Model
	}
	if r.Model == "" {
		return nil, errors.New("openresponses: request has no model")
	}
	r.Stream = stream
	body, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("openresponses: encode request: %w", err)
	}
	url := strings.TrimSuffix(c.BaseURL, "/") + ResponsesPath
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openresponses: build request: %w", err)
	}
	hr.Header.Set("Content-Type", "application/json")
	if stream {
		hr.Header.Set("Accept", "text/event-stream")
	} else {
		hr.Header.Set("Accept", "application/json")
	}
	for k, v := range c.RouteHeaders {
		hr.Header.Set(k, v)
	}
	if c.Headers != nil {
		extra, err := c.Headers(ctx, ResponsesPath)
		if err != nil {
			return nil, fmt.Errorf("openresponses: get call headers: %w", err)
		}
		for k, v := range extra {
			hr.Header.Set(k, v)
		}
	}
	return hr, nil
}

func (c *Client) do(hr *http.Request) (*http.Response, error) {
	hc, err := c.httpClient()
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(hr)
	if err != nil {
		return nil, fmt.Errorf("openresponses: call gateway: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, parseAPIError(resp)
	}
	return resp, nil
}

// httpClient returns the client for the call. With CAPEM it is a client that
// trusts only that bundle.
func (c *Client) httpClient() (*http.Client, error) {
	if c.CAPEM == "" {
		if c.HTTPClient != nil {
			return c.HTTPClient, nil
		}
		return http.DefaultClient, nil
	}
	if c.HTTPClient != nil {
		return nil, errors.New("openresponses: set CAPEM or HTTPClient, not both")
	}
	c.tlsOnce.Do(func() {
		tr, err := gwhttp.Transport(c.CAPEM)
		if err != nil {
			c.tlsErr = fmt.Errorf("openresponses: %w", err)
			return
		}
		c.tlsClient = &http.Client{Transport: tr}
	})
	return c.tlsClient, c.tlsErr
}

func parseAPIError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	e := &APIError{StatusCode: resp.StatusCode}
	var w struct {
		Error struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
			Param   string `json:"param"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &w) == nil && w.Error.Message != "" {
		e.Type, e.Code, e.Message, e.Param = w.Error.Type, w.Error.Code, w.Error.Message, w.Error.Param
		return e
	}
	e.Message = strings.TrimSpace(string(raw))
	return e
}

// Create makes one call and returns the whole response. A tool call is a
// Create with Request.Tools set. A response with the status "failed" returns
// an error.
func (c *Client) Create(ctx context.Context, req *Request) (*Response, error) {
	hr, err := c.newRequest(ctx, req, false)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(hr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("openresponses: decode response: %w", err)
	}
	c.reportUsage(&out)
	if out.Status == StatusFailed {
		return &out, responseFailure(&out)
	}
	return &out, nil
}

func (c *Client) reportUsage(r *Response) {
	if c.OnUsage != nil && r != nil && r.Usage != nil {
		c.OnUsage(*r.Usage)
	}
}

func responseFailure(r *Response) error {
	if r.Error != nil {
		return fmt.Errorf("openresponses: response failed (%s): %s", r.Error.Code, r.Error.Message)
	}
	return errors.New("openresponses: response failed")
}

// CreateStructured makes one call whose output must match a JSON schema. It
// sets text.format, decodes the text of the reply into out, and returns the
// response. The name labels the schema in the request.
func (c *Client) CreateStructured(
	ctx context.Context, req *Request, name string, schema any, out any,
) (*Response, error) {
	r := *req
	r.Text = &TextParam{Format: &TextFormat{Name: name, Schema: schema, Strict: true}}
	resp, err := c.Create(ctx, &r)
	if err != nil {
		return resp, err
	}
	text := resp.Text()
	if text == "" {
		return resp, errors.New("openresponses: structured reply has no text")
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return resp, fmt.Errorf("openresponses: decode structured reply: %w", err)
	}
	return resp, nil
}

// Stream makes one call with stream set and returns the events. The caller
// must call Close on the Stream.
func (c *Client) Stream(ctx context.Context, req *Request) (*Stream, error) {
	hr, err := c.newRequest(ctx, req, true)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(hr)
	if err != nil {
		return nil, err
	}
	s := newStream(resp.Body)
	s.onDone = c.reportUsage
	return s, nil
}
