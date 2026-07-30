package dxf

import (
	"fmt"
	"os"
	"strings"
)

// ApplyTemplateToModule takes a fixed template DXF and a module DXF, and produces
// an output DXF where:
//   - The block STRUCTURE (INSERT entities: positions, layers, which blocks exist)
//     comes from the TEMPLATE
//   - The ATTRIB VALUES inside each INSERT come from the MODULE (preserved)
//   - The $(TEMPLATE) attribute value is set to newTemplateName
//
// Algorithm:
//  1. Split both files into lines
//  2. Find the ENTITIES section boundary in both
//  3. Extract INSERT entities (with their ATTRIBs) from both template and module
//  4. Match INSERTs by block_name + sequential order (nth occurrence of that block)
//  5. For matched pairs: take template's INSERT structure (position, layer, etc.)
//     but replace ATTRIB values with module's values
//  6. For template-only INSERTs: add them with empty/placeholder ATTRIBs
//  7. For module-only INSERTs: skip them (they're removed)
//  8. Rebuild the ENTITIES section with the merged INSERTs + non-INSERT entities from template
//  9. Replace $(TEMPLATE) attribute value with newTemplateName
//
// This preserves the module's device-specific values while applying the corrected
// block arrangement from the template.
func ApplyTemplateToModule(templatePath, modulePath, outputPath, newTemplateName string) (result ApplyResult, err error) {
	// Read both files
	tmplData, err := os.ReadFile(templatePath)
	if err != nil {
		return result, fmt.Errorf("reading template: %w", err)
	}
	modData, err := os.ReadFile(modulePath)
	if err != nil {
		return result, fmt.Errorf("reading module: %w", err)
	}

	// Detect line ending
	lineEnding := "\r\n"
	if !strings.Contains(string(tmplData), "\r\n") {
		lineEnding = "\n"
	}

	tmplLines := splitLines(string(tmplData), lineEnding)
	modLines := splitLines(string(modData), lineEnding)

	// Find ENTITIES section boundaries in both files
	tmplEntStart, tmplEntEnd := findEntitiesSection(tmplLines)
	modEntStart, modEntEnd := findEntitiesSection(modLines)

	if tmplEntStart < 0 || tmplEntEnd < 0 {
		return result, fmt.Errorf("could not find ENTITIES section in template")
	}
	if modEntStart < 0 || modEntEnd < 0 {
		return result, fmt.Errorf("could not find ENTITIES section in module")
	}

	// Extract INSERT entities (with ATTRIBs) from template and module
	tmplInserts := extractInsertEntities(tmplLines, tmplEntStart, tmplEntEnd)
	modInserts := extractInsertEntities(modLines, modEntStart, modEntEnd)

	result.TemplateInserts = len(tmplInserts)
	result.ModuleInserts = len(modInserts)

	// Match INSERTs by block_name + sequential order
	// Build a queue of module INSERTs per block_name
	modByBlock := make(map[string][]insertEntity)
	for _, ins := range modInserts {
		modByBlock[ins.blockName] = append(modByBlock[ins.blockName], ins)
	}

	// Build the merged INSERT list
	var mergedInserts []insertEntity
	matched := 0
	addedFromTemplate := 0
	removedFromModule := 0

	for _, tIns := range tmplInserts {
		queue := modByBlock[tIns.blockName]
		if len(queue) > 0 {
			// Match found: take template structure, module ATTRIBs
			mIns := queue[0]
			modByBlock[tIns.blockName] = queue[1:]
			merged := mergeInsert(tIns, mIns)
			mergedInserts = append(mergedInserts, merged)
			matched++
		} else {
			// No matching module INSERT — add from template with placeholder ATTRIBs
			mergedInserts = append(mergedInserts, tIns)
			addedFromTemplate++
		}
	}

	// Count removed (module INSERTs not matched to any template INSERT)
	for _, queue := range modByBlock {
		removedFromModule += len(queue)
	}

	result.Matched = matched
	result.AddedFromTemplate = addedFromTemplate
	result.RemovedFromModule = removedFromModule

	// Rebuild the output file:
	// - Lines before ENTITIES section: from template (includes HEADER, TABLES, BLOCKS)
	// - ENTITIES section: rebuilt from merged INSERTs + non-INSERT entities from template
	// - Lines after ENTITIES section: from template (includes EOF)
	var outLines []string

	// 1. Everything before ENTITIES section from template
	outLines = append(outLines, tmplLines[:tmplEntStart]...)

	// 2. Rebuild ENTITIES section
	// Write the ENTITIES section
	outLines = append(outLines, tmplLines[tmplEntStart]) // "0" "SECTION" marker lines
	outLines = append(outLines, tmplLines[tmplEntStart+1])
	outLines = append(outLines, tmplLines[tmplEntStart+2])

	// Write non-INSERT entities first (or interleaved? DXF typically has INSERTs mixed)
	// Actually, we need to preserve the template's entity ORDER for non-INSERT entities
	// and replace INSERT entities with the merged ones.
	// Let's rebuild by walking the template's ENTITIES section in order,
	// replacing each INSERT with the corresponding merged INSERT.
	mergedIdx := 0
	for i := tmplEntStart + 3; i < tmplEntEnd; i++ {
		line := strings.TrimSpace(tmplLines[i])
		if line == "0" && i+1 < len(tmplLines) {
			nextLine := strings.TrimSpace(tmplLines[i+1])
			if nextLine == "INSERT" {
				// Replace this INSERT with the merged version
				if mergedIdx < len(mergedInserts) {
					outLines = append(outLines, mergedInserts[mergedIdx].lines...)
					mergedIdx++
				}
				// Skip the entire INSERT entity in the template
				i = skipInsertEntity(tmplLines, i)
				continue
			}
		}
		outLines = append(outLines, tmplLines[i])
	}

	// If there are more merged INSERTs than template INSERTs (shouldn't happen normally),
	// append them
	for ; mergedIdx < len(mergedInserts); mergedIdx++ {
		outLines = append(outLines, mergedInserts[mergedIdx].lines...)
	}

	// 3. Write ENDSEC marker
	// Find the ENDSEC line in the template
	if tmplEntEnd < len(tmplLines) {
		// Write "0" "ENDSEC"
		outLines = append(outLines, tmplLines[tmplEntEnd-1], tmplLines[tmplEntEnd])
	}

	// 4. Everything after ENTITIES section from template
	if tmplEntEnd+1 < len(tmplLines) {
		outLines = append(outLines, tmplLines[tmplEntEnd+1:]...)
	}

	// 5. Replace $(TEMPLATE) attribute value with new template name
	outLines = replaceTemplateAttr(outLines, newTemplateName)

	// Write output file
	outStr := strings.Join(outLines, lineEnding)
	if err := os.WriteFile(outputPath, []byte(outStr), 0644); err != nil {
		return result, fmt.Errorf("writing output: %w", err)
	}

	result.OutputPath = outputPath
	result.Success = true
	return result, nil
}

// ApplyResult holds statistics about the template application
type ApplyResult struct {
	Success           bool   `json:"success"`
	OutputPath        string `json:"output_path"`
	TemplateInserts   int    `json:"template_inserts"`
	ModuleInserts     int    `json:"module_inserts"`
	Matched           int    `json:"matched"`
	AddedFromTemplate int    `json:"added_from_template"`
	RemovedFromModule int    `json:"removed_from_module"`
}

// insertEntity holds the raw DXF lines for an INSERT entity (including its ATTRIBs)
// and the parsed block name
type insertEntity struct {
	blockName string
	lines     []string
	startIdx  int // index in the original file
	// Parsed ATTRIBs: tag -> value, for merging
	attribs map[string]string
}

// splitLines splits text into lines, preserving content without the line ending
func splitLines(text, lineEnding string) []string {
	return strings.Split(text, lineEnding)
}

// findEntitiesSection returns the start and end indices of the ENTITIES section
// startIdx points to the "0" code before "SECTION", endIdx points to the "0" code before "ENDSEC"
func findEntitiesSection(lines []string) (start, end int) {
	start = -1
	end = -1
	for i := 0; i < len(lines)-2; i++ {
		if strings.TrimSpace(lines[i]) == "0" &&
			strings.TrimSpace(lines[i+1]) == "SECTION" &&
			i+3 < len(lines) &&
			strings.TrimSpace(lines[i+2]) == "2" &&
			strings.TrimSpace(lines[i+3]) == "ENTITIES" {
			start = i
		}
		if start >= 0 && strings.TrimSpace(lines[i]) == "0" && strings.TrimSpace(lines[i+1]) == "ENDSEC" {
			end = i + 1 // points to "ENDSEC" line
			return start, end
		}
	}
	return start, end
}

// extractInsertEntities finds all INSERT entities (with their ATTRIBs) within the ENTITIES section
func extractInsertEntities(lines []string, start, end int) []insertEntity {
	var inserts []insertEntity

	for i := start; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "0" && i+1 < len(lines) {
			if strings.TrimSpace(lines[i+1]) == "INSERT" {
				ins := insertEntity{
					startIdx: i,
					attribs:  make(map[string]string),
				}
				// Collect all lines until the next "0" code that starts a new entity (not ATTRIB)
				j := i + 1
				ins.lines = append(ins.lines, lines[i]) // "0"
				ins.lines = append(ins.lines, lines[i+1]) // "INSERT"

				j = i + 2
				for j < end {
					code := strings.TrimSpace(lines[j])
					if code == "0" {
						nextVal := ""
						if j+1 < len(lines) {
							nextVal = strings.TrimSpace(lines[j+1])
						}
						if nextVal == "ATTRIB" {
							// Collect ATTRIB entity
							ins.lines = append(ins.lines, lines[j])   // "0"
							ins.lines = append(ins.lines, lines[j+1]) // "ATTRIB"
							// Parse ATTRIB to get tag (code 2) and value (code 1)
							attrTag := ""
							attrVal := ""
							k := j + 2
							for k < end {
								acode := strings.TrimSpace(lines[k])
								if acode == "0" {
									// End of ATTRIB
									if attrTag != "" {
										ins.attribs[attrTag] = attrVal
									}
									// Check if next is another ATTRIB or end of INSERT
									if k+1 < len(lines) && strings.TrimSpace(lines[k+1]) == "ATTRIB" {
										attrTag = ""
										attrVal = ""
										ins.lines = append(ins.lines, lines[k], lines[k+1])
										k += 2
										continue
									} else {
										// End of INSERT entity
										j = k
										break
									}
								} else if acode == "2" {
									if k+1 < len(lines) {
										attrTag = strings.TrimSpace(lines[k+1])
									}
									ins.lines = append(ins.lines, lines[k], lines[k+1])
									k += 2
								} else if acode == "1" {
									if k+1 < len(lines) {
										attrVal = strings.TrimSpace(lines[k+1])
									}
									ins.lines = append(ins.lines, lines[k], lines[k+1])
									k += 2
								} else {
									ins.lines = append(ins.lines, lines[k])
									k++
								}
							}
							// Handle remaining ATTRIB
							if attrTag != "" && j != k {
								// Already handled above
							}
							j = k
							continue
						} else {
							// End of INSERT entity (next entity starts)
							break
						}
					}
					// Parse block name (code 2)
					if code == "2" && ins.blockName == "" && j+1 < len(lines) {
						ins.blockName = strings.TrimSpace(lines[j+1])
					}
					ins.lines = append(ins.lines, lines[j])
					j++
				}
				ins.lines = append(ins.lines, lines[j:j]...) // nothing extra
				inserts = append(inserts, ins)
				i = j - 1 // will be incremented by loop
			}
		}
	}
	return inserts
}

// mergeInsert takes the template INSERT's structure and the module INSERT's ATTRIB values
// and produces a merged INSERT entity
func mergeInsert(tmpl, mod insertEntity) insertEntity {
	// Start with the template's lines
	merged := insertEntity{
		blockName: tmpl.blockName,
		lines:     make([]string, len(tmpl.lines)),
		attribs:   make(map[string]string),
	}
	copy(merged.lines, tmpl.lines)

	// Now replace ATTRIB values in the merged lines with the module's values
	// Walk through the merged lines, find ATTRIB sections, and replace code 1 values
	// with the corresponding module ATTRIB values (matched by tag from code 2)
	i := 0
	for i < len(merged.lines) {
		if strings.TrimSpace(merged.lines[i]) == "0" && i+1 < len(merged.lines) {
			if strings.TrimSpace(merged.lines[i+1]) == "ATTRIB" {
				// Found an ATTRIB - find its tag (code 2) and value (code 1)
				attrStart := i
				attrTag := ""
				valueIdx := -1 // index of code "1" line
				j := i + 2
				for j < len(merged.lines) {
					code := strings.TrimSpace(merged.lines[j])
					if code == "0" {
						// End of ATTRIB
						// Replace the value if we found the tag and the module has it
						if attrTag != "" && valueIdx >= 0 {
							if modVal, ok := mod.attribs[attrTag]; ok {
								merged.lines[valueIdx+1] = modVal
							}
						}
						i = j
						break
					}
					if code == "2" && j+1 < len(merged.lines) {
						attrTag = strings.TrimSpace(merged.lines[j+1])
					}
					if code == "1" && j+1 < len(merged.lines) {
						valueIdx = j
					}
					j++
				}
				// Handle case where ATTRIB ends at end of lines
				if i == attrStart {
					if attrTag != "" && valueIdx >= 0 {
						if modVal, ok := mod.attribs[attrTag]; ok {
							merged.lines[valueIdx+1] = modVal
						}
					}
					i = j
				}
			} else {
				i++
			}
		} else {
			i++
		}
	}

	// Copy module attribs to merged
	for k, v := range mod.attribs {
		merged.attribs[k] = v
	}

	return merged
}

// skipInsertEntity returns the index after the last line of the INSERT entity starting at idx
func skipInsertEntity(lines []string, idx int) int {
	// idx points to "0", idx+1 is "INSERT"
	j := idx + 2
	for j < len(lines)-1 {
		if strings.TrimSpace(lines[j]) == "0" {
			nextVal := strings.TrimSpace(lines[j+1])
			if nextVal == "ATTRIB" {
				// Skip ATTRIB
				k := j + 2
				for k < len(lines)-1 {
					if strings.TrimSpace(lines[k]) == "0" {
						if k+1 < len(lines) && strings.TrimSpace(lines[k+1]) == "ATTRIB" {
							k += 2
							continue
						}
						// End of INSERT
						return k - 1
					}
					k++
				}
				return k - 1
			} else {
				// End of INSERT, next entity starts
				return j - 1
			}
		}
		j++
	}
	return j - 1
}

// replaceTemplateAttr replaces the $(TEMPLATE) attribute value with the new name
func replaceTemplateAttr(lines []string, newName string) []string {
	for i := 0; i < len(lines)-1; i++ {
		if strings.TrimSpace(lines[i]) == "2" && strings.TrimSpace(lines[i+1]) == "$(TEMPLATE)" {
			// Walk backwards to find code "1" (the value)
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) == "1" {
					if j+1 < len(lines) {
						lines[j+1] = newName
					}
					break
				}
				if strings.TrimSpace(lines[j]) == "0" {
					break
				}
			}
		}
	}
	return lines
}