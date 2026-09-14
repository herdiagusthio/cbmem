//go:build !wasm

package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var migrations = []struct {
	version int
	name    string
	sql     string
}{
	{
		version: 1,
		name:    "initial_schema",
		sql: `CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY,
    repo TEXT NOT NULL,
    file_path TEXT NOT NULL,
    symbol_kind TEXT NOT NULL,
    name TEXT NOT NULL,
    qualified_name TEXT NOT NULL,
    signature TEXT,
    language TEXT NOT NULL,
    start_line INTEGER,
    end_line INTEGER,
    content_hash TEXT NOT NULL,
    ast_json TEXT,
    embedding BLOB,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now'))
);
CREATE TABLE IF NOT EXISTS edges (
    id INTEGER PRIMARY KEY,
    src_symbol_id INTEGER REFERENCES symbols(id),
    dst_symbol_id INTEGER REFERENCES symbols(id),
    edge_kind TEXT NOT NULL,
    confidence REAL DEFAULT 1.0
);
CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY,
    symbol_id INTEGER REFERENCES symbols(id),
    target_type TEXT NOT NULL,
    target_path TEXT NOT NULL,
    link_kind TEXT NOT NULL,
    created_at INTEGER DEFAULT (strftime('%s', 'now'))
);
CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(qualified_name, name, signature, file_path, content='symbols', content_rowid='id');
CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN INSERT INTO symbols_fts(rowid, qualified_name, name, signature, file_path) VALUES (new.id, new.qualified_name, new.name, new.signature, new.file_path); END;
CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, qualified_name, name, signature, file_path) VALUES ('delete', old.id, old.qualified_name, old.name, old.signature, old.file_path); END;
CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN INSERT INTO symbols_fts(symbols_fts, rowid, qualified_name, name, signature, file_path) VALUES ('delete', old.id, old.qualified_name, old.name, old.signature, old.file_path); INSERT INTO symbols_fts(rowid, qualified_name, name, signature, file_path) VALUES (new.id, new.qualified_name, new.name, new.signature, new.file_path); END;
CREATE INDEX IF NOT EXISTS idx_symbols_repo ON symbols(repo);
CREATE INDEX IF NOT EXISTS idx_symbols_qualified_name ON symbols(qualified_name);
CREATE INDEX IF NOT EXISTS idx_symbols_file_path ON symbols(file_path);
CREATE INDEX IF NOT EXISTS idx_symbols_content_hash ON symbols(content_hash);
CREATE INDEX IF NOT EXISTS idx_edges_src ON edges(src_symbol_id);
CREATE INDEX IF NOT EXISTS idx_edges_dst ON edges(dst_symbol_id);
CREATE INDEX IF NOT EXISTS idx_links_symbol ON links(symbol_id);
CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_type, target_path);`},
}

type Store struct{ db *sql.DB }

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate() error {
	s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at INTEGER DEFAULT (strftime('%s', 'now')))`)
	applied := make(map[int]bool)
	rows, _ := s.db.Query("SELECT version FROM schema_migrations")
	for rows.Next() {
		var v int
		rows.Scan(&v)
		applied[v] = true
	}
	rows.Close()
	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		s.db.Exec(m.sql)
		s.db.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.name)
	}
	return nil
}

func (s *Store) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}
func (s *Store) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}
func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *Store) UpsertSymbol(ctx context.Context, sym *Symbol) error {
	var id int64
	err := s.QueryRowContext(ctx, "SELECT id FROM symbols WHERE repo=? AND qualified_name=? AND file_path=?", sym.Repo, sym.QualifiedName, sym.FilePath).Scan(&id)
	if err == sql.ErrNoRows {
		result, e := s.ExecContext(ctx, `INSERT INTO symbols (repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sym.Repo, sym.FilePath, sym.SymbolKind, sym.Name, sym.QualifiedName, sym.Signature, sym.Language, sym.StartLine, sym.EndLine, sym.ContentHash, sym.ASTJSON, sym.Embedding)
		if e != nil {
			return e
		}
		id, _ = result.LastInsertId()
		sym.ID = id
	} else if err != nil {
		return err
	} else {
		s.ExecContext(ctx, `UPDATE symbols SET symbol_kind=?, name=?, qualified_name=?, signature=?, language=?, start_line=?, end_line=?, content_hash=?, ast_json=?, embedding=?, updated_at=strftime('%s', 'now') WHERE id=?`,
			sym.SymbolKind, sym.Name, sym.QualifiedName, sym.Signature, sym.Language, sym.StartLine, sym.EndLine, sym.ContentHash, sym.ASTJSON, sym.Embedding, id)
		sym.ID = id
	}
	return nil
}

func (s *Store) UpsertEdge(ctx context.Context, edge *Edge) error {
	var id int64
	err := s.QueryRowContext(ctx, "SELECT id FROM edges WHERE src_symbol_id=? AND dst_symbol_id=? AND edge_kind=?", edge.SrcSymbolID, edge.DstSymbolID, edge.EdgeKind).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = s.ExecContext(ctx, `INSERT INTO edges (src_symbol_id, dst_symbol_id, edge_kind, confidence) VALUES (?, ?, ?, ?)`, edge.SrcSymbolID, edge.DstSymbolID, edge.EdgeKind, edge.Confidence)
		return err
	}
	return err
}

func (s *Store) UpsertLink(ctx context.Context, link *Link) error {
	_, err := s.ExecContext(ctx, `INSERT INTO links (symbol_id, target_type, target_path, link_kind) VALUES (?, ?, ?, ?)`, link.SymbolID, link.TargetType, link.TargetPath, link.LinkKind)
	return err
}

func (s *Store) GetSymbolByQualifiedName(ctx context.Context, repo, qualifiedName string) (*Symbol, error) {
	row := s.QueryRowContext(ctx, `SELECT id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding FROM symbols WHERE repo=? AND qualified_name=?`, repo, qualifiedName)
	var sym Symbol
	err := row.Scan(&sym.ID, &sym.Repo, &sym.FilePath, &sym.SymbolKind, &sym.Name, &sym.QualifiedName, &sym.Signature, &sym.Language, &sym.StartLine, &sym.EndLine, &sym.ContentHash, &sym.ASTJSON, &sym.Embedding)
	if err != nil {
		return nil, err
	}
	return &sym, nil
}

func (s *Store) SearchSymbols(ctx context.Context, repo, query string, limit int) ([]*Symbol, error) {
	likeQuery := "%" + query + "%"
	rows, err := s.QueryContext(ctx, `SELECT id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding FROM symbols WHERE repo=? AND (qualified_name LIKE ? OR name LIKE ? OR signature LIKE ? OR file_path LIKE ?) ORDER BY CASE WHEN qualified_name = ? THEN 0 WHEN qualified_name LIKE ? THEN 1 WHEN name = ? THEN 2 ELSE 3 END, name LIMIT ?`,
		repo, likeQuery, likeQuery, likeQuery, likeQuery, query, query+"%", query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.ID, &sym.Repo, &sym.FilePath, &sym.SymbolKind, &sym.Name, &sym.QualifiedName, &sym.Signature, &sym.Language, &sym.StartLine, &sym.EndLine, &sym.ContentHash, &sym.ASTJSON, &sym.Embedding); err != nil {
			return nil, err
		}
		results = append(results, &sym)
	}
	return results, nil
}

func (s *Store) GetCallers(ctx context.Context, repo, qualifiedName string, depth int) ([]*Symbol, error) {
	if depth <= 0 {
		depth = 1
	}
	rows, err := s.QueryContext(ctx, `WITH RECURSIVE callers(id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding, level) AS (
			SELECT s.id, s.repo, s.file_path, s.symbol_kind, s.name, s.qualified_name, s.signature, s.language, s.start_line, s.end_line, s.content_hash, s.ast_json, s.embedding, 1 FROM symbols s JOIN edges e ON s.id = e.src_symbol_id JOIN symbols t ON e.dst_symbol_id = t.id WHERE t.repo = ? AND t.qualified_name = ? AND e.edge_kind = 'calls'
			UNION ALL
			SELECT s.id, s.repo, s.file_path, s.symbol_kind, s.name, s.qualified_name, s.signature, s.language, s.start_line, s.end_line, s.content_hash, s.ast_json, s.embedding, c.level + 1 FROM symbols s JOIN edges e ON s.id = e.src_symbol_id JOIN callers c ON e.dst_symbol_id = c.id WHERE e.edge_kind = 'calls' AND c.level < ?
		) SELECT DISTINCT id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding FROM callers ORDER BY level, qualified_name`, repo, qualifiedName, depth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.ID, &sym.Repo, &sym.FilePath, &sym.SymbolKind, &sym.Name, &sym.QualifiedName, &sym.Signature, &sym.Language, &sym.StartLine, &sym.EndLine, &sym.ContentHash, &sym.ASTJSON, &sym.Embedding); err != nil {
			return nil, err
		}
		results = append(results, &sym)
	}
	return results, nil
}

func (s *Store) GetCallees(ctx context.Context, repo, qualifiedName string, depth int) ([]*Symbol, error) {
	if depth <= 0 {
		depth = 1
	}
	rows, err := s.QueryContext(ctx, `WITH RECURSIVE callees(id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding, level) AS (
			SELECT s.id, s.repo, s.file_path, s.symbol_kind, s.name, s.qualified_name, s.signature, s.language, s.start_line, s.end_line, s.content_hash, s.ast_json, s.embedding, 1 FROM symbols s JOIN edges e ON s.id = e.dst_symbol_id JOIN symbols t ON e.src_symbol_id = t.id WHERE t.repo = ? AND t.qualified_name = ? AND e.edge_kind = 'calls'
			UNION ALL
			SELECT s.id, s.repo, s.file_path, s.symbol_kind, s.name, s.qualified_name, s.signature, s.language, s.start_line, s.end_line, s.content_hash, s.ast_json, s.embedding, c.level + 1 FROM symbols s JOIN edges e ON s.id = e.dst_symbol_id JOIN callees c ON e.src_symbol_id = c.id WHERE e.edge_kind = 'calls' AND c.level < ?
		) SELECT DISTINCT id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding FROM callees ORDER BY level, qualified_name`, repo, qualifiedName, depth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.ID, &sym.Repo, &sym.FilePath, &sym.SymbolKind, &sym.Name, &sym.QualifiedName, &sym.Signature, &sym.Language, &sym.StartLine, &sym.EndLine, &sym.ContentHash, &sym.ASTJSON, &sym.Embedding); err != nil {
			return nil, err
		}
		results = append(results, &sym)
	}
	return results, nil
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
func (s *Store) GetMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&value)
	return value, err
}

func (s *Store) ListSymbols(ctx context.Context, repo string, limit int) ([]*Symbol, error) {
	q := `SELECT id, repo, file_path, symbol_kind, name, qualified_name, signature, language, start_line, end_line, content_hash, ast_json, embedding FROM symbols WHERE repo=? ORDER BY file_path, start_line`
	args := []any{repo}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.ID, &sym.Repo, &sym.FilePath, &sym.SymbolKind, &sym.Name, &sym.QualifiedName, &sym.Signature, &sym.Language, &sym.StartLine, &sym.EndLine, &sym.ContentHash, &sym.ASTJSON, &sym.Embedding); err != nil {
			return nil, err
		}
		out = append(out, &sym)
	}
	return out, nil
}

func (s *Store) CountSymbols(ctx context.Context, repo string) (int, error) {
	var n int
	err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM symbols WHERE repo=?`, repo).Scan(&n)
	return n, err
}

func (s *Store) ListEdges(ctx context.Context, repo string) ([]EdgeWithNames, error) {
	rows, err := s.QueryContext(ctx, `SELECT e.src_symbol_id, e.dst_symbol_id, e.edge_kind, e.confidence, s.qualified_name, t.qualified_name FROM edges e JOIN symbols s ON s.id=e.src_symbol_id JOIN symbols t ON t.id=e.dst_symbol_id WHERE s.repo=? AND t.repo=?`, repo, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgeWithNames
	for rows.Next() {
		var e EdgeWithNames
		if err := rows.Scan(&e.SrcSymbolID, &e.DstSymbolID, &e.EdgeKind, &e.Confidence, &e.SrcQualified, &e.DstQualified); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

type Symbol struct {
	ID            int64
	Repo          string
	FilePath      string
	SymbolKind    string
	Name          string
	QualifiedName string
	Signature     string
	Language      string
	StartLine     int
	EndLine       int
	ContentHash   string
	ASTJSON       string
	Embedding     []byte
}

type Edge struct {
	SrcSymbolID int64
	DstSymbolID int64
	EdgeKind    string
	Confidence  float64
}

type EdgeWithNames struct {
	Edge
	SrcQualified string
	DstQualified string
}

type Link struct {
	SymbolID   int64
	TargetType string
	TargetPath string
	LinkKind   string
}