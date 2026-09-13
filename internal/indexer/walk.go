package indexer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type WalkOpts struct {
	RepoRoot     string
	ExcludePaths []string
	ChangedSet   map[string]bool // nil = all files
}

func CollectFiles(opts WalkOpts, disp *Dispatcher) ([]string, error) {
	var files []string
	err := filepath.WalkDir(opts.RepoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(opts.RepoRoot, path)
		if rel == "." {
			return nil
		}
		// exclude dirs
		if d.IsDir() {
			for _, ex := range opts.ExcludePaths {
				if rel == ex || strings.HasPrefix(rel, ex+"/") {
					return filepath.SkipDir
				}
			}
			// skip hidden .git etc
			if strings.HasPrefix(filepath.Base(path), ".") && filepath.Base(path) != "." {
				if filepath.Base(path) == ".git" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// exclude file patterns
		for _, ex := range opts.ExcludePaths {
			if matched, _ := filepath.Match(ex, filepath.Base(path)); matched {
				return nil
			}
			if strings.HasSuffix(rel, "/"+ex) {
				return nil
			}
		}
		// incremental filter
		if opts.ChangedSet != nil && !opts.ChangedSet[rel] {
			return nil
		}
		// check extension supported
		if disp.IndexerFor(path) == nil {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}