package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// DiffEntity represents a single entity difference for visualization
type DiffEntity struct {
	Type       string     `json:"type"`       // "line", "insert", "lwpolyline", "polyline"
	Status     string     `json:"status"`     // "added", "removed", "modified"
	Coords     []float64  `json:"coords"`     // flattened coordinates [x1,y1,x2,y2,...]
	Coords2D   [][]float64 `json:"coords_2d"`  // [[x,y], [x,y], ...]
	BlockName  string     `json:"block_name"` // for INSERT entities
	Layer      string     `json:"layer"`
}

// DiffResponse is the comparison result between template and module DXF
type DiffResponse struct {
	TemplateFile string       `json:"template_file"`
	ModuleFile   string       `json:"module_file"`
	Template     []*DiffEntity `json:"template_entities"`
	Module       []*DiffEntity `json:"module_entities"`
	Added        []*DiffEntity `json:"added"`     // entities in module but not in template
	Removed      []*DiffEntity `json:"removed"`   // entities in template but not in module
	Modified     []*DiffEntity `json:"modified"`  // entities with changed coordinates
	BoundingBox  [4]float64    `json:"bounding_box"` // [minX, minY, maxX, maxY]
	Summary      DiffSummary   `json:"summary"`
}

type DiffSummary struct {
	TemplateCount int `json:"template_count"`
	ModuleCount   int `json:"module_count"`
	AddedCount    int `json:"added_count"`
	RemovedCount  int `json:"removed_count"`
	ModifiedCount int `json:"modified_count"`
}

// handleDXFDiff compares two DXF files and returns entity-level differences
func (s *Server) handleDXFDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		TemplatePath string `json:"template_path"`
		ModulePath   string `json:"module_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.TemplatePath == "" || req.ModulePath == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_path and module_path are required")
		return
	}

	if _, err := os.Stat(req.TemplatePath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "template file does not exist")
		return
	}
	if _, err := os.Stat(req.ModulePath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "module file does not exist")
		return
	}

	// Parse both files
	templateEntities, err := extractEntities(req.TemplatePath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to parse template: "+err.Error())
		return
	}

	moduleEntities, err := extractEntities(req.ModulePath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to parse module: "+err.Error())
		return
	}

	// Compute bounding box from all entities
	bbox := computeBoundingBox(templateEntities, moduleEntities)

	// Compute diff
	added, removed, modified := computeEntityDiff(templateEntities, moduleEntities)

	resp := &DiffResponse{
		TemplateFile: req.TemplatePath,
		ModuleFile:   req.ModulePath,
		Template:     templateEntities,
		Module:       moduleEntities,
		Added:        added,
		Removed:      removed,
		Modified:     modified,
		BoundingBox:  bbox,
		Summary: DiffSummary{
			TemplateCount: len(templateEntities),
			ModuleCount:   len(moduleEntities),
			AddedCount:    len(added),
			RemovedCount:  len(removed),
			ModifiedCount: len(modified),
		},
	}

	JSONResponse(w, http.StatusOK, resp)
}

// extractEntities parses a DXF file and returns drawable entities with coordinates
func extractEntities(path string) ([]*DiffEntity, error) {
	drawing, err := dxf.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entities []*DiffEntity

	for _, ent := range drawing.Entities {
		typ := strings.TrimSpace(ent.Type)
		switch typ {
		case "LINE":
			sx := ent.GetFloatValue(10)
			sy := ent.GetFloatValue(20)
			ex := ent.GetFloatValue(11)
			ey := ent.GetFloatValue(21)
			layer := ent.GetStringValue(8)
			e := &DiffEntity{
				Type:      "line",
				Status:    "same",
				Layer:     layer,
				Coords:    []float64{sx, sy, ex, ey},
				Coords2D:  [][]float64{{sx, sy}, {ex, ey}},
			}
			entities = append(entities, e)

		case "LWPOLYLINE":
			layer := ent.GetStringValue(8)
			e := &DiffEntity{
				Type:   "lwpolyline",
				Status: "same",
				Layer:  layer,
			}
			var curX, curY float64
			xCount, yCount := 0, 0
			for _, p := range ent.Pairs {
				switch p.Code {
				case 10:
					curX = parseFloatStr(p.Value)
					xCount++
				case 20:
					curY = parseFloatStr(p.Value)
					yCount++
					if xCount == yCount {
						e.Coords = append(e.Coords, curX, curY)
						e.Coords2D = append(e.Coords2D, []float64{curX, curY})
					}
				}
			}
			entities = append(entities, e)

		case "POLYLINE":
			layer := ent.GetStringValue(8)
			e := &DiffEntity{
				Type:   "polyline",
				Status: "same",
				Layer:  layer,
			}
			var curX, curY float64
			headerOriginSkipped := false
			for _, p := range ent.Pairs {
				switch p.Code {
				case 10:
					curX = parseFloatStr(p.Value)
				case 20:
					curY = parseFloatStr(p.Value)
				case 30:
					if !headerOriginSkipped {
						headerOriginSkipped = true
					} else {
						e.Coords = append(e.Coords, curX, curY)
						e.Coords2D = append(e.Coords2D, []float64{curX, curY})
					}
				}
			}
			entities = append(entities, e)

		case "INSERT":
			blockName := ent.GetStringValue(2)
			if blockName == "COMPANY" || blockName == "CUSTOMER" {
				continue
			}
			ix := ent.GetFloatValue(10)
			iy := ent.GetFloatValue(20)
			layer := ent.GetStringValue(8)
			e := &DiffEntity{
				Type:      "insert",
				Status:    "same",
				BlockName: blockName,
				Layer:     layer,
				Coords:    []float64{ix, iy},
				Coords2D:  [][]float64{{ix, iy}},
			}
			entities = append(entities, e)
		}
	}

	return entities, nil
}

// parseFloatStr parses a float string
func parseFloatStr(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// computeEntityDiff finds added, removed, and modified entities
func computeEntityDiff(template, module []*DiffEntity) (added, removed, modified []*DiffEntity) {
	// Build lookup maps by entity signature (type + rounded coords)
	tMap := make(map[string]*DiffEntity)
	mMap := make(map[string]*DiffEntity)

	for _, e := range template {
		key := entityKey(e)
		tMap[key] = e
	}
	for _, e := range module {
		key := entityKey(e)
		mMap[key] = e
	}

	// Find removed (in template, not in module)
	for key, e := range tMap {
		if _, exists := mMap[key]; !exists {
			e.Status = "removed"
			removed = append(removed, e)
		}
	}

	// Find added (in module, not in template)
	for key, e := range mMap {
		if _, exists := tMap[key]; !exists {
			e.Status = "added"
			added = append(added, e)
		}
	}

	return added, removed, modified
}

// entityKey generates a hash-like key for an entity based on type + rounded coords
func entityKey(e *DiffEntity) string {
	var sb strings.Builder
	sb.WriteString(e.Type)
	sb.WriteString(":")
	sb.WriteString(e.BlockName)
	for _, c := range e.Coords {
		// Round to 2 decimal places to handle minor float differences
		sb.WriteString(strconv.FormatFloat(c, 'f', 2, 64))
		sb.WriteString(",")
	}
	return sb.String()
}

// computeBoundingBox returns [minX, minY, maxX, maxY] across all entities
func computeBoundingBox(entitySets ...[]*DiffEntity) [4]float64 {
	bbox := [4]float64{1e18, 1e18, -1e18, -1e18}
	hasPoint := false

	for _, entities := range entitySets {
		for _, e := range entities {
			for i := 0; i+1 < len(e.Coords); i += 2 {
				x, y := e.Coords[i], e.Coords[i+1]
				if x < bbox[0] {
					bbox[0] = x
				}
				if y < bbox[1] {
					bbox[1] = y
				}
				if x > bbox[2] {
					bbox[2] = x
				}
				if y > bbox[3] {
					bbox[3] = y
				}
				hasPoint = true
			}
		}
	}

	if !hasPoint {
		return [4]float64{0, 0, 100, 100}
	}
	return bbox
}

// handleCreateTemplate creates a new template from a mod folder file
func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		SourceFile      string `json:"source_file"`       // DXF file to use as new template
		TemplateFolder  string `json:"template_folder"`   // Where to save the new template
		TemplateName    string `json:"template_name"`     // Name for the new template (optional, defaults to filename)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.SourceFile == "" {
		ErrorResponse(w, http.StatusBadRequest, "source_file is required")
		return
	}
	if _, err := os.Stat(req.SourceFile); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "source file does not exist")
		return
	}

	// Determine template folder
	templateFolder := req.TemplateFolder
	if templateFolder == "" {
		templateFolder = s.settings.TemplateFolder
	}
	if templateFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_folder is required")
		return
	}

	// Determine template name
	templateName := req.TemplateName
	if templateName == "" {
		templateName = filepath.Base(req.SourceFile)
	}

	// Copy the file to the template folder
	destPath := filepath.Join(templateFolder, templateName)

	// Read source
	data, err := os.ReadFile(req.SourceFile)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to read source file: "+err.Error())
		return
	}

	// Write to template folder
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to write template: "+err.Error())
		return
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":       true,
		"message":  "template created",
		"path":     destPath,
		"name":     templateName,
	})
}