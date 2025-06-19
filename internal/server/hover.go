package server

import (
	"fmt"
	"go/doc"
	"strings"

	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_hover
func (s *Server) textDocumentHover(params *HoverParams) (*Hover, error) {
	proj := s.getProjWithFile()
	if proj == nil {
		return nil, nil
	}

	spxFile, err := s.fromDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get file path from document URI %q: %w", params.TextDocument.URI, err)
	}

	astFile, _ := proj.AST(spxFile)
	if astFile == nil {
		return nil, nil
	}
	if !astFile.Pos().IsValid() {
		return nil, nil
	}
	position := ToPosition(proj, astFile, params.Position)

	if spxResourceRef := spxResourceRefAtASTFilePosition(proj, astFile, position); spxResourceRef != nil {
		return &Hover{
			Contents: MarkupContent{
				Kind:  Markdown,
				Value: spxResourceRef.ID.URI().HTML(),
			},
			Range: RangeForNode(proj, spxResourceRef.Node),
		}, nil
	}
	ident := xgoutil.IdentAtPosition(proj, astFile, position)
	if ident == nil {
		// Check if the position is within an import declaration.
		// If so, return the package documentation.
		rpkg := spxImportsAtASTFilePosition(proj, astFile, position)
		if rpkg != nil {
			return &Hover{
				Contents: MarkupContent{
					Kind:  Markdown,
					Value: doc.Synopsis(rpkg.Pkg.Doc),
				},
				Range: RangeForNode(proj, rpkg.Node),
			}, nil
		}
		return nil, nil
	}

	spxDefs := SpxDefinitionsForIdent(proj, ident)
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
		Range: RangeForNode(proj, ident),
	}, nil
}
