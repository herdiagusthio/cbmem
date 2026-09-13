package typescript

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
func (g *Indexer) Language() string   { return "typescript" }
func (g *Indexer) Extensions() []string { return []string{".ts", ".tsx", ".js", ".jsx", ".astro", ".mjs", ".cjs"} }

var (
	reFunc      = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	reConstFn   = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(.*\)\s*=>`)
	reClass     = regexp.MustCompile(`^\s*(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	reInterface = regexp.MustCompile(`^\s*(?:export\s+)?interface\s+(\w+)`)
	reType      = regexp.MustCompile(`^\s*(?:export\s+)?type\s+(\w+)\s*=`)
	reEnum      = regexp.MustCompile(`^\s*(?:export\s+)?enum\s+(\w+)`)
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
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		var name, kind, sig string
		switch {
		case reFunc.MatchString(line):
			m := reFunc.FindStringSubmatch(line)
			name = m[1]
			kind = "function"
			sig = strings.TrimSpace(line)
		case reConstFn.MatchString(line):
			m := reConstFn.FindStringSubmatch(line)
			name = m[1]
			kind = "function"
			sig = strings.TrimSpace(line)
		case reClass.MatchString(line):
			m := reClass.FindStringSubmatch(line)
			name = m[1]
			kind = "struct"
			sig = strings.TrimSpace(line)
		case reInterface.MatchString(line):
			m := reInterface.FindStringSubmatch(line)
			name = m[1]
			kind = "interface"
			sig = strings.TrimSpace(line)
		case reType.MatchString(line):
			m := reType.FindStringSubmatch(line)
			name = m[1]
			kind = "type"
			sig = strings.TrimSpace(line)
		case reEnum.MatchString(line):
			m := reEnum.FindStringSubmatch(line)
			name = m[1]
			kind = "type"
			sig = strings.TrimSpace(line)
		default:
			continue
		}
		qual := pkg + "." + name
		if pkg == "" {
			qual = name
		}
		res.Symbols = append(res.Symbols, &storage.Symbol{
			Repo: repo, FilePath: filePath, SymbolKind: kind, Name: name,
			QualifiedName: qual, Signature: sig, Language: "typescript",
			StartLine: lineNum, EndLine: lineNum, ContentHash: hash,
		})
	}
	return res, nil
}

func pkgFromPath(p string) string {
	// use dir + file base as pkg hint, e.g. src/components/Button.tsx -> components.Button
	p = strings.TrimSuffix(p, ".ts")
	p = strings.TrimSuffix(p, ".tsx")
	p = strings.TrimSuffix(p, ".js")
	p = strings.TrimSuffix(p, ".jsx")
	p = strings.TrimSuffix(p, ".astro")
	p = strings.TrimSuffix(p, ".mjs")
	parts := strings.Split(p, "/")
	// drop leading src/
	if len(parts) > 0 && parts[0] == "src" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return ""
	}
	// last part is filename, keep it
	return strings.Join(parts, ".")
}
