package server

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/goplus/mod/modload"
	"github.com/goplus/mod/xgomod"
	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/internal"
	"github.com/goplus/xgolsw/internal/analysis"
	"github.com/goplus/xgolsw/internal/vfs"
	"github.com/goplus/xgolsw/jsonrpc2"
	"github.com/goplus/xgolsw/xgo"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// MessageReplier is an interface for sending messages back to the client.
type MessageReplier interface {
	// ReplyMessage sends a message back to the client.
	//
	// The message can be one of:
	//   - [jsonrpc2.Response]: sent in response to a call.
	//   - [jsonrpc2.Notification]: sent for server-initiated notifications.
	ReplyMessage(m jsonrpc2.Message) error
}

// FileMapGetter is a function that returns a map of file names to [vfs.MapFile]s.
type FileMapGetter func() map[string]vfs.MapFile

// Server is the core language server implementation that handles LSP messages.
type Server struct {
	workspaceRootURI DocumentURI
	workspaceRootFS  *vfs.MapFS
	replier          MessageReplier
	analyzers        []*analysis.Analyzer
	fileMapGetter    FileMapGetter // TODO(wyvern): Remove this field.
}

func (s *Server) getProj() *xgo.Project {
	return s.workspaceRootFS
}

func (s *Server) getProjWithFile() *xgo.Project {
	proj := s.workspaceRootFS
	proj.UpdateFiles(s.fileMapGetter())
	return proj
}

// New creates a new Server instance.
func New(mapFS *vfs.MapFS, replier MessageReplier, fileMapGetter FileMapGetter) *Server {
	mod := xgomod.New(modload.Default)
	if err := mod.ImportClasses(); err != nil {
		panic(fmt.Errorf("failed to import classes: %w", err))
	}
	mapFS.Path = "main"
	mapFS.Mod = mod
	mapFS.Importer = internal.Importer
	return &Server{
		// TODO(spxls): Initialize request should set workspaceRootURI value
		workspaceRootURI: "file:///",
		workspaceRootFS:  mapFS,
		replier:          replier,
		analyzers:        initAnalyzers(true),
		fileMapGetter:    fileMapGetter,
	}
}

// InitAnalyzers initializes the analyzers for the server.
func initAnalyzers(staticcheck bool) []*analysis.Analyzer {
	analyzers := slices.Collect(maps.Values(analysis.DefaultAnalyzers))
	if staticcheck {
		analyzers = slices.AppendSeq(analyzers, maps.Values(analysis.StaticcheckAnalyzers))
	}
	return analyzers
}

// HandleMessage handles an incoming LSP message.
func (s *Server) HandleMessage(m jsonrpc2.Message) error {
	switch m := m.(type) {
	case *jsonrpc2.Call:
		return s.handleCall(m)
	case *jsonrpc2.Notification:
		return s.handleNotification(m)
	}
	return fmt.Errorf("unsupported message type: %T", m)
}

// handleCall handles a call message.
func (s *Server) handleCall(c *jsonrpc2.Call) error {
	switch c.Method() {
	case "initialize":
		var params InitializeParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		return errors.New("TODO")
	case "shutdown":
		s.runWithResponse(c.ID(), func() (any, error) {
			return nil, nil // Protocol conformance only.
		})
	case "textDocument/hover":
		var params HoverParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentHover(&params)
		})
	case "textDocument/completion":
		var params CompletionParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentCompletion(&params)
		})
	case "textDocument/signatureHelp":
		var params SignatureHelpParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentSignatureHelp(&params)
		})
	case "textDocument/declaration":
		var params DeclarationParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentDeclaration(&params)
		})
	case "textDocument/definition":
		var params DefinitionParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentDefinition(&params)
		})
	case "textDocument/typeDefinition":
		var params TypeDefinitionParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentTypeDefinition(&params)
		})
	case "textDocument/implementation":
		var params ImplementationParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentImplementation(&params)
		})
	case "textDocument/references":
		var params ReferenceParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentReferences(&params)
		})
	case "textDocument/documentHighlight":
		var params DocumentHighlightParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentDocumentHighlight(&params)
		})
	case "textDocument/documentLink":
		var params DocumentLinkParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentDocumentLink(&params)
		})
	case "textDocument/diagnostic":
		var params DocumentDiagnosticParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentDiagnostic(&params)
		})
	case "workspace/diagnostic":
		var params WorkspaceDiagnosticParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.workspaceDiagnostic(&params)
		})
	case "textDocument/formatting":
		var params DocumentFormattingParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentFormatting(&params)
		})
	case "textDocument/prepareRename":
		var params PrepareRenameParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentPrepareRename(&params)
		})
	case "textDocument/rename":
		var params RenameParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentRename(&params)
		})
	case "textDocument/semanticTokens/full":
		var params SemanticTokensParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentSemanticTokensFull(&params)
		})
	case "textDocument/inlayHint":
		var params InlayHintParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.textDocumentInlayHint(&params)
		})
	case "workspace/executeCommand":
		var params ExecuteCommandParams
		if err := UnmarshalJSON(c.Params(), &params); err != nil {
			return s.replyParseError(c.ID(), err)
		}
		s.runWithResponse(c.ID(), func() (any, error) {
			return s.workspaceExecuteCommand(&params)
		})
	default:
		return s.replyMethodNotFound(c.ID(), c.Method())
	}
	return nil
}

// handleNotification handles a notification message.
func (s *Server) handleNotification(n *jsonrpc2.Notification) error {
	switch n.Method() {
	case "initialized":
		var params InitializedParams
		if err := UnmarshalJSON(n.Params(), &params); err != nil {
			return fmt.Errorf("failed to parse initialized params: %w", err)
		}
		return errors.New("TODO")
	case "exit":
		return nil // Protocol conformance only.
	case "textDocument/didOpen":
		var params DidOpenTextDocumentParams
		if err := UnmarshalJSON(n.Params(), &params); err != nil {
			return fmt.Errorf("failed to parse didOpen params: %w", err)
		}

		return s.didOpen(&params)
	case "textDocument/didChange":
		var params DidChangeTextDocumentParams
		if err := UnmarshalJSON(n.Params(), &params); err != nil {
			return fmt.Errorf("failed to parse didChange params: %w", err)
		}
		return s.didChange(&params)
	case "textDocument/didSave":
		var params DidSaveTextDocumentParams
		if err := UnmarshalJSON(n.Params(), &params); err != nil {
			return fmt.Errorf("failed to parse didSave params: %w", err)
		}
		return s.didSave(&params)
	case "textDocument/didClose":
		var params DidCloseTextDocumentParams
		if err := UnmarshalJSON(n.Params(), &params); err != nil {
			return fmt.Errorf("failed to parse didClose params: %w", err)
		}
		return s.didClose(&params)
	}
	return nil
}

// publishDiagnostics sends diagnostic notifications to the client.
func (s *Server) publishDiagnostics(uri DocumentURI, diagnostics []Diagnostic) error {
	params := &PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}
	n, err := jsonrpc2.NewNotification("textDocument/publishDiagnostics", params)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic notification: %w", err)
	}
	return s.replier.ReplyMessage(n)
}

// run runs the given function in a goroutine and replies to the client with any
// errors.
func (s *Server) run(id jsonrpc2.ID, fn func() error) {
	go func() {
		if err := fn(); err != nil {
			s.replyError(id, err)
		}
	}()
}

// runWithResponse runs the given function in a goroutine and handles the response.
func (s *Server) runWithResponse(id jsonrpc2.ID, fn func() (any, error)) {
	s.run(id, func() error {
		result, err := fn()
		resp, err := jsonrpc2.NewResponse(id, result, err)
		if err != nil {
			return err
		}
		return s.replier.ReplyMessage(resp)
	})
}

// replyError replies to the client with an error response.
func (s *Server) replyError(id jsonrpc2.ID, err error) error {
	resp, err := jsonrpc2.NewResponse(id, nil, err)
	if err != nil {
		return err
	}
	return s.replier.ReplyMessage(resp)
}

// replyMethodNotFound replies to the client with a method not found error response.
func (s *Server) replyMethodNotFound(id jsonrpc2.ID, method string) error {
	return s.replyError(id, fmt.Errorf("%w: %s", jsonrpc2.ErrMethodNotFound, method))
}

// replyParseError replies to the client with a parse error response.
func (s *Server) replyParseError(id jsonrpc2.ID, err error) error {
	return s.replyError(id, fmt.Errorf("%w: %s", jsonrpc2.ErrParse, err))
}

// fromDocumentURI returns the relative path from a [DocumentURI].
func (s *Server) fromDocumentURI(documentURI DocumentURI) (string, error) {
	uri := string(documentURI)
	rootURI := string(s.workspaceRootURI)
	if !strings.HasPrefix(uri, rootURI) {
		return "", fmt.Errorf("document URI %q does not have workspace root URI %q as prefix", uri, rootURI)
	}
	return strings.TrimPrefix(uri, rootURI), nil
}

// toDocumentURI returns the [DocumentURI] for a relative path.
func (s *Server) toDocumentURI(path string) DocumentURI {
	return DocumentURI(string(s.workspaceRootURI) + path)
}

// posDocumentURI returns the [DocumentURI] for the given position in the project.
func (s *Server) posDocumentURI(proj *xgo.Project, pos xgotoken.Pos) DocumentURI {
	return s.toDocumentURI(xgoutil.PosFilename(proj, pos))
}

// nodeDocumentURI returns the [DocumentURI] for the given node in the project.
func (s *Server) nodeDocumentURI(proj *xgo.Project, node xgoast.Node) DocumentURI {
	return s.posDocumentURI(proj, node.Pos())
}

// locationForPos returns the [Location] for the given position in the project.
func (s *Server) locationForPos(proj *xgo.Project, pos xgotoken.Pos) Location {
	return Location{
		URI:   s.posDocumentURI(proj, pos),
		Range: RangeForPos(proj, pos),
	}
}

// locationForNode returns the [Location] for the given node in the project.
func (s *Server) locationForNode(proj *xgo.Project, node xgoast.Node) Location {
	return Location{
		URI:   s.nodeDocumentURI(proj, node),
		Range: RangeForNode(proj, node),
	}
}
