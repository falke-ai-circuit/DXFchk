package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
)

// handleOpenExternal opens a DXF file in the configured external editor.
// If no editor path is set, uses the Windows default association (start command).
//
// POST /api/v1/open
// Body: {"path": "C:\\path\\to\\file.dxf"}
// Response: {"ok": true, "launched": true}
func (s *Server) handleOpenExternal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Path == "" {
		ErrorResponse(w, http.StatusBadRequest, "path is required")
		return
	}

	editorPath := s.settings.ExternalEditorPath
	var cmd *exec.Cmd

	if editorPath != "" {
		// Use configured editor
		cmd = exec.Command(editorPath, req.Path)
	} else {
		// Use Windows default association: start "" "path"
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

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":       true,
		"launched": true,
		"editor":   editorPath,
	})
}