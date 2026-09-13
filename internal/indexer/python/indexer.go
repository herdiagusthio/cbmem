package python

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"github.com/herdiagusthio/cbmem/internal/hasher"
	"github.com/herdiagusthio/cbmem/internal/indexer"
	"github.com/herdiagusthio/cbmem/internal/storage"
)

type Indexer struct{}

func New() *Indexer { return &Indexer{} }
func (g *Indexer) Language() string     { return "python" }
func (g *Indexer) Extensions() []string { return []string{".py", ".pyi"} }

var (
	reDef   = regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)\s*\(`)
	reClass = regexp.MustCompile(`^\s*class\s+(\w+)\s*[\(:]`)
)

func (g *Indexer) IndexFile(repo, filePath string, content []byte) (*indexer.Result, error) {
	pkg := pkgFromPath(filePath)
	res := &indexer.Result{}
	hash := hasher.ContentHash(content)

	s := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := s.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "\"\"\"") || strings.HasPrefix(trimmed, "'''") {
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			continue
		}
		var name, kind string
		if m := reDef.FindStringSubmatch(line); m != nil {
			name = m[1]
			// method vs function: indent heuristic
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
				kind = "method"
			} else {
				kind = "function"
			}
		} else if m := reClass.FindStringSubmatch(line); m != nil {
			name = m[1]
			kind = "struct"
		} else {
			continue
		}
		if name == "" || name == "_" {
			continue
		}
		qual := pkg + "." + name
		if pkg == "" {
			qual = name
		}
		sig := strings.TrimSpace(line)
		if len(sig) > 200 {
			sig = sig[:200]
		}
		res.Symbols = append(res.Symbols, &storage.Symbol{
			Repo: repo, FilePath: filePath, SymbolKind: kind, Name: name,
			QualifiedName: qual, Signature: sig, Language: "python",
			StartLine: lineNum, EndLine: lineNum, ContentHash: hash,
		})
	}
	return res, nil
}

func pkgFromPath(p string) string {
	p = strings.TrimSuffix(p, ".py")
	p = strings.TrimSuffix(p, ".pyi")
	parts := strings.Split(p, "/")
	if len(parts) > 0 && parts[0] == "src" {
		parts = parts[1:]
	}
	return strings.Join(parts, ".")
}
