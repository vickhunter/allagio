package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	marker       = "allagio:outbound fixed-host "
	guardPackage = "affilio-api/internal/outbound"
)

// Analyzer bans raw outbound HTTP and dials outside the shared guard.
var Analyzer = &analysis.Analyzer{
	Name:     "outbound",
	Doc:      "report raw http.Client, http.Get, http.Post, and net.Dial outside internal/outbound",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Path() == guardPackage {
		return nil, nil
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.Preorder([]ast.Node{(*ast.File)(nil)}, func(n ast.Node) {
		file := n.(*ast.File)
		name := pass.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(name, "_test.go") {
			return
		}
		called := map[token.Pos]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn := funcOf(pass, call.Fun)
			if fn == nil || !banned(fn) {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				called[sel.Pos()] = true
			}
			if !marked(file, pass.Fset, call.Fun.Pos()) {
				pass.Reportf(call.Fun.Pos(), "raw outbound call %s.%s; use internal/outbound or add //%s<reason>", fn.Pkg().Path(), fn.Name(), marker)
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CompositeLit:
				if isHTTPClient(pass, node.Type) && !marked(file, pass.Fset, node.Pos()) {
					pass.Reportf(node.Pos(), "raw http.Client; use internal/outbound or add //%s<reason>", marker)
				}
			case *ast.SelectorExpr:
				if called[node.Pos()] {
					return true
				}
				fn := funcOf(pass, node)
				if fn == nil || !banned(fn) || marked(file, pass.Fset, node.Pos()) {
					return true
				}
				pass.Reportf(node.Pos(), "raw outbound dial %s.%s; use internal/outbound or add //%s<reason>", fn.Pkg().Path(), fn.Name(), marker)
			}
			return true
		})
	})
	return nil, nil
}

func funcOf(pass *analysis.Pass, expr ast.Expr) *types.Func {
	switch e := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		fn, _ := pass.TypesInfo.Uses[e.Sel].(*types.Func)
		return fn
	case *ast.Ident:
		fn, _ := pass.TypesInfo.Uses[e].(*types.Func)
		return fn
	default:
		return nil
	}
}

func banned(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	recv := sig.Recv()
	switch fn.Pkg().Path() {
	case "net/http":
		// Header.Get and Client methods are not the package helpers.
		if recv != nil {
			return false
		}
		switch fn.Name() {
		case "Get", "Head", "Post", "PostForm":
			return true
		}
	case "net":
		if recv == nil {
			return fn.Name() == "Dial" || fn.Name() == "DialTimeout"
		}
		return isDialer(recv.Type()) && (fn.Name() == "Dial" || fn.Name() == "DialContext")
	case "crypto/tls":
		if recv != nil {
			return false
		}
		return fn.Name() == "Dial" || fn.Name() == "DialWithDialer"
	}
	return false
}

func isDialer(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "net" && named.Obj().Name() == "Dialer"
}

func isHTTPClient(pass *analysis.Pass, expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		name, _ := pass.TypesInfo.Uses[e.Sel].(*types.TypeName)
		return isClientName(name)
	case *ast.Ident:
		name, _ := pass.TypesInfo.Uses[e].(*types.TypeName)
		return isClientName(name)
	default:
		return false
	}
}

func isClientName(name *types.TypeName) bool {
	return name != nil && name.Pkg() != nil && name.Pkg().Path() == "net/http" && name.Name() == "Client"
}

func marked(file *ast.File, fset *token.FileSet, pos token.Pos) bool {
	at := fset.Position(pos)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			idx := strings.Index(comment.Text, marker)
			if idx < 0 {
				continue
			}
			reason := strings.TrimSpace(comment.Text[idx+len(marker):])
			if reason == "" {
				continue
			}
			line := fset.Position(comment.Pos())
			if line.Filename == at.Filename && (line.Line == at.Line || line.Line == at.Line-1) {
				return true
			}
		}
	}
	return false
}
