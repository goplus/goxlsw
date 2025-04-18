package server

import (
	"bytes"
	"fmt"
	"time"

	"github.com/goplus/goxlsw/gop"
	"github.com/goplus/goxlsw/jsonrpc2"
	"github.com/goplus/goxlsw/protocol"
)

// didOpen handles the textDocument/didOpen notification from the LSP client.
// It updates the project with the new file content and publishes diagnostics.
// The document URI is converted to a filesystem path, and a file change is created
// with the document's content and version number.
func (s *Server) didOpen(params *DidOpenTextDocumentParams) error {
	path, err := s.fromDocumentURI(params.TextDocument.URI)
	if err != nil {
		return err
	}

	return s.didModifyFile([]FileChange{{
		Path:    path,
		Content: []byte(params.TextDocument.Text),
		Version: int(params.TextDocument.Version),
	}})
}

// didChange handles the textDocument/didChange notification from the LSP client.
// It applies document changes to the project and publishes updated diagnostics.
// For simplicity, this implementation only uses the latest content change
// rather than applying incremental changes.
func (s *Server) didChange(params *DidChangeTextDocumentParams) error {
	path, err := s.fromDocumentURI(params.TextDocument.URI)
	if err != nil {
		return err
	}

	content, err := s.changedText(path, params.ContentChanges)
	if err != nil {
		return err
	}

	// Create a file change record
	changes := []FileChange{{
		Path:    path,
		Content: content,
		Version: int(params.TextDocument.Version),
	}}

	return s.didModifyFile(changes)
}

// didSave handles the textDocument/didSave notification from the LSP client.
// If the notification includes the document text, the project is updated.
// Otherwise, no change is made since the document content hasn't changed.
// Save notifications typically don't include version numbers, so 0 is used.
func (s *Server) didSave(params *DidSaveTextDocumentParams) error {
	// If text is included in save notification, update the file
	if params.Text != nil {
		path, err := s.fromDocumentURI(params.TextDocument.URI)
		if err != nil {
			return err
		}

		return s.didModifyFile([]FileChange{{
			Path:    path,
			Content: []byte(*params.Text),
			Version: int(time.Now().UnixMilli()),
		}})
	}
	return nil
}

// didClose handles the textDocument/didClose notification from the LSP client.
// When a document is closed, its diagnostics are cleared by sending an empty
// diagnostics array to the client.
func (s *Server) didClose(params *DidCloseTextDocumentParams) error {
	// Clear diagnostics when file is closed
	return s.publishDiagnostics(params.TextDocument.URI, nil)
}

// didModifyFile is a shared implementation for handling document modifications.
// It updates the project with file changes and asynchronously publishes diagnostics.
// The function:
// 1. Updates the project's files with the provided changes
// 2. Starts a goroutine to generate and publish diagnostics for each changed file
// 3. Returns immediately after updating files for better responsiveness
func (s *Server) didModifyFile(changes []FileChange) error {
	// 1. Update files synchronously
	s.ModifyFiles(changes)

	// 2. Get the project instance at this point
	proj := s.getProj()

	// 3. Asynchronously generate and publish diagnostics
	// This allows for quick response while diagnostics computation happens in background
	go func() {
		for _, change := range changes {
			// Convert path to URI for diagnostics
			uri := s.toDocumentURI(change.Path)

			// Get diags from AST and type checking
			diags := s.diagnose(proj, change.Path)

			// Publish diagnostics
			if err := s.publishDiagnostics(uri, diags); err != nil {
				// Log error but continue
				continue
			}
		}
	}()

	return nil
}

// changedText processes document content changes from the client.
// It supports two modes of operation:
// 1. Full replacement: Replace the entire document content (when only one change with no range is provided)
// 2. Incremental updates: Apply specific changes to portions of the document
//
// Returns the updated document content or an error if the changes couldn't be applied.
func (s *Server) changedText(uri string, changes []protocol.TextDocumentContentChangeEvent) ([]byte, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("%w: no content changes provided", jsonrpc2.ErrInternal)
	}

	// Check if the client sent the full content of the file.
	// We accept a full content change even if the server expected incremental changes.
	if len(changes) == 1 && changes[0].Range == nil && changes[0].RangeLength == 0 {
		// Full replacement mode
		return []byte(changes[0].Text), nil
	}

	// Incremental update mode
	return s.applyIncrementalChanges(uri, changes)
}

// applyIncrementalChanges applies a sequence of changes to the document content.
// For each change, it:
// 1. Computes the byte offsets for the specified range
// 2. Verifies the range is valid
// 3. Replaces the specified range with the new text
//
// Returns the updated document content or an error if the changes couldn't be applied.
func (s *Server) applyIncrementalChanges(path string, changes []protocol.TextDocumentContentChangeEvent) ([]byte, error) {
	// Get current file content
	file, ok := s.getProj().File(path)
	if !ok {
		return nil, fmt.Errorf("%w: file not found", jsonrpc2.ErrInternal)
	}

	content := file.Content

	// Apply each change sequentially
	for _, change := range changes {
		// Ensure the change includes range information
		if change.Range == nil {
			return nil, fmt.Errorf("%w: unexpected nil range for change", jsonrpc2.ErrInternal)
		}

		// Convert LSP positions to byte offsets
		start := positionOffset(content, change.Range.Start)
		end := positionOffset(content, change.Range.End)

		// Validate range
		if end < start {
			return nil, fmt.Errorf("%w: invalid range for content change", jsonrpc2.ErrInternal)
		}

		// Apply the change
		var buf bytes.Buffer
		buf.Write(content[:start])
		buf.WriteString(change.Text)
		buf.Write(content[end:])
		content = buf.Bytes()
	}

	return content, nil
}

// FileChange represents a file change.
type FileChange struct {
	Path    string
	Content []byte
	Version int // Version is timestamp in milliseconds
}

// ModifyFiles modifies files in the project.
func (s *Server) ModifyFiles(changes []FileChange) {
	// Get project
	p := s.getProj()
	// Process all changes in a batch
	for _, change := range changes {
		// Create new file with updated content
		file := &gop.FileImpl{
			Content: change.Content,
			Version: change.Version,
		}

		// Check if file exists
		if oldFile, ok := p.File(change.Path); ok {
			// Only update if version is newer
			if change.Version > oldFile.Version {
				p.PutFile(change.Path, file)
			}
		} else {
			// New file, always add
			p.PutFile(change.Path, file)
		}
	}
}
