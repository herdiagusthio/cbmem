package indexer

import (
	"path/filepath"
	"strings"

	"github.com/herdiagusthio/cbmem/internal/storage"
)

type Result struct {
	Symbols []*storage.Symbol
	Edges   []*storage.Edge
	// RawCalls holds unresolved call names to be resolved after all files indexed
	RawCalls []RawCall
}

type RawCall struct {
	CallerQualified string
	CalleeName      string // e.g. "FindUser" or "repo.Find"
	FilePath        string
}

type Indexer interface {
	Language() string
	Extensions() []string
	IndexFile(repo, filePath string, content []byte) (*Result, error)
}

// Dispatcher routes files to the right indexer by extension
type Dispatcher struct {
	indexers map[string]Indexer // ext -> indexer
}

func NewDispatcher(indexers ...Indexer) *Dispatcher {
	m := make(map[string]Indexer)
	for _, idx := range indexers {
		for _, ext := range idx.Extensions() {
			m[ext] = idx
		}
	}
	return &Dispatcher{indexers: m}
}

func (d *Dispatcher) IndexerFor(path string) Indexer {
	ext := strings.ToLower(filepath.Ext(path))
	// special: Dockerfile has no ext
	if filepath.Base(path) == "Dockerfile" {
		if idx, ok := d.indexers["Dockerfile"]; ok {
			return idx
		}
	}
	if idx, ok := d.indexers[ext]; ok {
		return idx
	}
	return nil
}

func (d *Dispatcher) SupportedExtensions() []string {
	var exts []string
	for k := range d.indexers {
		exts = append(exts, k)
	}
	return exts
}
