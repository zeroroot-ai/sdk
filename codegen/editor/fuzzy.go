// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"strings"
	"unicode"
)

// DefaultFuzzyThreshold is the default similarity threshold for fuzzy matching.
// A value of 0.9 means 90% similarity is required for a match.
const DefaultFuzzyThreshold = 0.9

// FuzzyMatcher performs fuzzy string matching using Levenshtein distance.
// It can find approximate matches for SEARCH blocks when exact matching fails,
// tolerating minor differences in whitespace, indentation, or small edits.
type FuzzyMatcher struct {
	// content is the original file content to search within
	content string

	// searchBlock is the block to find (approximately)
	searchBlock string

	// threshold is the minimum similarity score required (0.0 to 1.0)
	threshold float64

	// lineEnding is the detected line ending style
	lineEnding string
}

// NewFuzzyMatcher creates a new FuzzyMatcher with the default threshold (0.9).
func NewFuzzyMatcher(content, searchBlock string) *FuzzyMatcher {
	return &FuzzyMatcher{
		content:     content,
		searchBlock: searchBlock,
		threshold:   DefaultFuzzyThreshold,
		lineEnding:  detectLineEnding(content),
	}
}

// WithThreshold sets a custom similarity threshold for fuzzy matching.
// The threshold should be between 0.0 and 1.0, where 1.0 requires exact matches.
func (fm *FuzzyMatcher) WithThreshold(threshold float64) *FuzzyMatcher {
	if threshold < 0.0 {
		threshold = 0.0
	}
	if threshold > 1.0 {
		threshold = 1.0
	}
	fm.threshold = threshold
	return fm
}

// FuzzyMatchResult represents the result of a fuzzy search operation.
type FuzzyMatchResult struct {
	// Found indicates whether a match above the threshold was found
	Found bool

	// Similarity is the similarity score between 0.0 and 1.0
	Similarity float64

	// StartPos is the byte position where the match starts (-1 if not found)
	StartPos int

	// EndPos is the byte position where the match ends (-1 if not found)
	EndPos int

	// StartLine is the 1-based line number where the match starts
	StartLine int

	// EndLine is the 1-based line number where the match ends
	EndLine int

	// MatchedContent is the actual content that was matched
	MatchedContent string
}

// FindBestMatch performs fuzzy matching to find the best approximate match
// for the search block in the content. It uses a sliding window approach to
// compare substrings of similar length to the search block.
func (fm *FuzzyMatcher) FindBestMatch() FuzzyMatchResult {
	// Normalize both content and search block for comparison
	normalizedContent := fm.normalizeForFuzzy(fm.content)
	normalizedSearch := fm.normalizeForFuzzy(fm.searchBlock)

	searchLines := strings.Split(normalizedSearch, "\n")
	contentLines := strings.Split(normalizedContent, "\n")

	// Calculate search window size (number of lines)
	windowSize := len(searchLines)

	// Track the best match found
	var bestMatch FuzzyMatchResult
	bestMatch.Similarity = 0.0
	bestMatch.Found = false

	// Slide a window through the content
	for i := 0; i <= len(contentLines)-windowSize; i++ {
		// Extract window of lines
		windowLines := contentLines[i : i+windowSize]
		windowText := strings.Join(windowLines, "\n")

		// Calculate similarity
		similarity := calculateSimilarity(normalizedSearch, windowText)

		// Update best match if this is better
		if similarity > bestMatch.Similarity {
			// Map back to original content positions
			startLine := i + 1 // 1-based line numbering
			endLine := i + windowSize

			startPos, endPos := fm.getOriginalPositions(startLine, endLine)

			bestMatch = FuzzyMatchResult{
				Found:          similarity >= fm.threshold,
				Similarity:     similarity,
				StartPos:       startPos,
				EndPos:         endPos,
				StartLine:      startLine,
				EndLine:        endLine,
				MatchedContent: fm.content[startPos:endPos],
			}
		}
	}

	// Also try variable-sized windows (±20% of search block size)
	// This handles cases where line breaks differ slightly
	minWindow := max(1, windowSize-max(1, windowSize/5))
	maxWindow := windowSize + max(1, windowSize/5)

	for size := minWindow; size <= maxWindow; size++ {
		if size == windowSize {
			continue // Already tried this size
		}

		for i := 0; i <= len(contentLines)-size; i++ {
			windowLines := contentLines[i : i+size]
			windowText := strings.Join(windowLines, "\n")

			similarity := calculateSimilarity(normalizedSearch, windowText)

			if similarity > bestMatch.Similarity {
				startLine := i + 1
				endLine := i + size

				startPos, endPos := fm.getOriginalPositions(startLine, endLine)

				bestMatch = FuzzyMatchResult{
					Found:          similarity >= fm.threshold,
					Similarity:     similarity,
					StartPos:       startPos,
					EndPos:         endPos,
					StartLine:      startLine,
					EndLine:        endLine,
					MatchedContent: fm.content[startPos:endPos],
				}
			}
		}
	}

	return bestMatch
}

// normalizeForFuzzy normalizes text for fuzzy comparison by:
// - Converting to lowercase
// - Normalizing whitespace (collapse multiple spaces, trim lines)
// - Normalizing line endings
// - Removing trailing whitespace from lines
func (fm *FuzzyMatcher) normalizeForFuzzy(text string) string {
	// Normalize line endings first
	text = normalizeLineEndings(text, "\n")

	lines := strings.Split(text, "\n")
	normalized := make([]string, len(lines))

	for i, line := range lines {
		// Trim trailing whitespace
		line = strings.TrimRight(line, " \t")

		// Collapse multiple spaces into single space (but preserve tabs)
		line = collapseSpaces(line)

		// Convert to lowercase for case-insensitive comparison
		line = strings.ToLower(line)

		normalized[i] = line
	}

	return strings.Join(normalized, "\n")
}

// collapseSpaces collapses consecutive spaces into a single space while preserving tabs.
func collapseSpaces(s string) string {
	var result strings.Builder
	lastWasSpace := false

	for _, ch := range s {
		if ch == ' ' {
			if !lastWasSpace {
				result.WriteRune(ch)
				lastWasSpace = true
			}
		} else {
			result.WriteRune(ch)
			lastWasSpace = false
		}
	}

	return result.String()
}

// getOriginalPositions maps line numbers back to byte positions in the original content.
func (fm *FuzzyMatcher) getOriginalPositions(startLine, endLine int) (int, int) {
	lines := strings.Split(fm.content, "\n")

	// Calculate start position
	startPos := 0
	for i := 0; i < startLine-1 && i < len(lines); i++ {
		startPos += len(lines[i]) + len(fm.lineEnding)
	}

	// Calculate end position
	endPos := startPos
	for i := startLine - 1; i < endLine && i < len(lines); i++ {
		endPos += len(lines[i])
		if i < len(lines)-1 {
			endPos += len(fm.lineEnding)
		}
	}

	return startPos, endPos
}

// calculateSimilarity computes the Levenshtein similarity between two strings.
// Returns a value between 0.0 (completely different) and 1.0 (identical).
func calculateSimilarity(s1, s2 string) float64 {
	distance := levenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))

	if maxLen == 0 {
		return 1.0
	}

	// Convert distance to similarity score
	similarity := 1.0 - (float64(distance) / float64(maxLen))

	return similarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
// This is the minimum number of single-character edits (insertions, deletions,
// or substitutions) required to change one string into the other.
//
// This implementation uses the dynamic programming approach with space optimization.
func levenshteinDistance(s1, s2 string) int {
	// Convert to rune slices for proper Unicode handling
	r1 := []rune(s1)
	r2 := []rune(s2)

	len1 := len(r1)
	len2 := len(r2)

	// Handle empty strings
	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	// Create two work vectors of integer distances
	// We only need two rows at a time for space efficiency
	prevRow := make([]int, len2+1)
	currRow := make([]int, len2+1)

	// Initialize the first row (distances from empty string)
	for i := 0; i <= len2; i++ {
		prevRow[i] = i
	}

	// Calculate distances
	for i := 1; i <= len1; i++ {
		// First column (distance from empty string)
		currRow[0] = i

		for j := 1; j <= len2; j++ {
			// Cost is 0 if characters match, 1 if they don't
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			// Calculate minimum of:
			// - deletion: currRow[j-1] + 1
			// - insertion: prevRow[j] + 1
			// - substitution: prevRow[j-1] + cost
			currRow[j] = min(
				currRow[j-1]+1,    // deletion
				prevRow[j]+1,      // insertion
				prevRow[j-1]+cost, // substitution
			)
		}

		// Swap rows
		prevRow, currRow = currRow, prevRow
	}

	// The final distance is in the last cell of prevRow
	return prevRow[len2]
}

// FindAllMatches finds all matches above the threshold in the content.
// This is useful for debugging or when multiple similar blocks might exist.
func (fm *FuzzyMatcher) FindAllMatches() []FuzzyMatchResult {
	normalizedContent := fm.normalizeForFuzzy(fm.content)
	normalizedSearch := fm.normalizeForFuzzy(fm.searchBlock)

	searchLines := strings.Split(normalizedSearch, "\n")
	contentLines := strings.Split(normalizedContent, "\n")

	windowSize := len(searchLines)
	matches := []FuzzyMatchResult{}

	// Slide window through content
	for i := 0; i <= len(contentLines)-windowSize; i++ {
		windowLines := contentLines[i : i+windowSize]
		windowText := strings.Join(windowLines, "\n")

		similarity := calculateSimilarity(normalizedSearch, windowText)

		if similarity >= fm.threshold {
			startLine := i + 1
			endLine := i + windowSize

			startPos, endPos := fm.getOriginalPositions(startLine, endLine)

			matches = append(matches, FuzzyMatchResult{
				Found:          true,
				Similarity:     similarity,
				StartPos:       startPos,
				EndPos:         endPos,
				StartLine:      startLine,
				EndLine:        endLine,
				MatchedContent: fm.content[startPos:endPos],
			})
		}
	}

	return matches
}

// NormalizeWhitespace normalizes whitespace in text for comparison.
// It removes leading/trailing whitespace and collapses internal whitespace.
func NormalizeWhitespace(text string) string {
	// Split into words (removes all whitespace)
	words := strings.FieldsFunc(text, unicode.IsSpace)

	// Join with single spaces
	return strings.Join(words, " ")
}

// min returns the minimum of three integers.
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
