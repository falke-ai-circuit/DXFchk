# CLAUDE.md — DXFchk

This is the DXFchk project — a DXF Module-Template Comparison Tool for Valmet Eclipse automation systems.

## What It Is

A Go backend + React frontend single-binary application that compares DXF module files against DXF template files, detects modifications, and organizes modified files into `_modN` subfolders grouped by content hash. Rewritten from a Python (PyQt5) tool to Go + React.

**Current version: v0.6.7**

## How to Build

**CRITICAL: Rebuild frontend BEFORE Go binary.** Stale `//go:embed` = stale UI.

```bash
# 1. Build frontend
cd frontend && npm install && npm run build

# 2. Copy to embed directory
cp -r frontend/dist/* internal/api/frontend_dist/

# 3. Build backend
go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk ./cmd/dxfchk/

# Cross-compile for Windows:
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk.exe ./cmd/dxfchk/
```

Or use Makefile: `make build`, `make cross`, `make vet`, `make test`.

## How to Run

```bash
# Web server mode
./dxfchk --port 8643

# CLI mode (headless comparison)
./dxfchk cli -templates <dir> -search <dir> [-output <dir>] [options]
```

Default port: 8643. Open `http://localhost:8643` in browser.

## Project Structure

```
DXFchk/
├── cmd/
│   ├── dxfchk/                   # Main entry point (HTTP server + CLI mode)
│   └── debug_hash/               # Debug utility for content hashing
├── internal/
│   ├── dxf/                      # Native DXF parser
│   │   ├── parser.go             # Text parser, entity/section extraction
│   │   └── extractor.go          # High-level extraction (blocks/lines/polylines)
│   ├── compare/                  # Comparison engine
│   │   ├── processor.go          # ComparisonProcessor, _modN grouping
│   │   ├── parallel.go           # 4-worker parallel processing
│   │   └── template.go           # Template map building, $(TEMPLATE) extraction
│   └── api/                      # HTTP server + REST API
│       ├── server.go             # Server, routes, embedded frontend
│       ├── handlers.go           # Core API handlers
│       ├── jobs.go               # JobManager for parallel jobs
│       ├── browse.go             # Output folder tree builder
│       ├── diff.go               # DXF entity diff, rendering, template create
│       ├── projects.go           # Project CRUD (JSON store)
│       ├── template_apply.go     # Template groups, apply, edit scripts
│       ├── folderbrowser.go      # System folder browser, ZIP export/import
│       ├── session.go            # Session state persistence
│       ├── logdxf.go             # DXF render, raw content, log viewer
│       └── frontend_dist/        # Embedded built frontend
├── frontend/                     # React frontend (Vite + TypeScript)
│   ├── src/
│   │   ├── pages/                # Dashboard, Compare, Browse, Edit, Settings
│   │   ├── components/           # DXFViewer, Layout, FolderBrowser
│   │   ├── api.ts, store.ts, App.tsx, main.tsx
│   │   └── styles/theme.css      # Valmet green theme
│   └── public/                   # Valmet logo, favicon
├── reference-python-source/      # Original Python source (reference only)
├── .github/                      # CI workflows + agent briefs
├── BLUEPRINT.md, DESIGN.md, CHANGELOG.md, ROADMAP.md
├── CONTRIBUTING.md, CLAUDE.md, AGENTS.md
└── Makefile, go.mod, LICENSE
```

## Key Facts

- **Module path:** `github.com/falke-ai-circuit/DXFchk`
- **Go version:** 1.22 (tested with Go 1.23.12 at `/opt/data/go/bin/go1.23.12`)
- **Dependencies:** Zero external Go dependencies — stdlib only
- **Parser:** Native DXF text parser (no ezdxf equivalent needed)
- **Comparison:** `$(TEMPLATE)` extraction → template matching → geometry comparison → content hashing → `_modN` grouping
- **Content hash:** MD5 of JSON-serialized geometry (blocks, lines, polylines)
- **Parallel processing:** 4-worker pool (inter-project JobManager + intra-comparison workers)
- **Template matching:** `$(TEMPLATE)` attribute with filename prefix fallback; browse page uses folder path
- **Embedded frontend:** `//go:embed frontend_dist` in `internal/api/server.go`
- **Project store:** `~/.dxfchk/projects.json`
- **Session state:** `~/.dxfchk/session_<jobID>.json`
- **Reference:** `reference-python-source/` contains original Python tool

## Key Pitfalls (Learned)

- **Always rebuild frontend before Go binary** — `//go:embed` caches at build time
- **Go 1.22 ServeMux trailing-slash shadows routes** — use method patterns without trailing slashes
- **ATTRIBs render in world coordinates** — not inside INSERT transform
- **Invisible ATTRIB flag** = code 70 bit 0x80
- **Block names NOT visible in CAD** — BricsCAD doesn't show them, don't render as text
- **collectEntityPairs can infinite loop** on SEQEND-terminated entities (fixed v0.6.3)
- **Mod folder log path double-nesting** — logs go in template folder, not inside _modN subfolder (fixed v0.6.3)
- **Template matching from filename was unreliable** — use folder path for browse page (fixed v0.6.5)