// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"strings"
	"testing"
)

func TestFuzzyMatcher_ExactMatchShouldScore1(t *testing.T) {
	content := `func hello() {
	println("hello")
}`

	searchBlock := `func hello() {
	println("hello")
}`

	fm := NewFuzzyMatcher(content, searchBlock)
	result := fm.FindBestMatch()

	if !result.Found {
		t.Fatal("Expected to find match")
	}

	if result.Similarity < 0.99 {
		t.Errorf("Expected similarity near 1.0 for exact match, got %.2f", result.Similarity)
	}
}

func TestFuzzyMatcher_WhitespaceVariations(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		searchBlock   string
		threshold     float64
		wantFound     bool
		minSimilarity float64
	}{
		{
			name: "extra spaces in content",
			content: `func hello() {
	println("hello")
}`,
			searchBlock: `func hello() {
println("hello")
}`,
			threshold:     0.85,
			wantFound:     true,
			minSimilarity: 0.85,
		},
		{
			name: "trailing whitespace differences",
			content: `func test() {
	return 42
}`,
			searchBlock: `func test() {
	return 42
}`,
			threshold:     0.9,
			wantFound:     true,
			minSimilarity: 0.9,
		},
		{
			name: "tab vs space differences",
			content: `func test() {
    return 42
}`,
			searchBlock: `func test() {
	return 42
}`,
			threshold:     0.8,
			wantFound:     true,
			minSimilarity: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFuzzyMatcher(tt.content, tt.searchBlock).WithThreshold(tt.threshold)
			result := fm.FindBestMatch()

			if result.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", result.Found, tt.wantFound)
			}

			if result.Found && result.Similarity < tt.minSimilarity {
				t.Errorf("Similarity = %.2f, want >= %.2f", result.Similarity, tt.minSimilarity)
			}
		})
	}
}

func TestFuzzyMatcher_MinorEdits(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		searchBlock   string
		threshold     float64
		wantFound     bool
		minSimilarity float64
	}{
		{
			name: "single character difference",
			content: `func calculate(x int) int {
	return x * 2
}`,
			searchBlock: `func calculate(x int) int {
	return x * 3
}`,
			threshold:     0.9,
			wantFound:     true,
			minSimilarity: 0.9,
		},
		{
			name: "variable name difference",
			content: `func process(data string) {
	fmt.Println(data)
}`,
			searchBlock: `func process(text string) {
	fmt.Println(text)
}`,
			threshold:     0.80,
			wantFound:     true,
			minSimilarity: 0.80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFuzzyMatcher(tt.content, tt.searchBlock).WithThreshold(tt.threshold)
			result := fm.FindBestMatch()

			if result.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v (similarity: %.2f)", result.Found, tt.wantFound, result.Similarity)
			}

			if result.Similarity < tt.minSimilarity {
				t.Errorf("Similarity = %.2f, want >= %.2f", result.Similarity, tt.minSimilarity)
			}
		})
	}
}

func TestFuzzyMatcher_MultipleBlocks(t *testing.T) {
	content := `package main

func first() {
	return 1
}

func second() {
	return 2
}

func third() {
	return 3
}`

	searchBlock := `func second() {
	return 2
}`

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.9)
	result := fm.FindBestMatch()

	if !result.Found {
		t.Fatal("Expected to find match")
	}

	// Should find the second function
	if result.StartLine < 6 || result.StartLine > 8 {
		t.Errorf("StartLine = %d, expected around line 7", result.StartLine)
	}
}

func TestFuzzyMatcher_ThresholdSettings(t *testing.T) {
	content := `func test() {
	return 42
}`

	searchBlock := `func test() {
	return 100
}`

	tests := []struct {
		name      string
		threshold float64
		wantFound bool
	}{
		{"threshold 0.5 - should match", 0.5, true},
		{"threshold 0.7 - should match", 0.7, true},
		{"threshold 0.95 - might not match", 0.95, false},
		{"threshold 1.0 - exact only", 1.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(tt.threshold)
			result := fm.FindBestMatch()

			if result.Found != tt.wantFound {
				t.Errorf("With threshold %.2f: Found = %v, want %v (similarity: %.2f)",
					tt.threshold, result.Found, tt.wantFound, result.Similarity)
			}
		})
	}
}

func TestFuzzyMatcher_CaseInsensitive(t *testing.T) {
	content := `func Hello() {
	Println("HELLO")
}`

	searchBlock := `func hello() {
	println("hello")
}`

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.9)
	result := fm.FindBestMatch()

	if !result.Found {
		t.Errorf("Expected case-insensitive match, got similarity %.2f", result.Similarity)
	}
}

func TestFuzzyMatcher_LineNumbersCorrect(t *testing.T) {
	content := `line 1
line 2
func test() {
	return 42
}
line 6
line 7`

	searchBlock := `func test() {
	return 42
}`

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.9)
	result := fm.FindBestMatch()

	if !result.Found {
		t.Fatal("Expected to find match")
	}

	if result.StartLine != 3 {
		t.Errorf("StartLine = %d, want 3", result.StartLine)
	}

	if result.EndLine != 5 {
		t.Errorf("EndLine = %d, want 5", result.EndLine)
	}
}

func TestFuzzyMatcher_FindAllMatches(t *testing.T) {
	content := `func test1() {
	return 1
}

func test2() {
	return 2
}

func test3() {
	return 3
}`

	searchBlock := `func testX() {
	return X
}`

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.7)
	matches := fm.FindAllMatches()

	if len(matches) < 3 {
		t.Errorf("Expected at least 3 matches, got %d", len(matches))
	}

	for i, match := range matches {
		if !match.Found {
			t.Errorf("Match %d: Found = false", i)
		}
		if match.Similarity < 0.7 {
			t.Errorf("Match %d: Similarity %.2f below threshold", i, match.Similarity)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name string
		s1   string
		s2   string
		want int
	}{
		{"identical strings", "hello", "hello", 0},
		{"empty strings", "", "", 0},
		{"one empty", "hello", "", 5},
		{"single insertion", "hello", "helo", 1},
		{"single deletion", "helo", "hello", 1},
		{"single substitution", "hello", "hallo", 1},
		{"multiple changes", "kitten", "sitting", 3},
		{"completely different", "abc", "xyz", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := levenshteinDistance(tt.s1, tt.s2)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
			}
		})
	}
}

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		name          string
		s1            string
		s2            string
		minSimilarity float64
		maxSimilarity float64
	}{
		{"identical", "hello", "hello", 1.0, 1.0},
		{"very similar", "hello", "hallo", 0.8, 0.9},
		{"somewhat similar", "hello", "help", 0.5, 0.7},
		{"very different", "abc", "xyz", 0.0, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSimilarity(tt.s1, tt.s2)
			if got < tt.minSimilarity || got > tt.maxSimilarity {
				t.Errorf("calculateSimilarity(%q, %q) = %.2f, want between %.2f and %.2f",
					tt.s1, tt.s2, got, tt.minSimilarity, tt.maxSimilarity)
			}
		})
	}
}

func TestFuzzyMatcher_NormalizeForFuzzy(t *testing.T) {
	fm := NewFuzzyMatcher("", "")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase conversion",
			input: "HELLO World",
			want:  "hello world",
		},
		{
			name:  "trailing whitespace removal",
			input: "line1   \nline2  ",
			want:  "line1\nline2",
		},
		{
			name:  "collapse multiple spaces",
			input: "hello    world",
			want:  "hello world",
		},
		{
			name:  "mixed normalizations",
			input: "  HELLO    WORLD  \n  FOO   BAR  ",
			want:  " hello world\n foo bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fm.normalizeForFuzzy(tt.input)
			if got != tt.want {
				t.Errorf("normalizeForFuzzy() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollapseSpaces(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single space", "hello world", "hello world"},
		{"multiple spaces", "hello    world", "hello world"},
		{"tabs preserved", "hello\t\tworld", "hello\t\tworld"},
		{"mixed tabs and spaces", "hello \t  world", "hello \t world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapseSpaces(tt.input)
			if got != tt.want {
				t.Errorf("collapseSpaces() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "collapse all whitespace",
			input: "hello   \n\t  world",
			want:  "hello world",
		},
		{
			name:  "trim edges",
			input: "  hello world  ",
			want:  "hello world",
		},
		{
			name:  "multiple types",
			input: "hello\t\nworld\r\n\tfoo",
			want:  "hello world foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeWhitespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFuzzyMatcher_WindowSizeVariations(t *testing.T) {
	// Test that fuzzy matcher can handle blocks with slightly different line counts
	content := `func test() {
	x := 1
	y := 2
	return x + y
}`

	// Search block has extra blank line
	searchBlock := `func test() {

	x := 1
	y := 2
	return x + y
}`

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.7)
	result := fm.FindBestMatch()

	if !result.Found {
		t.Errorf("Expected to find match with different line counts, similarity: %.2f", result.Similarity)
	}
}

func TestFuzzyMatcher_EmptyContent(t *testing.T) {
	fm := NewFuzzyMatcher("", "search block")
	result := fm.FindBestMatch()

	if result.Found {
		t.Error("Should not find match in empty content")
	}
}

func TestFuzzyMatcher_EmptySearchBlock(t *testing.T) {
	fm := NewFuzzyMatcher("some content", "")
	result := fm.FindBestMatch()

	// Empty search block behavior depends on implementation
	// Currently it may or may not match - test just verifies no panic
	_ = result
}

func TestFuzzyMatcher_VeryLongContent(t *testing.T) {
	// Generate large content
	var builder strings.Builder
	for range 1000 {
		builder.WriteString("func test() {\n\treturn 42\n}\n\n")
	}
	// Add target block at the end
	builder.WriteString("func target() {\n\treturn 100\n}\n")

	content := builder.String()
	searchBlock := "func target() {\n\treturn 100\n}"

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.9)
	result := fm.FindBestMatch()

	if !result.Found {
		t.Error("Should find target block in large content")
	}

	if !strings.Contains(result.MatchedContent, "target") {
		t.Error("Matched content should contain the target function")
	}
}

// Benchmark tests
func BenchmarkFuzzyMatcher_FindBestMatch(b *testing.B) {
	content := strings.Repeat("func test() {\n\treturn 42\n}\n\n", 100)
	searchBlock := "func test() {\n\treturn 43\n}"

	b.ResetTimer()
	for range b.N {
		fm := NewFuzzyMatcher(content, searchBlock)
		fm.FindBestMatch()
	}
}

func BenchmarkLevenshteinDistance(b *testing.B) {
	s1 := strings.Repeat("hello world ", 50)
	s2 := strings.Repeat("hello world!", 50)

	b.ResetTimer()
	for range b.N {
		levenshteinDistance(s1, s2)
	}
}

func BenchmarkFuzzyMatcher_LargeFile(b *testing.B) {
	// Simulate searching in a large file
	var builder strings.Builder
	for range 500 {
		builder.WriteString("func test() {\n\treturn 42\n}\n\n")
	}
	content := builder.String()
	searchBlock := "func test() {\n\treturn 43\n}"

	b.ResetTimer()
	for range b.N {
		fm := NewFuzzyMatcher(content, searchBlock)
		fm.FindBestMatch()
	}
}
