package server

import (
	"cmp"
	"go/types"
	"slices"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_inlayHint
func (s *Server) textDocumentInlayHint(params *InlayHintParams) ([]InlayHint, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	if !astFile.Pos().IsValid() {
		return nil, nil
	}

	rangeStart := PosAt(result.proj, astFile, params.Range.Start)
	rangeEnd := PosAt(result.proj, astFile, params.Range.End)
	return collectInlayHints(result, astFile, rangeStart, rangeEnd), nil
}

// collectInlayHints collects inlay hints from the given AST file. If
// rangeStart and rangeEnd positions are provided (non-zero), only hints within
// the range are included.
func collectInlayHints(result *compileResult, astFile *xgoast.File, rangeStart, rangeEnd xgotoken.Pos) []InlayHint {
	typeInfo := getTypeInfo(result.proj)

	var inlayHints []InlayHint
	xgoast.Inspect(astFile, func(node xgoast.Node) bool {
		if node == nil || !node.Pos().IsValid() || !node.End().IsValid() {
			return true
		}

		if rangeStart.IsValid() && node.End() < rangeStart {
			return false
		}
		if rangeEnd.IsValid() && node.Pos() > rangeEnd {
			return false
		}

		switch node := node.(type) {
		case *xgoast.BranchStmt:
			if callExpr := xgoutil.CreateCallExprFromBranchStmt(typeInfo, node); callExpr != nil {
				hints := collectInlayHintsFromCallExpr(result, callExpr)
				inlayHints = append(inlayHints, hints...)
			}
		case *xgoast.CallExpr:
			hints := collectInlayHintsFromCallExpr(result, node)
			inlayHints = append(inlayHints, hints...)
		}
		return true
	})
	sortInlayHints(inlayHints)
	return inlayHints
}

// collectInlayHintsFromCallExpr collects inlay hints from a call expression.
func collectInlayHintsFromCallExpr(result *compileResult, callExpr *xgoast.CallExpr) []InlayHint {
	astFile := xgoutil.NodeASTFile(result.proj, callExpr)
	typeInfo := getTypeInfo(result.proj)
	fset := result.proj.Fset

	var inlayHints []InlayHint
	xgoutil.WalkCallExprArgs(typeInfo, callExpr, func(fun *types.Func, params *types.Tuple, paramIndex int, arg xgoast.Expr, argIndex int) bool {
		if paramIndex < argIndex {
			// Stop processing variadic arguments beyond the declared parameters.
			return false
		}

		switch arg.(type) {
		case *xgoast.LambdaExpr, *xgoast.LambdaExpr2:
			// Skip lambda expressions.
			return true
		}

		// Create an inlay hint with the parameter name before the argument.
		position := fset.Position(arg.Pos())
		label := params.At(paramIndex).Name()
		if fun.Signature().Variadic() && argIndex == params.Len()-1 {
			label += "..."
		}
		hint := InlayHint{
			Position: FromPosition(result.proj, astFile, position),
			Label:    label,
			Kind:     Parameter,
		}
		inlayHints = append(inlayHints, hint)
		return true
	})
	return inlayHints
}

// sortInlayHints sorts the given inlay hints in a stable manner.
func sortInlayHints(hints []InlayHint) {
	slices.SortFunc(hints, func(a, b InlayHint) int {
		// First sort by line number.
		if a.Position.Line != b.Position.Line {
			return cmp.Compare(a.Position.Line, b.Position.Line)
		}
		// If same line, sort by character position.
		if a.Position.Character != b.Position.Character {
			return cmp.Compare(a.Position.Character, b.Position.Character)
		}
		// If same position (unlikely), sort by label for stability.
		return cmp.Compare(a.Label, b.Label)
	})
}
