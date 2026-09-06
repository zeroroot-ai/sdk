// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package eval provides a lightweight evaluation framework for assessing AI agent performance.
// It integrates with Go's standard testing package and provides domain-specific scorers
// for security testing agents in the Gibson Framework.
//
// # GOEVALS=1 Opt-In Pattern
//
// Evaluation tests are opt-in using the GOEVALS=1 environment variable. This allows
// eval tests to coexist with regular unit tests without slowing down the test suite:
//
//	// Regular test - always runs
//	func TestAgentBasics(t *testing.T) {
//	    // ... standard unit test ...
//	}
//
//	// Eval test - only runs when GOEVALS=1 is set
//	func TestAgentEvaluation(t *testing.T) {
//	    eval.Run(t, "prompt_injection_detection", func(e *eval.E) {
//	        // ... evaluation logic ...
//	    })
//	}
//
// Run evaluations with: GOEVALS=1 go test ./...
//
// Without GOEVALS=1, eval tests are skipped, allowing fast iteration during development.
//
// # Quick Start
//
// Basic evaluation mission:
//
//	func TestMyAgent(t *testing.T) {
//	    eval.Run(t, "sql_injection_detection", func(e *eval.E) {
//	        // 1. Create a sample with expected behavior
//	        sample := eval.Sample{
//	            ID: "sqli-001",
//	            Task: agent.Task{
//	                Context: map[string]any{"objective": "Detect SQL injection in login form"},
//	            },
//	            ExpectedTools: []eval.ExpectedToolCall{
//	                {Name: "http-client", Required: true},
//	                {Name: "sqlmap", Required: true},
//	            },
//	            ExpectedFindings: []eval.GroundTruthFinding{
//	                {
//	                    ID: "sqli-login",
//	                    Severity: "high",
//	                    Category: "sql_injection",
//	                    Title: "SQL Injection in Login Form",
//	                },
//	            },
//	        }
//
//	        // 2. Execute agent with RecordingHarness
//	        harness := eval.NewRecordingHarness(actualHarness)
//	        result, err := myAgent.Execute(context.Background(), harness, sample.Task)
//	        if err != nil {
//	            t.Fatalf("agent execution failed: %v", err)
//	        }
//
//	        // 3. Capture trajectory and result
//	        sample.Trajectory = harness.Trajectory()
//	        sample.Result = result
//
//	        // 4. Score with multiple scorers
//	        evalResult := e.Score(sample,
//	            eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{}),
//	            eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
//	                ExpectedOutput: "vulnerability found",
//	                Binary: true,
//	            }),
//	            eval.NewFindingAccuracyScorer(eval.FindingAccuracyOptions{}),
//	        )
//
//	        // 5. Assert minimum score threshold
//	        e.RequireScore(evalResult, 0.8)
//	    })
//	}
//
// # Available Scorers
//
// The eval package provides specialized scorers for different aspects of agent performance:
//
// ToolCorrectnessScorer evaluates whether the agent called the correct tools with correct arguments.
// It compares actual tool calls from the trajectory against expected tool calls, supporting both
// ordered and unordered matching modes. Useful for verifying agent follows expected execution patterns.
//
//	scorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{
//	    OrderMatters: true,  // Enforce call sequence
//	    NumericTolerance: 0.01,  // Allow minor numeric differences
//	})
//
// TaskCompletionScorer evaluates whether the agent successfully completed its assigned task.
// It supports multiple evaluation modes: exact output matching, fuzzy string comparison, and
// LLM-as-judge evaluation using a rubric. Can combine multiple modes for comprehensive scoring.
//
//	scorer := eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
//	    ExpectedOutput: "vulnerability detected",
//	    FuzzyThreshold: 0.8,  // 80% similarity required
//	    Binary: true,  // Round to 0 or 1
//	})
//
// FindingAccuracyScorer evaluates the accuracy of security findings discovered by the agent.
// It calculates precision, recall, and F1 score by comparing actual findings against ground truth.
// Supports severity weighting and category matching for fine-grained evaluation.
//
//	scorer := eval.NewFindingAccuracyScorer(eval.FindingAccuracyOptions{
//	    MatchBySeverity: true,  // Weight by severity (critical=4, high=3, etc.)
//	    MatchByCategory: true,  // Require category match
//	    FuzzyTitleThreshold: 0.8,  // Allow fuzzy title matching
//	})
//
// TrajectoryScorer evaluates whether the agent's execution path matches expected steps.
// It supports three matching modes: exact sequence, subset (any order), and ordered subset
// (maintains relative order but allows extras). Useful for verifying reasoning patterns.
//
//	scorer := eval.NewTrajectoryScorer(eval.TrajectoryOptions{
//	    ExpectedSteps: []eval.ExpectedStep{
//	        {Type: "tool", Name: "mytool", Required: true},
//	        {Type: "llm", Name: "primary", Required: true},
//	        {Type: "finding", Name: "", Required: true},
//	    },
//	    Mode: eval.TrajectoryOrderedSubset,
//	    PenalizeExtra: 0.05,  // 5% penalty per extra step
//	})
//
// LLMJudgeScorer uses an LLM to evaluate agent performance based on a custom rubric.
// This provides flexible, nuanced evaluation for complex tasks that don't fit rule-based scoring.
// Includes automatic retry logic for JSON parsing failures and token usage tracking.
//
//	scorer, err := eval.NewLLMJudgeScorer(eval.LLMJudgeOptions{
//	    Provider: llmProvider,
//	    Rubric: "Score 1.0 if all vulnerabilities found, 0.0 if none",
//	    Temperature: 0.0,  // Deterministic for reproducibility
//	    TokenTracker: &tokenUsage,  // Track costs
//	    IncludeTrajectory: true,  // Include execution details
//	})
//
// # RecordingHarness
//
// RecordingHarness is a transparent wrapper around agent.Harness that records all operations
// as trajectory steps. It captures tool calls, LLM completions, memory operations, findings,
// and more for later analysis.
//
//	// Create recording harness wrapping actual harness
//	recorder := eval.NewRecordingHarness(actualHarness)
//
//	// Execute agent normally - all operations are recorded
//	result, err := agent.Execute(ctx, recorder, task)
//
//	// Retrieve recorded trajectory
//	trajectory := recorder.Trajectory()
//
//	// Trajectory contains all steps with inputs, outputs, timing, errors
//	for _, step := range trajectory.Steps {
//	    fmt.Printf("%s: %s (%.2fs)\n", step.Type, step.Name, step.Duration.Seconds())
//	}
//
// The harness records these operation types:
//   - "tool": Tool invocations with arguments and results
//   - "llm": LLM completions (both regular and streaming)
//   - "delegate": Agent-to-agent task delegation
//   - "finding": Security finding submissions
//   - "memory": Memory store operations (get, set, delete, list)
//   - "plugin": Plugin queries
//   - "graphrag": GraphRAG operations (queries, storage, traversal)
//
// # Eval Set Loading
//
// Evaluation sets are collections of samples stored in JSON or YAML files. They provide
// reusable test suites for regression testing and benchmarking:
//
//	// Load evaluation set from file
//	evalSet, err := eval.LoadEvalSet("testdata/sql-injection.yaml")
//	if err != nil {
//	    t.Fatalf("failed to load eval set: %v", err)
//	}
//
//	// Filter by tags (e.g., run only "smoke" tests)
//	filtered := evalSet.FilterByTags([]string{"smoke", "critical"})
//
//	// Run all samples through scorers
//	for _, sample := range filtered.Samples {
//	    // Execute agent and populate sample.Result and sample.Trajectory
//	    // ... execution logic ...
//
//	    result := e.Score(sample, scorers...)
//	    e.RequireScore(result, 0.8)
//	}
//
// Eval sets support YAML and JSON formats with automatic format detection:
//
//	# sql-injection.yaml
//	name: "SQL Injection Test Suite"
//	version: "1.0.0"
//	metadata:
//	  author: "security-team"
//	  created: "2025-01-05"
//	samples:
//	  - id: "sqli-001"
//	    task:
//	      goal: "Detect SQL injection in login form"
//	    expected_tools:
//	      - name: "http-client"
//	        required: true
//	    expected_findings:
//	      - id: "sqli-login"
//	        severity: "high"
//	        category: "sql_injection"
//	        title: "SQL Injection in Login Form"
//	    tags: ["smoke", "critical"]
//
// # Results Logging
//
// Evaluation results can be persisted to JSONL (JSON Lines) files for analysis, tracking
// improvements over time, and integration with external tools:
//
//	// Create JSONL logger
//	logger, err := eval.NewJSONLLogger("evals.jsonl")
//	if err != nil {
//	    t.Fatalf("failed to create logger: %v", err)
//	}
//	defer logger.Close()
//
//	// Configure evaluation with logger
//	eval.Run(t, "my_eval", func(e *eval.E) {
//	    e.WithLogger(logger)
//
//	    // Scores are automatically logged after each evaluation
//	    result := e.Score(sample, scorers...)
//	})
//
// Each evaluation produces a single JSON line in the log file:
//
//	{"timestamp":"2025-01-05T10:30:00Z","sample_id":"sqli-001","scores":{"tool_correctness":0.9,"task_completion":1.0},"overall_score":0.95,"duration_ms":1250}
//
// The JSONL format is streaming-friendly and easily processed by tools like jq, pandas, or BigQuery.
//
// # OpenTelemetry Integration
//
// Evaluations can emit metrics and traces to OpenTelemetry for monitoring and alerting:
//
//	// Configure OTel with your tracer and meter provider
//	eval.Run(t, "my_eval", func(e *eval.E) {
//	    e.WithOTel(eval.OTelOptions{
//	        Tracer: otelTracer,
//	        MeterProvider: meterProvider,
//	    })
//
//	    // Each Score() call creates:
//	    // - A span with score attributes
//	    // - Metrics for score histograms and counters
//	    result := e.Score(sample, scorers...)
//	})
//
// Emitted metrics include:
//   - eval.score (histogram): Distribution of evaluation scores
//   - eval.count (counter): Number of evaluations run
//   - eval.duration (histogram): Evaluation execution time
//
// Spans include attributes:
//   - eval.sample_id: Sample identifier
//   - eval.overall_score: Aggregated score
//   - eval.scorer_<name>: Individual scorer results
//
// # Langfuse Integration
//
// Langfuse is a observability platform for LLM applications. The eval package can export
// scores to Langfuse for visualization and analysis:
//
//	// Create Langfuse exporter
//	exporter := eval.NewLangfuseExporter(eval.LangfuseOptions{
//	    BaseURL: "https://cloud.langfuse.com",
//	    PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
//	    SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
//	})
//
//	eval.Run(t, "my_eval", func(e *eval.E) {
//	    e.WithLangfuse(exporter)
//
//	    // Scores are automatically exported to Langfuse dashboard
//	    result := e.Score(sample, scorers...)
//	})
//
// Langfuse integration provides:
//   - Score tracking over time
//   - Comparison across model versions
//   - Drill-down into individual evaluations
//   - Team collaboration features
//
// # Real-Time Evaluation Feedback
//
// Real-time evaluation provides streaming feedback during agent execution, allowing
// agents to adapt their behavior based on evaluation scores. This contrasts with
// traditional post-hoc evaluation where scores are computed after completion.
//
// When to use each approach:
//   - Real-time: Long-running agents, adaptive tasks, autonomous correction
//   - Post-hoc: Benchmarking, final validation, historical analysis
//   - Combined: Use both for comprehensive evaluation and real-time guidance
//
// # StreamingScorer Interface
//
// Streaming scorers extend the standard Scorer interface with ScorePartial() for
// evaluating incomplete trajectories. Not all scorers are suitable for streaming:
//
//	type StreamingScorer interface {
//	    Scorer
//	    ScorePartial(ctx context.Context, trajectory Trajectory) (PartialScore, error)
//	    SupportsStreaming() bool
//	}
//
// PartialScore includes:
//   - Score: Current score (0.0 to 1.0)
//   - Confidence: How confident the scorer is in this partial score (0.0 to 1.0)
//   - Status: ScoreStatusPending, ScoreStatusPartial, or ScoreStatusComplete
//   - Action: ActionContinue, ActionAdjust, ActionReconsider, or ActionAbort
//   - Feedback: Human-readable guidance for the agent
//
// Example streaming scorer usage:
//
//	scorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{})
//	if scorer.SupportsStreaming() {
//	    partial, err := scorer.ScorePartial(ctx, trajectory)
//	    if partial.Action == eval.ActionReconsider {
//	        // Agent should change strategy
//	    }
//	}
//
// # FeedbackHarness Usage
//
// FeedbackHarness wraps an agent.Harness with real-time evaluation capabilities.
// It automatically records trajectory steps and triggers scoring based on frequency settings:
//
//	// Create feedback harness with multiple streaming scorers
//	harness := eval.NewFeedbackHarness(innerHarness, eval.FeedbackOptions{
//	    Scorers: []eval.StreamingScorer{
//	        eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{}),
//	        eval.NewTrajectoryScorer(eval.TrajectoryOptions{
//	            ExpectedSteps: expectedSteps,
//	        }),
//	    },
//	    WarningThreshold:  0.5,  // Warn when score < 0.5
//	    CriticalThreshold: 0.2,  // Alert when score < 0.2
//	    Frequency: eval.FeedbackFrequency{
//	        EveryNSteps: 5,            // Evaluate every 5 steps
//	        Debounce:    time.Second,  // At most once per second
//	        OnThreshold: true,         // Always evaluate on threshold breach
//	    },
//	    AutoInject: false,  // Agent must explicitly call GetFeedback()
//	})
//	defer harness.Close()  // Stop background evaluator
//
//	// Execute agent with feedback-aware harness
//	result, err := agent.Execute(ctx, harness, task)
//
// Frequency settings control when evaluations are triggered:
//   - EveryNSteps: Evaluate after every Nth trajectory step (default: 1)
//   - Debounce: Minimum time between evaluations to prevent rapid-fire scoring
//   - OnThreshold: Always evaluate immediately on threshold breach (requires prior evaluation)
//
// Threshold settings determine when alerts are generated:
//   - WarningThreshold: Score below this triggers warning alerts (default: 0.5)
//   - CriticalThreshold: Score below this triggers critical alerts (default: 0.2)
//
// # Threshold Alerts
//
// When scores breach thresholds, alerts are generated with recommendations:
//
//	feedback := harness.GetFeedback()
//	if feedback != nil {
//	    for _, alert := range feedback.Alerts {
//	        switch alert.Level {
//	        case eval.AlertWarning:
//	            // Score is below expected but not critical
//	            // Recommended action: ActionAdjust
//	        case eval.AlertCritical:
//	            // Score is critically low
//	            // Recommended action: ActionAbort or ActionReconsider
//	        }
//	    }
//	}
//
// Alerts include:
//   - Level: AlertWarning or AlertCritical
//   - Scorer: Which scorer triggered the alert (empty for overall score)
//   - Score: The score that breached the threshold
//   - Threshold: The threshold value that was breached
//   - Message: Human-readable description
//   - Action: Recommended action to take
//
// # Feedback Consumption Patterns
//
// Agents can consume feedback in three ways:
//
// 1. Manual Polling - Agent explicitly checks for feedback:
//
//	func Execute(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
//	    // Type assert to access feedback methods
//	    fh, ok := h.(*eval.FeedbackHarness)
//	    if !ok {
//	        // Not a feedback harness, continue normally
//	        return executeNormally(ctx, h, task)
//	    }
//
//	    for i := 0; i < maxIterations; i++ {
//	        // Perform some work
//	        resp, _ := h.Complete(ctx, "primary", messages)
//
//	        // Check for feedback
//	        if feedback := fh.GetFeedback(); feedback != nil {
//	            switch feedback.Overall.Action {
//	            case eval.ActionContinue:
//	                // Keep going
//	            case eval.ActionAdjust:
//	                // Make minor adjustments
//	                messages = adjustStrategy(messages, feedback)
//	            case eval.ActionReconsider:
//	                // Significant change needed
//	                return reconsiderApproach(ctx, h, feedback)
//	            case eval.ActionAbort:
//	                // Stop execution
//	                return agent.Result{Status: "aborted"}, nil
//	            }
//	        }
//	    }
//	    return finalResult, nil
//	}
//
// 2. Auto-Injection - Feedback is automatically prepended to LLM calls:
//
//	harness := eval.NewFeedbackHarness(innerHarness, eval.FeedbackOptions{
//	    Scorers:    scorers,
//	    AutoInject: true,  // Enable automatic injection
//	})
//
//	// Feedback is automatically injected as a system message
//	// before each LLM completion. The agent doesn't need to
//	// explicitly check for feedback - it's in the message history.
//	resp, err := harness.Complete(ctx, "primary", messages)
//
// 3. FormatForLLM - Agent formats feedback for explicit inclusion:
//
//	feedback := fh.GetFeedback()
//	if feedback != nil {
//	    // Format feedback as text for LLM consumption
//	    feedbackText := feedback.FormatForLLM()
//
//	    // Inject into prompt or system message
//	    messages = append(messages, llm.Message{
//	        Role:    "system",
//	        Content: feedbackText,
//	    })
//
//	    resp, _ := h.Complete(ctx, "primary", messages)
//	}
//
// FormatForLLM() output includes:
//   - Overall score and confidence
//   - Recommended action with guidance
//   - Threshold breach alerts
//   - Individual scorer feedback
//   - Actionable suggestions based on the action recommendation
//
// # Example: Feedback-Aware Agent
//
// Complete example of an agent that adapts based on real-time feedback:
//
//	func TestFeedbackAwareAgent(t *testing.T) {
//	    eval.Run(t, "feedback_aware_agent", func(e *eval.E) {
//	        // Create streaming scorers
//	        toolScorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{})
//	        trajScorer := eval.NewTrajectoryScorer(eval.TrajectoryOptions{
//	            ExpectedSteps: []eval.ExpectedStep{
//	                {Type: "tool", Name: "mytool", Required: true},
//	                {Type: "finding", Name: "", Required: true},
//	            },
//	            Mode: eval.TrajectoryOrderedSubset,
//	        })
//
//	        // Create feedback harness
//	        harness := eval.NewFeedbackHarness(baseHarness, eval.FeedbackOptions{
//	            Scorers: []eval.StreamingScorer{toolScorer, trajScorer},
//	            WarningThreshold:  0.6,
//	            CriticalThreshold: 0.3,
//	            Frequency: eval.FeedbackFrequency{
//	                EveryNSteps: 3,
//	                Debounce:    500 * time.Millisecond,
//	                OnThreshold: true,
//	            },
//	            AutoInject: false,
//	        })
//	        defer harness.Close()
//
//	        // Execute agent
//	        result, err := adaptiveAgent.Execute(ctx, harness, sample.Task)
//	        if err != nil {
//	            t.Fatalf("execution failed: %v", err)
//	        }
//
//	        // Analyze feedback history
//	        history := harness.FeedbackHistory()
//	        for i, feedback := range history {
//	            t.Logf("Feedback %d: score=%.2f action=%s",
//	                i, feedback.Overall.Score, feedback.Overall.Action)
//	        }
//
//	        // Get final trajectory for post-hoc analysis
//	        trajectory := harness.RecordingHarness().Trajectory()
//	        sample.Trajectory = trajectory
//	        sample.Result = result
//
//	        // Post-hoc evaluation with final scorers
//	        evalResult := e.Score(sample,
//	            eval.NewFindingAccuracyScorer(eval.FindingAccuracyOptions{}),
//	            eval.NewTaskCompletionScorer(eval.TaskCompletionOptions{
//	                ExpectedOutput: "vulnerability found",
//	            }),
//	        )
//
//	        e.RequireScore(evalResult, 0.8)
//	    })
//	}
//
// The adaptive agent implementation:
//
//	func (a *AdaptiveAgent) Execute(ctx context.Context, h agent.Harness, task agent.Task) (agent.Result, error) {
//	    // Check if feedback is available
//	    fh, hasFeedback := h.(*eval.FeedbackHarness)
//
//	    strategy := a.defaultStrategy
//	    messages := []llm.Message{{Role: "user", Content: task.Context["objective"]}}
//
//	    for attempt := 0; attempt < maxAttempts; attempt++ {
//	        // Execute current strategy
//	        resp, err := h.Complete(ctx, "primary", messages)
//	        if err != nil {
//	            return agent.Result{}, err
//	        }
//
//	        // Check feedback if available
//	        if hasFeedback {
//	            feedback := fh.GetFeedback()
//	            if feedback != nil && feedback.Overall.Action != eval.ActionContinue {
//	                // Adapt strategy based on feedback
//	                strategy = a.adaptStrategy(strategy, feedback)
//
//	                // Include feedback in next message
//	                messages = append(messages, llm.Message{
//	                    Role:    "system",
//	                    Content: feedback.FormatForLLM(),
//	                })
//
//	                // Log adaptation
//	                h.Logger().Info("adapting strategy based on feedback",
//	                    "action", feedback.Overall.Action,
//	                    "score", feedback.Overall.Score)
//	            }
//	        }
//
//	        // Continue with potentially adapted strategy
//	        // ... rest of agent logic ...
//	    }
//
//	    return finalResult, nil
//	}
//
// # Best Practices
//
// 1. Use descriptive sample IDs: "sqli-login-001" not "test1"
// 2. Tag samples for filtering: ["smoke", "regression", "critical"]
// 3. Set realistic score thresholds: 0.8 for production agents
// 4. Track token usage with LLM judges to manage costs
// 5. Log results to JSONL for historical analysis
// 6. Use RecordingHarness in production for debugging
// 7. Combine multiple scorers for comprehensive evaluation
// 8. Version your eval sets with semantic versioning
// 9. Run smoke tests in CI, full evals nightly
// 10. Review false positives/negatives to improve ground truth
//
// # Performance Considerations
//
// Evaluations can be expensive, especially with LLM judges:
//
//   - Use Binary mode in TaskCompletionScorer to reduce LLM calls
//   - Set Temperature=0.0 for deterministic, cacheable LLM responses
//   - Filter eval sets by tags to run only relevant tests
//   - Use TrajectoryScorer for fast rule-based evaluation
//   - Track TokenUsage to monitor and limit LLM costs
//   - Run expensive evals nightly, fast ones in CI
//
// # Example: Complete Evaluation Pipeline
//
//	func TestAgentComprehensive(t *testing.T) {
//	    eval.Run(t, "comprehensive_eval", func(e *eval.E) {
//	        // Setup logging and observability
//	        logger, _ := eval.NewJSONLLogger("evals.jsonl")
//	        defer logger.Close()
//
//	        e.WithLogger(logger).
//	          WithOTel(eval.OTelOptions{
//	              Tracer: tracer,
//	              MeterProvider: meterProvider,
//	          })
//
//	        // Load eval set
//	        evalSet, _ := eval.LoadEvalSet("testdata/suite.yaml")
//	        smokeTests := evalSet.FilterByTags([]string{"smoke"})
//
//	        // Create scorers
//	        toolScorer := eval.NewToolCorrectnessScorer(eval.ToolCorrectnessOptions{})
//	        findingScorer := eval.NewFindingAccuracyScorer(eval.FindingAccuracyOptions{
//	            MatchBySeverity: true,
//	        })
//	        llmJudge, _ := eval.NewLLMJudgeScorer(eval.LLMJudgeOptions{
//	            Provider: llmProvider,
//	            Rubric: "Score based on completeness and accuracy",
//	        })
//
//	        // Run evaluations
//	        for _, sample := range smokeTests.Samples {
//	            // Execute agent with recording
//	            recorder := eval.NewRecordingHarness(harness)
//	            result, _ := agent.Execute(ctx, recorder, sample.Task)
//
//	            sample.Trajectory = recorder.Trajectory()
//	            sample.Result = result
//
//	            // Score and assert
//	            evalResult := e.Score(sample, toolScorer, findingScorer, llmJudge)
//	            e.RequireScore(evalResult, 0.8)
//	        }
//	    })
//	}
package eval
