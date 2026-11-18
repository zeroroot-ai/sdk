// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	componentpb "github.com/zeroroot-ai/sdk/api/gen/gibson/component/v1"
	harnesspb "github.com/zeroroot-ai/sdk/api/gen/gibson/harness/v1"
	"github.com/zeroroot-ai/sdk/openresponses"
)

func TestGrantHeaders(t *testing.T) {
	ctx := context.Background()

	h, err := grantHeaders(staticGrant("task-grant"))(ctx, "/v1/responses")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"x-capability-grant": "task-grant"}, h)

	_, err = grantHeaders(staticGrant(""))(ctx, "/v1/responses")
	require.ErrorContains(t, err, "no task grant")
}

func TestNewModelClientNeedsModelAndURL(t *testing.T) {
	_, err := newModelClient("primary", &resolvedModel{gatewayURL: "http://g"}, nil, nil)
	require.Error(t, err)
	_, err = newModelClient("primary", &resolvedModel{model: "m"}, nil, nil)
	require.Error(t, err)
}

type fakeHarnessCallback struct {
	harnesspb.UnimplementedHarnessCallbackServiceServer
	gatewayURL string
	caPEM      string
	gotSlot    string
}

func (f *fakeHarnessCallback) ResolveModel(
	_ context.Context, req *harnesspb.ResolveModelRequest,
) (*harnesspb.ResolveModelResponse, error) {
	f.gotSlot = req.GetSlot()
	if req.GetSlot() == "missing" {
		return nil, status.Error(codes.NotFound, "no model for slot")
	}
	return &harnesspb.ResolveModelResponse{
		Model: "claude-exact-1", Provider: "prov", GatewayUrl: f.gatewayURL, GatewayCaPem: f.caPEM,
		RouteHeaders:  map[string]string{"x-gibson-tenant": "t1", "x-gibson-provider": "prov"},
		ContextWindow: 200000, MaxOutputTokens: 8192,
	}, nil
}

func TestCallbackHarnessModelEndToEnd(t *testing.T) {
	var gotHeader http.Header
	var gotBody []byte
	gw := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"completed","output":[{"type":"message","role":"assistant",`+
			`"content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)
	}))
	defer gw.Close()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	caPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: gw.Certificate().Raw}))
	fake := &fakeHarnessCallback{gatewayURL: gw.URL, caPEM: caPEM}
	harnesspb.RegisterHarnessCallbackServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	cc := &CallbackClient{
		conn: conn, client: harnesspb.NewHarnessCallbackServiceClient(conn), connected: true, token: "task-grant",
	}
	h := &CallbackHarness{client: cc, tokenTracker: NewCallbackTokenTracker(), logger: nil}

	model, err := h.Model(context.Background(), "primary")
	require.NoError(t, err)
	assert.Equal(t, "primary", fake.gotSlot)
	assert.Equal(t, "claude-exact-1", model.Model)
	assert.Equal(t, 200000, model.ContextWindow)
	assert.Equal(t, caPEM, model.CAPEM)

	resp, err := model.Create(context.Background(), &openresponses.Request{
		Input: []openresponses.Item{openresponses.NewUserMessage("hi")},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Text())
	assert.Equal(t, "task-grant", gotHeader.Get("x-capability-grant"))
	assert.Equal(t, "t1", gotHeader.Get("x-gibson-tenant"))
	assert.Equal(t, "prov", gotHeader.Get("x-gibson-provider"))
	assert.Contains(t, string(gotBody), `"model":"claude-exact-1"`)
	assert.NotContains(t, string(gotBody), "primary")
	assert.Equal(t, 5, h.tokenTracker.BySlot("primary").TotalTokens)

	_, err = h.Model(context.Background(), "missing")
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(errors.Unwrap(errors.Unwrap(err))))
}

type fakeComponent struct {
	componentpb.UnimplementedComponentServiceServer
}

func (fakeComponent) ResolveModel(
	_ context.Context, req *componentpb.ResolveModelRequest,
) (*componentpb.ResolveModelResponse, error) {
	if req.GetWorkId() != "work-001" {
		return nil, status.Error(codes.PermissionDenied, "wrong work item")
	}
	return &componentpb.ResolveModelResponse{
		Model: "m-exact", GatewayUrl: "http://gw", RouteHeaders: map[string]string{"x-gibson-tenant": "t"},
	}, nil
}

func TestPlatformHarnessModel(t *testing.T) {
	harness, cleanup := setupParityTestWith(t, func(s *grpc.Server) {
		componentpb.RegisterComponentServiceServer(s, fakeComponent{})
	})
	defer cleanup()

	model, err := harness.Model(context.Background(), "primary")
	require.NoError(t, err)
	assert.Equal(t, "m-exact", model.Model)
	assert.Equal(t, "http://gw", model.BaseURL)
	assert.Equal(t, "t", model.RouteHeaders["x-gibson-tenant"])

	// The test client has no grant, so a call to the gateway must fail before it is sent.
	_, err = model.Create(context.Background(), &openresponses.Request{})
	require.ErrorContains(t, err, "no task grant")
}

// selfSignedPEMForServe returns a CA certificate that signed no test gateway.
func selfSignedPEMForServe(t *testing.T) string {
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
