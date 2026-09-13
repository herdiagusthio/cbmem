package sqlindex

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
func (g *Indexer) Language() string     { return "sql" }
func (g *Indexer) Extensions() []string { return []string{".sql"} }

var (
	reCreateTable = regexp.MustCompile(`(?i)CREATE\s+(?:TEMP(?:ORARY)?\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["` + "`" + `]?(\w+)["` + "`" + `]?`)
	reCreateView  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?VIEW\s+["` + "`" + `]?(\w+)["` + "`" + `]?`)
	reCreateFunc  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+["` + "`" + `]?(\w+)["` + "`" + `]?`)
	reCreateProc  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?PROCEDURE\s+["` + "`" + `]?(\w+)["` + "`" + `]?`)
	reCreateIndex = regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?["` + "`" + `]?(\w+)["` + "`" + `]?`)
)

func (g *Indexer) IndexFile(repo, filePath string, content []byte) (*indexer.Result, error) {
	res := &indexer.Result{}
	hash := hasher.ContentHash(content)
	pkg := strings.TrimSuffix(filePath, ".sql")
	pkg = strings.ReplaceAll(pkg, "/", ".")

	s := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := s.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		var name, kind string
		switch {
		case reCreateTable.MatchString(line):
			name = reCreateTable.FindStringSubmatch(line)[1]
			kind = "struct"
		case reCreateView.MatchString(line):
			name = reCreateView.FindStringSubmatch(line)[1]
			kind = "type"
		case reCreateFunc.MatchString(line):
			name = reCreateFunc.FindStringSubmatch(line)[1]
			kind = "function"
		case reCreateProc.MatchString(line):
			name = reCreateProc.FindStringSubmatch(line)[1]
			kind = "function"
		case reCreateIndex.MatchString(line):
			name = reCreateIndex.FindStringSubmatch(line)[1]
			kind = "const"
		default:
			continue
		}
		qual := pkg + "." + name
		sig := strings.TrimSpace(line)
		if len(sig) > 200 {
			sig = sig[:200]
		}
		res.Symbols = append(res.Symbols, &storage.Symbol{
			Repo: repo, FilePath: filePath, SymbolKind: kind, Name: name,
			QualifiedName: qual, Signature: sig, Language: "sql",
			StartLine: lineNum, EndLine: lineNum, ContentHash: hash,
		})
	}
	return res, nil
}
