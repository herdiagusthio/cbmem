# cbmem CLI Reference

**Type:** Reference — all commands, flags, examples.

## Synopsis

```bash
cbmem <command> [args] [flags]
cbmem --help
```

## Commands

### `cbmem init <repo>`

Initialize `second-brain/codebase/<repo>/` with `symbols.db` + meta.

```bash
cbmem init /path/to/repo
cbmem init . --json
```

### `cbmem index <repo> [flags]`

Index repo: walk → dispatcher (7 langs) → upsert symbols/edges → artifacts + meta.

**Flags:** `--incremental` `--json`

```bash
cbmem index /path/to/repo --json          # 655 sym 124 edges 41 files
cbmem index /path/to/repo --incremental   # only git-changed files via ChangedSet
```

**Output (`--json`):** `{repo, indexed_symbols, indexed_edges, indexed_files, output_dir}`

**Artifacts written:** `second-brain/codebase/<repo>/symbols.db map.md routes.md callgraph.dot notes.md links.jsonl`

### `cbmem query <term> --repo <repo>`

Search symbols via LIKE + FTS5 bm25 ranking.

```bash
cbmem query Handler --repo /path/to/repo
cbmem query Flight --repo ~/code-storage/flight-search-system | head
```

**Output:** `qualified_name  file:line  kind  signature` per line.

### `cbmem callers <qualifiedName> [--depth 1] [--json] --repo <repo>`

Recursive callers via CTE (who calls this symbol, depth-N transitive).

```bash
cbmem callers "flight.*FlightHandler.HandleSearch" --depth 2 --repo /path/to/repo
cbmem callers "domain.NewSearchResponse" --json --repo /path/to/repo
```

### `cbmem callees <qualifiedName> [--depth 1] [--json] --repo <repo>`

Recursive callees via CTE (what this symbol calls).

```bash
cbmem callees "usecase.*flightSearchUseCase.Search" --repo /path/to/repo
```

### `cbmem impact <qualifiedName> [--depth 3] [--json] --repo <repo>`

T2.4 — callers + callees + auto-classify `routes_affected`/`tests_affected`.

```bash
cbmem impact "flight.*FlightHandler.HandleSearch" --depth 3 --repo /path/to/repo --json
# -> {"symbol":...,"callers":0,"callees":36,"routes_affected":1,"tests_affected":0}
```

**Library:** `pkg/cbmem.Client.GetImpact(ctx, qualified, depth) *ImpactResult` where `ImpactResult{Symbol, Callers, Callees, RoutesAffected, TestsAffected}`.

### `cbmem links [repo] [--json]`

Build `links.jsonl` from 3 sources:
- `// cbmem:link symbol target` in code files
- ADR frontmatter `evidence:` (knowledge/decisions/journal/vault md)
- `<!-- cbmem:symbol=... -->` in any md under `second-brain/`

```bash
cbmem links /path/to/repo                  # 0 code + 0 adr + 3 notes -> links.jsonl
cbmem links /path/to/repo --json
```

### `cbmem stale [repo] [--json]`

Report drifted/deleted links: hash mismatch or symbol deleted since `links.jsonl` was written.

```bash
cbmem stale /path/to/repo --json
# -> []  (no stale)  or  [{"symbol":...,"target_path":...,"link_kind":...}, ...]
```

### `cbmem version`

Print version: `cbmem v0.1.0-dev (Go+TS+Python+SQL+YAML+Docker+Proto, SQLite FTS5, linker)`.

## Global Flags

- `--json` — machine-readable output (index, links, stale, impact, callers/callees)
- `--incremental` — re-index only `git diff HEAD` changed files (index only)
- `--repo PATH` — repo path (default `.`)

## Common Pitfalls

- **No git repo:** `cbmem index` falls back to full scan (no ChangedSet)
- **`mattn/go-sqlite3`:** requires gcc/CGO (`go build` fails with `gcc not found` → `apt install gcc`)
- **No symbol:** `cbmem callers` with unknown qual returns `no callers` (not error)
- **Routes.md empty:** YAML/Docker/Proto always skipped (heuristic only detects handler/route)

## Verification

```bash
./cbmem index /tmp/repo --json && cat ~/code-storage/second-brain/codebase/<repo>/map.md | head -20
./cbmem query Search --repo /tmp/repo | head
./cbmem impact "main.Func" --repo /tmp/repo --json | jq .callees[].QualifiedName
```