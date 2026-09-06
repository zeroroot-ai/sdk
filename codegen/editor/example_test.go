// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor_test

import (
	"fmt"

	"github.com/zeroroot-ai/sdk/codegen/editor"
)

// Example demonstrating exact string matching with SearchReplace.
func ExampleSearchReplace_exactMatch() {
	content := `package main

func hello() {
	println("hello")
}

func main() {
	hello()
}
`

	searchBlock := `func hello() {
	println("hello")
}`

	replaceBlock := `func hello() {
	fmt.Println("hello world")
}`

	sr := editor.NewSearchReplace(content, searchBlock, replaceBlock)
	newContent, result := sr.Apply()

	if result.Found {
		fmt.Printf("Match found at lines %d-%d\n", result.StartLine, result.EndLine)
		fmt.Printf("Replacement successful\n")
		_ = newContent // Use the modified content
	}

	// Output:
	// Match found at lines 3-5
	// Replacement successful
}

// Example demonstrating fuzzy matching for code with minor differences.
func ExampleFuzzyMatcher_minorDifferences() {
	content := `package main

func calculate(x int) int {
	return x * 2
}
`

	// Search block has a different multiplier value
	searchBlock := `func calculate(x int) int {
	return x * 3
}`

	fm := editor.NewFuzzyMatcher(content, searchBlock).WithThreshold(0.85)
	result := fm.FindBestMatch()

	if result.Found {
		fmt.Printf("Found fuzzy match with %.0f%% similarity\n", result.Similarity*100)
		fmt.Printf("Matched at lines %d-%d\n", result.StartLine, result.EndLine)
	}

	// Output:
	// Found fuzzy match with 98% similarity
	// Matched at lines 3-5
}

// Example demonstrating whitespace tolerance in fuzzy matching.
func ExampleFuzzyMatcher_whitespaceTolerance() {
	// Content has tabs for indentation
	content := `func test() {
	return 42
}`

	// Search block has spaces for indentation
	searchBlock := `func test() {
    return 42
}`

	fm := editor.NewFuzzyMatcher(content, searchBlock).WithThreshold(0.8)
	result := fm.FindBestMatch()

	if result.Found {
		fmt.Printf("Match found despite whitespace differences\n")
		fmt.Printf("Similarity: %.2f\n", result.Similarity)
	}

	// Output:
	// Match found despite whitespace differences
	// Similarity: 0.96
}

// Example demonstrating how to handle line ending differences.
func ExampleSearchReplace_lineEndings() {
	// Content with Windows line endings
	contentWindows := "line1\r\nline2\r\nline3"

	// Search block with Unix line endings
	searchUnix := "line2"

	replaceBlock := "REPLACED"

	sr := editor.NewSearchReplace(contentWindows, searchUnix, replaceBlock)
	_, result := sr.Apply()

	fmt.Printf("Match found: %v\n", result.Found)

	// Output:
	// Match found: true
}

// Example demonstrating indentation preservation.
func ExamplePreserveIndentation() {
	// Original block with tab indentation
	searchBlock := "\tfunc old() {\n\t\treturn 1\n\t}"

	// Replacement block without indentation
	replaceBlock := "func new() {\n\treturn 2\n}"

	// Adjust replacement to match original indentation
	adjusted := editor.PreserveIndentation(searchBlock, replaceBlock)

	fmt.Printf("Indentation preserved: %v\n", adjusted == "\tfunc new() {\n\t\treturn 2\n\t}")

	// Output:
	// Indentation preserved: true
}

// Example demonstrating how to find multiple similar matches.
func ExampleFuzzyMatcher_findAll() {
	content := `func test1() {
	return 1
}

func test2() {
	return 2
}

func test3() {
	return 3
}
`

	searchBlock := `func testX() {
	return X
}`

	fm := editor.NewFuzzyMatcher(content, searchBlock).WithThreshold(0.7)
	matches := fm.FindAllMatches()

	fmt.Printf("Found %d matches\n", len(matches))
	for i, match := range matches {
		fmt.Printf("Match %d: lines %d-%d (%.0f%% similar)\n",
			i+1, match.StartLine, match.EndLine, match.Similarity*100)
	}

	// Output:
	// Found 5 matches
	// Match 1: lines 1-3 (92% similar)
	// Match 2: lines 4-6 (81% similar)
	// Match 3: lines 5-7 (92% similar)
	// Match 4: lines 8-10 (81% similar)
	// Match 5: lines 9-11 (92% similar)
}

// Example demonstrating case-insensitive fuzzy matching.
func ExampleFuzzyMatcher_caseInsensitive() {
	content := `func Hello() {
	Println("HELLO")
}`

	searchBlock := `func hello() {
	println("hello")
}`

	fm := editor.NewFuzzyMatcher(content, searchBlock).WithThreshold(0.9)
	result := fm.FindBestMatch()

	fmt.Printf("Case-insensitive match: %v\n", result.Found)
	fmt.Printf("Similarity: %.2f\n", result.Similarity)

	// Output:
	// Case-insensitive match: true
	// Similarity: 1.00
}

// Example demonstrating threshold configuration.
func ExampleFuzzyMatcher_withThreshold() {
	content := `func test() {
	return 42
}`

	searchBlock := `func test() {
	return 100
}`

	// Try with high threshold (strict)
	fm1 := editor.NewFuzzyMatcher(content, searchBlock).WithThreshold(0.95)
	result1 := fm1.FindBestMatch()

	// Try with lower threshold (lenient)
	fm2 := editor.NewFuzzyMatcher(content, searchBlock).WithThreshold(0.80)
	result2 := fm2.FindBestMatch()

	fmt.Printf("Strict match (0.95): %v\n", result1.Found)
	fmt.Printf("Lenient match (0.80): %v\n", result2.Found)
	fmt.Printf("Actual similarity: %.2f\n", result2.Similarity)

	// Output:
	// Strict match (0.95): false
	// Lenient match (0.80): true
	// Actual similarity: 0.89
}

// Example demonstrating exact match position tracking.
func ExampleSearchReplace_positionTracking() {
	content := `package main

func first() {}

func second() {
	return 2
}

func third() {}
`

	searchBlock := `func second() {
	return 2
}`

	replaceBlock := `func second() {
	return 42
}`

	sr := editor.NewSearchReplace(content, searchBlock, replaceBlock)
	_, result := sr.Apply()

	fmt.Printf("Match found: %v\n", result.Found)
	fmt.Printf("Start line: %d\n", result.StartLine)
	fmt.Printf("End line: %d\n", result.EndLine)
	fmt.Printf("Start position: %d\n", result.StartPos)
	fmt.Printf("End position: %d\n", result.EndPos)

	// Output:
	// Match found: true
	// Start line: 5
	// End line: 7
	// Start position: 31
	// End position: 58
}
