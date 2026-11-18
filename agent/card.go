// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package agent

// CardSkill is one A2A AgentSkill: a capability entry in the "skills" list of
// an Agent Card.
//
// It is NOT an Agent Skill in the SKILL.md format. The catalog ids of the form
// skill:<name> name Agent Skills. A CardSkill describes what an agent can do,
// so that a client can find the agent. The names in Go and in the registration
// say "card skill" so the two never mix.
type CardSkill struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Examples    []string
	// InputModes and OutputModes override the default modes of the card.
	InputModes  []string
	OutputModes []string
}

// CardDescriber is an optional interface of an Agent. An agent that implements
// it sets the content of its Agent Card. The daemon serves the card from the
// catalog. The SDK sends this content at registration.
type CardDescriber interface {
	// CardSkills returns the capability entries of the card.
	CardSkills() []CardSkill
	// DefaultInputModes returns the media types that the agent accepts.
	DefaultInputModes() []string
	// DefaultOutputModes returns the media types that the agent returns.
	DefaultOutputModes() []string
}

// DefaultCardModes are the media types of a card when the agent names none.
var DefaultCardModes = []string{"text/plain", "application/json"}

// CardOf returns what the Agent Card of an agent needs. An agent that
// implements CardDescriber sets it. For any other agent the SDK builds one card
// skill for each capability, tagged with the target types and the technique
// types, and it uses DefaultCardModes.
func CardOf(a Agent) (skills []CardSkill, inputModes, outputModes []string) {
	if d, ok := a.(CardDescriber); ok {
		skills, inputModes, outputModes = d.CardSkills(), d.DefaultInputModes(), d.DefaultOutputModes()
	} else {
		skills = derivedCardSkills(a)
	}
	if len(inputModes) == 0 {
		inputModes = append([]string(nil), DefaultCardModes...)
	}
	if len(outputModes) == 0 {
		outputModes = append([]string(nil), DefaultCardModes...)
	}
	return skills, inputModes, outputModes
}

func derivedCardSkills(a Agent) []CardSkill {
	tags := append(append([]string(nil), a.TargetTypes()...), a.TechniqueTypes()...)
	caps := a.Capabilities()
	if len(caps) == 0 {
		return []CardSkill{{ID: a.Name(), Name: a.Name(), Description: a.Description(), Tags: tags}}
	}
	out := make([]CardSkill, 0, len(caps))
	for _, c := range caps {
		out = append(out, CardSkill{ID: c, Name: c, Description: a.Description(), Tags: tags})
	}
	return out
}
