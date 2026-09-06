// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package finding

import "errors"

// Category represents the type of security finding.
//
// Note: Although originally marked deprecated (in favor of using string directly), this type
// remains in active use across the codebase because it provides utility methods (IsValid,
// DisplayName, Description) and is referenced directly in example tests and backward
// compatibility tests. Removal would require a coordinated migration across multiple packages
// in both sdk and gibson modules. The constants below remain the canonical string values.
type Category string

const (
	// CategoryJailbreak indicates attempts to bypass LLM safety controls.
	// Examples: Prompt manipulation to bypass content filters, role-playing attacks
	CategoryJailbreak = "jailbreak"

	// CategoryPromptInjection indicates malicious prompt injection attacks.
	// Examples: System prompt manipulation, indirect prompt injection
	CategoryPromptInjection = "prompt_injection"

	// CategoryDataExtraction indicates unauthorized data access or exfiltration.
	// Examples: Training data extraction, PII leakage, model inversion
	CategoryDataExtraction = "data_extraction"

	// CategoryPrivilegeEscalation indicates unauthorized privilege elevation.
	// Examples: Role hijacking, permission bypass, access control violations
	CategoryPrivilegeEscalation = "privilege_escalation"

	// CategoryDOS indicates denial of service or resource exhaustion attacks.
	// Examples: Token flooding, infinite loops, resource exhaustion
	CategoryDOS = "dos"

	// CategoryModelManipulation indicates attacks that modify model behavior.
	// Examples: Poisoning attacks, backdoor injection, model reprogramming
	CategoryModelManipulation = "model_manipulation"

	// CategoryInformationDisclosure indicates unintended information exposure.
	// Examples: System information leaks, configuration disclosure, metadata exposure
	CategoryInformationDisclosure = "information_disclosure"
)

// IsValid returns true if the category is valid.
// As of the domain-agnostic refactor, any non-empty string is considered valid.
func (c Category) IsValid() bool {
	return string(c) != ""
}

// String returns the string representation of the category.
func (c Category) String() string {
	return string(c)
}

// DisplayName returns a human-readable display name for the category.
func (c Category) DisplayName() string {
	switch c {
	case CategoryJailbreak:
		return "Jailbreak"
	case CategoryPromptInjection:
		return "Prompt Injection"
	case CategoryDataExtraction:
		return "Data Extraction"
	case CategoryPrivilegeEscalation:
		return "Privilege Escalation"
	case CategoryDOS:
		return "Denial of Service"
	case CategoryModelManipulation:
		return "Model Manipulation"
	case CategoryInformationDisclosure:
		return "Information Disclosure"
	default:
		return string(c)
	}
}

// Description returns a brief description of the category.
// For known security categories, returns a detailed description.
// For unknown categories, returns the category string itself.
func (c Category) Description() string {
	switch c {
	case CategoryJailbreak:
		return "Attempts to bypass LLM safety controls and content filters"
	case CategoryPromptInjection:
		return "Malicious prompt injection to manipulate model behavior"
	case CategoryDataExtraction:
		return "Unauthorized access or exfiltration of sensitive data"
	case CategoryPrivilegeEscalation:
		return "Unauthorized elevation of privileges or permissions"
	case CategoryDOS:
		return "Denial of service or resource exhaustion attacks"
	case CategoryModelManipulation:
		return "Attacks that modify or reprogram model behavior"
	case CategoryInformationDisclosure:
		return "Unintended exposure of system or sensitive information"
	default:
		return string(c)
	}
}

// ParseCategory parses a string into a Category value.
// As of the domain-agnostic refactor, accepts any non-empty string.
// Returns an error only if the string is empty.
func ParseCategory(s string) (Category, error) {
	if s == "" {
		return "", errors.New("category cannot be empty")
	}
	return Category(s), nil
}

// AllCategories returns all well-known security categories.
func AllCategories() []string {
	return []string{
		CategoryJailbreak,
		CategoryPromptInjection,
		CategoryDataExtraction,
		CategoryPrivilegeEscalation,
		CategoryDOS,
		CategoryModelManipulation,
		CategoryInformationDisclosure,
	}
}
