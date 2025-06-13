package server

import (
	"go/types"
	"path"
	"regexp"

	xgoast "github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/x/typesutil"
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
		typeName := extractTypeName(xgoutil.DerefType(tv.Type))
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
