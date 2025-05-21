package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/types"
	"slices"
	"strconv"
	"strings"

	gopast "github.com/goplus/gop/ast"
	goptoken "github.com/goplus/gop/token"
	"github.com/goplus/goxlsw/internal/util"
	"github.com/goplus/goxlsw/internal/vfs"
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
	pos := result.posAt(astFile, param.Position)
	if !pos.IsValid() {
		return nil, nil
	}
	innermostScope := result.innermostScopeAt(pos)
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
				Package: util.ToPtr(obj.Pkg().Name()),
				Name:    util.ToPtr(obj.Name()),
			})

			isThis := name == "this"
			isSpxFileMatch := spxFile == name+".spx" || (spxFile == result.mainSpxFile && name == "Game")
			isMainScopeObj := isInMainScope && isSpxFileMatch
			if !isThis && !isMainScopeObj {
				continue
			}
			named, ok := unwrapPointerType(obj.Type()).(*types.Named)
			if !ok || !isNamedStructType(named) {
				continue
			}

			for _, def := range result.spxDefinitionsForNamedStruct(named) {
				if isInSpxEventHandler && def.ID.Name != nil {
					name := *def.ID.Name
					if idx := strings.LastIndex(name, "."); idx >= 0 {
						name = name[idx+1:]
					}
					if isSpxEventHandlerFuncName(name) {
						continue
					}
				}
				addDefID(def.ID)
			}
		}
	}

	// Add other definitions.
	addDefs(GetSpxPkgDefinitions()...)
	addDefs(GetBuiltinSpxDefinitions()...)
	addDefs(GeneralSpxDefinitions...)
	if innermostScope == astFileScope {
		addDefs(FileScopeSpxDefinitions...)
	}

	return defIDs, nil
}

// spxGetInputSlots gets input slots in a document.
func (s *Server) spxGetInputSlots(params []SpxGetInputSlotsParams) (slots []SpxInputSlot, err error) {
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

	if slotsIface, ok := result.computedCache.spxIntputSlots.Load(param.TextDocument.URI); ok {
		return slotsIface.([]SpxInputSlot), nil
	}
	defer func() {
		if err == nil {
			result.computedCache.spxIntputSlots.Store(param.TextDocument.URI, slices.Clip(slots))
		}
	}()

	return findInputSlots(result, astFile), nil
}

// findInputSlots finds all input slots in the AST file.
func findInputSlots(result *compileResult, astFile *gopast.File) []SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)

	var inputSlots []SpxInputSlot
	gopast.Inspect(astFile, func(node gopast.Node) bool {
		if node == nil {
			return true
		}

		switch node := node.(type) {
		case *gopast.BranchStmt:
			if callExpr := createCallExprFromBranchStmt(typeInfo, node); callExpr != nil {
				slots := findInputSlotsFromCallExpr(result, callExpr)
				inputSlots = append(inputSlots, slots...)
			}
		case *gopast.CallExpr:
			slots := findInputSlotsFromCallExpr(result, node)
			inputSlots = append(inputSlots, slots...)
		case *gopast.BinaryExpr:
			leftSlot := checkValueInputSlot(result, node.X, nil)
			if leftSlot != nil {
				inputSlots = append(inputSlots, *leftSlot)
			}

			rightSlot := checkValueInputSlot(result, node.Y, nil)
			if rightSlot != nil {
				inputSlots = append(inputSlots, *rightSlot)
			}
		case *gopast.UnaryExpr:
			slot := checkValueInputSlot(result, node.X, nil)
			if slot != nil {
				inputSlots = append(inputSlots, *slot)
			}
		case *gopast.AssignStmt:
			for _, lhs := range node.Lhs {
				slot := checkAddressInputSlot(result, lhs, nil)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}

			for i, rhs := range node.Rhs {
				var declaredType types.Type
				if len(node.Lhs) == len(node.Rhs) {
					declaredType = typeInfo.TypeOf(node.Lhs[i])
				}

				slot := checkValueInputSlot(result, rhs, declaredType)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}
		case *gopast.ForStmt:
			if node.Init != nil {
				if expr, ok := node.Init.(*gopast.ExprStmt); ok {
					slot := checkValueInputSlot(result, expr.X, nil)
					if slot != nil {
						inputSlots = append(inputSlots, *slot)
					}
				}
			}

			if node.Cond != nil {
				slot := checkValueInputSlot(result, node.Cond, types.Typ[types.Bool])
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}

			if node.Post != nil {
				if expr, ok := node.Post.(*gopast.ExprStmt); ok {
					slot := checkValueInputSlot(result, expr.X, nil)
					if slot != nil {
						inputSlots = append(inputSlots, *slot)
					}
				}
			}
		case *gopast.ValueSpec:
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
					inputSlots = append(inputSlots, *slot)
				}
			}
		case *gopast.ReturnStmt:
			for _, res := range node.Results {
				slot := checkValueInputSlot(result, res, nil)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}
		case *gopast.IfStmt:
			slot := checkValueInputSlot(result, node.Cond, types.Typ[types.Bool])
			if slot != nil {
				inputSlots = append(inputSlots, *slot)
			}
		case *gopast.SwitchStmt:
			if node.Tag != nil {
				slot := checkValueInputSlot(result, node.Tag, nil)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}
		case *gopast.CaseClause:
			for _, expr := range node.List {
				slot := checkValueInputSlot(result, expr, nil)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}
		case *gopast.RangeStmt:
			if node.Key != nil && !isBlank(node.Key) {
				slot := checkAddressInputSlot(result, node.Key, nil)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}

			if node.Value != nil && !isBlank(node.Value) {
				slot := checkAddressInputSlot(result, node.Value, nil)
				if slot != nil {
					inputSlots = append(inputSlots, *slot)
				}
			}

			slot := checkValueInputSlot(result, node.X, nil)
			if slot != nil {
				inputSlots = append(inputSlots, *slot)
			}
		case *gopast.IncDecStmt:
			slot := checkAddressInputSlot(result, node.X, nil)
			if slot != nil {
				inputSlots = append(inputSlots, *slot)
			}
		}
		return true
	})
	return inputSlots
}

// findInputSlotsFromCallExpr finds input slots from a call expression.
func findInputSlotsFromCallExpr(result *compileResult, callExpr *gopast.CallExpr) []SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)

	var inputSlots []SpxInputSlot
	walkCallExprArgs(typeInfo, callExpr, func(fun *types.Func, params *types.Tuple, paramIndex int, arg gopast.Expr, argIndex int) bool {
		param := params.At(paramIndex)
		if !param.Pos().IsValid() {
			return true
		}

		declaredType := unwrapPointerType(param.Type())
		if sliceType, ok := declaredType.(*types.Slice); ok {
			declaredType = unwrapPointerType(sliceType.Elem())
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
func collectPredefinedNames(result *compileResult, expr gopast.Expr, declaredType types.Type) []string {
	astFile := result.nodeASTFile(expr)
	innermostScope := result.innermostScopeAt(expr.Pos())

	var names []string
	growNames := func(n int) {
		names = slices.Grow(names, n)
	}
	seenNames := make(map[string]struct{})
	addNameOf := func(obj types.Object) {
		name := obj.Name()
		switch obj.(type) {
		case *types.Var, *types.Const:
			if declaredType != nil && !types.AssignableTo(obj.Type(), declaredType) {
				return
			}

			switch {
			case name == "this",
				name == "GopPackage",
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

			name = toLowerCamelCase(name)
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
				objType := unwrapPointerType(obj.Type())
				named, ok := objType.(*types.Named)
				if !ok || !isNamedStructType(named) {
					continue
				}

				walkStruct(named, func(member types.Object, selector *types.Named) bool {
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
func checkValueInputSlot(result *compileResult, expr gopast.Expr, declaredType types.Type) *SpxInputSlot {
	switch expr := expr.(type) {
	case *gopast.BasicLit:
		return createValueInputSlotFromBasicLit(result, expr, declaredType)
	case *gopast.Ident:
		return createValueInputSlotFromIdent(result, expr, declaredType)
	case *gopast.UnaryExpr:
		return createValueInputSlotFromUnaryExpr(result, expr, declaredType)
	case *gopast.CallExpr:
		return createValueInputSlotFromColorFuncCall(result, expr, declaredType)
	}
	return nil
}

// checkAddressInputSlot checks if the expression is an address input slot.
func checkAddressInputSlot(result *compileResult, expr gopast.Expr, declaredType types.Type) *SpxInputSlot {
	if ident, ok := expr.(*gopast.Ident); ok {
		return &SpxInputSlot{
			Kind:   SpxInputSlotKindAddress,
			Accept: SpxInputSlotAccept{Type: SpxInputTypeUnknown},
			Input: SpxInput{
				Kind: SpxInputKindPredefined,
				Type: SpxInputTypeUnknown,
				Name: ident.Name,
			},
			PredefinedNames: collectPredefinedNames(result, expr, declaredType),
			Range:           result.rangeForNode(ident),
		}
	}
	return nil
}

// createValueInputSlotFromBasicLit creates a value input slot from a basic literal.
func createValueInputSlotFromBasicLit(result *compileResult, lit *gopast.BasicLit, declaredType types.Type) *SpxInputSlot {
	input := SpxInput{Kind: SpxInputKindInPlace}
	switch lit.Kind {
	case goptoken.INT:
		input.Type = SpxInputTypeInteger
		v, err := strconv.ParseInt(lit.Value, 0, 64)
		if err != nil {
			return nil
		}
		input.Value = v
	case goptoken.FLOAT:
		input.Type = SpxInputTypeDecimal
		v, err := strconv.ParseFloat(lit.Value, 64)
		if err != nil {
			return nil
		}
		input.Value = v
	case goptoken.STRING:
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
				accept.ResourceContext = util.ToPtr(spxResourceRef.ID.ContextURI())
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
		Range:           result.rangeForNode(lit),
	}
}

// createValueInputSlotFromIdent creates a value input slot from an identifier.
func createValueInputSlotFromIdent(result *compileResult, ident *gopast.Ident, declaredType types.Type) *SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)
	typ := typeInfo.TypeOf(ident)
	if typ == nil {
		return nil
	}
	typ = unwrapPointerType(typ)

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
		if obj != nil && !isSpxPkgObject(obj) {
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
			accept.ResourceContext = util.ToPtr(SpxBackdropResourceContextURI)
		case GetSpxSoundNameType():
			accept.ResourceContext = util.ToPtr(SpxSoundResourceContextURI)
		case GetSpxSpriteNameType():
			accept.ResourceContext = util.ToPtr(SpxSpriteResourceContextURI)
		case GetSpxSpriteCostumeNameType():
			spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, ident)
			if spxSpriteResource == nil {
				return nil
			}
			accept.ResourceContext = util.ToPtr(FormatSpxSpriteCostumeResourceContextURI(spxSpriteResource.Name))
		case GetSpxSpriteAnimationNameType():
			spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, ident)
			if spxSpriteResource == nil {
				return nil
			}
			accept.ResourceContext = util.ToPtr(FormatSpxSpriteAnimationResourceContextURI(spxSpriteResource.Name))
		case GetSpxWidgetNameType():
			accept.ResourceContext = util.ToPtr(SpxWidgetResourceContextURI)
		default:
			return nil
		}
	}

	return &SpxInputSlot{
		Kind:            SpxInputSlotKindValue,
		Accept:          accept,
		Input:           input,
		PredefinedNames: collectPredefinedNames(result, ident, declaredType),
		Range:           result.rangeForNode(ident),
	}
}

// createValueInputSlotFromUnaryExpr creates a value input slot from a unary expression.
func createValueInputSlotFromUnaryExpr(result *compileResult, expr *gopast.UnaryExpr, declaredType types.Type) *SpxInputSlot {
	var inputSlot *SpxInputSlot
	switch x := expr.X.(type) {
	case *gopast.BasicLit:
		inputSlot = createValueInputSlotFromBasicLit(result, x, declaredType)
		if inputSlot == nil {
			return nil
		}

		switch expr.Op {
		case goptoken.ADD:
			// Nothing to do for unary plus.
		case goptoken.SUB:
			switch v := inputSlot.Input.Value.(type) {
			case int64:
				inputSlot.Input.Value = -v
			case float64:
				inputSlot.Input.Value = -v
			default:
				return nil
			}
		case goptoken.XOR:
			switch x.Kind {
			case goptoken.INT:
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
	case *gopast.Ident:
		inputSlot = createValueInputSlotFromIdent(result, x, declaredType)
		if inputSlot == nil {
			return nil
		}

		switch expr.Op {
		case goptoken.NOT:
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
	inputSlot.Range = result.rangeForNode(expr) // Update the range to include the entire unary expression.
	return inputSlot
}

// createValueInputSlotFromColorFuncCall creates a value input slot from an spx
// color function call.
func createValueInputSlotFromColorFuncCall(result *compileResult, callExpr *gopast.CallExpr, declaredType types.Type) *SpxInputSlot {
	typeInfo := getTypeInfo(result.proj)

	fun := funcFromCallExpr(typeInfo, callExpr)
	if fun == nil || !isSpxPkgObject(fun) || !isSpxColorFunc(fun) {
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
		lit, ok := argExpr.(*gopast.BasicLit)
		if !ok {
			return nil
		}

		var val float64
		switch lit.Kind {
		case goptoken.FLOAT:
			floatVal, err := strconv.ParseFloat(lit.Value, 64)
			if err != nil {
				return nil
			}
			val = floatVal
		case goptoken.INT:
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
		Range:           result.rangeForNode(callExpr),
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
func inferSpxSpriteResourceEnclosingNode(result *compileResult, node gopast.Node) *SpxSpriteResource {
	typeInfo := getTypeInfo(result.proj)
	spxFile := result.nodeFilename(node)
	astFile := result.nodeASTFile(node)

	var spxSpriteResource *SpxSpriteResource
	WalkNodesFromInterval(astFile, node.Pos(), node.End(), func(node gopast.Node) bool {
		if node == nil {
			return true
		}

		callExpr, ok := node.(*gopast.CallExpr)
		if !ok {
			return true
		}

		var spxSpriteName string
		if sel, ok := callExpr.Fun.(*gopast.SelectorExpr); ok {
			ident, ok := sel.X.(*gopast.Ident)
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
			} else if vfs.HasSpriteType(result.proj, named) {
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
func isBlank(expr gopast.Expr) bool {
	ident, ok := expr.(*gopast.Ident)
	return ok && ident.Name == "_"
}
