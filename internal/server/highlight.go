package server

import (
	"slices"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_documentHighlight
func (s *Server) textDocumentDocumentHighlight(params *DocumentHighlightParams) (*[]DocumentHighlight, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	position := ToPosition(result.proj, astFile, params.Position)

	targetIdent := xgoutil.IdentAtPosition(result.proj, astFile, position)
	typeInfo := getTypeInfo(result.proj)
	targetObj := typeInfo.ObjectOf(targetIdent)
	if targetObj == nil {
		return nil, nil
	}

	var highlights []DocumentHighlight
	xgoast.Inspect(astFile, func(node xgoast.Node) bool {
		if node == nil {
			return true
		}
		ident, ok := node.(*xgoast.Ident)
		if !ok {
			return true
		}
		obj := typeInfo.ObjectOf(ident)
		if obj != targetObj {
			return true
		}
		path, _ := xgoutil.PathEnclosingInterval(astFile, ident.Pos(), ident.End())
		if len(path) < 2 {
			return true
		}

		kind := Text

		for _, parent := range slices.Backward(path[:len(path)-1]) {
			switch p := parent.(type) {
			case *xgoast.ValueSpec:
				for _, name := range p.Names {
					if name == ident {
						kind = Write
						break
					}
				}
			case *xgoast.Field:
				if p.Names != nil {
					for _, name := range p.Names {
						if name == ident {
							kind = Write
							break
						}
					}
				}
			case *xgoast.FuncDecl:
				if p.Name == ident {
					kind = Write
				}
			case *xgoast.TypeSpec:
				if p.Name == ident {
					kind = Write
				}
			case *xgoast.LabeledStmt:
				if p.Label == ident {
					kind = Write
				}
			case *xgoast.AssignStmt:
				switch p.Tok {
				case xgotoken.ASSIGN:
					for _, lhs := range p.Lhs {
						if lhs == ident {
							kind = Write
							break
						}
					}
					if kind != Write {
						for _, rhs := range p.Rhs {
							if rhs == ident {
								kind = Read
								break
							}
						}
					}
				case xgotoken.DEFINE:
					for _, lhs := range p.Lhs {
						if lhs == ident {
							kind = Write
							break
						}
					}
				default:
					kind = Write
				}
			case *xgoast.IncDecStmt:
				if p.X == ident {
					kind = Write
				}
			case *xgoast.RangeStmt:
				if p.X == ident {
					kind = Read
				} else if p.Key == ident || p.Value == ident {
					kind = Write
				}
			case *xgoast.TypeSwitchStmt:
				if p.Assign != nil {
					if assign, ok := p.Assign.(*xgoast.AssignStmt); ok {
						for _, lhs := range assign.Lhs {
							if lhs == ident {
								kind = Write
								break
							}
						}
					}
				}
			case *xgoast.BinaryExpr,
				*xgoast.UnaryExpr,
				*xgoast.CallExpr,
				*xgoast.CompositeLit,
				*xgoast.IndexExpr,
				*xgoast.ReturnStmt,
				*xgoast.SendStmt:
				kind = Read
			case *xgoast.KeyValueExpr:
				if p.Key == ident || p.Value == ident {
					kind = Read
				}
			case *xgoast.SelectorExpr:
				if p.X == ident {
					kind = Read
				}
			}
			if kind != Text {
				break
			}
		}

		highlights = append(highlights, DocumentHighlight{
			Range: RangeForNode(result.proj, ident),
			Kind:  kind,
		})
		return true
	})
	return &highlights, nil
}
