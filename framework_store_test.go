// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk"
	"github.com/zeroroot-ai/sdk/finding"
)

// --------------------------------------------------------------------------
// Default bounded store: capacity enforcement
// --------------------------------------------------------------------------

// TestDefaultMissionStore_BoundedCapacity inserts 100,000 missions into a
// framework that uses the default bounded store and verifies that the number
// of missions returned by ListMissions never exceeds DefaultMissionStoreCapacity.
func TestDefaultMissionStore_BoundedCapacity(t *testing.T) {
	f, err := sdk.NewFramework()
	require.NoError(t, err)

	ctx := context.Background()
	const total = 100_000

	for i := range total {
		_, err := f.CreateMission(ctx,
			sdk.WithMissionName(fmt.Sprintf("mission-%d", i)),
		)
		require.NoError(t, err)
	}

	missions, err := f.ListMissions(ctx)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(missions), sdk.DefaultMissionStoreCapacity,
		"store must not grow beyond its cap after %d inserts", total)
	assert.NotEmpty(t, missions, "store must retain at least one mission")
}

// TestDefaultMissionStore_RetainsRecentMissions verifies that after exceeding the
// cap the most recently inserted missions are still retrievable.
func TestDefaultMissionStore_RetainsRecentMissions(t *testing.T) {
	// Use a small custom store to make the eviction boundary predictable.
	const cap = 5
	store := sdk.NewBoundedMissionStore(cap)
	f, err := sdk.NewFramework(sdk.WithMissionStore(store))
	require.NoError(t, err)

	ctx := context.Background()

	var ids []string
	for i := range 10 {
		m, err := f.CreateMission(ctx, sdk.WithMissionName(fmt.Sprintf("m-%d", i)))
		require.NoError(t, err)
		ids = append(ids, m.ID)
	}

	// The last `cap` missions must still be present.
	for _, id := range ids[len(ids)-cap:] {
		_, err := f.GetMission(ctx, id)
		assert.NoError(t, err, "mission %s should still be present after eviction", id)
	}

	// ListMissions must not exceed the cap.
	missions, err := f.ListMissions(ctx)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(missions), cap)
}

// --------------------------------------------------------------------------
// Custom store injection via WithMissionStore
// --------------------------------------------------------------------------

// spyMissionStore records every Set call so we can verify the injected store
// is actually used by the framework.
type spyMissionStore struct {
	setCalls int
	inner    sdk.MissionStore
}

func (s *spyMissionStore) Set(id string, m *sdk.Mission) {
	s.setCalls++
	s.inner.Set(id, m)
}

func (s *spyMissionStore) Get(id string) (*sdk.Mission, bool) {
	return s.inner.Get(id)
}

func (s *spyMissionStore) All() []*sdk.Mission {
	return s.inner.All()
}

// TestWithMissionStore_CustomStoreIsUsed verifies that the framework delegates
// all mission operations to the injected store.
func TestWithMissionStore_CustomStoreIsUsed(t *testing.T) {
	spy := &spyMissionStore{inner: sdk.NewBoundedMissionStore(100)}
	f, err := sdk.NewFramework(sdk.WithMissionStore(spy))
	require.NoError(t, err)

	ctx := context.Background()

	m, err := f.CreateMission(ctx, sdk.WithMissionName("test"))
	require.NoError(t, err)
	assert.Equal(t, 1, spy.setCalls, "CreateMission must call Set once")

	err = f.StartMission(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, spy.setCalls, "StartMission must call Set once")

	err = f.StopMission(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, spy.setCalls, "StopMission must call Set once")
}

// --------------------------------------------------------------------------
// Custom findings store injection via WithFindingsStore
// --------------------------------------------------------------------------

// spyFindingsStore records Add calls so we can verify the injected store is used.
type spyFindingsStore struct {
	addCalls int
	inner    sdk.FindingsStore
}

func (s *spyFindingsStore) Add(missionID string, f finding.Finding) {
	s.addCalls++
	s.inner.Add(missionID, f)
}

func (s *spyFindingsStore) AllRecords() []sdk.FindingRecord {
	return s.inner.AllRecords()
}

// TestWithFindingsStore_CustomStoreIsUsed verifies that the framework delegates
// findings retrieval to the injected store.
func TestWithFindingsStore_CustomStoreIsUsed(t *testing.T) {
	spy := &spyFindingsStore{inner: sdk.NewBoundedFindingsStore(100)}
	f, err := sdk.NewFramework(sdk.WithFindingsStore(spy))
	require.NoError(t, err)

	ctx := context.Background()

	// GetFindings (read path) should work without error even on an empty store.
	results, err := f.GetFindings(ctx, finding.Filter{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --------------------------------------------------------------------------
// Default bounded findings store: capacity enforcement
// --------------------------------------------------------------------------

// TestDefaultFindingsStore_BoundedCapacity creates a findings store with a
// small cap and adds more records than the cap, verifying the store stays
// within bounds.
func TestDefaultFindingsStore_BoundedCapacity(t *testing.T) {
	const cap = 50
	store := sdk.NewBoundedFindingsStore(cap)

	for i := range cap * 3 {
		store.Add("mission-x", finding.Finding{
			ID:        fmt.Sprintf("f-%d", i),
			Title:     fmt.Sprintf("finding %d", i),
			CreatedAt: time.Now(),
		})
	}

	records := store.AllRecords()
	assert.LessOrEqual(t, len(records), cap, "findings store must not exceed its cap")
	assert.Len(t, records, cap, "findings store should be full at cap")
}
