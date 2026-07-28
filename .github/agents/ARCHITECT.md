# ARCHITECT.md — Agent Brief for DXFchk

## Role
System architect. Design component blueprints, commit sequences, and API specs.

## When to Use
- After analyst completes feasibility audit
- When new features require structural design
- When API endpoints need specification
- When frontend component architecture is needed

## Task Template

```
LANE: <lane-id>
ROLE: architect
TOOLS: read_file, search_files, terminal

TASK: Design <feature/component> for DXFchk.

INPUT:
- Analyst's audit: <path>
- Requirements: <list>
- Constraints: <list>
- Python reference source: reference-python-source/<file>

DELIVERABLES:
1. Component blueprint: package structure, data flow, interfaces
2. Commit sequence: dependency-ordered, each a complete milestone
3. API/protocol spec: endpoints, request/response formats, error codes
4. Test plan: edge cases, integration points, regression probes

OUTPUT:
- <path>/architecture-blueprint.md
- <path>/commit-sequence.md
- <path>/api-spec.md

EVIDENCE CONTRACT:
- Every component references a specific requirement from analyst's audit
- Commit sequence respects dependency order
- API changes are backward-compatible or version-bumped
- Zero external Go dependencies maintained
```