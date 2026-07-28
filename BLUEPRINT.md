# DXFchk — Architecture Blueprint

> DXF Module-Template Comparison Tool
> Go backend + React frontend rewrite of Python DXF Compare Tool
> Repo: `github.com/falke-ai-circuit/DXFchk`

## 1. Overview

DXFchk compares DXF module files against DXF template files (Valmet Eclipse automation system). It:
- Extracts `$(TEMPLATE)` attribute from each module to identify its template
- Compares module geometry (blocks/lines/polylines) against template geometry
- Groups modules with identical modifications into `_modN` subfolders
- Saves detailed comparison logs per template
- Provides a web UI for running comparisons and viewing results

## 2. Project Structure

```
dxfchk/
├── cmd/
│   └── dxfchk/
│       └── main.go              # Entry point: HTTP server
├── internal/
│   ├── dxf/
│   │   ├── parser.go             # Native DXF text format parser
│   │   ├── entities.go            # Entity type definitions (INSERT, LINE, POLYLINE, etc.)
│   │   └── extractor.go           # High-level extraction (blocks/lines/polylines dicts)
│   ├── compare/
│   │   ├── processor.go           # ComparisonProcessor (port from Python)
│   │   ├── template.go            # Template map building + $(TEMPLATE) extraction
│   │   ├── diff.go                # Dict-of-lists comparison (port from Python)
│   │   └── hash.go                # Content hashing (MD5 of JSON)
│   ├── api/
│   │   ├── server.go              # HTTP server + routes
│   │   ├── handlers.go            # REST API handlers
│   │   └── middleware.go          # CORS, logging, auth
│   ├── db/
│   │   ├── database.go            # SQLite connection + migrations
│   │   └── models.go              # Data models/structs
│   └── config/
│       └── config.go              # App configuration
├── web/                           # React frontend
│   ├── src/
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   ├── components/
│   │   │   ├── Layout.tsx          # Main layout (sidebar + content)
│   │   │   ├── FolderSelect.tsx     # Template/search/output folder selection
│   │   │   ├── ComparisonRun.tsx    # Run comparison UI (progress, log)
│   │   │   ├── ResultsView.tsx      # Results tree (templates → _modN → files)
│   │   │   ├── DiffViewer.tsx       # Visual diff (template vs module)
│   │   │   └── LogView.tsx         # Comparison log viewer
│   │   ├── api/
│   │   │   └── client.ts           # API client (fetch wrapper)
│   │   └── styles/
│   │       └── theme.css           # Valmet green theme (#008a00)
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── go.mod
├── go.sum
├── CHANGELOG.md
├── README.md
└── BLUEPRINT.md
```

## 3. API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/settings` | Get current settings (folders, options) |
| POST | `/api/v1/settings` | Update settings |
| POST | `/api/v1/templates/scan` | Scan template folder → build template map |
| GET | `/api/v1/templates` | Get template map (template_name → file_path) |
| POST | `/api/v1/compare` | Run comparison (body: {search_folder, recursive, move_files, group_by_content}) |
| GET | `/api/v1/compare/status` | Get comparison progress (SSE stream or polling) |
| GET | `/api/v1/results` | Get comparison results (templates, modN folders, file lists) |
| GET | `/api/v1/results/{template}/files` | Get files in a template folder |
| GET | `/api/v1/results/{template}/log` | Get detailed comparison log for template |
| GET | `/api/v1/diff?file=path&template=name` | Get visual diff data (blocks/lines/polylines differences) |
| POST | `/api/v1/export` | Export results (ZIP) |

## 4. Data Models (Go Structs)

```go
// DXF entity types extracted from files
type BlockInstance struct {
    Name     string      // block name (entity.dxf.name)
    Position [3]float64  // (x, y, z) rounded to N decimals
}

type LineEntity struct {
    Layer   string
    Start   [3]float64
    End     [3]float64
}

type PolylineEntity struct {
    Layer    string
    Vertices [][3]float64
}

// DXFFile represents parsed DXF content
type DXFFile struct {
    Blocks    map[string][][3]float64    // block_name → list of positions
    Lines     map[string][][[2][3]float64]  // layer → list of (start, end) sorted pairs
    Polylines map[string][][][3]float64      // layer → list of vertex tuples (normalized)
    Template  string                       // $(TEMPLATE) attribute value (if found)
    FileHash  string                       // MD5 of file content
}

// ComparisonResult for a single file
type ComparisonResult struct {
    FileName    string    `json:"file_name"`
    Template    string    `json:"template"`    // template name or "notemplate"
    Status      string    `json:"status"`      // "match" | "different" | "no_template"
    ContentHash string    `json:"content_hash"` // MD5 of (blocks, lines, polylines)
    ModFolder   string    `json:"mod_folder"`  // e.g. "mod1", "mod2", or ""
    Differences *Diff     `json:"diffs,omitempty"`
}

type Diff struct {
    Blocks    DictDiff `json:"blocks"`
    Lines     DictDiff `json:"lines"`
    Polylines DictDiff `json:"polylines"`
}

type DictDiff struct {
    Common   []string `json:"common"`     // keys in both
    OnlyIn1  []string `json:"only_in_1"`  // keys only in module
    OnlyIn2  []string `json:"only_in_2"`  // keys only in template
    Details  map[string]KeyDiff `json:"details,omitempty"`
}

type KeyDiff struct {
    Common  []any `json:"common"`
    OnlyIn1 []any `json:"only_in_1"`
    OnlyIn2 []any `json:"only_in_2"`
}

// TemplateFolder represents output structure
type TemplateFolder struct {
    Name       string         `json:"name"`        // template name
    MatchCount int            `json:"match_count"` // identical files
    ModFolders []ModFolder    `json:"mod_folders"` // _modN subfolders
}

type ModFolder struct {
    Name     string   `json:"name"`      // "mod1", "mod2", ...
    Files    []string `json:"files"`     // file names in this mod folder
    Hash     string   `json:"hash"`     // content hash for this group
}
```

## 5. DXF Parser Design (Native Go)

DXF format is simple: alternating lines of (code, value) pairs.
- Odd lines = group code (integer, may have leading spaces)
- Even lines = value (string, float, or int depending on code)

```
0          ← entity type
INSERT
2          ← block name
AA
10         ← x coordinate
100.5
20         ← y coordinate
200.0
```

### Parser algorithm:
1. Read file line by line as text
2. Parse alternating (code, value) pairs — trim whitespace from code, parse as int
3. Group code pairs into entities (entity starts at code=0)
4. For each entity, extract relevant fields based on type:
   - **INSERT**: code 2 = block name, code 10/20/30 = insert position, code 66 = attribs follow flag
   - **ATTRIB**: code 2 = tag, code 1 = text value (follows INSERT entity)
   - **LINE**: code 8 = layer, code 10/20/30 = start, code 11/21/31 = end
   - **LWPOLYLINE**: code 8 = layer, code 10/20 = vertices (repeated)
   - **POLYLINE**: code 8 = layer, followed by VERTEX entities
5. For ATTRIB extraction specifically: scan all INSERT entities, collect their ATTRIB children, look for tag `$(TEMPLATE)` → return text value

### Key code pairs:
| Code | Meaning |
|------|---------|
| 0 | Entity type (INSERT, LINE, LWPOLYLINE, etc.) |
| 1 | Text value (ATTRIB text, TEXT content) |
| 2 | Name (block name for INSERT, tag for ATTRIB, layer for others) |
| 8 | Layer name |
| 10 | X coordinate (primary) |
| 20 | Y coordinate (primary) |
| 30 | Z coordinate (primary) |
| 11 | X coordinate (secondary - LINE end) |
| 21 | Y coordinate (secondary) |
| 31 | Z coordinate (secondary) |
| 66 | Attributes follow flag (INSERT) |

## 6. Template Matching Algorithm

1. Scan template folder for `.dxf` files
2. For each file, parse DXF → find INSERT entities → find ATTRIB with tag `$(TEMPLATE)`
3. Build `template_map`: `{template_name: file_path}`
4. For each module file in search folder:
   a. Parse DXF → find `$(TEMPLATE)` attribute value
   b. Look up template_name in template_map
   c. If found → compare module vs template
   d. If not found → copy to `output/notemplate/`

## 7. Comparison Algorithm (port from Python)

```python
# Python original (dxf_processor.py)
def get_blocks_lines_polylines_from_dxf(dxf_path, decimals=3):
    # Returns (blocks_dict, lines_dict, polylines_dict)
    # blocks_dict: {block_name: [(x, y, z), ...]}  (excludes "COMPANY" and "CUSTOMER")
    # lines_dict: {layer: [((x1,y1,z1), (x2,y2,z2)), ...]}  (endpoints sorted)
    # polylines_dict: {layer: [(vertices...), ...]}  (reversal-normalized)

def compare_dict_of_lists(dict1, dict2):
    # Returns (common_keys, only_in_1, only_in_2, diff)
    # diff: {key: {common: set, only_in_1: set, only_in_2: set}}

def create_content_hash(blocks_dict, lines_dict, polylines_dict):
    # MD5 of JSON-serialized (blocks, lines, polylines)
```

### Go port:
- `ExtractDXFContent(filePath string, decimals int) (*DXFFile, error)`
  - Parse DXF → extract blocks (INSERT positions, excluding COMPANY/CUSTOMER), lines (sorted endpoints), polylines (normalized vertices)
  - Round all coordinates to N decimals
- `CompareDicts(d1, d2 map[string][]any) DictDiff`
  - Set intersection/difference on keys and values
- `ContentHash(dxf *DXFFile) string`
  - `json.Marshal(dxf.Blocks, dxf.Lines, dxf.Polylines)` → MD5

## 8. _modN Folder Creation

After comparison:
1. Group all "different" files by template_name
2. Within each template group, sub-group by content_hash
3. Each unique content_hash → one `_modN` folder (N = 1, 2, 3, ...)
4. Files with same content_hash go in same `_modN` folder
5. Move files from template folder to `_modN` subfolder
6. Save detailed comparison log per template

## 9. Database Schema (SQLite)

```sql
CREATE TABLE IF NOT EXISTS modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    template_name TEXT,
    template_path TEXT,
    status TEXT,          -- 'match', 'different', 'no_template'
    content_hash TEXT,
    mod_folder TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_name TEXT UNIQUE NOT NULL,
    file_path TEXT NOT NULL,
    file_hash TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT
);
```

## 10. Frontend Structure

### Pages/Tabs:
1. **Setup** — Folder selection (template folder, search folder, output folder) + options (recursive, move files, group by content)
2. **Run** — Start comparison, progress bar, live log feed
3. **Results** — Tree view: template folders → _modN subfolders → files; click file to see diff
4. **Diff** — Side-by-side comparison: template vs module (blocks/lines/polylines differences highlighted)
5. **Logs** — Detailed comparison logs per template

### Theme (LOGReport style):
- Primary color: `#008a00` (Valmet green)
- Background: dark theme (`#1a1a1a` / `#0d0d0d`)
- Accent: `#00ff41` (matrix green glow for highlights)
- Font: monospace for technical data, sans-serif for UI
- Icons: Lucide React (file, folder, compare, check, alert, etc.)

## 11. Phased Implementation

### Phase 1: DXF Parser + Core Logic (Backend)
- `internal/dxf/parser.go` — native DXF text parser
- `internal/dxf/extractor.go` — blocks/lines/polylines extraction
- `internal/compare/template.go` — $(TEMPLATE) extraction + template map
- `internal/compare/diff.go` — dict comparison
- `internal/compare/hash.go` — content hashing
- `internal/compare/processor.go` — full ComparisonProcessor (port from Python)
- Unit tests: parse sample DXF files, verify extraction matches Python output

### Phase 2: REST API + Server
- `cmd/dxfchk/main.go` — HTTP server
- `internal/api/` — handlers for all endpoints
- `internal/db/` — SQLite database
- Test: curl all endpoints, verify API works

### Phase 3: React Frontend
- `web/` — Vite + React + TypeScript
- All components, API client, theme
- Build + embed in Go binary (`//go:embed`)
- Browser testing with vision audits

### Phase 4: Visual Diff
- DXF entity rendering (canvas/SVG)
- Highlight differences between template and module
- Color-coded overlay (green=common, red=only in module, blue=only in template)

### Phase 5: Export + Polish
- ZIP export of results
- Detailed log files
- Settings persistence
- Error handling and edge cases