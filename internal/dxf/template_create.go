package dxf

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// CreateTemplateFromFile takes a source DXF file (a module), normalizes it to create
// a template by applying the reverse-engineered DNA Explorer rules, and saves the
// result to outputPath.
//
// Reverse-engineered normalization rules (from 23 groups, 25,860 line diffs):
//
//  1. $(TEMPLATE) attribute value → new template name (code 1 near code 2 = $(TEMPLATE))
//  2. Module ID → template name in all code-1 attribute values
//     Module ID derived from filename: XXX_pYYYY_pZZZZ → XXX.YYYY.ZZZZ
//  3. Layer normalization (code 8): N_COM_HIDDEN, N_COM_EVAL_FALSE, N_COM_COM_HIDDEN,
//     N_COM_COM_EVAL_FALSE → "0" (template uses layer "0" for all hidden entities)
//  4. DEVICETAG normalization: detect pr:XXXXXXXXXXX patterns → pr:DEVICETAGn
//  5. Fixed placeholder restoration: PROJECT, TEMPLATE, DISPLAY, INSCODE, MC → kept as-is
//     (these are template-level values, not module-specific)
//
// This performs raw text-level replacement (not parse + re-serialize) to preserve
// the exact DXF file structure byte-for-byte.
//
// Returns the old template name (from $(TEMPLATE) attribute) that was replaced.
func CreateTemplateFromFile(srcPath, outputPath, newTemplateName string) (oldTemplate string, err error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("reading DXF: %w", err)
	}

	content := string(data)
	lineEnding := "\r\n"
	if !strings.Contains(content, "\r\n") && strings.Contains(content, "\n") {
		lineEnding = "\n"
	}
	lines := strings.Split(content, lineEnding)

	// Extract module ID from source filename (e.g., 602_p5000_p0006.dxf → 602.5000.0006)
	moduleID := extractModuleID(srcPath)

	// Track changes for reporting
	changes := struct {
		templateAttr int
		moduleID     int
		layers       int
		deviceTags   int
	}{}

	// === PASS 1: Find old template name and normalize $(TEMPLATE) attribute ===
	// Also collect module_id occurrences to know what to replace
	for i := 0; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) == "2" && strings.TrimSpace(lines[i+1]) == "$(TEMPLATE)" {
			// Walk backwards to find code 1 (text value)
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "1" {
					if j+1 < len(lines) {
						if oldTemplate == "" {
							oldTemplate = strings.TrimSpace(lines[j+1])
						}
						lines[j+1] = newTemplateName
						changes.templateAttr++
					}
					break
				}
				if strings.TrimSpace(lines[j]) == "0" {
					break
				}
			}
		}
	}

	if changes.templateAttr == 0 {
		return "", fmt.Errorf("could not find $(TEMPLATE) attribute in DXF file")
	}

	// === PASS 2: Replace module ID with template name in code-1 values ===
	// Module ID appears in attribute values as: "602.5000.0006.PL", "pr:602.5000.0006:av", etc.
	// We need to replace all occurrences of moduleID with newTemplateName
	if moduleID != "" {
		for i := 0; i < len(lines); i++ {
			// Only modify value lines (code 1 values), not code lines
			// Check if this is a value line (previous line is code "1")
			if i > 0 && strings.TrimSpace(lines[i-1]) == "1" {
				if strings.Contains(lines[i], moduleID) {
					lines[i] = strings.ReplaceAll(lines[i], moduleID, newTemplateName)
					changes.moduleID++
				}
			}
		}
	}

	// Also replace moduleID without dots (as seen in data: "60250000006" appears in some values)
	if moduleID != "" {
		moduleIDNoDots := strings.ReplaceAll(moduleID, ".", "")
		if moduleIDNoDots != moduleID {
			for i := 0; i < len(lines); i++ {
				if i > 0 && strings.TrimSpace(lines[i-1]) == "1" {
					if strings.Contains(lines[i], moduleIDNoDots) {
						lines[i] = strings.ReplaceAll(lines[i], moduleIDNoDots, newTemplateName)
						changes.moduleID++
					}
				}
			}
		}
	}

	// === PASS 3: Normalize layers (code 8) ===
	// Template has layer "0", module has layers like "1_COM_HIDDEN", "2_COM_EVAL_FALSE", etc.
	// Reverse: Replace all *_COM_HIDDEN, *_COM_EVAL_FALSE layers back to "0"
	// Also handle "N_COM_COM_HIDDEN" and "N_COM_COM_EVAL_FALSE"
	for i := 0; i < len(lines); i++ {
		if i > 0 && strings.TrimSpace(lines[i-1]) == "8" {
			val := strings.TrimSpace(lines[i])
			if isCOMLayer(val) {
				lines[i] = "0"
				changes.layers++
			}
		}
	}

	// Also handle layer "1" → "0" in some cases (from data: 10 times)
	// And "2" → "2_COM_HIDDEN" (6 times) etc. — these are less common and harder to determine
	// without the original template. Skip for now.

	// === PASS 4: DEVICETAG normalization ===
	// Template has "DEVICETAG1", "DEVICETAG2", etc.
	// Module has actual device tags like "63513200114" or "pr:63513200114"
	// Pattern: in code-1 values, detect device tag patterns and replace with DEVICETAGn
	// Device tags are: pr: followed by 10-13 digits, or bare 10-13 digit numbers
	// BUT: we can't know which DEVICETAGn to use without the original template
	// Strategy: detect pr:XXXXXXXXXXX patterns and replace with pr:DEVICETAGn
	// using a counter for each unique device tag
	deviceTagRegex := regexp.MustCompile(`pr:(\d{10,13})`)
	bareDeviceTagRegex := regexp.MustCompile(`^(\d{10,13})$`)

	// Build a mapping of unique device tags → DEVICETAGn
	deviceTagMap := make(map[string]string)
	tagCounter := 0

	for i := 0; i < len(lines); i++ {
		if i > 0 && strings.TrimSpace(lines[i-1]) == "1" {
			val := strings.TrimSpace(lines[i])
			// Check for pr:XXXXXXXXXXX pattern
			matches := deviceTagRegex.FindAllStringSubmatch(val, -1)
			for _, m := range matches {
				fullMatch := m[0] // pr:XXXXXXXXXXX
				if _, exists := deviceTagMap[fullMatch]; !exists {
					tagCounter++
					deviceTagMap[fullMatch] = fmt.Sprintf("pr:DEVICETAG%d", tagCounter)
				}
			}
			// Also check bare device tag numbers (only if entire value is a number)
			bareMatch := bareDeviceTagRegex.FindStringSubmatch(val)
			if bareMatch != nil {
				if _, exists := deviceTagMap[val]; !exists {
					tagCounter++
					deviceTagMap[val] = fmt.Sprintf("DEVICETAG%d", tagCounter)
				}
			}
		}
	}

	// Apply device tag replacements
	for i := 0; i < len(lines); i++ {
		if i > 0 && strings.TrimSpace(lines[i-1]) == "1" {
			val := lines[i]
			for original, replacement := range deviceTagMap {
				if strings.Contains(val, original) {
					lines[i] = strings.ReplaceAll(val, original, replacement)
					changes.deviceTags++
					break
		}
			}
		}
	}

	// Write the modified file
	out, err := os.Create(outputPath)
	if err != nil {
		return oldTemplate, fmt.Errorf("creating output file: %w", err)
	}
	defer out.Close()

	_, err = out.WriteString(strings.Join(lines, lineEnding))
	if err != nil {
		return oldTemplate, fmt.Errorf("writing output file: %w", err)
	}

	return oldTemplate, nil
}

// extractModuleID derives the module ID from the DXF filename.
// Pattern: XXX_pYYYY_pZZZZ.dxf → XXX.YYYY.ZZZZ
// Also handles: XXX_pYYYY.dxf → XXX.YYYY
// And: AU_c631_p1811_p0061.dxf → 631.1811.0061 (skip AU_ prefix)
func extractModuleID(filepath string) string {
	// Get base filename without extension
	base := filepath
	if idx := strings.LastIndex(base, "\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.TrimSuffix(base, ".dxf")

	// Split by underscore
	parts := strings.Split(base, "_")
	var numericParts []string
	for _, p := range parts {
		// Skip non-numeric parts (like "AU", "c631")
		if len(p) >= 4 && p[0] == 'c' {
			p = p[1:]
		}
		// Check if this part is a number (possibly with leading zeros)
		if isNumeric(p) {
			numericParts = append(numericParts, p)
		}
	}

	if len(numericParts) >= 3 {
		return strings.Join(numericParts[:3], ".")
	} else if len(numericParts) >= 2 {
		return strings.Join(numericParts[:2], ".")
	}
	return ""
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isCOMLayer checks if a layer name is a COM (component) hidden/eval layer
// that should be normalized back to "0" in templates.
func isCOMLayer(layer string) bool {
	// Patterns: N_COM_HIDDEN, N_COM_EVAL_FALSE, N_COM_COM_HIDDEN, N_COM_COM_EVAL_FALSE
	// Where N = 1, 2, 3, 4, etc.
	if strings.Contains(layer, "_COM_HIDDEN") || strings.Contains(layer, "_COM_EVAL_FALSE") {
		return true
	}
	// Also: N_COM_COM_HIDDEN, N_COM_COM_EVAL_FALSE
	if strings.Contains(layer, "_COM_COM_") {
		return true
	}
	return false
}