package api

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// Create zip in memory or temp file
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

	// Add template folder contents
	addFolderToZip(zipWriter, project.TemplateFolder, "templates/")
	// Add search folder contents
	addFolderToZip(zipWriter, project.SearchFolder, "search/")

	// Add output folder if exists
	if project.OutputFolder != "" {
		if _, err := os.Stat(project.OutputFolder); err == nil {
			addFolderToZip(zipWriter, project.OutputFolder, "output/")
		}
	}

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

	// Determine base path for extraction
	// Extract to a new folder based on project name
	homeDir, _ := os.UserHomeDir()
	extractBase := filepath.Join(homeDir, ".dxfchk", "imported", project.ID)
	os.MkdirAll(extractBase, 0755)

	// Extract all files
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			os.MkdirAll(filepath.Join(extractBase, f.Name), 0755)
			continue
		}

		// Create parent dir
		targetPath := filepath.Join(extractBase, f.Name)
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
	project.TemplateFolder = filepath.Join(extractBase, "templates")
	project.SearchFolder = filepath.Join(extractBase, "search")
	if _, err := os.Stat(filepath.Join(extractBase, "output")); err == nil {
		project.OutputFolder = filepath.Join(extractBase, "output")
	} else {
		project.OutputFolder = filepath.Join(project.SearchFolder, "DXFchk_output")
	}

	// Register project
	store := loadProjects()
	id := project.ID
	if _, exists := store.Projects[id]; exists {
		id = id + "_imported_" + filepath.Base(extractBase)
		project.ID = id
	}
	project.ID = id
	store.Projects[id] = &project
	store.ActiveID = id
	store.save()

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"project": project,
		"message": "Project imported and extracted to " + extractBase,
	})
}

// addFolderToZip adds all files from a folder to the zip under a prefix
func addFolderToZip(zipWriter *zip.Writer, folderPath, prefix string) {
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

		zipPath := prefix + relPath
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