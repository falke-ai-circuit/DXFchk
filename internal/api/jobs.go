package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/falke-ai-circuit/DXFchk/internal/compare"
)

// CompareJob holds the state of a single comparison job
type CompareJob struct {
	ID             string              `json:"id"`
	ProjectName    string              `json:"project_name"`
	Running        bool                `json:"running"`
	TotalFiles     int                 `json:"total_files"`
	ProcessedFiles int                 `json:"processed_files"`
	LogMessages    []string            `json:"log_messages"`
	Results        []any               `json:"results"`
	TemplateMap    compare.TemplateMap `json:"-"`
	StartTime      time.Time           `json:"start_time"`
	ElapsedTime    string              `json:"elapsed_time"`
	ETA            string              `json:"eta"`
	Matched        int                 `json:"matched"`
	Different      int                 `json:"different"`
	NoTemplate     int                 `json:"no_template"`
	StopChan       chan struct{}       `json:"-"`
	SearchFolder   string              `json:"search_folder"`
	OutputFolder   string              `json:"output_folder"`
	TemplateFolder string              `json:"template_folder"`
	mu             sync.Mutex          `json:"-"`
}

// JobManager manages multiple parallel comparison jobs
type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*CompareJob
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]*CompareJob)}
}

func (jm *JobManager) Get(id string) *CompareJob {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, ok := jm.jobs[id]; ok {
		return job
	}
	job := &CompareJob{ID: id, LogMessages: []string{}, StopChan: make(chan struct{}, 1)}
	jm.jobs[id] = job
	return job
}

func (jm *JobManager) GetExisting(id string) *CompareJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[id]
}

func (jm *JobManager) All() []*CompareJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	list := make([]*CompareJob, 0, len(jm.jobs))
	for _, j := range jm.jobs {
		list = append(list, j)
	}
	return list
}

func (jm *JobManager) RunningJobs() []*CompareJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	var running []*CompareJob
	for _, j := range jm.jobs {
		if j.Running {
			running = append(running, j)
		}
	}
	return running
}

func (jm *JobManager) Remove(id string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.jobs, id)
}

func (j *CompareJob) AddLog(msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.LogMessages = append(j.LogMessages, msg)
	if len(j.LogMessages) > 500 {
		j.LogMessages = j.LogMessages[len(j.LogMessages)-500:]
	}
}

func (j *CompareJob) RecentLogs(n int) []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.LogMessages) <= n {
		return j.LogMessages
	}
	return j.LogMessages[len(j.LogMessages)-n:]
}

func jobSessionPath(jobID string) string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".dxfchk", fmt.Sprintf("session_%s.json", jobID))
}

func SaveJobSession(job *CompareJob) error {
	path := jobSessionPath(job.ID)
	os.MkdirAll(filepath.Dir(path), 0755)
	state := &SessionState{
		ProjectID:      job.ID,
		SearchFolder:   job.SearchFolder,
		OutputFolder:   job.OutputFolder,
		TemplateFolder: job.TemplateFolder,
		StartTime:      job.StartTime,
		TotalFiles:     job.TotalFiles,
		ProcessedFiles: job.ProcessedFiles,
		Matched:        job.Matched,
		Different:      job.Different,
		NoTemplate:     job.NoTemplate,
		Status:         "running",
	}
	if !job.Running {
		state.Status = "completed"
		state.EndTime = time.Now()
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}