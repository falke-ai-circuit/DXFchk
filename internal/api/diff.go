package api

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// DiffEntity represents a single entity difference for visualization
type DiffEntity struct {
	Type       string     `json:"type"`       // "line", "insert", "lwpolyline", "polyline", "text", "circle", "arc", "point"
	Status     string     `json:"status"`     // "added", "removed", "modified", "same"
	Coords     []float64  `json:"coords"`     // flattened coordinates [x1,y1,x2,y2,...]
	Coords2D   [][]float64 `json:"coords_2d"`  // [[x,y], [x,y], ...]
	BlockName  string     `json:"block_name"` // for INSERT entities: block name; for TEXT: the text content
	Layer      string     `json:"layer"`
	Color      int        `json:"color"`      // ACI color index (0=ByBlock, 256=ByLayer)
	Rotation    float64   `json:"rotation"`   // rotation angle in degrees
	ScaleX     float64    `json:"scale_x"`    // INSERT scale X
	ScaleY     float64    `json:"scale_y"`    // INSERT scale Y
	HAlign     int        `json:"h_align"`    // text horizontal alignment (72)
	VAlign     int        `json:"v_align"`    // text vertical alignment (73)
	TextHeight float64    `json:"text_height"` // text height (code 40)
	Bulges     []float64  `json:"bulges"`     // LWPOLYLINE bulge values per vertex
	Closed     bool       `json:"closed"`     // LWPOLYLINE closed flag
	// Block rendering data (for INSERT entities)
	BlockEntities []*DiffEntity `json:"block_entities,omitempty"` // entities from block definition
	BlockBaseX    float64       `json:"block_base_x,omitempty"`
	BlockBaseY    float64       `json:"block_base_y,omitempty"`
	// ATTRIB data for INSERT entities (block attributes — terminal names, values, formulas)
	Attribs []DiffAttrib `json:"attribs,omitempty"`
}

// DiffAttrib represents a block attribute (ATTRIB entity inside an INSERT)
type DiffAttrib struct {
	Tag    string  `json:"tag"`     // ATTRIB tag (code 2) — e.g. "INPUT_PULSE", "OPERATOR"
	Text   string  `json:"text"`    // ATTRIB text value (code 1) — e.g. "602.5000.0006"
	X      float64 `json:"x"`        // ATTRIB insertion point X (code 10)
	Y      float64 `json:"y"`        // ATTRIB insertion point Y (code 20)
	Height float64 `json:"height"`   // text height (code 40)
	Rotation float64 `json:"rotation"` // rotation angle in degrees (code 50)
	HAlign int     `json:"h_align"`  // horizontal alignment (code 72)
	VAlign int     `json:"v_align"`  // vertical alignment (code 73)
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
	return extractFromDrawing(drawing), nil
}

// extractFromDrawing converts parsed DXF drawing to DiffEntity list
func extractFromDrawing(drawing *dxf.Drawing) []*DiffEntity {
	var entities []*DiffEntity

	for _, ent := range drawing.Entities {
		typ := strings.TrimSpace(ent.Type)
		layer := ent.GetStringValue(8)
		color := ent.GetIntValue(62) // 0=ByBlock, 256=ByLayer, absent=ByLayer
		if color == 0 && !hasCode(ent.Pairs, 62) {
			color = 256 // default ByLayer when code 62 absent
		}

		switch typ {
		case "LINE":
			sx := ent.GetFloatValue(10)
			sy := ent.GetFloatValue(20)
			ex := ent.GetFloatValue(11)
			ey := ent.GetFloatValue(21)
			e := &DiffEntity{
				Type:   "line",
				Status: "same",
				Layer:  layer,
				Color:  color,
				Coords: []float64{sx, sy, ex, ey},
			}
			entities = append(entities, e)

		case "LWPOLYLINE":
			e := &DiffEntity{
				Type:   "lwpolyline",
				Status: "same",
				Layer:  layer,
				Color:  color,
				Closed: ent.GetIntValue(70)&1 != 0,
			}
			var curX, curY float64
			for _, p := range ent.Pairs {
				switch p.Code {
				case 10:
					curX = parseFloatStr(p.Value)
				case 20:
					curY = parseFloatStr(p.Value)
					e.Coords = append(e.Coords, curX, curY)
					e.Coords2D = append(e.Coords2D, []float64{curX, curY})
				case 42:
					e.Bulges = append(e.Bulges, parseFloatStr(p.Value))
				}
			}
			// Pad bulges to match vertex count
			for len(e.Bulges) < len(e.Coords2D) {
				e.Bulges = append(e.Bulges, 0)
			}
			entities = append(entities, e)

		case "POLYLINE":
			e := &DiffEntity{
				Type:   "polyline",
				Status: "same",
				Layer:  layer,
				Color:  color,
				Closed: ent.GetIntValue(70)&1 != 0,
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
				case 42:
					e.Bulges = append(e.Bulges, parseFloatStr(p.Value))
				}
			}
			for len(e.Bulges) < len(e.Coords2D) {
				e.Bulges = append(e.Bulges, 0)
			}
			entities = append(entities, e)

		case "INSERT":
			blockName := ent.GetStringValue(2)
			if blockName == "COMPANY" || blockName == "CUSTOMER" {
				continue
			}
			ix := ent.GetFloatValue(10)
			iy := ent.GetFloatValue(20)
			rot := ent.GetFloatValue(50) * 180 / math.Pi // radians → degrees
			sx := ent.GetFloatValue(41)
			sy := ent.GetFloatValue(42)
			if sx == 0 {
				sx = 1
			}
			if sy == 0 {
				sy = 1
			}
			e := &DiffEntity{
				Type:      "insert",
				Status:    "same",
				BlockName: blockName,
				Layer:     layer,
				Color:     color,
				Coords:    []float64{ix, iy},
				Coords2D:  [][]float64{{ix, iy}},
				Rotation:  rot,
				ScaleX:    sx,
				ScaleY:    sy,
			}
			// Extract ATTRIB entities (block attributes — terminal names, values, formulas)
			for _, att := range ent.Attribs {
				// Check invisible flag (code 70 bit 0x80 = invisible) — skip invisible ATTRIBs
				attFlags := att.GetIntValue(70)
				if attFlags&0x80 != 0 {
					continue
				}
				attTag := att.GetStringValue(2)
				attText := stripMTextFormatting(att.GetStringValue(1))
				if attText == "" {
					continue
				}
				// Skip structural label markers (not user-visible in CAD)
				attTagUpper := strings.ToUpper(attTag)
				if attTagUpper == "LABEL_UP" || attTagUpper == "LABEL_RIGHT" ||
					attTagUpper == "LABEL_DOWN" || attTagUpper == "LABEL_LEFT" {
					continue
				}
				// Skip ATTRIBs where tag looks like a coordinate or internal code
				// (tags like "0,0@0" or "0,100.0" are coordinate echoes, not labels)
				if strings.HasPrefix(attTag, "0,") || strings.HasPrefix(attText, "0,") {
					// Allow if the text looks like a real value (has letters)
					if !containsLetter(attText) && !containsLetter(attTag) {
						continue
					}
				}
				ax := att.GetFloatValue(10)
				ay := att.GetFloatValue(20)
				ah := att.GetFloatValue(40)
				if ah == 0 {
					ah = 2.5
				}
				ar := att.GetFloatValue(50) * 180 / math.Pi
				ahAlign := att.GetIntValue(72)
				avAlign := att.GetIntValue(73)
				// Use alignment point if present
				if ahAlign != 0 || avAlign != 0 {
					ax2 := att.GetFloatValue(11)
					ay2 := att.GetFloatValue(21)
					if ax2 != 0 || ay2 != 0 {
						ax = ax2
						ay = ay2
					}
				}
				e.Attribs = append(e.Attribs, DiffAttrib{
					Tag: attTag, Text: attText,
					X: ax, Y: ay, Height: ah, Rotation: ar,
					HAlign: ahAlign, VAlign: avAlign,
				})
			}
			// Resolve block entities from BLOCKS section
			if block, ok := drawing.Blocks[strings.ToUpper(blockName)]; ok {
				e.BlockBaseX = block.BaseX
				e.BlockBaseY = block.BaseY
				blockEnts := extractFromEntities(block.Entities, drawing)
				e.BlockEntities = blockEnts
			}
			entities = append(entities, e)

		case "TEXT", "MTEXT":
			tx := ent.GetFloatValue(10)
			ty := ent.GetFloatValue(20)
			text := stripMTextFormatting(ent.GetStringValue(1))
			height := ent.GetFloatValue(40)
			if height == 0 {
				height = 2.5
			}
			rot := ent.GetFloatValue(50) * 180 / math.Pi
			hAlign := ent.GetIntValue(72)
			vAlign := ent.GetIntValue(73)
			// Alignment point (code 11/21) used when hAlign or vAlign nonzero
			if hAlign != 0 || vAlign != 0 {
				ax := ent.GetFloatValue(11)
				ay := ent.GetFloatValue(21)
				if ax != 0 || ay != 0 {
					tx = ax
					ty = ay
				}
			}
			// For MTEXT, also check code 11/21/31 for insertion point
			if typ == "MTEXT" {
				ix := ent.GetFloatValue(10)
				iy := ent.GetFloatValue(20)
				if ix != 0 || iy != 0 {
					tx = ix
					ty = iy
				}
			}
			e := &DiffEntity{
				Type:       "text",
				Status:     "same",
				Layer:      layer,
				Color:      color,
				BlockName:  text,
				Coords:     []float64{tx, ty},
				Coords2D:   [][]float64{{tx, ty}},
				Rotation:   rot,
				TextHeight: height,
				HAlign:     hAlign,
				VAlign:     vAlign,
			}
			entities = append(entities, e)

		case "CIRCLE":
			cx := ent.GetFloatValue(10)
			cy := ent.GetFloatValue(20)
			radius := ent.GetFloatValue(40)
			e := &DiffEntity{
				Type:   "circle",
				Status: "same",
				Layer:  layer,
				Color:  color,
				Coords: []float64{cx, cy, radius},
			}
			entities = append(entities, e)

		case "ARC":
			cx := ent.GetFloatValue(10)
			cy := ent.GetFloatValue(20)
			radius := ent.GetFloatValue(40)
			startAng := ent.GetFloatValue(50) * math.Pi / 180
			endAng := ent.GetFloatValue(51) * math.Pi / 180
			segments := 32
			var coords []float64
			for i := 0; i <= segments; i++ {
				t := float64(i) / float64(segments)
				ang := startAng + t*(endAng-startAng)
				x := cx + radius*math.Cos(ang)
				y := cy + radius*math.Sin(ang)
				coords = append(coords, x, y)
			}
			e := &DiffEntity{
				Type:   "arc",
				Status: "same",
				Layer:  layer,
				Color:  color,
				Coords: coords,
			}
			entities = append(entities, e)

		case "POINT":
			px := ent.GetFloatValue(10)
			py := ent.GetFloatValue(20)
			e := &DiffEntity{
				Type:   "point",
				Status: "same",
				Layer:  layer,
				Color:  color,
				Coords: []float64{px, py},
			}
			entities = append(entities, e)
		}
	}

	return entities
}

// extractFromEntities converts raw Entity list (from block definitions) to DiffEntity
func extractFromEntities(ents []dxf.Entity, drawing *dxf.Drawing) []*DiffEntity {
	var result []*DiffEntity
	for _, ent := range ents {
		typ := strings.TrimSpace(ent.Type)
		layer := ent.GetStringValue(8)
		color := ent.GetIntValue(62)
		if color == 0 && !hasCode(ent.Pairs, 62) {
			color = 256
		}

		switch typ {
		case "LINE":
			sx := ent.GetFloatValue(10)
			sy := ent.GetFloatValue(20)
			ex := ent.GetFloatValue(11)
			ey := ent.GetFloatValue(21)
			result = append(result, &DiffEntity{
				Type: "line", Status: "same", Layer: layer, Color: color,
				Coords: []float64{sx, sy, ex, ey},
			})
		case "LWPOLYLINE":
			e := &DiffEntity{
				Type: "lwpolyline", Status: "same", Layer: layer, Color: color,
				Closed: ent.GetIntValue(70)&1 != 0,
			}
			var curX, curY float64
			for _, p := range ent.Pairs {
				if p.Code == 10 {
					curX = parseFloatStr(p.Value)
				} else if p.Code == 20 {
					curY = parseFloatStr(p.Value)
					e.Coords = append(e.Coords, curX, curY)
					e.Coords2D = append(e.Coords2D, []float64{curX, curY})
				} else if p.Code == 42 {
					e.Bulges = append(e.Bulges, parseFloatStr(p.Value))
				}
			}
			for len(e.Bulges) < len(e.Coords2D) {
				e.Bulges = append(e.Bulges, 0)
			}
			result = append(result, e)
		case "POLYLINE":
			e := &DiffEntity{
				Type: "polyline", Status: "same", Layer: layer, Color: color,
				Closed: ent.GetIntValue(70)&1 != 0,
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
				case 42:
					e.Bulges = append(e.Bulges, parseFloatStr(p.Value))
				}
			}
			for len(e.Bulges) < len(e.Coords2D) {
				e.Bulges = append(e.Bulges, 0)
			}
			result = append(result, e)
		case "TEXT", "MTEXT":
			tx := ent.GetFloatValue(10)
			ty := ent.GetFloatValue(20)
			text := stripMTextFormatting(ent.GetStringValue(1))
			height := ent.GetFloatValue(40)
			if height == 0 {
				height = 2.5
			}
			rot := ent.GetFloatValue(50) * 180 / math.Pi
			hAlign := ent.GetIntValue(72)
			vAlign := ent.GetIntValue(73)
			if hAlign != 0 || vAlign != 0 {
				ax := ent.GetFloatValue(11)
				ay := ent.GetFloatValue(21)
				if ax != 0 || ay != 0 {
					tx = ax
					ty = ay
				}
			}
			// For MTEXT, also check code 11/21/31 for insertion point
			if typ == "MTEXT" {
				ix := ent.GetFloatValue(10)
				iy := ent.GetFloatValue(20)
				if ix != 0 || iy != 0 {
					tx = ix
					ty = iy
				}
			}
			result = append(result, &DiffEntity{
				Type: "text", Status: "same", Layer: layer, Color: color,
				BlockName: text, Coords: []float64{tx, ty},
				Rotation: rot, TextHeight: height,
				HAlign: hAlign, VAlign: vAlign,
			})
		case "CIRCLE":
			cx := ent.GetFloatValue(10)
			cy := ent.GetFloatValue(20)
			radius := ent.GetFloatValue(40)
			result = append(result, &DiffEntity{
				Type: "circle", Status: "same", Layer: layer, Color: color,
				Coords: []float64{cx, cy, radius},
			})
		case "ARC":
			cx := ent.GetFloatValue(10)
			cy := ent.GetFloatValue(20)
			radius := ent.GetFloatValue(40)
			startAng := ent.GetFloatValue(50) * math.Pi / 180
			endAng := ent.GetFloatValue(51) * math.Pi / 180
			segments := 32
			var coords []float64
			for i := 0; i <= segments; i++ {
				t := float64(i) / float64(segments)
				ang := startAng + t*(endAng-startAng)
				coords = append(coords, cx+radius*math.Cos(ang), cy+radius*math.Sin(ang))
			}
			result = append(result, &DiffEntity{
				Type: "arc", Status: "same", Layer: layer, Color: color,
				Coords: coords,
			})
		}
	}
	return result
}

// hasCode checks if a code exists in pairs
func hasCode(pairs []dxf.CodePair, code int) bool {
	for _, p := range pairs {
		if p.Code == code {
			return true
		}
	}
	return false
}

// parseFloatStr parses a float string
func parseFloatStr(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// containsLetter checks if a string contains any ASCII letter
func containsLetter(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

// stripMTextFormatting removes MTEXT formatting codes from text strings
// MTEXT can contain formatting codes like \P (paragraph), \S (stacking),
// \f (font), \C (color), \H (height), \W (width), \Q (obliquing),
// \T (tracking), \A (alignment), {\...} groups, etc.
func stripMTextFormatting(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Remove common MTEXT control sequences
	// \P - paragraph break → space
	s = strings.ReplaceAll(s, "\\P", " ")
	// \~ - non-breaking space → space
	s = strings.ReplaceAll(s, "\\~", " ")
	// \\ - escaped backslash → backslash
	s = strings.ReplaceAll(s, "\\\\", "\\")
	// Remove {\fArial|b0|i0|c0|p34; ...} style formatting groups
	// Simple approach: remove \f...; sequences
	for {
		idx := strings.Index(s, "\\f")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \C+number; (color)
	for {
		idx := strings.Index(s, "\\C")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \H+number; (height) and \H+numberx; (relative height)
	for {
		idx := strings.Index(s, "\\H")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \W+number; (width)
	for {
		idx := strings.Index(s, "\\W")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \Q+number; (obliquing)
	for {
		idx := strings.Index(s, "\\Q")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \T+number; (tracking)
	for {
		idx := strings.Index(s, "\\T")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \A+number; (alignment)
	for {
		idx := strings.Index(s, "\\A")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		s = s[:idx] + s[idx+end+1:]
	}
	// Remove \S...; (stacking/fractions) — keep content before /
	for {
		idx := strings.Index(s, "\\S")
		if idx < 0 {
			break
		}
		end := strings.Index(s[idx:], ";")
		if end < 0 {
			break
		}
		// Keep content but remove the stacking syntax
		content := s[idx+2 : idx+end]
		content = strings.ReplaceAll(content, "/", " ")
		content = strings.ReplaceAll(content, "#", " ")
		s = s[:idx] + content + s[idx+end+1:]
	}
	// Remove brace groups { ... } but keep content
	for {
		idx := strings.Index(s, "{")
		if idx < 0 {
			break
		}
		// Find matching closing brace
		depth := 1
		j := idx + 1
		for j < len(s) && depth > 0 {
			if s[j] == '{' {
				depth++
			} else if s[j] == '}' {
				depth--
			}
			if depth > 0 {
				j++
			}
		}
		if depth > 0 {
			break
		}
		inner := s[idx+1 : j]
		// Check if inner starts with a backslash formatting code
		if len(inner) > 0 && inner[0] == '\\' {
			// Find content after the formatting code (after first ;)
			semiIdx := strings.Index(inner, ";")
			if semiIdx >= 0 {
				inner = inner[semiIdx+1:]
			} else {
				inner = ""
			}
		}
		s = s[:idx] + inner + s[j+1:]
	}
	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
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