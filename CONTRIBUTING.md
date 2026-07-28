# DXFchk — Development Guidelines

## Repository Ruleset

### Branch Strategy
- `main` — production-ready code, always builds, always passes tests
- `feature/*` — new features (e.g., `feature/api-server`, `feature/frontend`)
- `fix/*` — bug fixes (e.g., `fix/parser-entities`)
- Branch from `main`, rebase before merge, delete after merge

### Versioning (Semantic Versioning)
- Format: `vMAJOR.MINOR.PATCH` (e.g., `v0.1.0`)
- MAJOR: breaking API changes
- MINOR: new features, new endpoints, backward-compatible
- PATCH: bug fixes, UI improvements, no new functionality
- Version bump happens in `cmd/dxfchk/main.go` (`version` const, injected via `-ldflags`)
- Tag the release: `git tag v0.1.0 && git push origin v0.1.0`

### Commit Conventions (Conventional Commits)
```
<type>(<scope>): <description>

type:    feat | fix | docs | refactor | test | chore | build | ci
scope:   parser | compare | api | db | web | ui | docs
```

Examples:
- `feat(parser): add CIRCLE entity support to DXF parser`
- `fix(compare): correct content hash for empty polylines`
- `feat(api): add comparison status endpoint with SSE`
- `docs: update README with API endpoint list`

### Before Every Commit
1. `go build ./...` — must compile clean
2. `go test ./internal/...` — all tests pass (use `-race` for CI)
3. `cd web && npm run build` — frontend builds clean (when available)
4. `git diff --cached` — review staged changes for scope creep
5. No commented-out code, no debug `fmt.Println`, no TODO without issue ref

### Before Every Push (Pre-Push Checklist)
1. All tests pass
2. Frontend built and embedded in server binary (when available)
3. CHANGELOG.md updated with version bump entry
4. If new endpoints added: update README.md API table
5. If new config flags: update `--help` output and README
6. Delegate to architect for design docs, README updates, and repo knowledge updates
7. `git push origin main` (or feature branch)

### Architect Delegation (Before Push)
When changes affect any of these surfaces, delegate to architect before pushing:
- **README.md** — new features, deployment changes, config changes
- **DESIGN.md** — architecture changes, new subsystems, data flow changes
- **BLUEPRINT.md** — scope changes, closure criteria changes
- **CHANGELOG.md** — version history entries
- **ROADMAP.md** — phase status updates

### File Organization
```
cmd/dxfchk/         — server entry point, flags, wiring (Phase 2)
cmd/test_parse/      — parser test utility
internal/dxf/        — DXF parser + extractor
internal/compare/    — comparison engine (processor, template, diff, hash)
internal/api/        — HTTP handlers, REST API (Phase 2)
internal/db/         — SQLite database (Phase 2)
internal/config/     — app configuration (Phase 2)
web/                 — React frontend (Phase 3)
reference-python-source/ — original Python source (reference only, not built)
embed.go             — go:embed directive for web/dist/ (Phase 3)
```

### Frontend Build (Phase 3)
- Source: `web/src/` (TypeScript + React)
- Build: `cd web && npm run build` → outputs to `web/dist/`
- Embedded into Go binary via `//go:embed all:web/dist` in `embed.go`
- **Always rebuild frontend before rebuilding server** — stale embed = stale UI

### Testing
- Unit tests: `go test ./internal/...` (with `-race` flag)
- Parser tests: `go run ./cmd/test_parse/ <dxf-file>` — verify extraction on real DXF files
- Integration tests: `go test ./internal/... -run Integration`
- Frontend: `cd web && npx tsc --noEmit` (type check)

### Cross-Compilation
```bash
# Windows amd64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dxfchk.exe ./cmd/dxfchk/

# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dxfchk ./cmd/dxfchk/
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