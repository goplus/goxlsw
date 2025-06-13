package server

import (
	"go/types"
	"slices"
	"sort"
	"strings"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

var (
	// semanticTokenTypesLegend defines the semantic token types we support
	// and their indexes.
	semanticTokenTypesLegend = []SemanticTokenTypes{
		NamespaceType,
		TypeType,
		InterfaceType,
		StructType,
		ParameterType,
		VariableType,
		PropertyType,
		FunctionType,
		MethodType,
		KeywordType,
		CommentType,
		StringType,
		NumberType,
		OperatorType,
		LabelType,
	}

	// semanticTokenModifiersLegend defines the semantic token modifiers we
	// support and their bit positions.
	semanticTokenModifiersLegend = []SemanticTokenModifiers{
		ModDeclaration,
		ModReadonly,
		ModStatic,
		ModDefaultLibrary,
	}
)

// getSemanticTokenTypeIndex returns the index of the given token type in the legend.
func getSemanticTokenTypeIndex(tokenType SemanticTokenTypes) uint32 {
	idx := slices.Index(semanticTokenTypesLegend, tokenType)
	if idx == -1 {
		return 0 // Fallback to first type.
	}
	return uint32(idx)
}

// getSemanticTokenModifiersMask returns the bit mask for the given modifiers.
func getSemanticTokenModifiersMask(modifiers []SemanticTokenModifiers) uint32 {
	var mask uint32
	for _, mod := range modifiers {
		if i := slices.Index(semanticTokenModifiersLegend, mod); i >= 0 {
			mask |= 1 << uint32(i)
		}
	}
	return mask
}

// semanticTokenInfo represents the information of a semantic token.
type semanticTokenInfo struct {
	startPos       xgotoken.Pos
	endPos         xgotoken.Pos
	tokenType      SemanticTokenTypes
	tokenModifiers []SemanticTokenModifiers
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_semanticTokens
func (s *Server) textDocumentSemanticTokensFull(params *SemanticTokensParams) (tokens *SemanticTokens, err error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}

	if tokensIface, ok := result.computedCache.semanticTokens.Load(params.TextDocument.URI); ok {
		return &SemanticTokens{
			Data: tokensIface.([]uint32),
		}, nil
	}
	defer func() {
		if err == nil {
			result.computedCache.semanticTokens.Store(params.TextDocument.URI, slices.Clip(tokens.Data))
		}
	}()

	var fset = result.proj.Fset
	var typeInfo = getTypeInfo(result.proj)
	var tokenInfos []semanticTokenInfo
	addToken := func(startPos, endPos xgotoken.Pos, tokenType SemanticTokenTypes, tokenModifiers []SemanticTokenModifiers) {
		if !startPos.IsValid() || !endPos.IsValid() {
			return
		}

		start := fset.Position(startPos)
		end := fset.Position(endPos)
		if start.Line <= 0 || start.Column <= 0 || end.Offset <= start.Offset {
			return
		}

		tokenInfos = append(tokenInfos, semanticTokenInfo{
			startPos:       startPos,
			endPos:         endPos,
			tokenType:      tokenType,
			tokenModifiers: tokenModifiers,
		})
	}

	xgoast.Inspect(astFile, func(node xgoast.Node) bool {
		if node == nil || !node.Pos().IsValid() {
			return true
		}

		switch node := node.(type) {
		case *xgoast.Comment:
			addToken(node.Pos(), node.End(), CommentType, nil)
		case *xgoast.BadExpr:
			addToken(node.From, node.To, OperatorType, nil)
		case *xgoast.BadStmt:
			addToken(node.From, node.To, OperatorType, nil)
		case *xgoast.EmptyStmt:
			if !node.Implicit {
				addToken(node.Semicolon, node.Semicolon+1, OperatorType, nil)
			}
		case *xgoast.Ident:
			obj := typeInfo.ObjectOf(node)
			if obj == nil {
				if xgotoken.Lookup(node.Name).IsKeyword() {
					addToken(node.Pos(), node.End(), KeywordType, nil)
				}
				return true
			}

			var (
				tokenType SemanticTokenTypes
				modifiers []SemanticTokenModifiers
			)
			switch obj := obj.(type) {
			case *types.Builtin:
				tokenType = KeywordType
				modifiers = append(modifiers, ModDefaultLibrary)
			case *types.TypeName:
				if named, ok := obj.Type().(*types.Named); ok {
					switch named.Underlying().(type) {
					case *types.Struct:
						tokenType = StructType
					case *types.Interface:
						tokenType = InterfaceType
					default:
						tokenType = TypeType
					}
				} else {
					tokenType = TypeType
				}
			case *types.Var:
				if obj.IsField() {
					if xgoutil.IsInMainPkg(obj) && xgoutil.IsDefinedInClassFieldsDecl(result.proj, obj) {
						tokenType = VariableType
					} else {
						tokenType = PropertyType
					}
				} else if obj.Parent() != nil && obj.Parent().Parent() == nil {
					defIdent := xgoutil.DefIdentFor(typeInfo, obj)
					if defIdent == node {
						tokenType = ParameterType
					} else {
						tokenType = VariableType
					}
				} else {
					tokenType = VariableType
				}
			case *types.Const:
				tokenType = VariableType
				modifiers = append(modifiers, ModStatic, ModReadonly)
			case *types.Func:
				if obj.Type().(*types.Signature).Recv() != nil {
					tokenType = MethodType
				} else {
					tokenType = FunctionType
				}
			case *types.PkgName:
				tokenType = NamespaceType
			case *types.Label:
				tokenType = LabelType
			}
			if xgoutil.DefIdentFor(typeInfo, obj) == node {
				modifiers = append(modifiers, ModDeclaration)
			}
			if obj.Pkg() != nil && !xgoutil.IsInMainPkg(obj) && !strings.Contains(xgoutil.PkgPath(obj.Pkg()), ".") {
				modifiers = append(modifiers, ModDefaultLibrary)
			}
			addToken(node.Pos(), node.End(), tokenType, modifiers)
		case *xgoast.BasicLit:
			var tokenType SemanticTokenTypes
			switch node.Kind {
			case xgotoken.STRING, xgotoken.CHAR, xgotoken.CSTRING:
				tokenType = StringType
			case xgotoken.INT, xgotoken.FLOAT, xgotoken.IMAG, xgotoken.RAT:
				tokenType = NumberType
			}
			addToken(node.ValuePos, node.ValuePos+xgotoken.Pos(len(node.Value)), tokenType, nil)

			if node.Extra != nil && len(node.Extra.Parts) > 0 {
				pos := node.ValuePos
				for _, part := range node.Extra.Parts {
					switch v := part.(type) {
					case string:
						nextPos := xgoast.NextPartPos(pos, v)
						addToken(pos, nextPos, StringType, nil)
						pos = nextPos
					case xgoast.Expr:
						pos = v.End()
					}
				}
			}
		case *xgoast.CompositeLit:
			addToken(node.Lbrace, node.Lbrace+1, OperatorType, nil)
			addToken(node.Rbrace, node.Rbrace+1, OperatorType, nil)
		case *xgoast.FuncLit:
			addToken(node.Type.Func, node.Type.Func+xgotoken.Pos(len("func")), KeywordType, nil)
		case *xgoast.SliceLit:
			addToken(node.Lbrack, node.Lbrack+1, OperatorType, nil)
			addToken(node.Rbrack, node.Rbrack+1, OperatorType, nil)
		case *xgoast.MatrixLit:
			addToken(node.Lbrack, node.Lbrack+1, OperatorType, nil)
			addToken(node.Rbrack, node.Rbrack+1, OperatorType, nil)
		case *xgoast.StarExpr:
			addToken(node.Star, node.Star+1, OperatorType, nil)
		case *xgoast.UnaryExpr:
			opLen := len(node.Op.String())
			addToken(node.OpPos, node.OpPos+xgotoken.Pos(opLen), OperatorType, nil)
		case *xgoast.BinaryExpr:
			opLen := len(node.Op.String())
			addToken(node.OpPos, node.OpPos+xgotoken.Pos(opLen), OperatorType, nil)
		case *xgoast.ParenExpr:
			addToken(node.Lparen, node.Lparen+1, OperatorType, nil)
			addToken(node.Rparen, node.Rparen+1, OperatorType, nil)
		case *xgoast.SelectorExpr:
			addToken(node.Sel.Pos()-1, node.Sel.Pos(), OperatorType, nil)
		case *xgoast.IndexExpr:
			addToken(node.Lbrack, node.Lbrack+1, OperatorType, nil)
			addToken(node.Rbrack, node.Rbrack+1, OperatorType, nil)
		case *xgoast.IndexListExpr:
			addToken(node.Lbrack, node.Lbrack+1, OperatorType, nil)
			addToken(node.Rbrack, node.Rbrack+1, OperatorType, nil)
		case *xgoast.SliceExpr:
			addToken(node.Lbrack, node.Lbrack+1, OperatorType, nil)
			addToken(node.Rbrack, node.Rbrack+1, OperatorType, nil)
		case *xgoast.TypeAssertExpr:
			addToken(node.Lparen-1, node.Lparen, OperatorType, nil)
			addToken(node.Lparen, node.Lparen+1, OperatorType, nil)
			if node.Type == nil {
				addToken(node.Lparen+1, node.Lparen+1+xgotoken.Pos(len("type")), KeywordType, nil)
			}
			addToken(node.Rparen, node.Rparen+1, OperatorType, nil)
		case *xgoast.CallExpr:
			addToken(node.Lparen, node.Lparen+1, OperatorType, nil)
			addToken(node.Rparen, node.Rparen+1, OperatorType, nil)
			if node.Ellipsis.IsValid() {
				addToken(node.Ellipsis, node.Ellipsis+3, OperatorType, nil)
			}
		case *xgoast.KeyValueExpr:
			addToken(node.Colon, node.Colon+1, OperatorType, nil)
		case *xgoast.ErrWrapExpr:
			addToken(node.TokPos, node.TokPos+1, OperatorType, nil)
			if node.Default != nil {
				addToken(node.TokPos+1, node.TokPos+2, OperatorType, nil)
			}
		case *xgoast.EnvExpr:
			addToken(node.TokPos, node.TokPos+1, OperatorType, nil)
			if node.HasBrace() {
				addToken(node.Lbrace, node.Lbrace+1, OperatorType, nil)
				addToken(node.Rbrace, node.Rbrace+1, OperatorType, nil)
			}
		case *xgoast.RangeExpr:
			addToken(node.To, node.To+1, OperatorType, nil)
			if node.Colon2.IsValid() {
				addToken(node.Colon2, node.Colon2+1, OperatorType, nil)
			}
		case *xgoast.ArrayType:
			addToken(node.Lbrack, node.Lbrack+1, OperatorType, nil)
			if node.Len == nil {
				addToken(node.Lbrack+1, node.Lbrack+2, OperatorType, nil)
			}
		case *xgoast.StructType:
			addToken(node.Struct, node.Struct+xgotoken.Pos(len("struct")), KeywordType, nil)
		case *xgoast.InterfaceType:
			addToken(node.Interface, node.Interface+xgotoken.Pos(len("interface")), KeywordType, nil)
		case *xgoast.FuncType:
			if node.Func.IsValid() {
				addToken(node.Func, node.Func+xgotoken.Pos(len("func")), KeywordType, nil)
			}
			if node.TypeParams != nil {
				addToken(node.TypeParams.Opening, node.TypeParams.Opening+1, OperatorType, nil)
				addToken(node.TypeParams.Closing, node.TypeParams.Closing+1, OperatorType, nil)
			}
		case *xgoast.MapType:
			addToken(node.Map, node.Map+xgotoken.Pos(len("map")), KeywordType, nil)
		case *xgoast.ChanType:
			addToken(node.Begin, node.Begin+xgotoken.Pos(len("chan")), KeywordType, nil)
			if node.Arrow.IsValid() {
				addToken(node.Arrow, node.Arrow+2, OperatorType, nil)
			}
		case *xgoast.GenDecl:
			switch node.Tok {
			case xgotoken.IMPORT:
				addToken(node.TokPos, node.TokPos+xgotoken.Pos(len("import")), KeywordType, nil)
			case xgotoken.CONST:
				addToken(node.TokPos, node.TokPos+xgotoken.Pos(len("const")), KeywordType, nil)
			case xgotoken.TYPE:
				addToken(node.TokPos, node.TokPos+xgotoken.Pos(len("type")), KeywordType, nil)
			case xgotoken.VAR:
				addToken(node.TokPos, node.TokPos+xgotoken.Pos(len("var")), KeywordType, nil)
			}
			if node.Lparen.IsValid() {
				addToken(node.Lparen, node.Lparen+1, OperatorType, nil)
			}
			if node.Rparen.IsValid() {
				addToken(node.Rparen, node.Rparen+1, OperatorType, nil)
			}
		case *xgoast.FuncDecl:
			if node.Shadow {
				return true
			}

			addToken(node.Type.Func, node.Type.Func+xgotoken.Pos(len("func")), KeywordType, nil)
			if node.Recv != nil {
				addToken(node.Recv.Opening, node.Recv.Opening+1, OperatorType, nil)
				addToken(node.Recv.Closing, node.Recv.Closing+1, OperatorType, nil)
			}
			if node.Operator {
				addToken(node.Name.Pos(), node.Name.End(), OperatorType, []SemanticTokenModifiers{ModDeclaration})
			}
		case *xgoast.OverloadFuncDecl:
			addToken(node.Func, node.Func+xgotoken.Pos(len("func")), KeywordType, nil)
			if node.Recv != nil {
				addToken(node.Recv.Opening, node.Recv.Opening+1, OperatorType, nil)
				addToken(node.Recv.Closing, node.Recv.Closing+1, OperatorType, nil)
			}
			if node.Operator {
				addToken(node.Name.Pos(), node.Name.End(), OperatorType, []SemanticTokenModifiers{ModDeclaration})
			} else {
				var tokenType SemanticTokenTypes
				if node.Recv != nil {
					tokenType = MethodType
				} else {
					tokenType = FunctionType
				}
				addToken(node.Name.Pos(), node.Name.End(), tokenType, []SemanticTokenModifiers{ModDeclaration})
			}
			addToken(node.Assign, node.Assign+1, OperatorType, nil)
			addToken(node.Lparen, node.Lparen+1, OperatorType, nil)
			addToken(node.Rparen, node.Rparen+1, OperatorType, nil)
		case *xgoast.ImportSpec:
			if node.Path != nil {
				addToken(node.Path.Pos(), node.Path.End(), StringType, nil)
			}
		case *xgoast.ValueSpec:
			if node.Type != nil {
				addToken(node.Type.Pos(), node.Type.End(), TypeType, nil)
			}
		case *xgoast.FieldList:
			if node.Opening.IsValid() {
				addToken(node.Opening, node.Opening+1, OperatorType, nil)
			}
			if node.Closing.IsValid() {
				addToken(node.Closing, node.Closing+1, OperatorType, nil)
			}
		case *xgoast.LabeledStmt:
			addToken(node.Label.Pos(), node.Label.End(), LabelType, nil)
			addToken(node.Colon, node.Colon+1, OperatorType, nil)
		case *xgoast.SendStmt:
			addToken(node.Arrow, node.Arrow+2, OperatorType, nil)
		case *xgoast.IncDecStmt:
			addToken(node.TokPos, node.TokPos+2, OperatorType, nil)
		case *xgoast.AssignStmt:
			opLen := len(node.Tok.String())
			addToken(node.TokPos, node.TokPos+xgotoken.Pos(opLen), OperatorType, nil)
		case *xgoast.GoStmt:
			addToken(node.Go, node.Go+xgotoken.Pos(len("go")), KeywordType, nil)
		case *xgoast.DeferStmt:
			addToken(node.Defer, node.Defer+xgotoken.Pos(len("defer")), KeywordType, nil)
		case *xgoast.ReturnStmt:
			addToken(node.Return, node.Return+xgotoken.Pos(len("return")), KeywordType, nil)
		case *xgoast.BranchStmt:
			opLen := len(node.Tok.String())
			addToken(node.TokPos, node.TokPos+xgotoken.Pos(opLen), KeywordType, nil)
		case *xgoast.BlockStmt:
			addToken(node.Lbrace, node.Lbrace+1, OperatorType, nil)
			addToken(node.Rbrace, node.Rbrace+1, OperatorType, nil)
		case *xgoast.IfStmt:
			addToken(node.If, node.If+xgotoken.Pos(len("if")), KeywordType, nil)
		case *xgoast.CaseClause:
			if node.List == nil {
				addToken(node.Case, node.Case+xgotoken.Pos(len("default")), KeywordType, nil)
			} else {
				addToken(node.Case, node.Case+xgotoken.Pos(len("case")), KeywordType, nil)
			}
		case *xgoast.SwitchStmt:
			addToken(node.Switch, node.Switch+xgotoken.Pos(len("switch")), KeywordType, nil)
		case *xgoast.TypeSwitchStmt:
			addToken(node.Switch, node.Switch+xgotoken.Pos(len("switch")), KeywordType, nil)
		case *xgoast.CommClause:
			if node.Comm == nil {
				addToken(node.Case, node.Case+xgotoken.Pos(len("default")), KeywordType, nil)
			} else {
				addToken(node.Case, node.Case+xgotoken.Pos(len("case")), KeywordType, nil)
			}
			addToken(node.Colon, node.Colon+1, OperatorType, nil)
		case *xgoast.SelectStmt:
			addToken(node.Select, node.Select+xgotoken.Pos(len("select")), KeywordType, nil)
		case *xgoast.ForStmt:
			addToken(node.For, node.For+xgotoken.Pos(len("for")), KeywordType, nil)
		case *xgoast.RangeStmt:
			addToken(node.For, node.For+xgotoken.Pos(len("for")), KeywordType, nil)
			if !node.NoRangeOp {
				addToken(node.For+xgotoken.Pos(len("for")+1), node.For+xgotoken.Pos(len("for range")), KeywordType, nil)
			}
			if node.Tok != xgotoken.ILLEGAL {
				addToken(node.TokPos, node.TokPos+xgotoken.Pos(len(node.Tok.String())), OperatorType, nil)
			}
		case *xgoast.LambdaExpr:
			addToken(node.Rarrow, node.Rarrow+2, OperatorType, nil)
			if node.LhsHasParen {
				addToken(node.First, node.First+1, OperatorType, nil)
				addToken(node.Rarrow-1, node.Rarrow, OperatorType, nil)
			}
			if node.RhsHasParen {
				addToken(node.Rarrow+2, node.Rarrow+3, OperatorType, nil)
				addToken(node.Last-1, node.Last, OperatorType, nil)
			}
		case *xgoast.LambdaExpr2:
			addToken(node.Rarrow, node.Rarrow+2, OperatorType, nil)
			if node.LhsHasParen {
				addToken(node.First, node.First+1, OperatorType, nil)
				addToken(node.Rarrow-1, node.Rarrow, OperatorType, nil)
			}
		case *xgoast.ForPhrase:
			addToken(node.For, node.For+xgotoken.Pos(len("for")), KeywordType, nil)
			addToken(node.TokPos, node.TokPos+2, OperatorType, nil)
			if node.IfPos.IsValid() {
				addToken(node.IfPos, node.IfPos+xgotoken.Pos(len("if")), KeywordType, nil)
			}
		case *xgoast.ForPhraseStmt:
			addToken(node.For, node.For+xgotoken.Pos(len("for")), KeywordType, nil)
			addToken(node.TokPos, node.TokPos+2, OperatorType, nil)
			if node.IfPos.IsValid() {
				addToken(node.IfPos, node.IfPos+xgotoken.Pos(len("if")), KeywordType, nil)
			}
			if node.Body != nil {
				addToken(node.Body.Lbrace, node.Body.Lbrace+1, OperatorType, nil)
				addToken(node.Body.Rbrace, node.Body.Rbrace+1, OperatorType, nil)
			}
		case *xgoast.ComprehensionExpr:
			addToken(node.Lpos, node.Lpos+1, OperatorType, nil)
			addToken(node.Rpos, node.Rpos+1, OperatorType, nil)
			if kvExpr, ok := node.Elt.(*xgoast.KeyValueExpr); ok {
				addToken(kvExpr.Colon, kvExpr.Colon+1, OperatorType, nil)
			}
		case *xgoast.Ellipsis:
			addToken(node.Ellipsis, node.Ellipsis+3, OperatorType, nil)
		case *xgoast.ElemEllipsis:
			addToken(node.Ellipsis, node.Ellipsis+3, OperatorType, nil)
		}
		return true
	})

	sort.Slice(tokenInfos, func(i, j int) bool {
		if tokenInfos[i].startPos != tokenInfos[j].startPos {
			return tokenInfos[i].startPos < tokenInfos[j].startPos
		}
		return tokenInfos[i].endPos < tokenInfos[j].endPos
	})

	var (
		tokensData         = make([]uint32, 0, len(tokenInfos))
		prevLine, prevChar uint32
	)
	for _, info := range tokenInfos {
		start := fset.Position(info.startPos)
		end := fset.Position(info.endPos)

		line := uint32(start.Line - 1)
		char := uint32(start.Column - 1)
		length := uint32(end.Offset - start.Offset)
		if line < prevLine || (line == prevLine && char < prevChar) {
			continue
		}

		typeIndex := getSemanticTokenTypeIndex(info.tokenType)
		modifiersMask := getSemanticTokenModifiersMask(info.tokenModifiers)

		if line == prevLine {
			tokensData = append(tokensData, 0, char-prevChar, length, typeIndex, modifiersMask)
		} else {
			tokensData = append(tokensData, line-prevLine, char, length, typeIndex, modifiersMask)
		}

		prevLine = line
		prevChar = char
	}
	return &SemanticTokens{
		Data: tokensData,
	}, nil
}
