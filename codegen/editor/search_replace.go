// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

package editor

import (
	"strings"
)

// SearchReplace performs exact string matching for SEARCH blocks in file content.
// It finds the exact occurrence of the search block and replaces it with the
// replacement block, preserving the original indentation style.
type SearchReplace struct {
	// content is the original file content to search within
	content string

	// searchBlock is the exact string to find
	searchBlock string

	// replaceBlock is the replacement string
	replaceBlock string

	// lineEnding is the detected line ending style (\n or \r\n)
	lineEnding string
}

// NewSearchReplace creates a new SearchReplace instance with the given content
// and blocks. It automatically detects the line ending style (Unix vs Windows).
func NewSearchReplace(content, searchBlock, replaceBlock string) *SearchReplace {
	lineEnding := detectLineEnding(content)

	return &SearchReplace{
		content:      content,
		searchBlock:  searchBlock,
		replaceBlock: replaceBlock,
		lineEnding:   lineEnding,
	}
}

// MatchResult represents the result of a search operation.
type MatchResult struct {
	// Found indicates whether the search block was found
	Found bool

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

// Apply performs the exact search and replace operation.
// Returns the modified content and a MatchResult indicating success or failure.
func (sr *SearchReplace) Apply() (string, MatchResult) {
	result := sr.FindExact()

	if !result.Found {
		return sr.content, result
	}

	// Perform the replacement
	newContent := sr.content[:result.StartPos] + sr.replaceBlock + sr.content[result.EndPos:]

	return newContent, result
}

// FindExact searches for an exact match of the search block in the content.
// It tries multiple normalization strategies to account for line ending differences.
func (sr *SearchReplace) FindExact() MatchResult {
	// Handle empty search block
	if len(sr.searchBlock) == 0 {
		return MatchResult{
			Found:     false,
			StartPos:  -1,
			EndPos:    -1,
			StartLine: -1,
			EndLine:   -1,
		}
	}

	// Strategy 1: Try exact string match as-is
	pos := strings.Index(sr.content, sr.searchBlock)
	if pos != -1 {
		return sr.createMatchResult(pos, pos+len(sr.searchBlock), sr.searchBlock)
	}

	// Strategy 2: Normalize both content and search block to Unix line endings
	normalizedContent := normalizeLineEndings(sr.content, "\n")
	normalizedSearch := normalizeLineEndings(sr.searchBlock, "\n")

	pos = strings.Index(normalizedContent, normalizedSearch)
	if pos != -1 {
		// Map the position back to the original content
		originalPos := mapNormalizedToOriginalPos(sr.content, pos)
		endPos := originalPos + len(sr.searchBlock)

		// Extract the matched content from original
		matchedContent := sr.content[originalPos:endPos]

		return sr.createMatchResult(originalPos, endPos, matchedContent)
	}

	// Strategy 3: Try with Windows line endings
	normalizedContent = normalizeLineEndings(sr.content, "\r\n")
	normalizedSearch = normalizeLineEndings(sr.searchBlock, "\r\n")

	pos = strings.Index(normalizedContent, normalizedSearch)
	if pos != -1 {
		originalPos := mapNormalizedToOriginalPos(sr.content, pos)
		endPos := originalPos + len(sr.searchBlock)
		matchedContent := sr.content[originalPos:endPos]

		return sr.createMatchResult(originalPos, endPos, matchedContent)
	}

	// No match found
	return MatchResult{
		Found:     false,
		StartPos:  -1,
		EndPos:    -1,
		StartLine: -1,
		EndLine:   -1,
	}
}

// createMatchResult creates a MatchResult with line numbers calculated from positions.
func (sr *SearchReplace) createMatchResult(startPos, endPos int, matchedContent string) MatchResult {
	startLine := calculateLineNumber(sr.content, startPos)
	endLine := calculateLineNumber(sr.content, endPos)

	return MatchResult{
		Found:          true,
		StartPos:       startPos,
		EndPos:         endPos,
		StartLine:      startLine,
		EndLine:        endLine,
		MatchedContent: matchedContent,
	}
}

// detectLineEnding detects the line ending style used in the content.
// Returns "\r\n" for Windows-style, "\n" for Unix-style (default).
func detectLineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// normalizeLineEndings converts all line endings in the text to the target style.
func normalizeLineEndings(text, targetEnding string) string {
	// First normalize everything to \n
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Then convert to target if not \n
	if targetEnding != "\n" {
		text = strings.ReplaceAll(text, "\n", targetEnding)
	}

	return text
}

// mapNormalizedToOriginalPos maps a position in normalized content back to
// the original content position. This is needed when we normalize line endings
// for matching but need to return positions in the original text.
func mapNormalizedToOriginalPos(original string, normalizedPos int) int {
	// Count characters until we reach the normalized position
	originalPos := 0
	normalizedCount := 0

	for originalPos < len(original) && normalizedCount < normalizedPos {
		if originalPos < len(original)-1 && original[originalPos] == '\r' && original[originalPos+1] == '\n' {
			// Windows line ending counts as 1 in normalized but 2 in original
			originalPos += 2
			normalizedCount++
		} else {
			originalPos++
			normalizedCount++
		}
	}

	return originalPos
}

// calculateLineNumber calculates the 1-based line number for a byte position in text.
func calculateLineNumber(text string, pos int) int {
	if pos < 0 || pos > len(text) {
		return -1
	}

	line := 1
	for i := 0; i < pos && i < len(text); i++ {
		if text[i] == '\n' {
			line++
		}
	}

	return line
}

// PreserveIndentation adjusts the replacement block to match the indentation
// of the search block in the original content. This ensures that the replaced
// code maintains consistent indentation with the surrounding context.
func PreserveIndentation(searchBlock, replaceBlock string) string {
	// Detect the indentation style and level of the search block
	indentStyle, baseIndent := detectIndentation(searchBlock)

	// If no indentation detected, return replacement as-is
	if baseIndent == 0 {
		return replaceBlock
	}

	// Apply the same indentation to the replacement block
	return applyIndentation(replaceBlock, indentStyle, baseIndent)
}

// detectIndentation analyzes a code block to determine the indentation style
// (tabs or spaces) and the base indentation level (number of tabs/spaces).
// Returns (indentChar, indentLevel) where indentChar is "\t" or " ".
func detectIndentation(block string) (string, int) {
	lines := strings.Split(block, "\n")

	// Find the first non-empty line
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		// Count leading whitespace
		indent := 0
		indentChar := " "

		for i, ch := range line {
			switch ch {
			case '\t':
				indentChar = "\t"
				indent++
			case ' ':
				indent++
			default:
				// Found first non-whitespace character
				if indentChar == "\t" {
					return "\t", indent
				}
				// For spaces, return the count
				return " ", indent
			}

			// Safety check to avoid processing entire line
			if i > 100 {
				break
			}
		}
	}

	// No indentation detected
	return " ", 0
}

// applyIndentation adds the specified indentation to each non-empty line
// in the block. The indentChar should be "\t" or " ", and indentLevel
// specifies how many to add.
func applyIndentation(block, indentChar string, indentLevel int) string {
	if indentLevel == 0 {
		return block
	}

	lines := strings.Split(block, "\n")
	indent := strings.Repeat(indentChar, indentLevel)

	for i, line := range lines {
		// Only add indentation to non-empty lines
		if len(strings.TrimSpace(line)) > 0 {
			lines[i] = indent + line
		}
	}

	return strings.Join(lines, "\n")
}

// TrimTrailingWhitespace removes trailing whitespace from each line in the text.
// This is useful for normalizing content before comparison.
func TrimTrailingWhitespace(text string) string {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return strings.Join(lines, "\n")
}
