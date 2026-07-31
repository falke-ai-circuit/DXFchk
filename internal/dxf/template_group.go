package dxf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CreateTemplateFromGroup creates a template by analyzing ALL modules in a group.
//
// The key insight: if you create a template from each DXF in a group using the
// existing normalization rules, the results should be IDENTICAL to the original
// template (for non-_modN groups). The values that DIFFER across modules are
// placeholders; the values that are IDENTICAL are fixed template content.
//
// Algorithm:
//  1. Normalize each module in the group (existing rules: $(TEMPLATE), module ID
//     replacement, entity-aware layer normalization)
//  2. Line-by-line compare all normalized versions
//  3. Where all modules agree → fixed value, keep it (use first module's value)
//  4. Where modules differ → placeholder position:
//     - Find the ATTRIB tag name (code 2) above the differing position
//     - Use the tag name as placeholder text (e.g., PROJECT, AP01, INSCODE)
//     - If no tag name found, use empty string (as some templates have empty values)
//  5. Write the merged result
//
// Returns stats about how many modules were analyzed and how many placeholders were inferred.
func CreateTemplateFromGroup(groupPath, outputPath, templateName string) (result GroupTemplateResult, err error) {
	// 1. Find all DXF files in the group folder
	entries, err := os.ReadDir(groupPath)
	if err != nil {
		return result, fmt.Errorf("reading group folder: %w", err)
	}

	var dxfFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dxf") {
			dxfFiles = append(dxfFiles, filepath.Join(groupPath, e.Name()))
		}
	}

	if len(dxfFiles) < 2 {
		return result, fmt.Errorf("need at least 2 DXF files in group for placeholder inference, found %d", len(dxfFiles))
	}

	result.ModulesAnalyzed = len(dxfFiles)

	// 2. Normalize each module into in-memory line arrays
	normalizedLines := make([][]string, len(dxfFiles))
	lineEnding := "\r\n"
	for idx, dxfFile := range dxfFiles {
		data, err := os.ReadFile(dxfFile)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", dxfFile, err)
		}
		content := string(data)
		if !strings.Contains(content, "\r\n") && strings.Contains(content, "\n") {
			lineEnding = "\n"
		}
		lines := strings.Split(content, lineEnding)
		normalizedLines[idx] = normalizeModuleLines(lines, dxfFile, templateName)
	}

	// 3. Verify all normalized versions have the same line count
	// (They should, since modules in the same group share the same template structure)
	lineCount := len(normalizedLines[0])
	for i, nl := range normalizedLines {
		if len(nl) != lineCount {
			// Line counts differ — this means structural differences exist.
			// Fall back to the first module's normalization.
			result.LineCountMismatch = true
			result.UsedFallback = true
			result.FallbackReason = fmt.Sprintf("module %d has %d lines, expected %d", i, len(nl), lineCount)
			// Use the first module's lines as the template
			out, err := os.Create(outputPath)
			if err != nil {
				return result, fmt.Errorf("creating output file: %w", err)
			}
			defer out.Close()
			_, err = out.WriteString(strings.Join(normalizedLines[0], lineEnding))
			return result, err
		}
	}

	// 4. Line-by-line merge: find positions where modules differ
	templateLines := make([]string, lineCount)
	placeholderCount := 0

	for lineIdx := 0; lineIdx < lineCount; lineIdx++ {
		// Check if all modules agree on this line
		allAgree := true
		firstVal := normalizedLines[0][lineIdx]
		for i := 1; i < len(normalizedLines); i++ {
			if normalizedLines[i][lineIdx] != firstVal {
				allAgree = false
				break
			}
		}

		if allAgree {
			// All modules agree — this is a fixed template value
			templateLines[lineIdx] = firstVal
		} else {
			// Modules differ — use majority voting to determine the template value.
			// DNA Explorer templates have fixed values, not placeholders. When modules
			// differ, it's because some modules were edited after creation. The
			// template value is the one shared by the MAJORITY of modules.
			majorityVal := majorityVote(normalizedLines, lineIdx)
			templateLines[lineIdx] = majorityVal
			placeholderCount++
		}
	}

	result.PlaceholdersInferred = placeholderCount

	// 5. Write the merged template
	out, err := os.Create(outputPath)
	if err != nil {
		return result, fmt.Errorf("creating output file: %w", err)
	}
	defer out.Close()

	_, err = out.WriteString(strings.Join(templateLines, lineEnding))
	if err != nil {
		return result, fmt.Errorf("writing output file: %w", err)
	}

	return result, nil
}

// GroupTemplateResult holds stats about the group template creation
type GroupTemplateResult struct {
	ModulesAnalyzed   int    `json:"modules_analyzed"`
	PlaceholdersInferred int  `json:"placeholders_inferred"`
	LineCountMismatch bool   `json:"line_count_mismatch"`
	UsedFallback       bool   `json:"used_fallback"`
	FallbackReason     string `json:"fallback_reason,omitempty"`
}

// normalizeModuleLines applies the existing normalization rules to a module's lines
// and returns the normalized line array (in-memory version of CreateTemplateFromFile)
func normalizeModuleLines(lines []string, srcPath, newTemplateName string) []string {
	moduleID := extractModuleID(srcPath)

	// PASS 1: Find old template name and normalize $(TEMPLATE) attribute
	for i := 0; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) == "2" && strings.TrimSpace(lines[i+1]) == "$(TEMPLATE)" {
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "1" {
					if j+1 < len(lines) {
						lines[j+1] = newTemplateName
					}
					break
				}
				if strings.TrimSpace(lines[j]) == "0" {
					break
				}
			}
		}
	}

	// PASS 2: Replace module ID with template name in code-1 values
	if moduleID != "" {
		for i := 0; i < len(lines); i++ {
			if i > 0 && strings.TrimSpace(lines[i-1]) == "1" {
				if strings.Contains(lines[i], moduleID) {
					lines[i] = strings.ReplaceAll(lines[i], moduleID, newTemplateName)
				}
			}
		}
	}

	// PASS 3: Entity-aware layer normalization (code 8)
	currentEntity := ""
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "0" && i+1 < len(lines) {
			currentEntity = strings.TrimSpace(lines[i+1])
			continue
		}
		if i > 0 && strings.TrimSpace(lines[i-1]) == "8" {
			val := strings.TrimSpace(lines[i])
			normalized := normalizeLayerByEntity(val, currentEntity)
			if normalized != val {
				lines[i] = normalized
			}
		}
	}

	return lines
}

// majorityVote returns the most common value among all modules at a given line index.
// This is used when modules differ: DNA Explorer templates have fixed values, not
// placeholders. When modules differ at a position, it's because some modules were
// edited after creation. The template value is the one shared by the majority.
func majorityVote(allModules [][]string, lineIdx int) string {
	counts := make(map[string]int)
	for _, lines := range allModules {
		if lineIdx < len(lines) {
			counts[lines[lineIdx]]++
		}
	}

	bestVal := ""
	bestCount := 0
	for val, count := range counts {
		if count > bestCount {
			bestVal = val
			bestCount = count
		}
	}
	return bestVal
}

// inferPlaceholder determines what placeholder text to use for a differing position.
// It looks backward from the current position to find the ATTRIB tag name (code 2).
//
// In DXF, an ATTRIB entity looks like:
//
//	0
//	ATTRIB
//	8
//	0
//	...
//	2          <- tag name code
//	PROJECT    <- tag name value (THIS is the placeholder name)
//	...
//	1          <- value code
//	051079-3   <- attribute value (THIS is where modules differ)
//
// So we look backward from the differing code-1 value line to find the
// nearest code-2 line that gives us the tag name.
func inferPlaceholder(allModules [][]string, lineIdx int, lineEnding string) string {
	// Look backward from lineIdx to find the ATTRIB tag name (code 2)
	for i := lineIdx - 1; i >= 0; i-- {
		if strings.TrimSpace(allModules[0][i]) == "0" {
			// We hit the start of an entity without finding code 2
			break
		}
		if strings.TrimSpace(allModules[0][i]) == "2" {
			// Found the tag name — the next line is the tag value
			if i+1 < len(allModules[0]) {
				tagName := strings.TrimSpace(allModules[0][i+1])
				if tagName != "" && tagName != "$(TEMPLATE)" {
					return tagName
				}
			}
			break
		}
	}

	// If no tag name found, use empty string (some template values are empty)
	return ""
}