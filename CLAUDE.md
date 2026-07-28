# CLAUDE.md — DXFchk

This is the DXFchk project — a DXF Module-Template Comparison Tool for Valmet Eclipse automation systems.

## What It Is

A Go backend + React frontend single-binary application that compares DXF module files against DXF template files, detects modifications, and organizes modified files into `_modN` subfolders grouped by content hash. Rewritten from a Python (PyQt5) tool to Go + React.

## How to Build

```bash
make build          # Build backend binary
make cross          # Cross-compile for all platforms
```

Or manually:
```bash
go build -ldflags "-X main.version=v0.1.0" -o dxfchk ./cmd/dxfchk/
```

## How to Run

```bash
# Start server (when API is implemented — Phase 2)
./dxfchk --port 8643 --db-path dxfchk-data

# Test parser on a DXF file
go run ./cmd/test_parse/ <path-to-dxf-file>
```

## Project Structure

```
DXFchk/
├── cmd/
│   ├── dxfchk/              # Main entry point — HTTP server (planned, Phase 2)
│   └── test_parse/          # Parser test utility
├── internal/
│   ├── dxf/                 # Native DXF parser (parser.go, extractor.go)
│   ├── compare/             # Comparison engine (processor.go, template.go)
│   ├── api/                 # HTTP handlers (planned)
│   ├── db/                  # SQLite database (planned)
│   └── config/              # App configuration (planned)
├── web/                     # React frontend (planned, Phase 3)
│   └── src/
│       ├── components/      # React components
│       ├── api/             # API client
│       └── styles/          # Valmet green theme
├── reference-python-source/  # Original Python source (not built, reference only)
├── .github/
│   ├── workflows/           # CI (build.yml)
│   └── agents/              # Agent briefs (ANALYST, ARCHITECT, CODER, REVIEWER)
├── AGENTS.md                # Agent delegation rules
├── CLAUDE.md                # This file
├── BLUEPRINT.md             # Architecture blueprint
├── ROADMAP.md               # Phase overview + timeline
├── CHANGELOG.md             # Release history
├── CONTRIBUTING.md          # PR process + conventions
├── README.md                # Project overview
├── LICENSE                  # MIT
├── Makefile                 # Build, test, vet, cross-compile
└── .gitignore
```

## Key Facts

- **Module path:** `github.com/falke-ai-circuit/DXFchk`
- **Go version:** 1.22 (tested with Go 1.23.12)
- **Dependencies:** Zero external Go dependencies — stdlib only
- **Parser:** Native DXF text parser (no ezdxf equivalent needed)
- **Comparison:** `$(TEMPLATE)` extraction → template matching → geometry comparison → content hashing → `_modN` grouping
- **Content hash:** MD5 of JSON-serialized geometry (blocks, lines, polylines)
- **Current version:** `v0.1.0` (parser + comparison engine complete)
- **Next phase:** Phase 2 — REST API + SQLite server
- **Reference:** `reference-python-source/` contains original Python tool (5 .py files, ~2028 lines)