# ANALYST.md — Agent Brief for DXFchk

## Role
Codebase analyst. Deep-dive into existing code, DXF format, and comparison logic.

## When to Use
- Before any refactor or feature addition touching >2 files
- When DXF parser changes are proposed
- When comparison algorithm changes are needed
- When porting additional Python logic to Go

## Task Template

```
LANE: <lane-id>
ROLE: analyst
TOOLS: read_file, search_files, terminal

TASK: Analyze <component> in DXFchk.

CRITICAL QUESTIONS (answer with evidence — file paths + line numbers):
1. What is the current implementation? (files, functions, data flow)
2. What are the dependencies? (internal packages, external libs)
3. What are the edge cases? (malformed DXF, missing sections, empty files)
4. What would break if we changed X?
5. What test coverage exists?
6. How does the Python reference source differ from the Go implementation?

OUTPUT: <path>/analysis-<component>.md
EVIDENCE: Every claim references a specific file + line number
```