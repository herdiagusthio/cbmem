package golang

import "testing"

func TestIndexer_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKinds []string
		wantCount int
	}{
		{
			name: "simple function",
			input: "package main\nfunc Hello() string { return \"hi\" }\n",
			wantKinds: []string{"function"},
			wantCount: 1,
		},
		{
			name: "struct definition",
			input: "package main\ntype User struct { Name string }\n",
			wantKinds: []string{"struct"},
			wantCount: 1,
		},
		{
			name: "interface definition",
			input: "package main\ntype Runner interface { Run() }\n",
			wantKinds: []string{"interface"},
			wantCount: 1,
		},
		{
			name: "method on struct",
			input: "package main\ntype Handler struct{}\nfunc (h *Handler) Serve() error { return nil }\n",
			wantKinds: []string{"struct", "method"},
			wantCount: 2,
		},
		{
			name: "const and var",
			input: "package main\nconst DefaultPort = 8080\nvar timeout = 30\n",
			wantKinds: []string{"const", "var"},
			wantCount: 2,
		},
	}

	idx := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := idx.IndexFile("test-repo", "test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("IndexFile error: %v", err)
			}
			if len(result.Symbols) != tt.wantCount {
				t.Errorf("got %d symbols, want %d", len(result.Symbols), tt.wantCount)
			}
			kindCount := make(map[string]int)
			for _, s := range result.Symbols {
				kindCount[s.SymbolKind]++
			}
			for _, k := range tt.wantKinds {
				if kindCount[k] == 0 {
					t.Errorf("expected kind %s not found in %v", k, kindCount)
				}
			}
		})
	}
}

func TestIndexer_ContentHash(t *testing.T) {
	idx := New()
	content := []byte("package main\nfunc Same() {}\n")
	result, _ := idx.IndexFile("repo", "file.go", content)
	if len(result.Symbols) == 0 {
		t.Fatal("no symbols")
	}
	for _, s := range result.Symbols {
		if s.ContentHash == "" {
			t.Error("empty content hash")
		}
		if len(s.ContentHash) != 64 {
			t.Errorf("hash len %d, want 64: %q", len(s.ContentHash), s.ContentHash)
		}
	}
}

func TestIndexer_QualifiedNames(t *testing.T) {
	idx := New()
	src := "package main\ntype Service struct{}\nfunc (s *Service) DoWork() { }\nfunc main() { }\n"
	result, _ := idx.IndexFile("r", "main.go", []byte(src))
	names := make(map[string]bool)
	for _, s := range result.Symbols {
		names[s.QualifiedName] = true
	}
	expected := []string{"main.Service", "main.*Service.DoWork", "main.main"}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing qualified name %q, got %v", e, names)
		}
	}
}

func TestIndexer_SymbolFields(t *testing.T) {
	idx := New()
	result, _ := idx.IndexFile("repo", "file.go", []byte("package main\nfunc F() {}"))
	if len(result.Symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result.Symbols))
	}
	s := result.Symbols[0]
	if s.Repo != "repo" {
		t.Errorf("repo: got %q, want %q", s.Repo, "repo")
	}
	if s.Language != "go" {
		t.Errorf("language: got %q, want %q", s.Language, "go")
	}
	if s.FilePath != "file.go" {
		t.Errorf("filepath: got %q, want %q", s.FilePath, "file.go")
	}
	if s.SymbolKind != "function" {
		t.Errorf("kind: got %q, want %q", s.SymbolKind, "function")
	}
}

func BenchmarkIndexer(b *testing.B) {
	src := "package bench\ntype LargeStruct struct {\nField1 string\nField2 int\nField3 bool\n}\nfunc (l *LargeStruct) Method1() {}\nfunc LargeFunc1() string { return \"a\" }\nvar GlobalVar = 42\nconst GlobalConst = \"test\"\n"
	idx := New()
	data := []byte(src)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.IndexFile("bench-repo", "bench.go", data)
	}
}
