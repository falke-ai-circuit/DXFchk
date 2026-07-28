# AGENTS.md — DXFchk (Agent Delegation Rules)

> This file is referenced by agent profiles when working with this repo.

## Repo Conventions

### Build
```bash
make build          # Build DXFchk binary
make test           # Run tests
make vet            # Run go vet
make cross          # Cross-compile for all platforms (linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64)
make run            # Build + run server on default port
make web            # Build frontend (when available)
```

### Commit Style
- Prefix: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`
- Tag: annotated with release notes (`git tag -a v0.1.0 -m "..."`)
- Push: `git push origin main --tags`

### Forbidden
- Force-push to main without explicit approval
- Breaking API changes without version bump
- Hardcoded secrets in source (use env vars)
- Direct edits to `go.mod`/`go.sum` (use `go get`/`go mod tidy`)
- New dependencies without justification in PR description
- No external Go dependencies — keep the parser stdlib-only

### Review Gates
1. `go build ./...` exits 0
2. `go vet ./...` passes
3. `go test ./...` passes (if tests exist)
4. Binary runs with `--help` cleanly
5. Integration test: server starts, API responds, comparison runs

### R-LIVE (mandatory for API/server changes)
- Start server binary on test port
- Verify health endpoint responds
- Test all API endpoints via curl
- Run comparison on sample DXF files
- Auto-re-loop on FAIL with exact failure evidence

### Creative Integration Testing
- For any deliverable talking to an external system, build a misbehaving mock and test with real I/O
- Mock DXF files: malformed entities, missing sections, empty files
- Mock API: invalid requests, wrong content types, connection drops

### Agent Briefs
See `.github/agents/` for per-agent task templates:
- `ANALYST.md` — Codebase analysis
- `ARCHITECT.md` — System design
- `CODER.md` — Implementation
- `REVIEWER.md` — Quality gate