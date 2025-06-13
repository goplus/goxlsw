package server

import (
	"go/types"

	xgoast "github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_references
func (s *Server) textDocumentReferences(params *ReferenceParams) ([]Location, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	position := ToPosition(result.proj, astFile, params.Position)

	ident := xgoutil.IdentAtPosition(result.proj, astFile, position)
	typeInfo := getTypeInfo(result.proj)
	obj := typeInfo.ObjectOf(ident)
	if obj == nil {
		return nil, nil
	}

	var locations []Location

	locations = append(locations, s.findReferenceLocations(result, obj)...)

	if fn, ok := obj.(*types.Func); ok && fn.Type().(*types.Signature).Recv() != nil {
		locations = append(locations, s.handleMethodReferences(result, fn)...)
		locations = append(locations, s.handleEmbeddedFieldReferences(result, obj)...)
	}

	if params.Context.IncludeDeclaration {
		defIdent := xgoutil.DefIdentFor(typeInfo, obj)
		if defIdent == nil {
			objPos := obj.Pos()
			if xgoutil.PosTokenFile(result.proj, objPos) != nil {
				locations = append(locations, s.locationForPos(result.proj, objPos))
			}
		} else if xgoutil.NodeTokenFile(result.proj, defIdent) != nil {
			locations = append(locations, s.locationForNode(result.proj, defIdent))
		}
	}

	return DedupeLocations(locations), nil
}

// findReferenceLocations returns all locations where the given object is referenced.
func (s *Server) findReferenceLocations(result *compileResult, obj types.Object) []Location {
	refIdents := xgoutil.RefIdentsFor(getTypeInfo(result.proj), obj)
	if len(refIdents) == 0 {
		return nil
	}
	locations := make([]Location, 0, len(refIdents))
	for _, refIdent := range refIdents {
		locations = append(locations, s.locationForNode(result.proj, refIdent))
	}
	return locations
}

// handleMethodReferences finds all references to a method, including interface
// implementations and interface method references.
func (s *Server) handleMethodReferences(result *compileResult, fn *types.Func) []Location {
	var locations []Location
	recvType := fn.Type().(*types.Signature).Recv().Type()
	if types.IsInterface(recvType) {
		iface, ok := recvType.(*types.Interface)
		if !ok {
			return nil
		}
		methodName := fn.Name()
		locations = append(locations, s.findEmbeddedInterfaceReferences(result, iface, methodName)...)
		locations = append(locations, s.findImplementingMethodReferences(result, iface, methodName)...)
	} else {
		locations = append(locations, s.findInterfaceMethodReferences(result, fn)...)
	}
	return locations
}

// findEmbeddedInterfaceReferences finds references to methods in interfaces
// that embed the given interface.
func (s *Server) findEmbeddedInterfaceReferences(result *compileResult, iface *types.Interface, methodName string) []Location {
	var locations []Location
	seenIfaces := make(map[*types.Interface]bool)
	typeInfo := getTypeInfo(result.proj)

	var find func(*types.Interface)
	find = func(current *types.Interface) {
		if seenIfaces[current] {
			return
		}
		seenIfaces[current] = true

		xgoutil.RangeASTSpecs(result.proj, token.TYPE, func(spec xgoast.Spec) {
			typeSpec := spec.(*xgoast.TypeSpec)
			typeName := typeInfo.ObjectOf(typeSpec.Name)
			if typeName == nil {
				return
			}
			embedIface, ok := typeName.Type().Underlying().(*types.Interface)
			if !ok {
				return
			}

			for typ := range embedIface.EmbeddedTypes() {
				if types.Identical(typ, current) {
					method, index, _ := types.LookupFieldOrMethod(embedIface, false, typeName.Pkg(), methodName)
					if method != nil && index != nil {
						locations = append(locations, s.findReferenceLocations(result, method)...)
					}
					find(embedIface)
				}
			}
		})
	}
	find(iface)
	return locations
}

// findImplementingMethodReferences finds references to all methods that
// implement the given interface method.
func (s *Server) findImplementingMethodReferences(result *compileResult, iface *types.Interface, methodName string) []Location {
	typeInfo := getTypeInfo(result.proj)
	var locations []Location
	xgoutil.RangeASTSpecs(result.proj, token.TYPE, func(spec xgoast.Spec) {
		typeSpec := spec.(*xgoast.TypeSpec)
		typeName := typeInfo.ObjectOf(typeSpec.Name)
		if typeName == nil {
			return
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok || !types.Implements(named, iface) {
			return
		}

		method, index, _ := types.LookupFieldOrMethod(named, false, named.Obj().Pkg(), methodName)
		if method == nil || index == nil {
			return
		}
		locations = append(locations, s.findReferenceLocations(result, method)...)
	})
	return locations
}

// findInterfaceMethodReferences finds references to interface methods that this
// method implements, including methods from embedded interfaces.
func (s *Server) findInterfaceMethodReferences(result *compileResult, fn *types.Func) []Location {
	typeInfo := getTypeInfo(result.proj)
	var locations []Location
	recvType := fn.Type().(*types.Signature).Recv().Type()
	seenIfaces := make(map[*types.Interface]bool)

	xgoutil.RangeASTSpecs(result.proj, token.TYPE, func(spec xgoast.Spec) {
		typeSpec := spec.(*xgoast.TypeSpec)
		typeName := typeInfo.ObjectOf(typeSpec.Name)
		if typeName == nil {
			return
		}
		ifaceType, ok := typeName.Type().Underlying().(*types.Interface)
		if !ok || !types.Implements(recvType, ifaceType) || seenIfaces[ifaceType] {
			return
		}
		seenIfaces[ifaceType] = true

		method, index, _ := types.LookupFieldOrMethod(ifaceType, false, typeName.Pkg(), fn.Name())
		if method == nil || index == nil {
			return
		}
		locations = append(locations, s.findReferenceLocations(result, method)...)
		locations = append(locations, s.findEmbeddedInterfaceReferences(result, ifaceType, fn.Name())...)
	})
	return locations
}

// handleEmbeddedFieldReferences finds all references through embedded fields.
func (s *Server) handleEmbeddedFieldReferences(result *compileResult, obj types.Object) []Location {
	typeInfo := getTypeInfo(result.proj)
	var locations []Location
	if fn, ok := obj.(*types.Func); ok {
		recv := fn.Type().(*types.Signature).Recv()
		if recv == nil {
			return nil
		}

		seenTypes := make(map[types.Type]bool)
		xgoutil.RangeASTSpecs(result.proj, token.TYPE, func(spec xgoast.Spec) {
			typeSpec := spec.(*xgoast.TypeSpec)
			typeName := typeInfo.ObjectOf(typeSpec.Name)
			if typeName == nil {
				return
			}
			named, ok := typeName.Type().(*types.Named)
			if !ok {
				return
			}

			locations = append(locations, s.findEmbeddedMethodReferences(result, fn, named, recv.Type(), seenTypes)...)
		})
	}
	return locations
}

// findEmbeddedMethodReferences recursively finds all references to a method
// through embedded fields.
func (s *Server) findEmbeddedMethodReferences(result *compileResult, fn *types.Func, named *types.Named, targetType types.Type, seenTypes map[types.Type]bool) []Location {
	if seenTypes[named] {
		return nil
	}
	seenTypes[named] = true

	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	var locations []Location
	hasEmbed := false
	for field := range st.Fields() {
		if !field.Embedded() {
			continue
		}

		if types.Identical(field.Type(), targetType) {
			hasEmbed = true

			method, _, _ := types.LookupFieldOrMethod(named, false, fn.Pkg(), fn.Name())
			if method != nil {
				locations = append(locations, s.findReferenceLocations(result, method)...)
			}
		}

		if fieldNamed, ok := field.Type().(*types.Named); ok {
			locations = append(locations, s.findEmbeddedMethodReferences(result, fn, fieldNamed, targetType, seenTypes)...)
		}
	}
	if hasEmbed {
		typeInfo := getTypeInfo(result.proj)
		xgoutil.RangeASTSpecs(result.proj, token.TYPE, func(spec xgoast.Spec) {
			typeSpec := spec.(*xgoast.TypeSpec)
			typeName := typeInfo.ObjectOf(typeSpec.Name)
			if typeName == nil {
				return
			}
			named, ok := typeName.Type().(*types.Named)
			if !ok {
				return
			}

			locations = append(locations, s.findEmbeddedMethodReferences(result, fn, named, named, seenTypes)...)
		})
	}
	return locations
}
