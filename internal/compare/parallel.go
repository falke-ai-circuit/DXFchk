package compare

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// fileDecision is the result of parallel processing for a single file
type fileDecision struct {
	FileName     string
	FilePath     string
	TemplateName string
	Status       string // "match", "different", "no_template", "error"
	ContentHash  string
	DetailedLog  string
	// For "different" files: the extracted content for hash grouping
	Content *dxf.DXFContent
}

// RunComparisonParallel processes files in parallel, then finalizes sequentially
func RunComparisonParallel(templateMap TemplateMap, searchFolder, outputFolder string, recursive, moveFiles, groupByContent bool, progressFn func(int, int) bool, logFn func(string), workers int) []Result {
	processor := NewComparisonProcessor(templateMap, outputFolder, false, groupByContent, logFn)

	logFn("Note: Original files in search folder will be preserved")
	logFn("Note: Files with differences will be moved from template folders to mod folders to avoid duplicates")
	logFn("Note: Detailed comparison logs will be saved for each template")

	// Find all DXF files
	var dxfFiles []string
	if recursive {
		filepath.Walk(searchFolder, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".dxf") {
				dxfFiles = append(dxfFiles, path)
			}
			return nil
		})
	} else {
		entries, _ := os.ReadDir(searchFolder)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dxf") {
				dxfFiles = append(dxfFiles, filepath.Join(searchFolder, e.Name()))
			}
		}
	}

	total := len(dxfFiles)
	logFn(fmt.Sprintf("Found %d DXF files to process", total))

	if workers <= 0 {
		workers = 4
	}
	if workers > total {
		workers = total
	}

	// Phase 1: Process all files in parallel (read-only work)
	decisions := make([]fileDecision, total)
	var processed atomic.Int64
	var stopped atomic.Bool

	var wg sync.WaitGroup
	fileCh := make(chan struct{ idx int; file string }, total)
	for i, f := range dxfFiles {
		fileCh <- struct{ idx int; file string }{i, f}
	}
	close(fileCh)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range fileCh {
				if stopped.Load() {
					return
				}
				decision := processFileParallel(item.file, templateMap, groupByContent, logFn)
				decisions[item.idx] = decision

				done := int(processed.Add(1))
				if progressFn != nil && !progressFn(done, total) {
					stopped.Store(true)
					return
				}
			}
		}()
	}
	wg.Wait()

	if stopped.Load() {
		logFn("=== Comparison stopped by user ===")
	}

	// Phase 2: Apply decisions sequentially (file writes, processor state updates)
	results := make([]Result, 0, total)
	for i := 0; i < total; i++ {
		d := decisions[i]
		if d.FileName == "" {
			continue
		}
		switch d.Status {
		case "match":
			processor.applyMatch(d)
			results = append(results, Result{FileName: d.FileName, Template: d.TemplateName, Status: "match"})
		case "different":
			processor.applyDifferent(d)
			results = append(results, Result{FileName: d.FileName, Template: d.TemplateName, Status: "different"})
		case "no_template":
			processor.applyNoTemplate(d)
			results = append(results, Result{FileName: d.FileName, Template: "notemplate", Status: "no_template"})
		case "error":
			results = append(results, Result{FileName: d.FileName, Template: "error", Status: "error"})
		}
	}

	processor.Finalize()

	if progressFn != nil {
		progressFn(total, total)
	}

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
	// Python: processed_count = results["matched_files"] + results["different_files"] + results["no_template_files"]
	processedCount := matched + different + noTemplate
	logFn(fmt.Sprintf("Processing complete. Processed %d of %d files.", processedCount, total))

	// Python: if processed_count < len(dxf_files):
	//             log_callback(f"Warning: {len(dxf_files) - processed_count} files may have been skipped due to errors.")
	if processedCount < total {
		logFn(fmt.Sprintf("Warning: %d files may have been skipped due to errors.", total-processedCount))
	}

	return results
}

// processFileParallel does the read-only work: parse DXF, find template, compare, hash
func processFileParallel(dxfFile string, templateMap TemplateMap, groupByContent bool, logFn func(string)) fileDecision {
	defer func() {
		if r := recover(); r != nil {
			logFn(fmt.Sprintf("ERROR: panic processing %s: %v", filepath.Base(dxfFile), r))
		}
	}()

	fileName := filepath.Base(dxfFile)
	logFn(fmt.Sprintf("Processing: %s", fileName))

	drawing, err := parseDXFFile(dxfFile)
	if err != nil {
		logFn(fmt.Sprintf("Error reading %s: %v", fileName, err))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "error", TemplateName: "error"}
	}

	// Extract template name
	templateName := getTemplateName(drawing, fileName, templateMap)

	if templateName == "" {
		// Python: self._log(f"Processing: {filename}")  ← duplicate
		logFn(fmt.Sprintf("Processing: %s", fileName))
		logFn(fmt.Sprintf("  -> No template found for %s", fileName))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "no_template", TemplateName: "notemplate"}
	}

	templatePath, found := templateMap[templateName]
	if !found {
		// Python: self._log(f"Processing: {filename}")  ← duplicate
		logFn(fmt.Sprintf("Processing: %s", fileName))
		logFn(fmt.Sprintf("  -> No template found for %s", fileName))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "no_template", TemplateName: "notemplate"}
	}

	// Compare
	identical, content1, _ := compareFilesParallel(dxfFile, templatePath, logFn)
	if identical {
		// Python: self._log(f"Processing: {filename}")
		logFn(fmt.Sprintf("Processing: %s", fileName))
		// Python: self._log(f"  -> Using template: {template_name}")
		logFn(fmt.Sprintf("  -> Using template: %s", templateName))
		// Python: self._log(f"  -> MATCH: {filename} is identical to template")
		logFn(fmt.Sprintf("  -> MATCH: %s is identical to template", fileName))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "match", TemplateName: templateName}
	}

	// Different — compute hash
	contentHash := ""
	if groupByContent && content1 != nil {
		contentHash = ContentHash(content1)
	}

	// Build detailed log (full version matching Python _compare_with_template)
	detailedLog := buildDetailedLog(fileName, dxfFile, templateName, templatePath, content1)

	// Python diff summary: f"  -> Found differences in {key}: {len(result['diff'])} layer(s) with differences"
	if content1 != nil {
		content2, _ := extractContent(templatePath)
		if content2 != nil {
			_, _, _, diffB := compareDictOfLists(content1.Blocks, content2.Blocks)
			_, _, _, diffL := compareDictOfListsLines(content1.Lines, content2.Lines)
			_, _, _, diffP := compareDictOfListsPolylines(content1.Polylines, content2.Polylines)
			if len(diffB) > 0 {
				logFn(fmt.Sprintf("  -> Found differences in blocks: %d layer(s) with differences", len(diffB)))
			}
			if len(diffL) > 0 {
				logFn(fmt.Sprintf("  -> Found differences in lines: %d layer(s) with differences", len(diffL)))
			}
			if len(diffP) > 0 {
				logFn(fmt.Sprintf("  -> Found differences in polylines: %d layer(s) with differences", len(diffP)))
			}
		}
	}

	// Python: self._log(f"  -> DIFFERENT: {filename} has differences from template")
	logFn(fmt.Sprintf("  -> DIFFERENT: %s has differences from template", fileName))

	if contentHash != "" {
		logFn(fmt.Sprintf("  -> Grouped with content hash: %s", contentHash[:8]))
	}

	return fileDecision{
		FileName:     fileName,
		FilePath:     dxfFile,
		TemplateName: templateName,
		Status:       "different",
		ContentHash:  contentHash,
		DetailedLog:  detailedLog,
		Content:      content1,
	}
}

// parseDXFFile is a wrapper that returns a Drawing
func parseDXFFile(path string) (*dxf.Drawing, error) {
	return dxf.ReadFile(path)
}

// getTemplateName extracts the template name from the drawing or filename matching
func getTemplateName(drawing *dxf.Drawing, fileName string, templateMap TemplateMap) string {
	templateName := drawing.GetTemplateAttribute()
	if templateName == "" {
		bestMatch := ""
		for name := range templateMap {
			if name == "" || strings.TrimSpace(name) == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(fileName), strings.ToLower(name)) {
				if len(name) > len(bestMatch) {
					bestMatch = name
				}
			}
		}
		templateName = bestMatch
	}
	return templateName
}

// compareFilesParallel compares two DXF files and returns (identical, content1, content2)
func compareFilesParallel(file1, file2 string, logFn func(string)) (bool, *dxf.DXFContent, *dxf.DXFContent) {
	content1, err1 := extractContent(file1)
	content2, err2 := extractContent(file2)
	if err1 != nil || err2 != nil {
		return false, content1, content2
	}
	_, onlyIn1B, onlyIn2B, diffB := compareDictOfLists(content1.Blocks, content2.Blocks)
	_, onlyIn1L, onlyIn2L, diffL := compareDictOfListsLines(content1.Lines, content2.Lines)
	_, onlyIn1P, onlyIn2P, diffP := compareDictOfListsPolylines(content1.Polylines, content2.Polylines)
	identical := len(onlyIn1B) == 0 && len(onlyIn2B) == 0 && len(diffB) == 0 &&
		len(onlyIn1L) == 0 && len(onlyIn2L) == 0 && len(diffL) == 0 &&
		len(onlyIn1P) == 0 && len(onlyIn2P) == 0 && len(diffP) == 0
	return identical, content1, content2
}

// buildDetailedLog creates the detailed comparison log string
// Full version matching Python _compare_with_template
func buildDetailedLog(fileName, dxfFile, templateName, templatePath string, content1 *dxf.DXFContent) string {
	content2, _ := extractContent(templatePath)
	if content2 == nil {
		// Error case
		var log []string
		log = append(log, "========== DETAILED COMPARISON LOG ==========")
		log = append(log, fmt.Sprintf("File: %s", fileName))
		log = append(log, fmt.Sprintf("Template: %s (%s)", templateName, filepath.Base(templatePath)))
		log = append(log, fmt.Sprintf("Comparison Time: %s", time.Now().Format("2006-01-02 15:04:05")))
		log = append(log, "==========================================\n")
		log = append(log, fmt.Sprintf("  -> Error reading DXF data from %s or template", fileName))
		log = append(log, "Processing failed - could not read DXF data")
		return strings.Join(log, "\n")
	}

	var detailedLog []string
	detailedLog = append(detailedLog, "========== DETAILED COMPARISON LOG ==========")
	detailedLog = append(detailedLog, fmt.Sprintf("File: %s", fileName))
	detailedLog = append(detailedLog, fmt.Sprintf("Template: %s (%s)", templateName, filepath.Base(templatePath)))
	detailedLog = append(detailedLog, fmt.Sprintf("Comparison Time: %s", time.Now().Format("2006-01-02 15:04:05")))
	detailedLog = append(detailedLog, "==========================================\n")

	// Entity counts
	blocksCount1 := 0
	for _, v := range content1.Blocks { blocksCount1 += len(v) }
	blocksCount2 := 0
	for _, v := range content2.Blocks { blocksCount2 += len(v) }
	linesCount1 := 0
	for _, v := range content1.Lines { linesCount1 += len(v) }
	linesCount2 := 0
	for _, v := range content2.Lines { linesCount2 += len(v) }
	polyCount1 := 0
	for _, v := range content1.Polylines { polyCount1 += len(v) }
	polyCount2 := 0
	for _, v := range content2.Polylines { polyCount2 += len(v) }

	detailedLog = append(detailedLog, "1. ENTITY COUNTS")
	detailedLog = append(detailedLog, fmt.Sprintf("  File Blocks: %d in %d block types", blocksCount1, len(content1.Blocks)))
	detailedLog = append(detailedLog, fmt.Sprintf("  Template Blocks: %d in %d block types", blocksCount2, len(content2.Blocks)))
	detailedLog = append(detailedLog, fmt.Sprintf("  File Lines: %d in %d layers", linesCount1, len(content1.Lines)))
	detailedLog = append(detailedLog, fmt.Sprintf("  Template Lines: %d in %d layers", linesCount2, len(content2.Lines)))
	detailedLog = append(detailedLog, fmt.Sprintf("  File Polylines: %d in %d layers", polyCount1, len(content1.Polylines)))
	detailedLog = append(detailedLog, fmt.Sprintf("  Template Polylines: %d in %d layers", polyCount2, len(content2.Polylines)))
	detailedLog = append(detailedLog, "")

	// Compare
	_, onlyIn1B, onlyIn2B, diffB := compareDictOfLists(content1.Blocks, content2.Blocks)
	_, onlyIn1L, onlyIn2L, diffL := compareDictOfListsLines(content1.Lines, content2.Lines)
	_, onlyIn1P, onlyIn2P, diffP := compareDictOfListsPolylines(content1.Polylines, content2.Polylines)

	// Block differences
	detailedLog = append(detailedLog, "2. BLOCK DIFFERENCES")
	if len(onlyIn1B) > 0 {
		detailedLog = append(detailedLog, "  Block types only in file:")
		sort.Strings(onlyIn1B)
		for _, name := range onlyIn1B {
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s (%d instances)", name, len(content1.Blocks[name])))
		}
	} else {
		detailedLog = append(detailedLog, "  No block types unique to file")
	}
	if len(onlyIn2B) > 0 {
		detailedLog = append(detailedLog, "  Block types only in template:")
		sort.Strings(onlyIn2B)
		for _, name := range onlyIn2B {
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s (%d instances)", name, len(content2.Blocks[name])))
		}
	} else {
		detailedLog = append(detailedLog, "  No block types unique to template")
	}
	if len(diffB) > 0 {
		detailedLog = append(detailedLog, "  Blocks with differences in common block types:")
		diffBKeys := make([]string, 0, len(diffB))
		for k := range diffB { diffBKeys = append(diffBKeys, k) }
		sort.Strings(diffBKeys)
		for _, name := range diffBKeys {
			d := diffB[name]
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s:", name))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d instances only in file", len(d.OnlyIn1)))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d instances only in template", len(d.OnlyIn2)))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d common instances", len(d.Common)))
		}
	} else {
		detailedLog = append(detailedLog, "  No differences in common block types")
	}
	detailedLog = append(detailedLog, "")

	// Line differences
	detailedLog = append(detailedLog, "3. LINE DIFFERENCES")
	if len(onlyIn1L) > 0 {
		detailedLog = append(detailedLog, "  Line layers only in file:")
		sort.Strings(onlyIn1L)
		for _, name := range onlyIn1L {
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s (%d lines)", name, len(content1.Lines[name])))
		}
	} else {
		detailedLog = append(detailedLog, "  No line layers unique to file")
	}
	if len(onlyIn2L) > 0 {
		detailedLog = append(detailedLog, "  Line layers only in template:")
		sort.Strings(onlyIn2L)
		for _, name := range onlyIn2L {
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s (%d lines)", name, len(content2.Lines[name])))
		}
	} else {
		detailedLog = append(detailedLog, "  No line layers unique to template")
	}
	if len(diffL) > 0 {
		detailedLog = append(detailedLog, "  Lines with differences in common layers:")
		diffLKeys := make([]string, 0, len(diffL))
		for k := range diffL { diffLKeys = append(diffLKeys, k) }
		sort.Strings(diffLKeys)
		for _, name := range diffLKeys {
			d := diffL[name]
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s:", name))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d lines only in file", len(d.OnlyIn1)))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d lines only in template", len(d.OnlyIn2)))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d common lines", len(d.Common)))
		}
	} else {
		detailedLog = append(detailedLog, "  No differences in common line layers")
	}
	detailedLog = append(detailedLog, "")

	// Polyline differences
	detailedLog = append(detailedLog, "4. POLYLINE DIFFERENCES")
	if len(onlyIn1P) > 0 {
		detailedLog = append(detailedLog, "  Polyline layers only in file:")
		sort.Strings(onlyIn1P)
		for _, name := range onlyIn1P {
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s (%d polylines)", name, len(content1.Polylines[name])))
		}
	} else {
		detailedLog = append(detailedLog, "  No polyline layers unique to file")
	}
	if len(onlyIn2P) > 0 {
		detailedLog = append(detailedLog, "  Polyline layers only in template:")
		sort.Strings(onlyIn2P)
		for _, name := range onlyIn2P {
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s (%d polylines)", name, len(content2.Polylines[name])))
		}
	} else {
		detailedLog = append(detailedLog, "  No polyline layers unique to template")
	}
	if len(diffP) > 0 {
		detailedLog = append(detailedLog, "  Polylines with differences in common layers:")
		diffPKeys := make([]string, 0, len(diffP))
		for k := range diffP { diffPKeys = append(diffPKeys, k) }
		sort.Strings(diffPKeys)
		for _, name := range diffPKeys {
			d := diffP[name]
			detailedLog = append(detailedLog, fmt.Sprintf("    - %s:", name))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d polylines only in file", len(d.OnlyIn1)))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d polylines only in template", len(d.OnlyIn2)))
			detailedLog = append(detailedLog, fmt.Sprintf("      * %d common polylines", len(d.Common)))
		}
	} else {
		detailedLog = append(detailedLog, "  No differences in common polyline layers")
	}
	detailedLog = append(detailedLog, "")

	// Summary
	hasDiffs := len(onlyIn1B) > 0 || len(onlyIn2B) > 0 || len(diffB) > 0 ||
		len(onlyIn1L) > 0 || len(onlyIn2L) > 0 || len(diffL) > 0 ||
		len(onlyIn1P) > 0 || len(onlyIn2P) > 0 || len(diffP) > 0

	detailedLog = append(detailedLog, "5. COMPARISON SUMMARY")
	if hasDiffs {
		detailedLog = append(detailedLog, fmt.Sprintf("  RESULT: DIFFERENT - '%s' has differences from template", fileName))
		detailedLog = append(detailedLog, "  File will be copied to template folder and organized into mod folders")
	} else {
		detailedLog = append(detailedLog, fmt.Sprintf("  RESULT: MATCH - '%s' is identical to template", fileName))
		detailedLog = append(detailedLog, "  File will be copied to the template folder")
	}

	return strings.Join(detailedLog, "\n")
}

// applyMatch handles a matching file (sequential — writes to disk and processor state)
func (c *ComparisonProcessor) applyMatch(d fileDecision) {
	safeName := sanitizeFilename(d.TemplateName)
	dir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(dir, 0755)
	dst := filepath.Join(dir, d.FileName)
	// Python: copy_file(dxf_file, target_path) — log_callback is None
	copyFileSilent(d.FilePath, dst)
	c.templateDirectCopies[safeName] = append(c.templateDirectCopies[safeName], d.FileName)
}

// applyNoTemplate handles a file with no matching template
func (c *ComparisonProcessor) applyNoTemplate(d fileDecision) {
	notemplateDir := filepath.Join(c.OutputFolder, "notemplate")
	os.MkdirAll(notemplateDir, 0755)
	dst := filepath.Join(notemplateDir, d.FileName)
	// Python: copy_file(dxf_file, target_path) — log_callback is None
	copyFileSilent(d.FilePath, dst)
}

// applyDifferent handles a file that differs from template (sequential)
func (c *ComparisonProcessor) applyDifferent(d fileDecision) {
	safeName := sanitizeFilename(d.TemplateName)
	templateDir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(templateDir, 0755)
	tempTargetPath := filepath.Join(templateDir, d.FileName)
	// Python: copy_file(dxf_path, temp_target_path) — log_callback is None
	copyFileSilent(d.FilePath, tempTargetPath)

	if c.GroupByContent && d.ContentHash != "" {
		if c.templateFileHashes[d.TemplateName] == nil {
			c.templateFileHashes[d.TemplateName] = make(map[string][]fileInfo)
		}
		c.templateFileHashes[d.TemplateName][d.ContentHash] = append(c.templateFileHashes[d.TemplateName][d.ContentHash], fileInfo{
			FilePath: tempTargetPath,
			FileName: d.FileName,
		})
	} else {
		if c.templateFileHashes[d.TemplateName] == nil {
			c.templateFileHashes[d.TemplateName] = make(map[string][]fileInfo)
		}
		c.templateFileHashes[d.TemplateName]["default"] = append(c.templateFileHashes[d.TemplateName]["default"], fileInfo{
			FilePath: tempTargetPath,
			FileName: d.FileName,
		})
	}

	c.templateDetailedLogs[d.TemplateName] = append(c.templateDetailedLogs[d.TemplateName], detailedLogEntry{
		Filename:    d.FileName,
		LogContent:  d.DetailedLog,
		Destination: d.ContentHash,
	})
}

