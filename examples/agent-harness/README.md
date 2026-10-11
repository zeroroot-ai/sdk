# agent-harness

A Go agent that calls each method of `agent.Harness` once and reads each
result. It is the first-party consumer of the Go Harness (D86).

`harness_test.go` runs the agent against a fake daemon through
`serve.CallbackHarness` (fake `HarnessCallbackService`) and
`serve.PlatformHarness` (fake `ComponentService`). The test checks the request
of each call and the value of each result. It also covers the options of the
callback client and the fork claim of D80.

The unwired-code guard (`make lint-unwired`) counts the reads in this package.
