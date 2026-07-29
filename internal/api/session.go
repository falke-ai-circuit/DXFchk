package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SessionState holds resumable comparison state
type SessionState struct {
	ProjectID       string    `json:"project_id"`
	SearchFolder    string    `json:"search_folder"`
	OutputFolder    string    `json:"output_folder"`
	TemplateFolder  string    `json:"template_folder"`
	Recursive       bool      `json:"recursive"`
	MoveFiles       bool      `json:"move_files"`
	GroupByContent  bool      `json:"group_by_content"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time,omitempty"`
	TotalFiles      int       `json:"total_files"`
	ProcessedFiles  int       `json:"processed_files"`
	ProcessedFiles_ []string  `json:"processed_files_list"` // files already done
	Matched         int       `json:"matched"`
	Different       int       `json:"different"`
	NoTemplate      int       `json:"no_template"`
	Status          string    `json:"status"` // "running", "completed", "stopped", "paused"
	Paused          bool      `json:"paused"`
}

// sessionPath returns the session file path
func sessionPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".dxfchk", "session.json")
}

// SaveSession saves the current session state to disk
func (s *Server) SaveSession(state *SessionState) error {
	path := sessionPath()
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadSession loads the session state from disk
func LoadSession() (*SessionState, error) {
	path := sessionPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ClearSession removes the session file
func ClearSession() error {
	path := sessionPath()
	return os.Remove(path)
}

// ElapsedTime returns formatted elapsed time
func (st *SessionState) ElapsedTime() string {
	if st.StartTime.IsZero() {
		return "00:00:00"
	}
	end := st.EndTime
	if end.IsZero() {
		end = time.Now()
	}
	elapsed := end.Sub(st.StartTime)
	return formatDuration(elapsed)
}

// ETA returns estimated time remaining
func (st *SessionState) ETA() string {
	if st.ProcessedFiles == 0 || st.TotalFiles == 0 {
		return "--:--:--"
	}
	end := st.EndTime
	if end.IsZero() {
		end = time.Now()
	}
	elapsed := end.Sub(st.StartTime)
	perFile := elapsed / time.Duration(st.ProcessedFiles)
	remaining := st.TotalFiles - st.ProcessedFiles
	eta := perFile * time.Duration(remaining)
	return formatDuration(eta)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return time.Time{}.Add(time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second).Format("15:04:05")
}