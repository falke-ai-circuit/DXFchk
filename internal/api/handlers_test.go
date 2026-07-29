package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper to create a test server with a temp projects file
func newTestServer(t *testing.T) *Server {
	t.Helper()
	s := NewServer()
	// Override the home dir for projects by setting HOME env
	// (projects.json is stored in ~/.dxfchk/projects.json)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	return s
}

// helper to make a request and get response
func doRequest(t *testing.T, s *Server, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

// helper to decode JSON response
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode JSON response: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// TestHealthEndpoint verifies the health endpoint returns status "ok".
func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(t, s, "GET", "/api/v1/health", "")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	result := decodeJSON(t, w)
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%v'", result["status"])
	}
	if result["version"] == nil {
		t.Error("expected version field in health response")
	}
}

// TestProjectsCRUD verifies the full project CRUD lifecycle.
func TestProjectsCRUD(t *testing.T) {
	s := newTestServer(t)
	tmpDir := t.TempDir()

	// Create folders for the project
	templateDir := filepath.Join(tmpDir, "templates")
	searchDir := filepath.Join(tmpDir, "search")
	os.MkdirAll(templateDir, 0755)
	os.MkdirAll(searchDir, 0755)

	// Copy a template file so scanning works
	templateData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "template_bi001.dxf"))
	os.WriteFile(filepath.Join(templateDir, "BI001.dxf"), templateData, 0644)

	// 1. Create project
	createBody := `{"name":"Test Project","template_folder":"` + templateDir + `","search_folder":"` + searchDir + `"}`
	w := doRequest(t, s, "POST", "/api/v1/projects", createBody)

	if w.Code != http.StatusOK {
		t.Fatalf("create project: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}

	project, ok := result["project"].(map[string]any)
	if !ok {
		t.Fatal("expected 'project' field in response")
	}
	projectID, _ := project["id"].(string)
	if projectID == "" {
		t.Fatal("expected non-empty project ID")
	}
	if project["name"] != "Test Project" {
		t.Errorf("expected name 'Test Project', got '%v'", project["name"])
	}

	// 2. List projects
	w = doRequest(t, s, "GET", "/api/v1/projects", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list projects: expected 200, got %d", w.Code)
	}

	result = decodeJSON(t, w)
	if result["count"].(float64) < 1 {
		t.Errorf("expected at least 1 project, got count=%v", result["count"])
	}

	// 3. Get single project
	w = doRequest(t, s, "GET", "/api/v1/project?id="+projectID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get project: expected 200, got %d", w.Code)
	}

	result = decodeJSON(t, w)
	project, ok = result["project"].(map[string]any)
	if !ok {
		t.Fatal("expected 'project' field in get response")
	}
	if project["id"] != projectID {
		t.Errorf("expected id='%s', got '%v'", projectID, project["id"])
	}

	// 4. Update project
	updateBody := `{"name":"Updated Project"}`
	w = doRequest(t, s, "POST", "/api/v1/project?id="+projectID, updateBody)
	if w.Code != http.StatusOK {
		t.Fatalf("update project: expected 200, got %d", w.Code)
	}

	result = decodeJSON(t, w)
	project, _ = result["project"].(map[string]any)
	if project["name"] != "Updated Project" {
		t.Errorf("expected updated name, got '%v'", project["name"])
	}

	// 5. Delete project
	w = doRequest(t, s, "DELETE", "/api/v1/project?id="+projectID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete project: expected 200, got %d", w.Code)
	}

	// 6. Verify deleted
	w = doRequest(t, s, "GET", "/api/v1/project?id="+projectID, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted project, got %d", w.Code)
	}
}

// TestProjectValidation verifies that missing required fields return errors.
func TestProjectValidation(t *testing.T) {
	s := newTestServer(t)

	// Missing name
	w := doRequest(t, s, "POST", "/api/v1/projects", `{"template_folder":"/tmp","search_folder":"/tmp"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}

	// Missing template_folder
	w = doRequest(t, s, "POST", "/api/v1/projects", `{"name":"Test","search_folder":"/tmp"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing template_folder, got %d", w.Code)
	}

	// Missing search_folder
	w = doRequest(t, s, "POST", "/api/v1/projects", `{"name":"Test","template_folder":"/tmp"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing search_folder, got %d", w.Code)
	}
}

// TestCompareStartAndStatus verifies starting a comparison and checking status.
func TestCompareStartAndStatus(t *testing.T) {
	s := newTestServer(t)
	tmpDir := t.TempDir()

	// Setup template and search folders
	templateDir := filepath.Join(tmpDir, "templates")
	searchDir := filepath.Join(tmpDir, "search")
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(templateDir, 0755)
	os.MkdirAll(searchDir, 0755)

	templateData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "template_bi001.dxf"))
	os.WriteFile(filepath.Join(templateDir, "BI001.dxf"), templateData, 0644)

	matchData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "module_bi001_match.dxf"))
	os.WriteFile(filepath.Join(searchDir, "BI001_match.dxf"), matchData, 0644)

	// Start comparison
	compareBody := `{"project_id":"test-job-1","project_name":"Test","search_folder":"` + searchDir + `","template_folder":"` + templateDir + `","output_folder":"` + outputDir + `"}`
	w := doRequest(t, s, "POST", "/api/v1/compare", compareBody)

	if w.Code != http.StatusOK {
		t.Fatalf("start compare: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
	if result["job_id"] != "test-job-1" {
		t.Errorf("expected job_id='test-job-1', got '%v'", result["job_id"])
	}

	// Check status (the comparison should have completed quickly for 1 file)
	// We may need to poll a few times since it runs in a goroutine
	var status map[string]any
	for i := 0; i < 100; i++ {
		w = doRequest(t, s, "GET", "/api/v1/compare/status?project_id=test-job-1", "")
		if w.Code != http.StatusOK {
			t.Fatalf("compare status: expected 200, got %d", w.Code)
		}
		status = decodeJSON(t, w)
		if status["running"] == false {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if status["running"] == true {
		t.Log("comparison still running after polling — may be slow, checking results anyway")
	}

	// Check that we got some results
	resultsCount, _ := status["results_count"].(float64)
	if resultsCount < 1 {
		// Check if it completed — give it more time
		for i := 0; i < 100; i++ {
			w = doRequest(t, s, "GET", "/api/v1/compare/status?project_id=test-job-1", "")
			status = decodeJSON(t, w)
			rc, _ := status["results_count"].(float64)
			if status["running"] == false && rc > 0 {
				resultsCount = rc
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if resultsCount < 1 {
			t.Errorf("expected results_count >= 1, got %v", status["results_count"])
		}
	}
}

// TestCompareStop verifies that stopping a non-running job returns an error.
func TestCompareStop(t *testing.T) {
	s := newTestServer(t)

	// Stop non-existent job
	w := doRequest(t, s, "POST", "/api/v1/compare/stop", `{"project_id":"nonexistent"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for stopping non-running job, got %d", w.Code)
	}
}

// TestCompareNoTemplates verifies that starting a comparison with no templates fails.
func TestCompareNoTemplates(t *testing.T) {
	s := newTestServer(t)
	tmpDir := t.TempDir()

	// Empty template folder
	templateDir := filepath.Join(tmpDir, "templates")
	searchDir := filepath.Join(tmpDir, "search")
	os.MkdirAll(templateDir, 0755)
	os.MkdirAll(searchDir, 0755)

	compareBody := `{"project_id":"test-no-templates","search_folder":"` + searchDir + `","template_folder":"` + templateDir + `"}`
	w := doRequest(t, s, "POST", "/api/v1/compare", compareBody)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for no templates, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBrowseTree verifies the browse tree endpoint returns folder structure.
func TestBrowseTree(t *testing.T) {
	s := newTestServer(t)
	tmpDir := t.TempDir()

	// Create output folder structure
	outputDir := filepath.Join(tmpDir, "output")
	templateFolder := filepath.Join(outputDir, "BI001")
	modFolder := filepath.Join(templateFolder, "BI001_mod1")
	os.MkdirAll(modFolder, 0755)

	// Add a DXF file in the mod folder
	dxfData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "module_bi001_different.dxf"))
	os.WriteFile(filepath.Join(modFolder, "BI001_module1.dxf"), dxfData, 0644)

	// Set output folder in settings
	s.settings.OutputFolder = outputDir

	w := doRequest(t, s, "GET", "/api/v1/browse", "")

	if w.Code != http.StatusOK {
		t.Fatalf("browse: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)
	if result["empty"] == true {
		t.Fatal("expected non-empty tree")
	}

	tree, ok := result["tree"].(map[string]any)
	if !ok {
		t.Fatal("expected 'tree' field in browse response")
	}

	// Tree should have BI001 as a child
	children, ok := tree["children"].([]any)
	if !ok {
		t.Fatal("expected 'children' in tree node")
	}

	var bi001Node map[string]any
	for _, c := range children {
		node := c.(map[string]any)
		if node["name"] == "BI001" {
			bi001Node = node
			break
		}
	}
	if bi001Node == nil {
		t.Fatal("expected BI001 folder in tree")
	}

	if bi001Node["is_template"] != true {
		t.Error("expected BI001 to be marked as template")
	}

	// BI001 should have BI001_mod1 as a child
	bi001Children, _ := bi001Node["children"].([]any)
	var modNode map[string]any
	for _, c := range bi001Children {
		node := c.(map[string]any)
		if node["name"] == "BI001_mod1" {
			modNode = node
			break
		}
	}
	if modNode == nil {
		t.Fatal("expected BI001_mod1 folder nested under BI001")
	}

	if modNode["is_mod"] != true {
		t.Error("expected BI001_mod1 to be marked as mod")
	}
}

// TestBrowseNonExistent verifies browse returns 404 for non-existent folder.
func TestBrowseNonExistent(t *testing.T) {
	s := newTestServer(t)
	s.settings.OutputFolder = "/nonexistent/path/12345"

	w := doRequest(t, s, "GET", "/api/v1/browse", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent folder, got %d", w.Code)
	}
}

// TestDXFRender verifies the render endpoint returns entities from a DXF file.
func TestDXFRender(t *testing.T) {
	s := newTestServer(t)
	dxfPath, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "template_bi001.dxf"))

	w := doRequest(t, s, "GET", "/api/v1/dxf/render?path="+dxfPath, "")

	if w.Code != http.StatusOK {
		t.Fatalf("render: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)
	count, _ := result["count"].(float64)
	if count < 1 {
		t.Errorf("expected at least 1 entity, got count=%v", count)
	}

	entities, ok := result["entities"].([]any)
	if !ok {
		t.Fatal("expected 'entities' array in render response")
	}
	if len(entities) == 0 {
		t.Error("expected non-empty entities array")
	}

	// Check bounding box
	bbox, ok := result["bounding_box"].([]any)
	if !ok || len(bbox) != 4 {
		t.Error("expected bounding_box with 4 elements")
	}
}

// TestDXFRenderValidation verifies render endpoint validates input.
func TestDXFRenderValidation(t *testing.T) {
	s := newTestServer(t)

	// Missing path
	w := doRequest(t, s, "GET", "/api/v1/dxf/render", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing path, got %d", w.Code)
	}

	// Non-DXF file
	w = doRequest(t, s, "GET", "/api/v1/dxf/render?path=/tmp/test.txt", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-DXF file, got %d", w.Code)
	}

	// Non-existent DXF
	w = doRequest(t, s, "GET", "/api/v1/dxf/render?path=/nonexistent/file.dxf", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent file, got %d", w.Code)
	}
}

// TestDXFDiff verifies the diff endpoint compares two DXF files.
func TestDXFDiff(t *testing.T) {
	s := newTestServer(t)
	templatePath, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "template_bi001.dxf"))
	modulePath, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "module_bi001_different.dxf"))

	diffBody := `{"template_path":"` + templatePath + `","module_path":"` + modulePath + `"}`
	w := doRequest(t, s, "POST", "/api/v1/diff", diffBody)

	if w.Code != http.StatusOK {
		t.Fatalf("diff: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)

	// Should have template and module entities
	templateEntities, ok := result["template_entities"].([]any)
	if !ok {
		t.Fatal("expected 'template_entities' in diff response")
	}
	moduleEntities, ok := result["module_entities"].([]any)
	if !ok {
		t.Fatal("expected 'module_entities' in diff response")
	}

	if len(templateEntities) == 0 {
		t.Error("expected non-empty template entities")
	}
	if len(moduleEntities) == 0 {
		t.Error("expected non-empty module entities")
	}

	// The different module has an extra LINE on layer "MODIFIED"
	// So there should be added entities
	added, ok := result["added"].([]any)
	if !ok {
		t.Fatal("expected 'added' array in diff response")
	}
	if len(added) == 0 {
		t.Error("expected at least 1 added entity (the MODIFIED layer line)")
	}

	// Check summary
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatal("expected 'summary' in diff response")
	}
	if summary["added_count"].(float64) < 1 {
		t.Errorf("expected added_count >= 1, got %v", summary["added_count"])
	}
}

// TestDXFContent verifies the content endpoint returns raw DXF text.
func TestDXFContent(t *testing.T) {
	s := newTestServer(t)
	dxfPath, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "template_bi001.dxf"))

	w := doRequest(t, s, "GET", "/api/v1/dxf/content?path="+dxfPath, "")

	if w.Code != http.StatusOK {
		t.Fatalf("content: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)
	content, _ := result["content"].(string)
	if content == "" {
		t.Error("expected non-empty content")
	}
	if !strings.Contains(content, "SECTION") {
		t.Error("expected DXF content to contain 'SECTION'")
	}
}

// TestSettingsGetSet verifies settings GET and POST.
func TestSettingsGetSet(t *testing.T) {
	s := newTestServer(t)

	// Get default settings
	w := doRequest(t, s, "GET", "/api/v1/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get settings: expected 200, got %d", w.Code)
	}

	// Update settings
	w = doRequest(t, s, "POST", "/api/v1/settings", `{"template_folder":"/tmp/templates","search_folder":"/tmp/search","output_folder":"/tmp/output","recursive":true,"group_by_content":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("set settings: expected 200, got %d", w.Code)
	}

	result := decodeJSON(t, w)
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}

	// Verify settings were updated
	if s.settings.TemplateFolder != "/tmp/templates" {
		t.Errorf("expected template_folder '/tmp/templates', got '%s'", s.settings.TemplateFolder)
	}
}

// TestScanTemplates verifies the template scanning endpoint.
func TestScanTemplates(t *testing.T) {
	s := newTestServer(t)
	tmpDir := t.TempDir()

	// Copy template files
	templateData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "template_bi001.dxf"))
	os.WriteFile(filepath.Join(tmpDir, "BI001.dxf"), templateData, 0644)

	templateData2, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "template_bo002.dxf"))
	os.WriteFile(filepath.Join(tmpDir, "BO002.dxf"), templateData2, 0644)

	scanBody := `{"template_folder":"` + tmpDir + `","recursive":false}`
	w := doRequest(t, s, "POST", "/api/v1/templates/scan", scanBody)

	if w.Code != http.StatusOK {
		t.Fatalf("scan templates: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := decodeJSON(t, w)
	count, _ := result["count"].(float64)
	if count != 2 {
		t.Errorf("expected 2 templates, got %v", count)
	}

	mapping, ok := result["mapping"].(map[string]any)
	if !ok {
		t.Fatal("expected 'mapping' in scan response")
	}
	if _, ok := mapping["BI001"]; !ok {
		t.Error("expected 'BI001' in template mapping")
	}
	if _, ok := mapping["BO002"]; !ok {
		t.Error("expected 'BO002' in template mapping")
	}
}

// TestAllJobs verifies the all-jobs endpoint.
func TestAllJobs(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(t, s, "GET", "/api/v1/compare/jobs", "")

	if w.Code != http.StatusOK {
		t.Fatalf("all jobs: expected 200, got %d", w.Code)
	}

	result := decodeJSON(t, w)
	if result["count"] == nil {
		t.Error("expected 'count' in all-jobs response")
	}
}

// TestCORSHeaders verifies CORS headers are set on API responses.
func TestCORSHeaders(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(t, s, "GET", "/api/v1/health", "")

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS Allow-Origin: *")
	}
}

// TestOPTIONSRequest verifies OPTIONS requests return 200 (preflight).
func TestOPTIONSRequest(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", w.Code)
	}
}