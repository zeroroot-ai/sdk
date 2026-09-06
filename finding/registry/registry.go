// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package registry

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

// CategoryInfo describes metadata for a registered category.
// This allows domain-specific categories to be registered with display
// information for UIs and validation.
type CategoryInfo struct {
	// Name is the category identifier (e.g., "jailbreak", "compliance_drift")
	Name string `json:"name"`

	// Domain groups related categories (e.g., "security", "compliance", "infrastructure")
	Domain string `json:"domain"`

	// DisplayName is the human-readable name (e.g., "Jailbreak Attack")
	DisplayName string `json:"display_name"`

	// Description provides context about the category
	Description string `json:"description"`

	// Color is an optional hex color for UI display (e.g., "#FF0000")
	Color string `json:"color,omitempty"`

	// Icon is an optional icon identifier for UI display
	Icon string `json:"icon,omitempty"`
}

// CategoryRegistry provides thread-safe category registration and validation.
// It enables domain-specific category management while maintaining backward
// compatibility with string-based categories.
type CategoryRegistry struct {
	mu         sync.RWMutex
	categories map[string]CategoryInfo
	domains    map[string][]string // domain -> category names
}

// NewCategoryRegistry creates a new empty category registry.
func NewCategoryRegistry() *CategoryRegistry {
	return &CategoryRegistry{
		categories: make(map[string]CategoryInfo),
		domains:    make(map[string][]string),
	}
}

// RegisterCategory adds a category to the registry.
// Returns an error if the category name is empty or already registered.
func (r *CategoryRegistry) RegisterCategory(info CategoryInfo) error {
	if info.Name == "" {
		return errors.New("category name cannot be empty")
	}
	if info.Domain == "" {
		return errors.New("category domain cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if _, exists := r.categories[info.Name]; exists {
		return fmt.Errorf("category %q already registered", info.Name)
	}

	// Register the category
	r.categories[info.Name] = info

	// Add to domain index
	r.domains[info.Domain] = append(r.domains[info.Domain], info.Name)

	return nil
}

// Validate performs soft validation of a category.
// If the category is not registered, it logs a warning but does NOT return an error.
// This allows for extensibility while providing helpful feedback.
func (r *CategoryRegistry) Validate(category string) error {
	if category == "" {
		return errors.New("category cannot be empty")
	}

	r.mu.RLock()
	_, exists := r.categories[category]
	r.mu.RUnlock()

	if !exists {
		// Soft validation: warn but don't error
		log.Printf("WARNING: category %q is not registered in the category registry", category)
	}

	return nil
}

// GetInfo retrieves metadata for a registered category.
// Returns the CategoryInfo and true if found, or an empty CategoryInfo and false if not found.
func (r *CategoryRegistry) GetInfo(category string) (CategoryInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.categories[category]
	return info, exists
}

// ListByDomain returns all categories registered for a specific domain.
// Returns an empty slice if the domain has no registered categories.
func (r *CategoryRegistry) ListByDomain(domain string) []CategoryInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categoryNames, exists := r.domains[domain]
	if !exists {
		return []CategoryInfo{}
	}

	result := make([]CategoryInfo, 0, len(categoryNames))
	for _, name := range categoryNames {
		if info, ok := r.categories[name]; ok {
			result = append(result, info)
		}
	}

	return result
}

// ListAll returns all registered categories.
func (r *CategoryRegistry) ListAll() []CategoryInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]CategoryInfo, 0, len(r.categories))
	for _, info := range r.categories {
		result = append(result, info)
	}

	return result
}

// DefaultRegistry returns a registry pre-populated with security categories.
// This provides backward compatibility with existing security-focused Finding usage.
func DefaultRegistry() *CategoryRegistry {
	registry := NewCategoryRegistry()

	// Security domain categories based on category.go constants
	securityCategories := []CategoryInfo{
		{
			Name:        "jailbreak",
			Domain:      "security",
			DisplayName: "Jailbreak",
			Description: "Attempts to bypass LLM safety controls and content filters",
			Color:       "#FF0000",
			Icon:        "shield-alert",
		},
		{
			Name:        "prompt_injection",
			Domain:      "security",
			DisplayName: "Prompt Injection",
			Description: "Malicious prompt injection to manipulate model behavior",
			Color:       "#FF4500",
			Icon:        "code-injection",
		},
		{
			Name:        "data_extraction",
			Domain:      "security",
			DisplayName: "Data Extraction",
			Description: "Unauthorized access or exfiltration of sensitive data",
			Color:       "#FF6347",
			Icon:        "database-export",
		},
		{
			Name:        "privilege_escalation",
			Domain:      "security",
			DisplayName: "Privilege Escalation",
			Description: "Unauthorized elevation of privileges or permissions",
			Color:       "#DC143C",
			Icon:        "arrow-up-circle",
		},
		{
			Name:        "dos",
			Domain:      "security",
			DisplayName: "Denial of Service",
			Description: "Denial of service or resource exhaustion attacks",
			Color:       "#B22222",
			Icon:        "server-off",
		},
		{
			Name:        "model_manipulation",
			Domain:      "security",
			DisplayName: "Model Manipulation",
			Description: "Attacks that modify or reprogram model behavior",
			Color:       "#8B0000",
			Icon:        "brain-circuit",
		},
		{
			Name:        "information_disclosure",
			Domain:      "security",
			DisplayName: "Information Disclosure",
			Description: "Unintended exposure of system or sensitive information",
			Color:       "#CD5C5C",
			Icon:        "eye-off",
		},
	}

	for _, cat := range securityCategories {
		// Ignore errors during initialization - these should never fail
		_ = registry.RegisterCategory(cat)
	}

	return registry
}
