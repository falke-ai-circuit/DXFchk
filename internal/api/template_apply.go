package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	TemplatePath string `json:"template_path"` // path to the fixed template file
	GroupName    string `json:"group_name"`    // template group name (e.g. "BI001")
	OutputFolder string `json:"output_folder"` // output folder to apply to
}

// handleApplyTemplate applies a fixed template to all files in a group
// This implements the "fix 90 templates instead of 1500 files" workflow:
// 1. User fixes a template DXF file in a _modN folder
// 2. User calls this endpoint to apply that fixed template to all files in that group
// 3. The fixed template replaces the original template
// 4. Optionally re-run comparison for that group only
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

	// Find all mod folders for this group
	groups := buildTemplateGroups(outputFolder)
	var targetGroup *TemplateGroup
	for i := range groups {
		if groups[i].TemplateName == req.GroupName {
			targetGroup = &groups[i]
			break
		}
	}
	if targetGroup == nil {
		ErrorResponse(w, http.StatusNotFound, "group not found: "+req.GroupName)
		return
	}

	// Copy the fixed template to the template folder (overwriting original)
	templateDir := filepath.Join(outputFolder, req.GroupName)
	os.MkdirAll(templateDir, 0755)
	fixedTemplateTarget := filepath.Join(templateDir, req.GroupName+".dxf")
	copyFileData(req.TemplatePath, fixedTemplateTarget)

	// Also copy the fixed template to the project's template folder
	// Use active project's template_folder if global settings don't have one
	templateFolder := s.settings.TemplateFolder
	if templateFolder == "" && s.settings.ActiveProjectID != "" {
		store := loadProjects()
		if proj, exists := store.Projects[s.settings.ActiveProjectID]; exists {
			templateFolder = proj.TemplateFolder
		}
	}
	if templateFolder != "" {
		projectTemplateTarget := filepath.Join(templateFolder, req.GroupName+".dxf")
		copyFileData(req.TemplatePath, projectTemplateTarget)
	}

	appliedCount := 0
	// For each mod folder, copy the fixed template as a reference
	for _, mod := range targetGroup.ModFolders {
		// Place a copy of the fixed template in each mod folder for reference
		fixedInMod := filepath.Join(mod.FolderPath, req.GroupName+"_fixed.dxf")
		copyFileData(req.TemplatePath, fixedInMod)
		appliedCount++
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":             true,
		"message":        fmt.Sprintf("Template applied to group %s (%d mod folders)", req.GroupName, appliedCount),
		"group":          req.GroupName,
		"mod_folders":    appliedCount,
		"total_files":    targetGroup.TotalFiles,
		"template_path":  fixedTemplateTarget,
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