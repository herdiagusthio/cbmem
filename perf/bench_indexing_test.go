package perf

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "path/filepath"
    "testing"

    golang "github.com/herdiagusthio/cbmem/internal/indexer/golang"
    "github.com/herdiagusthio/cbmem/internal/storage"
)

func generateFunctionsFile(n int, pkg string) string {
    var buf bytes.Buffer
    buf.WriteString("package " + pkg + "\n\n")
    for i := 0; i < n; i++ {
        fn := fmt.Sprintf("Func%d", i)
        buf.WriteString(fmt.Sprintf("func %s() { }\n", fn))
    }
    return buf.String()
}

func BenchmarkLargeIndexing(b *testing.B) {
    ctx := context.Background()
    tmp := filepath.Join(b.TempDir(), "bigrepo")
    os.MkdirAll(tmp, 0755)
    store, err := storage.NewStore(filepath.Join(tmp, "symbols.db"))
    if err != nil { b.Fatal(err) }
    defer store.Close()
    store.Migrate()

    idx := golang.New()
    const perFile = 1000
    const totalFiles = 10
    for i := 0; i < totalFiles; i++ {
        pkg := fmt.Sprintf("pkg%d", i)
        src := generateFunctionsFile(perFile, pkg)
        filePath := filepath.Join(tmp, fmt.Sprintf("file%d.go", i))
        os.WriteFile(filePath, []byte(src), 0644)
        res, err := idx.IndexFile("bench-repo", filePath, []byte(src))
        if err != nil { b.Fatalf("IndexFile error: %v", err) }
        for _, s := range res.Symbols { store.UpsertSymbol(ctx, s) }
    }
    b.StopTimer()
    count, _ := store.CountSymbols(ctx, "bench-repo")
    fmt.Fprintf(os.Stderr, "Indexed %d symbols from %d files\n", count, totalFiles)
}

