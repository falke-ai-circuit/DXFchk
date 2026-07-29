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
	ProjectID      string `json:"project_id"`
	ProjectName    string `json:"project_name"`
	SearchFolder   string `json:"search_folder"`
	TemplateFolder string `json:"template_folder"`
	OutputFolder   string `json:"output_folder"`
	Recursive      bool   `json:"recursive"`
	MoveFiles      bool   `json:"move_files"`
	GroupByContent bool   `json:"group_by_content"`
}

// handleHealth returns server health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	running := s.jobs.RunningJobs()
	JSONResponse(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"version":        "v0.6.0",
		"running":        len(running) > 0,
		"running_jobs":   len(running),
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

	templateMap := compare.BuildTemplateMap(req.TemplateFolder, req.Recursive, nil)
	s.settings.TemplateFolder = req.TemplateFolder

	JSONResponse(w, http.StatusOK, TemplateMapResult{
		Count:   len(templateMap),
		Mapping: templateMap,
	})
}

// handleGetTemplates returns the current template map (from settings-level scan)
func (s *Server) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	// Return count from settings — actual template map is per-job now
	JSONResponse(w, http.StatusOK, TemplateMapResult{
		Count:   0,
		Mapping: make(map[string]string),
	})
}

// handleCompare starts a comparison job (supports parallel jobs by project ID)
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
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

	templateFolder := req.TemplateFolder
	if templateFolder == "" {
		templateFolder = s.settings.TemplateFolder
	}
	if templateFolder == "" {
		ErrorResponse(w, http.StatusBadRequest, "template_folder is required")
		return
	}

	// Determine job ID — use project_id if provided, else generate from folders
	jobID := req.ProjectID
	if jobID == "" {
		jobID = fmt.Sprintf("job_%d", time.Now().Unix())
	}

	// Check if this job is already running
	existing := s.jobs.GetExisting(jobID)
	if existing != nil && existing.Running {
		ErrorResponse(w, http.StatusConflict, "comparison already running for this project")
		return
	}

	// Build template map
	templateMap := compare.BuildTemplateMap(templateFolder, req.Recursive, nil)
	if len(templateMap) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "no templates found in template folder")
		return
	}

	// Create/reset the job
	job := s.jobs.Get(jobID)
	job.mu.Lock()
	job.Running = true
	job.ProcessedFiles = 0
	job.TotalFiles = 0
	job.LogMessages = []string{}
	job.StartTime = time.Now()
	job.Matched = 0
	job.Different = 0
	job.NoTemplate = 0
	job.TemplateMap = templateMap
	job.SearchFolder = searchFolder
	job.OutputFolder = outputFolder
	job.TemplateFolder = templateFolder
	job.ProjectName = req.ProjectName
	job.StopChan = make(chan struct{}, 1)
	job.mu.Unlock()

	// Start comparison in background
	go s.runComparisonJob(job, req)

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":       true,
		"message":  "comparison started",
		"output":   outputFolder,
		"job_id":   jobID,
	})
}

// runComparisonJob runs a single comparison job in a background goroutine
func (s *Server) runComparisonJob(job *CompareJob, req CompareRequest) {
	startTime := time.Now()

	job.mu.Lock()
	job.StartTime = startTime
	job.LogMessages = []string{}
	job.mu.Unlock()

	logFn := func(msg string) {
		job.AddLog(msg)
	}

	progressFn := func(done, total int) bool {
		job.mu.Lock()
		job.TotalFiles = total
		job.ProcessedFiles = done
		elapsed := time.Since(startTime)
		job.ElapsedTime = formatDuration(elapsed)
		if done > 0 && total > 0 {
			perFile := elapsed / time.Duration(done)
			remaining := total - done
			job.ETA = formatDuration(perFile * time.Duration(remaining))
		}
		job.mu.Unlock()

		// Save session periodically
		if done%10 == 0 {
			SaveJobSession(job)
		}

		// Check stop channel
		select {
		case <-job.StopChan:
			logFn("=== Comparison stopped by user ===")
			return false
		default:
			return true
		}
	}

	results := compare.RunComparison(
		job.TemplateMap,
		job.SearchFolder,
		job.OutputFolder,
		req.Recursive,
		req.MoveFiles,
		req.GroupByContent,
		progressFn,
		logFn,
	)

	job.mu.Lock()
	job.Results = make([]any, len(results))
	for i, r := range results {
		job.Results[i] = r
	}
	job.Running = false

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
	job.Matched = matched
	job.Different = different
	job.NoTemplate = noTemplate

	elapsed := time.Since(startTime)
	job.ElapsedTime = formatDuration(elapsed)
	job.ETA = "00:00:00"
	job.mu.Unlock()

	logFn(fmt.Sprintf("=== SUMMARY: %d matched, %d different, %d no template ===", matched, different, noTemplate))
	logFn(fmt.Sprintf("=== Total time: %s ===", formatDuration(elapsed)))

	SaveJobSession(job)
}

// handleCompareStop stops a comparison job (by project_id or the first running job)
func (s *Server) handleCompareStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		ProjectID string `json:"project_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	jobID := req.ProjectID
	if jobID == "" {
		// Stop first running job
		running := s.jobs.RunningJobs()
		if len(running) == 0 {
			ErrorResponse(w, http.StatusBadRequest, "no comparison running")
			return
		}
		jobID = running[0].ID
	}

	job := s.jobs.GetExisting(jobID)
	if job == nil || !job.Running {
		ErrorResponse(w, http.StatusBadRequest, "no comparison running for this project")
		return
	}

	select {
	case job.StopChan <- struct{}{}:
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

	var req struct {
		ProjectID string `json:"project_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	jobID := req.ProjectID
	if jobID == "" {
		// Fallback: find any stopped job with a session
		ErrorResponse(w, http.StatusBadRequest, "project_id is required for resume")
		return
	}

	job := s.jobs.GetExisting(jobID)
	if job != nil && job.Running {
		ErrorResponse(w, http.StatusConflict, "comparison already running for this project")
		return
	}

	// Load saved session for this job
	sessPath := jobSessionPath(jobID)
	sessData, err := os.ReadFile(sessPath)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "no saved session for this project")
		return
	}
	var sess SessionState
	if err := json.Unmarshal(sessData, &sess); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "failed to load session")
		return
	}
	if sess.Status == "completed" {
		ErrorResponse(w, http.StatusBadRequest, "last session already completed")
		return
	}

	// Re-scan templates
	templateMap := compare.BuildTemplateMap(sess.TemplateFolder, sess.Recursive, nil)
	if len(templateMap) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "no templates found — cannot resume")
		return
	}

	// Build skip list
	skipFiles := buildSkipList(sess.OutputFolder)

	job = s.jobs.Get(jobID)
	job.mu.Lock()
	job.Running = true
	job.TemplateMap = templateMap
	job.SearchFolder = sess.SearchFolder
	job.OutputFolder = sess.OutputFolder
	job.TemplateFolder = sess.TemplateFolder
	job.StopChan = make(chan struct{}, 1)
	job.mu.Unlock()

	go s.runComparisonJob(job, CompareRequest{
		SearchFolder:   sess.SearchFolder,
		Recursive:      sess.Recursive,
		MoveFiles:      sess.MoveFiles,
		GroupByContent: sess.GroupByContent,
	})

	JSONResponse(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "comparison resumed",
		"skipped": len(skipFiles),
		"job_id":  jobID,
	})
}

// handleAllJobs returns all running comparison jobs (for global status bar)
func (s *Server) handleAllJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.jobs.All()
	type JobSummary struct {
		ID             string `json:"id"`
		ProjectName    string `json:"project_name"`
		Running        bool   `json:"running"`
		TotalFiles     int    `json:"total_files"`
		ProcessedFiles int    `json:"processed_files"`
		Progress       float64 `json:"progress"`
		ElapsedTime    string `json:"elapsed_time"`
		ETA            string `json:"eta"`
		Matched        int    `json:"matched"`
		Different      int    `json:"different"`
		NoTemplate     int    `json:"no_template"`
	}

	summaries := make([]JobSummary, 0, len(jobs))
	for _, j := range jobs {
		j.mu.Lock()
		progress := 0.0
		if j.TotalFiles > 0 {
			progress = float64(j.ProcessedFiles) / float64(j.TotalFiles) * 100
		}
		summaries = append(summaries, JobSummary{
			ID:             j.ID,
			ProjectName:    j.ProjectName,
			Running:        j.Running,
			TotalFiles:     j.TotalFiles,
			ProcessedFiles: j.ProcessedFiles,
			Progress:       progress,
			ElapsedTime:    j.ElapsedTime,
			ETA:            j.ETA,
			Matched:        j.Matched,
			Different:      j.Different,
			NoTemplate:     j.NoTemplate,
		})
		j.mu.Unlock()
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"jobs":     summaries,
		"count":    len(summaries),
		"running":  len(s.jobs.RunningJobs()),
	})
}

// handleCompareStatus returns status for a specific job (by project_id) or the first running job
func (s *Server) handleCompareStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")

	var job *CompareJob
	if projectID != "" {
		job = s.jobs.GetExisting(projectID)
	}
	if job == nil {
		// Fallback: return first running job, or first job overall
		running := s.jobs.RunningJobs()
		if len(running) > 0 {
			job = running[0]
		} else {
			all := s.jobs.All()
			if len(all) > 0 {
				job = all[0]
			}
		}
	}

	if job == nil {
		JSONResponse(w, http.StatusOK, map[string]any{
			"running":          false,
			"total_files":      0,
			"processed_files":  0,
			"progress":          0,
			"log_count":         0,
			"recent_logs":       []string{},
			"results_count":     0,
			"elapsed_time":      "",
			"eta":               "",
			"matched":           0,
			"different":         0,
			"no_template":       0,
		})
		return
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	progress := 0.0
	if job.TotalFiles > 0 {
		progress = float64(job.ProcessedFiles) / float64(job.TotalFiles) * 100
	}

	JSONResponse(w, http.StatusOK, map[string]any{
		"running":          job.Running,
		"total_files":      job.TotalFiles,
		"processed_files":  job.ProcessedFiles,
		"progress":          progress,
		"log_count":         len(job.LogMessages),
		"recent_logs":       getRecentLogs(job.LogMessages, 50),
		"results_count":     len(job.Results),
		"elapsed_time":      job.ElapsedTime,
		"eta":               job.ETA,
		"matched":           job.Matched,
		"different":         job.Different,
		"no_template":       job.NoTemplate,
		"job_id":            job.ID,
		"project_name":      job.ProjectName,
	})
}

// handleResults returns results for a specific job or first available
func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")

	var job *CompareJob
	if projectID != "" {
		job = s.jobs.GetExisting(projectID)
	}
	if job == nil {
		running := s.jobs.RunningJobs()
		if len(running) > 0 {
			job = running[0]
		} else {
			all := s.jobs.All()
			if len(all) > 0 {
				job = all[0]
			}
		}
	}

	if job == nil {
		JSONResponse(w, http.StatusOK, map[string]any{"results": []any{}, "count": 0})
		return
	}

	job.mu.Lock()
	defer job.mu.Unlock()
	JSONResponse(w, http.StatusOK, map[string]any{
		"results": job.Results,
		"count":   len(job.Results),
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

// formatDuration is imported from session.go
var _ = sync.Mutex{}