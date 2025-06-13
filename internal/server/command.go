package server

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"go/types"
	"slices"
	"strconv"
	"strings"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspace_executeCommand
func (s *Server) workspaceExecuteCommand(params *ExecuteCommandParams) (any, error) {
	switch params.Command {
	case "spx.renameResources":
		var cmdParams []SpxRenameResourceParams
		for _, arg := range params.Arguments {
			var cmdParam SpxRenameResourceParams
			if err := json.Unmarshal(arg, &cmdParam); err != nil {
				return nil, fmt.Errorf("failed to unmarshal command argument as SpxRenameResourceParams: %w", err)
			}
			cmdParams = append(cmdParams, cmdParam)
		}
		return s.spxRenameResources(cmdParams)
	case "spx.getDefinitions":
		var cmdParams []SpxGetDefinitionsParams
		for _, arg := range params.Arguments {
			var cmdParam SpxGetDefinitionsParams
			if err := json.Unmarshal(arg, &cmdParam); err != nil {
				return nil, fmt.Errorf("failed to unmarshal command argument as SpxGetDefinitionsParams: %w", err)
			}
			cmdParams = append(cmdParams, cmdParam)
		}
		return s.spxGetDefinitions(cmdParams)
	case "spx.getInputSlots":
		var cmdParams []SpxGetInputSlotsParams
		for _, arg := range params.Arguments {
			var cmdParam SpxGetInputSlotsParams
			if err := json.Unmarshal(arg, &cmdParam); err != nil {
				return nil, fmt.Errorf("failed to unmarshal command argument as SpxGetInputSlotsParams: %w", err)
			}
			cmdParams = append(cmdParams, cmdParam)
		}
		return s.spxGetInputSlots(cmdParams)
	}
	return nil, fmt.Errorf("unknown command: %s", params.Command)
}

// spxRenameResources renames spx resources in the workspace.
func (s *Server) spxRenameResources(params []SpxRenameResourceParams) (*WorkspaceEdit, error) {
	result, err := s.compile()
	if err != nil {
		return nil, err
	}
	return s.spxRenameResourcesWithCompileResult(result, params)
}

// spxRenameResourcesWithCompileResult renames spx resources in the workspace with the given compile result.
func (s *Server) spxRenameResourcesWithCompileResult(result *compileResult, params []SpxRenameResourceParams) (*WorkspaceEdit, error) {
	workspaceEdit := WorkspaceEdit{
		Changes: make(map[DocumentURI][]TextEdit),
	}
	seenTextEdits := make(map[DocumentURI]map[TextEdit]struct{})
	for _, param := range params {
		id, err := ParseSpxResourceURI(param.Resource.URI)
		if err != nil {
			return nil, fmt.Errorf("failed to parse spx resource URI: %w", err)
		}
		var changes map[DocumentURI][]TextEdit
		switch id := id.(type) {
		case SpxBackdropResourceID:
			changes, err = s.spxRenameBackdropResource(result, id, param.NewName)
		case SpxSoundResourceID:
			changes, err = s.spxRenameSoundResource(result, id, param.NewName)
		case SpxSpriteResourceID:
			changes, err = s.spxRenameSpriteResource(result, id, param.NewName)
		case SpxSpriteCostumeResourceID:
			changes, err = s.spxRenameSpriteCostumeResource(result, id, param.NewName)
		case SpxSpriteAnimationResourceID:
			changes, err = s.spxRenameSpriteAnimationResource(result, id, param.NewName)
		case SpxWidgetResourceID:
			changes, err = s.spxRenameWidgetResource(result, id, param.NewName)
		default:
			return nil, fmt.Errorf("unsupported spx resource type: %T", id)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to rename spx resource %q: %w", param.Resource.URI, err)
		}
		for documentURI, textEdits := range changes {
			if _, ok := seenTextEdits[documentURI]; !ok {
				seenTextEdits[documentURI] = make(map[TextEdit]struct{})
			}
			for _, textEdit := range textEdits {
				if _, ok := seenTextEdits[documentURI][textEdit]; ok {
					continue
				}
				seenTextEdits[documentURI][textEdit] = struct{}{}

				workspaceEdit.Changes[documentURI] = append(workspaceEdit.Changes[documentURI], textEdit)
			}
		}
	}
	return &workspaceEdit, nil
}

// spxGetDefinitions gets spx definitions at a specific position in a document.
func (s *Server) spxGetDefinitions(params []SpxGetDefinitionsParams) ([]SpxDefinitionIdentifier, error) {
	if l := len(params); l == 0 {
		return nil, nil
	} else if l > 1 {
		return nil, errors.New("spx.getDefinitions only supports one document at a time")
	}
	param := params[0]

	result, spxFile, astFile, err := s.compileAndGetASTFileForDocumentURI(param.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	astFileScope := getTypeInfo(result.proj).Scopes[astFile]

	// Find the innermost scope contains the position.
	pos := PosAt(result.proj, astFile, param.Position)
	if !pos.IsValid() {
		return nil, nil
	}
	innermostScope := xgoutil.InnermostScopeAt(result.proj, pos)
	if innermostScope == nil {
		return nil, nil
	}
	isInSpxEventHandler := result.isInSpxEventHandler(pos)

	var defIDs []SpxDefinitionIdentifier
	seenDefIDs := make(map[string]struct{})
	addDefID := func(defID SpxDefinitionIdentifier) {
		if _, ok := seenDefIDs[defID.String()]; ok {
			return
		}
		seenDefIDs[defID.String()] = struct{}{}
		defIDs = append(defIDs, defID)
	}
	addDefs := func(defs ...SpxDefinition) {
		defIDs = slices.Grow(defIDs, len(defs))
		for _, def := range defs {
			addDefID(def.ID)
		}
	}

	// Add local definitions from innermost scope and its parents.
	for scope := innermostScope; scope != nil && scope != types.Universe; scope = scope.Parent() {
		isInMainScope := innermostScope == astFileScope && scope == getPkg(result.proj).Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}
			addDefID(SpxDefinitionIdentifier{
				Package: ToPtr(obj.Pkg().Name()),
				Name:    ToPtr(obj.Name()),
			})

			isThis := name == "this"
			isSpxFileMatch := spxFile == name+".spx" || (spxFile == result.mainSpxFile && name == "Game")
			isMainScopeObj := isInMainScope && isSpxFileMatch
			if !isThis && !isMainScopeObj {
				continue
			}
			named, ok := xgoutil.DerefType(obj.Type()).(*types.Named)
			if !ok || !xgoutil.IsNamedStructType(named) {
				continue
			}

			for _, def := range result.spxDefinitionsForNamedStruct(named) {
				if isInSpxEventHandler && def.ID.Name != nil {
					name := *def.ID.Name
					if idx := strings.LastIndex(name, "."); idx >= 0 {
						name = name[idx+1:]
					}
					if IsSpxEventHandlerFuncName(name) {
						continue
					}
				}
				addDefID(def.ID)
			}
		}
	}

	// Add other definitions.
	addDefs(GetSpxPkgDefinitions()...)
	addDefs(GetMathPkgSpxDefinitions()...)
	addDefs(GetBuiltinSpxDefinitions()...)
	addDefs(GeneralSpxDefinitions...)
	if innermostScope == astFileScope {
		addDefs(FileScopeSpxDefinitions...)
	}

	return defIDs, nil
}

// spxGetInputSlots gets input slots in a document.
func (s *Server) spxGetInputSlots(params []SpxGetInputSlotsParams) ([]SpxInputSlot, error) {
	if l := len(params); l == 0 {
		return nil, nil
	} else if l > 1 {
		return nil, errors.New("spx.getInputSlots only supports one document at a time")
	}
	param := params[0]

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(param.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}

	return findInputSlots(result, astFile), nil
}

// findInputSlots finds all input slots in the AST file.
func findInputSlots(result *compileResult, astFile *xgoast.File) []SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)

	var inputSlots []SpxInputSlot
	addInputSlots := func(slots ...SpxInputSlot) {
		for _, slot := range slots {
			if slices.ContainsFunc(inputSlots, func(existing SpxInputSlot) bool {
				return IsRangesOverlap(existing.Range, slot.Range)
			}) {
				continue
			}
			inputSlots = append(inputSlots, slot)
		}
	}

	xgoast.Inspect(astFile, func(node xgoast.Node) bool {
		if node == nil {
			return true
		}

		switch node := node.(type) {
		case *xgoast.BranchStmt:
			if callExpr := xgoutil.CreateCallExprFromBranchStmt(typeInfo, node); callExpr != nil {
				slots := findInputSlotsFromCallExpr(result, callExpr)
				addInputSlots(slots...)
			}
		case *xgoast.CallExpr:
			slots := findInputSlotsFromCallExpr(result, node)
			addInputSlots(slots...)
		case *xgoast.BinaryExpr:
			leftSlot := checkValueInputSlot(result, node.X, nil)
			if leftSlot != nil {
				addInputSlots(*leftSlot)
			}

			rightSlot := checkValueInputSlot(result, node.Y, nil)
			if rightSlot != nil {
				addInputSlots(*rightSlot)
			}
		case *xgoast.UnaryExpr:
			slot := checkValueInputSlot(result, node.X, nil)
			if slot != nil {
				addInputSlots(*slot)
			}
		case *xgoast.AssignStmt:
			for _, lhs := range node.Lhs {
				slot := checkAddressInputSlot(result, lhs, nil)
				if slot != nil {
					addInputSlots(*slot)
				}
			}

			for i, rhs := range node.Rhs {
				var declaredType types.Type
				if len(node.Lhs) == len(node.Rhs) {
					declaredType = typeInfo.TypeOf(node.Lhs[i])
				}

				slot := checkValueInputSlot(result, rhs, declaredType)
				if slot != nil {
					addInputSlots(*slot)
				}
			}
		case *xgoast.ForStmt:
			if node.Init != nil {
				if expr, ok := node.Init.(*xgoast.ExprStmt); ok {
					slot := checkValueInputSlot(result, expr.X, nil)
					if slot != nil {
						addInputSlots(*slot)
					}
				}
			}

			if node.Cond != nil {
				slot := checkValueInputSlot(result, node.Cond, types.Typ[types.Bool])
				if slot != nil {
					addInputSlots(*slot)
				}
			}

			if node.Post != nil {
				if expr, ok := node.Post.(*xgoast.ExprStmt); ok {
					slot := checkValueInputSlot(result, expr.X, nil)
					if slot != nil {
						addInputSlots(*slot)
					}
				}
			}
		case *xgoast.ValueSpec:
			for i, value := range node.Values {
				var declaredType types.Type
				if len(node.Names) == len(node.Values) {
					nameIdent := node.Names[i]
					if nameIdent != nil && nameIdent.Name != "_" {
						obj := typeInfo.ObjectOf(nameIdent)
						if obj != nil {
							declaredType = obj.Type()
						}
					}
				}

				slot := checkValueInputSlot(result, value, declaredType)
				if slot != nil {
					addInputSlots(*slot)
				}
			}
		case *xgoast.ReturnStmt:
			for _, res := range node.Results {
				slot := checkValueInputSlot(result, res, nil)
				if slot != nil {
					addInputSlots(*slot)
				}
			}
		case *xgoast.IfStmt:
			slot := checkValueInputSlot(result, node.Cond, types.Typ[types.Bool])
			if slot != nil {
				addInputSlots(*slot)
			}
		case *xgoast.SwitchStmt:
			if node.Tag != nil {
				slot := checkValueInputSlot(result, node.Tag, nil)
				if slot != nil {
					addInputSlots(*slot)
				}
			}
		case *xgoast.CaseClause:
			for _, expr := range node.List {
				slot := checkValueInputSlot(result, expr, nil)
				if slot != nil {
					addInputSlots(*slot)
				}
			}
		case *xgoast.RangeStmt:
			if node.Key != nil && !isBlank(node.Key) {
				slot := checkAddressInputSlot(result, node.Key, nil)
				if slot != nil {
					addInputSlots(*slot)
				}
			}

			if node.Value != nil && !isBlank(node.Value) {
				slot := checkAddressInputSlot(result, node.Value, nil)
				if slot != nil {
					addInputSlots(*slot)
				}
			}

			slot := checkValueInputSlot(result, node.X, nil)
			if slot != nil {
				addInputSlots(*slot)
			}
		case *xgoast.IncDecStmt:
			slot := checkAddressInputSlot(result, node.X, nil)
			if slot != nil {
				addInputSlots(*slot)
			}
		}
		return true
	})
	sortSpxInputSlots(inputSlots)
	return inputSlots
}

// findInputSlotsFromCallExpr finds input slots from a call expression.
func findInputSlotsFromCallExpr(result *compileResult, callExpr *xgoast.CallExpr) []SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)

	var inputSlots []SpxInputSlot
	xgoutil.WalkCallExprArgs(typeInfo, callExpr, func(fun *types.Func, params *types.Tuple, paramIndex int, arg xgoast.Expr, argIndex int) bool {
		param := params.At(paramIndex)
		if !param.Pos().IsValid() {
			return true
		}

		declaredType := xgoutil.DerefType(param.Type())
		if sliceType, ok := declaredType.(*types.Slice); ok {
			declaredType = xgoutil.DerefType(sliceType.Elem())
		}

		slot := checkValueInputSlot(result, arg, declaredType)
		if slot != nil {
			inputSlots = append(inputSlots, *slot)
		}
		return true
	})
	return inputSlots
}

// collectPredefinedNames collects all predefined names for the given expression.
func collectPredefinedNames(result *compileResult, expr xgoast.Expr, declaredType types.Type) []string {
	astFile := xgoutil.NodeASTFile(result.proj, expr)
	innermostScope := xgoutil.InnermostScopeAt(result.proj, expr.Pos())

	var names []string
	growNames := func(n int) {
		names = slices.Grow(names, n)
	}
	seenNames := make(map[string]struct{})
	addNameOf := func(obj types.Object) {
		name := obj.Name()
		switch obj.(type) {
		case *types.Var, *types.Const:
			if typ := obj.Type(); typ != nil && declaredType != nil && !types.AssignableTo(typ, declaredType) {
				return
			}

			switch {
			case name == "this",
				name == xgoutil.XGoPackage,
				strings.HasPrefix(name, "Gop_"),
				strings.HasPrefix(name, "__gop_"):
				return
			}
		case *types.Func:
			if declaredType != nil {
				// For functions with no parameters and exactly one return value,
				// check if the return type is assignable to the declared type.
				funcSig := obj.Type().(*types.Signature)
				if funcSig.Params().Len() != 0 || funcSig.Results().Len() != 1 {
					return
				}
				funcReturnType := funcSig.Results().At(0).Type()
				if !types.AssignableTo(funcReturnType, declaredType) {
					return
				}
			}

			name = xgoutil.ToLowerCamelCase(name)
		default:
			return
		}
		if _, ok := seenNames[name]; ok {
			return
		}
		seenNames[name] = struct{}{}
		names = append(names, name)
	}

	for scope := innermostScope; scope != nil && scope != types.Universe; scope = scope.Parent() {
		growNames(len(scope.Names()))
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			if scope != innermostScope || obj.Pos() < expr.Pos() {
				switch obj.(type) {
				case *types.Var, *types.Const:
					addNameOf(obj)
				}
			}

			if astFile.IsClass && !obj.Pos().IsValid() && name == "this" {
				objType := xgoutil.DerefType(obj.Type())
				named, ok := objType.(*types.Named)
				if !ok || !xgoutil.IsNamedStructType(named) {
					continue
				}

				xgoutil.WalkStruct(named, func(member types.Object, selector *types.Named) bool {
					switch member := member.(type) {
					case *types.Var:
						if !member.Origin().Embedded() {
							addNameOf(member)
						}
					case *types.Func:
						// Add methods with no parameters and exactly one return value.
						// For example, the method `Game.BackdropName` can be used in `echo backdropname`.
						funcSig := member.Type().(*types.Signature)
						if funcSig.Params().Len() == 0 && funcSig.Results().Len() == 1 {
							addNameOf(member)
						}
					}
					return true
				})
			}
		}
	}

	for _, scope := range []*types.Scope{
		GetSpxPkg().Scope(),
		GetMathPkg().Scope(),
		types.Universe,
	} {
		growNames(len(scope.Names()))
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}
			if _, ok := obj.(*types.Var); ok {
				addNameOf(obj)
			}
		}
	}

	return names
}

// checkValueInputSlot checks if the expression is a value input slot.
func checkValueInputSlot(result *compileResult, expr xgoast.Expr, declaredType types.Type) *SpxInputSlot {
	switch expr := expr.(type) {
	case *xgoast.BasicLit:
		return createValueInputSlotFromBasicLit(result, expr, declaredType)
	case *xgoast.Ident:
		return createValueInputSlotFromIdent(result, expr, declaredType)
	case *xgoast.UnaryExpr:
		return createValueInputSlotFromUnaryExpr(result, expr, declaredType)
	case *xgoast.CallExpr:
		return createValueInputSlotFromColorFuncCall(result, expr, declaredType)
	}
	return nil
}

// checkAddressInputSlot checks if the expression is an address input slot.
func checkAddressInputSlot(result *compileResult, expr xgoast.Expr, declaredType types.Type) *SpxInputSlot {
	if ident, ok := expr.(*xgoast.Ident); ok {
		return &SpxInputSlot{
			Kind:   SpxInputSlotKindAddress,
			Accept: SpxInputSlotAccept{Type: SpxInputTypeUnknown},
			Input: SpxInput{
				Kind: SpxInputKindPredefined,
				Type: SpxInputTypeUnknown,
				Name: ident.Name,
			},
			PredefinedNames: collectPredefinedNames(result, expr, declaredType),
			Range:           RangeForNode(result.proj, ident),
		}
	}
	return nil
}

// createValueInputSlotFromBasicLit creates a value input slot from a basic literal.
func createValueInputSlotFromBasicLit(result *compileResult, lit *xgoast.BasicLit, declaredType types.Type) *SpxInputSlot {
	input := SpxInput{Kind: SpxInputKindInPlace}
	switch lit.Kind {
	case xgotoken.INT:
		input.Type = SpxInputTypeInteger
		v, err := strconv.ParseInt(lit.Value, 0, 64)
		if err != nil {
			return nil
		}
		input.Value = v
	case xgotoken.FLOAT:
		input.Type = SpxInputTypeDecimal
		v, err := strconv.ParseFloat(lit.Value, 64)
		if err != nil {
			return nil
		}
		input.Value = v
	case xgotoken.STRING:
		input.Type = SpxInputTypeString
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil
		}
		input.Value = v
	default:
		return nil
	}

	accept := SpxInputSlotAccept{Type: input.Type}
	if declaredType != nil {
		accept.Type = inferSpxInputTypeFromType(declaredType)
	}
	if accept.Type == SpxInputTypeResourceName {
		for _, spxResourceRef := range result.spxResourceRefs {
			if spxResourceRef.Node == lit {
				input.Type = SpxInputTypeResourceName
				input.Value = spxResourceRef.ID.URI()
				accept.ResourceContext = ToPtr(spxResourceRef.ID.ContextURI())
				break
			}
		}
		if accept.ResourceContext == nil {
			return nil
		}
	}

	return &SpxInputSlot{
		Kind:            SpxInputSlotKindValue,
		Accept:          accept,
		Input:           input,
		PredefinedNames: collectPredefinedNames(result, lit, declaredType),
		Range:           RangeForNode(result.proj, lit),
	}
}

// createValueInputSlotFromIdent creates a value input slot from an identifier.
func createValueInputSlotFromIdent(result *compileResult, ident *xgoast.Ident, declaredType types.Type) *SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)
	typ := typeInfo.TypeOf(ident)
	if typ == nil {
		return nil
	}
	typ = xgoutil.DerefType(typ)

	input := SpxInput{
		Kind: SpxInputKindPredefined,
		Type: inferSpxInputTypeFromType(typ),
		Name: ident.Name,
	}
	switch input.Type {
	case SpxInputTypeBoolean:
		if basicType, ok := typ.(*types.Basic); ok && basicType.Kind() == types.UntypedBool {
			input.Kind = SpxInputKindInPlace
			input.Value = ident.Name == "true"
			input.Name = ""
		}
	case SpxInputTypeDirection,
		SpxInputTypeEffectKind,
		SpxInputTypeKey,
		SpxInputTypePlayAction,
		SpxInputTypeSpecialObj,
		SpxInputTypeRotationStyle:
		obj := typeInfo.ObjectOf(ident)
		if obj != nil && !IsInSpxPkg(obj) {
			break
		}
		cnst, ok := obj.(*types.Const)
		if !ok {
			break
		}
		input.Kind = SpxInputKindInPlace
		switch input.Type {
		case SpxInputTypeDirection:
			input.Value, _ = strconv.ParseFloat(cnst.Val().ExactString(), 64)
		default:
			input.Value = cnst.Name()
		}
		input.Name = ""
	}

	accept := SpxInputSlotAccept{Type: input.Type}
	if declaredType != nil {
		accept.Type = inferSpxInputTypeFromType(declaredType)
	}
	if accept.Type == SpxInputTypeResourceName {
		switch declaredType {
		case GetSpxBackdropNameType():
			accept.ResourceContext = ToPtr(SpxBackdropResourceContextURI)
		case GetSpxSoundNameType():
			accept.ResourceContext = ToPtr(SpxSoundResourceContextURI)
		case GetSpxSpriteNameType():
			accept.ResourceContext = ToPtr(SpxSpriteResourceContextURI)
		case GetSpxSpriteCostumeNameType():
			spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, ident)
			if spxSpriteResource == nil {
				return nil
			}
			accept.ResourceContext = ToPtr(FormatSpxSpriteCostumeResourceContextURI(spxSpriteResource.Name))
		case GetSpxSpriteAnimationNameType():
			spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, ident)
			if spxSpriteResource == nil {
				return nil
			}
			accept.ResourceContext = ToPtr(FormatSpxSpriteAnimationResourceContextURI(spxSpriteResource.Name))
		case GetSpxWidgetNameType():
			accept.ResourceContext = ToPtr(SpxWidgetResourceContextURI)
		default:
			return nil
		}
	}

	return &SpxInputSlot{
		Kind:            SpxInputSlotKindValue,
		Accept:          accept,
		Input:           input,
		PredefinedNames: collectPredefinedNames(result, ident, declaredType),
		Range:           RangeForNode(result.proj, ident),
	}
}

// createValueInputSlotFromUnaryExpr creates a value input slot from a unary expression.
func createValueInputSlotFromUnaryExpr(result *compileResult, expr *xgoast.UnaryExpr, declaredType types.Type) *SpxInputSlot {
	var inputSlot *SpxInputSlot
	switch x := expr.X.(type) {
	case *xgoast.BasicLit:
		inputSlot = createValueInputSlotFromBasicLit(result, x, declaredType)
		if inputSlot == nil {
			return nil
		}

		switch expr.Op {
		case xgotoken.ADD:
			// Nothing to do for unary plus.
		case xgotoken.SUB:
			switch v := inputSlot.Input.Value.(type) {
			case int64:
				inputSlot.Input.Value = -v
			case float64:
				inputSlot.Input.Value = -v
			default:
				return nil
			}
		case xgotoken.XOR:
			switch x.Kind {
			case xgotoken.INT:
				switch v := inputSlot.Input.Value.(type) {
				case int64:
					inputSlot.Input.Value = ^v
				default:
					return nil
				}
			default:
				return nil
			}
		}
	case *xgoast.Ident:
		inputSlot = createValueInputSlotFromIdent(result, x, declaredType)
		if inputSlot == nil {
			return nil
		}

		switch expr.Op {
		case xgotoken.NOT:
			switch v := inputSlot.Input.Value.(type) {
			case bool:
				inputSlot.Input.Value = !v
			default:
				return nil
			}
		}
	default:
		return nil
	}
	inputSlot.Range = RangeForNode(result.proj, expr) // Update the range to include the entire unary expression.
	return inputSlot
}

// createValueInputSlotFromColorFuncCall creates a value input slot from an spx
// color function call.
func createValueInputSlotFromColorFuncCall(result *compileResult, callExpr *xgoast.CallExpr, declaredType types.Type) *SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)

	fun := xgoutil.FuncFromCallExpr(typeInfo, callExpr)
	if fun == nil || !IsInSpxPkg(fun) || !isSpxColorFunc(fun) {
		return nil
	}

	constructor := SpxInputTypeSpxColorConstructor(fun.Name())
	maxArgs := 3
	switch constructor {
	case SpxInputTypeSpxColorConstructorHSB:
	case SpxInputTypeSpxColorConstructorHSBA:
		maxArgs = 4
	default:
		return nil // This should never happen, but just in case.
	}

	var args []float64
	for i, argExpr := range callExpr.Args {
		if i >= maxArgs {
			break
		}
		lit, ok := argExpr.(*xgoast.BasicLit)
		if !ok {
			return nil
		}

		var val float64
		switch lit.Kind {
		case xgotoken.FLOAT:
			floatVal, err := strconv.ParseFloat(lit.Value, 64)
			if err != nil {
				return nil
			}
			val = floatVal
		case xgotoken.INT:
			intVal, err := strconv.ParseInt(lit.Value, 0, 64)
			if err != nil {
				return nil
			}
			val = float64(intVal)
		default:
			return nil
		}
		args = append(args, val)
	}
	if len(args) < maxArgs {
		return nil
	}

	return &SpxInputSlot{
		Kind:   SpxInputSlotKindValue,
		Accept: SpxInputSlotAccept{Type: SpxInputTypeColor},
		Input: SpxInput{
			Kind: SpxInputKindInPlace,
			Type: SpxInputTypeColor,
			Value: SpxColorInputValue{
				Constructor: constructor,
				Args:        args,
			},
		},
		PredefinedNames: collectPredefinedNames(result, callExpr, declaredType),
		Range:           RangeForNode(result.proj, callExpr),
	}
}

// isSpxColorFunc checks if the fun is an spx color function.
func isSpxColorFunc(fun *types.Func) bool {
	switch fun {
	case GetSpxHSBFunc(), GetSpxHSBAFunc():
		return true
	}
	return false
}

// inferSpxInputTypeFromType attempts to infer the input type from the given type.
func inferSpxInputTypeFromType(typ types.Type) SpxInputType {
	if basicType, ok := typ.(*types.Basic); ok {
		switch basicType.Kind() {
		case types.Bool, types.UntypedBool:
			return SpxInputTypeBoolean
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.UntypedInt:
			return SpxInputTypeInteger
		case types.Float32, types.Float64, types.UntypedFloat:
			return SpxInputTypeDecimal
		case types.String, types.UntypedString:
			return SpxInputTypeString
		}
		return SpxInputTypeUnknown
	}

	switch typ {
	case GetSpxBackdropNameType(),
		GetSpxSoundNameType(),
		GetSpxSpriteNameType(),
		GetSpxSpriteCostumeNameType(),
		GetSpxSpriteAnimationNameType(),
		GetSpxWidgetNameType():
		return SpxInputTypeResourceName
	case GetSpxDirectionType():
		return SpxInputTypeDirection
	case GetSpxEffectKindType():
		return SpxInputTypeEffectKind
	case GetSpxKeyType():
		return SpxInputTypeKey
	case GetSpxPlayActionType():
		return SpxInputTypePlayAction
	case GetSpxSpecialObjType():
		return SpxInputTypeSpecialObj
	case GetSpxRotationStyleType():
		return SpxInputTypeRotationStyle
	}
	return SpxInputTypeUnknown
}

// inferSpxSpriteResourceEnclosingNode infers the enclosing [SpxSpriteResource]
// for the given node. It returns nil if no [SpxSpriteResource] can be inferred.
func inferSpxSpriteResourceEnclosingNode(result *compileResult, node xgoast.Node) *SpxSpriteResource {
	typeInfo := getTypeInfo(result.proj)
	spxFile := xgoutil.NodeFilename(result.proj, node)
	astFile := xgoutil.NodeASTFile(result.proj, node)

	var spxSpriteResource *SpxSpriteResource
	xgoutil.WalkPathEnclosingInterval(astFile, node.Pos(), node.End(), false, func(node xgoast.Node) bool {
		if node == nil {
			return true
		}

		callExpr, ok := node.(*xgoast.CallExpr)
		if !ok {
			return true
		}

		var spxSpriteName string
		if sel, ok := callExpr.Fun.(*xgoast.SelectorExpr); ok {
			ident, ok := sel.X.(*xgoast.Ident)
			if !ok {
				return false
			}
			obj := typeInfo.ObjectOf(ident)
			if obj == nil {
				return false
			}
			named, ok := obj.Type().(*types.Named)
			if !ok {
				return false
			}

			if named == GetSpxSpriteType() {
				spxSpriteName = ident.Name
			} else if result.hasSpxSpriteType(named) {
				spxSpriteName = obj.Name()
			}
		} else if spxFile != "main.spx" {
			spxSpriteName = strings.TrimSuffix(spxFile, ".spx")
		}
		spxSpriteResource = result.spxResourceSet.sprites[spxSpriteName]
		return false
	})
	return spxSpriteResource
}

// isBlank checks if an expression is a blank identifier (_).
func isBlank(expr xgoast.Expr) bool {
	ident, ok := expr.(*xgoast.Ident)
	return ok && ident.Name == "_"
}

// sortSpxInputSlots sorts the given spx input slots in a stable manner.
func sortSpxInputSlots(slots []SpxInputSlot) {
	slices.SortFunc(slots, func(a, b SpxInputSlot) int {
		// First sort by line number.
		if a.Range.Start.Line != b.Range.Start.Line {
			return cmp.Compare(a.Range.Start.Line, b.Range.Start.Line)
		}
		// If same line, sort by character position.
		if a.Range.Start.Character != b.Range.Start.Character {
			return cmp.Compare(a.Range.Start.Character, b.Range.Start.Character)
		}
		// If same position (unlikely), sort by input kind for stability.
		return cmp.Compare(a.Kind, b.Kind)
	})
}
