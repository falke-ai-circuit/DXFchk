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

	var cmd *exec.Cmd

	if editorPath != "" {
		// Use configured editor — pass file path as argument
		cmd = exec.Command(editorPath, req.Path)
	} else {
		// Use Windows ShellExecute via rundll32 — works from SYSTEM context
		// This is more reliable than `start` when running as a service/SYSTEM
		cmd = exec.Command("rundll32.exe", "shell32.dll,ShellExec_RunDLL", req.Path)
	}

	// Detach — don't wait for the editor to close
	if err := cmd.Start(); err != nil {
		// Fallback: try explorer.exe to open the file
		cmd2 := exec.Command("explorer.exe", req.Path)
		if err2 := cmd2.Start(); err2 != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to launch editor: "+err.Error()+"; fallback also failed: "+err2.Error())
			return
		}
		if cmd2.Process != nil {
			cmd2.Process.Release()
		}
		editorPath = "explorer"
	} else {
		if cmd.Process != nil {
			cmd.Process.Release()
		}
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