# DXFchk

**DXF Module-Template Comparison Tool for Valmet Eclipse automation systems.**

Version: **v0.6.7**

DXFchk compares DXF module files against DXF template files, detects modifications, and organizes modified files into `_modN` subfolders grouped by content hash. It provides a web-based UI with visual CAD rendering, diff highlighting, and parallel comparison processing.

## Features (v0.6.7)

- **Dashboard** — Single-column LOGReport-style layout, inline project settings, project display as "ID — NAME", create/import/export/delete projects
- **Compare** — Parallel comparison with progress tracking, ETA, live log streaming, stop/resume support
- **Browse** — Single-view DXF rendering with diff highlighting (red=added, orange=removed), template matching from folder path, auto-expand tree, nested `_modN` folder navigation
- **Edit** — Visual CAD renderer (DXFViewer) with raw text toggle, right-click context menu for template creation, save with backup, edit script apply to template groups
- **Layout** — Always-on global status bar (idle/running/completed), Valmet green branding (#008a00)
- **Template management** — Template groups workflow ("fix 90 templates instead of 1500 files"), template apply to groups, edit scripts with find/replace
- **Project management** — JSON import/export, ZIP import/export, folder browser dialog, persistent project store
- **CLI mode** — Headless comparison without web UI (`dxfchk cli -templates <dir> -search <dir>`)

## Architecture

DXFchk is a Go backend + React frontend single-binary application. The React frontend is embedded into the Go binary at build time via `//go:embed`.

- **Backend**: Go 1.22+ (tested with Go 1.23.12) — `cmd/`, `internal/`
- **Frontend**: React 18 + TypeScript + Vite — `frontend/`
- **Embedded UI**: Go `embed.FS` — `internal/api/frontend_dist/`
- **Reference Python**: PyQt5 (historical) — `reference-python-source/` (not built, kept for reference)

### Backend Packages (`internal/`)

- `internal/dxf` — Native DXF text parser: entity parsing (INSERT, LINE, LWPOLYLINE, POLYLINE, CIRCLE, ARC, TEXT, MTEXT, ATTRIB), block definitions, layer colors, ATTRIB extraction with invisible flag filtering
- `internal/compare` — Comparison engine: template matching, geometry comparison (dict-of-lists), content hashing (MD5), `_modN` folder grouping, parallel processing (4-worker pool)
- `internal/api` — HTTP server + REST API: all handlers, embedded frontend serving, project store, session state, job manager

### Frontend (`frontend/src/`)

- `pages/Dashboard.tsx` — Project list, create/edit/delete, inline settings panel, stats overview
- `pages/Compare.tsx` — Folder selection, start/stop/resume comparison, live progress + log
- `pages/Browse.tsx` — Output tree with auto-expand, single-view DXF with diff highlighting, template matching from folder path, log viewer
- `pages/Edit.tsx` — Visual CAD renderer + raw text toggle, right-click context menu for template creation, save with backup
- `pages/Settings.tsx` — Global application settings
- `components/DXFViewer.tsx` — Canvas-based CAD renderer with ACI colors, block geometry, bulge arcs, text alignment, zoom/pan, ATTRIB rendering
- `components/Layout.tsx` — Nav bar + always-on global status bar
- `components/FolderBrowser.tsx` — System folder browser dialog

## Build

### Prerequisites

- Go 1.22+ (tested with Go 1.23.12)
- Node.js 18+ and npm

### Build steps

```bash
# 1. Build frontend (MUST run before Go build — stale embed = stale UI)
cd frontend && npm install && npm run build

# 2. Copy built frontend to embed directory
cp -r frontend/dist/* internal/api/frontend_dist/

# 3. Build backend
go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk ./cmd/dxfchk/

# Cross-compile for Windows:
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk.exe ./cmd/dxfchk/
```

Or use the Makefile:

```bash
make build      # Build backend
make vet        # Run go vet
make test       # Run tests
make cross      # Cross-compile for all platforms
```

### Run

```bash
# Web server mode
./dxfchk --port 8643

# CLI mode (headless comparison)
./dxfchk cli -templates <dir> -search <dir> [-output <dir>] [options]
```

The binary serves the React UI and REST API on the same port. Open `http://localhost:8643` in a browser.

### Flags (server mode)

- `--port` — HTTP server port (default: 8643)

### Flags (CLI mode)

- `-templates` — Template folder containing DXF templates (required)
- `-search` — Search folder containing DXF modules to compare (required)
- `-output` — Output folder for results (default: `<search>/DXFchk_output`)
- `-recursive` — Search subdirectories (default: true)
- `-group` — Group different files by content hash into `_modN` folders (default: true)
- `-verbose` — Print detailed logs
- `-json` — Output results as JSON

## REST API

Base URL: `http://localhost:8643/api/v1`

### Health & Settings

- `GET /api/v1/health` — Health check with running job count
- `GET /api/v1/settings` — Get current settings (folders, options)
- `POST /api/v1/settings` — Update settings

### Templates

- `POST /api/v1/templates/scan` — Scan template folder → build template map
- `GET /api/v1/templates` — Get template map
- `POST /api/v1/template/create` — Create new template from a mod folder file
- `POST /api/v1/template/apply` — Apply fixed template to all files in a group
- `POST /api/v1/template/edit-script` — Apply find/replace edit script to template + group
- `GET /api/v1/template/groups` — Get all template groups with mod folders
- `GET /api/v1/template/group?name=XXX` — Get details for a specific template group

### Compare

- `POST /api/v1/compare` — Start comparison job (supports parallel jobs by project ID)
- `GET /api/v1/compare/status` — Get comparison progress for a job
- `GET /api/v1/compare/jobs` — Get all running jobs (for global status bar)
- `POST /api/v1/compare/stop` — Stop a comparison job
- `POST /api/v1/compare/resume` — Resume a stopped/interrupted comparison

### Results & Browse

- `GET /api/v1/results` — Get comparison results
- `GET /api/v1/browse?path=<dir>` — Get output folder tree (max depth 5)
- `GET /api/v1/browse/folder?path=<dir>` — Get contents of a specific folder
- `GET /api/v1/browse/system?path=<dir>` — Browse system folders/drives for folder dialog

### DXF Rendering & Diff

- `GET /api/v1/dxf/render?path=<file>` — Get all entities from a DXF file for visual rendering
- `GET /api/v1/dxf/content?path=<file>` — Get raw text content of a DXF file
- `POST /api/v1/dxf/content` — Save modified DXF content (creates .bak backup)
- `POST /api/v1/diff` — Compare two DXF files, return entity-level differences

### Projects

- `GET /api/v1/projects` — List all projects
- `POST /api/v1/projects` — Create new project
- `GET /api/v1/project?id=<id>` — Get/activate a project
- `POST /api/v1/project?id=<id>` — Update project
- `DELETE /api/v1/project?id=<id>` — Delete project
- `GET /api/v1/project/export?id=<id>` — Export project config as JSON
- `POST /api/v1/project/import` — Import project from JSON
- `GET /api/v1/project/zip-export?id=<id>` — Export project folder structure as ZIP
- `POST /api/v1/project/zip-import` — Import project from ZIP

### Session & Logs

- `GET /api/v1/session` — Get saved session state
- `DELETE /api/v1/session` — Clear session
- `GET /api/v1/log?path=<file>` — Get content of a .log file

## Comparison Flow

1. **Template scan** — Scan template folder, build `template_name → file_path` map from `$(TEMPLATE)` attributes
2. **Module scan** — Scan search folder for DXF module files (recursive)
3. **Template matching** — Extract `$(TEMPLATE)` attribute from each module; fallback to filename prefix matching
4. **Parallel processing** — 4-worker pool processes files concurrently (parse, compare, hash)
5. **Sequential finalization** — File writes and `_modN` folder creation done sequentially
6. **Geometry comparison** — Compare module geometry (blocks/lines/polylines) against template using set-based dict comparison
7. **Content hashing** — MD5 hash of JSON-serialized geometry for grouping
8. **`_modN` grouping** — Modified files grouped by content hash into `_modN` subfolders under template folders
9. **Log generation** — Detailed comparison log per template

## Output Structure

```
DXFchk_output/
├── TEMPLATE_NAME_A/              # Template folder (matched files here)
│   ├── module1.dxf               # Matched (identical to template)
│   ├── module2.dxf
│   └── TEMPLATE_NAME_A_mod1/     # _modN subfolder (files with same modification)
│       ├── module3.dxf
│       └── module4.dxf
├── TEMPLATE_NAME_B/
│   └── TEMPLATE_NAME_B_mod1/
│       └── module5.dxf
└── notemplate/                   # Files with no matching template
    └── module6.dxf
```

## Development

### Run tests

```bash
# Go tests
go test ./internal/...

# Parser test utility
go run ./cmd/test_parse/ <dxf-file>
```

### Project structure

```
DXFchk/
├── cmd/
│   ├── dxfchk/                   # Main entry point (HTTP server + CLI mode)
│   └── debug_hash/               # Debug utility for content hashing
├── internal/
│   ├── dxf/                      # Native DXF parser
│   │   ├── parser.go             # Text parser, entity extraction
│   │   └── extractor.go          # High-level extraction (blocks/lines/polylines)
│   ├── compare/                  # Comparison engine
│   │   ├── processor.go          # ComparisonProcessor, _modN grouping
│   │   ├── parallel.go           # 4-worker parallel processing
│   │   └── template.go           # Template map building, $(TEMPLATE) extraction
│   └── api/                      # HTTP server + REST API
│       ├── server.go             # Server, routes, embedded frontend
│       ├── handlers.go           # Core API handlers (health, settings, compare, results)
│       ├── jobs.go               # JobManager for parallel comparison jobs
│       ├── browse.go             # Output folder tree builder
│       ├── diff.go               # DXF entity diff, visual rendering data
│       ├── projects.go           # Project CRUD (JSON store)
│       ├── template_apply.go     # Template groups, apply, edit scripts
│       ├── folderbrowser.go      # System folder browser, ZIP export/import
│       ├── session.go            # Session state persistence
│       ├── logdxf.go             # DXF render, raw content, log viewer
│       └── frontend_dist/        # Embedded built frontend (from frontend/dist/)
├── frontend/                     # React frontend (Vite + TypeScript)
│   ├── src/
│   │   ├── App.tsx               # Router setup
│   │   ├── api.ts                # API client
│   │   ├── store.ts              # Zustand store
│   │   ├── pages/                # Dashboard, Compare, Browse, Edit, Settings
│   │   ├── components/           # DXFViewer, Layout, FolderBrowser
│   │   └── styles/theme.css      # Valmet green theme
│   └── public/                   # Valmet logo, favicon
├── reference-python-source/      # Original Python source (not built, reference only)
├── .github/                      # CI workflows + agent briefs
├── BLUEPRINT.md                  # Architecture blueprint
├── DESIGN.md                     # Design documentation
├── CHANGELOG.md                  # Release history
├── ROADMAP.md                    # Phase overview + timeline
├── CLAUDE.md                     # Agent knowledge file
├── CONTRIBUTING.md               # PR process + conventions
└── Makefile                      # Build, test, vet, cross, clean
```

## Key Technical Details

- **Zero external Go dependencies** — parser uses only Go stdlib
- **Content hashing** — MD5 of JSON-serialized geometry (blocks, lines, polylines)
- **Coordinate rounding** — all coordinates rounded to N decimal places before comparison
- **Parallel processing** — 4-worker pool for file processing (inter-project JobManager + intra-comparison worker pool)
- **Template matching** — `$(TEMPLATE)` attribute extraction with filename prefix fallback
- **ATTRIB rendering** — invisible flag (code 70 bit 0x80) filtered, structural label markers skipped, MTEXT formatting stripped
- **Block rendering** — INSERT entities resolve block definitions from BLOCKS section, render block geometry at insert position with rotation/scale
- **Session persistence** — comparison state saved to `~/.dxfchk/session_<jobID>.json` for resume support
- **Project store** — `~/.dxfchk/projects.json`

## License

MIT — Falke AI Circuit.