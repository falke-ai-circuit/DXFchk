# DXFchk — End-to-End Workflow Test Document

This document describes a complete workflow test from zero: creating a project, importing templates and modules, running a comparison, verifying output, browsing results, rendering DXF files with diff highlighting, creating templates from mod folders, editing DXF files, applying templates to groups, and generating reports.

**Test Environment:**
- DXFchk deployed on Vegas VM (EAS-C2022), port 8643
- PROBE agent at `http://187.124.31.229:80`, agent `vegas-c2022`, login `admin/falke-admin-2026`
- Test data: TemplatesEclipse (1064 DXF templates), UncheckedEclipse (1753 DXF modules)
- Template folder: `C:\Users\Administrator\Desktop\TemplatesEclipse`
- Search folder: `C:\Users\Administrator\Desktop\UncheckedEclipse`
- Output folder: `C:\Users\Administrator\Desktop\DXFchk_output_new`

**Base URL for all curl commands:**
```bash
BASE="http://187.124.31.229:8643"
```

---

## Step 1: Create New Project

**What to do:** Create a new project with template folder, search folder, and output folder set.

**Expected result:** Project is created with a unique ID, all three folders configured, and returned in the response.

**How to verify:**
```bash
curl -s -X POST "$BASE/api/v1/projects" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Eclipse Full Test",
    "template_folder": "C:\\Users\\Administrator\\Desktop\\TemplatesEclipse",
    "search_folder": "C:\\Users\\Administrator\\Desktop\\UncheckedEclipse",
    "output_folder": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new"
  }' | jq .
```

**Expected output:**
```json
{
  "ok": true,
  "project": {
    "id": "eclipse-full-test",
    "name": "Eclipse Full Test",
    "template_folder": "C:\\Users\\Administrator\\Desktop\\TemplatesEclipse",
    "search_folder": "C:\\Users\\Administrator\\Desktop\\UncheckedEclipse",
    "output_folder": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new",
    "recursive": true,
    "group_by_content": true
  }
}
```

**Browser check:** Open `http://187.124.31.229:8643` → Dashboard should show the new project card in the project list.

---

## Step 2: Import Modules and Templates

**What to do:** Scan the template folder to build the template map. This reads the `$(TEMPLATE)` attribute from each DXF file in the template folder.

**Expected result:** Template map is built with ~1064 entries (one per template DXF file containing a `$(TEMPLATE)` attribute).

**How to verify:**
```bash
# Scan templates
curl -s -X POST "$BASE/api/v1/templates/scan" \
  -H "Content-Type: application/json" \
  -d '{
    "template_folder": "C:\\Users\\Administrator\\Desktop\\TemplatesEclipse",
    "recursive": true
  }' | jq '.count'
```

**Expected output:** A number close to 1064 (the exact count depends on how many DXF files have valid `$(TEMPLATE)` attributes).

**Browser check:** Settings panel should show template count. The project card should display the template folder path.

---

## Step 3: Start Comparison

**What to do:** Start a comparison job using the project's folders. This launches 4 parallel workers.

**Expected result:** Comparison starts in background, returns a job ID immediately.

**How to verify:**
```bash
# Get the project ID first
PROJECT_ID=$(curl -s "$BASE/api/v1/projects" | jq -r '.projects[0].id')

# Start comparison
curl -s -X POST "$BASE/api/v1/compare" \
  -H "Content-Type: application/json" \
  -d "{
    \"project_id\": \"$PROJECT_ID\",
    \"project_name\": \"Eclipse Full Test\",
    \"search_folder\": \"C:\\Users\\Administrator\\Desktop\\UncheckedEclipse\",
    \"template_folder\": \"C:\\Users\\Administrator\\Desktop\\TemplatesEclipse\",
    \"output_folder\": \"C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\",
    \"recursive\": true,
    \"group_by_content\": true
  }" | jq .
```

**Expected output:**
```json
{
  "ok": true,
  "message": "comparison started",
  "output": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new",
  "job_id": "eclipse-full-test"
}
```

**Browser check:** Dashboard should show a running progress bar with file count, ETA, and live log messages.

---

## Step 4: Confirm Logs Are Generated

**What to do:** After the comparison completes, verify that detailed comparison logs (`.log` files) are generated in the output folder — one per template folder and one per `_modN` folder.

**Expected result:** Each template folder and each `_modN` subfolder contains a `*_dxfanalyze.log` file with detailed comparison results.

**How to verify:**

Via PROBE (to inspect the Windows filesystem):
```
POST http://187.124.31.229:80/api/v1/exec
{
  "agent": "vegas-c2022",
  "command": "cmd /c dir /s /b C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\*_dxfanalyze.log"
}
```

Or via the log content API:
```bash
# Get log content for a specific template folder log
curl -s "$BASE/api/v1/log?path=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_dxfanalyze.log" | jq '.lines'
```

**Expected:** Log files exist in:
- `Output/BI001/BI001_dxfanalyze.log` — log for the BI001 template folder
- `Output/BI001/BI001_mod1/BI001_mod1_dxfanalyze.log` — log for the first modification group
- `Output/notemplate/notemplate_dxfanalyze.log` — log for files with no matching template

Each log should contain:
- Header with template name and timestamp
- Entity counts (blocks, lines, polylines)
- Detailed differences per file
- Summary (MATCH or DIFFERENT)

---

## Step 5: Confirm Folder Structure (Nested _modN)

**What to do:** Verify the output folder structure has nested `_modN` folders under each template name.

**Expected result:** Folder structure follows this pattern:
```
DXFchk_output_new/
├── BI001/                          # template folder
│   ├── BI001_module1.dxf           # matched file (identical to template)
│   ├── BI001_module2.dxf           # matched file
│   ├── BI001_dxfanalyze.log        # template-level log
│   └── BI001_mod1/                 # nested mod folder (different from template)
│       ├── BI001_modified.dxf
│       └── BI001_mod1_dxfanalyze.log
├── BO002/
│   ├── BO002_output1.dxf
│   └── BO002_mod1/
│       └── BO002_custom.dxf
└── notemplate/
    ├── ZZZ_unknown.dxf
    └── notemplate_dxfanalyze.log
```

**How to verify:**

Via PROBE:
```
POST http://187.124.31.229:80/api/v1/exec
{
  "agent": "vegas-c2022",
  "command": "cmd /c dir /ad /b C:\\Users\\Administrator\\Desktop\\DXFchk_output_new"
}
```

Then check for nested mod folders:
```
cmd /c dir /ad /s /b C:\Users\Administrator\Desktop\DXFchk_output_new\*_mod*
```

**Expected:** Template folders (BI001, BO002, etc.) appear at the top level. Each may contain `_modN` subfolders. The `notemplate` folder holds files with no matching template.

---

## Step 6: Confirm Browse Shows Correct Tree

**What to do:** Call the Browse API endpoint and verify the tree structure shows template folders with nested `_modN` folders visible.

**Expected result:** Browse endpoint returns a tree JSON with:
- Template folders marked `is_template: true`
- `_modN` subfolders marked `is_mod: true`
- DXF files with correct `dxf_count`
- Nested children visible (max depth 5)

**How to verify:**
```bash
# Get the browse tree
curl -s "$BASE/api/v1/browse?path=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new" | jq '.tree.children[] | {name, is_template, is_mod, dxf_count, children: [.children[]? | {name, is_mod, dxf_count}]}' | head -40
```

**Expected output:**
```json
{
  "name": "BI001",
  "is_template": true,
  "is_mod": false,
  "dxf_count": 5,
  "children": [
    {"name": "BI001_mod1", "is_mod": true, "dxf_count": 3},
    {"name": "BI001_module1.dxf", "is_mod": false, "dxf_count": 1}
  ]
}
```

**Browser check:** Open the Browse tab — tree should show:
- Top-level template folders with folder icons
- Expandable `_modN` subfolders nested inside
- DXF files with file icons
- File counts per folder
- `notemplate` folder at the bottom

---

## Step 7: Confirm DXF Rendering with Diff Highlighting

**What to do:** Click on a DXF file in a `_modN` folder. The render endpoint should display the DXF with differences highlighted against its template.

**Expected result:** The diff endpoint returns entity-level differences:
- `added` — entities in the module but not in the template (highlighted in green/red)
- `removed` — entities in the template but not in the module
- `modified` — entities with changed coordinates
- `bounding_box` — viewport bounds for rendering

**How to verify:**
```bash
# Render a single DXF file
curl -s "$BASE/api/v1/dxf/render?path=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf" | jq '{count, type_counts, bounding_box}'
```

**Expected output:**
```json
{
  "count": 25,
  "type_counts": {
    "line": 15,
    "insert": 5,
    "text": 3,
    "lwpolyline": 2
  },
  "bounding_box": [0.0, 0.0, 100.0, 50.0]
}
```

```bash
# Get diff between module and its template
curl -s -X POST "$BASE/api/v1/diff" \
  -H "Content-Type: application/json" \
  -d '{
    "template_path": "C:\\Users\\Administrator\\Desktop\\TemplatesEclipse\\BI001.dxf",
    "module_path": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf"
  }' | jq '.summary'
```

**Expected output:**
```json
{
  "template_count": 20,
  "module_count": 25,
  "added_count": 5,
  "removed_count": 0,
  "modified_count": 0
}
```

**Browser check:** Click a DXF file in Browse → canvas should render the DXF drawing with:
- Template entities in white/gray
- Added entities in green
- Removed entities in red
- Block attributes (ATTRIB text) displayed at their positions
- Layer colors respected from the DXF file

---

## Step 8: Create Template from _modN (Right-Click in Edit)

**What to do:** In the Edit tab, right-click a DXF file in a `_modN` folder and select "Create template". This copies the file to the template folder as a new template.

**Expected result:** The selected DXF file is copied to the project's template folder, creating a new template that can be used for future comparisons.

**How to verify:**
```bash
# Create template from a mod folder file
curl -s -X POST "$BASE/api/v1/template/create" \
  -H "Content-Type: application/json" \
  -d '{
    "source_file": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf",
    "template_folder": "C:\\Users\\Administrator\\Desktop\\TemplatesEclipse",
    "template_name": "BI001_v2.dxf"
  }' | jq .
```

**Expected output:**
```json
{
  "ok": true,
  "message": "template created",
  "path": "C:\\Users\\Administrator\\Desktop\\TemplatesEclipse\\BI001_v2.dxf",
  "name": "BI001_v2.dxf"
}
```

**Verify the file exists:**
```
POST http://187.124.31.229:80/api/v1/exec
{
  "agent": "vegas-c2022",
  "command": "cmd /c if exist C:\\Users\\Administrator\\Desktop\\TemplatesEclipse\\BI001_v2.dxf echo EXISTS"
}
```

**Browser check:** Edit tab → right-click a DXF file in a _modN folder → "Create template" → confirmation toast appears. The new template should appear in the template folder on the next scan.

---

## Step 9: Edit DXF and Apply Template to Subfolder Set

**What to do:** In the Edit tab, open a DXF file for editing. Modify the content, save it, then apply the modified template to all files in the template group (all `_modN` folders under that template name).

**Expected result:**
1. DXF file is edited and saved (with .bak backup)
2. The fixed template is copied to the template folder and to each `_modN` folder as `_fixed.dxf`
3. All files in the group receive the fixed template reference

**How to verify:**

**9a. Get raw DXF content:**
```bash
curl -s "$BASE/api/v1/dxf/content?path=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf" | jq '.lines'
```

**9b. Save modified DXF content:**
```bash
# Get current content, modify a line, save back
CONTENT=$(curl -s "$BASE/api/v1/dxf/content?path=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf" | jq -r '.content')
MODIFIED=$(echo "$CONTENT" | sed 's/some_old_text/some_new_text/')
curl -s -X POST "$BASE/api/v1/dxf/content" \
  -H "Content-Type: application/json" \
  -d "{
    \"path\": \"C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf\",
    \"content\": $(echo "$MODIFIED" | jq -Rs .)
  }" | jq .
```

**Expected output:**
```json
{
  "ok": true,
  "message": "Saved BI001_modified.dxf (backup at BI001_modified.dxf.bak)"
}
```

**9c. Apply template to group:**
```bash
# Apply the fixed template to all files in the BI001 group
curl -s -X POST "$BASE/api/v1/template/apply" \
  -H "Content-Type: application/json" \
  -d '{
    "template_path": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001_mod1\\BI001_modified.dxf",
    "group_name": "BI001",
    "output_folder": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new"
  }' | jq .
```

**Expected output:**
```json
{
  "ok": true,
  "message": "Template applied to group BI001 (3 mod folders)",
  "group": "BI001",
  "mod_folders": 3,
  "total_files": 15,
  "template_path": "C:\\Users\\Administrator\\Desktop\\DXFchk_output_new\\BI001\\BI001.dxf"
}
```

**9d. Verify template groups:**
```bash
curl -s "$BASE/api/v1/template/groups?output=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new" | jq '.groups[] | select(.template_name=="BI001") | {template_name, matched_count, total_files, mod_folders: [.mod_folders[] | {folder_name, file_count}]}'
```

**Browser check:**
- Edit tab → select a DXF file → raw text editor opens
- Edit content → Save → toast "Saved with backup"
- Right-click → "Apply template to group" → toast showing mod folder count
- Template Groups view should show BI001 group with all mod folders listed

---

## Step 10: Generate Unified Report

**What to do:** After comparison completes, generate a unified report for the scan. This is done by querying the results and template groups endpoints, which aggregate all comparison data.

**Expected result:** A complete overview of the scan results:
- Total files processed
- Matched / Different / No-template counts
- Per-template breakdown with mod folder counts
- File listings per mod folder

**How to verify:**

**10a. Get comparison results:**
```bash
PROJECT_ID=$(curl -s "$BASE/api/v1/projects" | jq -r '.projects[0].id')

# Get results
curl -s "$BASE/api/v1/results?project_id=$PROJECT_ID" | jq '{count, summary: ([.results[] | .status] | group_by(.) | map({status: .[0], count: length}))}'
```

**Expected output:**
```json
{
  "count": 1753,
  "summary": [
    {"status": "different", "count": 850},
    {"status": "match", "count": 700},
    {"status": "no_template", "count": 203}
  ]
}
```

**10b. Get template groups overview:**
```bash
curl -s "$BASE/api/v1/template/groups?output=C:\\Users\\Administrator\\Desktop\\DXFchk_output_new" | jq '{
  total_groups: .count,
  groups: [.groups[] | {
    template_name,
    matched_count,
    total_files,
    mod_folder_count: (.mod_folders | length)
  }]
}' | head -50
```

**Expected output:**
```json
{
  "total_groups": 90,
  "groups": [
    {"template_name": "BI001", "matched_count": 12, "total_files": 45, "mod_folder_count": 3},
    {"template_name": "BO002", "matched_count": 8, "total_files": 30, "mod_folder_count": 2}
  ]
}
```

**10c. Get final job status:**
```bash
curl -s "$BASE/api/v1/compare/status?project_id=$PROJECT_ID" | jq '{
  running,
  total_files,
  processed_files,
  progress,
  matched,
  different,
  no_template,
  elapsed_time,
  job_id,
  project_name
}'
```

**Expected output:**
```json
{
  "running": false,
  "total_files": 1753,
  "processed_files": 1753,
  "progress": 100,
  "matched": 700,
  "different": 850,
  "no_template": 203,
  "elapsed_time": "00:15:32",
  "job_id": "eclipse-full-test",
  "project_name": "Eclipse Full Test"
}
```

**Browser check:**
- Dashboard → project card shows final stats (matched/different/no-template counts)
- Browse tab → tree shows all output folders
- Template Groups tab → shows all 90 template groups with mod folder counts
- Click any group → shows mod folders with file lists
- Status bar shows "Completed" with elapsed time

---

## Appendix: Quick Verification Script

Run all steps in sequence (requires `jq` and `curl`):

```bash
#!/bin/bash
set -e
BASE="http://187.124.31.229:8643"
TPL="C:\\Users\\Administrator\\Desktop\\TemplatesEclipse"
SEARCH="C:\\Users\\Administrator\\Desktop\\UncheckedEclipse"
OUT="C:\\Users\\Administrator\\Desktop\\DXFchk_output_new"

echo "=== Step 1: Create Project ==="
PROJECT_ID=$(curl -s -X POST "$BASE/api/v1/projects" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Eclipse Test\",\"template_folder\":\"$TPL\",\"search_folder\":\"$SEARCH\",\"output_folder\":\"$OUT\"}" | jq -r '.project.id')
echo "Project ID: $PROJECT_ID"

echo "=== Step 2: Scan Templates ==="
TPL_COUNT=$(curl -s -X POST "$BASE/api/v1/templates/scan" \
  -H "Content-Type: application/json" \
  -d "{\"template_folder\":\"$TPL\",\"recursive\":true}" | jq -r '.count')
echo "Templates found: $TPL_COUNT"

echo "=== Step 3: Start Comparison ==="
curl -s -X POST "$BASE/api/v1/compare" \
  -H "Content-Type: application/json" \
  -d "{\"project_id\":\"$PROJECT_ID\",\"project_name\":\"Eclipse Test\",\"search_folder\":\"$SEARCH\",\"template_folder\":\"$TPL\",\"output_folder\":\"$OUT\",\"recursive\":true,\"group_by_content\":true}" | jq -r '.message'

echo "=== Step 4-5: Wait and check logs/folders ==="
echo "Polling status..."
while true; do
  RUNNING=$(curl -s "$BASE/api/v1/compare/status?project_id=$PROJECT_ID" | jq -r '.running')
  if [ "$RUNNING" = "false" ]; then break; fi
  sleep 5
done
curl -s "$BASE/api/v1/compare/status?project_id=$PROJECT_ID" | jq '{matched, different, no_template, elapsed_time}'

echo "=== Step 6: Browse Tree ==="
curl -s "$BASE/api/v1/browse?path=$OUT" | jq '.tree.children | length'

echo "=== Step 10: Final Report ==="
curl -s "$BASE/api/v1/results?project_id=$PROJECT_ID" | jq '.count'
curl -s "$BASE/api/v1/template/groups?output=$OUT" | jq '.count'

echo "=== DONE ==="
```

---

## Test Data Notes

- **TemplatesEclipse**: 1064 DXF template files at `C:\Users\Administrator\Desktop\TemplatesEclipse`
- **UncheckedEclipse**: 1753 DXF module files at `C:\Users\Administrator\Desktop\UncheckedEclipse`
- Each template DXF contains an INSERT entity with a `$(TEMPLATE)` ATTRIB that names the template (e.g., "BI001", "BO002")
- Module files are compared against templates by:
  1. Reading the `$(TEMPLATE)` attribute from the module
  2. If absent, falling back to filename prefix matching
  3. Comparing extracted geometry (blocks, lines, polylines) as sets
- Files identical to their template → `match` status → copied to `Output/TEMPLATE_NAME/`
- Files different from their template → `different` status → grouped by content hash → moved to `Output/TEMPLATE_NAME/TEMPLATE_NAME_modN/`
- Files with no matching template → `no_template` status → copied to `Output/notemplate/`