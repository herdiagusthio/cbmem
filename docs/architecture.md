# cbmem Architecture

**Type:** Reference — system architecture, component relationships.

## Overview

cbmem is a local-first codebase indexer that transforms any repository into a queryable second-brain artifact.

```
User → `cbmem index <repo>` → `second-brain/codebase/<repo>/`
           │
           ├─> walk → 7-language dispatcher
           │              ├─> golang → symbols.json
           │              ├─> typescript → symbols.json
           │              ├─> python → symbols.json
           │              ├─> sql → symbols.json
           │              ├─> yaml → symbols.json
           │              ├─> docker → symbols.json
           │              └─> proto → symbols.json
           │
           ├─> upsert → SQLite (symbols, edges, links, meta)
           │
           ├─> artifacts → map.md routes.md callgraph.dot notes.md
           │
           └─> links.jsonl ← `// cbmem:link` + ADR `evidence:` + `<!-- cbmem:symbol=... -->`
```

## Core Components

| Package | Purpose |
|---------|---------|
| `internal/config` | YAML + env config (`OutputDir`, `ExcludePaths`, `Language`) |
| `internal/hasher` | SHA256 per symbol content hash |
| `internal/git` | `GetHeadCommit()`, `GetChangedFiles()` for incremental |
| `internal/storage` | SQLite with FTS5, edges CTE, links table, migrations |
| `internal/indexer` | Dispatcher → language-specific indexers |
| `internal/linker` | Parse links code + ADR + notes, stale check |
| `internal/artifacts` | Generate map.md + routes.md + callgraph.dot + notes.md |
| `pkg/cbmem` | Public Go library for skill/embedding |

## Data Model

### Symbols Table

```sql
CREATE TABLE symbols (
  id INTEGER PRIMARY KEY,
  repo TEXT NOT NULL,
  file_path TEXT NOT NULL,
  symbol_kind TEXT NOT NULL,   -- function, method, struct, interface, type, const, var, field
  name TEXT NOT NULL,
  qualified_name TEXT NOT NULL,
  signature TEXT,
  language TEXT NOT NULL,
  start_line INTEGER,
  end_line INTEGER,
  content_hash TEXT NOT NULL,
  ast_json TEXT,
  embedding BLOB
);
```

### Edges Table

```sql
CREATE TABLE edges (
  id INTEGER PRIMARY KEY,
  src_symbol_id INTEGER REFERENCES symbols(id),
  dst_symbol_id INTEGER REFERENCES symbols(id),
  edge_kind TEXT NOT NULL,    -- calls, imports, references, implements, embeds, extends
  confidence REAL DEFAULT 1.0
);
```

### Links Table (Bidirectional)

```sql
CREATE TABLE links (
  id INTEGER PRIMARY KEY,
  symbol_id INTEGER REFERENCES symbols(id),
  target_type TEXT NOT NULL,  -- code, adr, decision, knowledge, note
  target_path TEXT NOT NULL,
  link_kind TEXT NOT NULL     -- documents, implements, tests, refactors, decides
);
```

## Language Indexers

| Language | File Types | Parser |
|----------|-----------|--------|
| Go | `.go` | `go/ast` + `go/types` |
| TypeScript | `.ts .tsx .js .jsx .astro .mjs .cjs` | regex |
| Python | `.py .pyi` | regex |
| SQL | `.sql` | regex |
| YAML | `.yaml .yml` | regex |
| Dockerfile | `Dockerfile` | regex |
| Protobuf | `.proto` | regex |

Ponytail: tree-sitter for semantic accuracy in future.

## Query Engine

- **FTS5**: `SELECT * FROM symbols_fts WHERE symbols_fts MATCH 'query'` — bm25 ranking
- **Callers**: Recursive CTE depth-N
- **Callees**: Recursive CTE depth-N
- **Impact**: Callers depth-N + callees + auto-classify routes/tests affected

## Artifacts

| File | Purpose | Update |
|------|---------|--------|
| `symbols.db` | SQLite primary store | `cbmem index` |
| `map.md` | Directory tree + key symbols | `cbmem index` |
| `routes.md` | Handler symbols → file:line | `cbmem index` |
| `callgraph.dot` | GraphViz export | `cbmem index` (trunc 200/500) |
| `notes.md` | Agent notes + staleness flags | `cbmem index` |
| `links.jsonl` | Symbol ↔ ADR ↔ note edges | `cbmem links` |

## Artifact JSONL Format

```json
{"symbol":"flight-search-system/domain.FlightFactory.NewFlight","file_path":"...","target_type":"note","target_path":"...","link_kind":"documents","symbol_id":123,"content_hash":"abc..."}
```