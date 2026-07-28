package compare

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// TemplateMap maps template names to file paths
type TemplateMap map[string]string

// Result represents the comparison result for a single file
type Result struct {
	FileName    string `json:"file_name"`
	Template    string `json:"template"`
	Status      string `json:"status"` // "match", "different", "no_template"
	ContentHash string `json:"content_hash,omitempty"`
	ModFolder   string `json:"mod_folder,omitempty"`
}

// ComparisonProcessor handles the full comparison workflow
type ComparisonProcessor struct {
	TemplateMap     TemplateMap
	OutputFolder     string
	MoveFiles        bool
	GroupByContent  bool
	LogCallback     func(string)

	// Internal state
	templateFileHashes map[string]map[string][]string // template → hash → file paths
	modFolders         []string
	directCopies       map[string][]string  // template → file paths (identical)
	detailedLogs       map[string][]string  // template → log lines
}

// NewComparisonProcessor creates a new processor
func NewComparisonProcessor(templateMap TemplateMap, outputFolder string, moveFiles bool, groupByContent bool, logFn func(string)) *ComparisonProcessor {
	return &ComparisonProcessor{
		TemplateMap:        templateMap,
		OutputFolder:       outputFolder,
		MoveFiles:          moveFiles,
		GroupByContent:     groupByContent,
		LogCallback:        logFn,
		templateFileHashes: make(map[string]map[string][]string),
		modFolders:         []string{},
		directCopies:       make(map[string][]string),
		detailedLogs:       make(map[string][]string),
	}
}

func (c *ComparisonProcessor) log(msg string) {
	if c.LogCallback != nil {
		c.LogCallback(msg)
	}
}

// ProcessFile processes a single DXF file, comparing it to templates
func (c *ComparisonProcessor) ProcessFile(dxfFile string) Result {
	fileName := filepath.Base(dxfFile)
	c.log(fmt.Sprintf("Processing: %s", fileName))

	// Parse the module DXF
	drawing, err := dxf.ReadFile(dxfFile)
	if err != nil {
		c.log(fmt.Sprintf("Error reading %s: %v", fileName, err))
		return Result{FileName: fileName, Template: "error", Status: "error"}
	}

	// Extract $(TEMPLATE) attribute
	templateName := drawing.GetTemplateAttribute()

	if templateName == "" {
		c.log(fmt.Sprintf("  -> No template found for %s", fileName))
		c.handleNoTemplate(dxfFile)
		return Result{FileName: fileName, Template: "notemplate", Status: "no_template"}
	}

	// Find matching template file
	templatePath, found := c.TemplateMap[templateName]
	if !found {
		c.log(fmt.Sprintf("  -> Template '%s' not found in template map", templateName))
		c.handleNoTemplate(dxfFile)
		return Result{FileName: fileName, Template: "notemplate", Status: "no_template"}
	}

	// Compare module vs template
	identical := c.filesAreIdentical(dxfFile, templatePath)
	if identical {
		c.log(fmt.Sprintf("  -> MATCH: %s is identical to template %s", fileName, templateName))
		c.handleMatchingFile(dxfFile, templateName)
		return Result{FileName: fileName, Template: templateName, Status: "match"}
	}

	c.log(fmt.Sprintf("  -> DIFFERENT: %s has differences from template %s", fileName, templateName))
	c.handleDifferentFile(dxfFile, templatePath, templateName)
	return Result{FileName: fileName, Template: templateName, Status: "different"}
}

// filesAreIdentical checks if two DXF files have identical geometry
func (c *ComparisonProcessor) filesAreIdentical(file1, file2 string) bool {
	content1, err := extractContent(file1)
	if err != nil {
		return false
	}
	content2, err := extractContent(file2)
	if err != nil {
		return false
	}
	hash1 := ContentHash(content1)
	hash2 := ContentHash(content2)
	return hash1 == hash2
}

// handleNoTemplate copies file to output/notemplate/
func (c *ComparisonProcessor) handleNoTemplate(dxfFile string) {
	notemplateDir := filepath.Join(c.OutputFolder, "notemplate")
	os.MkdirAll(notemplateDir, 0755)
	dst := filepath.Join(notemplateDir, filepath.Base(dxfFile))
	copyFile(dxfFile, dst, c.log)
}

// handleMatchingFile copies file to output/template_name/
func (c *ComparisonProcessor) handleMatchingFile(dxfFile, templateName string) {
	safeName := sanitizeFilename(templateName)
	dir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(dir, 0755)
	dst := filepath.Join(dir, filepath.Base(dxfFile))
	copyFile(dxfFile, dst, c.log)
	c.directCopies[templateName] = append(c.directCopies[templateName], filepath.Base(dxfFile))
}

// handleDifferentFile compares and groups the file
func (c *ComparisonProcessor) handleDifferentFile(dxfFile, templatePath, templateName string) {
	safeName := sanitizeFilename(templateName)
	dir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(dir, 0755)

	// Copy to template folder first
	dst := filepath.Join(dir, filepath.Base(dxfFile))
	copyFile(dxfFile, dst, c.log)

	// Group by content hash
	if c.GroupByContent {
		content, err := extractContent(dxfFile)
		if err == nil {
			hash := ContentHash(content)
			if c.templateFileHashes[templateName] == nil {
				c.templateFileHashes[templateName] = make(map[string][]string)
			}
			c.templateFileHashes[templateName][hash] = append(c.templateFileHashes[templateName][hash], dst)
			c.log(fmt.Sprintf("  -> Grouped with content hash: %s", hash[:8]))
		}
	}

	// Generate detailed comparison log
	c.generateDetailedLog(dxfFile, templatePath, templateName)
}

// Finalize creates _modN folders and moves files
func (c *ComparisonProcessor) Finalize() {
	c.log("Finalizing comparison and organizing mod folders...")

	for templateName, hashGroups := range c.templateFileHashes {
		safeName := sanitizeFilename(templateName)
		templateDir := filepath.Join(c.OutputFolder, safeName)

		// Sort hashes for deterministic ordering
		hashes := make([]string, 0, len(hashGroups))
		for h := range hashGroups {
			if len(hashGroups[h]) > 0 {
				hashes = append(hashes, h)
			}
		}
		sort.Strings(hashes)

		c.log(fmt.Sprintf("Processing template: %s with %d different content groups", templateName, len(hashes)))

		for i, hash := range hashes {
			files := hashGroups[hash]
			modName := fmt.Sprintf("_mod%d", i+1)
			modDir := filepath.Join(templateDir, modName)
			os.MkdirAll(modDir, 0755)

			c.log(fmt.Sprintf("  -> Using mod folder: %s (%d files)", modName, len(files)))

			for _, file := range files {
				fileName := filepath.Base(file)
				dst := filepath.Join(modDir, fileName)
				if c.MoveFiles {
					moveFile(file, dst, c.log)
				} else {
					copyFile(file, dst, c.log)
				}
			}
			c.modFolders = append(c.modFolders, modName)
		}
	}

	c.log(fmt.Sprintf("Created %d mod folders in total", len(c.modFolders)))
}

// GetModFolderCount returns the number of created mod folders
func (c *ComparisonProcessor) GetModFolderCount() int {
	return len(c.modFolders)
}

// RunComparison runs the full comparison workflow
func RunComparison(templateMap TemplateMap, searchFolder, outputFolder string, recursive, moveFiles, groupByContent bool, progressFn func(int, int) bool, logFn func(string)) []Result {
	processor := NewComparisonProcessor(templateMap, outputFolder, moveFiles, groupByContent, logFn)

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

	logFn(fmt.Sprintf("Found %d DXF files to process", len(dxfFiles)))

	results := make([]Result, 0, len(dxfFiles))
	for i, f := range dxfFiles {
		if progressFn != nil && !progressFn(i, len(dxfFiles)) {
			break
		}
		result := processor.ProcessFile(f)
		results = append(results, result)
	}

	processor.Finalize()

	if progressFn != nil {
		progressFn(len(dxfFiles), len(dxfFiles))
	}

	logFn(fmt.Sprintf("Processing complete. Processed %d files.", len(dxfFiles)))
	return results
}

// --- Helper functions ---

func extractContent(path string) (*dxf.DXFContent, error) {
	drawing, err := dxf.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return drawing.ExtractContent(3), nil
}

// ContentHash creates an MD5 hash from DXF content data
func ContentHash(content *dxf.DXFContent) string {
	data, err := json.Marshal(struct {
		Blocks    map[string][][3]float64      `json:"blocks"`
		Lines     map[string][][2][3]float64    `json:"lines"`
		Polylines map[string][][3]float64       `json:"polylines"`
	}{
		Blocks:    content.Blocks,
		Lines:     content.Lines,
		Polylines: content.Polylines,
	})
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", md5.Sum(data))
}

func sanitizeFilename(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func copyFile(src, dst string, log func(string)) {
	data, err := os.ReadFile(src)
	if err != nil {
		log(fmt.Sprintf("ERROR reading file: %v", err))
		return
	}
	err = os.WriteFile(dst, data, 0644)
	if err != nil {
		log(fmt.Sprintf("ERROR writing file: %v", err))
		return
	}
	log(fmt.Sprintf("Copied to: %s", dst))
}

func moveFile(src, dst string, log func(string)) {
	err := os.Rename(src, dst)
	if err != nil {
		// Fallback: copy + remove
		copyFile(src, dst, log)
		os.Remove(src)
	}
	log(fmt.Sprintf("Moved to: %s", dst))
}

func (c *ComparisonProcessor) generateDetailedLog(dxfFile, templatePath, templateName string) {
	safeName := sanitizeFilename(templateName)
	content1, err1 := extractContent(dxfFile)
	content2, err2 := extractContent(templatePath)

	var logLines []string
	logLines = append(logLines, "========== DETAILED COMPARISON LOG ==========")
	logLines = append(logLines, fmt.Sprintf("File: %s", filepath.Base(dxfFile)))
	logLines = append(logLines, fmt.Sprintf("Template: %s", templateName))

	if err1 != nil || err2 != nil {
		logLines = append(logLines, "Error reading DXF data")
	} else {
		// Entity counts
		logLines = append(logLines, "1. ENTITY COUNTS")
		logLines = append(logLines, fmt.Sprintf("  File Blocks: %d block types", len(content1.Blocks)))
		logLines = append(logLines, fmt.Sprintf("  Template Blocks: %d block types", len(content2.Blocks)))
		logLines = append(logLines, fmt.Sprintf("  File Lines: %d layers", len(content1.Lines)))
		logLines = append(logLines, fmt.Sprintf("  Template Lines: %d layers", len(content2.Lines)))

		// Block differences
		logLines = append(logLines, "2. BLOCK DIFFERENCES")
		compareKeys(&logLines, content1.Blocks, content2.Blocks, "Block types")

		// Line differences
		logLines = append(logLines, "3. LINE DIFFERENCES")
		compareKeys(&logLines, content1.Lines, content2.Lines, "Line layers")

		// Summary
		logLines = append(logLines, "5. COMPARISON SUMMARY")
		logLines = append(logLines, fmt.Sprintf("  RESULT: DIFFERENT - '%s' has differences from template", filepath.Base(dxfFile)))
	}

	logLines = append(logLines, "==========================================")

	c.detailedLogs[templateName] = append(c.detailedLogs[templateName], logLines...)

	// Save to file
	logPath := filepath.Join(c.OutputFolder, safeName, safeName+"_dxfanalyze.log")
	os.WriteFile(logPath, []byte(strings.Join(logLines, "\n")), 0644)
}

func compareKeys[V any](logLines *[]string, map1, map2 map[string]V, label string) {
	keys1 := make(map[string]bool)
	keys2 := make(map[string]bool)
	for k := range map1 {
		keys1[k] = true
	}
	for k := range map2 {
		keys2[k] = true
	}

	onlyIn1 := []string{}
	onlyIn2 := []string{}
	for k := range keys1 {
		if !keys2[k] {
			onlyIn1 = append(onlyIn1, k)
		}
	}
	for k := range keys2 {
		if !keys1[k] {
			onlyIn2 = append(onlyIn2, k)
		}
	}
	sort.Strings(onlyIn1)
	sort.Strings(onlyIn2)

	if len(onlyIn1) > 0 {
		*logLines = append(*logLines, fmt.Sprintf("  %s only in file:", label))
		for _, k := range onlyIn1 {
			*logLines = append(*logLines, fmt.Sprintf("    - %s", k))
		}
	}
	if len(onlyIn2) > 0 {
		*logLines = append(*logLines, fmt.Sprintf("  %s only in template:", label))
		for _, k := range onlyIn2 {
			*logLines = append(*logLines, fmt.Sprintf("    - %s", k))
		}
	}
}