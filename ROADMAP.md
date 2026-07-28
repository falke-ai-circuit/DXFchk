# ROADMAP — DXFchk v0.1.0

## Phase Overview

| Phase | Scope | Deliverable | Status |
|-------|-------|-------------|--------|
| **1** | DXF Parser + Core Logic | Native parser, extractor, comparison engine | ✅ Complete |
| **2** | REST API + Server | HTTP server, handlers, SQLite database | ⏳ Pending |
| **3** | React Frontend | Vite + React + TypeScript UI, embedded in Go binary | ⏳ Pending |
| **4** | Visual Diff | DXF entity rendering, difference highlighting | ⏳ Pending |
| **5** | Export + Polish | ZIP export, settings persistence, error handling | ⏳ Pending |

---

## Phase 1 — DXF Parser + Core Logic ✅

### 1.1: Native DXF Parser
- `internal/dxf/parser.go` — parse DXF text format, extract sections and entities
- `internal/dxf/extractor.go` — extract blocks (INSERT positions), lines (sorted endpoints), polylines (normalized vertices)
- Coordinate rounding to N decimal places
- Zero external dependencies (Go stdlib only)
- ✅ Complete

### 1.2: Comparison Engine
- `internal/compare/template.go` — `$(TEMPLATE)` extraction, template map building
- `internal/compare/processor.go` — full ComparisonProcessor ported from Python
- Content hashing (MD5 of JSON-serialized geometry)
- `_modN` folder creation and file grouping
- ✅ Complete

### 1.3: Test Utility
- `cmd/test_parse/main.go` — CLI tool to test parser on sample DXF files
- ✅ Complete

## Phase 2 — REST API + Server ⏳

### 2.1: HTTP Server
- `cmd/dxfchk/main.go` — main entry point with flags (--port, --db-path, --log-level)
- `internal/api/server.go` — HTTP server, routing, middleware (CORS, logging)
- `internal/api/handlers.go` — REST API handlers for all endpoints
- Health check endpoint

### 2.2: SQLite Database
- `internal/db/database.go` — SQLite connection + migrations
- `internal/db/models.go` — data models (Module, Template, Settings)
- Store comparison results, template maps, settings

### 2.3: API Endpoints
- `GET /api/v1/health` — health check
- `GET /api/v1/settings` — get current settings
- `POST /api/v1/settings` — update settings
- `POST /api/v1/templates/scan` — scan template folder
- `GET /api/v1/templates` — get template map
- `POST /api/v1/compare` — run comparison
- `GET /api/v1/compare/status` — comparison progress
- `GET /api/v1/results` — comparison results
- `GET /api/v1/results/{template}/files` — files in template folder
- `GET /api/v1/results/{template}/log` — comparison log

## Phase 3 — React Frontend ⏳

### 3.1: Project Setup
- `web/` — Vite + React + TypeScript project
- Valmet green theme (#008a00 primary, #1a1a1a dark background, #00ff41 accent)
- Lucide React icons
- API client (`web/src/api/client.ts`)

### 3.2: Components
- `Layout.tsx` — main layout (sidebar + content)
- `FolderSelect.tsx` — template/search/output folder selection + options
- `ComparisonRun.tsx` — start comparison, progress bar, live log feed
- `ResultsView.tsx` — tree view: templates → `_modN` subfolders → files
- `LogView.tsx` — detailed comparison logs per template

### 3.3: Build + Embed
- `embed.go` — `//go:embed` directive for `web/dist/`
- Build frontend, embed in Go binary
- Single-binary deployment

## Phase 4 — Visual Diff ⏳

### 4.1: DXF Entity Rendering
- Canvas/SVG rendering of DXF geometry (blocks, lines, polylines)
- Side-by-side view: template vs module
- Zoom, pan, entity selection

### 4.2: Difference Highlighting
- Color-coded overlay: green=common, red=only in module, blue=only in template
- Entity-level diff (added/removed/modified)
- Coordinate-level diff for modified entities

## Phase 5 — Export + Polish ⏳

### 5.1: Export
- ZIP export of results (modified files organized by `_modN`)
- Export comparison log as PDF/CSV
- Export summary report

### 5.2: Polish
- Settings persistence (SQLite)
- Error handling and edge cases
- Progress reporting (SSE or polling)
- Keyboard shortcuts
- Responsive design

---

## Timeline

| Phase | Est. Time | Status |
|-------|-----------|--------|
| 1 | Done | ✅ Complete |
| 2 | 1-2 turns | ⏳ Pending |
| 3 | 2-3 turns | ⏳ Pending |
| 4 | 1-2 turns | ⏳ Pending |
| 5 | 1 turn | ⏳ Pending |

**v0.1.0 delivered: native DXF parser, comparison engine, test utility. Ready for Phase 2 (REST API).**