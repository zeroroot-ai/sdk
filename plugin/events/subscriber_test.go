// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package events

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/plugin/manifest"
)

// ---- fakes ----

// fakeStream is a channel-backed EventStream for testing.
type fakeStream struct {
	ch chan Event
}

func newFakeStream() *fakeStream { return &fakeStream{ch: make(chan Event, 32)} }

func (f *fakeStream) send(ev Event) { f.ch <- ev }

func (f *fakeStream) Recv(ctx context.Context) (Event, error) {
	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case ev := <-f.ch:
		return ev, nil
	}
}

// fakeSecretsHook records calls to Invalidate and MarkRevoked.
type fakeSecretsHook struct {
	invalidated []string
	revoked     []string
}

func (f *fakeSecretsHook) Invalidate(name string)  { f.invalidated = append(f.invalidated, name) }
func (f *fakeSecretsHook) MarkRevoked(name string) { f.revoked = append(f.revoked, name) }

// fakeLifecycleHook records calls to MarkDegraded.
type fakeLifecycleHook struct {
	reasons []string
	failErr error // if non-nil, returned from MarkDegraded
}

func (f *fakeLifecycleHook) MarkDegraded(reason string) error {
	f.reasons = append(f.reasons, reason)
	return f.failErr
}

// fakeDrainer records DrainThenExit calls without actually exiting.
type fakeDrainer struct {
	reasons []string
}

func (f *fakeDrainer) DrainThenExit(reason string) {
	f.reasons = append(f.reasons, reason)
}

// testManifest builds a minimal valid manifest with the given secrets.
func testManifest(secrets ...manifest.SecretDecl) *manifest.Manifest {
	if len(secrets) == 0 {
		secrets = []manifest.SecretDecl{
			{Name: "cred:api_key", Scope: "startup", Rotation: "live", Required: true},
		}
	}
	return &manifest.Manifest{
		APIVersion: manifest.APIVersionV1,
		Kind:       manifest.KindPlugin,
		Metadata: manifest.ManifestMetadata{
			Name:    "test-plugin",
			Version: "0.1.0",
		},
		Spec: manifest.ManifestSpec{
			WorkloadClass: manifest.WorkloadClassPlugin,
			Secrets:       secrets,
			Methods: []manifest.MethodDecl{{
				Name: "Do",
			}},
			Runtime: "process",
		},
	}
}

// runSubscriberUntilDrained sends events to the stream, then cancels the
// subscriber and waits for it to return.
func runSubscriberUntilDrained(t *testing.T, s *Subscriber, stream *fakeStream, events []Event) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	for _, ev := range events {
		stream.send(ev)
	}
	// Give the subscriber time to process all queued events before cancelling.
	time.Sleep(20 * time.Millisecond)
	cancel()

	err := <-done
	require.NoError(t, err, "Run should return nil on context cancellation")
}

// ---- tests ----

func TestSubscriber_SecretRotated_Restart_CallsDrainer(t *testing.T) {
	stream := newFakeStream()
	sh := &fakeSecretsHook{}
	lh := &fakeLifecycleHook{}
	drainer := &fakeDrainer{}
	m := testManifest(manifest.SecretDecl{
		Name: "cred:db_pass", Scope: "startup", Rotation: "restart", Required: true,
	})
	s := NewWithDrainer(stream, sh, lh, drainer, m)

	ev := Event{
		Type:       EventTypeSecretRotated,
		Name:       "cred:db_pass",
		Version:    3,
		OccurredAt: time.Now(),
	}
	runSubscriberUntilDrained(t, s, stream, []Event{ev})

	require.Len(t, drainer.reasons, 1)
	assert.Contains(t, drainer.reasons[0], "cred:db_pass")
	assert.Contains(t, drainer.reasons[0], "secret_rotated_restart")
}

// errAfterStream returns a configurable error on the first Recv call.
type errAfterStream struct {
	err error
}

func (e *errAfterStream) Recv(_ context.Context) (Event, error) {
	return Event{}, e.err
}
