package docker

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
func (g *Indexer) Language() string     { return "dockerfile" }
func (g *Indexer) Extensions() []string { return []string{"Dockerfile", ".dockerfile"} }

var reInstr = regexp.MustCompile(`^\s*(FROM|RUN|COPY|ADD|ENV|ARG|EXPOSE|WORKDIR|CMD|ENTRYPOINT|LABEL)\b`)

func (g *Indexer) IndexFile(repo, filePath string, content []byte) (*indexer.Result, error) {
	res := &indexer.Result{}
	hash := hasher.ContentHash(content)
	pkg := strings.ReplaceAll(filePath, "/", ".")
	s := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := s.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		m := reInstr.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := strings.ToUpper(m[1]) + "_" + itoa(lineNum)
		qual := pkg + "." + name
		sig := trim
		if len(sig) > 200 {
			sig = sig[:200]
		}
		res.Symbols = append(res.Symbols, &storage.Symbol{
			Repo: repo, FilePath: filePath, SymbolKind: "field", Name: m[1],
			QualifiedName: qual, Signature: sig, Language: "dockerfile",
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
