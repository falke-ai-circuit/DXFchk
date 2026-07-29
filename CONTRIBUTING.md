# DXFchk — Development Guidelines

## Repository Ruleset

### Branch Strategy
- `main` — production-ready code, always builds, always passes tests
- `feature/*` — new features (e.g., `feature/parallel-processing`, `feature/template-groups`)
- `fix/*` — bug fixes (e.g., `fix/collectentitypairs-loop`, `fix/mod-log-path`)
- Branch from `main`, rebase before merge, delete after merge

### Versioning (Semantic Versioning)
- Format: `vMAJOR.MINOR.PATCH` (e.g., `v0.6.7`)
- MAJOR: breaking API changes
- MINOR: new features, new endpoints, backward-compatible
- PATCH: bug fixes, UI improvements, no new functionality
- Version injected via `-ldflags "-X main.cliVersion=v0.6.7"`
- Tag the release: `git tag v0.6.7 && git push origin v0.6.7`

### Commit Conventions (Conventional Commits)
```
<type>(<scope>): <description>

type:    feat | fix | docs | refactor | test | chore | build | ci
scope:   parser | compare | api | frontend | ui | docs | dxf
```

Examples:
- `feat(compare): add 4-worker parallel processing pool`
- `fix(dxf): correct collectEntityPairs infinite loop on SEQEND`
- `feat(ui): LOGReport-style dashboard with inline project settings`
- `docs: update README to v0.6.7`

### Before Every Commit
1. `go build ./...` — must compile clean
2. `go vet ./...` — must pass
3. `go test ./internal/...` — all tests pass (use `-race` for CI)
4. `cd frontend && npx tsc --noEmit` — frontend type check passes
5. `git diff --cached` — review staged changes for scope creep
6. No commented-out code, no debug `fmt.Println`, no TODO without issue ref

### Before Every Push (Pre-Push Checklist)
1. All tests pass
2. **Frontend built and copied to embed directory** — `cd frontend && npm run build && cp -r dist/* ../internal/api/frontend_dist/`
3. **Rebuild Go binary AFTER frontend** — stale `//go:embed` = stale UI
4. CHANGELOG.md updated with version bump entry
5. If new endpoints added: update README.md API table
6. If new config flags: update `--help` output and README
7. Delegate to architect for design docs, README updates, and repo knowledge updates
8. `git push origin main` (or feature branch)

### Architect Delegation (Before Push)
When changes affect any of these surfaces, delegate to architect before pushing:
- **README.md** — new features, deployment changes, config changes
- **DESIGN.md** — architecture changes, new subsystems, data flow changes
- **BLUEPRINT.md** — scope changes, closure criteria changes
- **CHANGELOG.md** — version history entries
- **ROADMAP.md** — phase status updates

### File Organization
```
cmd/dxfchk/              — server entry point + CLI mode, flags, wiring
cmd/debug_hash/          — content hashing debug utility
internal/dxf/            — DXF parser + extractor
internal/compare/        — comparison engine (processor, parallel, template)
internal/api/            — HTTP server, REST API, project store, session, browse, diff
frontend/src/            — React frontend (pages, components, api, store, styles)
internal/api/frontend_dist/ — embedded built frontend (from frontend/dist/)
reference-python-source/ — original Python source (reference only, not built)
```

### Frontend Build
- Source: `frontend/src/` (TypeScript + React + Vite)
- Build: `cd frontend && npm run build` → outputs to `frontend/dist/`
- Copy to embed: `cp -r frontend/dist/* internal/api/frontend_dist/`
- Embedded into Go binary via `//go:embed frontend_dist` in `internal/api/server.go`
- **Always rebuild frontend before rebuilding server** — stale embed = stale UI
- Verify JS hash changed after rebuild (check `frontend/dist/assets/*.js` filename)

### Testing
- Unit tests: `go test ./internal/...` (with `-race` flag)
- Parser tests: `go run ./cmd/test_parse/ <dxf-file>` — verify extraction on real DXF files
- Frontend type check: `cd frontend && npx tsc --noEmit`
- Integration test: start server, curl health endpoint, run comparison
- Browser testing: visual verification of all pages through browser

### Cross-Compilation
```bash
# Windows amd64 (primary target — Valmet VMs)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk.exe ./cmd/dxfchk/

# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.cliVersion=v0.6.7" -o dxfchk ./cmd/dxfchk/
```

### Security Rules
- Never commit credentials, tokens, or passwords
- `.env` files are gitignored
- No hardcoded paths — use flags or config

### Code Style
- Go: follow `gofmt` + `go vet` clean, meaningful error wrapping with `%w`
- TypeScript: strict mode, no `any` without justification, functional components with hooks
- CSS: CSS variables in `:root`, no CSS-in-JS, class-based styling
- No magic numbers — use named constants
- Functions < 50 lines preferred, extract if longer
- Zero external Go dependencies — parser must remain stdlib-only