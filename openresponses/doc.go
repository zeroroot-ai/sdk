// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

// Package openresponses holds the Open Responses item types and the one
// client that an agent uses to call a model.
//
// Open Responses is the open specification at https://www.openresponses.org.
// The conversation of an agent is a list of items. An item is a message, a
// function call, the output of a function call, or a reasoning record. The
// agent holds these items from the first message to the last tool result and
// sends them as they are.
//
// The client has no code for a model provider. It sends one request shape to
// the model gateway: POST {base URL}/v1/responses. The gateway does the
// conversion to the provider.
//
// The Client has four calls:
//
//   - Create makes one call and returns the whole response.
//   - Stream makes one call and returns the server-sent events.
//   - CreateStructured makes one call with a JSON schema and decodes the reply.
//   - A call with tools is Create with Request.Tools set.
package openresponses
