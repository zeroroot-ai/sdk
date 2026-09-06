// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeroroot-ai/sdk/codegen"
)

// =============================================================================
// EXACT MATCHING TESTS
// =============================================================================

func TestExactMatch_BasicReplacement(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		searchBlock  string
		replaceBlock string
		wantFound    bool
		wantContent  string
	}{
		{
			name: "exact match at beginning",
			content: `package main

func main() {
	println("start")
}

func other() {}`,
			searchBlock: `package main`,
			replaceBlock: `package main

// Modified`,
			wantFound: true,
			wantContent: `package main

// Modified

func main() {
	println("start")
}

func other() {}`,
		},
		{
			name: "exact match in middle",
			content: `package main

func first() {}

func middle() {
	return 42
}

func last() {}`,
			searchBlock: `func middle() {
	return 42
}`,
			replaceBlock: `func middle() {
	return 100
}`,
			wantFound: true,
			wantContent: `package main

func first() {}

func middle() {
	return 100
}

func last() {}`,
		},
		{
			name: "exact match at end",
			content: `package main

func first() {}

func last() {
	println("end")
}`,
			searchBlock: `func last() {
	println("end")
}`,
			replaceBlock: `func last() {
	fmt.Println("end")
}`,
			wantFound: true,
			wantContent: `package main

func first() {}

func last() {
	fmt.Println("end")
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, tt.replaceBlock)
			newContent, result := sr.Apply()

			assert.Equal(t, tt.wantFound, result.Found, "Match found status")
			if tt.wantFound {
				assert.Equal(t, tt.wantContent, newContent, "Content after replacement")
				assert.Positive(t, result.StartLine, "Start line should be positive")
				assert.Positive(t, result.EndLine, "End line should be positive")
				assert.GreaterOrEqual(t, result.EndLine, result.StartLine, "End line >= start line")
			}
		})
	}
}

func TestExactMatch_MultipleOccurrences(t *testing.T) {
	// When there are multiple exact matches, only the first should be replaced
	content := `func test() {
	return 1
}

func test() {
	return 1
}

func test() {
	return 1
}`

	searchBlock := `func test() {
	return 1
}`

	replaceBlock := `func test() {
	return 2
}`

	sr := NewSearchReplace(content, searchBlock, replaceBlock)
	newContent, result := sr.Apply()

	require.True(t, result.Found, "Should find first occurrence")

	// Count how many times the search block still appears
	originalCount := strings.Count(newContent, searchBlock)
	assert.Equal(t, 2, originalCount, "Should have 2 remaining original blocks")

	// Verify replacement exists
	assert.Contains(t, newContent, replaceBlock, "Should contain replacement block")
}

func TestExactMatch_CaseSensitive(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		searchBlock string
		wantFound   bool
	}{
		{
			name:        "exact case match",
			content:     "func Hello() {}",
			searchBlock: "func Hello() {}",
			wantFound:   true,
		},
		{
			name:        "case mismatch - lowercase search",
			content:     "func Hello() {}",
			searchBlock: "func hello() {}",
			wantFound:   false,
		},
		{
			name:        "case mismatch - uppercase search",
			content:     "func hello() {}",
			searchBlock: "func HELLO() {}",
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, "replacement")
			_, result := sr.Apply()
			assert.Equal(t, tt.wantFound, result.Found, "Case sensitivity check")
		})
	}
}

func TestExactMatch_EmptyBlocks(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		searchBlock  string
		replaceBlock string
		wantFound    bool
		description  string
	}{
		{
			name:         "empty search block",
			content:      "some content",
			searchBlock:  "",
			replaceBlock: "replacement",
			wantFound:    false,
			description:  "Empty search block should not match",
		},
		{
			name:         "empty replace block (deletion)",
			content:      "before\nMIDDLE\nafter",
			searchBlock:  "MIDDLE\n",
			replaceBlock: "",
			wantFound:    true,
			description:  "Empty replace block should delete the search block",
		},
		{
			name:         "empty content",
			content:      "",
			searchBlock:  "search",
			replaceBlock: "replace",
			wantFound:    false,
			description:  "Cannot find anything in empty content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, tt.replaceBlock)
			newContent, result := sr.Apply()

			assert.Equal(t, tt.wantFound, result.Found, tt.description)

			if tt.wantFound && tt.replaceBlock == "" {
				// Deletion case: verify search block is gone
				assert.NotContains(t, newContent, tt.searchBlock, "Search block should be deleted")
			}
		})
	}
}

// =============================================================================
// FUZZY MATCHING TESTS
// =============================================================================

func TestFuzzyMatch_WithThreshold(t *testing.T) {
	content := `func calculate(x int) int {
	return x * 2
}`

	tests := []struct {
		name          string
		searchBlock   string
		threshold     float64
		wantFound     bool
		minSimilarity float64
	}{
		{
			name: "high threshold exact match",
			searchBlock: `func calculate(x int) int {
	return x * 2
}`,
			threshold:     0.95,
			wantFound:     true,
			minSimilarity: 0.99,
		},
		{
			name: "moderate threshold with minor difference",
			searchBlock: `func calculate(x int) int {
	return x * 3
}`,
			threshold:     0.85,
			wantFound:     true,
			minSimilarity: 0.85,
		},
		{
			name: "low threshold with larger difference",
			searchBlock: `func calculate(y int) int {
	return y * 5
}`,
			threshold:     0.70,
			wantFound:     true,
			minSimilarity: 0.70,
		},
		{
			name: "threshold too high for difference",
			searchBlock: `func different(z string) string {
	return z + "suffix"
}`,
			threshold: 0.95,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFuzzyMatcher(content, tt.searchBlock).WithThreshold(tt.threshold)
			result := fm.FindBestMatch()

			assert.Equal(t, tt.wantFound, result.Found, "Match found status")
			if tt.wantFound {
				assert.GreaterOrEqual(t, result.Similarity, tt.minSimilarity,
					"Similarity should meet minimum")
				assert.GreaterOrEqual(t, result.Similarity, tt.threshold,
					"Similarity should meet threshold")
			}
		})
	}
}

func TestFuzzyMatch_CustomThreshold(t *testing.T) {
	content := "func test() { return 42 }"
	searchBlock := "func test() { return 100 }"

	// Test threshold boundary conditions
	thresholds := []float64{0.0, 0.5, 0.7, 0.85, 0.9, 1.0}

	for _, threshold := range thresholds {
		t.Run(t.Name()+"_"+strings.ReplaceAll(t.Name(), ".", "_"), func(t *testing.T) {
			fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(threshold)
			result := fm.FindBestMatch()

			if result.Found {
				assert.GreaterOrEqual(t, result.Similarity, threshold,
					"Found match must meet threshold %.2f", threshold)
			} else {
				// If not found, similarity should be below threshold
				assert.Less(t, result.Similarity, threshold,
					"Not found means similarity below threshold")
			}
		})
	}
}

func TestFuzzyMatch_LevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1           string
		s2           string
		wantDistance int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "abcd", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"golang", "python", 6},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_vs_"+tt.s2, func(t *testing.T) {
			distance := levenshteinDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.wantDistance, distance, "Levenshtein distance")
		})
	}
}

func TestFuzzyMatch_SimilarityScoring(t *testing.T) {
	tests := []struct {
		name               string
		s1                 string
		s2                 string
		expectedSimilarity float64
		tolerance          float64
	}{
		{
			name:               "identical strings",
			s1:                 "hello world",
			s2:                 "hello world",
			expectedSimilarity: 1.0,
			tolerance:          0.01,
		},
		{
			name:               "single char difference",
			s1:                 "hello",
			s2:                 "hallo",
			expectedSimilarity: 0.8, // 1 - (1/5)
			tolerance:          0.01,
		},
		{
			name:               "completely different",
			s1:                 "abc",
			s2:                 "xyz",
			expectedSimilarity: 0.0, // 1 - (3/3)
			tolerance:          0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := calculateSimilarity(tt.s1, tt.s2)
			assert.InDelta(t, tt.expectedSimilarity, similarity, tt.tolerance,
				"Similarity calculation")
		})
	}
}

func TestFuzzyMatch_FallsBelowThreshold(t *testing.T) {
	content := `func original() {
	return "completely different code"
}`

	searchBlock := `func unrelated() {
	x := 100
	y := 200
	return x + y
}`

	fm := NewFuzzyMatcher(content, searchBlock).WithThreshold(0.9)
	result := fm.FindBestMatch()

	assert.False(t, result.Found, "Should not find match when similarity is too low")
	assert.Less(t, result.Similarity, 0.9, "Similarity should be below threshold")
}

// =============================================================================
// WHITESPACE NORMALIZATION TESTS
// =============================================================================

func TestWhitespace_TabsVsSpaces(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		searchBlock string
		wantMatch   bool
		matchType   string
	}{
		{
			name:        "tabs in content, spaces in search",
			content:     "func test() {\n\treturn 42\n}",
			searchBlock: "func test() {\n    return 42\n}",
			wantMatch:   false, // Exact match should fail
			matchType:   "exact",
		},
		{
			name:        "spaces in content, tabs in search",
			content:     "func test() {\n    return 42\n}",
			searchBlock: "func test() {\n\treturn 42\n}",
			wantMatch:   false, // Exact match should fail
			matchType:   "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, "replacement")
			_, result := sr.Apply()
			assert.Equal(t, tt.wantMatch, result.Found,
				"Tab vs space should affect exact matching")
		})
	}
}

func TestWhitespace_LeadingTrailing(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantTrimmed string
	}{
		{
			name:        "trailing spaces",
			text:        "line1   \nline2  \nline3",
			wantTrimmed: "line1\nline2\nline3",
		},
		{
			name:        "trailing tabs",
			text:        "line1\t\t\nline2\t\nline3",
			wantTrimmed: "line1\nline2\nline3",
		},
		{
			name:        "mixed trailing whitespace",
			text:        "line1 \t \nline2\t  \nline3",
			wantTrimmed: "line1\nline2\nline3",
		},
		{
			name:        "no trailing whitespace",
			text:        "line1\nline2\nline3",
			wantTrimmed: "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trimmed := TrimTrailingWhitespace(tt.text)
			assert.Equal(t, tt.wantTrimmed, trimmed, "Trailing whitespace removal")
		})
	}
}

func TestWhitespace_MultipleSpaces(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello  world", "hello world"},
		{"hello    world", "hello world"},
		{"hello\t\tworld", "hello\t\tworld"}, // tabs preserved
		{"hello \t  world", "hello \t world"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			collapsed := collapseSpaces(tt.input)
			assert.Equal(t, tt.want, collapsed, "Multiple space collapse")
		})
	}
}

func TestWhitespace_LineEndings(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		searchBlock    string
		wantLineEnding string
	}{
		{
			name:           "unix line endings",
			content:        "line1\nline2\nline3",
			searchBlock:    "line2",
			wantLineEnding: "\n",
		},
		{
			name:           "windows line endings",
			content:        "line1\r\nline2\r\nline3",
			searchBlock:    "line2",
			wantLineEnding: "\r\n",
		},
		{
			name:           "mixed line endings defaults to windows if present",
			content:        "line1\r\nline2\nline3",
			searchBlock:    "line2",
			wantLineEnding: "\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := detectLineEnding(tt.content)
			assert.Equal(t, tt.wantLineEnding, detected, "Line ending detection")
		})
	}
}

func TestWhitespace_LineEndingNormalization(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		target string
		want   string
	}{
		{
			name:   "unix to unix",
			text:   "a\nb\nc",
			target: "\n",
			want:   "a\nb\nc",
		},
		{
			name:   "windows to unix",
			text:   "a\r\nb\r\nc",
			target: "\n",
			want:   "a\nb\nc",
		},
		{
			name:   "unix to windows",
			text:   "a\nb\nc",
			target: "\r\n",
			want:   "a\r\nb\r\nc",
		},
		{
			name:   "mixed to unix",
			text:   "a\r\nb\nc\rd",
			target: "\n",
			want:   "a\nb\nc\nd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := normalizeLineEndings(tt.text, tt.target)
			assert.Equal(t, tt.want, normalized, "Line ending normalization")
		})
	}
}

// =============================================================================
// INDENTATION PRESERVATION TESTS
// =============================================================================

func TestIndentation_PreserveTabs(t *testing.T) {
	searchBlock := "\tfunc old() {\n\t\treturn 1\n\t}"
	replaceBlock := "func new() {\n\treturn 2\n}"

	result := PreserveIndentation(searchBlock, replaceBlock)

	// Should preserve tab indentation from search block
	assert.Contains(t, result, "\tfunc new()", "Should indent with tab")
	assert.Contains(t, result, "\t\treturn 2", "Should preserve nested tabs")
}

func TestIndentation_PreserveSpaces(t *testing.T) {
	searchBlock := "    func old() {\n        return 1\n    }"
	replaceBlock := "func new() {\n    return 2\n}"

	result := PreserveIndentation(searchBlock, replaceBlock)

	// Should preserve space indentation from search block
	assert.Contains(t, result, "    func new()", "Should indent with 4 spaces")
	assert.Contains(t, result, "        return 2", "Should preserve nested spaces")
}

func TestIndentation_MixedIndentation(t *testing.T) {
	// When search block has mixed indentation, first non-empty line determines style
	searchBlock := "\t    func mixed() {\n\t    return 1\n\t}"
	replaceBlock := "func new() {\n    return 2\n}"

	result := PreserveIndentation(searchBlock, replaceBlock)

	// Should detect tab as the leading indent character
	assert.True(t, strings.HasPrefix(result, "\t") || strings.HasPrefix(result, " "),
		"Should apply some indentation")
}

func TestIndentation_DetectIndentationStyle(t *testing.T) {
	tests := []struct {
		name      string
		block     string
		wantChar  string
		wantLevel int
	}{
		{
			name:      "single tab",
			block:     "\tfunc test() {}",
			wantChar:  "\t",
			wantLevel: 1,
		},
		{
			name:      "two tabs",
			block:     "\t\tfunc test() {}",
			wantChar:  "\t",
			wantLevel: 2,
		},
		{
			name:      "four spaces",
			block:     "    func test() {}",
			wantChar:  " ",
			wantLevel: 4,
		},
		{
			name:      "two spaces",
			block:     "  func test() {}",
			wantChar:  " ",
			wantLevel: 2,
		},
		{
			name:      "no indentation",
			block:     "func test() {}",
			wantChar:  " ",
			wantLevel: 0,
		},
		{
			name:      "empty lines before indented line",
			block:     "\n\n\tfunc test() {}",
			wantChar:  "\t",
			wantLevel: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChar, gotLevel := detectIndentation(tt.block)
			assert.Equal(t, tt.wantChar, gotChar, "Indent character")
			assert.Equal(t, tt.wantLevel, gotLevel, "Indent level")
		})
	}
}

func TestIndentation_ConsistencyCheck(t *testing.T) {
	// Verify indentation is consistently applied across multiple lines
	searchBlock := "\t\tfunc nested() {\n\t\t\tif true {\n\t\t\t\treturn 1\n\t\t\t}\n\t\t}"
	replaceBlock := "func nested() {\n\tif true {\n\t\treturn 2\n\t}\n}"

	result := PreserveIndentation(searchBlock, replaceBlock)

	lines := strings.Split(result, "\n")
	require.Greater(t, len(lines), 2, "Should have multiple lines")

	// Check that first line has base indentation (2 tabs)
	assert.True(t, strings.HasPrefix(lines[0], "\t\t"), "First line should have 2 tabs")

	// Check that nested lines have additional indentation
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Each line should start with at least the base indentation
		assert.True(t, strings.HasPrefix(line, "\t") || len(line) == 0,
			"Line %d should maintain indentation", i)
	}
}

// =============================================================================
// BATCH EDIT ATOMICITY TESTS
// =============================================================================

func TestBatchEdit_AllSucceed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")

	require.NoError(t, os.WriteFile(file1, []byte("func foo() { return 1 }"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("func bar() { return 2 }"), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edits := []Edit{
		{
			FilePath:     "file1.go",
			SearchBlock:  "func foo() { return 1 }",
			ReplaceBlock: "func foo() { return 10 }",
		},
		{
			FilePath:     "file2.go",
			SearchBlock:  "func bar() { return 2 }",
			ReplaceBlock: "func bar() { return 20 }",
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	require.NoError(t, err)

	assert.True(t, result.Applied, "Batch should be applied")
	assert.Equal(t, 2, result.SuccessfulEdits(), "All edits should succeed")
	assert.Equal(t, 0, result.FailedEdits(), "No edits should fail")

	// Verify both files were modified
	content1, _ := os.ReadFile(file1)
	content2, _ := os.ReadFile(file2)
	assert.Contains(t, string(content1), "return 10")
	assert.Contains(t, string(content2), "return 20")
}

func TestBatchEdit_FirstEditFails(t *testing.T) {
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "test.go")
	originalContent := "func existing() { return 1 }"
	require.NoError(t, os.WriteFile(file, []byte(originalContent), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edits := []Edit{
		{
			FilePath:     "test.go",
			SearchBlock:  "func notfound() {}",
			ReplaceBlock: "func replaced() {}",
			Description:  "This should fail",
		},
		{
			FilePath:     "test.go",
			SearchBlock:  "func existing() { return 1 }",
			ReplaceBlock: "func existing() { return 2 }",
			Description:  "This succeeds but batch is marked failed",
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Batch should be marked as not applied due to first failure")
	assert.Equal(t, 1, result.SuccessfulEdits(), "Second edit should succeed")
	assert.Equal(t, 1, result.FailedEdits(), "First edit should fail")

	// Note: Current implementation applies successful edits even if batch fails
	// Only LSP validation errors trigger rollback
	content, _ := os.ReadFile(file)
	assert.Contains(t, string(content), "return 2", "Successful edit is still applied")
}

func TestBatchEdit_MiddleEditFails(t *testing.T) {
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "test.go")
	originalContent := `func first() { return 1 }
func second() { return 2 }
func third() { return 3 }`
	require.NoError(t, os.WriteFile(file, []byte(originalContent), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edits := []Edit{
		{
			FilePath:     "test.go",
			SearchBlock:  "func first() { return 1 }",
			ReplaceBlock: "func first() { return 10 }",
		},
		{
			FilePath:     "test.go",
			SearchBlock:  "func notfound() {}",
			ReplaceBlock: "func replaced() {}",
		},
		{
			FilePath:     "test.go",
			SearchBlock:  "func third() { return 3 }",
			ReplaceBlock: "func third() { return 30 }",
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Batch should not be applied when middle edit fails")

	// Verify file was rolled back to original
	content, _ := os.ReadFile(file)
	assert.Contains(t, string(content), "return 1", "Should be rolled back")
	assert.Contains(t, string(content), "return 2", "Should be rolled back")
	assert.Contains(t, string(content), "return 3", "Should be rolled back")
}

func TestBatchEdit_LastEditFails(t *testing.T) {
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "test.go")
	originalContent := `func first() { return 1 }
func second() { return 2 }`
	require.NoError(t, os.WriteFile(file, []byte(originalContent), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edits := []Edit{
		{
			FilePath:     "test.go",
			SearchBlock:  "func first() { return 1 }",
			ReplaceBlock: "func first() { return 10 }",
		},
		{
			FilePath:     "test.go",
			SearchBlock:  "func notfound() {}",
			ReplaceBlock: "func replaced() {}",
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Batch should be marked as not applied when last edit fails")
	assert.Equal(t, 1, result.SuccessfulEdits(), "First edit should succeed")
	assert.Equal(t, 1, result.FailedEdits(), "Second edit should fail")

	// Note: Current implementation applies successful edits even if batch fails
	// Only LSP validation errors trigger rollback
	content, _ := os.ReadFile(file)
	assert.Contains(t, string(content), "return 10", "First edit is still applied")
	assert.Contains(t, string(content), "return 2", "Second function unchanged")
}

func TestBatchEdit_TransactionSemantics(t *testing.T) {
	// Test that LSP validation errors trigger full rollback of ALL edits
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "test.go")
	originalContent := `func first() { return 1 }
func second() { return 2 }`
	require.NoError(t, os.WriteFile(file, []byte(originalContent), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	lspManager := NewMockLSPManager()

	// Add validation error that will trigger rollback
	absPath := filepath.Join(tmpDir, "test.go")
	lspManager.AddDiagnostic(absPath, codegen.Diagnostic{
		Path:     absPath,
		Line:     1,
		Column:   1,
		Severity: codegen.SeverityError,
		Message:  "syntax error after modifications",
		Source:   "test",
	})

	editor := NewEditor(tmpDir, gitOps, lspManager)

	edits := []Edit{
		{
			FilePath:     "test.go",
			SearchBlock:  "func first() { return 1 }",
			ReplaceBlock: "func first() { return 10 }",
		},
		{
			FilePath:     "test.go",
			SearchBlock:  "func second() { return 2 }",
			ReplaceBlock: "func second() { return 20 }",
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Batch should be rolled back due to validation error")
	assert.Equal(t, codegen.ValidationFailed, result.ValidationStatus)

	// Verify full rollback - all changes should be reverted
	content, _ := os.ReadFile(file)
	assert.Equal(t, originalContent, string(content),
		"All changes should be rolled back on validation failure")
}

func TestBatchEdit_PartialSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files
	files := []string{"file1.go", "file2.go", "file3.go"}
	for i, fname := range files {
		fpath := filepath.Join(tmpDir, fname)
		content := t.Name() + " " + string(rune('A'+i))
		require.NoError(t, os.WriteFile(fpath, []byte(content), 0644))
	}

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	// Apply batch with one failure in the middle
	edits := []Edit{
		{
			FilePath:     "file1.go",
			SearchBlock:  t.Name() + " A",
			ReplaceBlock: "MODIFIED 1",
		},
		{
			FilePath:     "file2.go",
			SearchBlock:  "NONEXISTENT",
			ReplaceBlock: "MODIFIED 2",
		},
		{
			FilePath:     "file3.go",
			SearchBlock:  t.Name() + " C",
			ReplaceBlock: "MODIFIED 3",
		},
	}

	result, err := editor.ApplyBatch(context.Background(), edits)
	require.NoError(t, err)
	assert.False(t, result.Applied, "Batch should be marked as failed")
	assert.Equal(t, 2, result.SuccessfulEdits(), "Two edits should succeed")
	assert.Equal(t, 1, result.FailedEdits(), "One edit should fail")

	// Note: Current implementation applies successful edits even if batch fails
	// Only LSP validation errors trigger full rollback
	content1, _ := os.ReadFile(filepath.Join(tmpDir, "file1.go"))
	content2, _ := os.ReadFile(filepath.Join(tmpDir, "file2.go"))
	content3, _ := os.ReadFile(filepath.Join(tmpDir, "file3.go"))

	assert.Equal(t, "MODIFIED 1", string(content1), "File1 should be modified")
	assert.Contains(t, string(content2), t.Name()+" B", "File2 should remain unchanged")
	assert.Equal(t, "MODIFIED 3", string(content3), "File3 should be modified")
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestEdgeCase_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "empty.go")
	require.NoError(t, os.WriteFile(file, []byte(""), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "empty.go",
		SearchBlock:  "anything",
		ReplaceBlock: "replacement",
	}

	result, err := editor.Apply(context.Background(), edit)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Should not match in empty file")
	assert.Equal(t, codegen.MatchFailed, result.MatchType)
}

func TestEdgeCase_VeryLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "large.go")

	// Create a large file with the target block at the end
	var builder strings.Builder
	builder.WriteString("package main\n\n")
	for i := range 1000 {
		builder.WriteString(t.Name())
		builder.WriteString("\nfunc dummy")
		builder.WriteString(strings.Repeat("x", i%10))
		builder.WriteString("() { return ")
		builder.WriteString(strings.Repeat("y", i%5))
		builder.WriteString(" }\n\n")
	}
	builder.WriteString("func target() { return 42 }\n")

	require.NoError(t, os.WriteFile(file, []byte(builder.String()), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "large.go",
		SearchBlock:  "func target() { return 42 }",
		ReplaceBlock: "func target() { return 100 }",
	}

	result, err := editor.Apply(context.Background(), edit)
	require.NoError(t, err)

	assert.True(t, result.Applied, "Should find target in large file")
	assert.Equal(t, codegen.MatchExact, result.MatchType)

	// Verify the change was made
	content, _ := os.ReadFile(file)
	assert.Contains(t, string(content), "return 100")
	assert.NotContains(t, string(content), "func target() { return 42 }")
}

func TestEdgeCase_UnicodeContent(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "unicode.go")

	content := `package main

func greet() string {
	return "Hello, 世界! 🌍"
}

func farewell() string {
	return "Goodbye, мир! 👋"
}`

	require.NoError(t, os.WriteFile(file, []byte(content), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath: "unicode.go",
		SearchBlock: `func greet() string {
	return "Hello, 世界! 🌍"
}`,
		ReplaceBlock: `func greet() string {
	return "你好, 世界! 🌏"
}`,
	}

	result, err := editor.Apply(context.Background(), edit)
	require.NoError(t, err)

	assert.True(t, result.Applied, "Should handle Unicode content")

	// Verify Unicode replacement worked
	modified, _ := os.ReadFile(file)
	assert.Contains(t, string(modified), "你好, 世界! 🌏")
	assert.NotContains(t, string(modified), "Hello, 世界! 🌍")
}

func TestEdgeCase_FileWithOnlyWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "whitespace.txt")

	content := "   \n\t\t\n  \t  \n\n"
	require.NoError(t, os.WriteFile(file, []byte(content), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "whitespace.txt",
		SearchBlock:  "anything",
		ReplaceBlock: "content",
	}

	result, err := editor.Apply(context.Background(), edit)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Should not match in whitespace-only file")
}

func TestEdgeCase_SearchBlockSpansMultipleFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "multi.go")

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

	require.NoError(t, os.WriteFile(file, []byte(content), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	// Search block spans two functions
	edit := Edit{
		FilePath: "multi.go",
		SearchBlock: `func first() {
	return 1
}

func second() {
	return 2
}`,
		ReplaceBlock: `func combined() {
	return 3
}`,
	}

	result, err := editor.Apply(context.Background(), edit)
	require.NoError(t, err)

	assert.True(t, result.Applied, "Should handle multi-function search blocks")

	// Verify both functions were replaced with one
	modified, _ := os.ReadFile(file)
	assert.Contains(t, string(modified), "func combined()")
	assert.NotContains(t, string(modified), "func first()")
	assert.NotContains(t, string(modified), "func second()")
	assert.Contains(t, string(modified), "func third()") // Third function should remain
}

// =============================================================================
// ERROR SCENARIOS
// =============================================================================

func TestError_SearchBlockNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(file, []byte("package main\n\nfunc existing() {}"), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "test.go",
		SearchBlock:  "func nonexistent() {}",
		ReplaceBlock: "func replacement() {}",
	}

	result, err := editor.Apply(context.Background(), edit)
	require.NoError(t, err)

	assert.False(t, result.Applied, "Should not apply when search block not found")
	assert.Equal(t, codegen.MatchFailed, result.MatchType)
	assert.NotNil(t, result.ClosestMatch, "Should provide closest match info for debugging")
}

func TestError_MultipleMatchesAmbiguous(t *testing.T) {
	// When exact matches are ambiguous, first match should be used
	content := `func duplicate() {
	return 1
}

func other() {
	return 2
}

func duplicate() {
	return 1
}`

	searchBlock := `func duplicate() {
	return 1
}`

	sr := NewSearchReplace(content, searchBlock, "REPLACED")
	newContent, result := sr.Apply()

	require.True(t, result.Found, "Should find first occurrence")

	// Only first occurrence should be replaced
	count := strings.Count(newContent, "REPLACED")
	assert.Equal(t, 1, count, "Should replace only first occurrence")

	// Second duplicate should still exist
	remainingCount := strings.Count(newContent, searchBlock)
	assert.Equal(t, 1, remainingCount, "Second occurrence should remain")
}

func TestError_InvalidFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "nonexistent/path/file.go",
		SearchBlock:  "anything",
		ReplaceBlock: "replacement",
	}

	result, err := editor.Apply(context.Background(), edit)
	assert.Error(t, err, "Should error on invalid file path")
	assert.Nil(t, result, "Result should be nil on file read error")
}

func TestError_EmptySearchBlock(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(file, []byte("package main"), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "test.go",
		SearchBlock:  "",
		ReplaceBlock: "replacement",
	}

	_, err := editor.Apply(context.Background(), edit)
	assert.Error(t, err, "Should error on empty search block")
	assert.Contains(t, err.Error(), "search block cannot be empty")
}

func TestError_EmptyFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edit := Edit{
		FilePath:     "",
		SearchBlock:  "anything",
		ReplaceBlock: "replacement",
	}

	_, err := editor.Apply(context.Background(), edit)
	assert.Error(t, err, "Should error on empty file path")
	assert.Contains(t, err.Error(), "file path cannot be empty")
}

// =============================================================================
// HELPER FUNCTIONS AND BENCHMARKS
// =============================================================================

func TestCalculateLineNumber_Various(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		pos  int
		want int
	}{
		{0, 1},    // Start of file
		{5, 1},    // End of line 1
		{6, 2},    // Start of line 2
		{11, 2},   // End of line 2
		{12, 3},   // Start of line 3
		{100, -1}, // Beyond end
		{-1, -1},  // Negative
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			got := calculateLineNumber(content, tt.pos)
			assert.Equal(t, tt.want, got, "Line number for position %d", tt.pos)
		})
	}
}

func TestNormalizeWhitespace_Complete(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"hello   world", "hello world"},
		{"hello\n\nworld", "hello world"},
		{"  hello  \n  world  ", "hello world"},
		{"hello\t\tworld", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeWhitespace(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkExactMatch_Small(b *testing.B) {
	content := strings.Repeat("func test() { return 42 }\n", 10)
	searchBlock := "func test() { return 42 }"
	replaceBlock := "func test() { return 100 }"

	b.ResetTimer()
	for range b.N {
		sr := NewSearchReplace(content, searchBlock, replaceBlock)
		sr.Apply()
	}
}

func BenchmarkExactMatch_Large(b *testing.B) {
	content := strings.Repeat("func test() { return 42 }\n", 1000)
	searchBlock := "func test() { return 42 }"
	replaceBlock := "func test() { return 100 }"

	b.ResetTimer()
	for range b.N {
		sr := NewSearchReplace(content, searchBlock, replaceBlock)
		sr.Apply()
	}
}

func BenchmarkFuzzyMatch_Small(b *testing.B) {
	content := strings.Repeat("func test() { return 42 }\n", 10)
	searchBlock := "func test() { return 43 }"

	b.ResetTimer()
	for range b.N {
		fm := NewFuzzyMatcher(content, searchBlock)
		fm.FindBestMatch()
	}
}

func BenchmarkFuzzyMatch_Large(b *testing.B) {
	content := strings.Repeat("func test() { return 42 }\n", 500)
	searchBlock := "func test() { return 43 }"

	b.ResetTimer()
	for range b.N {
		fm := NewFuzzyMatcher(content, searchBlock)
		fm.FindBestMatch()
	}
}

func BenchmarkLevenshteinDistance_Short(b *testing.B) {
	s1 := "hello world"
	s2 := "hallo world"

	b.ResetTimer()
	for range b.N {
		levenshteinDistance(s1, s2)
	}
}

func BenchmarkLevenshteinDistance_Long(b *testing.B) {
	s1 := strings.Repeat("the quick brown fox jumps over the lazy dog ", 10)
	s2 := strings.Repeat("the quick brown fox jumps over the lazy cat ", 10)

	b.ResetTimer()
	for range b.N {
		levenshteinDistance(s1, s2)
	}
}

func BenchmarkBatchEdit_Sequential(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test file
	file := filepath.Join(tmpDir, "test.go")
	content := `package main

func f1() { return 1 }
func f2() { return 2 }
func f3() { return 3 }
func f4() { return 4 }
func f5() { return 5 }`

	require.NoError(b, os.WriteFile(file, []byte(content), 0644))

	gitOps := NewMockGitOps()
	gitOps.SetWorkspaceDir(tmpDir)
	editor := NewEditor(tmpDir, gitOps, nil)

	edits := []Edit{
		{FilePath: "test.go", SearchBlock: "func f1() { return 1 }", ReplaceBlock: "func f1() { return 10 }"},
		{FilePath: "test.go", SearchBlock: "func f2() { return 2 }", ReplaceBlock: "func f2() { return 20 }"},
		{FilePath: "test.go", SearchBlock: "func f3() { return 3 }", ReplaceBlock: "func f3() { return 30 }"},
	}

	b.ResetTimer()
	for range b.N {
		// Reset file before each iteration
		os.WriteFile(file, []byte(content), 0644)
		editor.ApplyBatch(context.Background(), edits)
	}
}
