# DXFchk Design

## System Overview

DXFchk is a single-binary web application: a Go HTTP server with an embedded React frontend. It compares DXF module files against DXF template files for Valmet Eclipse automation systems, detects modifications, and organizes modified files into `_modN` subfolders grouped by content hash.

Version: **v0.6.7**

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      DXFchk Binary                               │
│                                                                  │
│  ┌───────────────┐    ┌────────────────────────────────────────┐ │
│  │  Go Server     │    │  Embedded React Frontend               │ │
│  │  (net/http)    │    │  (embed.FS → frontend_dist)            │ │
│  │                │    │                                        │ │
│  │  REST API      │◄──►│  Dashboard · Compare · Browse · Edit   │ │
│  │  handlers      │    │  Settings · DXFViewer · StatusBar      │ │
│  └───────┬────────┘    └────────────────────────────────────────┘ │
│          │                                                       │
│  ┌───────┴─────────────────────────────────────────────────────┐ │
│  │              Internal Packages                               │ │
│  │  dxf (parser, extractor) · compare (processor, parallel,    │ │
│  │  template) · api (server, handlers, jobs, browse, diff,     │ │
│  │  projects, template_apply, folderbrowser, session, logdxf)  │ │
│  └───────┬─────────────────────────────────────────────────────┘ │
│          │                                                       │
│  ┌───────┴──────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │  Filesystem   │  │  DXF files   │  │  Project store     │     │
│  │  (output)     │  │  (templates, │  │  (~/.dxfchk/       │     │
│  │  (_modN tree) │  │   modules)   │  │   projects.json)   │     │
│  └───────────────┘  └──────────────┘  └────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

## Comparison Flow (Parallel)

```
User selects project (template folder + search folder + output folder)
        │
        ▼
┌──────────────────────────┐
│  Scan template folder     │  ◄── Extract $(TEMPLATE) from each DXF
│  Build template map       │      template_name → file_path
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│  Scan search folder      │  ◄── Find all .dxf module files
│  (recursive)              │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│  PHASE 1: Parallel       │  ◄── 4-worker pool processes files
│  (read-only work)         │      concurrently:
│                           │        • Parse DXF
│                           │        • Extract template name
│                           │        • Compare geometry
│                           │        • Compute content hash
│                           │      Progress callback every file
│                           │      Stop channel for cancellation
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│  PHASE 2: Sequential     │  ◄── Apply decisions in order:
│  (file writes)            │        • match → copy to template folder
│                           │        • different → copy + hash group
│                           │        • no_template → copy to notemplate/
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│  Finalize                │  ◄── Create _modN subfolders
│                           │      Group by content hash
│                           │      Save per-template logs
└──────────────────────────┘
```

### Template Matching

```
DXF module file
    │
    ▼
Parse DXF → extract $(TEMPLATE) attribute from INSERT entities
    │
    ├── $(TEMPLATE) found  ──► match against template map
    │                           → compare geometry
    │                           → if different: hash + group
    │
    └── $(TEMPLATE) not found
            │
            ▼
        Filename prefix matching
        (longest matching template name wins)
            │
            ├── Match found    ──► compare geometry
            └── No match       ──► status: "no_template"
```

**Browse template matching** (frontend): When browsing output files, the template is determined from the folder path, not the filename. The output folder structure is `Output/TEMPLATE_NAME/[_modN]/file.dxf`, so the template name is extracted from the folder hierarchy. This was fixed in v0.6.5 (previously matched from filename which was unreliable).

### DXF Parsing

```
DXF file (text format)
    │
    ├── HEADER section    ──► (parsed, not used)
    ├── TABLES section    ──► LayerDef (name, ACI color, flags)
    ├── BLOCKS section    ──► BlockDef (name, base point, entities)
    ├── ENTITIES section  ──► Entity records with code/value pairs
    │                         INSERT, LINE, LWPOLYLINE, POLYLINE,
    │                         CIRCLE, ARC, TEXT, MTEXT, POINT, ATTRIB
    └── OBJECTS section   ──► (parsed, not used)
```

**Entity types extracted for rendering:**
- **LINE**: start/end coordinates (codes 10/20, 11/21), layer (8), color (62)
- **LWPOLYLINE**: vertices (10/20 pairs), bulges (42), closed flag (70)
- **POLYLINE**: vertices from VERTEX entities, bulges, closed flag
- **INSERT**: block name (2), insert position (10/20), rotation (50), scale (41/42), ATTRIB children
- **TEXT/MTEXT**: text content (1, MTEXT formatting stripped), position, height (40), alignment (72/73)
- **CIRCLE**: center (10/20), radius (40)
- **ARC**: center, radius, start/end angles (50/51) — tessellated to 32 segments
- **POINT**: position (10/20)

**ATTRIB handling:**
- Invisible ATTRIBs skipped (code 70 bit 0x80)
- Structural label markers skipped (LABEL_UP, LABEL_RIGHT, LABEL_DOWN, LABEL_LEFT)
- Coordinate echo tags skipped (tags starting with "0,")
- MTEXT formatting stripped from text values
- Alignment point (code 11/21) used when horizontal/vertical alignment is non-zero

### Parallel Processing Design

```
JobManager (inter-project parallelism)
    │
    ├── Job 1 (project A)  ──► RunComparisonParallel(workers=4)
    │                            │
    │                            ├── Worker 1 ──► processFileParallel()
    │                            ├── Worker 2 ──► processFileParallel()
    │                            ├── Worker 3 ──► processFileParallel()
    │                            └── Worker 4 ──► processFileParallel()
    │
    ├── Job 2 (project B)  ──► RunComparisonParallel(workers=4)
    │                            └── ...
    │
    └── GlobalStatusBar polls /api/v1/compare/jobs every 2s
```

**Two-phase design:**
- Phase 1 (parallel): Read-only work — parse DXF, find template, compare, hash. No file writes, no shared state mutation. 4 workers process files from a channel.
- Phase 2 (sequential): Apply decisions in order — copy files to output, create `_modN` folders, update processor state. Avoids race conditions on filesystem.

**Cancellation:** Each job has a `StopChan`. The progress callback checks the channel each iteration. When stop signal received, `atomic.Bool` flag set, workers exit early.

## Output Structure

```
DXFchk_output/
├── TEMPLATE_NAME_A/              # Template folder
│   ├── module1.dxf               # Matched (identical to template)
│   ├── module2.dxf
│   └── TEMPLATE_NAME_A_mod1/     # _modN subfolder (nested under template)
│       ├── module3.dxf           # Files with same content hash
│       └── module4.dxf
├── TEMPLATE_NAME_A_mod2/         # Another mod variant (different hash)
│   └── module5.dxf
├── TEMPLATE_NAME_B/
│   └── TEMPLATE_NAME_B_mod1/
│       └── module6.dxf
└── notemplate/                   # Files with no matching template
    └── module7.dxf
    └── notemplate_dxfanalyze.log
```

`_modN` folders are nested under the template folder. N = 1, 2, 3, ... assigned by content hash group. Files with identical modifications go in the same `_modN` folder.

## Data Flow

```
DXF files (filesystem)
    │
    ├── template folder ──► template.go ──► template map
    │                                         (name → path)
    ├── search folder   ──► parallel.go ──► for each module (4 workers):
    │                                         extract template
    │                                         compare geometry
    │                                         compute hash
    │
    ▼
ComparisonProcessor (sequential phase)
    │
    ├── "match" files     ──► copy to template folder in output
    ├── "different" files ──► copy to template folder, then group by hash
    │                         → move to _modN subfolder
    └── "no_template"    ──► copy to notemplate/ folder
    │
    ▼
Results
    │
    ├── _modN folders     ──► {output}/{template}/{template}_modN/
    ├── comparison logs   ──► per-template log files
    ├── database records ──► JobManager state (in-memory + session JSON)
    └── session state     ──► ~/.dxfchk/session_<jobID>.json
```

## Component Description

### `internal/dxf/parser.go`

Native DXF text parser. Reads DXF files line-by-line, extracts section boundaries and entity records. No external dependencies. Handles:
- Section detection (HEADER, TABLES, BLOCKS, ENTITIES, OBJECTS)
- Entity parsing (INSERT, LINE, LWPOLYLINE, POLYLINE, CIRCLE, ARC, TEXT, MTEXT, POINT, ATTRIB)
- Attribute extraction (group codes → key-value pairs)
- Coordinate parsing (float64 from string)
- Layer definitions (TABLES → LayerDef with ACI color)
- Block definitions (BLOCKS → BlockDef with base point + entities)
- ATTRIB children following INSERT (when code 66 = 1)

### `internal/dxf/extractor.go`

High-level extraction from parsed DXF. Converts raw entity records into structured content:
- **Blocks**: INSERT positions with attributes, excluding COMPANY/CUSTOMER
- **Lines**: endpoints sorted (min→max) for deterministic comparison
- **Polylines**: vertices normalized (order-independent)
- All coordinates rounded to N decimal places
- `GetTemplateAttribute()` — extracts `$(TEMPLATE)` value from INSERT ATTRIBs

### `internal/compare/template.go`

Template management. Builds the template map by scanning a folder for DXF files and extracting `$(TEMPLATE)` attributes. Provides:
- `BuildTemplateMap(folder string, recursive bool, progressFn) TemplateMap` — template_name → file_path
- Panic recovery on malformed files

### `internal/compare/processor.go`

Core comparison processor. Orchestrates the full comparison workflow:
- `ComparisonProcessor` struct holds template hashes, mod folders, direct copies, detailed logs
- `applyMatch()` — copy matching file to template folder in output
- `applyDifferent()` — copy file, register for hash grouping
- `applyNoTemplate()` — copy to notemplate folder
- `Finalize()` — create `_modN` subfolders, group by content hash, save logs
- Content hashing: MD5 of JSON-serialized geometry (blocks, lines, polylines)

### `internal/compare/parallel.go`

Parallel comparison engine:
- `RunComparisonParallel()` — two-phase parallel processing
- Phase 1: 4-worker pool, channel-based file distribution, atomic progress counter
- `processFileParallel()` — per-file read-only work (parse, match, compare, hash)
- `compareFilesParallel()` — dict-of-lists comparison for blocks, lines, polylines
- Panic recovery per file (malformed DXF files don't crash the worker)
- Cancellation via `atomic.Bool` + progress callback stop signal

### `internal/api/server.go`

HTTP server with Go 1.22+ ServeMux method patterns:
- Embedded frontend via `//go:embed frontend_dist`
- SPA fallback routing for React Router
- CORS headers for development
- 30+ API routes registered

### `internal/api/handlers.go`

Core API handlers:
- Health check (with running job count)
- Settings CRUD
- Template scanning
- Compare start (creates JobManager job, runs in background goroutine)
- Compare status (per-job, fallback to first running)
- Compare stop/resume
- All jobs summary (for global status bar)
- Results

### `internal/api/jobs.go`

JobManager for parallel comparison jobs:
- `JobManager` — map of job ID → CompareJob, RWMutex protected
- `CompareJob` — per-job state (progress, logs, results, stop channel)
- Session persistence to `~/.dxfchk/session_<jobID>.json`
- Log ring buffer (last 500 messages)

### `internal/api/browse.go`

Output folder tree builder:
- `buildTreeNode()` — recursive tree (max depth 5) with template/mod classification
- Template folders vs `_modN` folders vs files
- DXF file counting per node
- Lazy folder loading via `handleBrowseFolder`

### `internal/api/diff.go`

DXF entity diff and rendering:
- `extractEntities()` — parse DXF, convert to DiffEntity list for frontend
- `extractFromDrawing()` — handle all entity types with coordinates, colors, ATTRIBs
- `computeEntityDiff()` — entity-level diff (added/removed/modified)
- `computeBoundingBox()` — for canvas viewport
- `handleCreateTemplate()` — create new template from a mod folder file
- Block entity resolution from BLOCKS section
- ATTRIB filtering (invisible, structural labels, coordinate echoes)

### `internal/api/projects.go`

Project management:
- JSON-based project store at `~/.dxfchk/projects.json`
- CRUD operations (create, get, update, delete)
- Project activation updates server settings
- Slug-based ID generation from project name

### `internal/api/template_apply.go`

Template group workflow ("fix 90 templates instead of 1500 files"):
- `buildTemplateGroups()` — scan output, group by template name with mod folders
- `handleApplyTemplate()` — copy fixed template to template folder + all mod folders
- `handleEditScript()` — apply find/replace operations to template, then apply to group
- Template group detail endpoint

### `internal/api/folderbrowser.go`

System folder browser:
- Windows drive listing (C:-Z:) / Linux root listing
- Directory navigation with parent support
- ZIP export/import of project folder structures

### `internal/api/session.go`

Session state persistence:
- `SessionState` struct with all comparison parameters + progress
- Save/load/clear session to `~/.dxfchk/session.json`
- Elapsed time + ETA calculation
- Per-job session files (`session_<jobID>.json`)

### `internal/api/logdxf.go`

DXF rendering and content APIs:
- `handleDXFRender()` — full entity list for visual CAD rendering with layer colors
- `handleDXFContent()` — raw text content (GET) + save with backup (POST)
- `handleLogContent()` — .log file viewer
- `handleEditScript()` — find/replace edit script application

### Frontend: `frontend/src/components/DXFViewer.tsx`

Canvas-based CAD renderer:
- ACI color index → CSS color mapping (0-255)
- ByLayer color resolution from layer definitions
- Entity rendering: LINE, LWPOLYLINE (with bulge arcs), POLYLINE, INSERT (block geometry at position), TEXT/MTEXT, CIRCLE, ARC, POINT
- ATTRIB rendering with text alignment, rotation, height
- Zoom (wheel), pan (drag), fit-to-view
- Grid overlay toggle
- Diff highlighting: red=added, orange=removed

### Frontend: `frontend/src/components/Layout.tsx`

- Navigation bar with Valmet logo + 5 tabs (Dashboard, Compare, Browse, Edit, Settings)
- Health indicator (polls /health)
- **GlobalStatusBar** — always-on bottom bar, polls /compare/jobs every 2s, shows idle/running/completed state with progress

### Frontend: Pages

- **Dashboard.tsx** — Single-column LOGReport-style card list, inline project settings panel, project display as "ID — NAME", create/import/export/delete, folder browser dialog
- **Compare.tsx** — Folder selection (from active project or settings), start/stop/resume, live progress bar + log streaming, ETA
- **Browse.tsx** — Output tree with auto-expand, single-view DXF with diff highlighting (red=added, orange=removed), template matching from folder path, log viewer, template group workflow
- **Edit.tsx** — Visual CAD renderer (DXFViewer) + raw text toggle, right-click context menu for template creation, save with .bak backup, edit script apply
- **Settings.tsx** — Global settings management

## Theme

Valmet green dark industrial theme (matching LOGReport):
- Primary: `#008a00` (Valmet green)
- Background: `#1a1a1a` / `#0d0d0d` (dark)
- Accent: `#00ff41` (matrix green glow)
- Font: monospace for technical data, sans-serif for UI
- Icons: Lucide React