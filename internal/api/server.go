package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/falke-ai-circuit/DXFchk/internal/compare"
)

// Server is the HTTP API server
type Server struct {
	mux *http.ServeMux

	// Application state
	settings     *Settings
	compareState  *CompareState
}

// Settings holds the application settings
type Settings struct {
	TemplateFolder  string `json:"template_folder"`
	SearchFolder    string `json:"search_folder"`
	OutputFolder    string `json:"output_folder"`
	Recursive       bool   `json:"recursive"`
	MoveFiles       bool   `json:"move_files"`
	GroupByContent  bool   `json:"group_by_content"`
}

// CompareState holds the current comparison state
type CompareState struct {
	Running        bool               `json:"running"`
	TotalFiles     int                `json:"total_files"`
	ProcessedFiles int                `json:"processed_files"`
	LogMessages    []string           `json:"log_messages"`
	Results        []any              `json:"results"`
	TemplateMap    compare.TemplateMap `json:"-"`
}

// NewServer creates a new API server
func NewServer() *Server {
	s := &Server{
		settings: &Settings{
			GroupByContent: true,
		},
		compareState: &CompareState{
			LogMessages: []string{},
		},
	}
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
	s.mux.HandleFunc("GET /api/v1/results", s.handleResults)
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

	// Static file serving for frontend (if embedded)
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		s.serveFrontend(w, r)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	// TODO: serve embedded React frontend
	// For now, return a simple HTML placeholder
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>DXFchk</title></head><body style="background:#1a1a1a;color:#008a00;font-family:monospace;padding:2rem"><h1>DXFchk v0.1.0</h1><p>API is running. Frontend not yet built.</p><p>Health: <a href="/api/v1/health" style="color:#00ff41">/api/v1/health</a></p></body></html>`)
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