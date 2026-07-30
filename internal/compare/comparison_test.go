package compare

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// helper to get testdata path (from internal/compare, testdata is ../../testdata)
func cmpTestdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

// TestSetBasedComparison verifies the compareDictOfLists function works as a set comparison.
// Two dicts with the same keys and values should have no differences.
func TestSetBasedComparison(t *testing.T) {
	dict1 := map[string][][3]float64{
		"BLOCK_A": {{1.0, 2.0, 0.0}, {3.0, 4.0, 0.0}},
		"BLOCK_B": {{5.0, 6.0, 0.0}},
	}
	dict2 := map[string][][3]float64{
		"BLOCK_A": {{1.0, 2.0, 0.0}, {3.0, 4.0, 0.0}},
		"BLOCK_B": {{5.0, 6.0, 0.0}},
	}

	common, onlyIn1, onlyIn2, diff := compareDictOfLists(dict1, dict2)

	if len(common) != 2 {
		t.Errorf("expected 2 common keys, got %d: %v", len(common), common)
	}
	if len(onlyIn1) != 0 {
		t.Errorf("expected 0 only-in-1, got %d: %v", len(onlyIn1), onlyIn1)
	}
	if len(onlyIn2) != 0 {
		t.Errorf("expected 0 only-in-2, got %d: %v", len(onlyIn2), onlyIn2)
	}
	if len(diff) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(diff))
	}
}

// TestSetBasedComparisonWithDifferences verifies diff detection.
func TestSetBasedComparisonWithDifferences(t *testing.T) {
	dict1 := map[string][][3]float64{
		"BLOCK_A": {{1.0, 2.0, 0.0}, {3.0, 4.0, 0.0}},
		"BLOCK_C": {{7.0, 8.0, 0.0}}, // only in dict1
	}
	dict2 := map[string][][3]float64{
		"BLOCK_A": {{1.0, 2.0, 0.0}},               // missing (3,4,0) — has fewer entries
		"BLOCK_D": {{9.0, 10.0, 0.0}},              // only in dict2
	}

	common, onlyIn1, onlyIn2, diff := compareDictOfLists(dict1, dict2)

	if len(common) != 1 || common[0] != "BLOCK_A" {
		t.Errorf("expected 1 common key 'BLOCK_A', got %v", common)
	}
	if len(onlyIn1) != 1 || onlyIn1[0] != "BLOCK_C" {
		t.Errorf("expected only-in-1=['BLOCK_C'], got %v", onlyIn1)
	}
	if len(onlyIn2) != 1 || onlyIn2[0] != "BLOCK_D" {
		t.Errorf("expected only-in-2=['BLOCK_D'], got %v", onlyIn2)
	}
	if len(diff) != 1 {
		t.Errorf("expected 1 diff for BLOCK_A, got %d", len(diff))
	}
	if d, ok := diff["BLOCK_A"]; ok {
		if len(d.OnlyIn1) != 1 {
			t.Errorf("expected 1 only-in-1 for BLOCK_A, got %d", len(d.OnlyIn1))
		}
		if len(d.OnlyIn2) != 0 {
			t.Errorf("expected 0 only-in-2 for BLOCK_A, got %d", len(d.OnlyIn2))
		}
	}
}

// TestContentHash verifies that ContentHash produces a consistent MD5 hash.
// The hash must be Python-compatible (same JSON serialization format).
func TestContentHash(t *testing.T) {
	content1 := &dxf.DXFContent{
		Blocks: map[string][][3]float64{
			"BLOCK_A": {{1.0, 2.0, 0.0}},
		},
		Lines:     map[string][][2][3]float64{},
		Polylines: map[string][][][3]float64{},
	}

	hash1 := ContentHash(content1)
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}
	if len(hash1) != 32 {
		t.Errorf("expected 32-char MD5 hash, got %d chars: %s", len(hash1), hash1)
	}

	// Same content should produce same hash
	hash2 := ContentHash(content1)
	if hash1 != hash2 {
		t.Errorf("identical content should produce same hash: %s vs %s", hash1, hash2)
	}

	// Different content should produce different hash
	content2 := &dxf.DXFContent{
		Blocks: map[string][][3]float64{
			"BLOCK_A": {{1.0, 2.0, 0.0}, {3.0, 4.0, 0.0}},
		},
		Lines:     map[string][][2][3]float64{},
		Polylines: map[string][][][3]float64{},
	}
	hash3 := ContentHash(content2)
	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}
}

// TestContentHashPythonFormat verifies the pythonJSON formatting matches Python's
// json.dumps format: floats like 1.0 not 1, sorted keys, specific spacing.
func TestContentHashPythonFormat(t *testing.T) {
	// Test pythonFloat: integers should have .0 suffix (like Python json.dumps)
	if pythonFloat(1.0) != "1.0" {
		t.Errorf("pythonFloat(1.0): expected '1.0', got '%s'", pythonFloat(1.0))
	}
	if pythonFloat(0.0) != "0.0" {
		t.Errorf("pythonFloat(0.0): expected '0.0', got '%s'", pythonFloat(0.0))
	}
	if pythonFloat(42.0) != "42.0" {
		t.Errorf("pythonFloat(42.0): expected '42.0', got '%s'", pythonFloat(42.0))
	}
	// Non-integers: shortest representation
	if pythonFloat(1.5) != "1.5" {
		t.Errorf("pythonFloat(1.5): expected '1.5', got '%s'", pythonFloat(1.5))
	}
}

// TestContentHashEmptyContent verifies hash of empty content is stable.
func TestContentHashEmptyContent(t *testing.T) {
	empty := &dxf.DXFContent{
		Blocks:    map[string][][3]float64{},
		Lines:     map[string][][2][3]float64{},
		Polylines: map[string][][][3]float64{},
	}
	hash := ContentHash(empty)
	if hash == "" {
		t.Fatal("expected non-empty hash for empty content")
	}
	// Empty content should produce a deterministic hash
	expectedEmpty := ContentHash(&dxf.DXFContent{
		Blocks:    map[string][][3]float64{},
		Lines:     map[string][][2][3]float64{},
		Polylines: map[string][][][3]float64{},
	})
	if hash != expectedEmpty {
		t.Error("empty content hash should be deterministic")
	}
}

// TestTemplateMatchingByFilenamePrefix verifies the fallback template matching
// when $(TEMPLATE) attribute is absent. Files are matched by filename prefix
// to template names.
func TestTemplateMatchingByFilenamePrefix(t *testing.T) {
	templateMap := TemplateMap{
		"BI001": "/templates/BI001.dxf",
		"BO002": "/templates/BO002.dxf",
	}

	// Exact prefix match
	name := getTemplateName(&dxf.Drawing{}, "BI001_module1.dxf", templateMap)
	if name != "BI001" {
		t.Errorf("expected 'BI001', got '%s'", name)
	}

	// Different prefix
	name = getTemplateName(&dxf.Drawing{}, "BO002_output.dxf", templateMap)
	if name != "BO002" {
		t.Errorf("expected 'BO002', got '%s'", name)
	}

	// No match
	name = getTemplateName(&dxf.Drawing{}, "ZZZ_unknown.dxf", templateMap)
	if name != "" {
		t.Errorf("expected empty template for no match, got '%s'", name)
	}

	// Longest match should win (if BI001 and BI001p1 are both templates)
	templateMap["BI001p1"] = "/templates/BI001p1.dxf"
	name = getTemplateName(&dxf.Drawing{}, "BI001p1_test.dxf", templateMap)
	if name != "BI001p1" {
		t.Errorf("expected longest match 'BI001p1', got '%s'", name)
	}
}

// TestNestedModFolderCreation verifies that _modN folders are created flat
// in the output root (matching Python behavior).
// Expected structure: Output/BI001_mod1/file.dxf
func TestNestedModFolderCreation(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	// Create template and module files
	templateDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templateDir, 0755)
	searchDir := filepath.Join(tmpDir, "search")
	os.MkdirAll(searchDir, 0755)

	// Copy template file
	templateSrc := cmpTestdataPath("template_bi001.dxf")
	templateData, _ := os.ReadFile(templateSrc)
	os.WriteFile(filepath.Join(templateDir, "BI001.dxf"), templateData, 0644)

	// Create a different module file
	moduleSrc := cmpTestdataPath("module_bi001_different.dxf")
	moduleData, _ := os.ReadFile(moduleSrc)
	os.WriteFile(filepath.Join(searchDir, "BI001_module1.dxf"), moduleData, 0644)

	// Build template map
	templateMap := BuildTemplateMap(templateDir, false, nil)
	if len(templateMap) == 0 {
		t.Fatal("expected template map to have entries")
	}
	if _, ok := templateMap["BI001"]; !ok {
		t.Fatalf("expected 'BI001' in template map, got: %v", templateMap)
	}

	// Run comparison
	var logs []string
	logFn := func(msg string) { logs = append(logs, msg) }

	results := RunComparison(
		templateMap,
		searchDir,
		outputFolder,
		false, // non-recursive
		false, // don't move files
		true,  // group by content
		func(done, total int) bool { return true },
		logFn,
	)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Status != "different" {
		t.Errorf("expected status 'different', got '%s'", results[0].Status)
	}

	// Verify flat _mod1 folder exists: Output/BI001_mod1/
	modDir := filepath.Join(outputFolder, "BI001_mod1")
	if _, err := os.Stat(modDir); os.IsNotExist(err) {
		// Maybe it's just in the template folder if no mod grouping needed
		templateDir2 := filepath.Join(outputFolder, "BI001")
		entries, _ := os.ReadDir(templateDir2)
		t.Errorf("flat _mod1 folder not found at %s. Template dir contents: %v", modDir, entries)
	}

	// Verify the DXF file is in the _mod1 folder
	dxfInMod := filepath.Join(modDir, "BI001_module1.dxf")
	if _, err := os.Stat(dxfInMod); os.IsNotExist(err) {
		// Check if it's in the template folder instead (before Finalize moves it)
		dxfInTemplate := filepath.Join(outputFolder, "BI001", "BI001_module1.dxf")
		if _, err2 := os.Stat(dxfInTemplate); err2 == nil {
			t.Errorf("DXF file found in template folder but not moved to _mod1 folder")
		} else {
			t.Errorf("DXF file not found in _mod1 folder or template folder")
		}
	}
}

// TestProcessFileMatching verifies that a file identical to its template returns "match".
func TestProcessFileMatching(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	// Setup: template and identical module
	templatePath := cmpTestdataPath("template_bi001.dxf")
	modulePath := cmpTestdataPath("module_bi001_match.dxf")

	templateMap := TemplateMap{
		"BI001": templatePath,
	}

	processor := NewComparisonProcessor(templateMap, outputFolder, false, true, func(string) {})

	result := processor.ProcessFile(modulePath)

	if result.Status != "match" {
		t.Errorf("expected 'match', got '%s' (template='%s')", result.Status, result.Template)
	}
	if result.Template != "BI001" {
		t.Errorf("expected template 'BI001', got '%s'", result.Template)
	}

	// Verify file was copied to output/BI001/
	copiedPath := filepath.Join(outputFolder, "BI001", "BI001_module_match.dxf")
	// The actual filename is module_bi001_match.dxf
	copiedPath = filepath.Join(outputFolder, "BI001", filepath.Base(modulePath))
	if _, err := os.Stat(copiedPath); os.IsNotExist(err) {
		t.Errorf("matched file should be copied to %s", copiedPath)
	}
}

// TestProcessFileDifferent verifies that a file different from its template returns "different".
func TestProcessFileDifferent(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	templatePath := cmpTestdataPath("template_bi001.dxf")
	modulePath := cmpTestdataPath("module_bi001_different.dxf")

	templateMap := TemplateMap{
		"BI001": templatePath,
	}

	processor := NewComparisonProcessor(templateMap, outputFolder, false, true, func(string) {})

	result := processor.ProcessFile(modulePath)

	if result.Status != "different" {
		t.Errorf("expected 'different', got '%s'", result.Status)
	}
}

// TestProcessFileNoTemplate verifies that a file with no matching template returns "no_template".
func TestProcessFileNoTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	templateMap := TemplateMap{
		"BI001": cmpTestdataPath("template_bi001.dxf"),
	}

	// zzz_unknown.dxf has no $(TEMPLATE) attr and filename doesn't match any template prefix
	modulePath := cmpTestdataPath("zzz_unknown.dxf")

	processor := NewComparisonProcessor(templateMap, outputFolder, false, true, func(string) {})

	result := processor.ProcessFile(modulePath)

	if result.Status != "no_template" {
		t.Errorf("expected 'no_template', got '%s'", result.Status)
	}

	// Verify file was copied to output/notemplate/
	copiedPath := filepath.Join(outputFolder, "notemplate", filepath.Base(modulePath))
	if _, err := os.Stat(copiedPath); os.IsNotExist(err) {
		t.Errorf("notemplate file should be copied to %s", copiedPath)
	}
}

// TestSanitizeFilename verifies that special characters are replaced with underscores.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BI001", "BI001"},
		{"BI 001", "BI_001"},
		{"BI/001", "BI_001"},
		{"BI:001", "BI_001"},
		{"BI.001", "BI.001"}, // dots are allowed
		{"BI_001", "BI_001"},
		{"BI-001", "BI-001"}, // hyphens are allowed
	}

	for _, tc := range tests {
		got := sanitizeFilename(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeFilename(%q): expected '%s', got '%s'", tc.input, tc.expected, got)
		}
	}
}

// TestBuildTemplateMap verifies template map building from a folder.
func TestBuildTemplateMap(t *testing.T) {
	tmpDir := t.TempDir()

	// Copy template files to temp dir
	for _, name := range []string{"template_bi001.dxf", "template_bo002.dxf"} {
		src := cmpTestdataPath(name)
		data, _ := os.ReadFile(src)
		os.WriteFile(filepath.Join(tmpDir, name), data, 0644)
	}

	// Also add a file with no $(TEMPLATE) — should be skipped
	os.WriteFile(filepath.Join(tmpDir, "zzz_unknown.dxf"),
		mustReadFile(cmpTestdataPath("zzz_unknown.dxf")), 0644)

	templateMap := BuildTemplateMap(tmpDir, false, nil)

	if len(templateMap) != 2 {
		t.Errorf("expected 2 templates in map, got %d: %v", len(templateMap), templateMap)
	}
	if _, ok := templateMap["BI001"]; !ok {
		t.Error("expected 'BI001' in template map")
	}
	if _, ok := templateMap["BO002"]; !ok {
		t.Error("expected 'BO002' in template map")
	}
}

// TestFinalizeCreatesModFolders verifies that Finalize() creates _modN folders
// and moves files from template folders to mod folders.
func TestFinalizeCreatesModFolders(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	templatePath := cmpTestdataPath("template_bi001.dxf")
	modulePath := cmpTestdataPath("module_bi001_different.dxf")

	templateMap := TemplateMap{"BI001": templatePath}
	processor := NewComparisonProcessor(templateMap, outputFolder, false, true, func(string) {})

	// Process two different files (same content → same hash → one mod folder)
	processor.ProcessFile(modulePath)

	// Finalize should create _mod1 folder and move the file there
	processor.Finalize()

	// Check _mod1 folder was created (flat, matching Python)
	modDir := filepath.Join(outputFolder, "BI001_mod1")
	if _, err := os.Stat(modDir); os.IsNotExist(err) {
		t.Fatalf("expected _mod1 folder at %s", modDir)
	}

	// Check file was moved to _mod1
	movedFile := filepath.Join(modDir, filepath.Base(modulePath))
	if _, err := os.Stat(movedFile); os.IsNotExist(err) {
		t.Errorf("expected file moved to %s", movedFile)
	}

	// Check log file was created
	logFile := filepath.Join(modDir, "BI001_mod1_dxfanalyze.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("expected log file at %s", logFile)
	}
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}