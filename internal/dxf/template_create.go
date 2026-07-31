package dxf

import (
	"fmt"
	"os"
	"strings"
)

// CreateTemplateFromFile takes a source DXF file (a module), normalizes it to create
// a template by applying the reverse-engineered DNA Explorer rules, and saves the
// result to outputPath.
//
// Reverse-engineered normalization rules (from 23 groups, 25,860 line diffs,
// validated with position-aware entity-type analysis across 5 groups):
//
//  1. $(TEMPLATE) attribute value → new template name (code 1 near code 2 = $(TEMPLATE))
//  2. Module ID → template name in all code-1 attribute values
//     Module ID derived from filename: XXX_pYYYY_pZZZZ → XXX.YYYY.ZZZZ
//  3. Entity-aware layer normalization (code 8):
//     - ATTRIB entities: Module "N_COM_HIDDEN" → Template "0"
//       (DNA Explorer sets ATTRIB layers from "0" to "N_COM_HIDDEN" when creating modules)
//     - SEQEND entities: Module "N_COM_EVAL_FALSE" → Template "N_COM_HIDDEN"
//       (DNA Explorer shifts SEQEND layers one step forward)
//     - INSERT entities: Module "N_COM_HIDDEN" → Template "1" (rare, 5 occurrences)
//     - N_COM_COM_HIDDEN on ATTRIB: stays as-is (template also has it)
//     - N_COM_COM_EVAL_FALSE on SEQEND: → N_COM_COM_HIDDEN
//
// This performs raw text-level replacement to preserve the exact DXF file structure.
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

	// === PASS 1: Find old template name and normalize $(TEMPLATE) attribute ===
	for i := 0; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) == "2" && strings.TrimSpace(lines[i+1]) == "$(TEMPLATE)" {
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "1" {
					if j+1 < len(lines) {
						if oldTemplate == "" {
							oldTemplate = strings.TrimSpace(lines[j+1])
						}
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

	if oldTemplate == "" {
		return "", fmt.Errorf("could not find $(TEMPLATE) attribute in DXF file")
	}

	// === PASS 2: Replace module ID with template name in code-1 values ===
	if moduleID != "" {
		for i := 0; i < len(lines); i++ {
			if i > 0 && strings.TrimSpace(lines[i-1]) == "1" {
				if strings.Contains(lines[i], moduleID) {
					lines[i] = strings.ReplaceAll(lines[i], moduleID, newTemplateName)
				}
			}
		}
	// Note: We do NOT replace the dotless variant (e.g., "60250000006").
	// The original templates keep device tag numbers as-is (e.g., "63160610134").
	// Replacing dotless variants would incorrectly replace device tags with the template name.
	}

	// === PASS 3: Entity-aware layer normalization (code 8) ===
	// Walk through lines tracking the current entity type (from code 0).
	// When we find a code-8 value, normalize based on entity type:
	//   ATTRIB: N_COM_HIDDEN → 0, N_COM_COM_EVAL_FALSE → N_COM_COM_HIDDEN
	//   SEQEND: N_COM_EVAL_FALSE → N_COM_HIDDEN, N_COM_COM_EVAL_FALSE → N_COM_COM_HIDDEN
	//   INSERT: N_COM_HIDDEN → 1 (rare)
	currentEntity := ""
	for i := 0; i < len(lines); i++ {
		// Track entity type: code 0 followed by entity name
		if strings.TrimSpace(lines[i]) == "0" && i+1 < len(lines) {
			currentEntity = strings.TrimSpace(lines[i+1])
			continue
		}

		// Check for code 8 (layer)
		if i > 0 && strings.TrimSpace(lines[i-1]) == "8" {
			val := strings.TrimSpace(lines[i])
			normalized := normalizeLayerByEntity(val, currentEntity)
			if normalized != val {
				lines[i] = normalized
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

// normalizeLayerByEntity normalizes a layer value based on the DXF entity type.
//
// From position-aware analysis of 5 groups (3,724 layer diffs):
//
//	ATTRIB entities (3,140 diffs):
//	  Template "0" → Module "N_COM_HIDDEN"  (reverse: N_COM_HIDDEN → "0")
//	  Template "0" → Module "N_COM_COM_HIDDEN" (rare, stays as-is in template)
//	SEQEND entities (584 diffs):
//	  Template "N_COM_HIDDEN" → Module "N_COM_EVAL_FALSE"  (reverse: EVAL_FALSE → HIDDEN)
//	  Template "N_COM_COM_HIDDEN" → Module "N_COM_COM_EVAL_FALSE"  (reverse: same)
//	  Template "1" → Module "N" (color number changes, rare)
//	INSERT entities (5 diffs):
//	  Template "1" → Module "1_COM_HIDDEN"  (reverse: 1_COM_HIDDEN → "1")
func normalizeLayerByEntity(layer, entityType string) string {
	switch entityType {
	case "ATTRIB":
		// N_COM_HIDDEN → 0 (but NOT N_COM_COM_HIDDEN)
		if strings.HasSuffix(layer, "_COM_HIDDEN") && !strings.HasSuffix(layer, "_COM_COM_HIDDEN") {
			return "0"
		}
		// N_COM_COM_EVAL_FALSE → N_COM_COM_HIDDEN (one step back)
		if strings.HasSuffix(layer, "_COM_COM_EVAL_FALSE") {
			return strings.TrimSuffix(layer, "_COM_COM_EVAL_FALSE") + "_COM_COM_HIDDEN"
		}
		// N_COM_COM_HIDDEN stays as-is (template also has it)

	case "SEQEND":
		// N_COM_EVAL_FALSE → N_COM_HIDDEN (one step back)
		if strings.HasSuffix(layer, "_COM_COM_EVAL_FALSE") {
			return strings.TrimSuffix(layer, "_COM_COM_EVAL_FALSE") + "_COM_COM_HIDDEN"
		}
		if strings.HasSuffix(layer, "_COM_EVAL_FALSE") {
			return strings.TrimSuffix(layer, "_COM_EVAL_FALSE") + "_COM_HIDDEN"
		}
		// N_COM_HIDDEN stays as-is on SEQEND (template also has it)

	case "INSERT":
		// INSERT layer changes are rare (5 occurrences in 5 groups) and
		// position-dependent. Normalizing them introduces more errors than
		// it fixes. Leave INSERT layers as-is.
	}

	return layer
}

// extractModuleID derives the module ID from the DXF filename.
// Pattern: XXX_pYYYY_pZZZZ.dxf → XXX.YYYY.ZZZZ
// Also handles: AU_c631_p1811_p0061.dxf → 631.1811.0061
func extractModuleID(filepath string) string {
	base := filepath
	if idx := strings.LastIndex(base, "\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.TrimSuffix(base, ".dxf")

	parts := strings.Split(base, "_")
	var numericParts []string
	for _, p := range parts {
		stripped := p
		if len(p) > 1 && (p[0] == 'p' || p[0] == 'c') {
			rest := p[1:]
			if isNumeric(rest) && len(rest) >= 3 {
				stripped = rest
			}
		}
		if isNumeric(stripped) && len(stripped) >= 3 {
			numericParts = append(numericParts, stripped)
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

// ReadTemplateName reads a DXF file and returns the value of the $(TEMPLATE) attribute.
func ReadTemplateName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	lines := strings.Split(content, "\r\n")
	if len(lines) < 2 {
		lines = strings.Split(content, "\n")
	}

	for i := 0; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) == "2" && strings.TrimSpace(lines[i+1]) == "$(TEMPLATE)" {
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "1" {
					if j+1 < len(lines) {
						return strings.TrimSpace(lines[j+1]), nil
					}
					break
				}
				if strings.TrimSpace(lines[j]) == "0" {
					break
				}
			}
		}
	}
	return "", fmt.Errorf("$(TEMPLATE) attribute not found")
}