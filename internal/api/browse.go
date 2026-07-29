package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TreeNode represents a file or folder in the output tree
type TreeNode struct {
	Name       string      `json:"name"`
	Path       string      `json:"path"`
	IsDir      bool        `json:"is_dir"`
	Size       int64       `json:"size"`
	Modified   string      `json:"modified"`
	Children   []*TreeNode `json:"children,omitempty"`
	IsTemplate bool        `json:"is_template"`
	IsMod      bool        `json:"is_mod"`
	FileCount  int         `json:"file_count"`
	DXFCount   int         `json:"dxf_count"`
}

// handleBrowse returns the folder tree structure of the output folder
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	outputFolder := r.URL.Query().Get("path")
	if outputFolder == "" {
		outputFolder = s.settings.OutputFolder
	}
	if outputFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "output folder not set — select a project first")
		return
	}

	if _, err := os.Stat(outputFolder); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusNotFound, "output folder does not exist")
		return
	}

	// Check if folder is empty
	entries, err := os.ReadDir(outputFolder)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "cannot read output folder")
		return
	}
	if len(entries) == 0 {
		JSONResponse(w, http.StatusOK, map[string]any{
			"tree":  nil,
			"empty": true,
		})
		return
	}

	// Build tree (max depth 3 to keep response size manageable)
	tree := buildTreeNode(outputFolder, filepath.Base(outputFolder), 0, 3)

	JSONResponse(w, http.StatusOK, map[string]any{
		"tree":  tree,
		"empty": false,
	})
}

// buildTreeNode recursively builds a tree node
func buildTreeNode(path, name string, currentDepth, maxDepth int) *TreeNode {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	node := &TreeNode{
		Name:     name,
		Path:     path,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Modified: info.ModTime().Format("2006-01-02 15:04:05"),
	}

	if !info.IsDir() {
		// File — check type
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".dxf") {
			node.DXFCount = 1
		}
		return node
	}

	// Directory
	node.IsTemplate = !strings.Contains(name, "_mod") && name != "notemplate"
	node.IsMod = strings.Contains(name, "_mod")

	if currentDepth >= maxDepth {
		// Count files but don't recurse
		fileCount, dxfCount := countFiles(path)
		node.FileCount = fileCount
		node.DXFCount = dxfCount
		return node
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return node
	}

	// Sort: directories first, then files, alphabetically
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	fileCount, dxfCount := 0, 0

	for _, d := range dirs {
		child := buildTreeNode(filepath.Join(path, d.Name()), d.Name(), currentDepth+1, maxDepth)
		if child != nil {
			node.Children = append(node.Children, child)
			fileCount += child.FileCount
			dxfCount += child.DXFCount
		}
	}

	for _, f := range files {
		childPath := filepath.Join(path, f.Name())
		child := &TreeNode{
			Name:     f.Name(),
			Path:     childPath,
			IsDir:    false,
			Size:     fileSize(f),
			Modified: fileModTime(f).Format("2006-01-02 15:04:05"),
		}
		lower := strings.ToLower(f.Name())
		if strings.HasSuffix(lower, ".dxf") {
			child.DXFCount = 1
			dxfCount++
		}
		fileCount++
		node.Children = append(node.Children, child)
	}

	node.FileCount = fileCount
	node.DXFCount = dxfCount

	return node
}

// countFiles counts total files and DXF files in a directory (recursive)
func countFiles(dir string) (int, int) {
	total, dxf := 0, 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total++
		if strings.HasSuffix(strings.ToLower(info.Name()), ".dxf") {
			dxf++
		}
		return nil
	})
	return total, dxf
}

func fileSize(e os.DirEntry) int64 {
	info, err := e.Info()
	if err != nil {
		return 0
	}
	return info.Size()
}

func fileModTime(e os.DirEntry) time.Time {
	info, err := e.Info()
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// handleBrowseFolder returns contents of a specific folder (for lazy loading)
func (s *Server) handleBrowseFolder(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("path")
	if folderPath == "" {
		ErrorResponse(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusNotFound, "folder does not exist")
		return
	}

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "cannot read folder")
		return
	}

	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	children := []*TreeNode{}
	for _, d := range dirs {
		child := buildTreeNode(filepath.Join(folderPath, d.Name()), d.Name(), 0, 1)
		if child != nil {
			children = append(children, child)
		}
	}
	for _, f := range files {
		childPath := filepath.Join(folderPath, f.Name())
		child := &TreeNode{
			Name:     f.Name(),
			Path:     childPath,
			IsDir:    false,
			Size:     fileSize(f),
			Modified: fileModTime(f).Format("2006-01-02 15:04:05"),
		}
		if strings.HasSuffix(strings.ToLower(f.Name()), ".dxf") {
			child.DXFCount = 1
		}
		children = append(children, child)
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"children": children,
	})
}