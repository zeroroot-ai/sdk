// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package mission provides types and interfaces for mission management in Gibson.
//
// The mission package enables agents to autonomously create, manage, and monitor
// sub-missions. This allows for hierarchical mission structures where a parent
// mission can spawn child missions to handle specialized tasks.
//
// # MissionManager Interface
//
// Agents access mission management through the MissionManager interface,
// which is available via the agent harness:
//
//	// Inside an agent's Execute method
//	mm := harness.MissionManager()
//
//	// Create a sub-mission
//	info, err := mm.CreateMission(ctx, mission, targetID, &mission.CreateMissionOpts{
//	    Name: "network-scan-phase-2",
//	    Constraints: &mission.MissionConstraints{
//	        MaxDuration: 30 * time.Minute,
//	        MaxTokens:   100000,
//	    },
//	    Tags: []string{"automated", "network"},
//	})
//
//	// Start the mission
//	err = mm.RunMission(ctx, info.ID, &mission.RunMissionOpts{Wait: false})
//
//	// Check status
//	status, err := mm.GetMissionStatus(ctx, info.ID)
//	fmt.Printf("Progress: %.0f%%\n", status.Progress * 100)
//
// # Mission Lifecycle
//
// Missions go through the following states:
//
//   - Pending: Mission is created but not yet running
//   - Running: Mission is actively executing
//   - Paused: Mission execution is temporarily suspended
//   - Completed: Mission finished successfully
//   - Failed: Mission encountered an unrecoverable error
//   - Cancelled: Mission was stopped by user or agent
//
// Use MissionStatus.IsTerminal() to check if a mission has finished:
//
//	status, _ := mm.GetMissionStatus(ctx, missionID)
//	if status.Status.IsTerminal() {
//	    // Mission is done - retrieve results
//	    results, _ := mm.GetMissionResults(ctx, missionID)
//	}
//
// # Mission Constraints
//
// Constraints prevent runaway missions from consuming excessive resources:
//
//	constraints := &mission.MissionConstraints{
//	    MaxDuration: 1 * time.Hour,        // Maximum execution time
//	    MaxTokens:   500000,               // Maximum LLM tokens
//	    MaxCost:     10.0,                 // Maximum API cost in dollars
//	    MaxFindings: 1000,                 // Maximum findings to generate
//	}
//
// Zero values for any constraint field means no limit.
//
// # Waiting for Completion
//
// Use WaitForMission for synchronous completion:
//
//	result, err := mm.WaitForMission(ctx, missionID, 30*time.Minute)
//	if err != nil {
//	    // Handle timeout or error
//	}
//
// Or use RunMission with Wait: true for simpler cases:
//
//	err = mm.RunMission(ctx, missionID, &mission.RunMissionOpts{
//	    Wait:    true,
//	    Timeout: 30 * time.Minute,
//	})
//
// # Listing and Filtering Missions
//
// Query missions using MissionFilter:
//
//	// Find all running child missions
//	running := mission.MissionStatusRunning
//	missions, err := mm.ListMissions(ctx, &mission.MissionFilter{
//	    Status:          &running,
//	    ParentMissionID: &parentID,
//	})
//
//	// Find missions by tag
//	missions, err := mm.ListMissions(ctx, &mission.MissionFilter{
//	    Tags:  []string{"network", "critical"},
//	    Limit: 50,
//	})
//
// # Cancelling Missions
//
// Stop a running mission:
//
//	err := mm.CancelMission(ctx, missionID)
//
// Cancellation is idempotent - calling it multiple times is safe.
// Child missions are automatically cancelled when a parent is cancelled.
//
// # Retrieving Results
//
// After a mission completes, retrieve its results:
//
//	result, err := mm.GetMissionResults(ctx, missionID)
//	if err != nil {
//	    return err
//	}
//
//	// Process findings
//	for _, finding := range result.Findings {
//	    fmt.Printf("Found %s: %s\n", finding.Severity, finding.Title)
//	}
//
//	// Check metrics
//	fmt.Printf("Completed in %v, used %d tokens\n",
//	    result.Metrics.Duration, result.Metrics.TokensUsed)
//
// # Thread Safety
//
// All MissionManager methods are safe for concurrent use.
// Multiple goroutines can create, query, and manage missions simultaneously.
package mission
