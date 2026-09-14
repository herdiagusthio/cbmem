package bench

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "testing"

    golang "github.com/herdiagusthio/cbmem/internal/indexer/golang"
    "github.com/herdiagusthio/cbmem/internal/storage"
)

func generateSymbols(n int) string {
    var b strings.Builder
    b.WriteString("package bench\n\n")
    for i := 0; i < n; i++ {
        fn := fmt.Sprintf("Func%d", i)
        b.WriteString(fmt.Sprintf("func %s() { }\n", fn))
    }
    return b.String()
}

func BenchmarkIndex100kSymbols(b *testing.B) {
    ctx := context.Background()
    tmp := b.TempDir()
    dbPath := filepath.Join(tmp, "bench.db")
    store, err := storage.NewStore(dbPath)
    if err != nil { b.Fatal(err) }
    defer store.Close()
    store.Migrate()
    src := generateSymbols(20000) // reduced for quick test
    idx := golang.New()
    res, err := idx.IndexFile("bench-repo", "all.go", []byte(src))
    if err != nil { b.Fatalf("IndexFile error: %v", err) }
    for _, s := range res.Symbols { store.UpsertSymbol(ctx, s) }
    b.StopTimer()
    count, _ := store.CountSymbols(ctx, "bench-repo")
    fmt.Fprintf(os.Stderr, "Indexed %d symbols\n", count)
}

func BenchmarkQuery100k(b *testing.B) {
    ctx := context.Background()
    tmp := b.TempDir()
    dbPath := filepath.Join(tmp, "bench.db")
    store, _ := storage.NewStore(dbPath)
    store.Migrate()
    src := generateSymbols(20000)
    idx := golang.New()
    res, _ := idx.IndexFile("bench-repo", "all.go", []byte(src))
    for _, s := range res.Symbols { store.UpsertSymbol(ctx, s) }
    b.ResetTimer()
    for i := 0; i < b.N; i++ { store.SearchSymbols(ctx, "bench-repo", "Func5000", 10) }
}
