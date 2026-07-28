# DXFchk Design

## System Overview

DXFchk is a single-binary web application: a Go HTTP server with an embedded React frontend. It compares DXF module files against DXF template files for Valmet Eclipse automation systems, detects modifications, and organizes modified files into `_modN` subfolders.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    DXFchk Binary                          │
│                                                          │
│  ┌─────────────┐    ┌──────────────────────────────────┐ │
│  │  Go Server   │    │  Embedded React Frontend          │ │
│  │  (net/http)  │    │  (embed.FS → web/dist)            │ │
│  │              │    │                                   │ │
│  │  REST API    │◄──►│  Setup, Run, Results, Diff, Logs  │ │
│  │  handlers    │    │                                   │ │
│  └──────┬───────┘    └──────────────────────────────────┘ │
│         │                                                │
│  ┌──────┴──────────────────────────────────────────────┐ │
│  │           Internal Packages                          │ │
│  │  dxf (parser, extractor) · compare (processor,       │ │
│  │  template, diff, hash) · api · db · config           │ │
│  └──────┬──────────────────────────────────────────────┘ │
│         │                                                │
│  ┌──────┴──────┐  ┌────────────┐  ┌──────────────────┐  │
│  │  SQLite DB  │  │ filesystem  │ │  DXF files       │  │
│  │  (db)        │  │ (modules)   │ │  (templates)     │  │
│  └─────────────┘  └────────────┘  └──────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

## Comparison Flow

```
User selects template folder + search folder
        │
        ▼
┌──────────────────────┐
│  Scan template folder │  ◄── Extract $(TEMPLATE) from each DXF
│  Build template map    │      template_name → file_path
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Scan search folder  │  ◄── Find all .dxf module files
│  for DXF modules      │      (recursive option)
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  For each module:    │  ◄── Extract $(TEMPLATE) attribute
│  Extract template    │      Match against template map
│  attribute            │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Extract geometry    │  ◄── ExtractDXFContent()
│  (blocks/lines/poly)  │      Round coords to N decimals
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Compare against    │  ◄── CompareDicts()
│  template geometry   │      Dict-of-lists comparison
│  (same template)     │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Compute content     │  ◄── ContentHash()
│  hash if different    │      MD5 of JSON-serialized geometry
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Group by content   │  ◄── Files with same hash → same _modN
│  hash → _modN folders │      N = 1, 2, 3, ...
│  Move files           │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Save comparison    │  ◄── Per-template log file
│  log per template     │
└──────────────────────┘
```

### DXF Parsing

```
DXF file (text format)
    │
    ├── HEADER section    ──► (parsed, not used)
    ├── TABLES section    ──► (parsed, not used)
    ├── BLOCKS section    ──► parser.go → block definitions
    │                         extractor.go → blocks dict
    │                         (INSERT positions, excluding
    │                          COMPANY/CUSTOMER attributes)
    ├── ENTITIES section  ──► parser.go → entity records
    │                         extractor.go → lines dict
    │                         (sorted endpoints)
    │                         extractor.go → polylines dict
    │                         (normalized vertices)
    └── OBJECTS section   ──► (parsed, not used)
```

### Template Extraction

```
DXF module file
    │
    ▼
Search ENTITIES section for INSERT with $(TEMPLATE) attribute
    │
    ├── $(TEMPLATE) found  ──► template_name extracted
    │                           → match against template map
    │                           → compare geometry
    │                           → if different: hash + group
    │
    └── $(TEMPLATE) not found ─► status: "no_template"
```

## Data Flow

```
DXF files (filesystem)
    │
    ├── template folder ──► template.go ──► template map
    │                                         (name → path)
    │
    ├── search folder   ──► processor.go ──► for each module:
    │                                         extract template
    │                                         compare geometry
    │                                         compute hash
    │
    ▼
ComparisonProcessor
    │
    ├── "match" files     ──► stay in template folder
    ├── "different" files ──► moved to _modN subfolder
    │                         grouped by content_hash
    └── "no_template"    ──► logged, not moved
    │
    ▼
Results
    │
    ├── _modN folders     ──► {template_folder}/_mod1/, _mod2/, ...
    ├── comparison logs   ──► {output_folder}/{template_name}_log.txt
    └── database records ──► modules table, templates table
```

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    template_name TEXT,
    template_path TEXT,
    status TEXT,          -- 'match', 'different', 'no_template'
    content_hash TEXT,
    mod_folder TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_name TEXT UNIQUE NOT NULL,
    file_path TEXT NOT NULL,
    file_hash TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT
);
```

## Component Description

### `internal/dxf/parser.go`

Native DXF text parser. Reads DXF files line-by-line, extracts section boundaries and entity records. No external dependencies (no ezdxf equivalent). Handles:
- Section detection (HEADER, TABLES, BLOCKS, ENTITIES, OBJECTS)
- Entity parsing (INSERT, LINE, POLYLINE, CIRCLE, ARC, TEXT, ATTRIB)
- Attribute extraction (group codes → key-value pairs)
- Coordinate parsing (float64 from string)

### `internal/dxf/extractor.go`

High-level extraction from parsed DXF. Converts raw entity records into structured dicts:
- **Blocks**: INSERT positions with attributes, excluding COMPANY/CUSTOMER
- **Lines**: endpoints sorted (min→max) for deterministic comparison
- **Polylines**: vertices normalized (order-independent)
- All coordinates rounded to N decimal places

### `internal/compare/template.go`

Template management. Builds the template map by scanning a folder for DXF files and extracting `$(TEMPLATE)` attributes. Provides:
- `BuildTemplateMap(folder string) (map[string]string, error)` — template_name → file_path
- `ExtractTemplateName(filePath string) (string, error)` — get `$(TEMPLATE)` from a module file

### `internal/compare/processor.go`

Core comparison processor. Orchestrates the full comparison workflow:
- Scan search folder for DXF modules
- Extract template attribute from each module
- Match against template map
- Compare geometry (dict-of-lists comparison)
- Compute content hash for modified files
- Create `_modN` subfolders and move files
- Generate per-template comparison logs