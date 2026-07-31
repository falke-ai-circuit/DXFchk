package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/falke-ai-circuit/DXFchk/internal/compare"
)

//go:embed frontend_dist
var frontendFS embed.FS

// Server is the HTTP API server
type Server struct {
	mux          *http.ServeMux
	frontendFile http.Handler

	// Application state
	settings      *Settings
	jobs          *JobManager
	session       *SessionState
	stopChan      chan struct{}

	// Project copy progress tracking (keyed by project ID)
	copyStatuses  map[string]*CopyStatus
	copyMu        sync.Mutex
}

// Settings holds the application settings
type Settings struct {
	TemplateFolder     string `json:"template_folder"`
	SearchFolder       string `json:"search_folder"`
	OutputFolder       string `json:"output_folder"`
	Recursive          bool   `json:"recursive"`
	MoveFiles          bool   `json:"move_files"`
	GroupByContent     bool   `json:"group_by_content"`
	AutoCreateMod      bool   `json:"auto_create_mod_templates"`
	AutoApplyToGroup   bool   `json:"auto_apply_to_group"`
	ExternalEditorPath string `json:"external_editor_path"`
	ActiveProjectID    string `json:"active_project_id"`
	FolderTemplates    string `json:"folder_templates"`
	FolderUnchecked    string `json:"folder_unchecked"`
	FolderOutput       string `json:"folder_output"`
}

// CompareState holds the current comparison state
type CompareState struct {
	Running        bool               `json:"running"`
	TotalFiles     int                `json:"total_files"`
	ProcessedFiles int                `json:"processed_files"`
	LogMessages    []string           `json:"log_messages"`
	Results        []any              `json:"results"`
	TemplateMap    compare.TemplateMap `json:"-"`
	StartTime      time.Time          `json:"start_time"`
	ElapsedTime    string             `json:"elapsed_time"`
	ETA            string             `json:"eta"`
	Matched        int                `json:"matched"`
	Different      int                `json:"different"`
	NoTemplate     int                `json:"no_template"`
}

// NewServer creates a new API server
func NewServer() *Server {
	s := &Server{
		settings: &Settings{
			GroupByContent: true,
			Recursive:      true,
		},
		jobs:         NewJobManager(),
		copyStatuses: make(map[string]*CopyStatus),
	}

	// Create sub-filesystem for frontend (strips the frontend_dist/ prefix)
	subFS, err := fs.Sub(frontendFS, "frontend_dist")
	if err != nil {
		panic(fmt.Sprintf("failed to create sub filesystem: %v", err))
	}
	s.frontendFile = http.FileServer(http.FS(subFS))

	s.mux = http.NewServeMux()
	s.routes()
	return s
}

// routes registers all API routes
func (s *Server) routes() {
	// Go 1.22+ ServeMux with method patterns
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/settings", s.handleSettings)
	s.mux.HandleFunc("POST /api/v1/settings", s.handleSettings)
	s.mux.HandleFunc("POST /api/v1/templates/scan", s.handleScanTemplates)
	s.mux.HandleFunc("GET /api/v1/templates", s.handleGetTemplates)
	s.mux.HandleFunc("POST /api/v1/compare", s.handleCompare)
	s.mux.HandleFunc("GET /api/v1/compare/status", s.handleCompareStatus)
	s.mux.HandleFunc("GET /api/v1/compare/jobs", s.handleAllJobs) // all running jobs
	s.mux.HandleFunc("POST /api/v1/compare/stop", s.handleCompareStop)
	s.mux.HandleFunc("GET /api/v1/results", s.handleResults)
	s.mux.HandleFunc("GET /api/v1/projects", s.handleProjects)
	s.mux.HandleFunc("POST /api/v1/projects", s.handleProjects)
	s.mux.HandleFunc("GET /api/v1/project", s.handleProject)
	s.mux.HandleFunc("POST /api/v1/project", s.handleProject)
	s.mux.HandleFunc("DELETE /api/v1/project", s.handleProject)
	s.mux.HandleFunc("GET /api/v1/browse", s.handleBrowse)
	s.mux.HandleFunc("GET /api/v1/browse/folder", s.handleBrowseFolder)
	s.mux.HandleFunc("POST /api/v1/diff", s.handleDXFDiff)
	s.mux.HandleFunc("POST /api/v1/template/create", s.handleCreateTemplate)
	s.mux.HandleFunc("POST /api/v1/compare/resume", s.handleCompareResume)
	s.mux.HandleFunc("GET /api/v1/session", s.handleSession)
	s.mux.HandleFunc("DELETE /api/v1/session", s.handleSession)
	s.mux.HandleFunc("POST /api/v1/project/import", s.handleProjectImport)
	s.mux.HandleFunc("GET /api/v1/project/export", s.handleProjectExport)
	s.mux.HandleFunc("POST /api/v1/template/apply", s.handleApplyTemplate)
	s.mux.HandleFunc("POST /api/v1/template/preview", s.handlePreviewTemplate)
	s.mux.HandleFunc("POST /api/v1/template/create-group", s.handleCreateTemplateFromGroup)
	s.mux.HandleFunc("GET /api/v1/template/groups", s.handleTemplateGroups)
	s.mux.HandleFunc("GET /api/v1/log", s.handleLogContent)
	s.mux.HandleFunc("GET /api/v1/dxf/content", s.handleDXFContent)
	s.mux.HandleFunc("GET /api/v1/dxf/render", s.handleDXFRender)
	s.mux.HandleFunc("POST /api/v1/dxf/content", s.handleDXFContent)
	s.mux.HandleFunc("POST /api/v1/template/edit-script", s.handleEditScript)
	s.mux.HandleFunc("GET /api/v1/template/group", s.handleTemplateGroupDetail) // ?name=XXX
	s.mux.HandleFunc("GET /api/v1/browse/system", s.handleBrowseSystem)
	s.mux.HandleFunc("GET /api/v1/project/zip-export", s.handleProjectZipExport)
	s.mux.HandleFunc("POST /api/v1/project/zip-import", s.handleProjectZipImport)
	s.mux.HandleFunc("POST /api/v1/open", s.handleOpenExternal) // open DXF in external editor
	s.mux.HandleFunc("GET /api/v1/project/copy-status", s.handleCopyStatus) // project copy progress
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// API routes
	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.mux.ServeHTTP(w, r)
		return
	}

	// Serve frontend (SPA fallback)
	s.serveFrontend(w, r)
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	// For SPA: serve index.html for non-file routes
	rPath := r.URL.Path
	if rPath == "/" {
		s.frontendFile.ServeHTTP(w, r)
		return
	}

	// Try to serve the file directly
	fSub, err := fs.Sub(frontendFS, "frontend_dist")
	if err != nil {
		http.Error(w, "frontend not available", http.StatusInternalServerError)
		return
	}

	// Check if file exists
	f, err := fSub.Open(strings.TrimPrefix(rPath, "/"))
	if err != nil {
		// SPA fallback: serve index.html
		r.URL.Path = "/"
		s.frontendFile.ServeHTTP(w, r)
		return
	}
	f.Close()

	// File exists, serve it
	s.frontendFile.ServeHTTP(w, r)
}

// JSONResponse writes a JSON response
func JSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ErrorResponse writes a JSON error response
func ErrorResponse(w http.ResponseWriter, status int, msg string) {
	JSONResponse(w, status, map[string]any{"error": msg})
}