# Changelog

All notable changes to DXFchk are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

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