// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeroroot-ai/sdk/capabilitygrant"
)

// grantRenewBefore is how long before its expiry the SDK renews a task grant.
const grantRenewBefore = 2 * time.Minute

// taskGrant holds the task grant of one work item. The daemon gives it in
// PollWorkResponse.task_grant. The SDK sends it in x-capability-grant on each
// call to a gateway, and renews it with RenewCapabilityGrant before it ends.
type taskGrant struct {
	renew func(ctx context.Context, current string) (string, error)
	now   func() time.Time

	mu    sync.Mutex
	token string
}

func newTaskGrant(token string, renew func(context.Context, string) (string, error)) *taskGrant {
	return &taskGrant{token: token, renew: renew, now: time.Now}
}

// get returns the current grant. It renews the grant first when it is near its
// expiry. A work item with no grant gives errNoTaskGrant.
func (g *taskGrant) get(ctx context.Context) (string, error) {
	if g == nil {
		return "", errNoTaskGrant
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.token == "" {
		return "", errNoTaskGrant
	}
	claims, err := capabilitygrant.ClaimsOf(g.token)
	if err != nil {
		return "", fmt.Errorf("the task grant of the work item is not readable: %w", err)
	}
	if g.now().Add(grantRenewBefore).Before(claims.ExpiresAt) {
		return g.token, nil
	}
	if g.renew == nil {
		return g.token, nil
	}
	fresh, err := g.renew(ctx, g.token)
	if err == nil {
		g.token = fresh
		return g.token, nil
	}
	// A grant that has not ended yet still works. One that has ended does not.
	if g.now().Before(claims.ExpiresAt) {
		return g.token, nil
	}
	return "", fmt.Errorf("the task grant has ended and the renewal failed: %w", err)
}
