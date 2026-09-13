package yamlidx

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
func (g *Indexer) Language() string     { return "yaml" }
func (g *Indexer) Extensions() []string { return []string{".yaml", ".yml"} }

var reKey = regexp.MustCompile(`^\s*([A-Za-z_][\w.\-]*)\s*:`)

func (g *Indexer) IndexFile(repo, filePath string, content []byte) (*indexer.Result, error) {
	res := &indexer.Result{}
	hash := hasher.ContentHash(content)
	pkg := strings.TrimSuffix(filePath, ".yaml")
	pkg = strings.TrimSuffix(pkg, ".yml")
	pkg = strings.ReplaceAll(pkg, "/", ".")
	seen := make(map[string]bool)

	s := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := s.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		m := reKey.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if name == "" || seen[name+":"+string(rune(lineNum))] {
			// allow duplicate keys at different lines by line suffix
		}
		// dedupe exact line key to avoid noise from same key repeated; use line-qualified
		qual := pkg + "." + name
		// if already emitted same qual, make unique via line number
		if seen[qual] {
			qual = pkg + "." + name + "_" + itoa(lineNum)
		}
		seen[qual] = true
		seen[name+":"+string(rune(lineNum))] = true // dummy
		sig := trim
		if len(sig) > 120 {
			sig = sig[:120]
		}
		res.Symbols = append(res.Symbols, &storage.Symbol{
			Repo: repo, FilePath: filePath, SymbolKind: "field", Name: name,
			QualifiedName: qual, Signature: sig, Language: "yaml",
			StartLine: lineNum, EndLine: lineNum, ContentHash: hash,
		})
	}
	return res, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 8)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
