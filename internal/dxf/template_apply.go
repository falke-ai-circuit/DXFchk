package dxf

import (
	"fmt"
	"os"
	"strings"
)

// ApplyTemplateToModule takes a fixed template DXF and a module DXF, and produces
// an output DXF where:
//   - The block STRUCTURE (INSERT positions, layers, which blocks exist) comes from the TEMPLATE
//   - ALL module data is preserved (ATTRIB values, entity metadata, handles, owner refs, etc.)
//   - The $(TEMPLATE) attribute value is set to newTemplateName
//
// Algorithm (true in-place modification):
//  1. Start with the MODULE's lines as the output base
//  2. Parse template INSERTs to get their geometry (position, layer, scale) by block_name + order
//  3. Walk the MODULE's ENTITIES section:
//     - For each module INSERT: replace ONLY its geometry codes (8,10,20,30,50,41,42,43,62)
//       with the corresponding template INSERT's geometry
//     - Non-INSERT entities: copied as-is (untouched)
//  4. Template-only INSERTs (not in module): appended at end of ENTITIES section
//  5. Module-only INSERTs (not in template): removed (skip their lines)
//  6. Replace $(TEMPLATE) attribute value with newTemplateName
func ApplyTemplateToModule(templatePath, modulePath, outputPath, newTemplateName string) (result ApplyResult, err error) {
	tmplData, err := os.ReadFile(templatePath)
	if err != nil {
		return result, fmt.Errorf("reading template: %w", err)
	}
	modData, err := os.ReadFile(modulePath)
	if err != nil {
		return result, fmt.Errorf("reading module: %w", err)
	}

	lineEnding := "\r\n"
	if !strings.Contains(string(modData), "\r\n") {
		lineEnding = "\n"
	}

	tmplLines := splitLines(string(tmplData), lineEnding)
	modLines := splitLines(string(modData), lineEnding)

	tmplEntStart, tmplEntEnd := findEntitiesSection(tmplLines)
	modEntStart, modEntEnd := findEntitiesSection(modLines)

	if tmplEntStart < 0 || tmplEntEnd < 0 {
		return result, fmt.Errorf("could not find ENTITIES section in template")
	}
	if modEntStart < 0 || modEntEnd < 0 {
		return result, fmt.Errorf("could not find ENTITIES section in module")
	}

	// Extract template INSERT geometry (we only need geometry codes, not full entity)
	type insertGeom struct {
		blockName string
		codes     map[string]string // code -> value (8,10,20,30,50,41,42,43,62)
		lines     []string           // full template INSERT lines (for template-only INSERTs)
	}

	var tmplInserts []insertGeom
	i := tmplEntStart
	for i < tmplEntEnd-1 {
		if strings.TrimSpace(tmplLines[i]) == "0" && strings.TrimSpace(tmplLines[i+1]) == "INSERT" {
			geom := insertGeom{
				blockName: "",
				codes:     make(map[string]string),
			}
			// Collect INSERT-level codes (before first ATTRIB)
			j := i + 2
			inAttrib := false
			geomStart := i
			for j < tmplEntEnd-1 {
				code := strings.TrimSpace(tmplLines[j])
				if code == "0" {
					val := strings.TrimSpace(tmplLines[j+1])
					if val == "ATTRIB" {
						inAttrib = true
						// Still collect lines for template-only INSERT use
						geom.lines = append(geom.lines, tmplLines[geomStart:j]...)
						geom.lines = append(geom.lines, tmplLines[j], tmplLines[j+1])
						j += 2
						continue
					}
					if inAttrib && val != "ATTRIB" {
						// End of INSERT+ATTRIBs
						break
					}
					// End of INSERT (no ATTRIBs)
					geom.lines = append(geom.lines, tmplLines[geomStart:j]...)
					break
				}
				if !inAttrib {
					// INSERT-level code
					if code == "2" && geom.blockName == "" {
						geom.blockName = strings.TrimSpace(tmplLines[j+1])
					}
					// Collect geometry codes
					switch code {
					case "8", "10", "20", "30", "50", "41", "42", "43", "62":
						geom.codes[code] = tmplLines[j+1]
					}
				}
				j += 2
			}
			// If we didn't collect lines (no ATTRIB case handled above), collect rest
			if len(geom.lines) == 0 && j > geomStart {
				geom.lines = append(geom.lines, tmplLines[geomStart:j]...)
			}
			// Also collect ATTRIB lines if we were in attrib mode
			if inAttrib {
				// geom.lines already has the lines up to the break point
			}
			tmplInserts = append(tmplInserts, geom)
			i = j
		} else {
			i++
		}
	}

	// Extract module INSERT positions (to know which ones to skip/remove)
	type modInsertInfo struct {
		startIdx  int    // index in modLines where "0" "INSERT" is
		blockName string
	}
	var modInserts []modInsertInfo
	i = modEntStart
	for i < modEntEnd-1 {
		if strings.TrimSpace(modLines[i]) == "0" && strings.TrimSpace(modLines[i+1]) == "INSERT" {
			info := modInsertInfo{startIdx: i}
			// Find block name
			j := i + 2
			for j < modEntEnd-1 {
				code := strings.TrimSpace(modLines[j])
				if code == "0" {
					break
				}
				if code == "2" && info.blockName == "" {
					info.blockName = strings.TrimSpace(modLines[j+1])
				}
				j += 2
			}
			modInserts = append(modInserts, info)
			i = j
		} else {
			i++
		}
	}

	result.TemplateInserts = len(tmplInserts)
	result.ModuleInserts = len(modInserts)

	// Match template INSERTs to module INSERTs by block_name + sequential order
	modByBlock := make(map[string][]int) // block_name -> indices into modInserts
	for idx, ins := range modInserts {
		modByBlock[ins.blockName] = append(modByBlock[ins.blockName], idx)
	}

	matched := 0
	addedFromTemplate := 0
	removedFromModule := 0

	// Build a map: module INSERT index -> template INSERT geometry
	modInsertToTemplate := make(map[int]insertGeom)
	// Track which module INSERTs are matched
	matchedModIdxs := make(map[int]bool)
	// Template-only INSERTs (in template order)
	var templateOnlyInserts []insertGeom

	for _, tIns := range tmplInserts {
		queue := modByBlock[tIns.blockName]
		if len(queue) > 0 {
			modIdx := queue[0]
			modByBlock[tIns.blockName] = queue[1:]
			modInsertToTemplate[modIdx] = tIns
			matchedModIdxs[modIdx] = true
			matched++
		} else {
			templateOnlyInserts = append(templateOnlyInserts, tIns)
			addedFromTemplate++
		}
	}

	// Count removed
	for _, queue := range modByBlock {
		removedFromModule += len(queue)
	}

	result.Matched = matched
	result.AddedFromTemplate = addedFromTemplate
	result.RemovedFromModule = removedFromModule

	// Build output: copy module lines, modifying INSERT geometry in-place
	// and removing module-only INSERTs
	var outLines []string

	// 1. Everything before ENTITIES section from MODULE
	outLines = append(outLines, modLines[:modEntStart]...)

	// 2. Write ENTITIES section header
	outLines = append(outLines, modLines[modEntStart])   // "0"
	outLines = append(outLines, modLines[modEntStart+1]) // "SECTION"
	outLines = append(outLines, modLines[modEntStart+2]) // "2"
	outLines = append(outLines, modLines[modEntStart+3]) // "ENTITIES"

	// 3. Walk module ENTITIES section, modifying INSERTs in-place
	// Build a set of module INSERT start indices for quick lookup
	modInsertStarts := make(map[int]bool)
	for _, info := range modInserts {
		modInsertStarts[info.startIdx] = true
	}

	// Map: start index -> whether this INSERT should be removed (module-only)
	modInsertRemoved := make(map[int]bool)
	for idx := range modInserts {
		if !matchedModIdxs[idx] {
			modInsertRemoved[modInserts[idx].startIdx] = true
		}
	}

	// Walk the module's ENTITIES section line by line
	// Stop before the ENDSEC marker (modEntEnd-1 = "0", modEntEnd = "ENDSEC")
	i = modEntStart + 4
	for i < modEntEnd-1 {
		// Check if this is the start of an INSERT entity
		if strings.TrimSpace(modLines[i]) == "0" && i+1 < len(modLines) &&
			strings.TrimSpace(modLines[i+1]) == "INSERT" {

			// Find which module INSERT this is
			var modIdx int = -1
			for idx, info := range modInserts {
				if info.startIdx == i {
					modIdx = idx
					break
				}
			}

			if modIdx >= 0 && modInsertRemoved[i] {
				// Module-only INSERT — skip it entirely
				i = skipInsertEntity(modLines, i)
				i++ // move past the last line
				continue
			}

			if modIdx >= 0 {
				// Matched INSERT — copy its lines, replacing only geometry codes
				tGeom, hasGeom := modInsertToTemplate[modIdx]

				// Find the end of this INSERT entity (including ATTRIBs)
				insertEnd := skipInsertEntity(modLines, i)

				// Walk the INSERT's lines, replacing geometry codes in the INSERT-level section
				inAttrib := false
				j := i
				for j <= insertEnd && j < modEntEnd {
					code := strings.TrimSpace(modLines[j])
					if code == "0" && j+1 < len(modLines) {
						val := strings.TrimSpace(modLines[j+1])
						if val == "ATTRIB" {
							inAttrib = true
							// Copy "0" and "ATTRIB" lines as-is
							outLines = append(outLines, modLines[j], modLines[j+1])
							j += 2
							continue
						}
						if inAttrib && val != "ATTRIB" {
							// End of INSERT+ATTRIBs — copy remaining and break
							// This is the start of the next entity (e.g., SEQEND)
							break
						}
						// "0" "INSERT" — the initial pair
						outLines = append(outLines, modLines[j], modLines[j+1])
						j += 2
						continue
					}

					if !inAttrib && hasGeom {
						// INSERT-level code — replace geometry codes with template values
						switch code {
						case "8", "10", "20", "30", "50", "41", "42", "43", "62":
							if tmplVal, ok := tGeom.codes[code]; ok {
								outLines = append(outLines, modLines[j], tmplVal)
							} else {
								outLines = append(outLines, modLines[j], modLines[j+1])
							}
						default:
							outLines = append(outLines, modLines[j], modLines[j+1])
						}
					} else {
						// ATTRIB-level or no geometry — copy as-is
						outLines = append(outLines, modLines[j], modLines[j+1])
					}
					j += 2
				}

				i = j
				continue
			}

			// Unknown INSERT (shouldn't happen) — copy as-is
			outLines = append(outLines, modLines[i])
			i++
			continue
		}

		// Non-INSERT entity — copy as-is
		outLines = append(outLines, modLines[i])
		i++
	}

	// 4. Append template-only INSERTs at end of ENTITIES section
	for _, tIns := range templateOnlyInserts {
		outLines = append(outLines, tIns.lines...)
	}

	// 5. Write ENDSEC marker from MODULE
	if modEntEnd < len(modLines) {
		outLines = append(outLines, modLines[modEntEnd-1]) // "0"
		outLines = append(outLines, modLines[modEntEnd])   // "ENDSEC"
	}

	// 6. Everything after ENTITIES section from MODULE
	if modEntEnd+1 < len(modLines) {
		outLines = append(outLines, modLines[modEntEnd+1:]...)
	}

	// 7. Replace $(TEMPLATE) attribute value with new template name
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

	i := start
	for i < end-1 {
		if strings.TrimSpace(lines[i]) == "0" && strings.TrimSpace(lines[i+1]) == "INSERT" {
			ins := insertEntity{
				startIdx: i,
				attribs:  make(map[string]string),
			}
			// Collect all lines for this INSERT entity (including ATTRIBs)
			// DXF format: code on even lines, value on odd lines
			ins.lines = append(ins.lines, lines[i])   // "0"
			ins.lines = append(ins.lines, lines[i+1]) // "INSERT"

			// Parse code/value pairs within the INSERT entity
			j := i + 2
			inAttrib := false
			attribTag := ""
			attribVal := ""
			attribValLineIdx := -1

			for j < end-1 {
				code := strings.TrimSpace(lines[j])
				val := lines[j+1] // keep original (may have spaces)
				valTrim := strings.TrimSpace(val)

				if code == "0" {
					// End of current entity (ATTRIB or INSERT)
					if inAttrib {
						// Save last ATTRIB
						if attribTag != "" {
							ins.attribs[attribTag] = attribVal
							_ = attribValLineIdx
						}
						if valTrim == "ATTRIB" {
							// Start new ATTRIB
							inAttrib = true
							attribTag = ""
							attribVal = ""
							ins.lines = append(ins.lines, lines[j], lines[j+1])
							j += 2
							continue
						} else {
							// End of INSERT
							inAttrib = false
							break
						}
					} else {
						// Not in ATTRIB — end of INSERT
						break
					}
				}

				// Add lines to entity
				ins.lines = append(ins.lines, lines[j], lines[j+1])

				if inAttrib {
					if code == "2" {
						attribTag = valTrim
					} else if code == "1" {
						attribVal = valTrim
						attribValLineIdx = len(ins.lines) - 1 // index of value line
					}
				} else {
					// In INSERT entity itself
					if code == "2" && ins.blockName == "" {
						ins.blockName = valTrim
					}
				}

				j += 2
			}

			inserts = append(inserts, ins)
			i = j
		} else {
			i++
		}
	}
	return inserts
}

// mergeInsert takes the module INSERT's full entity structure (preserving handles,
// owner refs, AcDbEntity subclass data) and replaces only the INSERT-level geometry
// (position codes 10/20/30, layer code 8, rotation code 50, scale codes 41/42/43)
// with values from the template.
//
// This preserves ALL ATTRIB entity metadata from the module while applying the
// template's block positions/layers/scales.
//
// For template-only INSERTs (no module match), the template's lines are used as-is.
func mergeInsert(tmpl, mod insertEntity) insertEntity {
	// Start with the MODULE's lines (preserves all entity metadata, handles, ATTRIBs)
	merged := insertEntity{
		blockName: mod.blockName,
		lines:     make([]string, len(mod.lines)),
		attribs:   make(map[string]string),
	}
	copy(merged.lines, mod.lines)

	// Copy module attribs
	for k, v := range mod.attribs {
		merged.attribs[k] = v
	}

	// Now replace INSERT-level geometry codes with template values
	// We need to find the INSERT entity's own code/value pairs (before the first ATTRIB)
	// and replace position/layer/scale codes with template values.
	//
	// INSERT-level codes to override from template:
	//   8  = layer
	//   10 = X position
	//   20 = Y position
	//   30 = Z position
	//   50 = rotation
	//   41 = X scale
	//   42 = Y scale
	//   43 = Z scale
	//   62 = color
	//
	// Build a map of template INSERT-level codes (before first ATTRIB)
	tmplInsertCodes := make(map[string]string) // code -> value
	inAttrib := false
	for i := 0; i < len(tmpl.lines)-1; i += 2 {
		code := strings.TrimSpace(tmpl.lines[i])
		if code == "0" {
			val := strings.TrimSpace(tmpl.lines[i+1])
			if val == "ATTRIB" {
				inAttrib = true
				continue
			}
			if inAttrib && val != "ATTRIB" {
				// End of INSERT+ATTRIBs
				break
			}
			// Skip the initial "0 INSERT" pair
			continue
		}
		if inAttrib {
			continue // Skip ATTRIB-level codes
		}
		// This is an INSERT-level code
		val := tmpl.lines[i+1]
		tmplInsertCodes[code] = val
	}

	// Now walk the merged (module) lines and replace INSERT-level codes
	// (only before the first ATTRIB)
	inAttrib = false
	for i := 0; i < len(merged.lines)-1; i += 2 {
		code := strings.TrimSpace(merged.lines[i])
		if code == "0" {
			val := strings.TrimSpace(merged.lines[i+1])
			if val == "ATTRIB" {
				inAttrib = true
				continue
			}
			if inAttrib && val != "ATTRIB" {
				break
			}
			continue
		}
		if inAttrib {
			continue // Don't touch ATTRIB-level codes
		}

		// Replace INSERT-level geometry codes with template values
		geoCodes := map[string]bool{
			"8":  true, // layer
			"10": true, // X
			"20": true, // Y
			"30": true, // Z
			"50": true, // rotation
			"41": true, // X scale
			"42": true, // Y scale
			"43": true, // Z scale
			"62": true, // color
		}
		if geoCodes[code] {
			if tmplVal, ok := tmplInsertCodes[code]; ok {
				merged.lines[i+1] = tmplVal
			}
		}
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