package compare

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// TestParallelComparison4Workers verifies that the parallel comparison engine
// uses 4 workers and processes all files correctly.
func TestParallelComparison4Workers(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	// Setup template
	templateDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templateDir, 0755)
	templateData, _ := os.ReadFile(cmpTestdataPath("template_bi001.dxf"))
	os.WriteFile(filepath.Join(templateDir, "BI001.dxf"), templateData, 0644)

	// Setup search folder with multiple files
	searchDir := filepath.Join(tmpDir, "search")
	os.MkdirAll(searchDir, 0755)

	// 3 matching files + 2 different files + 1 no-template
	matchData, _ := os.ReadFile(cmpTestdataPath("module_bi001_match.dxf"))
	diffData, _ := os.ReadFile(cmpTestdataPath("module_bi001_different.dxf"))
	unknownData, _ := os.ReadFile(cmpTestdataPath("zzz_unknown.dxf"))

	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(searchDir, "BI001_match_"+string(rune('1'+i))+".dxf"), matchData, 0644)
	}
	for i := 0; i < 2; i++ {
		os.WriteFile(filepath.Join(searchDir, "BI001_diff_"+string(rune('1'+i))+".dxf"), diffData, 0644)
	}
	os.WriteFile(filepath.Join(searchDir, "zzz_unknown.dxf"), unknownData, 0644)

	templateMap := BuildTemplateMap(templateDir, false, nil)
	if len(templateMap) == 0 {
		t.Fatal("expected template map to have entries")
	}

	var logCount atomic.Int64
	logFn := func(msg string) { logCount.Add(1) }

	var progressCalls atomic.Int64
	progressFn := func(done, total int) bool {
		progressCalls.Add(1)
		return true
	}

	results := RunComparisonParallel(
		templateMap,
		searchDir,
		outputFolder,
		false, // non-recursive
		false, // don't move
		true,  // group by content
		progressFn,
		logFn,
		4, // 4 workers
	)

	if len(results) != 6 {
		t.Errorf("expected 6 results, got %d", len(results))
	}

	// Count statuses
	matched, different, noTemplate := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "match":
			matched++
		case "different":
			different++
		case "no_template":
			noTemplate++
		}
	}

	if matched != 3 {
		t.Errorf("expected 3 matched, got %d", matched)
	}
	if different != 2 {
		t.Errorf("expected 2 different, got %d", different)
	}
	if noTemplate != 1 {
		t.Errorf("expected 1 no_template, got %d", noTemplate)
	}

	// Progress should have been called
	if progressCalls.Load() == 0 {
		t.Error("expected progress function to be called")
	}
}

// TestFileDecisionStruct verifies the fileDecision struct is populated correctly
// by processFileParallel.
func TestFileDecisionStruct(t *testing.T) {
	templatePath := cmpTestdataPath("template_bi001.dxf")
	modulePath := cmpTestdataPath("module_bi001_different.dxf")

	templateMap := TemplateMap{"BI001": templatePath}

	// Test "different" decision
	decision := processFileParallel(modulePath, templateMap, true, func(string) {})

	if decision.FileName != filepath.Base(modulePath) {
		t.Errorf("expected filename '%s', got '%s'", filepath.Base(modulePath), decision.FileName)
	}
	if decision.Status != "different" {
		t.Errorf("expected status 'different', got '%s'", decision.Status)
	}
	if decision.TemplateName != "BI001" {
		t.Errorf("expected template 'BI001', got '%s'", decision.TemplateName)
	}
	if decision.ContentHash == "" {
		t.Error("expected non-empty content hash for different file with groupByContent=true")
	}
	if decision.Content == nil {
		t.Error("expected non-nil Content for different file")
	}
	if decision.DetailedLog == "" {
		t.Error("expected non-empty detailed log for different file")
	}

	// Test "match" decision
	matchPath := cmpTestdataPath("module_bi001_match.dxf")
	decision2 := processFileParallel(matchPath, templateMap, true, func(string) {})

	if decision2.Status != "match" {
		t.Errorf("expected status 'match', got '%s'", decision2.Status)
	}
	if decision2.Content != nil {
		t.Error("expected nil Content for matching file (no need to keep content)")
	}

	// Test "no_template" decision
	unknownPath := cmpTestdataPath("zzz_unknown.dxf")
	decision3 := processFileParallel(unknownPath, templateMap, true, func(string) {})

	if decision3.Status != "no_template" {
		t.Errorf("expected status 'no_template', got '%s'", decision3.Status)
	}
}

// TestAtomicProgress verifies that the progress counter is atomically incremented
// and the total matches the number of files.
func TestAtomicProgress(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	templateDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templateDir, 0755)
	searchDir := filepath.Join(tmpDir, "search")
	os.MkdirAll(searchDir, 0755)

	templateData, _ := os.ReadFile(cmpTestdataPath("template_bi001.dxf"))
	os.WriteFile(filepath.Join(templateDir, "BI001.dxf"), templateData, 0644)

	matchData, _ := os.ReadFile(cmpTestdataPath("module_bi001_match.dxf"))
	// Create 10 files
	for i := 0; i < 10; i++ {
		name := filepath.Join(searchDir, "BI001_file_"+string(rune('A'+i))+".dxf")
		os.WriteFile(name, matchData, 0644)
	}

	templateMap := BuildTemplateMap(templateDir, false, nil)

	var maxDone atomic.Int64
	progressFn := func(done, total int) bool {
		d := int64(done)
		for {
			curr := maxDone.Load()
			if d <= curr {
				break
			}
			if maxDone.CompareAndSwap(curr, d) {
				break
			}
		}
		return true
	}

	results := RunComparisonParallel(
		templateMap, searchDir, outputFolder,
		false, false, true,
		progressFn, func(string) {}, 4,
	)

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}

	// The max done should equal total (10)
	if maxDone.Load() != 10 {
		t.Errorf("expected max done=10, got %d", maxDone.Load())
	}
}

// TestParallelStop verifies that the stop signal is respected.
func TestParallelStop(t *testing.T) {
	tmpDir := t.TempDir()
	outputFolder := filepath.Join(tmpDir, "output")

	templateDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templateDir, 0755)
	searchDir := filepath.Join(tmpDir, "search")
	os.MkdirAll(searchDir, 0755)

	templateData, _ := os.ReadFile(cmpTestdataPath("template_bi001.dxf"))
	os.WriteFile(filepath.Join(templateDir, "BI001.dxf"), templateData, 0644)

	matchData, _ := os.ReadFile(cmpTestdataPath("module_bi001_match.dxf"))
	for i := 0; i < 20; i++ {
		os.WriteFile(filepath.Join(searchDir, "BI001_file_"+string(rune('A'+i))+".dxf"), matchData, 0644)
	}

	templateMap := BuildTemplateMap(templateDir, false, nil)

	// Stop after first file
	stopAfter := atomic.Int64{}
	stopAfter.Store(1)
	progressFn := func(done, total int) bool {
		return done < int(stopAfter.Load())
	}

	results := RunComparisonParallel(
		templateMap, searchDir, outputFolder,
		false, false, true,
		progressFn, func(string) {}, 4,
	)

	// Should have processed at least 1 but not all 20
	if len(results) == 0 {
		t.Error("expected at least 1 result before stop")
	}
	if len(results) == 20 {
		t.Error("expected stop to prevent all 20 from being processed")
	}
}

// TestProcessFileParallelError verifies that a non-existent file produces an error decision.
func TestProcessFileParallelError(t *testing.T) {
	templateMap := TemplateMap{"BI001": cmpTestdataPath("template_bi001.dxf")}

	decision := processFileParallel("/nonexistent/file.dxf", templateMap, true, func(string) {})

	if decision.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", decision.Status)
	}
}

// TestGetTemplateNameFromDrawing verifies template name extraction from drawing attributes.
func TestGetTemplateNameFromDrawing(t *testing.T) {
	// Drawing with $(TEMPLATE) attribute
	drawing := &dxf.Drawing{
		Entities: []dxf.Entity{
			{
				Type: "INSERT",
				Pairs: []dxf.CodePair{
					{Code: 2, Value: "TITLEBLOCK"},
					{Code: 66, Value: "1"},
				},
				Attribs: []dxf.Entity{
					{
						Type: "ATTRIB",
						Pairs: []dxf.CodePair{
							{Code: 2, Value: "$(TEMPLATE)"},
							{Code: 1, Value: "BI001"},
						},
					},
				},
			},
		},
	}

	templateMap := TemplateMap{"BI001": "/path/BI001.dxf"}
	name := getTemplateName(drawing, "anything.dxf", templateMap)
	if name != "BI001" {
		t.Errorf("expected 'BI001' from $(TEMPLATE) attr, got '%s'", name)
	}
}