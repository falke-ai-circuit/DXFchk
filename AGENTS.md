# AGENTS.md — DXFchk (Agent Delegation Rules)

> This file is referenced by agent profiles when working with this repo.
> Current version: **v0.6.7**

## Repo Conventions

### Build
```bash
# CRITICAL: Build frontend BEFORE Go binary (stale //go:embed = stale UI)
cd frontend && npm install && npm run build
cp -r frontend/dist/* internal/api/frontend_dist/
go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk ./cmd/dxfchk/

# Or via Makefile
make build          # Build DXFchk binary (assumes frontend already built)
make test           # Run tests
make vet            # Run go vet
make cross          # Cross-compile (linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64)
```

### Go Version
- Go 1.23.12 at `/opt/data/go/bin/go1.23.12` (set GOROOT=/opt/data/go)
- go.mod specifies `go 1.22`

### Commit Style
- Prefix: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`
- Scope: `feat(compare):`, `fix(dxf):`, `feat(ui):`, `docs:`
- Tag: annotated with release notes (`git tag -a v0.6.7 -m "..."`)
- Push: `git push origin main --tags`

### Forbidden
- Force-push to main without explicit approval
- Breaking API changes without version bump
- Hardcoded secrets in source (use env vars)
- Direct edits to `go.mod`/`go.sum` (use `go get`/`go mod tidy`)
- New dependencies without justification in PR description
- No external Go dependencies — keep the parser stdlib-only
- Building Go binary without rebuilding frontend first (stale embed)

### Review Gates
1. `go build ./...` exits 0
2. `go vet ./...` passes
3. `go test ./...` passes (if tests exist)
4. `cd frontend && npx tsc --noEmit` passes (frontend type check)
5. Binary runs with `--help` cleanly
6. Integration test: server starts, health endpoint responds, comparison runs
7. Frontend built and copied to `internal/api/frontend_dist/` before Go build

### R-LIVE (mandatory for API/server changes)
- Start server binary on test port
- Verify health endpoint responds
- Test all API endpoints via curl
- Run comparison on sample DXF files
- Browser test all pages (Dashboard, Compare, Browse, Edit, Settings)
- Auto-re-loop on FAIL with exact failure evidence

### Creative Integration Testing
- For any deliverable talking to an external system, build a misbehaving mock and test with real I/O
- Mock DXF files: malformed entities, missing sections, empty files, SEQEND without INSERT
- Mock API: invalid requests, wrong content types, connection drops

### Agent Briefs
See `.github/agents/` for per-agent task templates:
- `ANALYST.md` — Codebase analysis
- `ARCHITECT.md` — System design
- `CODER.md` — Implementation
- `REVIEWER.md` — Quality gate

## Key Architecture Facts

- **Backend**: Go, zero external deps, `internal/dxf` (parser) + `internal/compare` (engine) + `internal/api` (server)
- **Frontend**: React + Vite + TypeScript, embedded via `//go:embed`
- **Parallel processing**: JobManager (inter-project) + 4-worker pool (intra-comparison)
- **Template matching**: `$(TEMPLATE)` attr + filename prefix fallback; browse uses folder path
- **Output structure**: `Output/TEMPLATE_NAME/[_modN]/files.dxf` + `Output/notemplate/`
- **Project store**: `~/.dxfchk/projects.json`
- **Session state**: `~/.dxfchk/session_<jobID>.json`
- **Theme**: Valmet green (#008a00) dark industrial, matching LOGReport

## Known Pitfalls

- `//go:embed` caches at build time — always rebuild frontend first
- Go 1.22 ServeMux trailing-slash shadows routes — use method patterns
- ATTRIBs render in world coordinates (not inside INSERT transform)
- Invisible ATTRIB flag = code 70 bit 0x80
- Block names not visible in CAD (BricsCAD doesn't show them)
- collectEntityPairs can infinite loop on SEQEND (fixed v0.6.3)
- Mod folder log path double-nesting (fixed v0.6.3)
- Template matching from filename unreliable — use folder path (fixed v0.6.5)