package server

import (
	"fmt"
	"go/types"
	"io/fs"

	"github.com/goplus/gogen"
	gopast "github.com/goplus/gop/ast"
	gopscanner "github.com/goplus/gop/scanner"
	"github.com/goplus/goxlsw/gop"
	"github.com/goplus/goxlsw/internal/analysis/ast/inspector"
	"github.com/goplus/goxlsw/internal/analysis/passes/inspect"
	"github.com/goplus/goxlsw/internal/analysis/protocol"
	"github.com/qiniu/x/errors"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_diagnostic
func (s *Server) textDocumentDiagnostic(params *DocumentDiagnosticParams) (*DocumentDiagnosticReport, error) {
	file, err := s.fromDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get file path from document uri %q: %w", params.TextDocument.URI, err)
	}
	return &DocumentDiagnosticReport{Value: RelatedFullDocumentDiagnosticReport{
		FullDocumentDiagnosticReport: FullDocumentDiagnosticReport{
			Kind:  string(DiagnosticFull),
			Items: s.diagnose(s.getProj(), file),
		},
	}}, nil
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#workspace_diagnostic
func (s *Server) workspaceDiagnostic(params *WorkspaceDiagnosticParams) (*WorkspaceDiagnosticReport, error) {
	proj := s.getProj()
	var items []WorkspaceDocumentDiagnosticReport
	proj.RangeParsableFiles(func(file string) {
		items = append(items, WorkspaceDocumentDiagnosticReport{
			Value: WorkspaceFullDocumentDiagnosticReport{
				URI: s.toDocumentURI(file),
				FullDocumentDiagnosticReport: FullDocumentDiagnosticReport{
					Kind:  string(DiagnosticFull),
					Items: s.diagnose(proj, file),
				},
			},
		})
	})
	return &WorkspaceDiagnosticReport{Items: items}, nil
}

// diagnose generates diagnostic information for a specific file in the given
// project. It returns nil if not found.
//
// The checkes are performed in the following order:
//  1. AST parsing
//  2. Type checking
//  3. Static analysis
func (s *Server) diagnose(proj *gop.Project, file string) (diags []Diagnostic) {
	// 1. AST parsing
	astFile, err := proj.AST(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		var (
			errorList gopscanner.ErrorList
			codeError *gogen.CodeError
		)
		if errors.As(err, &errorList) {
			// Handle parse errors.
			for _, e := range errorList {
				diags = append(diags, Diagnostic{
					Severity: SeverityError,
					Range:    rangeForASTFilePosition(proj, astFile, e.Pos),
					Message:  e.Msg,
				})
			}
		} else if errors.As(err, &codeError) {
			// Handle code generation errors.
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Range:    rangeForPos(proj, codeError.Pos),
				Message:  codeError.Error(),
			})
		} else {
			// Handle unknown errors (including recovered panics).
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Message:  fmt.Sprintf("failed to parse file: %v", err),
			})
		}
	}
	if astFile == nil {
		return
	}
	if astFile.Name.Name != "main" {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Range:    rangeForASTFileNode(proj, astFile, astFile.Name),
			Message:  "package name must be main",
		})
		return
	}
	astFilePos := proj.Fset.Position(astFile.Pos())

	// 2. Type checking
	handleErr := func(err error) {
		if typeErr, ok := err.(types.Error); ok {
			if !typeErr.Pos.IsValid() {
				// This should never happen. types.Error.Pos is expected to always be valid.
				// If it's not, it's likely due to a bug in upstream gop/gogen logic.
				// We panic here to surface the issue instead of silently skipping it.
				panic(fmt.Errorf("invalid position for types.Error: %w", typeErr))
			}
			position := typeErr.Fset.Position(typeErr.Pos)
			if position.Filename == astFilePos.Filename {
				diags = append(diags, Diagnostic{
					Severity: SeverityError,
					Range:    rangeForPos(proj, typeErr.Pos),
					Message:  typeErr.Msg,
				})
			}
		}
	}
	_, typeInfo, err, _ := proj.TypeInfo()
	if err != nil {
		switch err := err.(type) {
		case errors.List:
			for _, e := range err {
				handleErr(e)
			}
		default:
			handleErr(err)
		}
	}

	// 3. Static analysis
	pass := &protocol.Pass{
		Fset:      proj.Fset,
		Files:     []*gopast.File{astFile},
		TypesInfo: typeInfo,
		Report: func(d protocol.Diagnostic) {
			diags = append(diags, Diagnostic{
				Range:    rangeForPosEnd(proj, d.Pos, d.End),
				Severity: SeverityError,
				Message:  d.Message,
			})
		},
		ResultOf: map[*protocol.Analyzer]any{
			inspect.Analyzer: inspector.New([]*gopast.File{astFile}),
		},
	}
	for _, analyzer := range s.analyzers {
		an := analyzer.Analyzer()
		if _, err := an.Run(pass); err != nil {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Message:  fmt.Sprintf("failed to run analyzer %q: %v", an.Name, err),
			})
		}
	}
	return
}
