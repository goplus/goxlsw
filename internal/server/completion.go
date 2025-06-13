package server

import (
	"cmp"
	"fmt"
	"go/types"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"

	xgoast "github.com/goplus/xgo/ast"
	xgoscanner "github.com/goplus/xgo/scanner"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgo/x/typesutil"
	"github.com/goplus/xgolsw/internal/pkgdata"
	"github.com/goplus/xgolsw/internal/vfs"
	"github.com/goplus/xgolsw/pkgdoc"
	"github.com/goplus/xgolsw/xgo"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_completion
func (s *Server) textDocumentCompletion(params *CompletionParams) ([]CompletionItem, error) {
	result, spxFile, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	if !astFile.Pos().IsValid() {
		return nil, nil
	}

	pos := PosAt(result.proj, astFile, params.Position)
	if !pos.IsValid() {
		return nil, nil
	}
	innermostScope := xgoutil.InnermostScopeAt(result.proj, pos)
	if innermostScope == nil {
		return nil, nil
	}

	proj := result.proj
	typeInfo := getTypeInfo(proj)
	ctx := &completionContext{
		itemSet:        newCompletionItemSet(),
		proj:           result.proj,
		typeInfo:       typeInfo,
		result:         result,
		spxFile:        spxFile,
		astFile:        astFile,
		astFileScope:   typeInfo.Scopes[astFile],
		tokenFile:      xgoutil.NodeTokenFile(proj, astFile),
		pos:            pos,
		innermostScope: innermostScope,
	}
	ctx.analyze()
	if err := ctx.collect(); err != nil {
		return nil, fmt.Errorf("failed to collect completion items: %w", err)
	}
	return ctx.sortedItems(), nil
}

// completionKind represents different kinds of completion contexts.
type completionKind int

const (
	completionKindUnknown completionKind = iota
	completionKindGeneral
	completionKindComment
	completionKindStringLit
	completionKindImport
	completionKindDot
	completionKindCall
	completionKindAssignOrDefine
	completionKindDecl
	completionKindReturn
	completionKindStructLit
	completionKindSwitchCase
	completionKindSelect
)

// completionContext represents the context for completion operations.
type completionContext struct {
	itemSet *completionItemSet

	proj           *xgo.Project
	typeInfo       *typesutil.Info
	result         *compileResult
	spxFile        string
	astFile        *xgoast.File
	astFileScope   *types.Scope
	tokenFile      *xgotoken.File
	pos            xgotoken.Pos
	innermostScope *types.Scope

	kind completionKind

	enclosingNode      xgoast.Node
	selectorExpr       *xgoast.SelectorExpr
	expectedTypes      []types.Type
	expectedStructType *types.Struct
	compositeLitType   *types.Named
	assignTargets      []*xgoast.Ident
	declValueSpec      *xgoast.ValueSpec
	switchTag          xgoast.Expr
	returnIndex        int

	inStringLit       bool
	inSpxEventHandler bool
}

func (ctx *completionContext) pkgDoc() *pkgdoc.PkgDoc {
	return getPkgDoc(ctx.proj)
}

// analyze analyzes the completion context to determine the kind of completion needed.
func (ctx *completionContext) analyze() {
	path, _ := xgoutil.PathEnclosingInterval(ctx.astFile, ctx.pos-1, ctx.pos)
	for i, node := range slices.Backward(path) {
		switch node := node.(type) {
		case *xgoast.ImportSpec:
			ctx.kind = completionKindImport
		case *xgoast.SelectorExpr:
			if node.Sel == nil || node.Sel.End() >= ctx.pos {
				ctx.kind = completionKindDot
				ctx.selectorExpr = node
			}
		case *xgoast.CallExpr:
			if _, ok := ctx.typeInfo.Types[node.Fun]; !ok {
				continue
			}

			// Only set call context if no more specific context has been set.
			if ctx.kind == completionKindUnknown {
				ctx.kind = completionKindCall
				ctx.enclosingNode = node
			}
		case *xgoast.CompositeLit:
			tv, ok := ctx.typeInfo.Types[node]
			if !ok {
				// Try to get type from the CompositeLit.Type field.
				if node.Type != nil {
					tv, ok = ctx.typeInfo.Types[node.Type]
				}
				if !ok {
					continue
				}
			}
			typ := xgoutil.DerefType(tv.Type)
			named, ok := typ.(*types.Named)
			if !ok {
				continue
			}
			st, ok := named.Underlying().(*types.Struct)
			if !ok {
				continue
			}

			// CompositeLit is more specific than other contexts, so override.
			ctx.kind = completionKindStructLit
			ctx.expectedStructType = st
			ctx.compositeLitType = named
			ctx.enclosingNode = node
		case *xgoast.AssignStmt:
			if node.Tok != xgotoken.ASSIGN && node.Tok != xgotoken.DEFINE {
				continue
			}
			for j, rhs := range node.Rhs {
				if rhs.Pos() > ctx.pos || ctx.pos > rhs.End() {
					continue
				}
				if j < len(node.Lhs) {
					ctx.kind = completionKindAssignOrDefine
					if tv, ok := ctx.typeInfo.Types[node.Lhs[j]]; ok {
						ctx.expectedTypes = []types.Type{tv.Type}
					}
					if ident, ok := node.Lhs[j].(*xgoast.Ident); ok {
						defIdent := xgoutil.DefIdentFor(ctx.typeInfo, ctx.typeInfo.ObjectOf(ident))
						if defIdent != nil {
							ctx.assignTargets = append(ctx.assignTargets, defIdent)
						}
					}
					break
				}
			}
		case *xgoast.ReturnStmt:
			sig := ctx.enclosingFunction(path[i+1:])
			if sig == nil {
				continue
			}
			results := sig.Results()
			if results.Len() == 0 {
				continue
			}
			ctx.kind = completionKindReturn
			ctx.returnIndex = ctx.findReturnValueIndex(node)
			if ctx.returnIndex >= 0 && ctx.returnIndex < results.Len() {
				ctx.expectedTypes = []types.Type{results.At(ctx.returnIndex).Type()}
			}
		case *xgoast.GoStmt:
			ctx.kind = completionKindCall
			ctx.enclosingNode = node.Call
		case *xgoast.DeferStmt:
			ctx.kind = completionKindCall
			ctx.enclosingNode = node.Call
		case *xgoast.SwitchStmt:
			ctx.kind = completionKindSwitchCase
			ctx.switchTag = node.Tag
		case *xgoast.SelectStmt:
			ctx.kind = completionKindSelect
		case *xgoast.DeclStmt:
			if genDecl, ok := node.Decl.(*xgoast.GenDecl); ok && (genDecl.Tok == xgotoken.VAR || genDecl.Tok == xgotoken.CONST) {
				for _, spec := range genDecl.Specs {
					valueSpec, ok := spec.(*xgoast.ValueSpec)
					if !ok || len(valueSpec.Names) == 0 {
						continue
					}
					ctx.kind = completionKindDecl
					if typ := ctx.typeInfo.TypeOf(valueSpec.Type); typ != nil && typ != types.Typ[types.Invalid] {
						ctx.expectedTypes = []types.Type{typ}
					}
					ctx.assignTargets = valueSpec.Names
					ctx.declValueSpec = valueSpec
					break
				}
			}
		case *xgoast.BasicLit:
			if node.Kind == xgotoken.STRING {
				if ctx.kind == completionKindUnknown {
					ctx.kind = completionKindStringLit
				}
				ctx.inStringLit = true
			}
		case *xgoast.BlockStmt:
			ctx.kind = completionKindUnknown
		}
	}
	if ctx.kind == completionKindUnknown {
		switch {
		case ctx.isInComment():
			ctx.kind = completionKindComment
		case ctx.isInImportStringLit():
			ctx.kind = completionKindImport
			ctx.inStringLit = true
		case ctx.isLineStart(), ctx.isInIdentifier():
			ctx.kind = completionKindGeneral
		}
	}

	ctx.inSpxEventHandler = ctx.result.isInSpxEventHandler(ctx.pos)
}

// isInComment reports whether the position of the current completion context
// is inside a comment.
func (ctx *completionContext) isInComment() bool {
	for _, comment := range ctx.astFile.Comments {
		if comment.Pos() <= ctx.pos && ctx.pos <= comment.End() {
			return true
		}
	}
	return false
}

// isInImportStringLit reports whether the position of the current completion
// context is inside an import string literal.
func (ctx *completionContext) isInImportStringLit() bool {
	var s xgoscanner.Scanner
	s.Init(ctx.tokenFile, ctx.astFile.Code, nil, 0)

	var (
		lastPos       xgotoken.Pos
		lastTok       xgotoken.Token
		inImportGroup bool
	)
	for {
		pos, tok, lit := s.Scan()
		if tok == xgotoken.EOF {
			break
		}

		// Track if we're inside an import group.
		if lastTok == xgotoken.IMPORT && tok == xgotoken.LPAREN {
			inImportGroup = true
		} else if tok == xgotoken.RPAREN {
			inImportGroup = false
		}

		// Check if we found `import` followed by `"` or we're in an import group.
		if (lastTok == xgotoken.IMPORT || inImportGroup) &&
			(tok == xgotoken.STRING || tok == xgotoken.ILLEGAL) {
			// Check if position is after `import` keyword or within an import
			// group, and inside a string literal (complete or incomplete).
			if lastPos <= ctx.pos && ctx.pos <= pos+xgotoken.Pos(len(lit)) {
				return true
			}
		}

		lastPos = pos
		lastTok = tok
	}
	return false
}

// isLineStart reports whether the position is preceded by only whitespace, or
// by a continuous sequence of non-whitespace characters (like an identifier or
// a member access expression).
func (ctx *completionContext) isLineStart() bool {
	fileBase := xgotoken.Pos(ctx.tokenFile.Base())
	relPos := ctx.pos - fileBase
	if relPos < 0 || int(relPos) > len(ctx.astFile.Code) {
		return false
	}

	line := ctx.tokenFile.Line(ctx.pos)
	lineStartPos := ctx.tokenFile.LineStart(line)
	relLineStartPos := lineStartPos - fileBase
	if relLineStartPos < 0 || int(relLineStartPos) >= len(ctx.astFile.Code) {
		return false
	}

	for pos := relLineStartPos; pos < relPos; pos++ {
		if !unicode.IsSpace(rune(ctx.astFile.Code[pos])) {
			text := string(ctx.astFile.Code[pos:relPos])
			return !slices.ContainsFunc([]rune(text), unicode.IsSpace)
		}
	}
	return true
}

// isInIdentifier reports whether the position is within an identifier.
func (ctx *completionContext) isInIdentifier() bool {
	fileBase := xgotoken.Pos(ctx.tokenFile.Base())
	relPos := ctx.pos - fileBase
	if relPos < 0 || int(relPos) > len(ctx.astFile.Code) {
		return false
	}

	var s xgoscanner.Scanner
	s.Init(ctx.tokenFile, ctx.astFile.Code, nil, 0)

	for {
		pos, tok, lit := s.Scan()
		if tok == xgotoken.EOF {
			break
		}

		// Check if position is inside this token. For identifiers, we should
		// be either in the middle or at the end to trigger completion (not
		// at the beginning).
		if pos < ctx.pos && ctx.pos <= pos+xgotoken.Pos(len(lit)) {
			return tok == xgotoken.IDENT
		}

		// If we've scanned past our position, we're not in an identifier.
		if pos > ctx.pos {
			break
		}
	}
	return false
}

// enclosingFunction gets the function signature containing the current position.
func (ctx *completionContext) enclosingFunction(path []xgoast.Node) *types.Signature {
	for _, node := range path {
		funcDecl, ok := node.(*xgoast.FuncDecl)
		if !ok {
			continue
		}
		obj := ctx.typeInfo.ObjectOf(funcDecl.Name)
		if obj == nil {
			continue
		}
		fun, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		return fun.Type().(*types.Signature)
	}
	return nil
}

// findReturnValueIndex finds the index of the return value at the current position.
func (ctx *completionContext) findReturnValueIndex(ret *xgoast.ReturnStmt) int {
	if len(ret.Results) == 0 {
		return 0
	}
	for i, expr := range ret.Results {
		if ctx.pos >= expr.Pos() && ctx.pos <= expr.End() {
			return i
		}
	}
	if ctx.pos > ret.Results[len(ret.Results)-1].End() {
		return len(ret.Results)
	}
	return -1
}

// collect collects completion items based on the completion context kind.
func (ctx *completionContext) collect() error {
	switch ctx.kind {
	case completionKindComment,
		completionKindStringLit:
		return nil
	case completionKindGeneral:
		return ctx.collectGeneral()
	case completionKindImport:
		return ctx.collectImport()
	case completionKindDot:
		return ctx.collectDot()
	case completionKindCall:
		return ctx.collectCall()
	case completionKindAssignOrDefine:
		return ctx.collectAssignOrDefine()
	case completionKindDecl:
		return ctx.collectDecl()
	case completionKindReturn:
		return ctx.collectReturn()
	case completionKindStructLit:
		return ctx.collectStructLit()
	case completionKindSwitchCase:
		return ctx.collectSwitchCase()
	case completionKindSelect:
		return ctx.collectSelect()
	}
	return nil
}

// collectGeneral collects general completions.
func (ctx *completionContext) collectGeneral() error {
	for _, expectedType := range ctx.expectedTypes {
		if err := ctx.collectTypeSpecific(expectedType); err != nil {
			return err
		}
	}

	if ctx.inStringLit {
		return nil
	}

	switch ctx.kind {
	case completionKindDecl:
		if ctx.declValueSpec.Values == nil { // var x in|
			ctx.itemSet.setSupportedKinds(
				ClassCompletion,
				InterfaceCompletion,
				StructCompletion,
			)
			break
		}
		fallthrough
	case completionKindAssignOrDefine:
		ctx.itemSet.setSupportedKinds(
			VariableCompletion,
			ConstantCompletion,
			FunctionCompletion, // TODO: Add return type compatibility check for FunctionCompletion.
			ClassCompletion,
			InterfaceCompletion,
			StructCompletion,
		)
	}
	ctx.itemSet.setExpectedTypes(ctx.expectedTypes)

	// Add local definitions from innermost scope and its parents.
	pkg := getPkg(ctx.proj)
	for scope := ctx.innermostScope; scope != nil; scope = scope.Parent() {
		isInMainScope := ctx.innermostScope == ctx.astFileScope && scope == pkg.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if !xgoutil.IsExportedOrInMainPkg(obj) {
				continue
			}
			if defIdent := xgoutil.DefIdentFor(ctx.typeInfo, obj); defIdent != nil && slices.Contains(ctx.assignTargets, defIdent) {
				continue
			}

			ctx.itemSet.addSpxDefs(ctx.result.spxDefinitionsFor(obj, "")...)

			isThis := name == "this"
			isSpxFileMatch := ctx.spxFile == name+".spx" || (ctx.spxFile == ctx.result.mainSpxFile && name == "Game")
			isMainScopeObj := isInMainScope && isSpxFileMatch
			if isThis || isMainScopeObj {
				named, ok := xgoutil.DerefType(obj.Type()).(*types.Named)
				if ok && xgoutil.IsNamedStructType(named) {
					for _, def := range ctx.result.spxDefinitionsForNamedStruct(named) {
						if ctx.inSpxEventHandler && def.ID.Name != nil {
							name := *def.ID.Name
							if idx := strings.LastIndex(name, "."); idx >= 0 {
								name = name[idx+1:]
							}
							if IsSpxEventHandlerFuncName(name) {
								continue
							}
						}
						ctx.itemSet.addSpxDefs(def)
					}
				}
			}
		}
	}

	// Add imported package definitions.
	for _, importSpec := range ctx.astFile.Imports {
		if importSpec.Path == nil {
			continue
		}
		pkgPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		pkgDoc, err := pkgdata.GetPkgDoc(pkgPath)
		if err != nil {
			continue
		}

		pkgPathBase := path.Base(pkgPath)
		pkgName := pkgPathBase
		if importSpec.Name != nil {
			pkgName = importSpec.Name.Name
		}

		ctx.itemSet.addSpxDefs(SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package: &pkgPath,
			},
			Overview: "package " + pkgPathBase,
			Detail:   pkgDoc.Doc,

			CompletionItemLabel:            pkgName,
			CompletionItemKind:             ModuleCompletion,
			CompletionItemInsertText:       pkgName,
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		})
	}

	// Add other definitions.
	ctx.itemSet.addSpxDefs(GetSpxPkgDefinitions()...)
	ctx.itemSet.addSpxDefs(GetMathPkgSpxDefinitions()...)
	ctx.itemSet.addSpxDefs(GetBuiltinSpxDefinitions()...)
	ctx.itemSet.addSpxDefs(GeneralSpxDefinitions...)
	if ctx.innermostScope == ctx.astFileScope {
		ctx.itemSet.addSpxDefs(FileScopeSpxDefinitions...)
	}

	return nil
}

// collectImport collects import completions.
func (ctx *completionContext) collectImport() error {
	pkgs, err := pkgdata.ListPkgs()
	if err != nil {
		return fmt.Errorf("failed to list packages: %w", err)
	}
	for _, pkgPath := range pkgs {
		pkgDoc, err := pkgdata.GetPkgDoc(pkgPath)
		if err != nil {
			continue
		}
		ctx.itemSet.addSpxDefs(SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package: &pkgPath,
			},
			Overview: "package " + path.Base(pkgPath),
			Detail:   pkgDoc.Doc,

			CompletionItemLabel:            pkgPath,
			CompletionItemKind:             ModuleCompletion,
			CompletionItemInsertText:       pkgPath,
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		})
	}
	return nil
}

// collectDot collects dot completions for member access.
func (ctx *completionContext) collectDot() error {
	if ctx.selectorExpr == nil {
		return nil
	}

	if ident, ok := ctx.selectorExpr.X.(*xgoast.Ident); ok {
		if obj := ctx.typeInfo.ObjectOf(ident); obj != nil {
			if pkgName, ok := obj.(*types.PkgName); ok {
				return ctx.collectPackageMembers(pkgName.Imported())
			}
		}
	}

	tv, ok := ctx.typeInfo.Types[ctx.selectorExpr.X]
	if !ok {
		return nil
	}
	typ := xgoutil.DerefType(tv.Type)
	named, ok := typ.(*types.Named)
	if ok && IsInSpxPkg(named.Obj()) && named.Obj().Name() == "Sprite" {
		typ = GetSpxSpriteImplType()
	}

	if iface, ok := typ.Underlying().(*types.Interface); ok {
		for method := range iface.Methods() {
			if !xgoutil.IsExportedOrInMainPkg(method) {
				continue
			}

			var recvTypeName string
			if named != nil && xgoutil.IsInMainPkg(named.Obj()) {
				recvTypeName = named.Obj().Name()
			}

			spxDef := ctx.result.spxDefinitionForMethod(method, recvTypeName)
			ctx.itemSet.addSpxDefs(spxDef)
		}
	} else if named, ok := typ.(*types.Named); ok && xgoutil.IsNamedStructType(named) {
		ctx.itemSet.addSpxDefs(ctx.result.spxDefinitionsForNamedStruct(named)...)
	}
	return nil
}

// collectPackageMembers collects members of a package.
func (ctx *completionContext) collectPackageMembers(pkg *types.Package) error {
	if pkg == nil {
		return nil
	}

	var pkgDoc *pkgdoc.PkgDoc
	if xgoutil.IsMainPkg(pkg) {
		pkgDoc = ctx.pkgDoc()
	} else {
		pkgPath := xgoutil.PkgPath(pkg)
		var err error
		pkgDoc, err = pkgdata.GetPkgDoc(pkgPath)
		if err != nil {
			return nil
		}
	}

	ctx.itemSet.addSpxDefs(GetSpxDefinitionsForPkg(pkg, pkgDoc)...)
	return nil
}

// collectCall collects function call completions.
func (ctx *completionContext) collectCall() error {
	callExpr, ok := ctx.enclosingNode.(*xgoast.CallExpr)
	if !ok {
		return nil
	}
	tv, ok := ctx.typeInfo.Types[callExpr.Fun]
	if !ok {
		return nil
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		// TODO: Handle invalid type with no signature, like `println`.
		return nil
	}
	argIndex := ctx.getCurrentArgIndex(callExpr)
	if argIndex < 0 {
		return nil
	}

	if fun := xgoutil.FuncFromCallExpr(ctx.typeInfo, callExpr); fun != nil {
		funcOverloads := xgoutil.ExpandXGoOverloadableFunc(fun)
		if len(funcOverloads) > 0 {
			expectedTypes := make([]types.Type, 0, len(funcOverloads))
			for _, funcOverload := range funcOverloads {
				sig := funcOverload.Type().(*types.Signature)
				if argIndex < sig.Params().Len() {
					expectedTypes = append(expectedTypes, sig.Params().At(argIndex).Type())
				} else if sig.Variadic() && argIndex >= sig.Params().Len()-1 {
					expectedTypes = append(expectedTypes, sig.Params().At(sig.Params().Len()-1).Type().(*types.Slice).Elem())
				}
			}
			ctx.expectedTypes = slices.Compact(expectedTypes)
			return ctx.collectGeneral()
		}
	}

	if argIndex < sig.Params().Len() {
		ctx.expectedTypes = []types.Type{sig.Params().At(argIndex).Type()}
	} else if sig.Variadic() && argIndex >= sig.Params().Len()-1 {
		ctx.expectedTypes = []types.Type{sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()}
	}
	return ctx.collectGeneral()
}

// getCurrentArgIndex gets the current argument index in a function call.
func (ctx *completionContext) getCurrentArgIndex(callExpr *xgoast.CallExpr) int {
	if len(callExpr.Args) == 0 {
		return 0
	}
	for i, arg := range callExpr.Args {
		if ctx.pos >= arg.Pos() && ctx.pos <= arg.End() {
			return i
		}
	}
	if ctx.pos > callExpr.Args[len(callExpr.Args)-1].End() {
		return len(callExpr.Args)
	}
	return -1
}

// collectAssignOrDefine collects completions for assignments and definitions.
func (ctx *completionContext) collectAssignOrDefine() error {
	return ctx.collectGeneral()
}

// collectDecl collects declaration completions.
func (ctx *completionContext) collectDecl() error {
	return ctx.collectGeneral()
}

// collectReturn collects return value completions.
func (ctx *completionContext) collectReturn() error {
	return ctx.collectGeneral()
}

// collectTypeSpecific collects type-specific completions.
func (ctx *completionContext) collectTypeSpecific(typ types.Type) error {
	if typ == nil || typ == types.Typ[types.Invalid] {
		return nil
	}

	var spxResourceIds []SpxResourceID
	switch typ {
	case GetSpxBackdropNameType():
		spxResourceIds = slices.Grow(spxResourceIds, len(ctx.result.spxResourceSet.backdrops))
		for spxBackdropName := range ctx.result.spxResourceSet.backdrops {
			spxResourceIds = append(spxResourceIds, SpxBackdropResourceID{spxBackdropName})
		}
	case GetSpxSpriteType(), GetSpxSpriteImplType():
		for spxSprite := range ctx.result.spxSpriteResourceAutoBindings {
			if spxSprite.Type() == typ {
				ctx.itemSet.addSpxDefs(ctx.result.spxDefinitionsFor(spxSprite, "Game")...)
			}
		}
	case GetSpxSpriteNameType():
		spxResourceIds = slices.Grow(spxResourceIds, len(ctx.result.spxResourceSet.sprites))
		for spxSpriteName := range ctx.result.spxResourceSet.sprites {
			spxResourceIds = append(spxResourceIds, SpxSpriteResourceID{spxSpriteName})
		}
	case GetSpxSpriteCostumeNameType():
		expectedSpxSprite := ctx.getSpxSpriteResource()
		for _, spxSprite := range ctx.result.spxResourceSet.sprites {
			if expectedSpxSprite == nil || spxSprite == expectedSpxSprite {
				spxResourceIds = slices.Grow(spxResourceIds, len(spxSprite.NormalCostumes))
				for _, spxSpriteCostume := range spxSprite.NormalCostumes {
					spxResourceIds = append(spxResourceIds, SpxSpriteCostumeResourceID{spxSprite.Name, spxSpriteCostume.Name})
				}
			}
		}
	case GetSpxSpriteAnimationNameType():
		expectedSpxSprite := ctx.getSpxSpriteResource()
		for _, spxSprite := range ctx.result.spxResourceSet.sprites {
			if expectedSpxSprite == nil || spxSprite == expectedSpxSprite {
				spxResourceIds = slices.Grow(spxResourceIds, len(spxSprite.Animations))
				for _, spxSpriteAnimation := range spxSprite.Animations {
					spxResourceIds = append(spxResourceIds, SpxSpriteAnimationResourceID{spxSprite.Name, spxSpriteAnimation.Name})
				}
			}
		}
	case GetSpxSoundType():
		for spxSound := range ctx.result.spxSoundResourceAutoBindings {
			if spxSound.Type() == typ {
				ctx.itemSet.addSpxDefs(ctx.result.spxDefinitionsFor(spxSound, "Game")...)
			}
		}
	case GetSpxSoundNameType():
		spxResourceIds = slices.Grow(spxResourceIds, len(ctx.result.spxResourceSet.sounds))
		for spxSoundName := range ctx.result.spxResourceSet.sounds {
			spxResourceIds = append(spxResourceIds, SpxSoundResourceID{spxSoundName})
		}
	case GetSpxWidgetNameType():
		spxResourceIds = slices.Grow(spxResourceIds, len(ctx.result.spxResourceSet.widgets))
		for spxWidgetName := range ctx.result.spxResourceSet.widgets {
			spxResourceIds = append(spxResourceIds, SpxWidgetResourceID{spxWidgetName})
		}
	}
	for _, spxResourceId := range spxResourceIds {
		name := spxResourceId.Name()
		if !ctx.inStringLit {
			name = strconv.Quote(name)
		}
		ctx.itemSet.add(CompletionItem{
			Label:            name,
			Kind:             TextCompletion,
			Documentation:    &Or_CompletionItem_documentation{Value: MarkupContent{Kind: Markdown, Value: spxResourceId.URI().HTML()}},
			InsertText:       name,
			InsertTextFormat: ToPtr(PlainTextTextFormat),
		})
	}
	return nil
}

// getSpxSpriteResource returns a [SpxSpriteResource] for the current context.
// It returns nil if no [SpxSpriteResource] can be inferred.
func (ctx *completionContext) getSpxSpriteResource() *SpxSpriteResource {
	if ctx.kind != completionKindCall {
		return nil
	}

	callExpr, ok := ctx.enclosingNode.(*xgoast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := callExpr.Fun.(*xgoast.SelectorExpr)
	if !ok {
		if ctx.spxFile == "main.spx" {
			return nil
		}
		return ctx.result.spxResourceSet.sprites[strings.TrimSuffix(ctx.spxFile, ".spx")]
	}

	ident, ok := sel.X.(*xgoast.Ident)
	if !ok {
		return nil
	}
	obj := ctx.typeInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil
	}

	if named == GetSpxSpriteType() {
		return ctx.result.spxResourceSet.sprites[ident.Name]
	}
	if vfs.HasSpriteType(ctx.proj, named) {
		return ctx.result.spxResourceSet.sprites[obj.Name()]
	}
	return nil
}

// collectStructLit collects struct literal completions.
func (ctx *completionContext) collectStructLit() error {
	if ctx.expectedStructType == nil || ctx.compositeLitType == nil {
		return nil
	}

	selectorTypeName := ctx.compositeLitType.Obj().Name()
	if IsInSpxPkg(ctx.compositeLitType.Obj()) && selectorTypeName == "SpriteImpl" {
		selectorTypeName = "Sprite"
	}

	seenFields := make(map[string]struct{})

	// Collect already used fields.
	if composite, ok := ctx.enclosingNode.(*xgoast.CompositeLit); ok {
		for _, elem := range composite.Elts {
			if kv, ok := elem.(*xgoast.KeyValueExpr); ok {
				if ident, ok := kv.Key.(*xgoast.Ident); ok {
					seenFields[ident.Name] = struct{}{}
				}
			}
		}
	}

	// Add unused fields.
	for field := range ctx.expectedStructType.Fields() {
		if !xgoutil.IsExportedOrInMainPkg(field) {
			continue
		}
		if _, ok := seenFields[field.Name()]; ok {
			continue
		}

		spxDef := ctx.result.spxDefinitionForField(field, selectorTypeName)
		spxDef.CompletionItemInsertText = field.Name() + ": ${1:}"
		spxDef.CompletionItemInsertTextFormat = SnippetTextFormat
		ctx.itemSet.addSpxDefs(spxDef)
	}

	return nil
}

// collectSwitchCase collects switch/case completions.
func (ctx *completionContext) collectSwitchCase() error {
	if ctx.switchTag == nil {
		for _, name := range []string{"int", "string", "bool", "error"} {
			if obj := types.Universe.Lookup(name); obj != nil {
				ctx.itemSet.addSpxDefs(GetSpxDefinitionForBuiltinObj(obj))
			}
		}
		return nil
	}

	tv, ok := ctx.typeInfo.Types[ctx.switchTag]
	if !ok {
		return nil
	}
	named, ok := tv.Type.(*types.Named)
	if !ok {
		return nil
	}
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return nil
	}

	var pkgDoc *pkgdoc.PkgDoc
	if xgoutil.IsMainPkg(pkg) {
		pkgDoc = ctx.pkgDoc()
	} else {
		pkgPath := xgoutil.PkgPath(pkg)
		pkgDoc, _ = pkgdata.GetPkgDoc(pkgPath)
	}

	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		c, ok := obj.(*types.Const)
		if !ok {
			continue
		}

		if types.Identical(c.Type(), tv.Type) {
			ctx.itemSet.addSpxDefs(GetSpxDefinitionForConst(c, pkgDoc))
		}
	}
	return nil
}

// collectSelect collects select statement completions.
func (ctx *completionContext) collectSelect() error {
	ctx.itemSet.add(
		CompletionItem{
			Label:            "case",
			Kind:             KeywordCompletion,
			InsertText:       "case ${1:ch} <- ${2:value}:$0",
			InsertTextFormat: ToPtr(SnippetTextFormat),
		},
		CompletionItem{
			Label:            "default",
			Kind:             KeywordCompletion,
			InsertText:       "default:$0",
			InsertTextFormat: ToPtr(SnippetTextFormat),
		},
	)
	return nil
}

// completionItemKindPriority is the priority order for different completion
// item kinds.
var completionItemKindPriority = map[CompletionItemKind]int{
	VariableCompletion:  1,
	FieldCompletion:     2,
	MethodCompletion:    3,
	FunctionCompletion:  4,
	ConstantCompletion:  5,
	ClassCompletion:     6,
	InterfaceCompletion: 7,
	ModuleCompletion:    8,
	KeywordCompletion:   9,
}

// sortedItems returns the sorted items.
func (ctx *completionContext) sortedItems() []CompletionItem {
	slices.SortStableFunc(ctx.itemSet.items, func(a, b CompletionItem) int {
		if p1, p2 := completionItemKindPriority[a.Kind], completionItemKindPriority[b.Kind]; p1 != p2 {
			return p1 - p2
		}
		return cmp.Compare(a.Label, b.Label)
	})
	return ctx.itemSet.items
}

// completionItemSet is a set of completion items.
type completionItemSet struct {
	items                         []CompletionItem
	seenSpxDefs                   map[string]struct{}
	supportedKinds                map[CompletionItemKind]struct{}
	isCompatibleWithExpectedTypes func(typ types.Type) bool
}

// newCompletionItemSet creates a new [completionItemSet].
func newCompletionItemSet() *completionItemSet {
	return &completionItemSet{
		items:       []CompletionItem{},
		seenSpxDefs: make(map[string]struct{}),
	}
}

// setSupportedKinds sets the supported kinds for the completion items.
func (s *completionItemSet) setSupportedKinds(kinds ...CompletionItemKind) {
	if len(kinds) == 0 {
		return
	}

	s.supportedKinds = make(map[CompletionItemKind]struct{})
	for _, kind := range kinds {
		s.supportedKinds[kind] = struct{}{}
	}
}

// setExpectedTypes sets the expected types for the completion items.
func (s *completionItemSet) setExpectedTypes(expectedTypes []types.Type) {
	if len(expectedTypes) == 0 {
		return
	}

	s.isCompatibleWithExpectedTypes = func(typ types.Type) bool {
		for _, expectedType := range expectedTypes {
			if expectedType != types.Typ[types.Invalid] && xgoutil.IsTypesCompatible(typ, expectedType) {
				return true
			}
		}
		return false
	}
}

// add adds items to the set.
func (s *completionItemSet) add(items ...CompletionItem) {
	for _, item := range items {
		if s.supportedKinds != nil {
			if _, ok := s.supportedKinds[item.Kind]; !ok {
				continue
			}
		}
		s.items = append(s.items, item)
	}
}

// addSpxDefs adds spx definitions to the set.
func (s *completionItemSet) addSpxDefs(spxDefs ...SpxDefinition) {
	for _, spxDef := range spxDefs {
		if s.isCompatibleWithExpectedTypes != nil && !s.isCompatibleWithExpectedTypes(spxDef.TypeHint) {
			continue
		}

		spxDefIDKey := spxDef.ID.String()
		if _, ok := s.seenSpxDefs[spxDefIDKey]; ok {
			continue
		}
		s.seenSpxDefs[spxDefIDKey] = struct{}{}

		s.add(spxDef.CompletionItem())
	}
}
