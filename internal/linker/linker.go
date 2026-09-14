package linker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/herdiagusthio/cbmem/internal/storage"
)

type LinkRecord struct {
	Symbol      string `json:"symbol"`
	SymbolID    int64  `json:"symbol_id,omitempty"`
	FilePath    string `json:"file_path"`
	TargetType  string `json:"target_type"`
	TargetPath  string `json:"target_path"`
	LinkKind    string `json:"link_kind"`
	ContentHash string `json:"content_hash,omitempty"`
}

func ParseCodeLinks(repoRoot string) ([]LinkRecord, error) {
	var out []LinkRecord
	reSym := regexp.MustCompile(`symbol\s*[=:]\s*([A-Za-z0-9_.\-/$*]+)`)
	reTgt := regexp.MustCompile(`target\s*[=:]\s*(\S+)`)

	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".cbmem" || name == "codebase" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCodeFile(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		s := bufio.NewScanner(bytes.NewReader(data))
		for s.Scan() {
			line := s.Text()
			if !strings.Contains(line, "cbmem:link") {
				continue
			}
			sym := ""
			tgt := ""
			if m := reSym.FindStringSubmatch(line); m != nil {
				sym = m[1]
			}
			if m := reTgt.FindStringSubmatch(line); m != nil {
				tgt = strings.Trim(m[1], `"' `)
			}
			if sym == "" {
				after := line[strings.Index(line, "cbmem:link")+10:]
				toks := strings.Fields(after)
				if len(toks) > 0 {
					sym = strings.Trim(toks[0], `"' `)
				}
				if tgt == "" && len(toks) > 1 {
					tgt = strings.Trim(toks[1], `"' `)
				}
			}
			if sym == "" {
				continue
			}
			if tgt == "" {
				tgt = "code:" + rel
			}
			kind := "references"
			if strings.Contains(line, "implements") {
				kind = "implements"
			} else if strings.Contains(line, "documents") {
				kind = "documents"
			}
			tType := "code"
			if strings.HasSuffix(tgt, ".md") {
				tType = "adr"
				if strings.Contains(tgt, "decision") {
					tType = "decision"
				} else if strings.Contains(tgt, "learning") {
					tType = "learning"
				}
			}
			out = append(out, LinkRecord{Symbol: sym, FilePath: rel, TargetType: tType, TargetPath: tgt, LinkKind: kind})
		}
		return nil
	})
	return out, err
}

func isCodeFile(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".sql", ".proto", ".yaml", ".yml":
		return true
	}
	if filepath.Base(p) == "Dockerfile" {
		return true
	}
	return false
}

func ParseADRLinks(secondBrainRoot string) ([]LinkRecord, error) {
	var out []LinkRecord
	roots := []string{
		filepath.Join(secondBrainRoot, "knowledge"),
		filepath.Join(secondBrainRoot, "decisions"),
		filepath.Join(secondBrainRoot, "journal"),
		filepath.Join(secondBrainRoot, "vault"),
	}
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".md" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(secondBrainRoot, path)
			fm := extractFrontmatter(data)
			if fm == nil {
				return nil
			}
			ev := fm["evidence"]
			if ev == nil {
				return nil
			}
			var evList []string
			switch v := ev.(type) {
			case string:
				evList = []string{v}
			case []any:
				for _, e := range v {
					if s, ok := e.(string); ok {
						evList = append(evList, s)
					}
				}
			default:
				return nil
			}
			for _, e := range evList {
				e = strings.TrimSpace(e)
				if e == "" {
					continue
				}
				tType := "adr"
				if strings.Contains(rel, "decision") {
					tType = "decision"
				}
				out = append(out, LinkRecord{Symbol: e, FilePath: rel, TargetType: tType, TargetPath: rel, LinkKind: "documents"})
			}
			return nil
		})
	}
	return out, nil
}

func ParseNoteLinks(searchRoot string) ([]LinkRecord, error) {
	re := regexp.MustCompile(`<!--\s*cbmem:symbol\s*=\s*["']?([A-Za-z0-9_.:\-/$*]+)["']?\s*-->`)
	var out []LinkRecord
	err := filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				n := d.Name()
				if n == ".git" || n == "vendor" || n == "node_modules" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(searchRoot, path)
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			sym := strings.TrimSpace(m[1])
			if sym == "" || sym == "..." || sym == "…" {
				continue
			}
			tType := "note"
			if strings.Contains(rel, "decisions") {
				tType = "decision"
			} else if strings.Contains(rel, "knowledge") {
				tType = "knowledge"
			}
			out = append(out, LinkRecord{Symbol: sym, FilePath: rel, TargetType: tType, TargetPath: rel, LinkKind: "documents"})
		}
		return nil
	})
	return out, err
}

func extractFrontmatter(data []byte) map[string]any {
	if !bytes.HasPrefix(data, []byte("---")) {
		return nil
	}
	rest := data[3:]
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return nil
	}
	fmRaw := rest[:idx]
	var fm map[string]any
	if err := yaml.Unmarshal(fmRaw, &fm); err != nil {
		return nil
	}
	return fm
}

func WriteJSONL(ctx context.Context, outputDir string, links []LinkRecord, store *storage.Store, repoName string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(outputDir, "links.jsonl")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for i := range links {
		lr := &links[i]
		if store != nil && lr.Symbol != "" {
			if sym, err := store.GetSymbolByQualifiedName(ctx, repoName, lr.Symbol); err == nil && sym != nil {
				lr.SymbolID = sym.ID
				lr.ContentHash = sym.ContentHash
				_ = store.UpsertLink(ctx, &storage.Link{SymbolID: sym.ID, TargetType: lr.TargetType, TargetPath: lr.TargetPath, LinkKind: lr.LinkKind})
			} else if syms, err := store.SearchSymbols(ctx, repoName, lr.Symbol, 1); err == nil && len(syms) > 0 {
				lr.SymbolID = syms[0].ID
				lr.ContentHash = syms[0].ContentHash
				_ = store.UpsertLink(ctx, &storage.Link{SymbolID: syms[0].ID, TargetType: lr.TargetType, TargetPath: lr.TargetPath, LinkKind: lr.LinkKind})
			}
		}
		if err := enc.Encode(lr); err != nil {
			return err
		}
	}
	return nil
}

func StaleCheck(ctx context.Context, outputDir string, store *storage.Store, repoName string) ([]LinkRecord, error) {
	path := filepath.Join(outputDir, "links.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no links.jsonl: %w", err)
	}
	var stale []LinkRecord
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var lr LinkRecord
		if err := json.Unmarshal(line, &lr); err != nil {
			continue
		}
		if store == nil {
			continue
		}
		var sym *storage.Symbol
		if lr.Symbol != "" {
			sym, _ = store.GetSymbolByQualifiedName(ctx, repoName, lr.Symbol)
			if sym == nil {
				if syms, _ := store.SearchSymbols(ctx, repoName, lr.Symbol, 1); len(syms) > 0 {
					sym = syms[0]
				}
			}
		}
		if sym == nil {
			stale = append(stale, lr)
			continue
		}
		if lr.ContentHash != "" && lr.ContentHash != sym.ContentHash {
			stale = append(stale, lr)
		}
	}
	return stale, nil
}
