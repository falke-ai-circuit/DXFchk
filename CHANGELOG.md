# Changelog

All notable changes to DXFchk are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [v0.2.0] — 2026-07-28

### Added

- **React frontend** (`frontend/`) — full SPA with Vite + TypeScript + Tailwind CSS, Valmet green dark industrial theme matching LOGReport
  - Dashboard page: stats cards (total/matched/different/no template), progress bars, log output
  - Compare page: folder selection, options, start/stop, live progress, log streaming
  - Results page: filterable table with status badges, file names, templates, mod folders
  - Settings page: API settings management
- **Embedded frontend** — `//go:embed frontend_dist` serves built React app from Go binary
- **Decompiled comparison_processor.py** — full Python source reconstructed from .pyc bytecode for reference

### Changed

- **Comparison processor rewritten** to match Python tool exactly:
  - `compare_dict_of_lists` set-based comparison (not just hash matching)
  - `_modN` folders at output root level (e.g., `AA_mod1/`), not nested in template folder
  - Files moved (not copied) from template folder to mod folder — no duplicates
  - Fallback template matching by filename prefix when `$(TEMPLATE)` attr is missing
  - Full detailed comparison logs with entity counts + block/line/polyline breakdown
  - `notemplate_dxfanalyze.log` created in notemplate folder
  - `_ensure_all_folders_have_logs` ensures every output folder has a log file
- **Polylines storage** changed from flat `[][3]float64` to per-polyline `[][][3]float64` matching Python's tuple structure
- **Line endpoint sorting** now includes Z coordinate comparison (matching Python's tuple sort)
- **Server** updated to serve embedded frontend with SPA fallback routing

## [v0.1.0] — 2026-07-28

### Added

- **Native DXF parser** (`internal/dxf/parser.go`) — parses DXF text format, extracts entities (INSERT, LINE, POLYLINE, CIRCLE, ARC, TEXT, etc.) from ENTITY and BLOCKS sections
- **High-level extractor** (`internal/dxf/extractor.go`) — extracts blocks (INSERT positions, excluding COMPANY/CUSTOMER attributes), lines (sorted endpoints), polylines (normalized vertices) from parsed DXF
- **Comparison processor** (`internal/compare/processor.go`) — full ComparisonProcessor ported from Python: template matching, geometry comparison, `_modN` folder creation, content-hash grouping
- **Template management** (`internal/compare/template.go`) — `$(TEMPLATE)` attribute extraction, template map building (template_name → file_path)
- **Test parse utility** (`cmd/test_parse/main.go`) — CLI tool to test DXF parser on sample files
- **Reference Python source** (`reference-python-source/`) — original Python DXF Compare Tool kept for reference (5 .py files, ~2028 lines)
- **Architecture blueprint** (`BLUEPRINT.md`) — full architecture, API spec, data flow, phased implementation plan

### Technical Details

- **Zero external Go dependencies** — native DXF parser uses only Go stdlib
- **Content hashing** — MD5 of JSON-serialized geometry (blocks, lines, polylines) for grouping
- **Coordinate rounding** — all coordinates rounded to N decimal places before comparison
- **`_modN` grouping** — modified files grouped by content hash into numbered subfolders

### Known Limitations

- No REST API yet (Phase 2)
- No frontend yet (Phase 3)
- No visual diff viewer yet (Phase 4)
- No SQLite database yet (Phase 2)
- Parser tested on Valmet Eclipse DXF files only