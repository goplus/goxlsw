package server

import (
	"go/types"

	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_implementation
func (s *Server) textDocumentImplementation(params *ImplementationParams) (any, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	position := ToPosition(result.proj, astFile, params.Position)

	ident := xgoutil.IdentAtPosition(result.proj, astFile, position)
	obj := getTypeInfo(result.proj).ObjectOf(ident)
	if !xgoutil.IsInMainPkg(obj) {
		return nil, nil
	}

	if method, ok := obj.(*types.Func); ok && method.Type().(*types.Signature).Recv() != nil {
		if recv := method.Type().(*types.Signature).Recv().Type(); types.IsInterface(recv) {
			locations := s.findImplementingMethodDefinitions(result, recv.(*types.Interface), method.Name())
			return DedupeLocations(locations), nil
		}
	}

	return s.locationForPos(result.proj, obj.Pos()), nil
}

// findImplementingMethodDefinitions finds the definition locations of all
// methods that implement the given interface method.
func (s *Server) findImplementingMethodDefinitions(result *compileResult, iface *types.Interface, methodName string) []Location {
	var implementations []Location
	typeInfo := getTypeInfo(result.proj)
	for _, obj := range typeInfo.Defs {
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if !types.Implements(named, iface) {
			continue
		}

		for method := range named.Methods() {
			if method.Name() == methodName {
				implementations = append(implementations, s.locationForPos(result.proj, method.Pos()))
			}
		}
	}
	return implementations
}
