package astgo

import (
	"go/ast"
)

// NodeContext describes the AST neighborhood of a byte offset.
type NodeContext struct {
	Function string
	Symbol   string
	Kind     string
}

// ContextAt returns the enclosing function, ident, and node kind for offset.
func (f *File) ContextAt(offset int) NodeContext {
	var ctx NodeContext
	f.WalkComments(func(c Comment) {
		if offset >= c.Span.Start && offset < c.Span.End {
			ctx.Kind = "comment"
		}
	})
	f.WalkStrings(func(l Literal) {
		if offset >= l.Span.Start && offset < l.Span.End {
			ctx.Kind = "string"
		}
	})

	ast.Inspect(f.AST, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		start := f.Fset.Position(n.Pos()).Offset
		end := f.Fset.Position(n.End()).Offset
		switch x := n.(type) {
		case *ast.FuncDecl:
			if offset >= start && offset < end {
				ctx.Function = funcName(x)
			}
		case *ast.Ident:
			if offset >= start && offset < end {
				ctx.Symbol = x.Name
				if ctx.Kind == "" {
					ctx.Kind = "ident"
				}
			}
		}
		return true
	})
	if ctx.Kind == "" {
		ctx.Kind = "code"
	}
	return ctx
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Name == nil {
		return ""
	}
	name := fn.Name.Name
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return name
	}
	recv := recvName(fn.Recv.List[0].Type)
	if recv == "" {
		return name
	}
	return recv + "." + name
}

func recvName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		inner := recvName(t.X)
		if inner == "" {
			return ""
		}
		return "*" + inner
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	default:
		return ""
	}
}
