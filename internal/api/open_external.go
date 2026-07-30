package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleOpenExternal opens a DXF file in the configured external editor.
// If no editor path is set, uses the Windows default association.
//
// POST /api/v1/open
// Body: {"path": "C:\\path\\to\\file.dxf"}
// Response: {"ok": true, "launched": true, "editor": "..."}
func (s *Server) handleOpenExternal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path       string `json:"path"`
		EditorPath string `json:"editor_path"` // optional override
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Path == "" {
		ErrorResponse(w, http.StatusBadRequest, "path is required")
		return
	}

	// Normalize path — convert forward slashes to backslashes
	req.Path = strings.ReplaceAll(req.Path, "/", "\\")

	editorPath := req.EditorPath
	if editorPath == "" {
		editorPath = s.settings.ExternalEditorPath
	}

	// Use cmd /c start with proper quoting to launch detached
	// "start" launches the app in a new process and returns immediately
	var cmd *exec.Cmd

	if editorPath != "" {
		// Use configured editor with file as argument
		// cmd /c start "" "editor.exe" "file.dxf"
		cmd = exec.Command("cmd", "/c", "start", "", editorPath, req.Path)
	} else {
		// Use Windows default file association
		// cmd /c start "" "file.dxf"
		cmd = exec.Command("cmd", "/c", "start", "", req.Path)
	}

	// Detach — don't wait for the editor to close
	if err := cmd.Start(); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to launch editor: "+err.Error())
		return
	}

	// Release the process so it doesn't become a zombie
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	// Use the filename as the editor display name if no editor was configured
	displayName := editorPath
	if displayName == "" {
		displayName = filepath.Base(req.Path)
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":       true,
		"launched": true,
		"editor":   displayName,
	})
}