# codebase-onboard Skill

**Type:** Skill Package Documentation — how to install, use, extend.

## Overview

Hermes skill that wraps `cbmem` CLI for chat-native codebase onboarding. Installs to `~/.hermes/skills/software-development/codebase-onboard/`.

## Installation

```bash
# From Hermes chat:
/skill install codebase-onboard
# Or manually copy:
cp -r ~/.hermes/skills/software-development/codebase-onboard /path/to/project
```

## Scripts

| Script | Purpose | Usage |
|--------|---------|-------|
| `onboard.sh` | init + index | `onboard.sh <repo> [--incremental] [--json]` |
| `query.sh` | search symbols | `query.sh <term> [repo] [json-flag]` |
| `links.sh` | sync links.jsonl (code+adr+notes) | `links.sh <repo> [--json]` |
| `stale.sh` | report drifted links | `stale.sh <repo> [--json]` |
| `sync.sh` | commit to second-brain | `sync.sh <repo> [commit-message]` |

## GitHub Actions Template

Fork `.github/workflows/cbmem-index.yml` for your own second-brain repo — runs on push, commits artifacts.

## Extending

- Add TS/tree-sitter indexers: `internal/indexer/typescript/indexer.go`
- Add Mermaid export: `artifacts/artifacts.go WriteCallgraphMermaid`
- Add route annotation: `// cbmem:route GET /path`

## Troubleshooting

- `cbmem not found`: install CBEM or `go build -o cbmem ./...`
- `gcc not found`: `apt install gcc` / `brew install gcc`
- Large repo (100k+ symbols): batch inserts in `store.go` already; ponytail `PRAGMA mmap_size`
- No routes.md: add `// cbmem:route` comments or expect yaml/proto skip

## Related Skills

- `systemic-planning-with-files` — sync task_plan.md tracking
- `gstack-review`, `gstack-plan-eng-review` — architecture review
- `hermes-agent-skill-authoring` — create new skills

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 0.1.0 | 2026-09-13 | 7-lang indexer, linker, artifacts |
| 0.2.0 | 2026-09-14 | impact analysis, note-sync |
| 0.3.0 | 2026-09-14 | git hooks, GH Action, link/stale scripts |