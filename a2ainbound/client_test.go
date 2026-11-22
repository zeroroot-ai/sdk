// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package a2ainbound

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
)

const (
	fakeAgentID = "agent-row-1"
	// maxLifetime is the longest life that ext-authz accepts for a component
	// token (DefaultComponentMaxLifetime of the daemon).
	maxLifetime = 5 * time.Minute
)

// fakeEdge is the platform as an outside caller sees it. It serves the
// enrollment endpoints, and it serves the inbound A2A endpoint behind a check
// of the token. The check is a copy of the checks of ext-authz for the inbound
// class (gibson internal/server/extauthz/cgjwt ComponentVerifier.verify).
type fakeEdge struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	key      ed25519.PublicKey // the agent key that the caller registered
	seenJTI  map[string]bool
	paths    []string // the path of each accepted request
	messages []*a2a.Message
	// refuse, when set, is the status that the edge answers before any check.
	refuse int
	// answerMessage makes SendMessage answer with a message and no task.
	answerMessage bool
	// reject makes SendMessage answer with a Task in the state rejected.
	reject bool
}

func newFakeEdge(t *testing.T) *fakeEdge {
	t.Helper()
	e := &fakeEdge{t: t, seenJTI: map[string]bool{}}
	rpc := a2asrv.NewJSONRPCHandler(&fakeRuns{edge: e})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"protocol_version": "1.0",
			"endpoints":        map[string]string{"register": e.srv.URL + "/register"},
		})
	})
	mux.HandleFunc("POST /register", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AgentKeyJWK struct {
				X string `json:"x"`
			} `json:"agent_key_jwk"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pub, err := base64.RawURLEncoding.DecodeString(req.AgentKeyJWK.X)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			http.Error(w, "bad agent key", http.StatusBadRequest)
			return
		}
		e.mu.Lock()
		e.key = pub
		e.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"agent_id": fakeAgentID, "component_scope": "component:agent/caller"})
	})
	mux.Handle("POST /a2a/inbound/{agent}", e.guard(rpc))
	mux.Handle("GET /a2a/inbound/{agent}/.well-known/agent-card.json", e.guard(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(&a2a.AgentCard{
				Name: r.PathValue("agent"),
				SupportedInterfaces: []*a2a.AgentInterface{
					a2a.NewAgentInterface(e.srv.URL+"/a2a/inbound/"+r.PathValue("agent"), a2a.TransportProtocolJSONRPC),
				},
			})
		})))
	e.srv = httptest.NewTLSServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

// guard refuses a request with no valid token, as ext-authz does.
func (e *fakeEdge) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.mu.Lock()
		refuse := e.refuse
		e.mu.Unlock()
		if refuse != 0 {
			http.Error(w, "refused", refuse)
			return
		}
		// ext-authz reads the :path of the request, which holds the query too.
		if err := e.verify(r.Header.Get("x-capability-grant"), r.URL.RequestURI()); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		e.mu.Lock()
		e.paths = append(e.paths, r.URL.Path)
		e.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// verify holds the checks of the daemon side, in the same order.
func (e *fakeEdge) verify(token, path string) error {
	if token == "" {
		return errors.New("no capability grant")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("not a compact JWT")
	}
	var header struct{ Typ, Alg, Kid string }
	if err := decodeSegment(parts[0], &header); err != nil {
		return err
	}
	if header.Typ != "agent+jwt" {
		return fmt.Errorf("unexpected typ %q", header.Typ)
	}
	if header.Kid != fakeAgentID {
		return fmt.Errorf("unknown kid %q", header.Kid)
	}
	if header.Alg != "EdDSA" {
		return fmt.Errorf("unexpected alg %q", header.Alg)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("decode the signature: %w", err)
	}
	e.mu.Lock()
	key := e.key
	e.mu.Unlock()
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return errors.New("signature invalid")
	}
	return e.verifyClaims(parts[1], path)
}

// verifyClaims checks the claims of a token with a good signature: the
// audience, the life, the binding to the path and the single use.
func (e *fakeEdge) verifyClaims(segment, path string) error {
	var claims struct {
		Aud    string `json:"aud"`
		Iat    *int64 `json:"iat"`
		Exp    *int64 `json:"exp"`
		JTI    string `json:"jti"`
		Method string `json:"method"`
	}
	if err := decodeSegment(segment, &claims); err != nil {
		return err
	}
	if claims.Aud != "zeroroot.ai/gibson-daemon" {
		return fmt.Errorf("audience %q", claims.Aud)
	}
	if claims.Exp == nil || claims.Iat == nil {
		return errors.New("no exp or no iat")
	}
	now, exp, iat := time.Now(), time.Unix(*claims.Exp, 0), time.Unix(*claims.Iat, 0)
	if !exp.After(now) {
		return errors.New("expired")
	}
	if iat.After(now.Add(time.Second)) {
		return errors.New("issued in the future")
	}
	if exp.Sub(iat) > maxLifetime || exp.Sub(now) > maxLifetime {
		return errors.New("lifetime too long")
	}
	if claims.Method == "" {
		return errors.New("no method claim")
	}
	if claims.Method != path {
		return fmt.Errorf("token bound to %q, presented at %q", claims.Method, path)
	}
	if claims.JTI == "" {
		return errors.New("no jti")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.seenJTI[claims.JTI] {
		return fmt.Errorf("jti %s already used", claims.JTI)
	}
	e.seenJTI[claims.JTI] = true
	return nil
}

func decodeSegment(seg string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return fmt.Errorf("decode a JWT segment: %w", err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("parse a JWT segment: %w", err)
	}
	return nil
}

func (e *fakeEdge) caPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: e.srv.Certificate().Raw}))
}

// enroll registers a caller identity at the fake edge.
func (e *fakeEdge) enroll(t *testing.T) *capabilitygrant.Client {
	t.Helper()
	t.Setenv("SPIFFE_ENDPOINT_SOCKET", "")
	cg, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
		PlatformURL: e.srv.URL, BootstrapToken: "boot-1", AgentName: "caller",
		HostKeyPath: filepath.Join(t.TempDir(), "host_key.json"),
	})
	require.NoError(t, err)
	cg.SetHTTPClient(e.srv.Client())
	require.NoError(t, cg.Discover(context.Background()))
	require.NoError(t, cg.Register(context.Background()))
	return cg
}

func (e *fakeEdge) client(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(Config{BaseURL: e.srv.URL, Signer: e.enroll(t), CAPEM: e.caPEM()})
	require.NoError(t, err)
	return c
}

// fakeRuns is the A2A handler behind the edge. The embedded interface is nil:
// the inbound endpoint serves three methods only.
type fakeRuns struct {
	a2asrv.RequestHandler
	edge *fakeEdge
}

func (f *fakeRuns) SendMessage(_ context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	f.edge.mu.Lock()
	defer f.edge.mu.Unlock()
	f.edge.messages = append(f.edge.messages, req.Message)
	if f.edge.answerMessage {
		return a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("hello")), nil
	}
	if f.edge.reject {
		// The answer of the daemon for a refused message (RejectedTask).
		t := &a2a.Task{ID: "refused-1", ContextID: "refused-1", Status: a2a.TaskStatus{State: a2a.TaskStateRejected}}
		t.Status.Message = a2a.NewMessageForTask(a2a.MessageRoleAgent, t,
			a2a.NewTextPart("the agent is not available for inbound calls"))
		return t, nil
	}
	return &a2a.Task{ID: "mission-1", ContextID: "ctx-1", Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted}}, nil
}

func (f *fakeRuns) GetTask(_ context.Context, req *a2a.GetTaskRequest) (*a2a.Task, error) {
	if req.ID != "mission-1" {
		return nil, a2a.ErrTaskNotFound
	}
	return &a2a.Task{ID: req.ID, ContextID: "ctx-1", Status: a2a.TaskStatus{State: a2a.TaskStateWorking}}, nil
}

func (f *fakeRuns) CancelTask(_ context.Context, req *a2a.CancelTaskRequest) (*a2a.Task, error) {
	return &a2a.Task{ID: req.ID, ContextID: "ctx-1", Status: a2a.TaskStatus{State: a2a.TaskStateCanceled}}, nil
}

func TestClientCallsEachMethodWithANewTokenForItsPath(t *testing.T) {
	e := newFakeEdge(t)
	c := e.client(t)
	ctx := context.Background()

	card, err := c.AgentCard(ctx, "recon")
	require.NoError(t, err)
	assert.Equal(t, "recon", card.Name)
	require.Len(t, card.SupportedInterfaces, 1)
	assert.Equal(t, e.srv.URL+"/a2a/inbound/recon", card.SupportedInterfaces[0].URL)

	task, err := c.SendMessage(ctx, "recon", "scan the hosts", "target-1", "target-2")
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskID("mission-1"), task.ID)
	assert.Equal(t, a2a.TaskStateSubmitted, task.Status.State)

	got, err := c.GetTask(ctx, "recon", task.ID)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateWorking, got.Status.State)

	// A second read proves that two calls of one path never share a token: the
	// edge refuses a jti that it saw before.
	_, err = c.GetTask(ctx, "recon", task.ID)
	require.NoError(t, err)

	canceled, err := c.CancelTask(ctx, "recon", task.ID)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCanceled, canceled.Status.State)

	e.mu.Lock()
	defer e.mu.Unlock()
	assert.Equal(t, []string{
		"/a2a/inbound/recon/.well-known/agent-card.json",
		"/a2a/inbound/recon", "/a2a/inbound/recon", "/a2a/inbound/recon", "/a2a/inbound/recon",
	}, e.paths)
	assert.Len(t, e.seenJTI, 5, "each request has its own jti")

	// The message holds the text, and the targets in a data part.
	require.Len(t, e.messages, 1)
	msg := e.messages[0]
	assert.Equal(t, a2a.MessageRoleUser, msg.Role)
	assert.NotEmpty(t, msg.ID)
	require.Len(t, msg.Parts, 2)
	assert.Equal(t, "scan the hosts", msg.Parts[0].Text())
	assert.Equal(t, map[string]any{TargetsKey: []any{"target-1", "target-2"}}, msg.Parts[1].Data())
}

func TestSendMessageWithNoTargetSendsNoDataPart(t *testing.T) {
	e := newFakeEdge(t)
	_, err := e.client(t).SendMessage(context.Background(), "recon", "scan")
	require.NoError(t, err)
	require.Len(t, e.messages, 1)
	require.Len(t, e.messages[0].Parts, 1)
	assert.Equal(t, "scan", e.messages[0].Parts[0].Text())
}

func TestSendMessageRefusesAnAnswerThatIsNoTask(t *testing.T) {
	e := newFakeEdge(t)
	e.answerMessage = true
	_, err := e.client(t).SendMessage(context.Background(), "recon", "scan")
	require.ErrorContains(t, err, "not a task")
}

// A refused message is a Task in the state rejected. It is a value and not an
// error, and it holds the reason.
func TestARejectedTaskIsAValue(t *testing.T) {
	e := newFakeEdge(t)
	e.reject = true
	task, err := e.client(t).SendMessage(context.Background(), "recon", "scan", "target-outside")
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateRejected, task.Status.State)
	require.NotNil(t, task.Status.Message)
	require.Len(t, task.Status.Message.Parts, 1)
	assert.Equal(t, "the agent is not available for inbound calls", task.Status.Message.Parts[0].Text())
}

func TestACanceledContextEndsTheCall(t *testing.T) {
	e := newFakeEdge(t)
	c := e.client(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.SendMessage(ctx, "recon", "scan")
	require.ErrorIs(t, err, context.Canceled)
	_, err = c.GetTask(ctx, "recon", "mission-1")
	require.ErrorIs(t, err, context.Canceled)
	_, err = c.CancelTask(ctx, "recon", "mission-1")
	require.ErrorIs(t, err, context.Canceled)
	_, err = c.AgentCard(ctx, "recon")
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, e.paths, "no request reached the agent")
}

// ext-authz compares the method claim with the full path of the request, and
// that path holds the query. So the client signs the path with the query.
func TestARequestWithAQueryVerifies(t *testing.T) {
	e := newFakeEdge(t)
	c := e.client(t)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		e.srv.URL+"/a2a/inbound/recon/.well-known/agent-card.json?probe=1", http.NoBody)
	require.NoError(t, err)
	resp, err := c.http.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "the edge accepts the token of a request with a query")
}

// The edge checks the binding of a token to its path. A token for one agent is
// refused at the endpoint of another agent, and a token is refused the second
// time.
func TestTheFakeEdgeRefusesAnotherPathAndAReplay(t *testing.T) {
	e := newFakeEdge(t)
	cg := e.enroll(t)
	token, err := cg.SignPath("/a2a/inbound/recon")
	require.NoError(t, err)

	require.ErrorContains(t, e.verify(token, "/a2a/inbound/other"), "presented at")
	require.ErrorContains(t, e.verify(token, "/a2a/inbound/recon?x=1"), "presented at")
	require.NoError(t, e.verify(token, "/a2a/inbound/recon"))
	require.ErrorContains(t, e.verify(token, "/a2a/inbound/recon"), "already used")
	require.ErrorContains(t, e.verify("", "/a2a/inbound/recon"), "no capability grant")

	require.ErrorContains(t, e.verify(tamperSignature(t, token), "/a2a/inbound/recon"), "signature")
}

// tamperSignature flips one bit of the decoded signature, so the result always
// differs from the original. Text edits of the encoded signature can decode to
// the same bytes: the last byte of an Ed25519 signature is below 0x10, so a
// suffix of "AA" leaves it unchanged in about 1 run in 16.
func tamperSignature(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	sig[0] ^= 0x01
	return parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestARefusedCallerGetsTheA2AError(t *testing.T) {
	ctx := context.Background()
	for status, want := range map[int]error{
		http.StatusUnauthorized: a2a.ErrUnauthenticated,
		http.StatusForbidden:    a2a.ErrUnauthorized,
	} {
		e := newFakeEdge(t)
		c := e.client(t)
		e.refuse = status

		_, err := c.AgentCard(ctx, "recon")
		require.ErrorIs(t, err, want, "AgentCard with HTTP %d", status)
		_, err = c.SendMessage(ctx, "recon", "scan")
		require.ErrorIs(t, err, want, "SendMessage with HTTP %d", status)
		_, err = c.GetTask(ctx, "recon", "mission-1")
		require.ErrorIs(t, err, want, "GetTask with HTTP %d", status)
		_, err = c.CancelTask(ctx, "recon", "mission-1")
		require.ErrorIs(t, err, want, "CancelTask with HTTP %d", status)
	}
}

func TestAnErrorOfTheAgentIsAnError(t *testing.T) {
	e := newFakeEdge(t)
	_, err := e.client(t).GetTask(context.Background(), "recon", "mission-of-another-caller")
	require.ErrorIs(t, err, a2a.ErrTaskNotFound)
}

func TestTheClientNeverFollowsARedirect(t *testing.T) {
	var elsewhere int
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere++ }))
	defer other.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/elsewhere", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL, Signer: staticSigner("token"), Transport: srv.Client().Transport})
	require.NoError(t, err)
	_, err = c.AgentCard(context.Background(), "recon")
	require.Error(t, err)
	_, err = c.SendMessage(context.Background(), "recon", "scan")
	require.Error(t, err)
	assert.Zero(t, elsewhere, "no request, and so no token, went to the other host")
}

type staticSigner string

func (s staticSigner) SignPath(string) (string, error) { return string(s), nil }

type failingSigner struct{}

func (failingSigner) SignPath(string) (string, error) { return "", errors.New("no key") }

func TestASignerErrorStopsTheCallAndHoldsNoToken(t *testing.T) {
	e := newFakeEdge(t)
	c, err := NewClient(Config{BaseURL: e.srv.URL, Signer: failingSigner{}, CAPEM: e.caPEM()})
	require.NoError(t, err)
	_, err = c.SendMessage(context.Background(), "recon", "scan")
	require.ErrorContains(t, err, "no key")
	assert.Empty(t, e.paths)
}

func TestTheClientRefusesABadAgentName(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://api.example.com", Signer: staticSigner("t")})
	require.NoError(t, err)
	ctx := context.Background()
	for _, agent := range []string{"", "Recon", "re/con", "../x", "-recon", "recon?x=1", strings.Repeat("a", 64)} {
		_, err := c.SendMessage(ctx, agent, "scan")
		require.ErrorContains(t, err, "not a DNS label", "agent %q", agent)
		_, err = c.AgentCard(ctx, agent)
		require.ErrorContains(t, err, "not a DNS label", "agent %q", agent)
	}
}

func TestNewClientRefusals(t *testing.T) {
	signer := staticSigner("t")
	for name, tt := range map[string]struct {
		cfg  Config
		want string
	}{
		"plain http":          {Config{BaseURL: "http://api.example.com", Signer: signer}, "https"},
		"no URL":              {Config{Signer: signer}, "https"},
		"a URL with a path":   {Config{BaseURL: "https://api.example.com/a2a/inbound", Signer: signer}, "no path"},
		"a URL with a query":  {Config{BaseURL: "https://api.example.com?x=1", Signer: signer}, "no path"},
		"no signer":           {Config{BaseURL: "https://api.example.com"}, "Signer is required"},
		"a bundle with no CA": {Config{BaseURL: "https://api.example.com", Signer: signer, CAPEM: "not pem"}, "no certificate"},
		"a bundle and a transport": {
			Config{BaseURL: "https://api.example.com", Signer: signer, CAPEM: "x", Transport: http.DefaultTransport}, "not both",
		},
	} {
		_, err := NewClient(tt.cfg)
		require.ErrorContains(t, err, tt.want, name)
	}
	// A trailing slash on the origin is accepted.
	_, err := NewClient(Config{BaseURL: "https://api.example.com/", Signer: signer})
	require.NoError(t, err)
}

func TestSendCarriesTheWholeMessage(t *testing.T) {
	e := newFakeEdge(t)
	msg := a2a.NewMessage(a2a.MessageRoleUser,
		a2a.NewTextPart("a fixed note"),
		a2a.NewDataPart(map[string]any{"webhook": map[string]any{"id": "evt-1", "body": map[string]any{"k": "v"}}}))
	msg.ID = "webhook:edr:evt-1"
	msg.Metadata = map[string]any{"webhook_id": "evt-1"}

	task, err := e.client(t).Send(context.Background(), "recon", msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskID("mission-1"), task.ID)

	e.mu.Lock()
	defer e.mu.Unlock()
	require.Len(t, e.messages, 1)
	got := e.messages[0]
	assert.Equal(t, "webhook:edr:evt-1", got.ID)
	assert.Equal(t, map[string]any{"webhook_id": "evt-1"}, got.Metadata)
	require.Len(t, got.Parts, 2)
	assert.Equal(t, "a fixed note", got.Parts[0].Text())
	wantData := map[string]any{"webhook": map[string]any{"id": "evt-1", "body": map[string]any{"k": "v"}}}
	assert.Equal(t, wantData, got.Parts[1].Data())
	// The call carries a grant that is bound to its path, as every call does.
	assert.Equal(t, []string{"/a2a/inbound/recon"}, e.paths)
}

func TestSendFillsAnEmptyIDAndRoleOnACopy(t *testing.T) {
	e := newFakeEdge(t)
	msg := &a2a.Message{Parts: a2a.ContentParts{a2a.NewTextPart("scan")}}
	_, err := e.client(t).Send(context.Background(), "recon", msg)
	require.NoError(t, err)

	assert.Empty(t, msg.ID, "Send must not change the message of the caller")
	assert.Empty(t, msg.Role)
	e.mu.Lock()
	defer e.mu.Unlock()
	require.Len(t, e.messages, 1)
	assert.NotEmpty(t, e.messages[0].ID)
	assert.Equal(t, a2a.MessageRoleUser, e.messages[0].Role)
}

func TestSendRefusesAMessageWithNoPart(t *testing.T) {
	e := newFakeEdge(t)
	c := e.client(t)
	_, err := c.Send(context.Background(), "recon", nil)
	require.Error(t, err)
	_, err = c.Send(context.Background(), "recon", &a2a.Message{})
	require.Error(t, err)
	e.mu.Lock()
	defer e.mu.Unlock()
	assert.Empty(t, e.messages)
	assert.Empty(t, e.paths, "a refused message must send no request")
}

func TestSendRefusesABadAgentNameAndASignerError(t *testing.T) {
	e := newFakeEdge(t)
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("scan"))
	_, err := e.client(t).Send(context.Background(), "Not A Label", msg)
	require.Error(t, err)

	c, err := NewClient(Config{BaseURL: e.srv.URL, Signer: failingSigner{}, CAPEM: e.caPEM()})
	require.NoError(t, err)
	_, err = c.Send(context.Background(), "recon", msg)
	require.ErrorContains(t, err, "no key")
}
