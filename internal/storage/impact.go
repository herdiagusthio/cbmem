//go:build !wasm

package storage

import (
	"context"
	"strings"
)

// ImpactResult groups what breaks if `qualifiedName` changes.
type ImpactResult struct {
	Symbol         *Symbol   `json:"symbol"`
	Callers        []*Symbol `json:"callers"`
	Callees        []*Symbol `json:"callees"`
	RoutesAffected []*Symbol `json:"routes_affected"`
	TestsAffected  []*Symbol `json:"tests_affected"`
}

func (s *Store) GetImpact(ctx context.Context, repo, qualifiedName string, depth int) (*ImpactResult, error) {
	if depth <= 0 {
		depth = 3
	}
	sym, err := s.GetSymbolByQualifiedName(ctx, repo, qualifiedName)
	if err != nil {
		// still return callers/callees even if exact sym not found (e.g. suffix) — try LIKE fallback
		syms, _ := s.SearchSymbols(ctx, repo, qualifiedName, 1)
		if len(syms) > 0 {
			sym = syms[0]
			qualifiedName = sym.QualifiedName
		} else {
			return nil, err
		}
	}
	callers, _ := s.GetCallers(ctx, repo, qualifiedName, depth)
	callees, _ := s.GetCallees(ctx, repo, qualifiedName, depth)

	// collect affected files from callers+self to derive routes/tests
	affected := make(map[string]*Symbol)
	for _, c := range callers {
		affected[c.FilePath] = c
	}
	// classify
	var routes []*Symbol
	var tests []*Symbol
	isTestFile := func(p string) bool {
		lp := strings.ToLower(p)
		return strings.Contains(lp, "_test.go") || strings.Contains(lp, ".test.") || strings.Contains(lp, "/test/") || strings.HasSuffix(lp, "_test.py") || strings.HasSuffix(lp, ".spec.ts") || strings.HasSuffix(lp, ".spec.js")
	}
	isRouteSym := func(sym *Symbol) bool {
		lp := strings.ToLower(sym.FilePath)
		lq := strings.ToLower(sym.QualifiedName)
		ls := strings.ToLower(sym.Signature)
		return strings.Contains(lp, "handler") || strings.Contains(lp, "route") || strings.Contains(lp, "controller") ||
			strings.Contains(lq, "handler") || strings.Contains(lq, "route") ||
			strings.Contains(ls, "echo.context") || strings.Contains(ls, "gin.context") || strings.Contains(ls, "http.request") || strings.Contains(ls, "fiber.ctx")
	}
	// routes/tests among callers
	for _, c := range callers {
		if isTestFile(c.FilePath) {
			tests = append(tests, c)
		}
		if isRouteSym(c) {
			routes = append(routes, c)
		}
	}
	// if the symbol itself is a route/test, include
	if isRouteSym(sym) {
		// avoid duplicate if already in routes
		found := false
		for _, r := range routes {
			if r.QualifiedName == sym.QualifiedName {
				found = true
				break
			}
		}
		if !found {
			routes = append(routes, sym)
		}
	}
	if isTestFile(sym.FilePath) {
		tests = append(tests, sym)
	}

	return &ImpactResult{
		Symbol:         sym,
		Callers:        callers,
		Callees:        callees,
		RoutesAffected: routes,
		TestsAffected:  tests,
	}, nil
}
