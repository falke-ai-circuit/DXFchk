package dxf

import (
	"fmt"
	"os"
	"strings"
)

// PreviewResult shows what would change when applying a template to a module.
// It categorizes changes as:
//   - Structural: blocks added/removed (e.g., extra I/O block, missing label)
//   - Positional: blocks moved (same block_name, different x/y coordinates)
//   - LayerChange: blocks on different layers
//   - AttributeChange: ATTRIB values differ (expected — these are design member variables)
type PreviewResult struct {
	TemplateInserts int              `json:"template_inserts"`
	ModuleInserts   int              `json:"module_inserts"`
	Matched         int              `json:"matched"`
	AddedFromTemplate int            `json:"added_from_template"`
	RemovedFromModule int            `json:"removed_from_module"`
	PositionChanged  int            `json:"position_changed"`
	LayerChanged     int            `json:"layer_changed"`
	AttributeChanged int            `json:"attribute_changed"`
	Details         []PreviewDetail  `json:"details"`
	Summary         string           `json:"summary"`
	CanApply        bool            `json:"can_apply"`
	Warning         string          `json:"warning,omitempty"`
}

// PreviewDetail describes one specific change
type PreviewDetail struct {
	Type        string `json:"type"`         // "added", "removed", "moved", "layer_change", "attrib_change"
	BlockName   string `json:"block_name"`
	TemplatePos string `json:"template_pos,omitempty"`
	ModulePos   string `json:"module_pos,omitempty"`
	TemplateLayer string `json:"template_layer,omitempty"`
	ModuleLayer   string `json:"module_layer,omitempty"`
	Description string `json:"description"`
}

// PreviewTemplate compares a template to a module and returns what would change.
// This is a read-only operation — it does NOT modify any files.
func PreviewTemplate(templatePath, modulePath string) (PreviewResult, error) {
	var result PreviewResult

	tmplData, err := os.ReadFile(templatePath)
	if err != nil {
		return result, fmt.Errorf("reading template: %w", err)
	}
	modData, err := os.ReadFile(modulePath)
	if err != nil {
		return result, fmt.Errorf("reading module: %w", err)
	}

	lineEnding := "\r\n"
	if !strings.Contains(string(tmplData), "\r\n") {
		lineEnding = "\n"
	}

	tmplLines := splitLines(string(tmplData), lineEnding)
	modLines := splitLines(string(modData), lineEnding)

	tmplEntStart, tmplEntEnd := findEntitiesSection(tmplLines)
	modEntStart, modEntEnd := findEntitiesSection(modLines)

	if tmplEntStart < 0 || modEntStart < 0 {
		return result, fmt.Errorf("could not find ENTITIES section")
	}

	tmplInserts := extractInsertEntities(tmplLines, tmplEntStart, tmplEntEnd)
	modInserts := extractInsertEntities(modLines, modEntStart, modEntEnd)

	result.TemplateInserts = len(tmplInserts)
	result.ModuleInserts = len(modInserts)

	// Build queues by block_name
	modByBlock := make(map[string][]insertEntity)
	for _, ins := range modInserts {
		modByBlock[ins.blockName] = append(modByBlock[ins.blockName], ins)
	}

	// Compare
	var details []PreviewDetail
	matched := 0
	posChanged := 0
	layerChanged := 0
	attribChanged := 0

	for _, tIns := range tmplInserts {
		queue := modByBlock[tIns.blockName]
		if len(queue) > 0 {
			mIns := queue[0]
			modByBlock[tIns.blockName] = queue[1:]
			matched++

			// Compare positions
			tPos := getInsertPos(tIns)
			mPos := getInsertPos(mIns)
			if tPos != mPos {
				posChanged++
				details = append(details, PreviewDetail{
					Type:        "moved",
					BlockName:   tIns.blockName,
					TemplatePos: tPos,
					ModulePos:   mPos,
					Description: fmt.Sprintf("Block %s moved from %s to %s", tIns.blockName, mPos, tPos),
				})
			}

			// Compare layers
			tLayer := getInsertLayer(tIns)
			mLayer := getInsertLayer(mIns)
			if tLayer != mLayer {
				layerChanged++
				details = append(details, PreviewDetail{
					Type:          "layer_change",
					BlockName:     tIns.blockName,
					TemplateLayer: tLayer,
					ModuleLayer:   mLayer,
					Description:   fmt.Sprintf("Block %s layer changed from %s to %s", tIns.blockName, mLayer, tLayer),
				})
			}

			// Compare ATTRIBs
			if !attribsEqual(tIns.attribs, mIns.attribs) {
				attribChanged++
				// Find which specific attribs differ
				for tag, tVal := range tIns.attribs {
					mVal, exists := mIns.attribs[tag]
					if !exists || mVal != tVal {
						details = append(details, PreviewDetail{
							Type:      "attrib_change",
							BlockName:  tIns.blockName,
							Description: fmt.Sprintf("Block %s attr %s: template=%q module=%q", tIns.blockName, tag, tVal, mVal),
						})
					}
				}
				for tag, mVal := range mIns.attribs {
					if _, exists := tIns.attribs[tag]; !exists {
						details = append(details, PreviewDetail{
							Type:      "attrib_change",
							BlockName:  tIns.blockName,
							Description: fmt.Sprintf("Block %s attr %s: only in module=%q", tIns.blockName, tag, mVal),
						})
					}
				}
			}
		} else {
			// Added from template
			result.AddedFromTemplate++
			details = append(details, PreviewDetail{
				Type:      "added",
				BlockName: tIns.blockName,
				Description: fmt.Sprintf("Block %s added (exists in template, not in module)", tIns.blockName),
			})
		}
	}

	// Count removed (module INSERTs not matched)
	for _, queue := range modByBlock {
		for _, ins := range queue {
			result.RemovedFromModule++
			details = append(details, PreviewDetail{
				Type:      "removed",
				BlockName: ins.blockName,
				Description: fmt.Sprintf("Block %s removed (exists in module, not in template)", ins.blockName),
			})
		}
	}

	result.Matched = matched
	result.PositionChanged = posChanged
	result.LayerChanged = layerChanged
	result.AttributeChanged = attribChanged
	result.Details = details

	// Build summary and determine if safe to apply
	structuralChanges := result.AddedFromTemplate + result.RemovedFromModule
	result.CanApply = true

	if structuralChanges > 0 {
		result.Summary = fmt.Sprintf("%d blocks added, %d removed, %d moved, %d layer changes, %d attrib changes",
			result.AddedFromTemplate, result.RemovedFromModule, posChanged, layerChanged, attribChanged)
	} else if posChanged > 0 || layerChanged > 0 {
		result.Summary = fmt.Sprintf("No structural changes — %d blocks moved, %d layer changes, %d attrib changes",
			posChanged, layerChanged, attribChanged)
	} else {
		result.Summary = fmt.Sprintf("Only attribute changes (%d) — block structure is identical", attribChanged)
	}

	// Warn if large structural changes (might indicate wrong template)
	if structuralChanges > 20 {
		result.Warning = fmt.Sprintf("Large structural change (%d blocks added/removed). Verify this is the correct template for this module group.", structuralChanges)
	}

	return result, nil
}

// getInsertPos extracts the x,y coordinates as a string "x,y"
func getInsertPos(ins insertEntity) string {
	x := ""
	y := ""
	for i, line := range ins.lines {
		code := strings.TrimSpace(line)
		if code == "10" && i+1 < len(ins.lines) {
			x = strings.TrimSpace(ins.lines[i+1])
		}
		if code == "20" && i+1 < len(ins.lines) {
			y = strings.TrimSpace(ins.lines[i+1])
		}
		if code == "0" && i > 0 {
			break
		}
	}
	// Normalize: parse as float and format back to avoid "0.0" vs "0" mismatches
	return normalizeFloat(x) + "," + normalizeFloat(y)
}

// getInsertLayer extracts the layer name
func getInsertLayer(ins insertEntity) string {
	for i, line := range ins.lines {
		code := strings.TrimSpace(line)
		if code == "8" && i+1 < len(ins.lines) {
			return strings.TrimSpace(ins.lines[i+1])
		}
		if code == "0" && i > 0 {
			break
		}
	}
	return ""
}

// normalizeFloat parses a float string and returns a normalized form
func normalizeFloat(s string) string {
	// Try to parse as float and format back
	// This handles "0.0" vs "0" and "580.0" vs "580"
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err == nil {
		return fmt.Sprintf("%g", f)
	}
	return s
}

// attribsEqual compares two attrib maps
func attribsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}