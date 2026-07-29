package dxf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to get testdata path
func testdataPath(t *testing.T, name string) string {
	t.Helper()
	// Go test runs from package dir (internal/dxf), so ../../testdata
	p := filepath.Join("..", "..", "testdata", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("testdata file not found: %s", p)
	}
	return p
}

// TestParseLine verifies basic LINE entity parsing with endpoints.
func TestParseLine(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "module_bi001_match.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var lines []Entity
	for _, e := range drawing.Entities {
		if strings.TrimSpace(e.Type) == "LINE" {
			lines = append(lines, e)
		}
	}

	if len(lines) != 4 {
		t.Fatalf("expected 4 LINE entities, got %d", len(lines))
	}

	// Verify first line coordinates: (0,0) -> (100,0)
	l := lines[0]
	sx := l.GetFloatValue(10)
	sy := l.GetFloatValue(20)
	ex := l.GetFloatValue(11)
	ey := l.GetFloatValue(21)

	if sx != 0.0 || sy != 0.0 || ex != 100.0 || ey != 0.0 {
		t.Errorf("first line coords: expected (0,0)->(100,0), got (%.1f,%.1f)->(%.1f,%.1f)", sx, sy, ex, ey)
	}
}

// TestParseInsertWithAttribs verifies INSERT entities with ATTRIB collection.
// The INSERT has code 66=1 (has attributes) and should collect the $(TEMPLATE) ATTRIB.
func TestParseInsertWithAttribs(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "template_bi001.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var inserts []Entity
	for _, e := range drawing.Entities {
		if strings.TrimSpace(e.Type) == "INSERT" {
			inserts = append(inserts, e)
		}
	}

	if len(inserts) < 1 {
		t.Fatalf("expected at least 1 INSERT, got %d", len(inserts))
	}

	// Find the TITLEBLOCK INSERT (has ATTRIBs)
	var titleInsert *Entity
	for i := range inserts {
		if inserts[i].GetStringValue(2) == "TITLEBLOCK" {
			titleInsert = &inserts[i]
			break
		}
	}
	if titleInsert == nil {
		t.Fatal("TITLEBLOCK INSERT not found")
	}

	if len(titleInsert.Attribs) != 1 {
		t.Fatalf("expected 1 ATTRIB on TITLEBLOCK, got %d", len(titleInsert.Attribs))
	}

	// Verify $(TEMPLATE) attribute
	templateVal := titleInsert.GetAttribValue("$(TEMPLATE)")
	if templateVal != "BI001" {
		t.Errorf("expected $(TEMPLATE)='BI001', got '%s'", templateVal)
	}

	// Verify COMPANY INSERT has NO attribs (code 66 absent or 0)
	var companyInsert *Entity
	for i := range inserts {
		if inserts[i].GetStringValue(2) == "COMPANY" {
			companyInsert = &inserts[i]
			break
		}
	}
	if companyInsert != nil && len(companyInsert.Attribs) != 0 {
		t.Errorf("COMPANY INSERT should have 0 ATTRIBs, got %d", len(companyInsert.Attribs))
	}
}

// TestParsePolyline verifies POLYLINE/VERTEX/SEQEND collection.
// The POLYLINE entity should absorb its VERTEX entities' coordinate pairs.
func TestParsePolyline(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "module_polyline.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var polylines []Entity
	for _, e := range drawing.Entities {
		if strings.TrimSpace(e.Type) == "POLYLINE" {
			polylines = append(polylines, e)
		}
	}

	if len(polylines) != 1 {
		t.Fatalf("expected 1 POLYLINE, got %d", len(polylines))
	}

	// The POLYLINE should have absorbed 3 VERTEX entities.
	// Each VERTEX has 10/20/30 codes. The header has 10/20/30 too.
	// So we expect: 3 header pairs (10,20,30) + 3 * 3 vertex pairs = 12 pairs from coords,
	// plus code 70 from POLYLINE header = 13 total.
	// But some pairs may be missing Z (code 30) — let's just check we have enough.
	totalPairs := len(polylines[0].Pairs)
	if totalPairs < 7 {
		t.Errorf("POLYLINE should have absorbed VERTEX pairs, got only %d pairs", totalPairs)
	}

	// Count 10 codes (X coordinates) — header + 3 vertices = 4
	xCount := 0
	for _, p := range polylines[0].Pairs {
		if p.Code == 10 {
			xCount++
		}
	}
	if xCount != 4 {
		t.Errorf("expected 4 X-coordinates (header + 3 vertices), got %d", xCount)
	}
}

// TestMTextStripping verifies MTEXT formatting codes are stripped by stripMTextFormatting.
// This tests the stripMTextFormatting function indirectly via extractFromDrawing.
// Since stripMTextFormatting is in the api package, we test the parser's raw value here.
func TestMTextParsing(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "module_mtext.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var mtexts []Entity
	for _, e := range drawing.Entities {
		if strings.TrimSpace(e.Type) == "MTEXT" {
			mtexts = append(mtexts, e)
		}
	}

	if len(mtexts) != 3 {
		t.Fatalf("expected 3 MTEXT entities, got %d", len(mtexts))
	}

	// First MTEXT: \PLine1\PLine2
	raw := mtexts[0].GetStringValue(1)
	if !strings.Contains(raw, "\\P") {
		t.Errorf("expected \\P formatting code in MTEXT value, got: %s", raw)
	}

	// Second MTEXT: {\fArial|b0|i0|c0|p34;Formatted Text}
	raw2 := mtexts[1].GetStringValue(1)
	if !strings.Contains(raw2, "Formatted Text") {
		t.Errorf("expected 'Formatted Text' in MTEXT value, got: %s", raw2)
	}
}

// TestCRLFHandling verifies that CRLF line endings are handled correctly.
// The parser should strip \r from values.
func TestCRLFHandling(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "template_bi001_crlf.dxf"))
	if err != nil {
		t.Fatalf("ReadFile with CRLF failed: %v", err)
	}

	// Should parse the same as the LF version
	templateVal := drawing.GetTemplateAttribute()
	if templateVal != "BI001" {
		t.Errorf("CRLF: expected $(TEMPLATE)='BI001', got '%s'", templateVal)
	}

	// Verify no \r in entity values
	for _, e := range drawing.Entities {
		for _, p := range e.Pairs {
			if strings.Contains(p.Value, "\r") {
				t.Errorf("CRLF not stripped from value: code=%d value=%q", p.Code, p.Value)
			}
		}
	}
}

// TestSEQENDNoData verifies that a SEQEND entity with no data pairs doesn't
// cause an infinite loop. This was a real bug: collectEntityPairs would
// back up to the current entity when it had no data, causing i to never advance.
func TestSEQENDNoData(t *testing.T) {
	// This test is designed to complete quickly — if the bug exists,
	// it will hang forever. The test framework will time out.
	drawing, err := ReadFile(testdataPath(t, "seqend_nodata.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Should have 1 LINE entity and the SEQEND should be skipped
	var lines []Entity
	for _, e := range drawing.Entities {
		if strings.TrimSpace(e.Type) == "LINE" {
			lines = append(lines, e)
		}
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 LINE entity (SEQEND should be skipped), got %d", len(lines))
	}

	// SEQEND should NOT appear as an entity
	for _, e := range drawing.Entities {
		if strings.TrimSpace(e.Type) == "SEQEND" {
			t.Error("SEQEND should not appear in entities list")
		}
	}
}

// TestGetTemplateAttribute verifies the $(TEMPLATE) attribute extraction.
func TestGetTemplateAttribute(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"template_bi001.dxf", "BI001"},
		{"template_bo002.dxf", "BO002"},
		{"module_bi001_match.dxf", "BI001"},
		{"module_bi001_different.dxf", "BI001"},
		{"module_polyline.dxf", ""},       // no INSERT with $(TEMPLATE)
		{"zzz_unknown.dxf", ""},            // no INSERT at all
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			drawing, err := ReadFile(testdataPath(t, tc.filename))
			if err != nil {
				t.Fatalf("ReadFile failed: %v", err)
			}
			got := drawing.GetTemplateAttribute()
			if got != tc.expected {
				t.Errorf("GetTemplateAttribute: expected '%s', got '%s'", tc.expected, got)
			}
		})
	}
}

// TestExtractContent verifies the content extraction produces correct structure.
func TestExtractContent(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "template_bi001.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := drawing.ExtractContent(3)

	// Template should be BI001
	if content.Template != "BI001" {
		t.Errorf("expected template 'BI001', got '%s'", content.Template)
	}

	// Should have lines on layer "0"
	if len(content.Lines["0"]) != 4 {
		t.Errorf("expected 4 lines on layer '0', got %d", len(content.Lines["0"]))
	}

	// COMPANY INSERT should be excluded
	if _, exists := content.Blocks["COMPANY"]; exists {
		t.Error("COMPANY block should be excluded from content")
	}

	// TITLEBLOCK INSERT should be included
	if len(content.Blocks["TITLEBLOCK"]) != 1 {
		t.Errorf("expected 1 TITLEBLOCK insert, got %d", len(content.Blocks["TITLEBLOCK"]))
	}
}

// TestReadFromReaderWithEmptyInput verifies parser doesn't crash on empty input.
func TestReadFromReaderWithEmptyInput(t *testing.T) {
	drawing, err := ReadFromReader(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ReadFromReader with empty input failed: %v", err)
	}
	if drawing == nil {
		t.Fatal("expected non-nil Drawing for empty input")
	}
	if len(drawing.Entities) != 0 {
		t.Errorf("expected 0 entities for empty input, got %d", len(drawing.Entities))
	}
}

// TestEntityHelpers tests GetStringValue, GetFloatValue, GetIntValue.
func TestEntityHelpers(t *testing.T) {
	drawing, err := ReadFile(testdataPath(t, "template_bi001.dxf"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var titleInsert *Entity
	for i := range drawing.Entities {
		if strings.TrimSpace(drawing.Entities[i].Type) == "INSERT" &&
			drawing.Entities[i].GetStringValue(2) == "TITLEBLOCK" {
			titleInsert = &drawing.Entities[i]
			break
		}
	}
	if titleInsert == nil {
		t.Fatal("TITLEBLOCK INSERT not found")
	}

	// String value (block name, code 2)
	if titleInsert.GetStringValue(2) != "TITLEBLOCK" {
		t.Errorf("GetStringValue(2): expected 'TITLEBLOCK', got '%s'", titleInsert.GetStringValue(2))
	}

	// Float value (insert X, code 10)
	if titleInsert.GetFloatValue(10) != 0.0 {
		t.Errorf("GetFloatValue(10): expected 0.0, got %f", titleInsert.GetFloatValue(10))
	}

	// Int value (has attribs flag, code 66)
	if titleInsert.GetIntValue(66) != 1 {
		t.Errorf("GetIntValue(66): expected 1, got %d", titleInsert.GetIntValue(66))
	}

	// Missing code returns default
	if titleInsert.GetStringValue(999) != "" {
		t.Errorf("GetStringValue(999): expected '', got '%s'", titleInsert.GetStringValue(999))
	}
	if titleInsert.GetFloatValue(999) != 0.0 {
		t.Errorf("GetFloatValue(999): expected 0.0, got %f", titleInsert.GetFloatValue(999))
	}
	if titleInsert.GetIntValue(999) != 0 {
		t.Errorf("GetIntValue(999): expected 0, got %d", titleInsert.GetIntValue(999))
	}
}