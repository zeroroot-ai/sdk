// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Command a2a-inbound-client calls a published Gibson agent with A2A.
//
// In plain words: an outside program gives work to a Gibson agent. It enrolls
// once with a bootstrap token, reads the Agent Card, sends one message, and
// waits for the Task to end. When the wait is over its limit, it cancels the
// Task.
//
// It is the first-party consumer of the packages a2ainbound and of
// capabilitygrant.Client.SignPath: the unwired-code guard of the sdk counts
// the reads in this package.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zeroroot-ai/sdk/a2ainbound"
	"github.com/zeroroot-ai/sdk/capabilitygrant"
)

// Environment variables of the program.
const (
	// envPlatformURL is the public API origin of the platform, with https.
	envPlatformURL = "GIBSON_PLATFORM_URL"
	// envAgent is the name of the published agent.
	envAgent = "GIBSON_INBOUND_AGENT"
	// envTargets is an optional list of target IDs, with commas between them.
	// The list narrows the targets of the published definition.
	envTargets = "GIBSON_INBOUND_TARGETS"
	// envHostKey is an optional path of the host key file.
	envHostKey = "GIBSON_HOST_KEY_PATH"
)

// options holds what one run needs.
type options struct {
	platformURL string
	agent       string
	text        string
	targets     []string
	hostKeyPath string
	// wait is the longest time to wait for the Task. poll is the time between
	// two reads of the Task.
	wait, poll time.Duration
	// transport is the base transport of each HTTP call. Nil means the default
	// transport with the system roots.
	transport http.RoundTripper
}

func main() {
	opts, err := optionsFrom(os.Getenv, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if err := run(context.Background(), opts, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// optionsFrom reads the options from the environment and the arguments. The
// arguments are the text of the message.
func optionsFrom(getenv func(string) string, args []string) (options, error) {
	opts := options{
		platformURL: getenv(envPlatformURL),
		agent:       getenv(envAgent),
		text:        strings.Join(args, " "),
		hostKeyPath: getenv(envHostKey),
		wait:        10 * time.Minute,
		poll:        2 * time.Second,
	}
	for _, id := range strings.Split(getenv(envTargets), ",") {
		if id = strings.TrimSpace(id); id != "" {
			opts.targets = append(opts.targets, id)
		}
	}
	switch {
	case opts.platformURL == "":
		return options{}, errors.New(envPlatformURL + " is not set")
	case opts.agent == "":
		return options{}, errors.New(envAgent + " is not set")
	case opts.text == "":
		return options{}, errors.New("usage: a2a-inbound-client <text of the message>")
	}
	return opts, nil
}

// run enrolls the caller, sends the message and waits for the Task.
func run(ctx context.Context, opts options, out io.Writer) error {
	// The bootstrap token of the first run comes from GIBSON_BOOTSTRAP_TOKEN. A
	// later run signs with the host key on disk and needs no token.
	//nolint:contextcheck // NewClient takes no context. It makes keys and contacts no server.
	identity, err := capabilitygrant.NewClient(capabilitygrant.ClientConfig{
		PlatformURL: opts.platformURL,
		AgentName:   "a2a-inbound-client",
		HostKeyPath: opts.hostKeyPath,
	})
	if err != nil {
		return fmt.Errorf("make the identity: %w", err)
	}
	defer func() { _ = identity.Close() }()
	if opts.transport != nil {
		identity.SetHTTPClient(&http.Client{Transport: opts.transport})
	}
	if err := identity.Discover(ctx); err != nil {
		return fmt.Errorf("discover the platform: %w", err)
	}
	if err := identity.Register(ctx); err != nil {
		return fmt.Errorf("enroll the identity: %w", err)
	}

	client, err := a2ainbound.NewClient(a2ainbound.Config{
		BaseURL: opts.platformURL, Signer: identity, Transport: opts.transport,
	})
	if err != nil {
		return fmt.Errorf("make the A2A client: %w", err)
	}

	card, err := client.AgentCard(ctx, opts.agent)
	if err != nil {
		return fmt.Errorf("read the agent card: %w", err)
	}
	_, _ = fmt.Fprintf(out, "agent %s: %s\n", card.Name, card.Description)

	task, err := client.SendMessage(ctx, opts.agent, opts.text, opts.targets...)
	if err != nil {
		return fmt.Errorf("send the message: %w", err)
	}
	_, _ = fmt.Fprintf(out, "task %s: %s\n", task.ID, task.Status.State)

	deadline := time.Now().Add(opts.wait)
	// A Task that needs input waits for a person in the tenant. An outside
	// caller cannot answer it, so the wait goes on until a final state.
	for !task.Status.State.Terminal() {
		if !time.Now().Before(deadline) {
			// The mission runs too long for this caller, so the caller ends it.
			task, err = client.CancelTask(ctx, opts.agent, task.ID)
			if err != nil {
				return fmt.Errorf("cancel the task: %w", err)
			}
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for the task %s: %w", task.ID, ctx.Err())
		case <-time.After(opts.poll):
		}
		task, err = client.GetTask(ctx, opts.agent, task.ID)
		if err != nil {
			return fmt.Errorf("read the task: %w", err)
		}
	}
	_, _ = fmt.Fprintf(out, "task %s: %s, %d artifacts\n", task.ID, task.Status.State, len(task.Artifacts))
	return nil
}
