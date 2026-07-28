package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/falke-ai-circuit/DXFchk/internal/compare"
)

// TemplateMapResult holds template scan results
type TemplateMapResult struct {
	Count   int               `json:"count"`
	Mapping map[string]string `json:"mapping"`
}

// CompareRequest is the body for POST /api/v1/compare
type CompareRequest struct {
	SearchFolder   string `json:"search_folder"`
	Recursive      bool   `json:"recursive"`
	MoveFiles      bool   `json:"move_files"`
	GroupByContent bool   `json:"group_by_content"`
}

// handleHealth returns server health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version":  "v0.1.0",
		"running":  s.compareState.Running,
	})
}

// handleSettings gets or updates settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		JSONResponse(w, http.StatusOK, s.settings)
	case http.MethodPost:
		var settings Settings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		s.settings = &settings
		JSONResponse(w, http.StatusOK, map[string]any{"ok": true, "settings": s.settings})
	default:
		ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleScanTemplates scans the template folder and builds the template map
func (s *Server) handleScanTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		TemplateFolder string `json:"template_folder"`
		Recursive      bool   `json:"recursive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.TemplateFolder == "" {
		req.TemplateFolder = s.settings.TemplateFolder
	}
	if req.TemplateFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_folder is required")
		return
	}

	if _, err := os.Stat(req.TemplateFolder); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "template folder does not exist")
		return
	}

	// Build template map
	templateMap := compare.BuildTemplateMap(req.TemplateFolder, req.Recursive, nil)

	s.settings.TemplateFolder = req.TemplateFolder
	s.compareState.TemplateMap = templateMap

	JSONResponse(w, http.StatusOK, TemplateMapResult{
		Count:   len(templateMap),
		Mapping: templateMap,
	})
}

// handleGetTemplates returns the current template map
func (s *Server) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	if s.compareState.TemplateMap == nil {
		JSONResponse(w, http.StatusOK, TemplateMapResult{Count: 0, Mapping: make(map[string]string)})
		return
	}
	JSONResponse(w, http.StatusOK, TemplateMapResult{
		Count:   len(s.compareState.TemplateMap),
		Mapping: s.compareState.TemplateMap,
	})
}

// handleCompare starts a comparison run
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	if s.compareState.Running {
		ErrorResponse(w, http.StatusConflict, "comparison already running")
		return
	}

	if s.compareState.TemplateMap == nil || len(s.compareState.TemplateMap) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "no template map — scan templates first")
		return
	}

	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	searchFolder := req.SearchFolder
	if searchFolder == "" {
		searchFolder = s.settings.SearchFolder
	}
	if searchFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "search_folder is required")
		return
	}

	if _, err := os.Stat(searchFolder); os.IsNotExist(err) {
		ErrorResponse(w, http.StatusBadRequest, "search folder does not exist")
		return
	}

	outputFolder := s.settings.OutputFolder
	if outputFolder == "" {
		outputFolder = filepath.Join(searchFolder, "DXFchk_output")
	}
	os.MkdirAll(outputFolder, 0755)

	// Start comparison in background
	go s.runComparison(searchFolder, outputFolder, req)

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message":  "comparison started",
		"output":  outputFolder,
	})
}

// runComparison runs the comparison in a background goroutine
func (s *Server) runComparison(searchFolder, outputFolder string, req CompareRequest) {
	mu := sync.Mutex{}

	s.compareState.Running = true
	s.compareState.ProcessedFiles = 0
	s.compareState.LogMessages = []string{}

	logFn := func(msg string) {
		mu.Lock()
		s.compareState.LogMessages = append(s.compareState.LogMessages, msg)
		// Keep last 500 messages
		if len(s.compareState.LogMessages) > 500 {
			s.compareState.LogMessages = s.compareState.LogMessages[len(s.compareState.LogMessages)-500:]
		}
		mu.Unlock()
	}

	progressFn := func(done, total int) bool {
		mu.Lock()
		s.compareState.TotalFiles = total
		s.compareState.ProcessedFiles = done
		mu.Unlock()
		return true
	}

	results := compare.RunComparison(
		s.compareState.TemplateMap,
		searchFolder,
		outputFolder,
		req.Recursive,
		req.MoveFiles,
		req.GroupByContent,
		progressFn,
		logFn,
	)

	s.compareState.Results = make([]any, len(results))
	for i, r := range results {
		s.compareState.Results[i] = r
	}
	s.compareState.Running = false

	// Count results
	matched := 0
	different := 0
	noTemplate := 0
	for _, r := range results {
		switch r.Status {
		case "match":
			matched++
		case "different":
			different++
		case "no_template":
			noTemplate++
		}
	}

	logFn(fmt.Sprintf("=== SUMMARY: %d matched, %d different, %d no template ===", matched, different, noTemplate))
}

// handleCompareStatus returns the current comparison progress
func (s *Server) handleCompareStatus(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]any{
		"running":          s.compareState.Running,
		"total_files":     s.compareState.TotalFiles,
		"processed_files": s.compareState.ProcessedFiles,
		"progress":        progressPercent(s.compareState.ProcessedFiles, s.compareState.TotalFiles),
		"log_count":       len(s.compareState.LogMessages),
		"recent_logs":     getRecentLogs(s.compareState.LogMessages, 20),
		"results_count":   len(s.compareState.Results),
	})
}

// handleResults returns the comparison results
func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]any{
		"results": s.compareState.Results,
		"count":   len(s.compareState.Results),
	})
}

// progressPercent calculates percentage
func progressPercent(done, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(done) / float64(total) * 100
}

// getRecentLogs returns the last N log messages
func getRecentLogs(logs []string, n int) []string {
	if len(logs) <= n {
		return logs
	}
	return logs[len(logs)-n:]
}