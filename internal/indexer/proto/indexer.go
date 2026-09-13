package protoidx

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
func (g *Indexer) Language() string     { return "proto" }
func (g *Indexer) Extensions() []string { return []string{".proto"} }

var (
	reMessage = regexp.MustCompile(`^\s*message\s+(\w+)`)
	reService = regexp.MustCompile(`^\s*service\s+(\w+)`)
	reRpc     = regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\(`)
	reEnum    = regexp.MustCompile(`^\s*enum\s+(\w+)`)
)

func (g *Indexer) IndexFile(repo, filePath string, content []byte) (*indexer.Result, error) {
	res := &indexer.Result{}
	hash := hasher.ContentHash(content)
	pkg := strings.TrimSuffix(filePath, ".proto")
	pkg = strings.ReplaceAll(pkg, "/", ".")

	s := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := s.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "//") {
			continue
		}
		var name, kind string
		switch {
		case reMessage.MatchString(line):
			name = reMessage.FindStringSubmatch(line)[1]
			kind = "struct"
		case reService.MatchString(line):
			name = reService.FindStringSubmatch(line)[1]
			kind = "interface"
		case reRpc.MatchString(line):
			name = reRpc.FindStringSubmatch(line)[1]
			kind = "method"
		case reEnum.MatchString(line):
			name = reEnum.FindStringSubmatch(line)[1]
			kind = "type"
		default:
			continue
		}
		qual := pkg + "." + name
		sig := trim
		if len(sig) > 150 {
			sig = sig[:150]
		}
		res.Symbols = append(res.Symbols, &storage.Symbol{
			Repo: repo, FilePath: filePath, SymbolKind: kind, Name: name,
			QualifiedName: qual, Signature: sig, Language: "proto",
			StartLine: lineNum, EndLine: lineNum, ContentHash: hash,
		})
	}
	return res, nil
}
