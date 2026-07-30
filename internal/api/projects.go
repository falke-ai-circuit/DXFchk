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
	"sync"
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
		//   {templates}/   ← DXF template files copied from source
		//   {unchecked}/   ← DXF files to check copied from source
		//   {output}/      ← comparison results (created empty)
		// Folder names come from settings (configurable), defaults: templates, unchecked, output
		folderTemplates, folderUnchecked, folderOutput := s.getFolderNames()

		// Use raw project number and name for folder name (preserve case, underscores)
		projectFolderName := fmt.Sprintf("%s_%s", req.ProjectNumber, req.Name)
		projectDir := filepath.Join(req.ProjectPath, projectFolderName)
		templateDir := filepath.Join(projectDir, folderTemplates)
		uncheckedDir := filepath.Join(projectDir, folderUnchecked)
		outputDir := filepath.Join(projectDir, folderOutput)

		// Create folder structure
		for _, dir := range []string{projectDir, templateDir, uncheckedDir, outputDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to create directory %s: %v", dir, err))
				return
			}
		}

		// Generate ID from project number (slug-like, lowercase for internal ID)
		id := slugify(req.ProjectNumber)
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

		// Start async copy with progress tracking
		copyStatus := s.startProjectCopy(id, req.TemplateFolder, templateDir, req.SearchFolder, uncheckedDir)

		// Register project immediately (copy happens in background)
		store.Projects[id] = project
		store.ActiveID = id
		s.settings.ActiveProjectID = id
		store.save()

		JSONResponse(w, http.StatusOK, map[string]any{
			"ok":             true,
			"project":        project,
			"project_folder": projectDir,
			"copy_status":    copyStatus,
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

// getFolderNames returns the subfolder names from settings, with defaults
func (s *Server) getFolderNames() (templates, unchecked, output string) {
	templates = "templates"
	unchecked = "unchecked"
	output = "output"
	if s.settings != nil {
		if s.settings.FolderTemplates != "" {
			templates = s.settings.FolderTemplates
		}
		if s.settings.FolderUnchecked != "" {
			unchecked = s.settings.FolderUnchecked
		}
		if s.settings.FolderOutput != "" {
			output = s.settings.FolderOutput
		}
	}
	return
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

// CopyStatus tracks progress of async project file copying
type CopyStatus struct {
	ProjectID    string `json:"project_id"`
	Phase        string `json:"phase"`        // "scanning", "copying_templates", "copying_unchecked", "done", "error"
	TotalFiles   int    `json:"total_files"`
	CopiedFiles  int    `json:"copied_files"`
	FailedFiles  int    `json:"failed_files"`
	CurrentFile  string `json:"current_file"`
	Elapsed      string `json:"elapsed"`
	ETA          string `json:"eta"`
	Done         bool   `json:"done"`
	Error        string `json:"error"`
	startTime     time.Time
}

// startProjectCopy starts async parallel copying of DXF files with progress tracking
func (s *Server) startProjectCopy(projectID, tplSrc, tplDst, srchSrc, srchDst string) *CopyStatus {
	cs := &CopyStatus{
		ProjectID: projectID,
		Phase:     "scanning",
		startTime: time.Now(),
	}

	s.copyMu.Lock()
	s.copyStatuses[projectID] = cs
	s.copyMu.Unlock()

	go func() {
		// Scan both source folders for DXF files
		tplFiles := listDXFFiles(tplSrc)
		srchFiles := listDXFFiles(srchSrc)
		totalFiles := len(tplFiles) + len(srchFiles)

		cs.TotalFiles = totalFiles
		cs.Phase = "copying_templates"

		start := time.Now()

		// Shared counter across both phases
		var counter int64

		// Copy templates with parallel pool
		copyResults := make(chan int, len(tplFiles))
		copyPoolShared(tplFiles, tplSrc, tplDst, 8, copyResults, cs, &counter)
		close(copyResults)
		for range copyResults {}

		// Copy unchecked files with parallel pool (counter continues)
		cs.Phase = "copying_unchecked"
		copyResults2 := make(chan int, len(srchFiles))
		copyPoolShared(srchFiles, srchSrc, srchDst, 8, copyResults2, cs, &counter)
		close(copyResults2)
		for range copyResults2 {}

		cs.Phase = "done"
		cs.Done = true
		cs.CurrentFile = ""
		elapsed := time.Since(start)
		cs.Elapsed = formatDuration(elapsed)
		cs.ETA = "00:00:00"
	}()

	return cs
}

// listDXFFiles returns all .dxf file paths under src (recursive)
func listDXFFiles(src string) []string {
	var files []string
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".dxf") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// copyPoolShared copies files in parallel using a worker pool with a shared counter
func copyPoolShared(files []string, srcBase, dstBase string, workers int, results chan<- int, cs *CopyStatus, counter *int64) {
	jobs := make(chan string, len(files))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				relPath, err := filepath.Rel(srcBase, filePath)
				if err != nil {
					results <- 0
					continue
				}
				targetPath := filepath.Join(dstBase, relPath)
				targetDir := filepath.Dir(targetPath)
				os.MkdirAll(targetDir, 0755)

				// Fast copy: read all then write all
				data, err := os.ReadFile(filePath)
				if err != nil {
					mu.Lock()
					cs.FailedFiles++
					mu.Unlock()
					results <- 0
					continue
				}
				if err := os.WriteFile(targetPath, data, 0644); err != nil {
					mu.Lock()
					cs.FailedFiles++
					mu.Unlock()
					results <- 0
					continue
				}

				mu.Lock()
				*counter++
				cs.CopiedFiles = int(*counter)
				cs.CurrentFile = filepath.Base(filePath)
				elapsed := time.Since(cs.startTime)
				cs.Elapsed = formatDuration(elapsed)
				if cs.CopiedFiles > 0 && cs.TotalFiles > 0 {
					perFile := elapsed / time.Duration(cs.CopiedFiles)
					remaining := cs.TotalFiles - cs.CopiedFiles
					cs.ETA = formatDuration(perFile * time.Duration(remaining))
				}
				mu.Unlock()
				results <- 1
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
}

// handleCopyStatus returns the copy progress for a project
func (s *Server) handleCopyStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		JSONResponse(w, http.StatusOK, map[string]any{"done": true, "phase": "no_project"})
		return
	}

	s.copyMu.Lock()
	cs, exists := s.copyStatuses[projectID]
	s.copyMu.Unlock()

	if !exists {
		JSONResponse(w, http.StatusOK, map[string]any{"done": true, "phase": "no_copy"})
		return
	}

	JSONResponse(w, http.StatusOK, cs)
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