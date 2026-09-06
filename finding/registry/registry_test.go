// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package registry

import (
	"sync"
	"testing"
)

func TestNewCategoryRegistry(t *testing.T) {
	registry := NewCategoryRegistry()

	if registry == nil {
		t.Fatal("NewCategoryRegistry returned nil")
	}

	if registry.categories == nil {
		t.Error("categories map is nil")
	}

	if registry.domains == nil {
		t.Error("domains map is nil")
	}

	if len(registry.categories) != 0 {
		t.Errorf("new registry should be empty, got %d categories", len(registry.categories))
	}
}

func TestRegisterCategory(t *testing.T) {
	tests := []struct {
		name      string
		info      CategoryInfo
		wantError bool
	}{
		{
			name: "valid security category",
			info: CategoryInfo{
				Name:        "test_vuln",
				Domain:      "security",
				DisplayName: "Test Vulnerability",
				Description: "A test security vulnerability",
				Color:       "#FF0000",
				Icon:        "alert",
			},
			wantError: false,
		},
		{
			name: "valid compliance category",
			info: CategoryInfo{
				Name:        "compliance_drift",
				Domain:      "compliance",
				DisplayName: "Compliance Drift",
				Description: "Configuration drift from compliance baseline",
			},
			wantError: false,
		},
		{
			name: "empty name",
			info: CategoryInfo{
				Name:        "",
				Domain:      "security",
				DisplayName: "Test",
				Description: "Test",
			},
			wantError: true,
		},
		{
			name: "empty domain",
			info: CategoryInfo{
				Name:        "test",
				Domain:      "",
				DisplayName: "Test",
				Description: "Test",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewCategoryRegistry()
			err := registry.RegisterCategory(tt.info)

			if tt.wantError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantError {
				// Verify category was registered
				info, exists := registry.GetInfo(tt.info.Name)
				if !exists {
					t.Error("category was not registered")
				}
				if info.Name != tt.info.Name {
					t.Errorf("expected name %q, got %q", tt.info.Name, info.Name)
				}
				if info.Domain != tt.info.Domain {
					t.Errorf("expected domain %q, got %q", tt.info.Domain, info.Domain)
				}
			}
		})
	}
}

func TestRegisterCategory_Duplicate(t *testing.T) {
	registry := NewCategoryRegistry()

	info := CategoryInfo{
		Name:        "test",
		Domain:      "security",
		DisplayName: "Test",
		Description: "Test",
	}

	// First registration should succeed
	err := registry.RegisterCategory(info)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = registry.RegisterCategory(info)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestValidate(t *testing.T) {
	registry := NewCategoryRegistry()

	// Register a test category
	info := CategoryInfo{
		Name:        "registered_category",
		Domain:      "security",
		DisplayName: "Registered",
		Description: "A registered category",
	}
	_ = registry.RegisterCategory(info)

	tests := []struct {
		name      string
		category  string
		wantError bool
	}{
		{
			name:      "empty category",
			category:  "",
			wantError: true,
		},
		{
			name:      "registered category",
			category:  "registered_category",
			wantError: false,
		},
		{
			name:      "unregistered category - soft validation",
			category:  "unregistered_category",
			wantError: false, // soft validation warns but doesn't error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Validate(tt.category)

			if tt.wantError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetInfo(t *testing.T) {
	registry := NewCategoryRegistry()

	// Register a test category
	expectedInfo := CategoryInfo{
		Name:        "test_category",
		Domain:      "security",
		DisplayName: "Test Category",
		Description: "A test category",
		Color:       "#FF0000",
		Icon:        "test-icon",
	}
	_ = registry.RegisterCategory(expectedInfo)

	// Test getting registered category
	info, exists := registry.GetInfo("test_category")
	if !exists {
		t.Fatal("category should exist")
	}

	if info.Name != expectedInfo.Name {
		t.Errorf("expected name %q, got %q", expectedInfo.Name, info.Name)
	}
	if info.Domain != expectedInfo.Domain {
		t.Errorf("expected domain %q, got %q", expectedInfo.Domain, info.Domain)
	}
	if info.DisplayName != expectedInfo.DisplayName {
		t.Errorf("expected display name %q, got %q", expectedInfo.DisplayName, info.DisplayName)
	}
	if info.Description != expectedInfo.Description {
		t.Errorf("expected description %q, got %q", expectedInfo.Description, info.Description)
	}
	if info.Color != expectedInfo.Color {
		t.Errorf("expected color %q, got %q", expectedInfo.Color, info.Color)
	}
	if info.Icon != expectedInfo.Icon {
		t.Errorf("expected icon %q, got %q", expectedInfo.Icon, info.Icon)
	}

	// Test getting non-existent category
	_, exists = registry.GetInfo("nonexistent")
	if exists {
		t.Error("nonexistent category should not exist")
	}
}

func TestListByDomain(t *testing.T) {
	registry := NewCategoryRegistry()

	// Register categories in different domains
	securityCategories := []CategoryInfo{
		{Name: "sec1", Domain: "security", DisplayName: "Security 1", Description: "Desc 1"},
		{Name: "sec2", Domain: "security", DisplayName: "Security 2", Description: "Desc 2"},
	}

	complianceCategories := []CategoryInfo{
		{Name: "comp1", Domain: "compliance", DisplayName: "Compliance 1", Description: "Desc 1"},
	}

	for _, cat := range securityCategories {
		_ = registry.RegisterCategory(cat)
	}
	for _, cat := range complianceCategories {
		_ = registry.RegisterCategory(cat)
	}

	// Test security domain
	secList := registry.ListByDomain("security")
	if len(secList) != 2 {
		t.Errorf("expected 2 security categories, got %d", len(secList))
	}

	// Test compliance domain
	compList := registry.ListByDomain("compliance")
	if len(compList) != 1 {
		t.Errorf("expected 1 compliance category, got %d", len(compList))
	}

	// Test non-existent domain
	emptyList := registry.ListByDomain("nonexistent")
	if len(emptyList) != 0 {
		t.Errorf("expected empty list for nonexistent domain, got %d", len(emptyList))
	}
}

func TestListAll(t *testing.T) {
	registry := NewCategoryRegistry()

	// Empty registry
	list := registry.ListAll()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Register some categories
	categories := []CategoryInfo{
		{Name: "cat1", Domain: "domain1", DisplayName: "Cat 1", Description: "Desc 1"},
		{Name: "cat2", Domain: "domain2", DisplayName: "Cat 2", Description: "Desc 2"},
		{Name: "cat3", Domain: "domain1", DisplayName: "Cat 3", Description: "Desc 3"},
	}

	for _, cat := range categories {
		_ = registry.RegisterCategory(cat)
	}

	// List all
	list = registry.ListAll()
	if len(list) != 3 {
		t.Errorf("expected 3 categories, got %d", len(list))
	}
}

func TestDefaultRegistry(t *testing.T) {
	registry := DefaultRegistry()

	if registry == nil {
		t.Fatal("DefaultRegistry returned nil")
	}

	// Verify security categories are registered
	expectedCategories := []string{
		"jailbreak",
		"prompt_injection",
		"data_extraction",
		"privilege_escalation",
		"dos",
		"model_manipulation",
		"information_disclosure",
	}

	for _, name := range expectedCategories {
		t.Run("category_"+name, func(t *testing.T) {
			info, exists := registry.GetInfo(name)
			if !exists {
				t.Errorf("category %q should be registered in DefaultRegistry", name)
			}

			if info.Domain != "security" {
				t.Errorf("expected domain 'security', got %q", info.Domain)
			}

			if info.DisplayName == "" {
				t.Error("DisplayName should not be empty")
			}

			if info.Description == "" {
				t.Error("Description should not be empty")
			}
		})
	}

	// Verify all categories are in security domain
	securityList := registry.ListByDomain("security")
	if len(securityList) != len(expectedCategories) {
		t.Errorf("expected %d security categories, got %d", len(expectedCategories), len(securityList))
	}
}

func TestConcurrentAccess(t *testing.T) {
	registry := NewCategoryRegistry()

	// Number of concurrent goroutines
	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent registration
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			for range numOperations {
				// Try to register (some will fail due to duplicates, which is fine)
				info := CategoryInfo{
					Name:        "category",
					Domain:      "test",
					DisplayName: "Test",
					Description: "Test",
				}
				_ = registry.RegisterCategory(info)

				// Read operations
				_, _ = registry.GetInfo("category")
				_ = registry.ListByDomain("test")
				_ = registry.ListAll()

				// Validate
				_ = registry.Validate("category")
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is still functional
	info, exists := registry.GetInfo("category")
	if !exists {
		t.Error("category should exist after concurrent operations")
	}
	if info.Name != "category" {
		t.Errorf("expected name 'category', got %q", info.Name)
	}
}

func TestConcurrentAccessDifferentCategories(t *testing.T) {
	registry := NewCategoryRegistry()

	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine registers a unique category
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()

			info := CategoryInfo{
				Name:        string(rune('a' + id)),
				Domain:      "test",
				DisplayName: "Test",
				Description: "Test",
			}
			_ = registry.RegisterCategory(info)

			// Read while others write
			_ = registry.ListAll()
			_ = registry.ListByDomain("test")
		}(i)
	}

	wg.Wait()

	// Verify all categories were registered
	list := registry.ListByDomain("test")
	if len(list) != numGoroutines {
		t.Errorf("expected %d categories, got %d", numGoroutines, len(list))
	}
}

func BenchmarkRegisterCategory(b *testing.B) {
	registry := NewCategoryRegistry()

	info := CategoryInfo{
		Name:        "bench_category",
		Domain:      "security",
		DisplayName: "Benchmark Category",
		Description: "A category for benchmarking",
	}

	b.ResetTimer()
	for range b.N {
		registry = NewCategoryRegistry() // Reset for each iteration
		_ = registry.RegisterCategory(info)
	}
}

func BenchmarkGetInfo(b *testing.B) {
	registry := DefaultRegistry()

	b.ResetTimer()
	for range b.N {
		_, _ = registry.GetInfo("jailbreak")
	}
}

func BenchmarkValidate(b *testing.B) {
	registry := DefaultRegistry()

	b.ResetTimer()
	for range b.N {
		_ = registry.Validate("jailbreak")
	}
}

func BenchmarkListByDomain(b *testing.B) {
	registry := DefaultRegistry()

	b.ResetTimer()
	for range b.N {
		_ = registry.ListByDomain("security")
	}
}

func BenchmarkConcurrentGetInfo(b *testing.B) {
	registry := DefaultRegistry()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = registry.GetInfo("jailbreak")
		}
	})
}
