package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"

	"github.com/goplus/xgolsw/protocol"
)

type (
	URI         = protocol.URI
	DocumentURI = protocol.DocumentURI
	Position    = protocol.Position
	Range       = protocol.Range
	Location    = protocol.Location

	TextEdit      = protocol.TextEdit
	WorkspaceEdit = protocol.WorkspaceEdit

	TextDocumentPositionParams = protocol.TextDocumentPositionParams
	TextDocumentIdentifier     = protocol.TextDocumentIdentifier

	InsertTextFormat = protocol.InsertTextFormat

	MarkupContent = protocol.MarkupContent

	DocumentHighlightParams = protocol.DocumentHighlightParams
	DocumentHighlight       = protocol.DocumentHighlight

	DocumentFormattingParams = protocol.DocumentFormattingParams

	PrepareRenameParams = protocol.PrepareRenameParams
	RenameParams        = protocol.RenameParams

	Diagnostic                            = protocol.Diagnostic
	DocumentDiagnosticParams              = protocol.DocumentDiagnosticParams
	WorkspaceDiagnosticParams             = protocol.WorkspaceDiagnosticParams
	DocumentDiagnosticReport              = protocol.DocumentDiagnosticReport
	FullDocumentDiagnosticReport          = protocol.FullDocumentDiagnosticReport
	RelatedFullDocumentDiagnosticReport   = protocol.RelatedFullDocumentDiagnosticReport
	WorkspaceDiagnosticReport             = protocol.WorkspaceDiagnosticReport
	WorkspaceDocumentDiagnosticReport     = protocol.WorkspaceDocumentDiagnosticReport
	WorkspaceFullDocumentDiagnosticReport = protocol.WorkspaceFullDocumentDiagnosticReport
	PublishDiagnosticsParams              = protocol.PublishDiagnosticsParams

	CompletionParams                = protocol.CompletionParams
	CompletionItemKind              = protocol.CompletionItemKind
	CompletionItem                  = protocol.CompletionItem
	Or_CompletionItem_documentation = protocol.Or_CompletionItem_documentation

	DocumentLinkParams = protocol.DocumentLinkParams
	DocumentLink       = protocol.DocumentLink

	DeclarationParams    = protocol.DeclarationParams
	DefinitionParams     = protocol.DefinitionParams
	TypeDefinitionParams = protocol.TypeDefinitionParams

	ReferenceParams  = protocol.ReferenceParams
	ReferenceContext = protocol.ReferenceContext

	HoverParams = protocol.HoverParams
	Hover       = protocol.Hover

	ImplementationParams = protocol.ImplementationParams

	SemanticTokenTypes     = protocol.SemanticTokenTypes
	SemanticTokenModifiers = protocol.SemanticTokenModifiers
	SemanticTokensParams   = protocol.SemanticTokensParams
	SemanticTokens         = protocol.SemanticTokens

	SignatureHelpParams  = protocol.SignatureHelpParams
	SignatureHelp        = protocol.SignatureHelp
	SignatureInformation = protocol.SignatureInformation
	ParameterInformation = protocol.ParameterInformation

	InitializeParams     = protocol.InitializeParams
	InitializedParams    = protocol.InitializedParams
	ExecuteCommandParams = protocol.ExecuteCommandParams

	DidOpenTextDocumentParams   = protocol.DidOpenTextDocumentParams
	DidChangeTextDocumentParams = protocol.DidChangeTextDocumentParams
	DidCloseTextDocumentParams  = protocol.DidCloseTextDocumentParams
	DidSaveTextDocumentParams   = protocol.DidSaveTextDocumentParams

	InlayHintParams = protocol.InlayHintParams
	InlayHint       = protocol.InlayHint
	InlayHintKind   = protocol.InlayHintKind
)

const (
	SeverityError   = protocol.SeverityError
	SeverityWarning = protocol.SeverityWarning

	TextCompletion      = protocol.TextCompletion
	ClassCompletion     = protocol.ClassCompletion
	InterfaceCompletion = protocol.InterfaceCompletion
	StructCompletion    = protocol.StructCompletion
	VariableCompletion  = protocol.VariableCompletion
	ConstantCompletion  = protocol.ConstantCompletion
	KeywordCompletion   = protocol.KeywordCompletion
	FieldCompletion     = protocol.FieldCompletion
	MethodCompletion    = protocol.MethodCompletion
	FunctionCompletion  = protocol.FunctionCompletion
	ModuleCompletion    = protocol.ModuleCompletion

	DiagnosticFull = protocol.DiagnosticFull

	Markdown = protocol.Markdown
	Text     = protocol.Text

	Write = protocol.Write
	Read  = protocol.Read

	PlainTextTextFormat = protocol.PlainTextTextFormat
	SnippetTextFormat   = protocol.SnippetTextFormat

	NamespaceType = protocol.NamespaceType
	TypeType      = protocol.TypeType
	InterfaceType = protocol.InterfaceType
	StructType    = protocol.StructType
	EnumType      = protocol.EnumType
	EnumMember    = protocol.EnumMember
	VariableType  = protocol.VariableType
	ParameterType = protocol.ParameterType
	FunctionType  = protocol.FunctionType
	MethodType    = protocol.MethodType
	PropertyType  = protocol.PropertyType
	KeywordType   = protocol.KeywordType
	CommentType   = protocol.CommentType
	StringType    = protocol.StringType
	NumberType    = protocol.NumberType
	OperatorType  = protocol.OperatorType
	LabelType     = protocol.LabelType

	ModDeclaration    = protocol.ModDeclaration
	ModReadonly       = protocol.ModReadonly
	ModStatic         = protocol.ModStatic
	ModDefinition     = protocol.ModDefinition
	ModDefaultLibrary = protocol.ModDefaultLibrary

	Type      = protocol.Type
	Parameter = protocol.Parameter
)

// UnmarshalJSON unmarshals msg into the variable pointed to by params.
// In JSONRPC, optional messages may be "null", in which case it is a no-op.
func UnmarshalJSON(msg json.RawMessage, v any) error {
	if len(msg) == 0 || bytes.Equal(msg, []byte("null")) {
		return nil
	}
	return json.Unmarshal(msg, v)
}

// toURI converts a string to a [URI].
func toURI(s string) *URI {
	u := URI(s)
	return &u
}

// SpxRenameResourceParams represents parameters to rename an spx resource in
// the workspace.
type SpxRenameResourceParams struct {
	// The spx resource.
	Resource SpxResourceIdentifier `json:"resource"`
	// The new name of the spx resource.
	NewName string `json:"newName"`
}

// SpxResourceIdentifier identifies an spx resource.
type SpxResourceIdentifier struct {
	// The spx resource's URI.
	URI SpxResourceURI `json:"uri"`
}

// SpxResourceURI represents a URI string for an spx resource.
type SpxResourceURI string

// HTML returns the HTML representation of the spx resource URI.
func (u SpxResourceURI) HTML() string {
	return fmt.Sprintf("<resource-preview resource=%q />\n", template.HTMLEscapeString(string(u)))
}

// SpxResourceContextURI represents a URI for resource context.
// Examples:
// - `spx://resources/sprites`
// - `spx://resources/sounds`
// - `spx://resources/sprites/<sName>/costumes`
type SpxResourceContextURI string

// SpxGetDefinitionsParams represents parameters to get definitions at a
// specific position in a document.
type SpxGetDefinitionsParams struct {
	// The text document position params.
	protocol.TextDocumentPositionParams
}

// SpxDefinitionIdentifier identifies an spx definition.
type SpxDefinitionIdentifier struct {
	// Full name of source package.
	// If not provided, it's assumed to be kind-statement.
	// If `main`, it's the current user package.
	// Examples:
	// - `fmt`
	// - `github.com/goplus/spx/v2`
	// - `main`
	Package *string `json:"package,omitempty"`

	// Exported name of the definition.
	// If not provided, it's assumed to be kind-package.
	// Examples:
	// - `Println`
	// - `Sprite`
	// - `Sprite.turn`
	// - `for_statement_with_single_condition`
	Name *string `json:"name,omitempty"`

	// Overload Identifier.
	OverloadID *string `json:"overloadId,omitempty"`
}

// String implements [fmt.Stringer].
func (id SpxDefinitionIdentifier) String() string {
	s := "xgo:"
	if id.Package != nil {
		s += *id.Package
	}
	if id.Name != nil {
		s += "?" + url.QueryEscape(*id.Name)
		if id.OverloadID != nil {
			s += "#" + url.QueryEscape(*id.OverloadID)
		}
	}
	return s
}

// SpxGetInputSlotsParams represents parameters to get input slots for a
// specific document.
type SpxGetInputSlotsParams struct {
	// The text document indentifier.
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
}

// SpxInputSlot represents a modifiable item in the code.
type SpxInputSlot struct {
	Kind            SpxInputSlotKind   `json:"kind"`
	Accept          SpxInputSlotAccept `json:"accept"`
	Input           SpxInput           `json:"input"`
	PredefinedNames []string           `json:"predefinedNames"`
	Range           Range              `json:"range"`
}

// SpxInputSlotKind represents the kind of input slot.
type SpxInputSlotKind string

// SpxInputSlotKind constants.
const (
	// SpxInputSlotKindValue slot accepts value, which may be an in-place value or a predefined identifier.
	SpxInputSlotKindValue SpxInputSlotKind = "value"

	// SpxInputSlotKindAddress slot accepts address, which must be a predefined identifier.
	SpxInputSlotKindAddress SpxInputSlotKind = "address"
)

// SpxInputSlotAccept represents info about what inputs are accepted by a slot.
type SpxInputSlotAccept struct {
	// Type of input accepted by the slot.
	Type SpxInputType `json:"type"`

	// Resource context for SpxInputTypeResourceName.
	// Only valid when Type is SpxInputTypeResourceName.
	ResourceContext *SpxResourceContextURI `json:"resourceContext,omitempty"`
}

// SpxInputType represents the type of input for a slot.
type SpxInputType string

// SpxInputType constants.
const (
	SpxInputTypeInteger       SpxInputType = "integer"
	SpxInputTypeDecimal       SpxInputType = "decimal"
	SpxInputTypeString        SpxInputType = "string"
	SpxInputTypeBoolean       SpxInputType = "boolean"
	SpxInputTypeResourceName  SpxInputType = "spx-resource-name"
	SpxInputTypeDirection     SpxInputType = "spx-direction"
	SpxInputTypeColor         SpxInputType = "spx-color"
	SpxInputTypeEffectKind    SpxInputType = "spx-effect-kind"
	SpxInputTypeKey           SpxInputType = "spx-key"
	SpxInputTypePlayAction    SpxInputType = "spx-play-action"
	SpxInputTypeSpecialObj    SpxInputType = "spx-special-obj"
	SpxInputTypeRotationStyle SpxInputType = "spx-rotation-style"
	SpxInputTypeUnknown       SpxInputType = "unknown"
)

// SpxInputTypeSpxColorConstructor represents the name for color constructors.
type SpxInputTypeSpxColorConstructor string

// SpxInputTypeSpxColorConstructor constants.
const (
	SpxInputTypeSpxColorConstructorHSB  SpxInputTypeSpxColorConstructor = "HSB"
	SpxInputTypeSpxColorConstructorHSBA SpxInputTypeSpxColorConstructor = "HSBA"
)

// SpxColorInputValue represents the value structure for an [SpxInput] when its
// type is [SpxInputTypeColor] and kind is [SpxInputKindInPlace].
type SpxColorInputValue struct {
	Constructor SpxInputTypeSpxColorConstructor `json:"constructor"`
	Args        []float64                       `json:"args"`
}

// SpxInput represents the current input in a slot.
type SpxInput struct {
	Kind  SpxInputKind `json:"kind"`
	Type  SpxInputType `json:"type"`
	Value any          `json:"value,omitempty"` // For InPlace kind
	Name  string       `json:"name,omitempty"`  // For Predefined kind
}

// SpxInputKind represents the kind of input.
type SpxInputKind string

// SpxInputKind constants.
const (
	// SpxInputKindInPlace in-place value like "hello world", 123, true, etc.
	SpxInputKindInPlace SpxInputKind = "in-place"

	// SpxInputKindPredefined reference to user predefined identifier.
	SpxInputKindPredefined SpxInputKind = "predefined"
)

// SpxResourceRefDocumentLinkData represents data for an spx resource reference
// document link.
type SpxResourceRefDocumentLinkData struct {
	// The kind of the spx resource reference.
	Kind SpxResourceRefKind `json:"kind"`
}

// CompletionItemData represents data in a completion item.
type CompletionItemData struct {
	// The corresponding definition of the completion item.
	Definition *SpxDefinitionIdentifier `json:"definition,omitempty"`
}
