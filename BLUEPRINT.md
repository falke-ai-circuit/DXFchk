# DXFchk — Architecture Blueprint

> DXF Module-Template Comparison Tool
> Go backend + React frontend rewrite of Python DXF Compare Tool
> Repo: `github.com/falke-ai-circuit/DXFchk`
> Current version: **v0.6.7**

## 1. Overview

DXFchk compares DXF module files against DXF template files (Valmet Eclipse automation system). It:
- Extracts `$(TEMPLATE)` attribute from each module to identify its template
- Compares module geometry (blocks/lines/polylines) against template geometry
- Groups modules with identical modifications into `_modN` subfolders
- Saves detailed comparison logs per template
- Provides a web UI with visual CAD rendering, diff highlighting, and parallel comparison

## 2. Project Structure (Actual)

```
dxfchk/
├── cmd/
│   ├── dxfchk/
│   │   └── main.go              # Entry point: HTTP server + CLI mode
│   └── debug_hash/              # Content hashing debug utility
├── internal/
│   ├── dxf/
│   │   ├── parser.go            # Native DXF text parser, entity/section extraction
│   │   └── extractor.go         # High-level extraction (blocks/lines/polylines)
│   ├── compare/
│   │   ├── processor.go         # ComparisonProcessor, _modN grouping, hash
│   │   ├── parallel.go          # 4-worker parallel processing (two-phase)
│   │   └── template.go          # Template map building, $(TEMPLATE) extraction
│   └── api/
│       ├── server.go            # HTTP server, routes, embedded frontend
│       ├── handlers.go          # Core API handlers (health, settings, compare, results)
│       ├── jobs.go              # JobManager for parallel comparison jobs
│       ├── browse.go            # Output folder tree builder
│       ├── diff.go              # DXF entity diff, rendering, template create
│       ├── projects.go          # Project CRUD (JSON store)
│       ├── template_apply.go    # Template groups, apply, edit scripts
│       ├── folderbrowser.go     # System folder browser, ZIP export/import
│       ├── session.go           # Session state persistence
│       ├── logdxf.go            # DXF render, raw content, log viewer
│       └── frontend_dist/       # Embedded built frontend (from frontend/dist/)
├── frontend/
│   ├── src/
│   │   ├── App.tsx              # Router setup (React Router v6)
│   │   ├── api.ts               # API client (fetch wrapper)
│   │   ├── store.ts             # Zustand store
│   │   ├── main.tsx             # Entry point
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx    # Project list, inline settings, "ID — NAME"
│   │   │   ├── Compare.tsx      # Folder selection, start/stop/resume, live progress
│   │   │   ├── Browse.tsx       # Output tree, single-view DXF with diff highlighting
│   │   │   ├── Edit.tsx         # Visual CAD + raw text toggle, right-click template creation
│   │   │   └── Settings.tsx     # Global settings
│   │   ├── components/
│   │   │   ├── DXFViewer.tsx    # Canvas-based CAD renderer with ACI colors
│   │   │   ├── Layout.tsx       # Nav bar + GlobalStatusBar
│   │   │   └── FolderBrowser.tsx # System folder browser dialog
│   │   └── styles/
│   │       └── theme.css        # Valmet green theme (#008a00)
│   ├── public/                  # Valmet logo, favicon
│   ├── package.json, vite.config.ts, tsconfig.json
├── reference-python-source/     # Original Python source (reference only)
├── go.mod
├── Makefile
├── BLUEPRINT.md, DESIGN.md, CHANGELOG.md, ROADMAP.md
├── CLAUDE.md, AGENTS.md, CONTRIBUTING.md
└── LICENSE
```

## 3. API Endpoints (Current — 30+ routes)

### Health & Settings
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check with running job count |
| GET | `/api/v1/settings` | Get current settings |
| POST | `/api/v1/settings` | Update settings |

### Templates
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/templates/scan` | Scan template folder → build template map |
| GET | `/api/v1/templates` | Get template map |
| POST | `/api/v1/template/create` | Create new template from mod folder file |
| POST | `/api/v1/template/apply` | Apply fixed template to all files in a group |
| POST | `/api/v1/template/edit-script` | Apply find/replace edit script to template + group |
| GET | `/api/v1/template/groups` | Get all template groups with mod folders |
| GET | `/api/v1/template/group` | Get details for a specific template group (?name=XXX) |

### Compare
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/compare` | Start comparison job (by project ID) |
| GET | `/api/v1/compare/status` | Get comparison progress (by project_id) |
| GET | `/api/v1/compare/jobs` | Get all running jobs (for global status bar) |
| POST | `/api/v1/compare/stop` | Stop a comparison job |
| POST | `/api/v1/compare/resume` | Resume a stopped/interrupted comparison |

### Results & Browse
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/results` | Get comparison results |
| GET | `/api/v1/browse` | Get output folder tree (max depth 5) |
| GET | `/api/v1/browse/folder` | Get contents of a specific folder |
| GET | `/api/v1/browse/system` | Browse system folders/drives for folder dialog |

### DXF Rendering & Diff
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/dxf/render` | Get all entities for visual rendering (?path=) |
| GET | `/api/v1/dxf/content` | Get raw text content (?path=) |
| POST | `/api/v1/dxf/content` | Save modified DXF content (creates .bak) |
| POST | `/api/v1/diff` | Compare two DXF files, return entity-level differences |

### Projects
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/projects` | List all projects |
| POST | `/api/v1/projects` | Create new project |
| GET | `/api/v1/project` | Get/activate a project (?id=) |
| POST | `/api/v1/project` | Update project (?id=) |
| DELETE | `/api/v1/project` | Delete project (?id=) |
| GET | `/api/v1/project/export` | Export project config as JSON (?id=) |
| POST | `/api/v1/project/import` | Import project from JSON |
| GET | `/api/v1/project/zip-export` | Export project as ZIP (?id=) |
| POST | `/api/v1/project/zip-import` | Import project from ZIP |

### Session & Logs
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/session` | Get saved session state |
| DELETE | `/api/v1/session` | Clear session |
| GET | `/api/v1/log` | Get content of a .log file (?path=) |

## 4. Data Models (Go Structs)

```go
// DXF entity types
type CodePair struct {
    Code  int
    Value string
}

type Entity struct {
    Type    string
    Pairs   []CodePair
    Attribs []Entity
}

type LayerDef struct {
    Name  string
    Color int    // ACI color index
    Flags int
}

type BlockDef struct {
    Name     string
    BaseX    float64
    BaseY    float64
    Entities []Entity
}

type Drawing struct {
    Entities []Entity
    Layers   map[string]*LayerDef
    Blocks   map[string]*BlockDef
}

// DXFContent for comparison
type DXFContent struct {
    Blocks    map[string][][3]float64
    Lines     map[string][][[2][3]float64
    Polylines map[string][][][3]float64
}

// Comparison result
type Result struct {
    FileName    string `json:"file_name"`
    Template    string `json:"template"`
    Status      string `json:"status"`       // "match", "different", "no_template"
    ContentHash string `json:"content_hash,omitempty"`
    ModFolder   string `json:"mod_folder,omitempty"`
}

// Compare job (JobManager)
type CompareJob struct {
    ID             string
    ProjectName    string
    Running        bool
    TotalFiles     int
    ProcessedFiles int
    LogMessages    []string
    Results        []any
    StartTime      time.Time
    ElapsedTime    string
    ETA            string
    Matched        int
    Different      int
    NoTemplate     int
    StopChan       chan struct{}
    SearchFolder   string
    OutputFolder   string
    TemplateFolder string
}

// Project
type Project struct {
    ID             string
    Name           string
    TemplateFolder string
    SearchFolder   string
    OutputFolder   string
    CreatedAt      time.Time
    LastUsed       time.Time
    Recursive      bool
    GroupByContent bool
    MoveFiles      bool
}

// Diff entity (for frontend rendering)
type DiffEntity struct {
    Type       string     // "line", "insert", "lwpolyline", "polyline", "text", "circle", "arc", "point"
    Status     string     // "added", "removed", "modified", "same"
    Coords     []float64
    Coords2D   [][]float64
    BlockName  string
    Layer      string
    Color      int        // ACI color index
    Rotation   float64
    ScaleX     float64
    ScaleY     float64
    HAlign     int
    VAlign     int
    TextHeight float64
    Bulges     []float64
    Closed     bool
    BlockEntities []*DiffEntity
    BlockBaseX    float64
    BlockBaseY    float64
    Attribs    []DiffAttrib
}

type DiffAttrib struct {
    Tag      string
    Text     string
    X, Y     float64
    Height   float64
    Rotation float64
    HAlign   int
    VAlign   int
}
```

## 5. DXF Parser Design (Native Go)

DXF format: alternating lines of (code, value) pairs.
- Odd lines = group code (integer, may have leading spaces)
- Even lines = value (string, float, or int depending on code)

### Parser algorithm:
1. Read file line by line as text (bufio.Scanner)
2. Parse alternating (code, value) pairs — trim whitespace from code, parse as int
3. Group code pairs into entities (entity starts at code=0)
4. Track section boundaries (HEADER, TABLES, BLOCKS, ENTITIES, OBJECTS)
5. For each entity, extract relevant fields based on type
6. INSERT entities with code 66=1 have ATTRIB children until SEQEND
7. Build Drawing struct with Entities, Layers, Blocks

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
| 11 | X coordinate (secondary - LINE end, alignment point) |
| 21 | Y coordinate (secondary) |
| 31 | Z coordinate (secondary) |
| 40 | Radius / Text height |
| 41 | X scale (INSERT) |
| 42 | Y scale (INSERT) / Bulge (LWPOLYLINE) |
| 50 | Rotation angle |
| 62 | ACI color index |
| 66 | Attributes follow flag (INSERT) |
| 70 | Flags (entity-specific, invisible bit for ATTRIB = 0x80) |

## 6. Template Matching Algorithm

1. Scan template folder for `.dxf` files
2. For each file, parse DXF → find INSERT entities → find ATTRIB with tag `$(TEMPLATE)`
3. Build `template_map`: `{template_name: file_path}`
4. For each module file in search folder:
   a. Parse DXF → find `$(TEMPLATE)` attribute value
   b. If found → look up in template_map → compare
   c. If not found → filename prefix matching (longest match wins)
   d. If still no match → status: "no_template"

**Browse page template matching** (frontend): Template derived from folder path in output tree:
- `Output/TEMPLATE_NAME/file.dxf` → template = TEMPLATE_NAME
- `Output/TEMPLATE_NAME/TEMPLATE_NAME_mod1/file.dxf` → template = TEMPLATE_NAME
- `Output/notemplate/file.dxf` → no template

## 7. Comparison Algorithm (port from Python)

### Go implementation:
- `ExtractDXFContent(filePath string, decimals int) (*DXFContent, error)`
  - Parse DXF → extract blocks (INSERT positions, excluding COMPANY/CUSTOMER), lines (sorted endpoints), polylines (normalized vertices)
  - Round all coordinates to N decimals
- `compareDictOfLists(d1, d2)` — set intersection/difference on keys and values
- `compareDictOfListsLines(d1, d2)` — line-specific comparison
- `compareDictOfListsPolylines(d1, d2)` — polyline-specific comparison
- `ContentHash(content *DXFContent) string` — MD5 of JSON-serialized geometry

### Parallel processing (v0.6.0+):
- Phase 1 (parallel, 4 workers): parse, match, compare, hash — read-only
- Phase 2 (sequential): file writes, _modN creation, log generation
- `atomic.Int64` for progress, `atomic.Bool` for cancellation

## 8. _modN Folder Creation

After comparison:
1. Group all "different" files by template_name
2. Within each template group, sub-group by content_hash
3. Each unique content_hash → one `_modN` folder (N = 1, 2, 3, ...)
4. Files with same content_hash go in same `_modN` folder
5. `_modN` folders nested under template folder: `Output/TEMPLATE/TEMPLATE_modN/`
6. Save detailed comparison log per template

## 9. Frontend Structure

### Pages:
1. **Dashboard** — Single-column LOGReport-style card list, inline project settings, "ID — NAME" display, create/import/export/delete projects
2. **Compare** — Folder selection (from active project), start/stop/resume, live progress bar + log streaming, ETA
3. **Browse** — Output tree with auto-expand, single-view DXF with diff highlighting (red=added, orange=removed), template matching from folder path, log viewer, template group workflow
4. **Edit** — Visual CAD renderer (DXFViewer) + raw text toggle, right-click context menu for template creation, save with .bak backup, edit script apply
5. **Settings** — Global application settings

### Components:
- **DXFViewer** — Canvas-based CAD renderer: ACI colors, block geometry, bulge arcs, text alignment, ATTRIB rendering, zoom/pan, grid overlay, diff highlighting
- **Layout** — Nav bar (5 tabs) + always-on GlobalStatusBar (polls /compare/jobs every 2s)
- **FolderBrowser** — System folder browser dialog (Windows drives / Linux root)

### Theme (LOGReport style):
- Primary color: `#008a00` (Valmet green)
- Background: dark theme (`#1a1a1a` / `#0d0d0d`)
- Accent: `#00ff41` (matrix green glow for highlights)
- Font: monospace for technical data, sans-serif for UI
- Icons: Lucide React

## 10. Phased Implementation (All Complete)

### Phase 1: DXF Parser + Core Logic ✅
- Native DXF parser, extractor, comparison engine, template management

### Phase 2: REST API + Server ✅
- HTTP server, 30+ API routes, project store, session persistence, CLI mode

### Phase 3: React Frontend ✅
- Vite + React + TypeScript, 5 pages, 3 components, Zustand store, embedded in Go binary

### Phase 4: Visual Diff + CAD Rendering ✅
- DXFViewer canvas renderer, ACI colors, block geometry, ATTRIB rendering, diff highlighting

### Phase 5: Parallel Processing + Polish ✅
- JobManager, 4-worker pool, global status bar, template group workflow, ZIP import/export, folder browser