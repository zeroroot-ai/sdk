// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package a2ainbound is the client of the inbound A2A endpoint of Gibson.
//
// An outside program calls a published Gibson agent with A2A (JSON-RPC
// binding). The call starts a mission from the definition that the tenant
// published for the agent, and the A2A Task shows the state and the findings of
// that mission.
//
// The caller is an enrolled identity (package capabilitygrant). It signs each
// request with a new Capability Grant JWT that is bound to the path of the
// request, and it sends the token in the header x-capability-grant. A standard
// A2A client cannot do this, so this package adds the signer to the A2A client.
//
//	cg, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
//	    PlatformURL: "https://api.example.com", AgentName: "my-caller",
//	})
//	// handle err, then cg.Discover(ctx) and cg.Register(ctx)
//
//	c, err := a2ainbound.NewClient(a2ainbound.Config{
//	    BaseURL: "https://api.example.com", Signer: cg,
//	})
//	// handle err
//	task, err := c.SendMessage(ctx, "recon", "scan the staging hosts")
//	// handle err
//	task, err = c.GetTask(ctx, "recon", task.ID)
//
// The program examples/a2a-inbound-client shows the full sequence.
package a2ainbound

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
	"github.com/zeroroot-ai/sdk/internal/gwhttp"
)

const (
	// pathPrefix is the path prefix of the inbound endpoint. The endpoint of an
	// agent is pathPrefix + <agent>.
	pathPrefix = "/a2a/inbound/"
	// cardSuffix follows the endpoint path of an agent to give the path of its
	// Agent Card.
	cardSuffix = "/.well-known/agent-card.json"
)

// TargetsKey is the key of the target list in the data part of a message. The
// list narrows the targets of the published definition. The daemon reads a
// target from this list only, and never from the text of a message.
const TargetsKey = "targets"

// agentPattern is a DNS label. The daemon refuses each other agent name.
var agentPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// PathSigner signs the Capability Grant JWT of one request. The token is bound
// to the path of the request. *capabilitygrant.Client is a PathSigner.
type PathSigner interface {
	SignPath(path string) (string, error)
}

// Config configures a Client.
type Config struct {
	// BaseURL is the public API origin of the platform, for example
	// "https://api.example.com". The scheme must be https, because each request
	// carries a credential.
	BaseURL string

	// Signer is the enrolled identity of the caller. Required.
	Signer PathSigner

	// CAPEM is an optional PEM bundle of the CA of the platform edge. The client
	// then trusts exactly that bundle. When it is empty, the client uses the
	// system roots. There is no option to skip verification.
	CAPEM string

	// Transport is an optional base transport, for a caller that needs its own
	// dial rules. The caller owns the TLS rules of this transport: the client
	// adds no CA and no minimum version to it. It cannot be set together with
	// CAPEM.
	Transport http.RoundTripper
}

// Client calls the inbound A2A endpoint. It is safe for concurrent use.
type Client struct {
	base string
	http *http.Client
}

// NewClient returns a Client. It does not contact the platform.
func NewClient(cfg Config) (*Client, error) {
	u, err := capabilitygrant.ParsePlatformURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("a2ainbound: %w", err)
	}
	if strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("a2ainbound: the base URL %q must be an origin with no path and no query", cfg.BaseURL)
	}
	if cfg.Signer == nil {
		return nil, errors.New("a2ainbound: Config.Signer is required")
	}
	next := cfg.Transport
	if cfg.CAPEM != "" {
		if next != nil {
			return nil, errors.New("a2ainbound: set Config.CAPEM or Config.Transport, not both")
		}
		tr, err := gwhttp.Transport(cfg.CAPEM)
		if err != nil {
			return nil, fmt.Errorf("a2ainbound: %w", err)
		}
		next = tr
	}

	signer := cfg.Signer
	hc := gwhttp.NewClientOn(next, nil, func(_ context.Context, route string) (map[string]string, error) {
		// One new token for each request: a token is single use, and it is
		// bound to the path of this request.
		token, err := signer.SignPath(route)
		if err != nil {
			return nil, fmt.Errorf("sign the grant of the request: %w", err)
		}
		return map[string]string{capabilitygrant.MetadataHeaderCapabilityGrant: token}, nil
	})
	// A redirect would make the client sign a token for a URL that the caller
	// did not name, so the client never follows one.
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	return &Client{base: u.Scheme + "://" + u.Host, http: hc}, nil
}

// endpoint returns the URL of the JSON-RPC endpoint of an agent.
func (c *Client) endpoint(agent string) (string, error) {
	if !agentPattern.MatchString(agent) {
		return "", fmt.Errorf("a2ainbound: the agent name %q is not a DNS label", agent)
	}
	return c.base + pathPrefix + agent, nil
}

// rpc runs one A2A call to the endpoint of an agent.
func (c *Client) rpc(ctx context.Context, agent string, call func(*a2aclient.Client) (*a2a.Task, error)) (*a2a.Task, error) {
	endpoint, err := c.endpoint(agent)
	if err != nil {
		return nil, err
	}
	client, err := a2aclient.NewFromEndpoints(ctx,
		[]*a2a.AgentInterface{a2a.NewAgentInterface(endpoint, a2a.TransportProtocolJSONRPC)},
		a2aclient.WithConfig(a2aclient.Config{PreferredTransports: []a2a.TransportProtocol{a2a.TransportProtocolJSONRPC}}),
		a2aclient.WithJSONRPCTransport(c.http),
	)
	if err != nil {
		return nil, fmt.Errorf("a2ainbound: create the a2a client: %w", err)
	}
	defer func() { _ = client.Destroy() }()
	return call(client)
}

// SendMessage sends a text to a published agent and returns the Task of the
// mission that the message started. The Task ID is the handle for GetTask and
// CancelTask.
//
// The targets are optional target IDs. They narrow the targets of the
// published definition, and they travel in a data part under TargetsKey. With
// no target, the mission runs on all targets of the definition.
//
// A refused message is a value and not an error: the Task has the state
// rejected, its status message holds the reason, and no mission starts. The
// platform answers this way for a target outside the definition, for an agent
// that is not published, and for a caller with no grant to the agent. The last
// two get the same answer, so a caller cannot tell which agents exist.
//
// An error means that the platform refused the caller before the agent saw the
// message. a2a.ErrUnauthenticated reports a token that the platform does not
// accept, for example the token of a revoked identity. a2a.ErrUnauthorized
// reports an identity that is not a caller identity: it does not have the
// ceiling mission:originate, or it holds another capability.
func (c *Client) SendMessage(ctx context.Context, agent, text string, targets ...string) (*a2a.Task, error) {
	parts := []*a2a.Part{a2a.NewTextPart(text)}
	if len(targets) > 0 {
		ids := make([]any, len(targets))
		for i, id := range targets {
			ids[i] = id
		}
		parts = append(parts, a2a.NewDataPart(map[string]any{TargetsKey: ids}))
	}
	msg := a2a.NewMessage(a2a.MessageRoleUser, parts...)
	return c.rpc(ctx, agent, func(client *a2aclient.Client) (*a2a.Task, error) {
		res, err := client.SendMessage(ctx, &a2a.SendMessageRequest{Message: msg})
		if err != nil {
			return nil, fmt.Errorf("a2ainbound: send a message to %s: %w", agent, err)
		}
		task, ok := res.(*a2a.Task)
		if !ok {
			return nil, fmt.Errorf("a2ainbound: send a message to %s: the answer is a %T and not a task", agent, res)
		}
		return task, nil
	})
}

// GetTask returns the Task of a mission that this caller started. The Task
// state is the mission state. A finished Task holds the findings of the
// mission in its artifacts.
func (c *Client) GetTask(ctx context.Context, agent string, id a2a.TaskID) (*a2a.Task, error) {
	return c.rpc(ctx, agent, func(client *a2aclient.Client) (*a2a.Task, error) {
		task, err := client.GetTask(ctx, &a2a.GetTaskRequest{ID: id})
		if err != nil {
			return nil, fmt.Errorf("a2ainbound: get the task %s: %w", id, err)
		}
		return task, nil
	})
}

// CancelTask ends a mission that this caller started, and it returns the Task.
// The caller needs its grant to the agent at the time of the call. Without the
// grant, the call gives an error.
func (c *Client) CancelTask(ctx context.Context, agent string, id a2a.TaskID) (*a2a.Task, error) {
	return c.rpc(ctx, agent, func(client *a2aclient.Client) (*a2a.Task, error) {
		task, err := client.CancelTask(ctx, &a2a.CancelTaskRequest{ID: id})
		if err != nil {
			return nil, fmt.Errorf("a2ainbound: cancel the task %s: %w", id, err)
		}
		return task, nil
	})
}

// AgentCard returns the Agent Card of a published agent. The platform serves a
// card only to a caller with a grant to the agent. A caller with no grant gets
// the error of an agent that is not published, so it cannot tell which agents
// exist. This error is not a2a.ErrUnauthenticated and not a2a.ErrUnauthorized.
// Those two report a refused token and an identity that is not a caller
// identity, as for SendMessage.
func (c *Client) AgentCard(ctx context.Context, agent string) (*a2a.AgentCard, error) {
	endpoint, err := c.endpoint(agent)
	if err != nil {
		return nil, err
	}
	card, err := agentcard.NewResolver(c.http).Resolve(ctx, endpoint+cardSuffix)
	if err != nil {
		var status *agentcard.ErrStatusNotOK
		if errors.As(err, &status) {
			switch status.StatusCode {
			case http.StatusUnauthorized:
				err = fmt.Errorf("%w: %w", a2a.ErrUnauthenticated, err)
			case http.StatusForbidden:
				err = fmt.Errorf("%w: %w", a2a.ErrUnauthorized, err)
			}
		}
		return nil, fmt.Errorf("a2ainbound: get the agent card of %s: %w", agent, err)
	}
	return card, nil
}
