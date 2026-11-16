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

const guardPackage = "affilio-api/internal/outbound"

var markerPrefixes = []string{
	"allagio:outbound fixed-host ",
	"allagio:outbound operator-config ",
}

// Analyzer bans raw outbound HTTP and dials outside the shared guard.
var Analyzer = &analysis.Analyzer{
	Name:     "outbound",
	Doc:      "report raw HTTP clients, transports, and dials outside internal/outbound",
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
			switch node := node.(type) {
			case *ast.CallExpr:
				if id, ok := node.Fun.(*ast.Ident); ok && id.Name == "new" && len(node.Args) == 1 && bannedType(pass, node.Args[0]) {
					report(pass, file, node.Pos(), "raw "+typeLabel(pass, node.Args[0]))
					return true
				}
				fn := funcOf(pass, node.Fun)
				if fn == nil || !bannedFunc(fn) {
					return true
				}
				if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
					called[sel.Pos()] = true
				}
				report(pass, file, node.Fun.Pos(), "raw outbound call "+fn.Pkg().Path()+"."+fn.Name())
			case *ast.CompositeLit:
				if bannedType(pass, node.Type) {
					report(pass, file, node.Pos(), "raw "+typeLabel(pass, node.Type))
				}
			case *ast.ValueSpec:
				if len(node.Values) == 0 && bannedType(pass, node.Type) {
					report(pass, file, node.Pos(), "raw "+typeLabel(pass, node.Type))
				}
			case *ast.SelectorExpr:
				if called[node.Pos()] {
					return true
				}
				if fn := funcOf(pass, node); fn != nil && bannedFunc(fn) {
					report(pass, file, node.Pos(), "raw outbound dial "+fn.Pkg().Path()+"."+fn.Name())
					return true
				}
				if obj, ok := pass.TypesInfo.Uses[node.Sel].(*types.Var); ok && bannedVar(obj) {
					report(pass, file, node.Pos(), "raw "+obj.Pkg().Name()+"."+obj.Name())
				}
			}
			return true
		})
	})
	return nil, nil
}

func report(pass *analysis.Pass, file *ast.File, pos token.Pos, kind string) {
	if marked(file, pass.Fset, pos) {
		return
	}
	pass.Reportf(pos, "%s; use internal/outbound or add //allagio:outbound fixed-host <reason> or //allagio:outbound operator-config <reason>", kind)
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

func bannedFunc(fn *types.Func) bool {
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
		if recv != nil {
			return false
		}
		switch fn.Name() {
		case "Get", "Head", "Post", "PostForm":
			return true
		}
	case "net/http/httputil":
		return recv == nil && fn.Name() == "NewSingleHostReverseProxy"
	case "net":
		if recv == nil {
			switch fn.Name() {
			case "Dial", "DialTimeout", "DialTCP", "DialUDP", "DialIP":
				return true
			}
			return false
		}
		return (isNamed(recv.Type(), "net", "Dialer") && (fn.Name() == "Dial" || fn.Name() == "DialContext"))
	case "crypto/tls":
		if recv == nil {
			return fn.Name() == "Dial" || fn.Name() == "DialWithDialer"
		}
		return isNamed(recv.Type(), "crypto/tls", "Dialer") && (fn.Name() == "Dial" || fn.Name() == "DialContext")
	case guardPackage:
		return recv == nil && fn.Name() == "NewClient"
	}
	return false
}

func bannedVar(obj *types.Var) bool {
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "net/http" {
		return false
	}
	return obj.Name() == "DefaultClient" || obj.Name() == "DefaultTransport"
}

func bannedType(pass *analysis.Pass, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	named := typeName(pass, expr)
	if named == nil || named.Pkg() == nil {
		return false
	}
	switch named.Pkg().Path() {
	case "net/http":
		return named.Name() == "Client" || named.Name() == "Transport"
	case "net/http/httputil":
		return named.Name() == "ReverseProxy"
	case "crypto/tls":
		return named.Name() == "Dialer"
	}
	return false
}

func typeLabel(pass *analysis.Pass, expr ast.Expr) string {
	named := typeName(pass, expr)
	if named == nil || named.Pkg() == nil {
		return "outbound value"
	}
	return named.Pkg().Name() + "." + named.Name()
}

func typeName(pass *analysis.Pass, expr ast.Expr) *types.TypeName {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return typeName(pass, e.X)
	case *ast.SelectorExpr:
		name, _ := pass.TypesInfo.Uses[e.Sel].(*types.TypeName)
		return name
	case *ast.Ident:
		name, _ := pass.TypesInfo.Uses[e].(*types.TypeName)
		return name
	default:
		return nil
	}
}

func isNamed(t types.Type, pkg, name string) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == pkg && named.Obj().Name() == name
}

func marked(file *ast.File, fset *token.FileSet, pos token.Pos) bool {
	at := fset.Position(pos)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if !hasMarker(comment.Text) {
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

func hasMarker(text string) bool {
	for _, prefix := range markerPrefixes {
		idx := strings.Index(text, prefix)
		if idx < 0 {
			continue
		}
		if strings.TrimSpace(text[idx+len(prefix):]) != "" {
			return true
		}
	}
	return false
}
