package server

import (
	"go/types"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgo/x/typesutil"
	"github.com/goplus/xgolsw/internal/pkgdata"
	"github.com/goplus/xgolsw/internal/vfs"
	"github.com/goplus/xgolsw/pkgdoc"
	"github.com/goplus/xgolsw/xgo"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// spxEventHandlerFuncNameRE is the regular expression of the spx event handler
// function name.
var spxEventHandlerFuncNameRE = regexp.MustCompile(`^on[A-Z]\w*$`)

// IsSpxEventHandlerFuncName reports whether the given function name is an
// spx event handler function name.
func IsSpxEventHandlerFuncName(name string) bool {
	return spxEventHandlerFuncNameRE.MatchString(name)
}

// IsInSpxPkg reports whether the given object is defined in the spx package.
func IsInSpxPkg(obj types.Object) bool {
	return obj != nil && obj.Pkg() == GetSpxPkg()
}

// GetSimplifiedTypeString returns the string representation of the given type,
// with the spx package name omitted while other packages use their short names.
func GetSimplifiedTypeString(typ types.Type) string {
	return types.TypeString(typ, func(p *types.Package) string {
		if p == GetSpxPkg() {
			return ""
		}
		return p.Name()
	})
}

// SelectorTypeNameForIdent returns the selector type name for the given
// identifier. It returns empty string if no selector can be inferred.
func SelectorTypeNameForIdent(proj *xgo.Project, ident *xgoast.Ident) string {
	astFile := xgoutil.NodeASTFile(proj, ident)
	if astFile == nil {
		return ""
	}

	typeInfo := getTypeInfo(proj)

	// Check for selector expression context first.
	if typeName := tryGetSelectorContext(typeInfo, astFile, ident); typeName != "" {
		return typeName
	}

	obj := typeInfo.ObjectOf(ident)
	if obj == nil || obj.Pkg() == nil {
		return ""
	}

	// Handle spx package's implicit receiver semantics.
	if typeName := tryGetSpxImplicitReceiver(proj, typeInfo, astFile, ident, obj); typeName != "" {
		return typeName
	}

	// Infer type from object properties.
	return getTypeFromObject(typeInfo, obj)
}

// spxImportsAtASTFilePosition returns the import at the given position in the given AST file.
func spxImportsAtASTFilePosition(proj *xgo.Project, astFile *xgoast.File, position xgotoken.Position) *SpxReferencePkg {
	fset := proj.Fset
	for _, imp := range astFile.Imports {
		nodePos := fset.Position(imp.Pos())
		nodeEnd := fset.Position(imp.End())
		if nodePos.Filename != position.Filename ||
			position.Line != nodePos.Line ||
			position.Column < nodePos.Column ||
			position.Column > nodeEnd.Column {
			continue
		}

		pkg, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		pkgDoc, err := pkgdata.GetPkgDoc(pkg)
		if err != nil {
			continue
		}
		return &SpxReferencePkg{
			Pkg:     pkgDoc,
			PkgPath: pkg,
			Node:    imp,
		}
	}
	return nil
}

// spxResourceRefAtASTFilePosition returns the spx resource reference at the
// given position in the given AST file.
func spxResourceRefAtASTFilePosition(proj *xgo.Project, astFile *xgoast.File, position xgotoken.Position) *SpxResourceRef {
	pkg, typeInfo, _, _ := proj.TypeInfo()
	filter := xgoutil.FilterExprAtPosition(proj, astFile, position)
	spxSpriteTypes := make(map[types.Type]struct{})
	vfs.RangeSpriteNames(proj, func(name string) bool {
		obj := pkg.Scope().Lookup(name)
		if obj != nil {
			named, ok := obj.Type().(*types.Named)
			if ok {
				spxSpriteTypes[named] = struct{}{}
			}
		}
		return true
	})

	resourceSet := inspectForSpxResourceSet(proj)
	if resourceSet == nil {
		return nil
	}

	// Check all identifier definitions.
	for ident, obj := range typeInfo.Defs {
		if !filter(ident) {
			continue
		}

		switch obj.(type) {
		case *types.Const, *types.Var:
			if ident.Obj == nil {
				break
			}
			valueSpec, ok := ident.Obj.Decl.(*xgoast.ValueSpec)
			if !ok {
				break
			}
			idx := slices.Index(valueSpec.Names, ident)
			if idx < 0 || idx >= len(valueSpec.Values) {
				break
			}
			expr := valueSpec.Values[idx]

			return inspectSpxResourceRefForTypeAtExpr(proj, expr, xgoutil.DerefType(obj.Type()), nil)
		}

		v, ok := obj.(*types.Var)
		if !ok {
			continue
		}
		varType, ok := v.Type().(*types.Named)
		if !ok {
			continue
		}

		var (
			isSpxSoundResourceAutoBinding  bool
			isSpxSpriteResourceAutoBinding bool
		)
		switch varType {
		case GetSpxSoundType():
			isSpxSoundResourceAutoBinding = resourceSet.Sound(v.Name()) != nil
		case GetSpxSpriteType():
			isSpxSpriteResourceAutoBinding = resourceSet.Sprite(v.Name()) != nil
		default:
			_, hasSpxSpriteType := spxSpriteTypes[varType]
			isSpxSpriteResourceAutoBinding = v.Name() == varType.Obj().Name() && hasSpxSpriteType
		}
		if !isSpxSoundResourceAutoBinding && !isSpxSpriteResourceAutoBinding {
			continue
		}

		return inspectSpxResourceRefForTypeAtExpr(proj, ident, xgoutil.DerefType(obj.Type()), nil)
	}

	// Check all type-checked expressions.
	for expr, tv := range typeInfo.Types {
		if expr == nil || !expr.Pos().IsValid() || tv.IsType() || tv.Type == nil {
			continue
		}

		switch expr := expr.(type) {
		case *xgoast.CallExpr:
			var use bool
			for _, arg := range expr.Args {
				if filter(arg) {
					use = true
				}
			}
			if !use {
				continue
			}
			funcTV, ok := typeInfo.Types[expr.Fun]
			if !ok {
				continue
			}
			funcSig, ok := funcTV.Type.(*types.Signature)
			if !ok {
				continue
			}

			var spxSpriteResource *SpxSpriteResource
			if recv := funcSig.Recv(); recv != nil {
				recvType := xgoutil.DerefType(recv.Type())
				switch recvType {
				case GetSpxSpriteType(), GetSpxSpriteImplType():
					spxSpriteResource = inspectSpxSpriteResourceAtExpr(proj, resourceSet, expr, recvType)
				}
			}

			var lastParamType types.Type
			for i, arg := range expr.Args {
				var paramType types.Type
				if i < funcSig.Params().Len() {
					paramType = xgoutil.DerefType(funcSig.Params().At(i).Type())
					lastParamType = paramType
				} else {
					// Use the last parameter type for variadic functions.
					paramType = lastParamType
				}

				// Handle slice/array parameter types.
				if sliceType, ok := paramType.(*types.Slice); ok {
					paramType = xgoutil.DerefType(sliceType.Elem())
				} else if arrayType, ok := paramType.(*types.Array); ok {
					paramType = xgoutil.DerefType(arrayType.Elem())
				}

				if sliceLit, ok := arg.(*xgoast.SliceLit); ok {
					for _, elt := range sliceLit.Elts {
						if filter(elt) {
							return inspectSpxResourceRefForTypeAtExpr(proj, elt, paramType, spxSpriteResource)
						}
					}
				} else {
					if filter(arg) {
						return inspectSpxResourceRefForTypeAtExpr(proj, arg, paramType, spxSpriteResource)
					}
				}
			}
		default:
			if !filter(expr) {
				continue
			}
			typ := xgoutil.DerefType(tv.Type)
			if _, ok := spxSpriteTypes[typ]; ok || isInspectableSpxResourceType(typ) {
				return inspectSpxResourceRefForTypeAtExpr(proj, expr, typ, nil)
			}
		}
	}
	return nil
}

// inspectSpxResourceRefForTypeAtExpr inspects an spx resource reference for a
// given type at an expression.
func inspectSpxResourceRefForTypeAtExpr(proj *xgo.Project, expr xgoast.Expr, typ types.Type, spxSpriteResource *SpxSpriteResource) *SpxResourceRef {
	if ident, ok := expr.(*xgoast.Ident); ok {
		switch typ {
		case GetSpxBackdropNameType(),
			GetSpxSpriteNameType(),
			GetSpxSoundNameType(),
			GetSpxWidgetNameType():
			astFile := xgoutil.NodeASTFile(proj, ident)
			if astFile == nil {
				return nil
			}

			xgoutil.WalkPathEnclosingInterval(astFile, ident.Pos(), ident.End(), false, func(node xgoast.Node) bool {
				assignStmt, ok := node.(*xgoast.AssignStmt)
				if !ok {
					return true
				}

				idx := slices.IndexFunc(assignStmt.Lhs, func(lhs xgoast.Expr) bool {
					return lhs == ident
				})
				if idx < 0 || idx >= len(assignStmt.Rhs) {
					return true
				}
				expr = assignStmt.Rhs[idx]
				return false
			})
		}
	}

	switch typ {
	case GetSpxBackdropNameType():
		return inspectSpxBackdropResourceRefAtExpr(proj, expr, typ)
	case GetSpxSpriteNameType(), GetSpxSpriteType():
		return inspectSpxSpriteResourceRefAtExpr(proj, expr, typ)
	case GetSpxSpriteCostumeNameType():
		return inspectSpxSpriteCostumeResourceRefAtExpr(proj, spxSpriteResource, expr, typ)
	case GetSpxSpriteAnimationNameType():
		return inspectSpxSpriteAnimationResourceRefAtExpr(proj, spxSpriteResource, expr, typ)
	case GetSpxSoundNameType(), GetSpxSoundType():
		return inspectSpxSoundResourceRefAtExpr(proj, expr, typ)
	case GetSpxWidgetNameType():
		return inspectSpxWidgetResourceRefAtExpr(proj, expr, typ)
	default:
		return inspectSpxSpriteResourceRefAtExpr(proj, expr, typ)
	}
}

// inspectSpxWidgetResourceRefAtExpr inspects an spx widget resource reference
// at an expression. It returns the spx widget resource if it was successfully
// retrieved.
func inspectSpxWidgetResourceRefAtExpr(proj *xgo.Project, expr xgoast.Expr, declaredType types.Type) *SpxResourceRef {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxWidgetNameType() {
		return nil
	}

	spxWidgetName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*xgoast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	if spxWidgetName == "" {
		return nil
	}
	return &SpxResourceRef{
		ID:   SpxWidgetResourceID{WidgetName: spxWidgetName},
		Kind: spxResourceRefKind,
		Node: expr,
	}
}

// inspectSpxSoundResourceRefAtExpr inspects an spx sound resource reference at
// an expression. It returns the spx sound resource if it was successfully
// retrieved.
func inspectSpxSoundResourceRefAtExpr(proj *xgo.Project, expr xgoast.Expr, declaredType types.Type) *SpxResourceRef {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var (
		spxSoundName       string
		spxResourceRefKind SpxResourceRefKind
	)
	switch typ {
	case GetSpxSoundNameType():
		var ok bool
		spxSoundName, ok = xgoutil.StringLitOrConstValue(expr, exprTV)
		if !ok {
			return nil
		}
		spxResourceRefKind = SpxResourceRefKindStringLiteral
		if _, ok := expr.(*xgoast.Ident); ok {
			spxResourceRefKind = SpxResourceRefKindConstantReference
		}
	case GetSpxSoundType():
		ident, ok := expr.(*xgoast.Ident)
		if !ok {
			return nil
		}
		obj := typeInfo.ObjectOf(ident)
		if obj == nil {
			return nil
		}
		spxSoundName = obj.Name()
		defIdent := xgoutil.DefIdentFor(typeInfo, obj)
		if defIdent == ident {
			spxResourceRefKind = SpxResourceRefKindAutoBinding
		} else {
			spxResourceRefKind = SpxResourceRefKindAutoBindingReference
		}
	default:
		return nil
	}
	if spxSoundName == "" {
		return nil
	}
	return &SpxResourceRef{
		ID:   SpxSoundResourceID{SoundName: spxSoundName},
		Kind: spxResourceRefKind,
		Node: expr,
	}
}

// inspectSpxSpriteAnimationResourceRefAtExpr inspects an spx sprite animation
// resource reference at an expression. It returns the spx sprite animation
// resource if it was successfully retrieved.
func inspectSpxSpriteAnimationResourceRefAtExpr(proj *xgo.Project, spxSpriteResource *SpxSpriteResource, expr xgoast.Expr, declaredType types.Type) *SpxResourceRef {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxSpriteAnimationNameType() {
		return nil
	}

	spxSpriteAnimationName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*xgoast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	if spxSpriteAnimationName == "" {
		return nil
	}
	return &SpxResourceRef{
		ID:   SpxSpriteAnimationResourceID{SpriteName: spxSpriteResource.Name, AnimationName: spxSpriteAnimationName},
		Kind: spxResourceRefKind,
		Node: expr,
	}
}

// inspectSpxSpriteCostumeResourceRefAtExpr inspects an spx sprite costume
// resource reference at an expression. It returns the spx sprite costume
// resource if it was successfully retrieved.
func inspectSpxSpriteCostumeResourceRefAtExpr(proj *xgo.Project, spxSpriteResource *SpxSpriteResource, expr xgoast.Expr, declaredType types.Type) *SpxResourceRef {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxSpriteCostumeNameType() {
		return nil
	}

	spxSpriteCostumeName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}

	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*xgoast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	return &SpxResourceRef{
		ID:   SpxSpriteCostumeResourceID{SpriteName: spxSpriteResource.Name, CostumeName: spxSpriteCostumeName},
		Kind: spxResourceRefKind,
		Node: expr,
	}
}

// inspectSpxBackdropResourceRefAtExpr inspects an spx backdrop resource
// reference at an expression. It returns the spx backdrop resource if it was
// successfully retrieved.
func inspectSpxBackdropResourceRefAtExpr(proj *xgo.Project, expr xgoast.Expr, declaredType types.Type) *SpxResourceRef {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxBackdropNameType() {
		return nil
	}

	spxBackdropName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	if spxBackdropName == "" {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*xgoast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	return &SpxResourceRef{
		ID:   SpxBackdropResourceID{BackdropName: spxBackdropName},
		Kind: spxResourceRefKind,
		Node: expr,
	}
}

// inspectForSpxResourceSet inspects for spx resource set in main.spx.
func inspectForSpxResourceSet(proj *xgo.Project) *SpxResourceSet {
	var mainSpxFile string
	proj.RangeFiles(func(path string) bool {
		if strings.HasSuffix(path, "main.spx") {
			mainSpxFile = path
			return false
		}
		return true
	})
	if mainSpxFile == "" {
		return nil
	}
	mainASTFile, _ := proj.AST(mainSpxFile)
	typeInfo := getTypeInfo(proj)

	var spxResourceRootDir string
	for expr, tv := range typeInfo.Types {
		if expr == nil || !expr.Pos().IsValid() || expr.Pos() < mainASTFile.Pos() || expr.End() > mainASTFile.End() {
			continue
		}

		callExpr, ok := expr.(*xgoast.CallExpr)
		if !ok || len(callExpr.Args) == 0 || tv.Type != GetSpxGoptGameRunFunc().Type() {
			continue
		}
		firstArg := callExpr.Args[0]
		firstArgTV, ok := typeInfo.Types[firstArg]
		if !ok {
			continue
		}

		if types.AssignableTo(firstArgTV.Type, types.Typ[types.String]) {
			spxResourceRootDir, _ = xgoutil.StringLitOrConstValue(firstArg, firstArgTV)
		}
		break
	}
	if spxResourceRootDir == "" {
		spxResourceRootDir = "assets"
	}
	spxResourceRootFS := vfs.Sub(proj, spxResourceRootDir)

	spxResourceSet, err := NewSpxResourceSet(spxResourceRootFS)
	if err != nil {
		return nil
	}
	return spxResourceSet
}

// inspectSpxSpriteResourceRefAtExpr inspects an spx sprite resource reference
// at an expression. It returns the spx sprite resource if it was successfully
// retrieved.
func inspectSpxSpriteResourceAtExpr(proj *xgo.Project, resourceSet *SpxResourceSet, expr xgoast.Expr, declaredType types.Type) *SpxSpriteResource {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var spxSpriteName string
	if callExpr, ok := expr.(*xgoast.CallExpr); ok {
		switch fun := callExpr.Fun.(type) {
		case *xgoast.Ident:
			spxSpriteName = strings.TrimSuffix(path.Base(xgoutil.NodeFilename(proj, callExpr)), ".spx")
		case *xgoast.SelectorExpr:
			ident, ok := fun.X.(*xgoast.Ident)
			if !ok {
				return nil
			}
			return inspectSpxSpriteResourceAtExpr(proj, resourceSet, ident, declaredType)
		default:
			return nil
		}
	}
	if spxSpriteName == "" {
		if typ == GetSpxSpriteNameType() {
			var ok bool
			spxSpriteName, ok = xgoutil.StringLitOrConstValue(expr, exprTV)
			if !ok {
				return nil
			}
		} else {
			ident, ok := expr.(*xgoast.Ident)
			if !ok {
				return nil
			}
			obj := typeInfo.ObjectOf(ident)
			if obj == nil {
				return nil
			}
			spxSpriteName = obj.Name()
		}
	}

	spxSpriteResource := resourceSet.Sprite(spxSpriteName)
	if spxSpriteResource == nil {
		return nil
	}
	return spxSpriteResource
}

// inspectSpxSpriteResourceRefAtExpr inspects an spx sprite resource reference
// at an expression. It returns the spx sprite resource if it was successfully
// retrieved.
func inspectSpxSpriteResourceRefAtExpr(proj *xgo.Project, expr xgoast.Expr, declaredType types.Type) *SpxResourceRef {
	typeInfo := getTypeInfo(proj)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var spxSpriteName string
	if callExpr, ok := expr.(*xgoast.CallExpr); ok {
		switch fun := callExpr.Fun.(type) {
		case *xgoast.Ident:
			spxSpriteName = strings.TrimSuffix(path.Base(xgoutil.NodeFilename(proj, callExpr)), ".spx")
		case *xgoast.SelectorExpr:
			ident, ok := fun.X.(*xgoast.Ident)
			if !ok {
				return nil
			}
			return inspectSpxSpriteResourceRefAtExpr(proj, ident, declaredType)
		default:
			return nil
		}
	}
	if spxSpriteName == "" {
		var spxResourceRefKind SpxResourceRefKind
		if typ == GetSpxSpriteNameType() {
			var ok bool
			spxSpriteName, ok = xgoutil.StringLitOrConstValue(expr, exprTV)
			if !ok {
				return nil
			}
			spxResourceRefKind = SpxResourceRefKindStringLiteral
			if _, ok := expr.(*xgoast.Ident); ok {
				spxResourceRefKind = SpxResourceRefKindConstantReference
			}
		} else {
			ident, ok := expr.(*xgoast.Ident)
			if !ok {
				return nil
			}
			obj := typeInfo.ObjectOf(ident)
			if obj == nil {
				return nil
			}
			spxSpriteName = obj.Name()
			defIdent := xgoutil.DefIdentFor(typeInfo, obj)
			if defIdent == ident {
				spxResourceRefKind = SpxResourceRefKindAutoBinding
			} else {
				spxResourceRefKind = SpxResourceRefKindAutoBindingReference
			}
		}
		if spxSpriteName == "" {
			return nil
		}
		return &SpxResourceRef{
			ID:   SpxSpriteResourceID{SpriteName: spxSpriteName},
			Kind: spxResourceRefKind,
			Node: expr,
		}
	}

	return nil
}

// spxDefinitionsForIdent returns all spx definitions for the given identifier.
// It returns multiple definitions only if the identifier is an XGo
// overloadable function.
func SpxDefinitionsForIdent(proj *xgo.Project, ident *xgoast.Ident) []SpxDefinition {
	if ident.Name == "_" {
		return nil
	}
	typeInfo := getTypeInfo(proj)
	return spxDefinitionsFor(proj, typeInfo.ObjectOf(ident), SelectorTypeNameForIdent(proj, ident))
}

// spxDefinitionsFor returns all spx definitions for the given object. It
// returns multiple definitions only if the object is an XGo overloadable
// function.
func spxDefinitionsFor(proj *xgo.Project, obj types.Object, selectorTypeName string) []SpxDefinition {
	if obj == nil {
		return nil
	}
	if xgoutil.IsInBuiltinPkg(obj) {
		return []SpxDefinition{GetSpxDefinitionForBuiltinObj(obj)}
	}

	var pkgDoc *pkgdoc.PkgDoc
	if xgoutil.IsInMainPkg(obj) {
		pkgDoc = getPkgDoc(proj)
	} else {
		pkgPath := xgoutil.PkgPath(obj.Pkg())
		pkgDoc, _ = pkgdata.GetPkgDoc(pkgPath)
	}

	switch obj := obj.(type) {
	case *types.Var:
		return []SpxDefinition{GetSpxDefinitionForVar(obj, selectorTypeName, xgoutil.IsDefinedInClassFieldsDecl(proj, obj), pkgDoc)}
	case *types.Const:
		return []SpxDefinition{GetSpxDefinitionForConst(obj, pkgDoc)}
	case *types.TypeName:
		return []SpxDefinition{GetSpxDefinitionForType(obj, pkgDoc)}
	case *types.Func:
		if defIdent := xgoutil.DefIdentFor(getTypeInfo(proj), obj); defIdent != nil && defIdent.Implicit() {
			return nil
		}
		if xgoutil.IsUnexpandableXGoOverloadableFunc(obj) {
			return nil
		}
		if funcOverloads := xgoutil.ExpandXGoOverloadableFunc(obj); funcOverloads != nil {
			defs := make([]SpxDefinition, 0, len(funcOverloads))
			for _, funcOverload := range funcOverloads {
				defs = append(defs, GetSpxDefinitionForFunc(funcOverload, selectorTypeName, pkgDoc))
			}
			return defs
		}
		return []SpxDefinition{GetSpxDefinitionForFunc(obj, selectorTypeName, pkgDoc)}
	case *types.PkgName:
		return []SpxDefinition{GetSpxDefinitionForPkg(obj, pkgDoc)}
	}
	return nil
}

// func SpxResourceRefAtPosition(proj *xgo.Project, astFile *ast.File, position token.Position) *ast.Ident {
// 	expr := exprAtPosition(proj, astFile, position)
// }

// func exprAtPosition(proj *xgo.Project, astFile *ast.File, position token.Position) *xgoast.Ident {
// 	return nil
// }

// tryGetSelectorContext checks if the identifier is in a selector expression context.
func tryGetSelectorContext(typeInfo *typesutil.Info, astFile *xgoast.File, ident *xgoast.Ident) string {
	var typeName string
	xgoutil.WalkPathEnclosingInterval(astFile, ident.Pos(), ident.End(), true, func(node xgoast.Node) bool {
		sel, ok := node.(*xgoast.SelectorExpr)
		if !ok {
			return true
		}
		tv, ok := typeInfo.Types[sel.X]
		if !ok {
			return true
		}
		typeName = extractTypeName(xgoutil.DerefType(tv.Type))
		return typeName == ""
	})
	return typeName
}

// tryGetSpxImplicitReceiver handles spx package's special implicit receiver semantics.
func tryGetSpxImplicitReceiver(proj *xgo.Project, typeInfo *typesutil.Info, astFile *xgoast.File, ident *xgoast.Ident, obj types.Object) string {
	if !IsInSpxPkg(obj) {
		return ""
	}

	astFileScope := typeInfo.Scopes[astFile]
	innermostScope := xgoutil.InnermostScopeAt(proj, ident.Pos())

	// Check if we're in the right scope context.
	if innermostScope != astFileScope && (!astFile.HasShadowEntry() || xgoutil.InnermostScopeAt(proj, astFile.ShadowEntry.Pos()) != innermostScope) {
		return ""
	}

	spxFile := xgoutil.NodeFilename(proj, ident)
	if path.Base(spxFile) == "main.spx" {
		return "Game"
	}
	return "Sprite"
}

// getTypeFromObject infers type from the identifier's object.
func getTypeFromObject(typeInfo *typesutil.Info, obj types.Object) string {
	switch obj := obj.(type) {
	case *types.Var:
		if !obj.IsField() {
			return ""
		}
		return findFieldOwnerType(typeInfo, obj)
	case *types.Func:
		sig, ok := obj.Type().(*types.Signature)
		if !ok {
			return ""
		}
		recv := sig.Recv()
		if recv == nil {
			return ""
		}
		return extractTypeName(xgoutil.DerefType(recv.Type()))
	}
	return ""
}

// extractTypeName extracts a clean type name from a types.Type.
func extractTypeName(typ types.Type) string {
	switch typ := typ.(type) {
	case *types.Named:
		obj := typ.Obj()
		typeName := obj.Name()
		if IsInSpxPkg(obj) && typeName == "SpriteImpl" {
			return "Sprite"
		}
		return typeName
	case *types.Interface:
		if typ.String() == "interface{}" {
			return ""
		}
		return typ.String()
	}
	return ""
}

// findFieldOwnerType finds the type that owns a given field.
func findFieldOwnerType(typeInfo *typesutil.Info, field *types.Var) string {
	if !field.IsField() {
		return ""
	}

	fieldPkg := field.Pkg()
	if fieldPkg == nil {
		return ""
	}

	// Search through named types in the same package.
	scope := fieldPkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}

		named, ok := typeName.Type().(*types.Named)
		if !ok || !xgoutil.IsNamedStructType(named) {
			continue
		}

		// Check if this struct contains our field.
		if ownerName := checkStructForField(named, field, fieldPkg); ownerName != "" {
			return ownerName
		}
	}

	// Fallback: search through all type definitions.
	return searchAllDefsForField(typeInfo, field)
}

// checkStructForField checks if a struct type contains the given field.
func checkStructForField(named *types.Named, field *types.Var, fieldPkg *types.Package) string {
	foundObj, indices, _ := types.LookupFieldOrMethod(named, false, fieldPkg, field.Name())
	if foundObj == nil || len(indices) == 0 {
		return ""
	}

	foundField, ok := foundObj.(*types.Var)
	if !ok || foundField != field {
		return ""
	}

	typeName := named.Obj().Name()
	if IsInSpxPkg(named.Obj()) && typeName == "SpriteImpl" {
		return "Sprite"
	}
	return typeName
}

// searchAllDefsForField is a fallback method that searches all type definitions.
func searchAllDefsForField(typeInfo *typesutil.Info, field *types.Var) string {
	fieldPkg := field.Pkg()
	for _, def := range typeInfo.Defs {
		if def == nil || def.Pkg() != fieldPkg {
			continue
		}

		named, ok := xgoutil.DerefType(def.Type()).(*types.Named)
		if !ok || !xgoutil.IsNamedStructType(named) {
			continue
		}

		if ownerName := checkStructForField(named, field, fieldPkg); ownerName != "" {
			return ownerName
		}
	}
	return ""
}
