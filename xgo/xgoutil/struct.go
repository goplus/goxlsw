package xgoutil

import "go/types"

// IsNamedStructType reports whether the given named type is a struct type.
func IsNamedStructType(named *types.Named) bool {
	if named == nil {
		return false
	}
	_, ok := named.Underlying().(*types.Struct)
	return ok
}

// IsXGoClassStructType reports whether the given named type is an XGo class struct type.
func IsXGoClassStructType(named *types.Named) bool {
	if named == nil {
		return false
	}
	obj := named.Obj()
	if obj == nil {
		return false
	}
	pkg := obj.Pkg()
	if !IsMarkedAsXGoPackage(pkg) {
		return false
	}

	// FIXME: This is a workaround for the fact that XGo does not have the ability to
	// recognize XGo class struct types.
	switch PkgPath(pkg) + "." + obj.Name() {
	case "github.com/goplus/spx/v2.Game",
		"github.com/goplus/spx/v2.SpriteImpl":
		return true
	}

	return false
}

// WalkStruct walks a struct and calls the given onMember for each field and
// method. If onMember returns false, the walk is stopped.
func WalkStruct(named *types.Named, onMember func(member types.Object, selector *types.Named) bool) {
	if named == nil {
		return
	}
	walked := make(map[*types.Named]struct{})
	seenMembers := make(map[string]struct{})
	var walk func(named *types.Named, namedPath []*types.Named) bool
	walk = func(named *types.Named, namedPath []*types.Named) bool {
		if _, ok := walked[named]; ok {
			return true
		}
		walked[named] = struct{}{}

		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			return true
		}

		selector := named
		for _, named := range namedPath {
			if !IsExportedOrInMainPkg(named.Obj()) {
				break
			}
			selector = named
			if IsXGoClassStructType(selector) {
				break
			}
		}

		for field := range st.Fields() {
			if _, ok := seenMembers[field.Name()]; ok || !IsExportedOrInMainPkg(field) {
				continue
			}
			seenMembers[field.Name()] = struct{}{}

			if !onMember(field, selector) {
				return false
			}
		}
		for method := range named.Methods() {
			if _, ok := seenMembers[method.Name()]; ok || !IsExportedOrInMainPkg(method) {
				continue
			}
			seenMembers[method.Name()] = struct{}{}

			if !onMember(method, selector) {
				return false
			}
		}
		for field := range st.Fields() {
			if !field.Embedded() {
				continue
			}
			fieldType := DerefType(field.Type())
			namedField, ok := fieldType.(*types.Named)
			if !ok || !IsNamedStructType(namedField) {
				continue
			}

			if !walk(namedField, append(namedPath, namedField)) {
				return false
			}
		}
		return true
	}
	walk(named, []*types.Named{named})
}
