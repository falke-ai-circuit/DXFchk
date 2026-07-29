package compare

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

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
	logFn(fmt.Sprintf("Note: Using %d parallel workers", workers))

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
	logFn(fmt.Sprintf("Processing complete. Processed %d of %d files.", matched+different+noTemplate, total))
	logFn(fmt.Sprintf("=== SUMMARY: %d matched, %d different, %d no template ===", matched, different, noTemplate))

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

	if templateName == "" || templateName == "notemplate" {
		logFn(fmt.Sprintf("  -> No template found for %s", fileName))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "no_template", TemplateName: "notemplate"}
	}

	templatePath, found := templateMap[templateName]
	if !found {
		logFn(fmt.Sprintf("  -> No template found for %s", fileName))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "no_template", TemplateName: "notemplate"}
	}

	// Compare
	identical, content1, _ := compareFilesParallel(dxfFile, templatePath, logFn)
	if identical {
		logFn(fmt.Sprintf("  -> Using template: %s", templateName))
		logFn(fmt.Sprintf("  -> MATCH: %s is identical to template", fileName))
		return fileDecision{FileName: fileName, FilePath: dxfFile, Status: "match", TemplateName: templateName}
	}

	// Different — compute hash and build detailed log
	contentHash := ""
	if groupByContent && content1 != nil {
		contentHash = ContentHash(content1)
		logFn(fmt.Sprintf("  -> Grouped with content hash: %s", contentHash[:8]))
	}

	// Build detailed log
	detailedLog := buildDetailedLog(fileName, dxfFile, templateName, templatePath, content1)

	logFn(fmt.Sprintf("  -> DIFFERENT: %s has differences from template", fileName))

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
func buildDetailedLog(fileName, dxfFile, templateName, templatePath string, content1 *dxf.DXFContent) string {
	var detailedLog []string
	detailedLog = append(detailedLog, "========== DETAILED COMPARISON LOG ==========")
	detailedLog = append(detailedLog, fmt.Sprintf("File: %s", fileName))
	detailedLog = append(detailedLog, fmt.Sprintf("Template: %s (%s)", templateName, filepath.Base(templatePath)))
	detailedLog = append(detailedLog, "")
	detailedLog = append(detailedLog, "Result: DIFFERENT from template")
	detailedLog = append(detailedLog, "==========================================")
	return strings.Join(detailedLog, "\n")
}

// applyMatch handles a matching file (sequential — writes to disk and processor state)
func (c *ComparisonProcessor) applyMatch(d fileDecision) {
	safeName := sanitizeFilename(d.TemplateName)
	dir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(dir, 0755)
	dst := filepath.Join(dir, d.FileName)
	copyFile(d.FilePath, dst, c.log)
	c.templateDirectCopies[safeName] = append(c.templateDirectCopies[safeName], d.FileName)
}

// applyNoTemplate handles a file with no matching template
func (c *ComparisonProcessor) applyNoTemplate(d fileDecision) {
	notemplateDir := filepath.Join(c.OutputFolder, "notemplate")
	os.MkdirAll(notemplateDir, 0755)
	dst := filepath.Join(notemplateDir, d.FileName)
	copyFile(d.FilePath, dst, c.log)
}

// applyDifferent handles a file that differs from template (sequential)
func (c *ComparisonProcessor) applyDifferent(d fileDecision) {
	safeName := sanitizeFilename(d.TemplateName)
	templateDir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(templateDir, 0755)
	tempTargetPath := filepath.Join(templateDir, d.FileName)
	copyFile(d.FilePath, tempTargetPath, c.log)

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

