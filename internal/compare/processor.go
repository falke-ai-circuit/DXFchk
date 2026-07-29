package compare

import (
	"crypto/md5"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

// fileInfo holds file path and name for hash grouping (mirrors Python dict)
type fileInfo struct {
	FilePath string
	FileName string
}

// ComparisonProcessor handles the full comparison workflow
// Port of Python ComparisonProcessor class
type ComparisonProcessor struct {
	TemplateMap    TemplateMap
	OutputFolder   string
	MoveFiles      bool
	GroupByContent bool
	LogCallback    func(string)

	// Internal state (mirrors Python)
	templateFileHashes  map[string]map[string][]fileInfo // template → hash → files
	modFolders          map[string]bool                   // set of mod folder paths
	templateDirectCopies map[string][]string             // sanitized_name → filenames
	templateDetailedLogs map[string][]detailedLogEntry   // template_name → log entries
}

// detailedLogEntry mirrors Python's (filename, log_content, destination_flag) tuple
type detailedLogEntry struct {
	Filename     string
	LogContent   string
	Destination  string // "template", hash string, or "" (None)
}

// NewComparisonProcessor creates a new processor
func NewComparisonProcessor(templateMap TemplateMap, outputFolder string, moveFiles bool, groupByContent bool, logFn func(string)) *ComparisonProcessor {
	// Python creates notemplate folder in __init__
	notemplateDir := filepath.Join(outputFolder, "notemplate")
	os.MkdirAll(notemplateDir, 0755)

	return &ComparisonProcessor{
		TemplateMap:          templateMap,
		OutputFolder:         outputFolder,
		MoveFiles:            moveFiles,
		GroupByContent:       groupByContent,
		LogCallback:          logFn,
		templateFileHashes:   make(map[string]map[string][]fileInfo),
		modFolders:           make(map[string]bool),
		templateDirectCopies: make(map[string][]string),
		templateDetailedLogs: make(map[string][]detailedLogEntry),
	}
}

func (c *ComparisonProcessor) log(msg string) {
	if c.LogCallback != nil {
		c.LogCallback(msg)
	}
}

// ProcessFile processes a single DXF file, comparing it to templates.
// Port of Python process_file()
// Recovers from panics to prevent one bad DXF from crashing the entire run.
func (c *ComparisonProcessor) ProcessFile(dxfFile string) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			c.log(fmt.Sprintf("ERROR: panic processing %s: %v", filepath.Base(dxfFile), r))
			result = Result{FileName: filepath.Base(dxfFile), Template: "error", Status: "error"}
		}
	}()
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

	// Python fallback: if no $(TEMPLATE) attr, try matching by filename prefix
	if templateName == "" {
		for name := range c.TemplateMap {
			if name == "" || strings.TrimSpace(name) == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(fileName), strings.ToLower(name)) {
				// Pick longest matching name (like Python)
				templateName = name
				break
			}
		}
		// Find longest match
		bestMatch := ""
		for name := range c.TemplateMap {
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

	if templateName == "" || templateName == "notemplate" {
		c.handleNoTemplate(dxfFile)
		return Result{FileName: fileName, Template: "notemplate", Status: "no_template"}
	}

	// Find matching template file
	templatePath, found := c.TemplateMap[templateName]
	if !found {
		c.handleNoTemplate(dxfFile)
		return Result{FileName: fileName, Template: "notemplate", Status: "no_template"}
	}

	// Compare using compare_dict_of_lists approach (like Python)
	identical := c.filesAreIdentical(dxfFile, templatePath)
	if identical {
		c.handleMatchingFile(dxfFile, templateName)
		return Result{FileName: fileName, Template: templateName, Status: "match"}
	}

	c.handleDifferentFile(dxfFile, templatePath, templateName)
	return Result{FileName: fileName, Template: templateName, Status: "different"}
}

// filesAreIdentical checks if two DXF files have identical geometry
// Port of Python _files_are_identical using compare_dict_of_lists
func (c *ComparisonProcessor) filesAreIdentical(file1, file2 string) bool {
	content1, err := extractContent(file1)
	if err != nil {
		c.log(fmt.Sprintf("Error comparing files: %v", err))
		return false
	}
	content2, err := extractContent(file2)
	if err != nil {
		c.log(fmt.Sprintf("Error comparing files: %v", err))
		return false
	}

	// Compare blocks
	_, onlyIn1B, onlyIn2B, diffB := compareDictOfLists(content1.Blocks, content2.Blocks)
	// Compare lines
	_, onlyIn1L, onlyIn2L, diffL := compareDictOfListsLines(content1.Lines, content2.Lines)
	// Compare polylines
	_, onlyIn1P, onlyIn2P, diffP := compareDictOfListsPolylines(content1.Polylines, content2.Polylines)

	return len(onlyIn1B) == 0 && len(onlyIn2B) == 0 && len(diffB) == 0 &&
		len(onlyIn1L) == 0 && len(onlyIn2L) == 0 && len(diffL) == 0 &&
		len(onlyIn1P) == 0 && len(onlyIn2P) == 0 && len(diffP) == 0
}

// handleNoTemplate copies file to output/notemplate/
func (c *ComparisonProcessor) handleNoTemplate(dxfFile string) {
	fileName := filepath.Base(dxfFile)
	c.log(fmt.Sprintf("  -> No template found for %s", fileName))

	notemplateDir := filepath.Join(c.OutputFolder, "notemplate")
	os.MkdirAll(notemplateDir, 0755)
	dst := filepath.Join(notemplateDir, fileName)
	copyFile(dxfFile, dst, c.log)
}

// handleMatchingFile copies file to output/template_name/
func (c *ComparisonProcessor) handleMatchingFile(dxfFile, templateName string) {
	fileName := filepath.Base(dxfFile)
	c.log(fmt.Sprintf("  -> Using template: %s", templateName))
	c.log(fmt.Sprintf("  -> MATCH: %s is identical to template", fileName))

	safeName := sanitizeFilename(templateName)
	dir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(dir, 0755)
	dst := filepath.Join(dir, fileName)
	copyFile(dxfFile, dst, c.log)

	c.templateDirectCopies[safeName] = append(c.templateDirectCopies[safeName], fileName)
}

// handleDifferentFile compares and groups the file
// Port of Python _compare_with_template
func (c *ComparisonProcessor) handleDifferentFile(dxfFile, templatePath, templateName string) {
	fileName := filepath.Base(dxfFile)
	safeName := sanitizeFilename(templateName)

	content1, err1 := extractContent(dxfFile)
	content2, err2 := extractContent(templatePath)

	var detailedLog []string
	detailedLog = append(detailedLog, "========== DETAILED COMPARISON LOG ==========")
	detailedLog = append(detailedLog, fmt.Sprintf("File: %s", fileName))
	detailedLog = append(detailedLog, fmt.Sprintf("Template: %s (%s)", templateName, filepath.Base(templatePath)))
	detailedLog = append(detailedLog, fmt.Sprintf("Comparison Time: %s", time.Now().Format("2006-01-02 15:04:05")))
	detailedLog = append(detailedLog, "==========================================\n")

	if err1 != nil || err2 != nil {
		errMsg := fmt.Sprintf("  -> Error reading DXF data from %s or template", fileName)
		c.log(errMsg)
		detailedLog = append(detailedLog, errMsg)
		detailedLog = append(detailedLog, "Processing failed - could not read DXF data")
		c.templateDetailedLogs[templateName] = append(c.templateDetailedLogs[templateName], detailedLogEntry{
			Filename:   fileName,
			LogContent: strings.Join(detailedLog, "\n"),
			Destination: "",
		})
		return
	}

	// Entity counts
	blocksCount1 := 0
	for _, v := range content1.Blocks {
		blocksCount1 += len(v)
	}
	blocksCount2 := 0
	for _, v := range content2.Blocks {
		blocksCount2 += len(v)
	}
	linesCount1 := 0
	for _, v := range content1.Lines {
		linesCount1 += len(v)
	}
	linesCount2 := 0
	for _, v := range content2.Lines {
		linesCount2 += len(v)
	}
	polyCount1 := 0
	for _, v := range content1.Polylines {
		polyCount1 += len(v)
	}
	polyCount2 := 0
	for _, v := range content2.Polylines {
		polyCount2 += len(v)
	}

	detailedLog = append(detailedLog, "1. ENTITY COUNTS")
	detailedLog = append(detailedLog, fmt.Sprintf("  File Blocks: %d in %d block types", blocksCount1, len(content1.Blocks)))
	detailedLog = append(detailedLog, fmt.Sprintf("  Template Blocks: %d in %d block types", blocksCount2, len(content2.Blocks)))
	detailedLog = append(detailedLog, fmt.Sprintf("  File Lines: %d in %d layers", linesCount1, len(content1.Lines)))
	detailedLog = append(detailedLog, fmt.Sprintf("  Template Lines: %d in %d layers", linesCount2, len(content2.Lines)))
	detailedLog = append(detailedLog, fmt.Sprintf("  File Polylines: %d in %d layers", polyCount1, len(content1.Polylines)))
	detailedLog = append(detailedLog, fmt.Sprintf("  Template Polylines: %d in %d layers", polyCount2, len(content2.Polylines)))
	detailedLog = append(detailedLog, "")

	// Compare blocks
	commonB, onlyIn1B, onlyIn2B, diffB := compareDictOfLists(content1.Blocks, content2.Blocks)
	commonL, onlyIn1L, onlyIn2L, diffL := compareDictOfListsLines(content1.Lines, content2.Lines)
	commonP, onlyIn1P, onlyIn2P, diffP := compareDictOfListsPolylines(content1.Polylines, content2.Polylines)

	_ = commonB
	_ = commonL
	_ = commonP

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
		for k := range diffB {
			diffBKeys = append(diffBKeys, k)
		}
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
		for k := range diffL {
			diffLKeys = append(diffLKeys, k)
		}
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
		for k := range diffP {
			diffPKeys = append(diffPKeys, k)
		}
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

	// Has differences check
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

	// Copy to template folder first
	templateDir := filepath.Join(c.OutputFolder, safeName)
	os.MkdirAll(templateDir, 0755)
	tempTargetPath := filepath.Join(templateDir, fileName)
	// Push diff summary to live log (matches Python lines 373-374)
	if len(onlyIn1B) > 0 || len(onlyIn2B) > 0 || len(diffB) > 0 {
		c.log(fmt.Sprintf("  -> Found differences in blocks: %d block type(s) with differences", len(onlyIn1B)+len(onlyIn2B)+len(diffB)))
	}
	if len(onlyIn1L) > 0 || len(onlyIn2L) > 0 || len(diffL) > 0 {
		c.log(fmt.Sprintf("  -> Found differences in lines: %d layer(s) with differences", len(onlyIn1L)+len(onlyIn2L)+len(diffL)))
	}
	if len(onlyIn1P) > 0 || len(onlyIn2P) > 0 || len(diffP) > 0 {
		c.log(fmt.Sprintf("  -> Found differences in polylines: %d layer(s) with differences", len(onlyIn1P)+len(onlyIn2P)+len(diffP)))
	}

	c.log(fmt.Sprintf("  -> DIFFERENT: %s has differences from template", fileName))
	copyFile(dxfFile, tempTargetPath, c.log)

	var contentHash string
	if c.GroupByContent {
		contentHash = ContentHash(content1)
		if c.templateFileHashes[templateName] == nil {
			c.templateFileHashes[templateName] = make(map[string][]fileInfo)
		}
		c.templateFileHashes[templateName][contentHash] = append(c.templateFileHashes[templateName][contentHash], fileInfo{
			FilePath: tempTargetPath,
			FileName: fileName,
		})
		c.log(fmt.Sprintf("  -> Grouped with content hash: %s", contentHash[:8]))
	} else {
		c.templateFileHashes[templateName]["default"] = append(c.templateFileHashes[templateName]["default"], fileInfo{
			FilePath: tempTargetPath,
			FileName: fileName,
		})
	}

	// Store detailed log
	dest := contentHash
	if !hasDiffs {
		dest = "template"
	}
	c.templateDetailedLogs[templateName] = append(c.templateDetailedLogs[templateName], detailedLogEntry{
		Filename:   fileName,
		LogContent: strings.Join(detailedLog, "\n"),
		Destination: dest,
	})
}

// Finalize creates mod folders and moves files (like Python finalize())
func (c *ComparisonProcessor) Finalize() {
	c.log("Finalizing comparison and organizing mod folders...")

	// Save detailed logs first (like Python)
	c.saveDetailedLogs()

	// Organize mod folders
	for templateName, hashFiles := range c.templateFileHashes {
		safeName := sanitizeFilename(templateName)
		c.log(fmt.Sprintf("Processing template: %s with %d different content groups", templateName, len(hashFiles)))

		// Sort hashes for deterministic ordering
		hashes := make([]string, 0, len(hashFiles))
		for h := range hashFiles {
			if len(hashFiles[h]) > 0 {
				hashes = append(hashes, h)
			}
		}
		sort.Strings(hashes)

		for i, hash := range hashes {
			files := hashFiles[hash]
			// Python: mod_folder_name = f"{sanitized_template_name}_mod{mod_idx}"
			modFolderName := fmt.Sprintf("%s_mod%d", safeName, i+1)
			// Nest _modN under the template name folder: Output/BI001/BI001_mod1/
			modDir := filepath.Join(c.OutputFolder, safeName, modFolderName)
			os.MkdirAll(modDir, 0755)

			c.log(fmt.Sprintf("  -> Using mod folder: %s with %d files", modFolderName, len(files)))

			for _, fi := range files {
				if _, err := os.Stat(fi.FilePath); os.IsNotExist(err) {
					continue
				}
				targetPath := filepath.Join(modDir, fi.FileName)
				// Python uses move_file (copy + delete original)
				moveFile(fi.FilePath, targetPath, c.log)
				c.log(fmt.Sprintf("    -> Moved %s from template folder to %s", fi.FileName, modFolderName))
			}
			c.modFolders[modDir] = true
		}
	}

	// Ensure all folders have logs (like Python _ensure_all_folders_have_logs)
	c.ensureAllFoldersHaveLogs()

	c.log(fmt.Sprintf("Created %d mod folders in total", len(c.modFolders)))
}

// GetModFolderCount returns the number of created mod folders
func (c *ComparisonProcessor) GetModFolderCount() int {
	return len(c.modFolders)
}

// RunComparison runs the full comparison workflow
func RunComparison(templateMap TemplateMap, searchFolder, outputFolder string, recursive, moveFiles, groupByContent bool, progressFn func(int, int) bool, logFn func(string)) []Result {
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

	matched := 0
	different := 0
	noTemplate := 0
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
	logFn(fmt.Sprintf("Processing complete. Processed %d of %d files.", matched+different+noTemplate, len(dxfFiles)))
	logFn(fmt.Sprintf("=== SUMMARY: %d matched, %d different, %d no template ===", matched, different, noTemplate))

	return results
}

// --- Comparison helpers (port of Python compare_dict_of_lists) ---

// DiffResult holds the comparison result for one key
type DiffResult struct {
	Common  []string
	OnlyIn1 []string
	OnlyIn2 []string
}

// compareDictOfLists compares blocks: map[string][][3]float64
func compareDictOfLists(dict1, dict2 map[string][][3]float64) ([]string, []string, []string, map[string]DiffResult) {
	keys1 := make(map[string]bool)
	keys2 := make(map[string]bool)
	for k := range dict1 {
		keys1[k] = true
	}
	for k := range dict2 {
		keys2[k] = true
	}

	commonKeys := []string{}
	onlyIn1 := []string{}
	onlyIn2 := []string{}

	for k := range keys1 {
		if keys2[k] {
			commonKeys = append(commonKeys, k)
		} else {
			onlyIn1 = append(onlyIn1, k)
		}
	}
	for k := range keys2 {
		if !keys1[k] {
			onlyIn2 = append(onlyIn2, k)
		}
	}
	sort.Strings(commonKeys)
	sort.Strings(onlyIn1)
	sort.Strings(onlyIn2)

	diff := make(map[string]DiffResult)
	for _, k := range commonKeys {
		set1 := make(map[[3]float64]bool)
		set2 := make(map[[3]float64]bool)
		for _, v := range dict1[k] {
			set1[v] = true
		}
		for _, v := range dict2[k] {
			set2[v] = true
		}

		common := []string{}
		onlyIn1ForK := []string{}
		onlyIn2ForK := []string{}

		for v := range set1 {
			if set2[v] {
				common = append(common, fmt.Sprintf("%v", v))
			} else {
				onlyIn1ForK = append(onlyIn1ForK, fmt.Sprintf("%v", v))
			}
		}
		for v := range set2 {
			if !set1[v] {
				onlyIn2ForK = append(onlyIn2ForK, fmt.Sprintf("%v", v))
			}
		}

		sort.Strings(common)
		sort.Strings(onlyIn1ForK)
		sort.Strings(onlyIn2ForK)

		if len(onlyIn1ForK) > 0 || len(onlyIn2ForK) > 0 {
			diff[k] = DiffResult{Common: common, OnlyIn1: onlyIn1ForK, OnlyIn2: onlyIn2ForK}
		}
	}

	return commonKeys, onlyIn1, onlyIn2, diff
}

// compareDictOfListsLines compares lines: map[string][][2][3]float64
func compareDictOfListsLines(dict1, dict2 map[string][][2][3]float64) ([]string, []string, []string, map[string]DiffResult) {
	keys1 := make(map[string]bool)
	keys2 := make(map[string]bool)
	for k := range dict1 {
		keys1[k] = true
	}
	for k := range dict2 {
		keys2[k] = true
	}

	commonKeys := []string{}
	onlyIn1 := []string{}
	onlyIn2 := []string{}

	for k := range keys1 {
		if keys2[k] {
			commonKeys = append(commonKeys, k)
		} else {
			onlyIn1 = append(onlyIn1, k)
		}
	}
	for k := range keys2 {
		if !keys1[k] {
			onlyIn2 = append(onlyIn2, k)
		}
	}
	sort.Strings(commonKeys)
	sort.Strings(onlyIn1)
	sort.Strings(onlyIn2)

	diff := make(map[string]DiffResult)
	for _, k := range commonKeys {
		set1 := make(map[[2][3]float64]bool)
		set2 := make(map[[2][3]float64]bool)
		for _, v := range dict1[k] {
			set1[v] = true
		}
		for _, v := range dict2[k] {
			set2[v] = true
		}

		common := []string{}
		onlyIn1ForK := []string{}
		onlyIn2ForK := []string{}

		for v := range set1 {
			if set2[v] {
				common = append(common, fmt.Sprintf("%v", v))
			} else {
				onlyIn1ForK = append(onlyIn1ForK, fmt.Sprintf("%v", v))
			}
		}
		for v := range set2 {
			if !set1[v] {
				onlyIn2ForK = append(onlyIn2ForK, fmt.Sprintf("%v", v))
			}
		}

		sort.Strings(common)
		sort.Strings(onlyIn1ForK)
		sort.Strings(onlyIn2ForK)

		if len(onlyIn1ForK) > 0 || len(onlyIn2ForK) > 0 {
			diff[k] = DiffResult{Common: common, OnlyIn1: onlyIn1ForK, OnlyIn2: onlyIn2ForK}
		}
	}

	return commonKeys, onlyIn1, onlyIn2, diff
}

// compareDictOfListsPolylines compares polylines: map[string][][][3]float64
// Each entry is a polyline (list of vertices). We compare as sets of vertex-slices.
func compareDictOfListsPolylines(dict1, dict2 map[string][][][3]float64) ([]string, []string, []string, map[string]DiffResult) {
	keys1 := make(map[string]bool)
	keys2 := make(map[string]bool)
	for k := range dict1 {
		keys1[k] = true
	}
	for k := range dict2 {
		keys2[k] = true
	}

	commonKeys := []string{}
	onlyIn1 := []string{}
	onlyIn2 := []string{}

	for k := range keys1 {
		if keys2[k] {
			commonKeys = append(commonKeys, k)
		} else {
			onlyIn1 = append(onlyIn1, k)
		}
	}
	for k := range keys2 {
		if !keys1[k] {
			onlyIn2 = append(onlyIn2, k)
		}
	}
	sort.Strings(commonKeys)
	sort.Strings(onlyIn1)
	sort.Strings(onlyIn2)

	diff := make(map[string]DiffResult)
	for _, k := range commonKeys {
		// Compare as sets of polylines (each polyline is a [][3]float64)
		set1 := make(map[string]bool)
		set2 := make(map[string]bool)
		for _, v := range dict1[k] {
			set1[fmt.Sprintf("%v", v)] = true
		}
		for _, v := range dict2[k] {
			set2[fmt.Sprintf("%v", v)] = true
		}

		common := []string{}
		onlyIn1ForK := []string{}
		onlyIn2ForK := []string{}

		for v := range set1 {
			if set2[v] {
				common = append(common, v)
			} else {
				onlyIn1ForK = append(onlyIn1ForK, v)
			}
		}
		for v := range set2 {
			if !set1[v] {
				onlyIn2ForK = append(onlyIn2ForK, v)
			}
		}

		sort.Strings(common)
		sort.Strings(onlyIn1ForK)
		sort.Strings(onlyIn2ForK)

		if len(onlyIn1ForK) > 0 || len(onlyIn2ForK) > 0 {
			diff[k] = DiffResult{Common: common, OnlyIn1: onlyIn1ForK, OnlyIn2: onlyIn2ForK}
		}
	}

	return commonKeys, onlyIn1, onlyIn2, diff
}

// --- Detailed log methods ---

func (c *ComparisonProcessor) saveDetailedLogs() {
	// Build file destinations map
	fileDestinations := make(map[string]string)

	for templateName, hashFiles := range c.templateFileHashes {
		safeName := sanitizeFilename(templateName)
		sortedHashItems := make([]string, 0, len(hashFiles))
		for h := range hashFiles {
			sortedHashItems = append(sortedHashItems, h)
		}
		sort.Strings(sortedHashItems)

		for modIdx, hashVal := range sortedHashItems {
			files := hashFiles[hashVal]
			if len(files) == 0 {
				continue
			}
			modFolderName := fmt.Sprintf("%s_mod%d", safeName, modIdx+1)
			nestedPath := filepath.Join(safeName, modFolderName)
			for _, fi := range files {
				fileDestinations[fi.FileName] = nestedPath
			}
		}
	}

	// Notemplate log
	notemplatePath := filepath.Join(c.OutputFolder, "notemplate")
	if _, err := os.Stat(notemplatePath); err == nil {
		logPath := filepath.Join(notemplatePath, "notemplate_dxfanalyze.log")
		var logContent strings.Builder
		logContent.WriteString("DETAILED COMPARISON LOG FOR: notemplate\n")
		logContent.WriteString(fmt.Sprintf("Created: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		logContent.WriteString("Files in this folder have no matching template.\n")
		logContent.WriteString("=============================================\n\n")

		entries, _ := os.ReadDir(notemplatePath)
		var dxfFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dxf") {
				dxfFiles = append(dxfFiles, e.Name())
			}
		}
		logContent.WriteString(fmt.Sprintf("Number of files without templates: %d\n\n", len(dxfFiles)))
		if len(dxfFiles) > 0 {
			logContent.WriteString("Files without templates:\n")
			sort.Strings(dxfFiles)
			for _, f := range dxfFiles {
				logContent.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
		os.WriteFile(logPath, []byte(logContent.String()), 0644)
		c.log(fmt.Sprintf("Saved detailed comparison log to: %s", logPath))
	}

	// Per-template logs
	for templateName, logs := range c.templateDetailedLogs {
		if len(logs) == 0 {
			continue
		}
		safeName := sanitizeFilename(templateName)

		// Group logs by folder
		folderLogs := make(map[string][]string)
		for _, entry := range logs {
			if entry.Destination == "template" {
				folderLogs[safeName] = append(folderLogs[safeName], entry.LogContent)
			} else if entry.Destination == "" {
				continue
			}
			if dest, ok := fileDestinations[entry.Filename]; ok {
				folderLogs[dest] = append(folderLogs[dest], entry.LogContent)
			}
		}

		// Template folder log
		templateFolderPath := filepath.Join(c.OutputFolder, safeName)
		if _, err := os.Stat(templateFolderPath); err == nil {
			logPath := filepath.Join(templateFolderPath, fmt.Sprintf("%s_dxfanalyze.log", safeName))
			var logContent strings.Builder
			logContent.WriteString(fmt.Sprintf("DETAILED COMPARISON LOG FOR: %s\n", safeName))
			logContent.WriteString(fmt.Sprintf("Created: %s\n", time.Now().Format("2006-01-02 15:04:05")))

			directCopies := c.templateDirectCopies[safeName]
			fileCount := len(folderLogs[safeName]) + len(directCopies)
			logContent.WriteString(fmt.Sprintf("Number of files compared: %d\n", fileCount))
			logContent.WriteString("=============================================\n\n")

			if logs, ok := folderLogs[safeName]; ok && len(logs) > 0 {
				logContent.WriteString(strings.Join(logs, "\n\n"))
			}

			if len(directCopies) > 0 {
				logContent.WriteString("\n\n")
				// Find filenames that have detailed logs
				loggedFiles := make(map[string]bool)
				for _, entry := range logs {
					if entry.Destination == "template" {
						loggedFiles[entry.Filename] = true
					}
				}
				for _, fn := range directCopies {
					if !loggedFiles[fn] {
						logContent.WriteString("========== BASIC MATCH INFORMATION ==========\n")
						logContent.WriteString(fmt.Sprintf("File: %s\n", fn))
						logContent.WriteString(fmt.Sprintf("Template: %s\n", templateName))
						logContent.WriteString(fmt.Sprintf("Comparison Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
						logContent.WriteString("==========================================\n\n")
						logContent.WriteString("RESULT: MATCH - File is identical to template\n\n")
					}
				}
			}

			if fileCount == 0 {
				logContent.WriteString("No files matched the template exactly.")
			}

			os.WriteFile(logPath, []byte(logContent.String()), 0644)
			c.log(fmt.Sprintf("Saved detailed comparison log to: %s", logPath))
			delete(folderLogs, safeName)
		}

		// Mod folder logs
		for folderName, folderLogContents := range folderLogs {
			if len(folderLogContents) == 0 {
				continue
			}
			folderPath := filepath.Join(c.OutputFolder, folderName)
			os.MkdirAll(folderPath, 0755)
			// Use only the last path component for the log filename to avoid
			// creating nested subfolders (folderName may contain slashes like
			// "BI001/BI001_mod1")
			logBaseName := filepath.Base(folderName)
			logPath := filepath.Join(folderPath, fmt.Sprintf("%s_dxfanalyze.log", logBaseName))
			var logContent strings.Builder
			logContent.WriteString(fmt.Sprintf("DETAILED COMPARISON LOG FOR: %s\n", logBaseName))
			logContent.WriteString(fmt.Sprintf("Created: %s\n", time.Now().Format("2006-01-02 15:04:05")))
			logContent.WriteString(fmt.Sprintf("Number of files compared: %d\n", len(folderLogContents)))
			logContent.WriteString("=============================================\n\n")
			logContent.WriteString(strings.Join(folderLogContents, "\n\n"))
			os.WriteFile(logPath, []byte(logContent.String()), 0644)
			c.log(fmt.Sprintf("Saved detailed comparison log to: %s", logPath))
		}
	}
}

func (c *ComparisonProcessor) ensureAllFoldersHaveLogs() {
	entries, err := os.ReadDir(c.OutputFolder)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "notemplate" {
			continue
		}
		folderPath := filepath.Join(c.OutputFolder, entry.Name())
		logPath := filepath.Join(folderPath, fmt.Sprintf("%s_dxfanalyze.log", entry.Name()))
		if _, err := os.Stat(logPath); err == nil {
			continue
		}

		subEntries, _ := os.ReadDir(folderPath)
		var dxfFiles []string
		for _, se := range subEntries {
			if !se.IsDir() && strings.HasSuffix(strings.ToLower(se.Name()), ".dxf") {
				dxfFiles = append(dxfFiles, se.Name())
			}
		}
		if len(dxfFiles) == 0 {
			continue
		}

		c.log(fmt.Sprintf("Creating missing log file for folder: %s", entry.Name()))
		var logContent strings.Builder
		logContent.WriteString(fmt.Sprintf("DETAILED COMPARISON LOG FOR: %s\n", entry.Name()))
		logContent.WriteString(fmt.Sprintf("Created: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		logContent.WriteString(fmt.Sprintf("Number of files: %d\n", len(dxfFiles)))
		logContent.WriteString("=============================================\n\n")
		logContent.WriteString("DXF FILES IN THIS FOLDER:\n")
		sort.Strings(dxfFiles)
		for _, f := range dxfFiles {
			logContent.WriteString(fmt.Sprintf("- %s\n", f))
		}
		logContent.WriteString("\n\nNote: This is an automatically generated log for a folder that was missing a detailed log file.\n")
		logContent.WriteString("For folders like BI001, BI001p1, BO001p6, etc., detailed analysis was not recorded during processing.\n")
		os.WriteFile(logPath, []byte(logContent.String()), 0644)
		c.log(fmt.Sprintf("Saved basic log file to: %s", logPath))
	}
}

// --- Helper functions ---

func extractContent(path string) (*dxf.DXFContent, error) {
	drawing, err := dxf.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return drawing.ExtractContent(3), nil
}

// pythonFloat formats a float64 like Python's json.dumps would
// Python always includes a decimal point: 1.0 not 1, 0.0 not 0
func pythonFloat(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return fmt.Sprintf("%.1f", f) // 1.0, 0.0, 42.0
	}
	// For non-integers, use repr-like formatting (Python uses str(float))
	// json.dumps uses float.__repr__ which gives shortest representation
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// pythonJSON serializes DXF content to match Python's json.dumps([blocks, lines, polylines], sort_keys=True)
// This is critical for content hash compatibility — the hash must match exactly
func pythonJSON(blocks map[string][][3]float64, lines map[string][][2][3]float64, polylines map[string][][][3]float64) string {
	var sb strings.Builder
	sb.WriteByte('[')
	sb.WriteString(pythonJSONBlocks(blocks))
	sb.WriteString(", ")
	sb.WriteString(pythonJSONLines(lines))
	sb.WriteString(", ")
	sb.WriteString(pythonJSONPolylines(polylines))
	sb.WriteByte(']')
	return sb.String()
}

func pythonJSONBlocks(m map[string][][3]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(": ")
		sb.WriteByte('[')
		for j, v := range m[k] {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("[")
			sb.WriteString(pythonFloat(v[0]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(v[1]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(v[2]))
			sb.WriteString("]")
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}

func pythonJSONLines(m map[string][][2][3]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(": ")
		sb.WriteByte('[')
		for j, pair := range m[k] {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("[[")
			sb.WriteString(pythonFloat(pair[0][0]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[0][1]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[0][2]))
			sb.WriteString("], [")
			sb.WriteString(pythonFloat(pair[1][0]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[1][1]))
			sb.WriteString(", ")
			sb.WriteString(pythonFloat(pair[1][2]))
			sb.WriteString("]]")
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}

func pythonJSONPolylines(m map[string][][][3]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(": ")
		sb.WriteByte('[')
		for j, poly := range m[k] {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteByte('[')
			for l, v := range poly {
				if l > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString("[")
				sb.WriteString(pythonFloat(v[0]))
				sb.WriteString(", ")
				sb.WriteString(pythonFloat(v[1]))
				sb.WriteString(", ")
				sb.WriteString(pythonFloat(v[2]))
				sb.WriteString("]")
			}
			sb.WriteByte(']')
		}
		sb.WriteByte(']')
	}
	sb.WriteByte('}')
	return sb.String()
}

// ContentHash creates an MD5 hash from DXF content data
// Must match Python: json.dumps([blocks_dict, lines_dict, polylines_dict], sort_keys=True)
// where blocks_dict = {"block_name": [(x,y,z), ...], ...}
func ContentHash(content *dxf.DXFContent) string {
	contentStr := pythonJSON(content.Blocks, content.Lines, content.Polylines)
	return fmt.Sprintf("%x", md5.Sum([]byte(contentStr)))
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

