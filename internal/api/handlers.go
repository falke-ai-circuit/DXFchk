package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	TemplateFolder string `json:"template_folder"`
	OutputFolder   string `json:"output_folder"`
	Recursive      bool   `json:"recursive"`
	MoveFiles      bool   `json:"move_files"`
	GroupByContent bool   `json:"group_by_content"`
}

// handleHealth returns server health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"version":  "v0.4.0",
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

	outputFolder := req.OutputFolder
	if outputFolder == "" {
		outputFolder = s.settings.OutputFolder
	}
	if outputFolder == "" {
		outputFolder = filepath.Join(searchFolder, "DXFchk_output")
	}
	os.MkdirAll(outputFolder, 0755)

	// If template_folder provided in request, update settings and re-scan
	if req.TemplateFolder != "" {
		s.settings.TemplateFolder = req.TemplateFolder
		s.settings.SearchFolder = searchFolder
		s.settings.OutputFolder = outputFolder
		// Re-scan templates from the provided folder
		templateMap := compare.BuildTemplateMap(req.TemplateFolder, req.Recursive, nil)
		s.compareState.TemplateMap = templateMap
	}

	// Set running state BEFORE starting goroutine to avoid race condition
	s.compareState.Running = true
	s.compareState.ProcessedFiles = 0
	s.compareState.TotalFiles = 0
	s.compareState.LogMessages = []string{}

	// Create stop channel
	s.stopChan = make(chan struct{}, 1)

	// Start comparison in background
	go s.runComparison(searchFolder, outputFolder, req, nil)

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "comparison started",
		"output":  outputFolder,
	})
}

// runComparison runs the comparison in a background goroutine
// skipFiles is a set of filenames to skip (for resume — already processed)
func (s *Server) runComparison(searchFolder, outputFolder string, req CompareRequest, skipFiles map[string]bool) {
	mu := sync.Mutex{}

	startTime := time.Now()
	s.compareState.Running = true
	s.compareState.ProcessedFiles = 0
	s.compareState.LogMessages = []string{}
	s.compareState.StartTime = startTime
	s.compareState.Matched = 0
	s.compareState.Different = 0
	s.compareState.NoTemplate = 0

	// Save session state
	session := &SessionState{
		ProjectID:      "",
		SearchFolder:   searchFolder,
		OutputFolder:   outputFolder,
		TemplateFolder: s.settings.TemplateFolder,
		Recursive:      req.Recursive,
		MoveFiles:      req.MoveFiles,
		GroupByContent: req.GroupByContent,
		StartTime:      startTime,
		Status:         "running",
	}
	s.session = session
	s.SaveSession(session)

	logFn := func(msg string) {
		mu.Lock()
		s.compareState.LogMessages = append(s.compareState.LogMessages, msg)
		// Keep last 500 messages
		if len(s.compareState.LogMessages) > 500 {
			s.compareState.LogMessages = s.compareState.LogMessages[len(s.compareState.LogMessages)-500:]
		}
		mu.Unlock()
	}

	// Progress function with stop check and timing updates
	progressFn := func(done, total int) bool {
		mu.Lock()
		s.compareState.TotalFiles = total
		s.compareState.ProcessedFiles = done
		// Update timing
		elapsed := time.Since(startTime)
		s.compareState.ElapsedTime = formatDuration(elapsed)
		if done > 0 && total > 0 {
			perFile := elapsed / time.Duration(done)
			remaining := total - done
			s.compareState.ETA = formatDuration(perFile * time.Duration(remaining))
		}
		// Update session
		session.TotalFiles = total
		session.ProcessedFiles = done
		session.Matched = s.compareState.Matched
		session.Different = s.compareState.Different
		session.NoTemplate = s.compareState.NoTemplate
		mu.Unlock()

		// Save session periodically (every 10 files)
		if done%10 == 0 {
			s.SaveSession(session)
		}

		// Check stop channel
		select {
		case <-s.stopChan:
			logFn("=== Comparison stopped by user ===")
			return false
		default:
			return true
		}
	}

	// Custom wrapper to count results and track processed files for resume
	wrappedProgressFn := func(done, total int) bool {
		return progressFn(done, total)
	}

	results := compare.RunComparison(
		s.compareState.TemplateMap,
		searchFolder,
		outputFolder,
		req.Recursive,
		req.MoveFiles,
		req.GroupByContent,
		wrappedProgressFn,
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
	s.compareState.Matched = matched
	s.compareState.Different = different
	s.compareState.NoTemplate = noTemplate

	// Final timing
	elapsed := time.Since(startTime)
	s.compareState.ElapsedTime = formatDuration(elapsed)
	s.compareState.ETA = "00:00:00"

	logFn(fmt.Sprintf("=== SUMMARY: %d matched, %d different, %d no template ===", matched, different, noTemplate))
	logFn(fmt.Sprintf("=== Total time: %s ===", formatDuration(elapsed)))

	// Update session as completed
	session.Status = "completed"
	session.EndTime = time.Now()
	session.Matched = matched
	session.Different = different
	session.NoTemplate = noTemplate
	s.SaveSession(session)
}

// handleCompareStop stops the running comparison
func (s *Server) handleCompareStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if !s.compareState.Running {
		ErrorResponse(w, http.StatusBadRequest, "no comparison running")
		return
	}
	select {
	case s.stopChan <- struct{}{}:
	default:
	}
	JSONResponse(w, http.StatusOK, map[string]any{"ok": true, "message": "stop signal sent"})
}

// handleCompareResume resumes a stopped/interrupted comparison
func (s *Server) handleCompareResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.compareState.Running {
		ErrorResponse(w, http.StatusConflict, "comparison already running")
		return
	}

	// Load saved session
	sess, err := LoadSession()
	if err != nil || sess == nil {
		ErrorResponse(w, http.StatusBadRequest, "no saved session to resume")
		return
	}
	if sess.Status == "completed" {
		ErrorResponse(w, http.StatusBadRequest, "last session already completed")
		return
	}

	// Re-scan templates
	if s.settings.TemplateFolder == "" {
		s.settings.TemplateFolder = sess.TemplateFolder
	}
	templateMap := compare.BuildTemplateMap(s.settings.TemplateFolder, sess.Recursive, nil)
	if len(templateMap) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "no templates found — cannot resume")
		return
	}
	s.compareState.TemplateMap = templateMap

	// Build skip list from already-processed files in output
	skipFiles := buildSkipList(sess.OutputFolder)

	s.stopChan = make(chan struct{}, 1)
	go s.runComparison(sess.SearchFolder, sess.OutputFolder, CompareRequest{
		SearchFolder:   sess.SearchFolder,
		Recursive:      sess.Recursive,
		MoveFiles:      sess.MoveFiles,
		GroupByContent: sess.GroupByContent,
	}, skipFiles)

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "comparison resumed",
		"skipped": len(skipFiles),
	})
}

// handleSession gets or clears the session state
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sess, err := LoadSession()
		if err != nil || sess == nil {
			JSONResponse(w, http.StatusOK, map[string]any{"session": nil})
			return
		}
		// Add live timing
		elapsed := sess.ElapsedTime()
		eta := sess.ETA()
		JSONResponse(w, http.StatusOK, map[string]any{
			"session":       sess,
			"elapsed_time":  elapsed,
			"eta":           eta,
		})
	case http.MethodDelete:
		ClearSession()
		s.session = nil
		s.compareState.StartTime = time.Time{}
		s.compareState.ElapsedTime = ""
		s.compareState.ETA = ""
		s.compareState.Matched = 0
		s.compareState.Different = 0
		s.compareState.NoTemplate = 0
		s.compareState.Results = nil
		s.compareState.LogMessages = []string{}
		JSONResponse(w, http.StatusOK, map[string]any{"ok": true, "message": "session cleared"})
	default:
		ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// buildSkipList builds a set of filenames already in the output folder
func buildSkipList(outputFolder string) map[string]bool {
	skip := make(map[string]bool)
	filepath.Walk(outputFolder, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".dxf") {
			skip[info.Name()] = true
		}
		return nil
	})
	return skip
}

// handleCompareStatus returns the current comparison progress
func (s *Server) handleCompareStatus(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]any{
		"running":          s.compareState.Running,
		"total_files":     s.compareState.TotalFiles,
		"processed_files": s.compareState.ProcessedFiles,
		"progress":        progressPercent(s.compareState.ProcessedFiles, s.compareState.TotalFiles),
		"log_count":       len(s.compareState.LogMessages),
		"recent_logs":     getRecentLogs(s.compareState.LogMessages, 50),
		"results_count":   len(s.compareState.Results),
		"elapsed_time":    s.compareState.ElapsedTime,
		"eta":             s.compareState.ETA,
		"matched":         s.compareState.Matched,
		"different":       s.compareState.Different,
		"no_template":     s.compareState.NoTemplate,
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