package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/herdiagusthio/cbmem/internal/config"
	"github.com/herdiagusthio/cbmem/internal/git"
	"github.com/herdiagusthio/cbmem/internal/indexer"
	golang "github.com/herdiagusthio/cbmem/internal/indexer/golang"
	"github.com/herdiagusthio/cbmem/internal/indexer/python"
	sqlindex "github.com/herdiagusthio/cbmem/internal/indexer/sqlindex"
	"github.com/herdiagusthio/cbmem/internal/indexer/typescript"
	yamlidx "github.com/herdiagusthio/cbmem/internal/indexer/yaml"
	dockeridx "github.com/herdiagusthio/cbmem/internal/indexer/docker"
	protoidx "github.com/herdiagusthio/cbmem/internal/indexer/proto"
	"github.com/herdiagusthio/cbmem/internal/artifacts"
	"github.com/herdiagusthio/cbmem/internal/linker"
	"github.com/herdiagusthio/cbmem/internal/storage"
)

var (
	cfgFile     string
	repoPath    string
	outputDir   string
	incremental bool
	jsonOutput  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cbmem",
		Short: "Codebase Memory System — local-first indexer",
		Long:  `cbmem indexes your codebase into second-brain/codebase/<repo>/ with SQLite + Markdown artifacts.`,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default .cbmem.yaml)")

	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(queryCmd())
	rootCmd.AddCommand(callersCmd())
	rootCmd.AddCommand(calleesCmd())
	rootCmd.AddCommand(linksCmd())
	rootCmd.AddCommand(staleCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [repo-path]",
		Short: "Initialize cbmem storage for a repo",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := "."
			if len(args) > 0 {
				repo = args[0]
			}
			if repoPath != "" {
				repo = repoPath
			}
			abs, err := filepath.Abs(repo)
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			if outputDir == "" {
				outputDir = cfg.OutputDir
			}
			// Resolve repo name
			repoName := filepath.Base(abs)
			if r, err := git.GetRepoRoot(abs); err == nil {
				repoName = filepath.Base(r)
				abs = r
			}
			// Default to second-brain location if not absolute
			if !filepath.IsAbs(outputDir) {
				// Try second-brain/codebase/<repo>
				candidates := []string{
					filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName),
					filepath.Join(abs, outputDir),
				}
				outputDir = candidates[0]
				// ensure parent exists check not needed
			}
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}
			dbPath := filepath.Join(outputDir, "symbols.db")
			store, err := storage.NewStore(dbPath)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.Migrate(); err != nil {
				return err
			}
			// write meta
			_ = store.SetMeta("repo", repoName)
			_ = store.SetMeta("repo_path", abs)
			if head, err := git.GetHeadCommit(abs); err == nil {
				_ = store.SetMeta("head_commit", head)
			}
			fmt.Printf("initialized %s -> %s\n", repoName, dbPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo path (default cwd)")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir (default second-brain/codebase/<repo>)")
	return cmd
}

func indexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index [repo-path]",
		Short: "Index a codebase (incremental by default if git available)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := "."
			if len(args) > 0 {
				repo = args[0]
			}
			if repoPath != "" {
				repo = repoPath
			}
			abs, err := filepath.Abs(repo)
			if err != nil {
				return err
			}
			if r, err := git.GetRepoRoot(abs); err == nil {
				abs = r
			}
			repoName := filepath.Base(abs)

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			if outputDir == "" {
				outputDir = filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName)
				if cfg.OutputDir != ".cbmem" && cfg.OutputDir != "" {
					outputDir = cfg.OutputDir
				}
			}
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}
			dbPath := filepath.Join(outputDir, "symbols.db")
			store, err := storage.NewStore(dbPath)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.Migrate(); err != nil {
				return err
			}

			ctx := context.Background()

			var changedSet map[string]bool
			if incremental {
				if files, err := git.GetChangedFiles(abs); err == nil && len(files) > 0 {
					changedSet = make(map[string]bool, len(files))
					for _, f := range files {
						changedSet[f] = true
					}
					fmt.Printf("incremental: %d changed files\n", len(files))
				} else {
					fmt.Println("incremental: no git changes or not a git repo, full index")
				}
			}

			disp := indexer.NewDispatcher(golang.New(), typescript.New(), python.New(), sqlindex.New(), yamlidx.New(), dockeridx.New(), protoidx.New())
			files, err := indexer.CollectFiles(indexer.WalkOpts{
				RepoRoot:     abs,
				ExcludePaths: cfg.ExcludePaths,
				ChangedSet:   changedSet,
			}, disp)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				fmt.Println("no files to index")
			}
			// aggregate for call resolution
			var allResults []*indexer.Result
			symByQual := make(map[string]*storage.Symbol)
			// first pass: symbols
			for _, f := range files {
				rel, _ := filepath.Rel(abs, f)
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				idx := disp.IndexerFor(f)
				if idx == nil {
					continue
				}
				res, err := idx.IndexFile(repoName, rel, data)
				if err != nil || res == nil {
					continue
				}
				for _, s := range res.Symbols {
					// normalize repo already set
					if err := store.UpsertSymbol(ctx, s); err == nil {
						symByQual[s.QualifiedName] = s
					}
				}
				allResults = append(allResults, res)
			}
			// second pass: edges from RawCalls (best-effort: exact qual match, else suffix match)
			edgeCount := 0
			for _, res := range allResults {
				for _, rc := range res.RawCalls {
					callerSym, ok := symByQual[rc.CallerQualified]
					if !ok {
						// fallback: find by suffix
						for q, s := range symByQual {
							if strings.HasSuffix(q, "."+rc.CallerQualified) || q == rc.CallerQualified {
								callerSym = s
								ok = true
								break
							}
						}
					}
					if !ok || callerSym == nil {
						continue
					}
					// resolve callee by name/suffix
					var calleeSym *storage.Symbol
					// exact
					if s, ok := symByQual[rc.CalleeName]; ok {
						calleeSym = s
					} else {
						for q, s := range symByQual {
							if strings.HasSuffix(q, "."+rc.CalleeName) || q == rc.CalleeName {
								calleeSym = s
								break
							}
						}
					}
					if calleeSym == nil {
						continue
					}
					edge := &storage.Edge{SrcSymbolID: callerSym.ID, DstSymbolID: calleeSym.ID, EdgeKind: "calls", Confidence: 1.0}
					if err := store.UpsertEdge(ctx, edge); err == nil {
						edgeCount++
					}
				}
			}
			count := len(symByQual)
			// write meta + artifacts
			head, _ := git.GetHeadCommit(abs)
			_ = store.SetMeta("head_commit", head)
			_ = store.SetMeta("last_indexed", head)
			_ = store.SetMeta("repo", repoName)
			_ = store.SetMeta("indexed_symbols", fmt.Sprintf("%d", count))
			_ = store.SetMeta("indexed_edges", fmt.Sprintf("%d", edgeCount))
			_ = store.SetMeta("indexed_files", fmt.Sprintf("%d", len(files)))

			// artifacts: map.md + routes.md + callgraph.dot + notes.md
			_ = writeMapMD(outputDir, repoName, files, count, edgeCount)
			_ = artifacts.WriteRoutesMD(ctx, outputDir, repoName, store)
			_ = artifacts.WriteCallgraphDOT(ctx, outputDir, repoName, store)
			_ = artifacts.WriteNotesMD(ctx, outputDir, repoName, store)

			if jsonOutput {
				b, _ := json.Marshal(map[string]any{"repo": repoName, "indexed_symbols": count, "indexed_edges": edgeCount, "indexed_files": len(files), "output_dir": outputDir})
				fmt.Println(string(b))
			} else {
				fmt.Printf("indexed %s -> %s (%d symbols, %d edges, %d files, incremental=%v)\n", repoName, outputDir, count, edgeCount, len(files), incremental)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo path")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir")
	cmd.Flags().BoolVar(&incremental, "incremental", false, "only re-index changed files")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "json output for skill")
	return cmd
}

func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <term>",
		Short: "Search symbols",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			term := args[0]
			repo := repoPath
			if repo == "" {
				repo = "."
			}
			abs, _ := filepath.Abs(repo)
			if r, err := git.GetRepoRoot(abs); err == nil {
				abs = r
			}
			repoName := filepath.Base(abs)
			if outputDir == "" {
				outputDir = filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName)
			}
			dbPath := filepath.Join(outputDir, "symbols.db")
			store, err := storage.NewStore(dbPath)
			if err != nil {
				return err
			}
			defer store.Close()
			results, err := store.SearchSymbols(context.Background(), repoName, term, 20)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("no results")
				return nil
			}
			for _, s := range results {
				fmt.Printf("%s  %s:%d  %s  %s\n", s.QualifiedName, s.FilePath, s.StartLine, s.SymbolKind, s.Signature)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo name or path")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir")
	return cmd
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start MCP server (Phase 2)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("serve: MCP server not yet implemented (Phase 2, T2.6)")
			return nil
		},
	}
}

func callersCmd() *cobra.Command {
	var depth int
	cmd := &cobra.Command{
		Use:   "callers <qualifiedName>",
		Short: "Find callers of a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qn := args[0]
			repo := repoPath
			if repo == "" { repo = "." }
			abs, _ := filepath.Abs(repo)
			if r, err := git.GetRepoRoot(abs); err == nil { abs = r }
			repoName := filepath.Base(abs)
			if outputDir == "" {
				outputDir = filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName)
			}
			store, err := storage.NewStore(filepath.Join(outputDir, "symbols.db"))
			if err != nil { return err }
			defer store.Close()
			res, err := store.GetCallers(context.Background(), repoName, qn, depth)
			if err != nil { return err }
			if len(res) == 0 { fmt.Println("no callers"); return nil }
			if jsonOutput {
				b, _ := json.Marshal(res)
				fmt.Println(string(b))
			} else {
				for _, s := range res {
					fmt.Printf("%s  %s:%d  %s\n", s.QualifiedName, s.FilePath, s.StartLine, s.Signature)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 1, "transitive depth")
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo path")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "json output")
	return cmd
}

func calleesCmd() *cobra.Command {
	var depth int
	cmd := &cobra.Command{
		Use:   "callees <qualifiedName>",
		Short: "Find callees of a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qn := args[0]
			repo := repoPath
			if repo == "" { repo = "." }
			abs, _ := filepath.Abs(repo)
			if r, err := git.GetRepoRoot(abs); err == nil { abs = r }
			repoName := filepath.Base(abs)
			if outputDir == "" {
				outputDir = filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName)
			}
			store, err := storage.NewStore(filepath.Join(outputDir, "symbols.db"))
			if err != nil { return err }
			defer store.Close()
			res, err := store.GetCallees(context.Background(), repoName, qn, depth)
			if err != nil { return err }
			if len(res) == 0 { fmt.Println("no callees"); return nil }
			if jsonOutput {
				b, _ := json.Marshal(res)
				fmt.Println(string(b))
			} else {
				for _, s := range res {
					fmt.Printf("%s  %s:%d  %s\n", s.QualifiedName, s.FilePath, s.StartLine, s.Signature)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 1, "depth")
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo path")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "json")
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("cbmem v0.1.0-dev (Go+TS+Python+SQL+YAML+Docker+Proto, SQLite FTS5, linker)")
		},
	}
}

func linksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links [repo-path]",
		Short: "Build links.jsonl from // cbmem:link and ADR evidence:",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := "."
			if len(args) > 0 {
				repo = args[0]
			}
			if repoPath != "" {
				repo = repoPath
			}
			abs, _ := filepath.Abs(repo)
			if r, err := git.GetRepoRoot(abs); err == nil {
				abs = r
			}
			repoName := filepath.Base(abs)
			if outputDir == "" {
				outputDir = filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName)
			}
			_ = os.MkdirAll(outputDir, 0755)
			store, _ := storage.NewStore(filepath.Join(outputDir, "symbols.db"))
			if store != nil {
				defer store.Close()
				_ = store.Migrate()
			}
			ctx := context.Background()
			codeLinks, _ := linker.ParseCodeLinks(abs)
			// ADR links: second-brain root is parent of codebase output
			sbRoot := filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain")
			adrLinks, _ := linker.ParseADRLinks(sbRoot)
			// filter ADR links to this repo's symbols only (symbol contains repo hint or keep all for now)
			all := append(codeLinks, adrLinks...)
			if err := linker.WriteJSONL(ctx, outputDir, all, store, repoName); err != nil {
				return err
			}
			if jsonOutput {
				b, _ := json.Marshal(map[string]any{"repo": repoName, "links": len(all), "output": filepath.Join(outputDir, "links.jsonl")})
				fmt.Println(string(b))
			} else {
				fmt.Printf("links %s: %d (code %d + adr %d) -> %s\n", repoName, len(all), len(codeLinks), len(adrLinks), filepath.Join(outputDir, "links.jsonl"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo path")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "json")
	return cmd
}

func staleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stale [repo-path]",
		Short: "Report stale links (symbol hash drift or deleted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := "."
			if len(args) > 0 {
				repo = args[0]
			}
			if repoPath != "" {
				repo = repoPath
			}
			abs, _ := filepath.Abs(repo)
			if r, err := git.GetRepoRoot(abs); err == nil {
				abs = r
			}
			repoName := filepath.Base(abs)
			if outputDir == "" {
				outputDir = filepath.Join(os.Getenv("HOME"), "code-storage", "second-brain", "codebase", repoName)
			}
			store, _ := storage.NewStore(filepath.Join(outputDir, "symbols.db"))
			if store != nil {
				defer store.Close()
			}
			stale, err := linker.StaleCheck(context.Background(), outputDir, store, repoName)
			if err != nil {
				return err
			}
			if jsonOutput {
				b, _ := json.Marshal(stale)
				fmt.Println(string(b))
			} else {
				if len(stale) == 0 {
					fmt.Println("no stale links")
				} else {
					fmt.Printf("stale %d:\n", len(stale))
					for _, s := range stale {
						fmt.Printf("  %s -> %s (%s)\n", s.Symbol, s.TargetPath, s.LinkKind)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repoPath, "repo", "", "repo")
	cmd.Flags().StringVar(&outputDir, "output", "", "output dir")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "json")
	return cmd
}

func writeMapMD(outputDir, repoName string, files []string, symCount, edgeCount int) error {
	var b strings.Builder
	b.WriteString("# " + repoName + " — codebase map\n\n")
	b.WriteString(fmt.Sprintf("Symbols: %d  Edges: %d  Files: %d\n\n", symCount, edgeCount, len(files)))
	b.WriteString("## Files\n\n")
	for _, f := range files {
		b.WriteString("- " + f + "\n")
	}
	return os.WriteFile(filepath.Join(outputDir, "map.md"), []byte(b.String()), 0644)
}
