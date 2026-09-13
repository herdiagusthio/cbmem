package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/herdiagusthio/cbmem/internal/hasher"
	"github.com/herdiagusthio/cbmem/internal/indexer"
	"github.com/herdiagusthio/cbmem/internal/storage"
)

type Indexer struct{}

func New() *Indexer { return &Indexer{} }
func (g *Indexer) Language() string   { return "go" }
func (g *Indexer) Extensions() []string { return []string{".go"} }

func (g *Indexer) IndexFile(repo, filePath string, content []byte) (*indexer.Result, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		// still return empty, caller can log
		return &indexer.Result{}, nil
	}
	pkgName := f.Name.Name
	base := filepath.Base(filePath)

	res := &indexer.Result{}
	// collect symbols first for call resolution
	// map short name -> qualified
	shortToQual := make(map[string]string)

	emit := func(kind, name, qual, sig string, start, end token.Pos) {
		sPos := fset.Position(start)
		ePos := fset.Position(end)
		hash := hasher.StringHash(qual + sig + string(content[sPos.Offset:ePos.Offset]))
		sym := &storage.Symbol{
			Repo:          repo,
			FilePath:      filePath,
			SymbolKind:    kind,
			Name:          name,
			QualifiedName: qual,
			Signature:     sig,
			Language:      "go",
			StartLine:     sPos.Line,
			EndLine:       ePos.Line,
			ContentHash:   hash,
		}
		res.Symbols = append(res.Symbols, sym)
		shortToQual[name] = qual
		// also map qual itself
		shortToQual[qual] = qual
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			sig := funcSig(d)
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recv := recvType(d.Recv.List[0].Type)
				qual := pkgName + "." + recv + "." + name
				emit("method", name, qual, sig, d.Pos(), d.End())
			} else {
				qual := pkgName + "." + name
				emit("function", name, qual, sig, d.Pos(), d.End())
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					kind := typeKind(s.Type)
					qual := pkgName + "." + s.Name.Name
					sig := strings.TrimSpace(string(content[fset.Position(s.Pos()).Offset:fset.Position(s.End()).Offset]))
					// truncate sig to first line
					if idx := strings.Index(sig, "\n"); idx > 0 {
						sig = sig[:idx]
					}
					if len(sig) > 300 {
						sig = sig[:300]
					}
					emit(kind, s.Name.Name, qual, sig, s.Pos(), s.End())
					_ = base
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok.String() == "const" {
						kind = "const"
					}
					for _, n := range s.Names {
						qual := pkgName + "." + n.Name
						sig := kind + " " + n.Name
						if s.Type != nil {
							sig += " " + exprString(s.Type)
						}
						emit(kind, n.Name, qual, sig, n.Pos(), n.End())
					}
				}
			}
		}
	}

	// second pass: collect calls
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := callCallee(call.Fun)
		if callee == "" {
			return true
		}
		// find enclosing func for caller
		caller := enclosingFunc(f, fset, call.Pos())
		if caller == "" {
			caller = pkgName + ".__file__"
			// ensure file-level symbol exists
			if _, ok := shortToQual[caller]; !ok {
				emit("file", filepath.Base(filePath), caller, "file "+filePath, f.Pos(), f.End())
				shortToQual[caller] = caller
			}
		} else {
			// caller is short name, qualify
			if q, ok := shortToQual[caller]; ok {
				caller = q
			} else {
				caller = pkgName + "." + caller
			}
		}
		res.RawCalls = append(res.RawCalls, indexer.RawCall{
			CallerQualified: caller,
			CalleeName:      callee,
			FilePath:        filePath,
		})
		return true
	})

	return res, nil
}

func funcSig(d *ast.FuncDecl) string {
	var buf bytes.Buffer
	buf.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		buf.WriteString("(" + recvType(d.Recv.List[0].Type) + ") ")
	}
	buf.WriteString(d.Name.Name)
	if d.Type.Params != nil {
		buf.WriteString(paramsString(d.Type.Params))
	} else {
		buf.WriteString("()")
	}
	if d.Type.Results != nil {
		buf.WriteString(" " + resultsString(d.Type.Results))
	}
	return buf.String()
}

func paramsString(f *ast.FieldList) string {
	if f == nil {
		return "()"
	}
	var parts []string
	for _, field := range f.List {
		t := exprString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, t)
		} else {
			for _, n := range field.Names {
				parts = append(parts, n.Name+" "+t)
			}
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func resultsString(f *ast.FieldList) string {
	if f == nil || len(f.List) == 0 {
		return ""
	}
	if len(f.List) == 1 && len(f.List[0].Names) == 0 {
		return exprString(f.List[0].Type)
	}
	var parts []string
	for _, field := range f.List {
		t := exprString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, t)
		} else {
			for _, n := range field.Names {
				parts = append(parts, n.Name+" "+t)
			}
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func recvType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + recvType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return exprString(t)
	case *ast.IndexListExpr:
		return exprString(t)
	default:
		return exprString(expr)
	}
}

func typeKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	default:
		return "type"
	}
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.FuncType:
		return "func" + paramsString(t.Params)
	case *ast.ChanType:
		return "chan " + exprString(t.Value)
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	default:
		return ""
	}
}

func callCallee(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// X.Sel -> return Sel (method) or X.Sel
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	default:
		return ""
	}
}

func enclosingFunc(f *ast.File, fset *token.FileSet, pos token.Pos) string {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if pos >= fn.Pos() && pos <= fn.End() {
				return fn.Name.Name
			}
		}
	}
	return ""
}
