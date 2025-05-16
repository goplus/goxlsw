package server

import (
	"go/types"
	"slices"

	gopast "github.com/goplus/gop/ast"
	goptoken "github.com/goplus/gop/token"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_inlayHint
func (s *Server) textDocumentInlayHint(params *InlayHintParams) (hints []InlayHint, err error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}

	cacheKey := inlayHintsCacheKey{
		URI:   params.TextDocument.URI,
		Range: params.Range,
	}
	if hintsIface, ok := result.computedCache.inlayHints.Load(cacheKey); ok {
		return hintsIface.([]InlayHint), nil
	}
	defer func() {
		if err == nil {
			result.computedCache.inlayHints.Store(cacheKey, slices.Clip(hints))
		}
	}()

	rangeStart := result.posAt(astFile, params.Range.Start)
	rangeEnd := result.posAt(astFile, params.Range.End)
	return collectInlayHints(result, astFile, rangeStart, rangeEnd), nil
}

// collectInlayHints collects inlay hints from the given AST file. If
// rangeStart and rangeEnd positions are provided (non-zero), only hints within
// the range are included.
func collectInlayHints(result *compileResult, astFile *gopast.File, rangeStart, rangeEnd goptoken.Pos) []InlayHint {
	typeInfo := getTypeInfo(result.proj)

	var inlayHints []InlayHint
	gopast.Inspect(astFile, func(node gopast.Node) bool {
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
		case *gopast.BranchStmt:
			if callExpr := createCallExprFromBranchStmt(typeInfo, node); callExpr != nil {
				hints := collectInlayHintsFromCallExpr(result, callExpr)
				inlayHints = append(inlayHints, hints...)
			}
		case *gopast.CallExpr:
			hints := collectInlayHintsFromCallExpr(result, node)
			inlayHints = append(inlayHints, hints...)
		}
		return true
	})
	return inlayHints
}

// collectInlayHintsFromCallExpr collects inlay hints from a call expression.
func collectInlayHintsFromCallExpr(result *compileResult, callExpr *gopast.CallExpr) []InlayHint {
	astFile := result.nodeASTFile(callExpr)
	typeInfo := getTypeInfo(result.proj)
	fset := result.proj.Fset

	var inlayHints []InlayHint
	walkCallExprArgs(typeInfo, callExpr, func(fun *types.Func, params *types.Tuple, paramIndex int, arg gopast.Expr, argIndex int) bool {
		if paramIndex < argIndex {
			// Stop processing variadic arguments beyond the declared parameters.
			return false
		}

		switch arg.(type) {
		case *gopast.LambdaExpr, *gopast.LambdaExpr2:
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
			Position: result.fromPosition(astFile, position),
			Label:    label,
			Kind:     Parameter,
		}
		inlayHints = append(inlayHints, hint)
		return true
	})
	return inlayHints
}
