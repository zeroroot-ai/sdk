// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package serve

// Option is a functional option for configuring a Server.
// Options provide a flexible way to customize server behavior
// without requiring a large number of constructor parameters.
type Option func(*Config)
