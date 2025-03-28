package server

import (
	"go/types"

	"github.com/goplus/goxlsw/gop/goputil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_declaration
func (s *Server) textDocumentDeclaration(params *DeclarationParams) (any, error) {
	return s.textDocumentDefinition(&DefinitionParams{
		TextDocumentPositionParams: params.TextDocumentPositionParams,
		WorkDoneProgressParams:     params.WorkDoneProgressParams,
		PartialResultParams:        params.PartialResultParams,
	})
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_definition
func (s *Server) textDocumentDefinition(params *DefinitionParams) (any, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	position := toPosition(result.proj, astFile, params.Position)

	ident := goputil.IdentAtPosition(result.proj, astFile, position)
	obj := getTypeInfo(result.proj).ObjectOf(ident)
	if !isMainPkgObject(obj) {
		return nil, nil
	}

	defIdent := goputil.DefIdentFor(result.proj, obj)
	if defIdent == nil {
		objPos := obj.Pos()
		if goputil.PosTokenFile(result.proj, objPos) == nil {
			return nil, nil
		}
		return result.locationForPos(objPos), nil
	} else if goputil.NodeTokenFile(result.proj, defIdent) == nil {
		return nil, nil
	}
	return result.locationForNode(defIdent), nil
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_typeDefinition
func (s *Server) textDocumentTypeDefinition(params *TypeDefinitionParams) (any, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	position := toPosition(result.proj, astFile, params.Position)

	ident := goputil.IdentAtPosition(result.proj, astFile, position)
	obj := getTypeInfo(result.proj).ObjectOf(ident)
	if !isMainPkgObject(obj) {
		return nil, nil
	}

	objType := unwrapPointerType(obj.Type())
	named, ok := objType.(*types.Named)
	if !ok {
		return nil, nil
	}

	objPos := named.Obj().Pos()
	if goputil.PosTokenFile(result.proj, objPos) == nil {
		return nil, nil
	}
	return result.locationForPos(objPos), nil
}
