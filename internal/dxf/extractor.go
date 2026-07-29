package dxf

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// DXFContent represents the extracted geometry from a DXF file
// This is the Go equivalent of the Python get_blocks_lines_polylines_from_dxf()
type DXFContent struct {
	Blocks    map[string][][3]float64     // block_name → list of positions
	Lines     map[string][][2][3]float64 // layer → list of (start, end) sorted pairs
	Polylines map[string][][][3]float64   // layer → list of polylines (each polyline = list of vertices)
	Template  string                      // $(TEMPLATE) attribute value
}

// ExtractContent extracts blocks, lines, and polylines from a Drawing.
// This is a direct port of the Python get_blocks_lines_polylines_from_dxf().
//
// - Blocks (INSERT): excludes "COMPANY" and "CUSTOMER" block names
// - Lines (LINE): endpoints sorted so (A->B) equals (B->A)
// - Polylines (LWPOLYLINE/POLYLINE): vertices normalized so reversal = same
// - All coordinates rounded to N decimal places
func (d *Drawing) ExtractContent(decimals int) *DXFContent {
	content := &DXFContent{
		Blocks:    make(map[string][][3]float64),
		Lines:     make(map[string][][2][3]float64),
		Polylines: make(map[string][][][3]float64),
	}
	content.Template = d.GetTemplateAttribute()

	round := func(v float64) float64 {
		pow := math.Pow(10, float64(decimals))
		return math.Round(v*pow) / pow
	}

	for _, e := range d.Entities {
		switch strings.TrimSpace(e.Type) {
		case "INSERT":
			blockName := e.GetStringValue(2)
			if blockName == "COMPANY" || blockName == "CUSTOMER" {
				continue
			}
			x := round(e.GetFloatValue(10))
			y := round(e.GetFloatValue(20))
			z := round(e.GetFloatValue(30))
			content.Blocks[blockName] = append(content.Blocks[blockName], [3]float64{x, y, z})

		case "LINE":
			layer := e.GetStringValue(8)
			sx := round(e.GetFloatValue(10))
			sy := round(e.GetFloatValue(20))
			sz := round(e.GetFloatValue(30))
			ex := round(e.GetFloatValue(11))
			ey := round(e.GetFloatValue(21))
			ez := round(e.GetFloatValue(31))
			start := [3]float64{sx, sy, sz}
			end := [3]float64{ex, ey, ez}
			// Sort endpoints so (A->B) equals (B->A) — matches Python sorted([start, end])
			sorted := [2][3]float64{start, end}
			if start[0] > end[0] || (start[0] == end[0] && start[1] > end[1]) ||
				(start[0] == end[0] && start[1] == end[1] && start[2] > end[2]) {
				sorted = [2][3]float64{end, start}
			}
			content.Lines[layer] = append(content.Lines[layer], sorted)

		case "LWPOLYLINE":
			layer := e.GetStringValue(8)
			// Collect vertices from repeated 10/20 pairs
			var vertices [][3]float64
			xCount := 0
			yCount := 0
			var lastX, lastY float64
			for _, p := range e.Pairs {
				switch p.Code {
				case 10:
					lastX = round(parseFloat(p.Value))
					xCount++
				case 20:
					lastY = round(parseFloat(p.Value))
					yCount++
					if xCount == yCount {
						vertices = append(vertices, [3]float64{lastX, lastY, 0.0})
					}
				}
			}
			normalized := normalizeVertices(vertices)
			// Store as one polyline entry (like Python appends the tuple)
			content.Polylines[layer] = append(content.Polylines[layer], normalized)

		case "POLYLINE":
			layer := e.GetStringValue(8)
			// POLYLINE entities have a header with codes 10/20/30 for the origin point
			// (usually 0,0,0), followed by VERTEX entities with actual vertex coordinates.
			// The parser collects VERTEX pairs into the POLYLINE's Pairs, so we need to
			// skip the header's 10/20/30 and only process the VERTEX 10/20/30 triples.
			var vertices [][3]float64
			var curX, curY, curZ float64
			vertexCount := 0
			headerOriginSkipped := false
			for _, p := range e.Pairs {
				switch p.Code {
				case 10:
					curX = round(parseFloat(p.Value))
				case 20:
					curY = round(parseFloat(p.Value))
				case 30:
					curZ = round(parseFloat(p.Value))
					if !headerOriginSkipped {
						// Skip the first 10/20/30 triple (POLYLINE header origin, usually 0,0,0)
						headerOriginSkipped = true
					} else {
						vertices = append(vertices, [3]float64{curX, curY, curZ})
						vertexCount++
					}
				}
			}
			// If no vertices were found (no 30 codes after header), try without skipping
			if vertexCount == 0 {
				vertices = vertices[:0]
				for _, p := range e.Pairs {
					if p.Code == 10 {
						curX = round(parseFloat(p.Value))
						vertices = append(vertices, [3]float64{curX, 0.0, 0.0})
					}
				}
			}
			normalized := normalizeVertices(vertices)
			content.Polylines[layer] = append(content.Polylines[layer], normalized)
		}
	}

	return content
}

// GetTemplateAttribute extracts the $(TEMPLATE) attribute value from INSERT entities
func (d *Drawing) GetTemplateAttribute() string {
	for _, e := range d.Entities {
		if strings.TrimSpace(e.Type) != "INSERT" {
			continue
		}
		val := e.GetAttribValue("$(TEMPLATE)")
		if val != "" {
			return val
		}
	}
	return ""
}

// normalizeVertices normalizes polyline vertices so reversal = same
// Port of Python: min(tup_points, tup_rev)
func normalizeVertices(vertices [][3]float64) [][3]float64 {
	if len(vertices) < 2 {
		return vertices
	}
	rev := make([][3]float64, len(vertices))
	for i, v := range vertices {
		rev[len(vertices)-1-i] = v
	}
	// Compare lexicographically
	if compareVertices(vertices, rev) <= 0 {
		return vertices
	}
	return rev
}

func compareVertices(a, b [][3]float64) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		for j := 0; j < 3; j++ {
			if a[i][j] < b[i][j] {
				return -1
			}
			if a[i][j] > b[i][j] {
				return 1
			}
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		return 0.0
	}
	return f
}

// SortedKeys returns sorted keys from a map (for deterministic output)
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}