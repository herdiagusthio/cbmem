# cbmem — Codebase Memory

Local-first indexer that turns any repo into a queryable second-brain snapshot.

**Version:** 0.3.0 (2026-09-14) | License: MIT

## Quick Start

```bash
go build -o cbmem .           # needs gcc (CGO, mattn/go-sqlite3)
./cbmem index <repo> --json
./cbmem query <term> --repo <repo>
./cbmem impact <qualified> --depth 3 --repo <repo>
```

## Artifacts

Land in `~/code-storage/second-brain/codebase/<repo>/`:

| File | Purpose |
|------|---------|
| `symbols.db` | SQLite: symbols, edges (calls), links, FTS5 |
| `map.md` | files + counts |
| `routes.md` | handler/route table (heuristic) |
| `callgraph.dot` | Graphviz LR (trunc 200 nodes / 500 edges) |
| `links.jsonl` | symbol ↔ ADR/decision edges + content_hash for staleness |

## Languages

- **Go:** `go/ast` exact, calls resolved
- **TypeScript:** `.ts .tsx .js .jsx .astro .mjs .cjs` via regex
- **Python:** `.py .pyi` via regex  
- **SQL:** `.sql` via regex
- **YAML:** `.yaml .yml` via regex
- **Dockerfile:** via regex
- **Protobuf:** `.proto` via regex

Ponytail: tree-sitter for semantic accuracy.

## CLI Commands

| Command | Example | Purpose |
|---------|---------|---------|
| `init <repo>` | `cbmem init .` | create symbols.db + meta |
| `index <repo>` | `cbmem index . --json` | walk → dispatcher → upsert → artifacts |
| `query <term>` | `cbmem query Flight --repo repo` | LIKE + FTS5 bm25 |
| `callers <qual>` | `cbmem callers "Service.Do" -d 2` | recursive callers |
| `callees <qual>` | `cbmem callees "main" -d 2` | recursive callees |
| `impact <qual>` | `cbmem impact "Handler" -j` | callers+callees+routes/tests |
| `links [repo]` | `cbmem links .` | build links.jsonl (code+adr+notes) |
| `stale [repo]` | `cbmem stale .` | report drifted/deleted links |
| `version` | `./cbmem version` | print version |

## Hermes Skill

Installed at `~/.hermes/skills/software-development/codebase-onboard/`:

```bash
scripts/onboard.sh <repo> [--incremental] [--json]
scripts/query.sh <term> [repo]
scripts/links.sh <repo>
scripts/stale.sh <repo>
scripts/sync.sh <repo> [msg]    # commits to second-brain, opens PR branch
```

## Link Sources (T5.4)

- **Code:** `// cbmem:link symbol=Foo target=knowledge/foo.md kind=documents`
- **ADR:** frontmatter `evidence: [symbol_or_path, ...]` in `knowledge/`, `decisions/`, `journal/`
- **Notes:** `<!-- cbmem:symbol=... -->` in any `.md` — parsed by `ParseNoteLinks`

## Verify

```bash
go vet ./... && go test ./... && go build -o /tmp/cbmem .
/tmp/cbmem index ~/code-storage/flight-search-system --json  # 655 sym 124 edges 41 files
ls ~/code-storage/second-brain/codebase/flight-search-system/
```

## Docs

- `docs/architecture.md` — data model, components, flow
- `docs/cli.md` — reference
- `docs/installation.md` — install from binary or source

## Release Notes

- **v0.1.0**: 7-lang indexer, SQLite FTS5, linker (code+ADR), artifacts
- **v0.2.0**: impact analysis (`GetImpact`), note-sync `<!-- cbmem:symbol=... -->`
- **v0.3.0**: git hooks, GH Action `cbmem-index.yml`, skill v0.3.0 with links/stale/sync scripts