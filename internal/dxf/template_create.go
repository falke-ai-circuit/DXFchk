package dxf

import (
	"fmt"
	"os"
	"strings"
)

// CreateTemplateFromFile takes a source DXF file, changes the $(TEMPLATE) attribute
// value to newTemplateName, and saves the result to outputPath.
//
// This performs a raw text-level replacement (not parse + re-serialize) to preserve
// the exact DXF file structure byte-for-byte except for the template name change.
//
// The DXF has TWO instances of $(TEMPLATE):
// 1. ATTDEF (Attribute Definition) in the BLOCKS section — defines the attribute
// 2. ATTRIB (Attribute) in the ENTITIES section — the actual value used by the file
//
// Both must be changed. The pattern in DXF text format is:
//   code 1  → template value (e.g. "VLV01")
//   ...
//   code 2  → "$(TEMPLATE)"
//
// Returns the old template name that was replaced.
func CreateTemplateFromFile(srcPath, outputPath, newTemplateName string) (oldTemplate string, err error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("opening source DXF: %w", err)
	}
	defer f.Close()

	// Read all lines preserving the raw content
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("reading DXF: %w", err)
	}

	// Split into lines, preserving line endings info
	content := string(data)
	// Detect line ending style
	lineEnding := "\r\n"
	if !strings.Contains(content, "\r\n") && strings.Contains(content, "\n") {
		lineEnding = "\n"
	}
	lines := strings.Split(content, lineEnding)

	// Find ALL $(TEMPLATE) attribute instances and change their code 1 value
	// Pattern: code "1" followed by old value, then later code "2" = "$(TEMPLATE)"
	// We scan for code 2 = "$(TEMPLATE)" and walk backwards to find code 1
	changed := 0
	for i := 0; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) == "2" && strings.TrimSpace(lines[i+1]) == "$(TEMPLATE)" {
			// Walk backwards to find the code 1 (text value) in this entity
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "1" {
					if j+1 < len(lines) {
						if oldTemplate == "" {
							oldTemplate = strings.TrimSpace(lines[j+1])
						}
						lines[j+1] = newTemplateName
						changed++
					}
					break
				}
				// If we hit code 0 (new entity) without finding code 1, stop
				if strings.TrimSpace(lines[j]) == "0" {
					break
				}
			}
		}
	}

	if changed == 0 {
		return "", fmt.Errorf("could not find $(TEMPLATE) attribute in DXF file")
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