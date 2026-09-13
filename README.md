# cbmem — Codebase Memory

Local-first indexer that turns any repo into a queryable second-brain snapshot.

```
cbmem index <repo> --json
cbmem query <term> --repo <repo>
cbmem callers <QualifiedName> --depth 3
cbmem callees <QualifiedName>
cbmem links [repo]      # // cbmem:link + ADR evidence: -> links.jsonl
cbmem stale [repo]      # hash drift / deleted symbols
cat codebase/<repo>/callgraph.dot | dot -Tsvg > graph.svg
```

Artifacts land in `~/code-storage/second-brain/codebase/<repo>/`:

- `symbols.db` — SQLite: symbols, edges (calls), links, FTS5
- `map.md` — files + counts
- `routes.md` — handler/route table (heuristic)
- `callgraph.dot` — Graphviz LR (trunc 200 nodes / 500 edges)
- `links.jsonl` — symbol ↔ ADR/decision edges + content hash for staleness

## Install

```bash
go build -o cbmem .          # needs gcc (CGO, mattn/go-sqlite3)
./cbmem --help
```

Hermes skill: `~/.hermes/skills/software-development/codebase-onboard/` wraps `cbmem`:

```bash
~/.hermes/skills/software-development/codebase-onboard/scripts/onboard.sh ~/code-storage/flight-search-system --json
~/.hermes/skills/software-development/codebase-onboard/scripts/query.sh Flight --repo ~/code-storage/flight-search-system
```

## Languages

Go (`go/ast` exact, calls resolved) + TS/TSX/JS/Astro/Python/SQL/YAML/Dockerfile/Proto via regex (ponytail: tree-sitter).

Dispatcher: `internal/indexer/*` + `walk.go` (ExcludePaths, ChangedSet `--incremental` via `git diff`).

## Query

```bash
./cbmem index ~/code-storage/flight-search-system --json
# 655 sym 124 edges 41 files (incl docs/swagger.yaml)

./cbmem query Flight --repo ~/code-storage/flight-search-system | head
./cbmem callers api.SetupRouter --repo ~/code-storage/flight-search-system --depth 2 --json
./cbmem links ~/code-storage/flight-search-system
./cbmem stale ~/code-storage/flight-search-system
```

Lib for embedding: `pkg/cbmem/api.go` `Client{Search,Callers,Callees,Impact}`.

## Links (moat)

- Code: `// cbmem:link symbol=Foo target=knowledge/foo.md` (also `symbol: Foo`, bare tokens, `implements`/`documents` kinds auto).
- ADRs: frontmatter `evidence: [pkg.Foo, docs/bar.md]` under `knowledge/ decisions/ journal/ vault/`.
- `cbmem links` writes `links.jsonl` + `links` SQLite table with `content_hash`; `cbmem stale` flags drift/deleted.

## Caveats

- TS/Python/YAML etc are regex — good enough for 10k-line repos, not semantic.
- `routes.md` heuristic only (handler/echo/gin/http) — add `// cbmem:route` comments for explicit routes later.
- Needs `gcc` (CGO). `modernc.org/sqlite` was tried and dropped.
- No vector search yet — planned ONNX + sqlite-vec.

## Verify

```bash
go vet ./... && go build -o /tmp/cbmem . && /tmp/cbmem version
/tmp/cbmem index ~/code-storage/thio.dev --json
/tmp/cbmem index ~/code-storage/flight-search-system --json
ls ~/code-storage/second-brain/codebase/flight-search-system/
```

License: MIT.
