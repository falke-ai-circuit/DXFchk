# ROADMAP — DXFchk v0.6.7

## Phase Overview

| Phase | Scope | Deliverable | Status |
|-------|-------|-------------|--------|
| **1** | DXF Parser + Core Logic | Native parser, extractor, comparison engine | ✅ Complete |
| **2** | REST API + Server | HTTP server, handlers, project store | ✅ Complete |
| **3** | React Frontend | Vite + React + TypeScript UI, embedded in Go binary | ✅ Complete |
| **4** | Visual Diff + CAD Rendering | DXF entity rendering, difference highlighting | ✅ Complete |
| **5** | Parallel Processing + Polish | JobManager, 4-worker pool, global status bar, template workflow | ✅ Complete |

---

## Phase 1 — DXF Parser + Core Logic ✅

- `internal/dxf/parser.go` — native DXF text parser, entity/section extraction, layer/block definitions
- `internal/dxf/extractor.go` — blocks (INSERT positions), lines (sorted endpoints), polylines (normalized vertices)
- `internal/compare/template.go` — `$(TEMPLATE)` extraction, template map building
- `internal/compare/processor.go` — full ComparisonProcessor ported from Python
- Content hashing (MD5 of JSON-serialized geometry)
- `_modN` folder creation and file grouping
- `cmd/debug_hash/` — debug utility

## Phase 2 — REST API + Server ✅

- `cmd/dxfchk/main.go` — HTTP server + CLI mode
- `internal/api/server.go` — Go 1.22+ ServeMux with method patterns, 30+ routes
- `internal/api/handlers.go` — health, settings, compare, results, session
- `internal/api/projects.go` — project CRUD (JSON store at `~/.dxfchk/projects.json`)
- `internal/api/session.go` — session state persistence
- CLI mode for headless comparison

## Phase 3 — React Frontend ✅

- `frontend/` — Vite + React + TypeScript
- Pages: Dashboard, Compare, Browse, Edit, Settings
- Components: DXFViewer, Layout (with GlobalStatusBar), FolderBrowser
- Valmet green dark industrial theme (#008a00)
- Embedded in Go binary via `//go:embed frontend_dist`
- `frontend/src/store.ts` — Zustand state management

## Phase 4 — Visual Diff + CAD Rendering ✅

- `frontend/src/components/DXFViewer.tsx` — canvas-based CAD renderer
- ACI color index support (0-255), ByLayer resolution
- Entity rendering: LINE, LWPOLYLINE (bulge arcs), POLYLINE, INSERT (block geometry), TEXT/MTEXT, CIRCLE, ARC, POINT
- ATTRIB rendering with text alignment, rotation, height
- Diff highlighting: red=added, orange=removed (single-view, v0.6.6)
- `internal/api/diff.go` — entity extraction, diff computation, bounding box
- Zoom, pan, grid overlay

## Phase 5 — Parallel Processing + Polish ✅

- `internal/api/jobs.go` — JobManager for multiple concurrent comparison jobs
- `internal/compare/parallel.go` — 4-worker pool, two-phase (parallel read / sequential write)
- GlobalStatusBar — always-on bottom bar, polls /compare/jobs every 2s
- Template group workflow ("fix 90 templates instead of 1500 files")
- Template apply, edit scripts, template creation from mod files
- Compare stop/resume with session persistence
- ZIP export/import for projects
- System folder browser dialog

---

## Future Considerations

- Per-group re-comparison (only re-run comparison for one template group)
- Batch template editing (apply edit scripts across multiple groups)
- PDF/CSV export of comparison reports
- Keyboard shortcuts
- Responsive design for tablet/mobile