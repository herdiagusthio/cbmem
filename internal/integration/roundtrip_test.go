//go:build !wasm

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	golang "github.com/herdiagusthio/cbmem/internal/indexer/golang"
	"github.com/herdiagusthio/cbmem/internal/linker"
	"github.com/herdiagusthio/cbmem/internal/storage"
)

func resolveCallee(qualMap map[string]*storage.Symbol, calleeName string) *storage.Symbol {
	if c, ok := qualMap[calleeName]; ok {
		return c
	}
	last := calleeName
	if idx := strings.LastIndex(calleeName, "."); idx >= 0 {
		last = calleeName[idx+1:]
	}
	for q, sym := range qualMap {
		if q == calleeName || strings.HasSuffix(q, "."+last) || strings.HasSuffix(q, "."+calleeName) || q == last {
			return sym
		}
	}
	for q, sym := range qualMap {
		if strings.Contains(q, last) {
			return sym
		}
	}
	return nil
}

func TestRoundtrip_IndexQueryImpactLink(t *testing.T) {
	ctx := context.Background()
	repo := "test-repo"
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "symbols.db")
	outDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	src := "package demo\n\nfunc Helper() string { return \"hi\" }\nfunc DoWork() string { return Helper() }\nfunc Standalone() int { return 42 }\n// cbmem:link symbol=demo.DoWork target=docs/adr-001.md kind=documents\n"
	filePath := "demo/service.go"
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, filePath), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module demo\n"), 0644)

	idx := golang.New()
	res, err := idx.IndexFile(repo, filePath, []byte(src))
	if err != nil || res == nil {
		t.Fatalf("IndexFile: %v", err)
	}
	if len(res.Symbols) < 3 {
		t.Fatalf("expected >=3 symbols, got %d: %+v", len(res.Symbols), res.Symbols)
	}

	qualMap := map[string]*storage.Symbol{}
	for _, s := range res.Symbols {
		if err := store.UpsertSymbol(ctx, s); err != nil {
			t.Fatalf("UpsertSymbol %s: %v", s.QualifiedName, err)
		}
		qualMap[s.QualifiedName] = s
	}
	for _, rc := range res.RawCalls {
		caller, ok := qualMap[rc.CallerQualified]
		if !ok {
			continue
		}
		callee := resolveCallee(qualMap, rc.CalleeName)
		if callee == nil {
			continue
		}
		_ = store.UpsertEdge(ctx, &storage.Edge{SrcSymbolID: caller.ID, DstSymbolID: callee.ID, EdgeKind: "calls", Confidence: 1.0})
	}

	t.Run("query", func(t *testing.T) {
		results, err := store.SearchSymbols(ctx, repo, "DoWork", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Fatal("SearchSymbols DoWork: no results")
		}
		found := false
		for _, r := range results {
			if r.QualifiedName == "demo.DoWork" {
				found = true
			}
		}
		if !found {
			t.Errorf("demo.DoWork not found in %+v", results)
		}
	})

	t.Run("graph", func(t *testing.T) {
		callees, err := store.GetCallees(ctx, repo, "demo.DoWork", 2)
		if err != nil {
			t.Fatal(err)
		}
		hasHelper := false
		for _, c := range callees {
			if c.QualifiedName == "demo.Helper" {
				hasHelper = true
			}
		}
		if !hasHelper {
			t.Errorf("expected callee demo.Helper in %v", callees)
		}
		callers, err := store.GetCallers(ctx, repo, "demo.Helper", 2)
		if err != nil {
			t.Fatal(err)
		}
		hasCaller := false
		for _, c := range callers {
			if c.QualifiedName == "demo.DoWork" {
				hasCaller = true
			}
		}
		if !hasCaller {
			t.Errorf("expected caller demo.DoWork in %v", callers)
		}
	})

	t.Run("impact", func(t *testing.T) {
		imp, err := store.GetImpact(ctx, repo, "demo.Helper", 3)
		if err != nil {
			t.Fatal(err)
		}
		if imp.Symbol == nil {
			t.Fatal("impact symbol nil")
		}
		if len(imp.Callers) == 0 {
			t.Errorf("impact callers empty, want >=1: %+v", imp)
		}
	})

	t.Run("links", func(t *testing.T) {
		links, err := linker.ParseCodeLinks(repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		if len(links) == 0 {
			t.Fatal("ParseCodeLinks: no links")
		}
		if err := linker.WriteJSONL(ctx, outDir, links, store, repo); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(outDir, "links.jsonl")); err != nil {
			t.Fatal(err)
		}
		stale, err := linker.StaleCheck(ctx, outDir, store, repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(stale) != 0 {
			t.Errorf("expected no stale, got %d: %+v", len(stale), stale)
		}
		first := links[0]
		sym, err := store.GetSymbolByQualifiedName(ctx, repo, first.Symbol)
		if err != nil || sym == nil {
			syms, _ := store.SearchSymbols(ctx, repo, first.Symbol, 1)
			if len(syms) > 0 {
				sym = syms[0]
			}
		}
		if sym != nil {
			sym.ContentHash = "deadbeef"
			_ = store.UpsertSymbol(ctx, sym)
			stale2, _ := linker.StaleCheck(ctx, outDir, store, repo)
			if len(stale2) == 0 {
				t.Error("expected stale after hash drift")
			}
		}
	})
}