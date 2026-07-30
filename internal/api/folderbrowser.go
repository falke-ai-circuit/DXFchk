package api

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FolderEntry represents a folder or drive for the folder browser
type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "drive", "folder", "parent"
}

// handleBrowseSystem returns system folders/drives for the folder browser dialog
func (s *Server) handleBrowseSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	path := r.URL.Query().Get("path")

	var entries []FolderEntry

	if path == "" {
		// Return root — on Windows return drives, on Linux return root
		entries = listDrives()
	} else {
		// Add parent folder
		parent := filepath.Dir(path)
		if parent != path {
			entries = append(entries, FolderEntry{
				Name: "..",
				Path: parent,
				Type: "parent",
			})
		}

		// List subdirectories
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, "cannot read folder: "+err.Error())
			return
		}

		for _, e := range dirEntries {
			if !e.IsDir() {
				continue
			}
			// Skip hidden folders
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			fullPath := filepath.Join(path, e.Name())
			entries = append(entries, FolderEntry{
				Name: e.Name(),
				Path: fullPath,
				Type: "folder",
			})
		}
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"entries": entries,
		"current": path,
	})
}

// listDrives returns available drives on Windows or root on Linux
func listDrives() []FolderEntry {
	var drives []FolderEntry

	// Try Windows drive letters
	for c := 'C'; c <= 'Z'; c++ {
		drive := string(c) + ":\\"
		if _, err := os.Stat(drive); err == nil {
			drives = append(drives, FolderEntry{
				Name: string(c) + ":",
				Path: drive,
				Type: "drive",
			})
		}
	}

	// If no Windows drives found, return root (Linux)
	if len(drives) == 0 {
		drives = append(drives, FolderEntry{
			Name: "/",
			Path: "/",
			Type: "drive",
		})
		// Also add /opt/data and home dir
		if home, err := os.UserHomeDir(); err == nil {
			drives = append(drives, FolderEntry{
				Name: "~",
				Path: home,
				Type: "drive",
			})
		}
		drives = append(drives, FolderEntry{
			Name: "/opt/data",
			Path: "/opt/data",
			Type: "drive",
		})
	}

	return drives
}

// handleProjectZipExport exports a project's entire folder structure as a zip
func (s *Server) handleProjectZipExport(w http.ResponseWriter, r *http.Request) {
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

	// Determine the project root folder to zip
	// For standardized projects: project.ProjectPath is the full project dir
	// For legacy projects: fall back to using template/search folders
	projectDir := project.ProjectPath
	if projectDir == "" {
		// Legacy mode — zip individual folders
		projectDir = project.OutputFolder
		if projectDir == "" {
			ErrorResponse(w, http.StatusBadRequest, "project has no project_path or output_folder")
			return
		}
	}

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusNotFound, "project folder does not exist: "+projectDir)
		return
	}

	// Create zip in temp
	zipPath := filepath.Join(os.TempDir(), project.ID+"_export.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "cannot create zip: "+err.Error())
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)

	// Add project config JSON
	configData, _ := json.MarshalIndent(project, "", "  ")
	configWriter, err := zipWriter.Create("project.json")
	if err == nil {
		configWriter.Write(configData)
	}

	// For standardized projects, zip the entire project folder (templates/ + unchecked/ + output/)
	// The top-level folder name in the zip is the project folder name (e.g., ECLIPSE-V04-TEST_Eclipse-v04-test/)
	topLevelName := filepath.Base(projectDir)
	addFolderToZipNamed(zipWriter, projectDir, topLevelName+"/")

	zipWriter.Close()

	// Serve the zip file
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+project.ID+".zip")
	http.ServeFile(w, r, zipPath)

	// Clean up
	os.Remove(zipPath)
}

// handleProjectZipImport imports a project from a zip file
func (s *Server) handleProjectZipImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "cannot parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "no file uploaded")
		return
	}
	defer file.Close()

	// Save uploaded zip to temp
	zipPath := filepath.Join(os.TempDir(), header.Filename)
	outFile, err := os.Create(zipPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "cannot save file")
		return
	}
	io.Copy(outFile, file)
	outFile.Close()

	// Open and extract zip
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "cannot open zip: "+err.Error())
		return
	}
	defer zipReader.Close()
	defer os.Remove(zipPath)

	// Find project.json
	var project Project
	foundConfig := false
	for _, f := range zipReader.File {
		if f.Name == "project.json" {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				json.Unmarshal(data, &project)
				rc.Close()
				foundConfig = true
			}
			break
		}
	}

	if !foundConfig {
		ErrorResponse(w, http.StatusBadRequest, "invalid project zip — no project.json found")
		return
	}

	// Find the top-level folder name from the zip (e.g., ECLIPSE-V04-TEST_Eclipse-v04-test/)
	var topLevelFolder string
	for _, f := range zipReader.File {
		parts := strings.SplitN(strings.TrimPrefix(f.Name, "/"), "/", 2)
		if len(parts) > 0 && parts[0] != "" && parts[0] != "project.json" {
			topLevelFolder = parts[0]
			break
		}
	}

	// Determine extraction base: user home / .dxfchk / imported
	homeDir, _ := os.UserHomeDir()
	extractBase := filepath.Join(homeDir, ".dxfchk", "imported")
	os.MkdirAll(extractBase, 0755)

	// Extract all files, preserving the top-level folder structure
	for _, f := range zipReader.File {
		if f.Name == "project.json" {
			continue
		}

		targetPath := filepath.Join(extractBase, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		// Create parent dir
		os.MkdirAll(filepath.Dir(targetPath), 0755)

		rc, err := f.Open()
		if err != nil {
			continue
		}
		outFile, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}

	// Update project paths to point to extracted locations
	// Standardized structure: {extractBase}/{topLevelFolder}/{templates|unchecked|output}/
	if topLevelFolder != "" {
		extractedProjectDir := filepath.Join(extractBase, topLevelFolder)
		project.ProjectPath = extractedProjectDir
		project.TemplateFolder = filepath.Join(extractedProjectDir, "templates")
		project.SearchFolder = filepath.Join(extractedProjectDir, "unchecked")
		project.OutputFolder = filepath.Join(extractedProjectDir, "output")
		// Ensure output dir exists
		os.MkdirAll(project.OutputFolder, 0755)
	} else {
		// Legacy format
		project.TemplateFolder = filepath.Join(extractBase, "templates")
		project.SearchFolder = filepath.Join(extractBase, "search")
		if _, err := os.Stat(filepath.Join(extractBase, "output")); err == nil {
			project.OutputFolder = filepath.Join(extractBase, "output")
		} else {
			project.OutputFolder = filepath.Join(project.SearchFolder, "DXFchk_output")
		}
	}

	// Register project
	store := loadProjects()
	id := project.ID
	if _, exists := store.Projects[id]; exists {
		id = id + "_imported_" + time.Now().Format("150405")
		project.ID = id
	}
	project.ID = id
	store.Projects[id] = &project
	store.ActiveID = id
	store.save()

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"project": project,
		"message": "Project imported and extracted to " + filepath.Join(extractBase, topLevelFolder),
	})
}

// addFolderToZip adds all files from a folder to the zip under a prefix
// addFolderToZip adds all files from a folder to the zip under a prefix
func addFolderToZip(zipWriter *zip.Writer, folderPath, prefix string) {
	addFolderToZipNamed(zipWriter, folderPath, prefix)
}

// addFolderToZipNamed adds all files from a folder to the zip, using the folder's
// own name as the top-level directory in the zip (preserving the folder name).
func addFolderToZipNamed(zipWriter *zip.Writer, folderPath, zipPrefix string) {
	if folderPath == "" {
		return
	}
	if _, err := os.Stat(folderPath); err != nil {
		return
	}

	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return nil
		}

		zipPath := zipPrefix + relPath
		// Use forward slashes in zip
		zipPath = strings.ReplaceAll(zipPath, "\\", "/")
		if info.IsDir() {
			// Create directory entry in zip
			if !strings.HasSuffix(zipPath, "/") {
				zipPath += "/"
			}
			_, err := zipWriter.Create(zipPath)
			if err != nil {
				return nil
			}
			return nil
		}

		// Add file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		writer, err := zipWriter.Create(zipPath)
		if err != nil {
			return nil
		}
		writer.Write(data)
		return nil
	})
}