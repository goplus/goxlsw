package server

import (
	"go/types"
	"path"
	"regexp"
	"slices"

	gopast "github.com/goplus/gop/ast"
	"github.com/goplus/goxlsw/gop"
	"github.com/goplus/goxlsw/gop/goputil"
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
func SelectorTypeNameForIdent(proj *gop.Project, ident *gopast.Ident) string {
	astFile := goputil.NodeASTFile(proj, ident)
	if astFile == nil {
		return ""
	}

	typeInfo := getTypeInfo(proj)

	if path, _ := goputil.PathEnclosingInterval(astFile, ident.Pos(), ident.End()); len(path) > 0 {
		for _, node := range slices.Backward(path) {
			sel, ok := node.(*gopast.SelectorExpr)
			if !ok {
				continue
			}
			tv, ok := typeInfo.Types[sel.X]
			if !ok {
				continue
			}

			switch typ := goputil.DerefType(tv.Type).(type) {
			case *types.Named:
				obj := typ.Obj()
				typeName := obj.Name()
				if IsInSpxPkg(obj) && typeName == "SpriteImpl" {
					typeName = "Sprite"
				}
				return typeName
			case *types.Interface:
				if typ.String() == "interface{}" {
					return ""
				}
				return typ.String()
			}
		}
	}

	obj := typeInfo.ObjectOf(ident)
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	if IsInSpxPkg(obj) {
		astFileScope := typeInfo.Scopes[astFile]
		innermostScope := goputil.InnermostScopeAt(proj, ident.Pos())
		if innermostScope == astFileScope || (astFile.HasShadowEntry() && goputil.InnermostScopeAt(proj, astFile.ShadowEntry.Pos()) == innermostScope) {
			spxFile := goputil.NodeFilename(proj, ident)
			if spxFileBaseName := path.Base(spxFile); spxFileBaseName == "main.spx" {
				return "Game"
			}
			return "Sprite"
		}
	}
	switch obj := obj.(type) {
	case *types.Var:
		if !obj.IsField() {
			return ""
		}

		for _, def := range typeInfo.Defs {
			if def == nil {
				continue
			}
			named, ok := goputil.DerefType(def.Type()).(*types.Named)
			if !ok || named.Obj().Pkg() != obj.Pkg() || !goputil.IsNamedStructType(named) {
				continue
			}

			var typeName string
			goputil.WalkStruct(named, func(member types.Object, selector *types.Named) bool {
				if field, ok := member.(*types.Var); ok && field == obj {
					typeName = selector.Obj().Name()
					return false
				}
				return true
			})
			if IsInSpxPkg(obj) && typeName == "SpriteImpl" {
				typeName = "Sprite"
			}
			if typeName != "" {
				return typeName
			}
		}
	case *types.Func:
		recv := obj.Type().(*types.Signature).Recv()
		if recv == nil {
			return ""
		}

		switch typ := goputil.DerefType(recv.Type()).(type) {
		case *types.Named:
			obj := typ.Obj()
			typeName := obj.Name()
			if IsInSpxPkg(obj) && typeName == "SpriteImpl" {
				typeName = "Sprite"
			}
			return typeName
		case *types.Interface:
			if typ.String() == "interface{}" {
				return ""
			}
			return typ.String()
		}
	}
	return ""
}
