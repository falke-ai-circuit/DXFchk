# DXFchk

**DXF Module-Template Comparison Tool for Valmet Eclipse automation systems.**

Version: **v0.1.0**

DXFchk compares DXF module files against DXF template files, detects modifications, and organizes modified files into `_modN` subfolders grouped by content hash. It provides a web-based UI for running comparisons and viewing results.

## Architecture

DXFchk is a Go backend + React frontend single-binary application. The React frontend is embedded into the Go binary at build time.

| Layer | Tech | Location |
|-------|------|----------|
| Backend | Go 1.22+ | `cmd/`, `internal/` |
| Frontend | React 18 + TypeScript + Vite | `web/src/` (not yet built) |
| Embedded UI | Go `embed.FS` | `embed.go` → `web/dist/` |
| Reference Python | PyQt5 (historical) | `reference-python-source/` (not built, kept for reference) |

### Backend Packages (`internal/`)

| Package | Purpose |
|---------|---------|
| `internal/dxf` | Native DXF text parser — blocks (INSERT positions), lines (sorted endpoints), polylines (normalized vertices) |
| `internal/compare` | Comparison engine — template matching, geometry comparison, content hashing, `_modN` folder grouping |
| `internal/api` | HTTP handlers — REST API (planned, Phase 2) |
| `internal/db` | SQLite database — modules, templates, settings (planned, Phase 2) |
| `internal/config` | App configuration (planned, Phase 2) |

### Frontend (`web/src/`) — Planned

| Component | Purpose |
|-----------|---------|
| `Layout.tsx` | Main layout (sidebar + content) |
| `FolderSelect.tsx` | Template/search/output folder selection |
| `ComparisonRun.tsx` | Run comparison UI (progress, log) |
| `ResultsView.tsx` | Results tree (templates → `_modN` → files) |
| `DiffViewer.tsx` | Visual diff (template vs module) |
| `LogView.tsx` | Comparison log viewer |

## Build

### Prerequisites

- Go 1.22+ (tested with Go 1.23.12)
- Node.js 18+ and npm (for frontend build — not yet needed, frontend not built)

### Build steps

```bash
# 1. Build frontend (when available)
# cd web && npm install && npm run build

# 2. Build backend with version injection
go build -ldflags "-X main.version=v0.1.0" -o dxfchk ./cmd/dxfchk/

# Or build for Windows:
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=v0.1.0" -o dxfchk.exe ./cmd/dxfchk/
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
./dxfchk --port 8643 --db-path dxfchk-data
```

The binary serves the React UI and REST API on the same port. Open `http://localhost:8643` in a browser.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 8643 | HTTP server port |
| `--db-path` | dxfchk-data | Data directory |
| `--log-level` | info | debug, info, warn, error |
| `--version` | | Print version and exit |

## REST API

Base URL: `http://localhost:8643/api/v1`

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/settings` | GET, POST | Get/update settings (folders, options) |
| `/templates/scan` | POST | Scan template folder → build template map |
| `/templates` | GET | Get template map (template_name → file_path) |
| `/compare` | POST | Run comparison |
| `/compare/status` | GET | Get comparison progress |
| `/results` | GET | Get comparison results |
| `/results/{template}/files` | GET | Get files in a template folder |
| `/results/{template}/log` | GET | Get comparison log for template |

## Comparison Flow

1. **Template scan** — scan template folder, build `template_name → file_path` map from `$(TEMPLATE)` attributes
2. **Module scan** — scan search folder for DXF module files
3. **Template extraction** — extract `$(TEMPLATE)` attribute from each module to identify its template
4. **Geometry comparison** — compare module geometry (blocks/lines/polylines) against template geometry
5. **Content hashing** — MD5 hash of JSON-serialized geometry for grouping
6. **`_modN` grouping** — group modified files by content hash, create `_modN` subfolders
7. **Log generation** — save detailed comparison log per template

## Development

### Run tests

```bash
# Go tests
go test ./internal/...

# Test parser on sample DXF files
go run ./cmd/test_parse/ <dxf-file>
```

### Project structure

```
DXFchk/
├── cmd/
│   ├── dxfchk/              # Main entry point (HTTP server — planned)
│   └── test_parse/          # Parser test utility
├── internal/               # Go backend packages
│   ├── dxf/                 # Native DXF parser (parser.go, extractor.go)
│   ├── compare/             # Comparison engine (processor.go, template.go)
│   ├── api/                 # HTTP handlers (planned)
│   ├── db/                  # SQLite database (planned)
│   └── config/              # App configuration (planned)
├── web/                     # React frontend (planned)
│   └── src/
│       ├── components/      # React components
│       ├── api/             # API client functions
│       └── styles/          # Valmet green theme (#008a00)
├── reference-python-source/  # Original Python source (not built)
├── BLUEPRINT.md             # Architecture blueprint
├── go.mod                   # Go module: github.com/falke-ai-circuit/DXFchk
├── Makefile                 # Build, test, vet, cross, clean
└── .github/                 # CI workflows + agent briefs
```

## License

MIT — Falke AI Circuit.