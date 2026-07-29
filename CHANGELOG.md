# Changelog

All notable changes to DXFchk are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [v0.6.7] — 2026-07-29

### Changed

- **Dashboard project display** — projects shown as "ID — NAME" format (LOGReport style) instead of just name
- Dashboard layout refined to single-column LOGReport style with inline settings

## [v0.6.6] — 2026-07-29

### Changed

- **Browse single-view diff** — replaced side-by-side comparison with single-view DXF rendering
- Diff highlighting: red=added entities, orange=removed entities (overlay on single DXF view)
- Simplified browse UX — no more split panel, cleaner diff visualization

## [v0.6.5] — 2026-07-29

### Added

- **LOGReport-style dashboard** — single-column layout with inline project settings panel
- **Browse template fix** — template matching now uses folder path instead of filename (Output/TEMPLATE_NAME/[_modN]/file.dxf → template derived from folder hierarchy)
- **Edit right-click context menu** — right-click on DXF file in Edit page opens context menu for template creation
- **Always-on global status bar** — bottom status bar visible on all pages, shows idle/running/completed state with progress

### Fixed

- Template matching from filename was unreliable — fixed by deriving template name from folder path in output tree

## [v0.6.4] — 2026-07-29

### Added

- **Inline project settings panel** — edit project folders/options directly from dashboard without modal
- **Browse tree auto-expand** — output tree automatically expands to show template folders and `_modN` subfolders

## [v0.6.3] — 2026-07-29

### Fixed

- **collectEntityPairs infinite loop** — fixed infinite loop when parsing SEQEND-terminated INSERT entities with ATTRIBs. The parser could loop forever on certain DXF files with complex block attribute structures.
- **Mod folder log path double-nesting** — comparison logs for `_modN` folders were being written to double-nested paths (e.g., `Output/TEMPLATE/TEMPLATE_mod1/TEMPLATE_mod1_log.txt` instead of `Output/TEMPLATE/TEMPLATE_mod1_log.txt`)

## [v0.6.2] — 2026-07-29

### Added

- **Dashboard click opens edit** — clicking a project card on the dashboard opens the inline edit panel (E2E verified)

## [v0.6.1] — 2026-07-29

### Added

- **Parallel file processing** — 4-worker pool processes files within each comparison job concurrently (Phase 1: parallel read-only work, Phase 2: sequential file writes)
- `RunComparisonParallel()` function in `internal/compare/parallel.go`
- Channel-based work distribution, atomic progress counter, panic recovery per file

## [v0.6.0] — 2026-07-29

### Added

- **Parallel comparisons** — JobManager supports multiple concurrent comparison jobs by project ID
- **Nested _modN browsing** — output tree shows nested `_modN` subfolders under template folders (max depth 5)
- **Global status bar** — always-on status bar polling all running jobs every 2 seconds
- `GET /api/v1/compare/jobs` endpoint for all running job summaries
- `CompareJob` struct with per-job state (progress, logs, results, stop channel)
- Session persistence per job (`~/.dxfchk/session_<jobID>.json`)
- Compare resume support (`POST /api/v1/compare/resume`)

## [v0.5.6] — 2026-07-28

### Added

- Global status bar (initial implementation)
- Faster server startup

### Removed

- Block name text rendering from INSERT entities (block names not visible in CAD)

## [v0.5.5] — 2026-07-28

### Removed

- Block name text rendering from INSERT entities — BricsCAD doesn't show block names, so rendering them was visual noise

## [v0.5.4] — 2026-07-28

### Added

- **Invisible ATTRIB filtering** — ATTRIBs with invisible flag (code 70 bit 0x80) are now filtered out
- Coordinate noise reduction — coordinate echo ATTRIBs skipped

### Fixed

- Browse render fix for files with no diff data

## [v0.5.3] — 2026-07-28

### Fixed

- **ATTRIB world coordinates** — ATTRIBs now render at correct world coordinates (previously rendered inside INSERT transform)
- Minimum text size enforced (7px screen text minimum)
- Browse page always renders DXF even when no diff data available

## [v0.5.2] — 2026-07-28

### Added

- **ATTRIB extraction** — block attributes (terminal names, values, formulas) extracted and rendered
- MTEXT formatting stripping
- Y-flip fix for canvas rendering (DXF Y-up → canvas Y-down)
- Browse DXFViewer component
- Nested `_modN` structure support

## [v0.5.1] — 2026-07-28

### Added

- **Proper DXF rendering** — ACI color index support (0-255), block geometry rendering, bulge arc tessellation, text alignment (horizontal/vertical)

## [v0.5.0] — 2026-07-28

### Added

- **Visual CAD rendering** in Edit page — DXFViewer component with canvas-based rendering
- Zoom, pan, grid overlay
- Entity rendering: LINE, LWPOLYLINE, POLYLINE, INSERT, TEXT, CIRCLE, ARC, POINT

## [v0.4.3] — 2026-07-28

### Added

- Valmet logo and favicon
- Two-column Compare layout (later replaced by single-view in v0.6.6)

## [v0.4.2] — 2026-07-28

### Added

- Folder browser component
- Project edit modal
- ZIP export/import for projects

## [v0.2.0] — 2026-07-28

### Added

- **React frontend** (`frontend/`) — full SPA with Vite + TypeScript, Valmet green dark industrial theme matching LOGReport
  - Dashboard page: stats cards, progress bars, log output
  - Compare page: folder selection, options, start/stop, live progress, log streaming
  - Results page: filterable table with status badges
  - Settings page: API settings management
- **Embedded frontend** — `//go:embed frontend_dist` serves built React app from Go binary
- Decompiled comparison_processor.py — full Python source reconstructed from .pyc bytecode for reference

### Changed

- Comparison processor rewritten to match Python tool exactly:
  - `compare_dict_of_lists` set-based comparison
  - `_modN` folders at output root level (e.g., `AA_mod1/`)
  - Files moved (not copied) from template folder to mod folder
  - Fallback template matching by filename prefix when `$(TEMPLATE)` attr is missing
  - Full detailed comparison logs with entity counts
  - `notemplate_dxfanalyze.log` created in notemplate folder
- Polylines storage changed to per-polyline `[][][3]float64` matching Python's tuple structure
- Line endpoint sorting includes Z coordinate comparison

## [v0.1.0] — 2026-07-28

### Added

- **Native DXF parser** (`internal/dxf/parser.go`) — parses DXF text format, extracts entities from ENTITY and BLOCKS sections
- **High-level extractor** (`internal/dxf/extractor.go`) — extracts blocks, lines, polylines from parsed DXF
- **Comparison processor** (`internal/compare/processor.go`) — full ComparisonProcessor ported from Python
- **Template management** (`internal/compare/template.go`) — `$(TEMPLATE)` attribute extraction, template map building
- **Test parse utility** (`cmd/test_parse/main.go`) — CLI tool to test DXF parser on sample files
- **Reference Python source** (`reference-python-source/`) — original Python DXF Compare Tool (5 .py files, ~2028 lines)
- **Architecture blueprint** (`BLUEPRINT.md`)

### Technical Details

- Zero external Go dependencies — native DXF parser uses only Go stdlib
- Content hashing — MD5 of JSON-serialized geometry
- Coordinate rounding — all coordinates rounded to N decimal places
- `_modN` grouping — modified files grouped by content hash into numbered subfolders