package server

import (
	"go/types"
	"strings"

	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_signatureHelp
func (s *Server) textDocumentSignatureHelp(params *SignatureHelpParams) (*SignatureHelp, error) {
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
	if obj == nil {
		return nil, nil
	}

	fun, ok := obj.(*types.Func)
	if !ok {
		return nil, nil
	}
	sig, ok := fun.Type().(*types.Signature)
	if !ok {
		return nil, nil
	}

	var paramsInfo []ParameterInformation
	for param := range sig.Params().Variables() {
		paramsInfo = append(paramsInfo, ParameterInformation{
			Label: param.Name() + " " + GetSimplifiedTypeString(param.Type()),
			// TODO: Add documentation.
		})
	}

	label := fun.Name() + "("
	if sig.Params().Len() > 0 {
		var paramLabels []string
		for _, p := range paramsInfo {
			paramLabels = append(paramLabels, p.Label)
		}
		label += strings.Join(paramLabels, ", ")
	}
	label += ")"

	if results := sig.Results(); results != nil && results.Len() > 0 {
		var returnTypes []string
		for result := range results.Variables() {
			returnTypes = append(returnTypes, GetSimplifiedTypeString(result.Type()))
		}
		label += " (" + strings.Join(returnTypes, ", ") + ")"
	}

	return &SignatureHelp{
		Signatures: []SignatureInformation{{
			Label: label,
			// TODO: Add documentation.
			Parameters: paramsInfo,
		}},
	}, nil
}
