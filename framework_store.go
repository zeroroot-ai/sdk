// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package sdk

import (
	"container/list"
	"sync"

	"github.com/zeroroot-ai/sdk/finding"
)

// DefaultMissionStoreCapacity is the maximum number of missions retained by the
// default bounded in-memory store.  Oldest entries are evicted when the cap is
// reached.
const DefaultMissionStoreCapacity = 10_000

// MissionStore is the storage abstraction for missions.  Implementations must
// be safe for concurrent use by multiple goroutines.
type MissionStore interface {
	// Set adds or replaces the mission with the given ID.
	Set(id string, m *Mission)

	// Get returns the mission for the given ID and a boolean indicating
	// whether the ID was present.
	Get(id string) (*Mission, bool)

	// All returns a snapshot of every mission currently in the store.
	// The order of the returned slice is unspecified.
	All() []*Mission
}

// FindingRecord wraps a finding with its mission context.
// It replaces the unexported findingRecord type.
type FindingRecord struct {
	MissionID string
	Finding   finding.Finding
}

// FindingsStore is the storage abstraction for finding records.
// Implementations must be safe for concurrent use by multiple goroutines.
type FindingsStore interface {
	// Add appends a finding record to the store.
	Add(missionID string, f finding.Finding)

	// AllRecords returns a snapshot of every finding record currently in the
	// store.  The order of the returned slice is unspecified.
	AllRecords() []FindingRecord
}

// --------------------------------------------------------------------------
// boundedMissionStore — default bounded LRU implementation of MissionStore
// --------------------------------------------------------------------------

// boundedMissionStore keeps at most cap missions.  When the cap is exceeded the
// oldest entry (by insertion/update time) is evicted.  The store is internally
// synchronised; callers must not hold any lock when calling its methods.
type boundedMissionStore struct {
	mu  sync.Mutex
	cap int
	idx map[string]*list.Element // mission ID → element in lru
	lru list.List                // front = most recently used
}

type missionEntry struct {
	id      string
	mission *Mission
}

// NewBoundedMissionStore returns a MissionStore backed by a bounded LRU cache
// with the given capacity.  Panics if cap < 1.
//
// This is the same implementation used by default when no WithMissionStore
// option is provided to NewFramework.
func NewBoundedMissionStore(cap int) MissionStore {
	if cap < 1 {
		panic("boundedMissionStore: capacity must be >= 1")
	}
	s := &boundedMissionStore{
		cap: cap,
		idx: make(map[string]*list.Element, cap),
	}
	s.lru.Init()
	return s
}

// Set adds or replaces the entry for id, promoting it to the front.
// If the store is full and id is new, the oldest entry is evicted.
func (s *boundedMissionStore) Set(id string, m *Mission) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if elem, ok := s.idx[id]; ok {
		// Update existing entry and move to front.
		elem.Value.(*missionEntry).mission = m
		s.lru.MoveToFront(elem)
		return
	}

	// Evict the oldest entry if at capacity.
	for len(s.idx) >= s.cap {
		back := s.lru.Back()
		if back == nil {
			break
		}
		s.lru.Remove(back)
		delete(s.idx, back.Value.(*missionEntry).id)
	}

	elem := s.lru.PushFront(&missionEntry{id: id, mission: m})
	s.idx[id] = elem
}

// Get returns the mission for id, moving it to the front on a hit.
func (s *boundedMissionStore) Get(id string) (*Mission, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	elem, ok := s.idx[id]
	if !ok {
		return nil, false
	}
	s.lru.MoveToFront(elem)
	return elem.Value.(*missionEntry).mission, true
}

// All returns a snapshot of all missions, without changing LRU order.
func (s *boundedMissionStore) All() []*Mission {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*Mission, 0, len(s.idx))
	for elem := s.lru.Front(); elem != nil; elem = elem.Next() {
		out = append(out, elem.Value.(*missionEntry).mission)
	}
	return out
}

// --------------------------------------------------------------------------
// boundedFindingsStore — default bounded implementation of FindingsStore
// --------------------------------------------------------------------------

// DefaultFindingsStoreCapacity is the maximum number of individual finding
// records retained by the default bounded in-memory store.
const DefaultFindingsStoreCapacity = 10_000

// boundedFindingsStore keeps the most recent N finding records.  When the cap
// is exceeded the oldest record is dropped.
type boundedFindingsStore struct {
	mu      sync.Mutex
	cap     int
	records []FindingRecord
	head    int // index of next write slot (ring buffer)
	count   int // number of valid records
}

// NewBoundedFindingsStore returns a FindingsStore backed by a bounded ring
// buffer with the given capacity.  Panics if cap < 1.
//
// This is the same implementation used by default when no WithFindingsStore
// option is provided to NewFramework.
func NewBoundedFindingsStore(cap int) FindingsStore {
	if cap < 1 {
		panic("boundedFindingsStore: capacity must be >= 1")
	}
	return &boundedFindingsStore{
		cap:     cap,
		records: make([]FindingRecord, cap),
	}
}

// Add appends a finding record, evicting the oldest when at capacity.
func (s *boundedFindingsStore) Add(missionID string, f finding.Finding) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[s.head] = FindingRecord{MissionID: missionID, Finding: f}
	s.head = (s.head + 1) % s.cap
	if s.count < s.cap {
		s.count++
	}
}

// AllRecords returns a snapshot of all stored finding records in insertion
// order (oldest first).
func (s *boundedFindingsStore) AllRecords() []FindingRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.count == 0 {
		return nil
	}

	out := make([]FindingRecord, s.count)

	if s.count < s.cap {
		// Buffer not yet wrapped: records are in [0, s.count).
		copy(out, s.records[:s.count])
		return out
	}

	// Buffer is full and has wrapped; oldest entry is at s.head.
	n := copy(out, s.records[s.head:])
	copy(out[n:], s.records[:s.head])
	return out
}
