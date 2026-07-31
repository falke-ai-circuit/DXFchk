package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/falke-ai-circuit/DXFchk/internal/dxf"
)

// handleProjectExport exports a project configuration as a downloadable JSON file
func (s *Server) handleProjectExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		ErrorResponse(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	store := loadProjects()
	project, exists := store.Projects[projectID]
	if !exists {
		ErrorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	// Return project as downloadable JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", project.ID))
	json.NewEncoder(w).Encode(project)
}

// handleProjectImport imports a project from uploaded JSON
func (s *Server) handleProjectImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var project Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if project.Name == "" {
		ErrorResponse(w, http.StatusBadRequest, "project name is required")
		return
	}
	if project.TemplateFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_folder is required")
		return
	}
	if project.SearchFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "search_folder is required")
		return
	}

	// Generate new ID
	id := slugify(project.Name)
	store := loadProjects()
	if _, exists := store.Projects[id]; exists {
		id = id + "_" + fmt.Sprintf("%d", len(store.Projects))
	}

	project.ID = id
	if project.OutputFolder == "" {
		project.OutputFolder = filepath.Join(project.SearchFolder, "DXFchk_output")
	}

	// Import: optionally copy DXF files into a project structure
	// Like LOGReport: project creates a folder structure
	// We create the project entry and optionally set up folder structure
	importMode := r.URL.Query().Get("mode")
	if importMode == "copy" {
		// Create project directory structure
		projectDir := filepath.Join(filepath.Dir(project.SearchFolder), "dxfchk-projects", project.ID)
		os.MkdirAll(filepath.Join(projectDir, "templates"), 0755)
		os.MkdirAll(filepath.Join(projectDir, "search"), 0755)
		os.MkdirAll(filepath.Join(projectDir, "output"), 0755)
		// Note: actual file copy would be done via a separate API or client-side
	}

	store.Projects[id] = &project
	store.ActiveID = id
	store.save()

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"project": project,
	})
}

// TemplateGroup represents a group of files sharing the same template
type TemplateGroup struct {
	TemplateName string   `json:"template_name"`
	TemplatePath string   `json:"template_path"`
	ModFolders   []ModGroup `json:"mod_folders"`
	MatchedCount int      `json:"matched_count"`
	TotalFiles   int      `json:"total_files"`
}

// ModGroup represents a _modN folder with its files
type ModGroup struct {
	FolderName string   `json:"folder_name"`
	FolderPath string   `json:"folder_path"`
	FileCount  int      `json:"file_count"`
	Files      []string `json:"files"`
}

// handleTemplateGroups returns all template groups with their mod folders
// This is for the "fix 90 templates instead of 1500 files" workflow
func (s *Server) handleTemplateGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	outputFolder := r.URL.Query().Get("output")
	if outputFolder == "" {
		outputFolder = s.settings.OutputFolder
	}
	if outputFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "output folder is required (set output query param or activate a project)")
		return
	}

	groups := buildTemplateGroups(outputFolder)
	JSONResponse(w, http.StatusOK, map[string]any{
		"groups": groups,
		"count":  len(groups),
	})
}

// buildTemplateGroups scans the output folder and groups files by template
// Supports nested _modN structure: Output/BI001/BI001_mod1/
func buildTemplateGroups(outputFolder string) []TemplateGroup {
	// Map: templateName -> TemplateGroup
	groupMap := make(map[string]*TemplateGroup)

	// Walk the output folder recursively
	filepath.Walk(outputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == outputFolder {
			return nil
		}

		name := info.Name()
		if name == "notemplate" {
			return nil
		}

		// Check if it's a mod folder (contains _mod)
		if strings.Contains(name, "_mod") {
			// Extract base template name
			parts := strings.Split(name, "_mod")
			if len(parts) < 2 {
				return nil
			}
			baseName := parts[0]

			// Count DXF files in this mod folder
			subEntries, _ := os.ReadDir(path)
			var files []string
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(strings.ToLower(se.Name()), ".dxf") {
					files = append(files, se.Name())
				}
			}

			if groupMap[baseName] == nil {
				groupMap[baseName] = &TemplateGroup{
					TemplateName: baseName,
					ModFolders:   []ModGroup{},
				}
			}
			groupMap[baseName].ModFolders = append(groupMap[baseName].ModFolders, ModGroup{
				FolderName: name,
				FolderPath: path,
				FileCount:  len(files),
				Files:      files,
			})
			groupMap[baseName].TotalFiles += len(files)
		} else {
			// This is a template folder (matched files)
			subEntries, _ := os.ReadDir(path)
			count := 0
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(strings.ToLower(se.Name()), ".dxf") {
					count++
				}
			}

			if groupMap[name] == nil {
				groupMap[name] = &TemplateGroup{
					TemplateName: name,
					ModFolders:   []ModGroup{},
				}
			}
			groupMap[name].MatchedCount = count
			groupMap[name].TotalFiles += count
			groupMap[name].TemplatePath = filepath.Join(path, name+".dxf")
		}
		return nil
	})

	// Convert to sorted slice
	groups := make([]TemplateGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].TemplateName < groups[j].TemplateName
	})

	return groups
}

// ApplyTemplateRequest is the body for POST /api/v1/template/apply
type ApplyTemplateRequest struct {
	TemplatePath string `json:"template_path"` // path to the fixed template DXF file
	GroupName    string `json:"group_name"`    // template group name (e.g. "VLV01_mod1")
	OutputFolder string `json:"output_folder"` // output folder (optional, uses project default)
	DryRun       bool   `json:"dry_run"`       // if true, don't write files, just report what would change
}

// ApplyFileResult holds the result of applying template to one module file
type ApplyFileResult struct {
	File          string `json:"file"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	TemplateInserts int  `json:"template_inserts"`
	ModuleInserts   int  `json:"module_inserts"`
	Matched         int  `json:"matched"`
	Added           int  `json:"added_from_template"`
	Removed         int  `json:"removed_from_module"`
}

// handleApplyTemplate applies a fixed template to all files in a group.
//
// Workflow:
// 1. User creates template from _modN file (POST /api/v1/template/create)
// 2. User fixes the template in DNA Explorer (moves blocks, adds/removes I/O, etc.)
// 3. User calls this endpoint to apply the fixed template to all files in the group
// 4. For each module file: template block STRUCTURE replaces module's block structure,
//    but module's ATTRIB VALUES (device tags, I/O names, area refs) are preserved
// 5. $(TEMPLATE) attribute updated to new template name
func (s *Server) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req ApplyTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.TemplatePath == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_path is required")
		return
	}
	if req.GroupName == "" {
		ErrorResponse(w, http.StatusBadRequest, "group_name is required")
		return
	}

	outputFolder := req.OutputFolder
	if outputFolder == "" {
		outputFolder = s.settings.OutputFolder
	}
	if outputFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "output_folder is required")
		return
	}

	// Verify the fixed template file exists
	if _, err := os.Stat(req.TemplatePath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "template file does not exist: "+req.TemplatePath)
		return
	}

	// Find the _modN folder(s) for this group
	// GroupName can be either:
	//   - A specific _modN folder name (e.g. "VLV01_mod1") → apply to that folder only
	//   - A base template name (e.g. "VLV01") → apply to ALL _modN folders for that template
	groups := buildTemplateGroups(outputFolder)
	var targetGroup *TemplateGroup
	for i := range groups {
		if groups[i].TemplateName == req.GroupName {
			targetGroup = &groups[i]
			break
		}
		// Also check if GroupName matches a specific _modN folder within this group
		for _, mod := range groups[i].ModFolders {
			if mod.FolderName == req.GroupName {
				// Create a group with just this one mod folder
				targetGroup = &TemplateGroup{
					TemplateName: groups[i].TemplateName,
					ModFolders:   []ModGroup{mod},
					TotalFiles:   mod.FileCount,
				}
				break
			}
		}
		if targetGroup != nil {
			break
		}
	}
	if targetGroup == nil {
		ErrorResponse(w, http.StatusNotFound, "group not found: "+req.GroupName)
		return
	}

	// Determine the new template name
	// If GroupName contains _mod, use it directly; otherwise use the base template name
	newTemplateName := req.GroupName

	// Apply the template to each module file in each mod folder
	var fileResults []ApplyFileResult
	totalApplied := 0
	totalErrors := 0

	for _, mod := range targetGroup.ModFolders {
		for _, fileName := range mod.Files {
			modPath := filepath.Join(mod.FolderPath, fileName)
			var outputPath string

			if req.DryRun {
				// Don't write, just report
				outputPath = filepath.Join(os.TempDir(), "dxfchk_dryrun.dxf")
			} else {
				// Write back to the same file (in-place update)
				outputPath = modPath
			}

			result, err := dxf.ApplyTemplateToModule(req.TemplatePath, modPath, outputPath, newTemplateName)
			fr := ApplyFileResult{
				File:            fileName,
				Success:         err == nil && result.Success,
				TemplateInserts: result.TemplateInserts,
				ModuleInserts:   result.ModuleInserts,
				Matched:         result.Matched,
				Added:           result.AddedFromTemplate,
				Removed:         result.RemovedFromModule,
			}
			if err != nil {
				fr.Error = err.Error()
				totalErrors++
			} else {
				totalApplied++
			}

			// Clean up dry run temp file
			if req.DryRun {
				os.Remove(outputPath)
			}

			fileResults = append(fileResults, fr)
		}
	}

	// Also update the template file in the template folder
	templateFolder := s.settings.TemplateFolder
	if templateFolder == "" && s.settings.ActiveProjectID != "" {
		store := loadProjects()
		if proj, exists := store.Projects[s.settings.ActiveProjectID]; exists {
			templateFolder = proj.TemplateFolder
		}
	}
	if templateFolder != "" && !req.DryRun {
		projectTemplateTarget := filepath.Join(templateFolder, req.GroupName+".dxf")
		copyFileData(req.TemplatePath, projectTemplateTarget)
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":           totalErrors == 0,
		"message":      fmt.Sprintf("Template applied to group %s: %d files updated, %d errors", req.GroupName, totalApplied, totalErrors),
		"group":        req.GroupName,
		"template":     req.TemplatePath,
		"files_updated": totalApplied,
		"errors":       totalErrors,
		"total_files":  targetGroup.TotalFiles,
		"file_results": fileResults,
		"dry_run":      req.DryRun,
	})
}

// copyFileData copies a file (simple version without logging)
func copyFileData(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// CreateTemplateRequest is the body for POST /api/v1/template/create
type CreateTemplateRequest struct {
	SourceFile      string `json:"source_file"`      // DXF file to use as basis for new template
	TemplateFolder  string `json:"template_folder"`   // Where to save the new template (optional, defaults to project template folder)
	TemplateName    string `json:"template_name"`     // New template name (e.g. "VLV01_mod1") — becomes the $(TEMPLATE) attr value
	SaveToModFolder bool   `json:"save_to_mod"`      // Also save in the mod folder for DNA Explorer editing
}

// handleCreateTemplate creates a new template from a _modN DXF file
// by changing the $(TEMPLATE) attribute value to the new template name.
//
// Workflow:
// 1. User has a _modN folder (e.g. VLV01_mod1) with files that differ from original template
// 2. User picks one file from that folder as the basis for a new template
// 3. This endpoint changes $(TEMPLATE) attr from "VLV01" to "VLV01_mod1"
// 4. Saves the new template to:
//    - The project's template folder (so it becomes a recognized template)
//    - The _modN folder (for user to fix in DNA Explorer)
// 5. User fixes the template in DNA Explorer, then applies it to all files in the group
func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.SourceFile == "" {
		ErrorResponse(w, http.StatusBadRequest, "source_file is required")
		return
	}
	if req.TemplateName == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_name is required")
		return
	}

	// Verify source file exists
	if _, err := os.Stat(req.SourceFile); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "source file does not exist: "+req.SourceFile)
		return
	}

	// Determine template folder
	templateFolder := req.TemplateFolder
	if templateFolder == "" {
		templateFolder = s.settings.TemplateFolder
		if templateFolder == "" && s.settings.ActiveProjectID != "" {
			store := loadProjects()
			if proj, exists := store.Projects[s.settings.ActiveProjectID]; exists {
				templateFolder = proj.TemplateFolder
			}
		}
	}
	if templateFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_folder is required (set template_folder or activate a project)")
		return
	}

	// Create the template by modifying $(TEMPLATE) attribute
	// Save to template folder
	templatePath := filepath.Join(templateFolder, req.TemplateName+".dxf")

	oldTemplate, err := dxf.CreateTemplateFromFile(req.SourceFile, templatePath, req.TemplateName)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create template: "+err.Error())
		return
	}

	// Also save in the mod folder for DNA Explorer editing
	modFolderPath := ""
	if req.SaveToModFolder {
		modFolderPath = filepath.Join(filepath.Dir(req.SourceFile), req.TemplateName+".dxf")
		copyFileData(templatePath, modFolderPath)
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":            true,
		"message":       fmt.Sprintf("Template '%s' created from '%s' (changed $(TEMPLATE) from '%s' to '%s')", req.TemplateName, filepath.Base(req.SourceFile), oldTemplate, req.TemplateName),
		"old_template":  oldTemplate,
		"new_template":  req.TemplateName,
		"source_file":   req.SourceFile,
		"template_path": templatePath,
		"mod_path":      modFolderPath,
	})
}

// PreviewTemplateRequest is the body for POST /api/v1/template/preview
type PreviewTemplateRequest struct {
	TemplatePath string `json:"template_path"` // path to the fixed template DXF
	ModulePath   string `json:"module_path"`   // path to a module DXF to compare against
}

// handlePreviewTemplate compares a template to a module and returns what would change.
// This is a read-only preview — no files are modified.
// It categorizes changes as structural (blocks added/removed), positional (blocks moved),
// layer changes, and attribute changes (expected design member variable differences).
func (s *Server) handlePreviewTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req PreviewTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.TemplatePath == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_path is required")
		return
	}
	if req.ModulePath == "" {
		ErrorResponse(w, http.StatusBadRequest, "module_path is required")
		return
	}

	// Verify both files exist
	if _, err := os.Stat(req.TemplatePath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "template file does not exist: "+req.TemplatePath)
		return
	}
	if _, err := os.Stat(req.ModulePath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "module file does not exist: "+req.ModulePath)
		return
	}

	result, err := dxf.PreviewTemplate(req.TemplatePath, req.ModulePath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "preview failed: "+err.Error())
		return
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":     true,
		"result": result,
	})
}

// CreateGroupTemplateRequest is the body for POST /api/v1/template/create-group
type CreateGroupTemplateRequest struct {
	GroupPath      string `json:"group_path"`      // Path to the group folder containing multiple DXF modules
	TemplateFolder string `json:"template_folder"`  // Where to save the new template (optional, defaults to project template folder)
	TemplateName   string `json:"template_name"`    // New template name (e.g. "MF001Hp1") — becomes the $(TEMPLATE) attr value
}

// handleCreateTemplateFromGroup creates a template by analyzing ALL modules in a group.
// This uses cross-module placeholder inference: values that differ across modules
// are replaced with their ATTRIB tag name as a placeholder.
func (s *Server) handleCreateTemplateFromGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req CreateGroupTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.GroupPath == "" {
		ErrorResponse(w, http.StatusBadRequest, "group_path is required")
		return
	}
	if req.TemplateName == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_name is required")
		return
	}

	// Verify group folder exists
	if _, err := os.Stat(req.GroupPath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "group folder does not exist: "+req.GroupPath)
		return
	}

	// Determine template folder
	templateFolder := req.TemplateFolder
	if templateFolder == "" {
		templateFolder = s.settings.TemplateFolder
		if templateFolder == "" && s.settings.ActiveProjectID != "" {
			store := loadProjects()
			if proj, exists := store.Projects[s.settings.ActiveProjectID]; exists {
				templateFolder = proj.TemplateFolder
			}
		}
	}
	if templateFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_folder is required (set template_folder or activate a project)")
		return
	}

	// Create the template using cross-module placeholder inference
	templatePath := filepath.Join(templateFolder, req.TemplateName+".dxf")
	result, err := dxf.CreateTemplateFromGroup(req.GroupPath, templatePath, req.TemplateName)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create template from group: "+err.Error())
		return
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":              true,
		"message":         fmt.Sprintf("Template '%s' created from %d modules in group, %d placeholders inferred", req.TemplateName, result.ModulesAnalyzed, result.PlaceholdersInferred),
		"template_name":   req.TemplateName,
		"template_path":   templatePath,
		"modules_analyzed": result.ModulesAnalyzed,
		"placeholders":    result.PlaceholdersInferred,
		"line_count_mismatch": result.LineCountMismatch,
		"used_fallback":   result.UsedFallback,
		"fallback_reason": result.FallbackReason,
	})
}