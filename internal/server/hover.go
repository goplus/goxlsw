package server

import (
	"go/doc"
	"strings"

	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_hover
func (s *Server) textDocumentHover(params *HoverParams) (*Hover, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}
	if !astFile.Pos().IsValid() {
		return nil, nil
	}
	position := ToPosition(result.proj, astFile, params.Position)

	if spxResourceRef := result.spxResourceRefAtASTFilePosition(astFile, position); spxResourceRef != nil {
		return &Hover{
			Contents: MarkupContent{
				Kind:  Markdown,
				Value: spxResourceRef.ID.URI().HTML(),
			},
			Range: RangeForNode(result.proj, spxResourceRef.Node),
		}, nil
	}

	ident := xgoutil.IdentAtPosition(result.proj, astFile, position)
	if ident == nil {
		// Check if the position is within an import declaration.
		// If so, return the package documentation.
		rpkg := result.spxImportsAtASTFilePosition(astFile, position)
		if rpkg != nil {
			return &Hover{
				Contents: MarkupContent{
					Kind:  Markdown,
					Value: doc.Synopsis(rpkg.Pkg.Doc),
				},
				Range: RangeForNode(result.proj, rpkg.Node),
			}, nil
		}
		return nil, nil
	}

	spxDefs := result.spxDefinitionsForIdent(ident)
	if spxDefs == nil {
		return nil, nil
	}

	var hoverContent strings.Builder
	for _, spxDef := range spxDefs {
		hoverContent.WriteString(spxDef.HTML())
	}
	return &Hover{
		Contents: MarkupContent{
			Kind:  Markdown,
			Value: hoverContent.String(),
		},
		Range: RangeForNode(result.proj, ident),
	}, nil
}
