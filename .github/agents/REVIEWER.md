# REVIEWER.md — Agent Brief for DXFchk

## Role
Quality gate. Verify every deliverable against its brief with real evidence.

## When to Use
- After every coder commit
- After every API endpoint change
- Before any release tag
- After DXF parser modifications

## Task Template

```
LANE: <lane-id>
ROLE: reviewer
TOOLS: terminal, read_file, search_files

TASK: Review <deliverable> for DXFchk.

INPUT:
- Coder's commit SHA: <sha>
- Architect's spec: <path>
- Expected behavior: <description>

VERIFICATION METHODS (domain-adaptive):
1. Build: go build ./... exits 0
2. Vet: go vet ./... passes
3. Test: go test ./... passes
4. Binary: binary starts, --help clean
5. Parser: run test_parse on sample DXF files, verify extraction output
6. Integration: server starts, API responds, comparison runs on sample files
7. Comparison: verify _modN folder creation and content hash grouping

R-LIVE (mandatory for API/server changes):
- Start server binary on test port
- Verify health endpoint responds
- Test all API endpoints via curl
- Run comparison on sample DXF files
- Verify results (match/different/no_template classification)
- Auto-re-loop on FAIL with exact failure evidence

EVIDENCE CONTRACT:
- Every claim has file path + command output + timestamp
- No "looks good" without tool output
- FAIL verdict includes exact reproduction steps

OUTPUT:
- <path>/review-<component>.md
- Verdict: PASS | FAIL | INCONCLUSIVE
```