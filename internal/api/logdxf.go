package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleDXFRender returns all entities from a single DXF file for visual rendering
func (s *Server) handleDXFRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		ErrorResponse(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	if !strings.HasSuffix(strings.ToLower(path), ".dxf") {
		ErrorResponse(w, http.StatusBadRequest, "only .dxf files are allowed")
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusNotFound, "file not found")
		return
	}

	entities, err := extractEntities(path)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to parse DXF: "+err.Error())
		return
	}

	bbox := computeBoundingBox(entities)

	// Count entity types
	typeCounts := make(map[string]int)
	for _, e := range entities {
		typeCounts[e.Type]++
	}

	// Collect layers
	layerSet := make(map[string]bool)
	for _, e := range entities {
		if e.Layer != "" {
			layerSet[e.Layer] = true
		}
	}
	layers := make([]string, 0, len(layerSet))
	for l := range layerSet {
		layers = append(layers, l)
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"entities":      entities,
		"count":         len(entities),
		"bounding_box":  bbox,
		"type_counts":   typeCounts,
		"layers":        layers,
		"layer_count":   len(layers),
		"path":          path,
		"name":          filepath.Base(path),
	})
}

// handleLogContent returns the content of a .log file
func (s *Server) handleLogContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		ErrorResponse(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	// Security: only allow .log files
	if !strings.HasSuffix(strings.ToLower(path), ".log") {
		ErrorResponse(w, http.StatusBadRequest, "only .log files are allowed")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "file not found: "+err.Error())
		return
	}

	content := string(data)
	lines := strings.Count(content, "\n") + 1

	JSONResponse(w, http.StatusOK, map[string]any{
		"content": content,
		"lines":   lines,
		"path":    path,
	})
}

// handleDXFContent returns the raw text content of a DXF file
func (s *Server) handleDXFContent(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		path := r.URL.Query().Get("path")
		if path == "" {
			ErrorResponse(w, http.StatusBadRequest, "path parameter is required")
			return
		}

		if !strings.HasSuffix(strings.ToLower(path), ".dxf") {
			ErrorResponse(w, http.StatusBadRequest, "only .dxf files are allowed")
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			ErrorResponse(w, http.StatusNotFound, "file not found: "+err.Error())
			return
		}

		content := string(data)
		// Limit to first 50000 chars to avoid huge responses
		if len(content) > 50000 {
			content = content[:50000] + "\n\n... (truncated, showing first 50000 characters)"
		}
		lines := strings.Count(content, "\n") + 1

		JSONResponse(w, http.StatusOK, map[string]any{
			"content": content,
			"lines":   lines,
			"path":    path,
			"name":    filepath.Base(path),
		})

	case http.MethodPost:
		// Save modified DXF content
		var req struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if req.Path == "" {
			ErrorResponse(w, http.StatusBadRequest, "path is required")
			return
		}
		if !strings.HasSuffix(strings.ToLower(req.Path), ".dxf") {
			ErrorResponse(w, http.StatusBadRequest, "only .dxf files can be saved")
			return
		}

		// Create backup of original file
		backupPath := req.Path + ".bak"
		origData, err := os.ReadFile(req.Path)
		if err == nil {
			os.WriteFile(backupPath, origData, 0644)
		}

		err = os.WriteFile(req.Path, []byte(req.Content), 0644)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
			return
		}

		JSONResponse(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": fmt.Sprintf("Saved %s (backup at %s.bak)", filepath.Base(req.Path), filepath.Base(req.Path)),
		})

	default:
		ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleEditScript applies an edit script to a template and then applies to group
// The edit script is a series of find/replace operations on the DXF text
type EditScriptRequest struct {
	TemplatePath string `json:"template_path"`
	EditScript   string `json:"edit_script"`    // JSON array of {find, replace} operations
	GroupName    string `json:"group_name"`
	OutputFolder string `json:"output_folder"`
}

type EditOperation struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
}

func (s *Server) handleEditScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req EditScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.TemplatePath == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_path is required")
		return
	}
	if req.EditScript == "" {
		ErrorResponse(w, http.StatusBadRequest, "edit_script is required")
		return
	}

	// Parse edit operations
	var ops []EditOperation
	if err := json.Unmarshal([]byte(req.EditScript), &ops); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid edit script: "+err.Error())
		return
	}

	// Read template file
	data, err := os.ReadFile(req.TemplatePath)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "template file not found: "+err.Error())
		return
	}

	content := string(data)

	// Apply edit operations
	modified := 0
	for _, op := range ops {
		if op.Find == "" {
			continue
		}
		newContent := strings.ReplaceAll(content, op.Find, op.Replace)
		if newContent != content {
			modified++
			content = newContent
		}
	}

	// Save modified template
	backupPath := req.TemplatePath + ".bak"
	os.WriteFile(backupPath, data, 0644)
	err = os.WriteFile(req.TemplatePath, []byte(content), 0644)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to write template: "+err.Error())
		return
	}

	// If group specified, apply to group
	if req.GroupName != "" {
		outputFolder := req.OutputFolder
		if outputFolder == "" {
			outputFolder = s.settings.OutputFolder
		}

		// Apply the modified template to the group
		groups := buildTemplateGroups(outputFolder)
		for _, g := range groups {
			if g.TemplateName == req.GroupName {
				templateDir := filepath.Join(outputFolder, req.GroupName)
				os.MkdirAll(templateDir, 0755)
				fixedTarget := filepath.Join(templateDir, req.GroupName+".dxf")
				copyFileData(req.TemplatePath, fixedTarget)

				if s.settings.TemplateFolder != "" {
					projectTarget := filepath.Join(s.settings.TemplateFolder, req.GroupName+".dxf")
					copyFileData(req.TemplatePath, projectTarget)
				}

				for _, mod := range g.ModFolders {
					fixedInMod := filepath.Join(mod.FolderPath, req.GroupName+"_fixed.dxf")
					copyFileData(req.TemplatePath, fixedInMod)
				}

				JSONResponse(w, http.StatusOK, map[string]any{
					"ok":       true,
					"message":  fmt.Sprintf("Edit script applied: %d replacements, template applied to group %s (%d mod folders)", modified, req.GroupName, len(g.ModFolders)),
					"modified": modified,
				})
				return
			}
		}
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":       true,
		"message":  fmt.Sprintf("Edit script applied: %d replacements to template %s", modified, filepath.Base(req.TemplatePath)),
		"modified": modified,
	})
}

// handleTemplateGroupDetail returns details for a specific template group
func (s *Server) handleTemplateGroupDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	groupName := r.URL.Query().Get("name")
	if groupName == "" {
		ErrorResponse(w, http.StatusBadRequest, "name parameter is required")
		return
	}

	outputFolder := r.URL.Query().Get("output")
	if outputFolder == "" {
		outputFolder = s.settings.OutputFolder
	}

	groups := buildTemplateGroups(outputFolder)
	for _, g := range groups {
		if g.TemplateName == groupName {
			JSONResponse(w, http.StatusOK, map[string]any{
				"group": g,
			})
			return
		}
	}

	JSONResponse(w, http.StatusOK, map[string]any{"group": nil})
}