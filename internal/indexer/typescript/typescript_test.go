package typescript

import "testing"

func TestSimpleTS(t *testing.T) {
    src := `
    export function add(a: number, b: number): number { return a + b }
    `
    idx := New()
    _, err := idx.IndexFile("ts-repo", "add.ts", []byte(src))
    if err != nil {
        t.Fatalf("Index error: %v", err)
    }
}

