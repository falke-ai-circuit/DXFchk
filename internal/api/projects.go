package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Project represents a DXFchk project configuration
type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ProjectNumber string    `json:"project_number"`
	ProjectPath    string    `json:"project_path"`
	TemplateFolder string   `json:"template_folder"`
	SearchFolder   string   `json:"search_folder"`
	OutputFolder   string   `json:"output_folder"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsed       time.Time `json:"last_used"`
	Recursive      bool      `json:"recursive"`
	GroupByContent bool      `json:"group_by_content"`
	MoveFiles      bool      `json:"move_files"`
}

// ProjectStore manages projects on disk
type ProjectStore struct {
	filePath string
	Projects map[string]*Project `json:"projects"`
	ActiveID string              `json:"active_id"`
}

// loadProjects loads the project store from disk
func loadProjects() *ProjectStore {
	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, ".dxfchk", "projects.json")

	store := &ProjectStore{
		filePath: filePath,
		Projects: make(map[string]*Project),
	}

	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, store)
	}
	if store.Projects == nil {
		store.Projects = make(map[string]*Project)
	}
	return store
}

// save writes the project store to disk
func (ps *ProjectStore) save() error {
	dir := filepath.Dir(ps.filePath)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.filePath, data, 0644)
}

// handleProjects handles CRUD for projects
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	store := loadProjects()

	switch r.Method {
	case http.MethodGet:
		// List all projects
		list := make([]*Project, 0, len(store.Projects))
		for _, p := range store.Projects {
			list = append(list, p)
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].LastUsed.After(list[j].LastUsed)
		})
		JSONResponse(w, http.StatusOK, map[string]any{
			"projects":   list,
			"active_id":  store.ActiveID,
			"count":      len(list),
		})

	case http.MethodPost:
		// Create new project
		var req struct {
			Name            string `json:"name"`
			ProjectNumber   string `json:"project_number"`
			ProjectPath     string `json:"project_path"`
			TemplateFolder  string `json:"template_folder"`
			SearchFolder    string `json:"search_folder"`
			OutputFolder    string `json:"output_folder"`
			Recursive       *bool  `json:"recursive"`
			GroupByContent  *bool  `json:"group_by_content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if req.Name == "" {
			ErrorResponse(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.ProjectNumber == "" {
			ErrorResponse(w, http.StatusBadRequest, "project_number is required")
			return
		}
		if req.ProjectPath == "" {
			ErrorResponse(w, http.StatusBadRequest, "project_path is required")
			return
		}
		if req.TemplateFolder == "" {
			ErrorResponse(w, http.StatusBadRequest, "template_folder is required (source folder with DXF templates)")
			return
		}
		if req.SearchFolder == "" {
			ErrorResponse(w, http.StatusBadRequest, "search_folder is required (source folder with unchecked DXF files)")
			return
		}

		// Verify source folders exist
		if _, err := os.Stat(req.TemplateFolder); os.IsNotExist(err) {
			ErrorResponse(w, http.StatusBadRequest, "template source folder does not exist: "+req.TemplateFolder)
			return
		}
		if _, err := os.Stat(req.SearchFolder); os.IsNotExist(err) {
			ErrorResponse(w, http.StatusBadRequest, "search source folder does not exist: "+req.SearchFolder)
			return
		}

		// Build standardized project folder structure (LOGReport style):
		// {ProjectPath}/{ProjectNumber}_{Name}/
		//   templates/   ← DXF template files copied from source
		//   unchecked/   ← DXF files to check copied from source
		//   output/      ← comparison results (created empty)
		projectFolderName := fmt.Sprintf("%s_%s", req.ProjectNumber, slugify(req.Name))
		projectDir := filepath.Join(req.ProjectPath, projectFolderName)
		templateDir := filepath.Join(projectDir, "templates")
		uncheckedDir := filepath.Join(projectDir, "unchecked")
		outputDir := filepath.Join(projectDir, "output")

		// Create folder structure
		for _, dir := range []string{projectDir, templateDir, uncheckedDir, outputDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to create directory %s: %v", dir, err))
				return
			}
		}

		// Copy DXF files from source template folder to project templates/
		tplCount, err := copyDXFFolder(req.TemplateFolder, templateDir)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to copy template files: %v", err))
			return
		}

		// Copy DXF files from source search folder to project unchecked/
		srchCount, err := copyDXFFolder(req.SearchFolder, uncheckedDir)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to copy search files: %v", err))
			return
		}

		// Generate ID from project number (slug-like)
		id := slugify(req.ProjectNumber)
		// Ensure unique
		if _, exists := store.Projects[id]; exists {
			id = id + "_" + time.Now().Format("150405")
		}

		now := time.Now()
		recursive := true
		groupBy := true
		if req.Recursive != nil {
			recursive = *req.Recursive
		}
		if req.GroupByContent != nil {
			groupBy = *req.GroupByContent
		}

		project := &Project{
			ID:             id,
			Name:           req.Name,
			ProjectNumber:  req.ProjectNumber,
			ProjectPath:     projectDir,
			TemplateFolder: templateDir,
			SearchFolder:    uncheckedDir,
			OutputFolder:    outputDir,
			CreatedAt:       now,
			LastUsed:        now,
			Recursive:       recursive,
			GroupByContent:  groupBy,
			MoveFiles:       false,
		}

		store.Projects[id] = project
		store.ActiveID = id
		s.settings.ActiveProjectID = id
		store.save()

		JSONResponse(w, http.StatusOK, map[string]any{
			"ok":             true,
			"project":        project,
			"project_folder": projectDir,
			"templates_copied": tplCount,
			"files_copied":     srchCount,
		})

	default:
		ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleProject handles single project operations (get, update, delete, activate)
func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	store := loadProjects()

	// Extract project ID from path
	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		ErrorResponse(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	project, exists := store.Projects[projectID]
	if !exists {
		ErrorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Activate and return project
		store.ActiveID = projectID
		project.LastUsed = time.Now()
		store.save()

		// Also update server settings from project
		s.settings.TemplateFolder = project.TemplateFolder
		s.settings.SearchFolder = project.SearchFolder
		s.settings.OutputFolder = project.OutputFolder
		s.settings.Recursive = project.Recursive
		s.settings.GroupByContent = project.GroupByContent
		s.settings.MoveFiles = project.MoveFiles
		s.settings.ActiveProjectID = projectID

		JSONResponse(w, http.StatusOK, map[string]any{
			"project": project,
		})

	case http.MethodPost:
		// Update project
		var req struct {
			Name           *string `json:"name"`
			TemplateFolder *string `json:"template_folder"`
			SearchFolder   *string `json:"search_folder"`
			OutputFolder   *string `json:"output_folder"`
			Recursive      *bool   `json:"recursive"`
			GroupByContent *bool   `json:"group_by_content"`
			MoveFiles      *bool   `json:"move_files"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if req.Name != nil {
			project.Name = *req.Name
		}
		if req.TemplateFolder != nil {
			project.TemplateFolder = *req.TemplateFolder
		}
		if req.SearchFolder != nil {
			project.SearchFolder = *req.SearchFolder
		}
		if req.OutputFolder != nil {
			project.OutputFolder = *req.OutputFolder
		}
		if req.Recursive != nil {
			project.Recursive = *req.Recursive
		}
		if req.GroupByContent != nil {
			project.GroupByContent = *req.GroupByContent
		}
		if req.MoveFiles != nil {
			project.MoveFiles = *req.MoveFiles
		}
		project.LastUsed = time.Now()
		store.save()

		JSONResponse(w, http.StatusOK, map[string]any{
			"ok":      true,
			"project": project,
		})

	case http.MethodDelete:
		delete(store.Projects, projectID)
		if store.ActiveID == projectID {
			store.ActiveID = ""
		}
		store.save()

		JSONResponse(w, http.StatusOK, map[string]any{
			"ok": true,
		})

	default:
		ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// copyDXFFolder copies all DXF files (recursively) from src to dst, preserving subfolder structure.
// Returns the number of files copied.
func copyDXFFolder(src, dst string) (int, error) {
	count := 0
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Only copy .dxf files
		lower := strings.ToLower(info.Name())
		if !strings.HasSuffix(lower, ".dxf") {
			return nil
		}

		// Compute relative path to preserve subfolder structure
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return nil // skip if we can't compute relative path
		}

		targetPath := filepath.Join(dst, relPath)
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer srcFile.Close()

		dstFile, err := os.Create(targetPath)
		if err != nil {
			return nil
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return nil
		}
		count++
		return nil
	})
	return count, err
}

// slugify converts a name to a URL-safe ID
func slugify(name string) string {
	var result []byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		} else if c >= 'A' && c <= 'Z' {
			result = append(result, c+32)
		} else if c == ' ' || c == '_' {
			result = append(result, '-')
		}
	}
	if len(result) == 0 {
		return "project"
	}
	return string(result)
}