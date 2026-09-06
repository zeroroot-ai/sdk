// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"strings"
	"testing"
)

func TestSearchReplace_ExactMatch(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		searchBlock  string
		replaceBlock string
		wantFound    bool
		wantContent  string
	}{
		{
			name: "simple exact match",
			content: `package main

func hello() {
	println("hello")
}`,
			searchBlock: `func hello() {
	println("hello")
}`,
			replaceBlock: `func hello() {
	fmt.Println("hello world")
}`,
			wantFound: true,
			wantContent: `package main

func hello() {
	fmt.Println("hello world")
}`,
		},
		{
			name:         "match with tabs",
			content:      "func test() {\n\treturn 42\n}",
			searchBlock:  "func test() {\n\treturn 42\n}",
			replaceBlock: "func test() {\n\treturn 100\n}",
			wantFound:    true,
			wantContent:  "func test() {\n\treturn 100\n}",
		},
		{
			name: "no match - different content",
			content: `func hello() {
	println("hello")
}`,
			searchBlock: `func goodbye() {
	println("bye")
}`,
			replaceBlock: `func goodbye() {
	fmt.Println("bye")
}`,
			wantFound: false,
			wantContent: `func hello() {
	println("hello")
}`,
		},
		{
			name: "match middle of file",
			content: `package main

func first() {}

func second() {
	return 2
}

func third() {}`,
			searchBlock: `func second() {
	return 2
}`,
			replaceBlock: `func second() {
	return 42
}`,
			wantFound: true,
			wantContent: `package main

func first() {}

func second() {
	return 42
}

func third() {}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, tt.replaceBlock)
			newContent, result := sr.Apply()

			if result.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", result.Found, tt.wantFound)
			}

			if newContent != tt.wantContent {
				t.Errorf("Content mismatch.\nGot:\n%s\n\nWant:\n%s", newContent, tt.wantContent)
			}

			if result.Found {
				if result.StartLine <= 0 {
					t.Errorf("StartLine = %d, want > 0", result.StartLine)
				}
				if result.EndLine <= 0 {
					t.Errorf("EndLine = %d, want > 0", result.EndLine)
				}
				if result.StartLine > result.EndLine {
					t.Errorf("StartLine (%d) > EndLine (%d)", result.StartLine, result.EndLine)
				}
			}
		})
	}
}

func TestSearchReplace_LineEndings(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		searchBlock  string
		replaceBlock string
		wantFound    bool
	}{
		{
			name:         "unix line endings in both",
			content:      "line1\nline2\nline3",
			searchBlock:  "line2",
			replaceBlock: "REPLACED",
			wantFound:    true,
		},
		{
			name:         "windows line endings in content",
			content:      "line1\r\nline2\r\nline3",
			searchBlock:  "line2",
			replaceBlock: "REPLACED",
			wantFound:    true,
		},
		{
			name:         "windows in search, unix in content",
			content:      "line1\nline2\nline3",
			searchBlock:  "line2",
			replaceBlock: "REPLACED",
			wantFound:    true,
		},
		{
			name:         "mixed line endings - unix search, windows content",
			content:      "func test() {\r\n\treturn 42\r\n}",
			searchBlock:  "func test() {\n\treturn 42\n}",
			replaceBlock: "func test() {\n\treturn 100\n}",
			wantFound:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, tt.replaceBlock)
			_, result := sr.Apply()

			if result.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", result.Found, tt.wantFound)
			}
		})
	}
}

func TestDetectLineEnding(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "unix line endings",
			content: "line1\nline2\nline3",
			want:    "\n",
		},
		{
			name:    "windows line endings",
			content: "line1\r\nline2\r\nline3",
			want:    "\r\n",
		},
		{
			name:    "mixed - windows first",
			content: "line1\r\nline2\nline3",
			want:    "\r\n",
		},
		{
			name:    "no line endings",
			content: "single line",
			want:    "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectLineEnding(tt.content)
			if got != tt.want {
				t.Errorf("detectLineEnding() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		target string
		want   string
	}{
		{
			name:   "unix to unix",
			text:   "line1\nline2\nline3",
			target: "\n",
			want:   "line1\nline2\nline3",
		},
		{
			name:   "windows to unix",
			text:   "line1\r\nline2\r\nline3",
			target: "\n",
			want:   "line1\nline2\nline3",
		},
		{
			name:   "unix to windows",
			text:   "line1\nline2\nline3",
			target: "\r\n",
			want:   "line1\r\nline2\r\nline3",
		},
		{
			name:   "mixed to unix",
			text:   "line1\r\nline2\nline3\rline4",
			target: "\n",
			want:   "line1\nline2\nline3\nline4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLineEndings(tt.text, tt.target)
			if got != tt.want {
				t.Errorf("normalizeLineEndings() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateLineNumber(t *testing.T) {
	content := "line1\nline2\nline3\nline4"

	tests := []struct {
		name string
		pos  int
		want int
	}{
		{"start of file", 0, 1},
		{"start of line 2", 6, 2},
		{"start of line 3", 12, 3},
		{"middle of line 2", 8, 2},
		{"invalid negative", -1, -1},
		{"invalid too large", 1000, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLineNumber(content, tt.pos)
			if got != tt.want {
				t.Errorf("calculateLineNumber(%d) = %d, want %d", tt.pos, got, tt.want)
			}
		})
	}
}

func TestPreserveIndentation(t *testing.T) {
	tests := []struct {
		name         string
		searchBlock  string
		replaceBlock string
		want         string
	}{
		{
			name:         "preserve tab indentation",
			searchBlock:  "\tfunc old() {\n\t\treturn 1\n\t}",
			replaceBlock: "func new() {\n\treturn 2\n}",
			want:         "\tfunc new() {\n\t\treturn 2\n\t}",
		},
		{
			name:         "preserve space indentation",
			searchBlock:  "    func old() {\n        return 1\n    }",
			replaceBlock: "func new() {\n    return 2\n}",
			want:         "    func new() {\n        return 2\n    }",
		},
		{
			name:         "no indentation",
			searchBlock:  "func old() {\nreturn 1\n}",
			replaceBlock: "func new() {\nreturn 2\n}",
			want:         "func new() {\nreturn 2\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PreserveIndentation(tt.searchBlock, tt.replaceBlock)
			if got != tt.want {
				t.Errorf("PreserveIndentation() mismatch.\nGot:\n%q\n\nWant:\n%q", got, tt.want)
			}
		})
	}
}

func TestDetectIndentation(t *testing.T) {
	tests := []struct {
		name      string
		block     string
		wantChar  string
		wantLevel int
	}{
		{
			name:      "tabs",
			block:     "\tfunc test() {\n\t\treturn 1\n\t}",
			wantChar:  "\t",
			wantLevel: 1,
		},
		{
			name:      "four spaces",
			block:     "    func test() {\n        return 1\n    }",
			wantChar:  " ",
			wantLevel: 4,
		},
		{
			name:      "two spaces",
			block:     "  func test() {\n    return 1\n  }",
			wantChar:  " ",
			wantLevel: 2,
		},
		{
			name:      "no indentation",
			block:     "func test() {\nreturn 1\n}",
			wantChar:  " ",
			wantLevel: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChar, gotLevel := detectIndentation(tt.block)
			if gotChar != tt.wantChar {
				t.Errorf("detectIndentation() char = %q, want %q", gotChar, tt.wantChar)
			}
			if gotLevel != tt.wantLevel {
				t.Errorf("detectIndentation() level = %d, want %d", gotLevel, tt.wantLevel)
			}
		})
	}
}

func TestTrimTrailingWhitespace(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "trailing spaces",
			text: "line1   \nline2  \nline3",
			want: "line1\nline2\nline3",
		},
		{
			name: "trailing tabs",
			text: "line1\t\t\nline2\t\nline3",
			want: "line1\nline2\nline3",
		},
		{
			name: "mixed trailing whitespace",
			text: "line1 \t \nline2\t  \nline3",
			want: "line1\nline2\nline3",
		},
		{
			name: "no trailing whitespace",
			text: "line1\nline2\nline3",
			want: "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimTrailingWhitespace(tt.text)
			if got != tt.want {
				t.Errorf("TrimTrailingWhitespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearchReplace_MultipleOccurrences(t *testing.T) {
	// SearchReplace should only replace the first occurrence
	content := `func test() {
	return 1
}

func test() {
	return 1
}
`

	searchBlock := `func test() {
	return 1
}`

	replaceBlock := `func test() {
	return 2
}`

	sr := NewSearchReplace(content, searchBlock, replaceBlock)
	newContent, result := sr.Apply()

	if !result.Found {
		t.Fatal("Expected to find match")
	}

	// Count occurrences of the original block in the result
	count := strings.Count(newContent, searchBlock)
	if count != 1 {
		t.Errorf("Expected 1 remaining occurrence of search block, got %d", count)
	}

	// Verify the replacement exists
	if !strings.Contains(newContent, replaceBlock) {
		t.Error("Replacement block not found in result")
	}
}

func TestSearchReplace_EmptyBlocks(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		searchBlock  string
		replaceBlock string
		wantFound    bool
	}{
		{
			name:         "empty search block",
			content:      "some content",
			searchBlock:  "",
			replaceBlock: "replacement",
			wantFound:    false,
		},
		{
			name:         "empty content",
			content:      "",
			searchBlock:  "search",
			replaceBlock: "replace",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := NewSearchReplace(tt.content, tt.searchBlock, tt.replaceBlock)
			_, result := sr.Apply()

			if result.Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", result.Found, tt.wantFound)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSearchReplace_ExactMatch(b *testing.B) {
	content := strings.Repeat("func test() {\n\treturn 42\n}\n\n", 100)
	searchBlock := "func test() {\n\treturn 42\n}"
	replaceBlock := "func test() {\n\treturn 100\n}"

	b.ResetTimer()
	for range b.N {
		sr := NewSearchReplace(content, searchBlock, replaceBlock)
		sr.Apply()
	}
}

func BenchmarkSearchReplace_NoMatch(b *testing.B) {
	content := strings.Repeat("func test() {\n\treturn 42\n}\n\n", 100)
	searchBlock := "func notfound() {\n\treturn 0\n}"
	replaceBlock := "func replaced() {\n\treturn 1\n}"

	b.ResetTimer()
	for range b.N {
		sr := NewSearchReplace(content, searchBlock, replaceBlock)
		sr.Apply()
	}
}
