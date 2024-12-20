package nosprintf

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "nosprintf",
	Doc:      "nosprintf warns fmt.Sprintf for better performance.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// FIXME: check alias import, dot import, etc.
func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		if strings.HasSuffix(pass.Fset.File(n.Pos()).Name(), "_test.go") {
			return
		}

		call := n.(*ast.CallExpr)
		if types.ExprString(call.Fun) != "fmt.Sprintf" {
			return
		}
		if canUse(pass, call) {
			return
		}

		pass.Reportf(call.Pos(), "Don't use fmt.Sprintf")
	})

	return nil, nil
}

func canUse(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) > 5 {
		return true
	}

	if call.Ellipsis != token.NoPos {
		return true
	}

	if v, ok := call.Args[0].(*ast.BasicLit); ok {
		if len(v.Value) > 32 {
			return true
		}
		if strings.Contains(v.Value, "%0") {
			return true
		}
		if strings.Contains(v.Value, "%1") {
			return true
		}
		if strings.Contains(v.Value, "%.") {
			return true
		}
		if strings.Contains(v.Value, "%x") {
			return true
		}
		if strings.Contains(v.Value, "%+v") {
			return true
		}
		if strings.Contains(v.Value, "%#v") {
			return true
		}
	}

	for _, v := range call.Args {
		switch t := v.(type) {
		case *ast.SelectorExpr:
			return true
		case *ast.IndexExpr:
			if _, ok := pass.TypesInfo.TypeOf(t.X).(*types.Basic); !ok {
				return false
			}
		case *ast.CallExpr:
			typ := pass.TypesInfo.TypeOf(t.Fun)
			if sig, ok := typ.(*types.Signature); ok {
				if _, ok := sig.Results().At(0).Type().(*types.Basic); !ok {
					return false
				}
			} else {
				pass.Reportf(t.Pos(), "Unknown type: %v, %#v", typ, typ)
			}
		case *ast.Ident:
			if _, ok := pass.TypesInfo.ObjectOf(t).Type().(*types.Basic); !ok {
				return true
			}
		}
	}

	return false
}
