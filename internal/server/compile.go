package server

import (
	"bytes"
	"fmt"
	"go/types"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/goplus/gogen"
	gopast "github.com/goplus/gop/ast"
	gopscanner "github.com/goplus/gop/scanner"
	goptoken "github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"github.com/goplus/goxlsw/gop"
	"github.com/goplus/goxlsw/gop/goputil"
	"github.com/goplus/goxlsw/internal"
	"github.com/goplus/goxlsw/internal/analysis/ast/inspector"
	"github.com/goplus/goxlsw/internal/analysis/passes/inspect"
	"github.com/goplus/goxlsw/internal/analysis/protocol"
	"github.com/goplus/goxlsw/internal/pkgdata"
	"github.com/goplus/goxlsw/internal/util"
	"github.com/goplus/goxlsw/internal/vfs"
	"github.com/goplus/goxlsw/pkgdoc"
	"github.com/goplus/mod/gopmod"
	"github.com/goplus/mod/modload"
	"github.com/qiniu/x/errors"
)

// errNoMainSpxFile is the error returned when no valid main.spx file is found
// in the main package while compiling.
var errNoMainSpxFile = errors.New("no valid main.spx file found in main package")

func getPkgDoc(proj *gop.Project) *pkgdoc.PkgDoc {
	ret, _ := proj.PkgDoc()
	return ret
}

func getTypeInfo(proj *gop.Project) *typesutil.Info {
	_, ret, _, _ := proj.TypeInfo()
	return ret
}

func getPkg(proj *gop.Project) *types.Package {
	ret, _, _, _ := proj.TypeInfo()
	return ret
}

func getASTPkg(proj *gop.Project) *gopast.Package {
	ret, _ := proj.ASTPackage()
	return ret
}

// compileResult contains the compile results and additional information from
// the compile process.
type compileResult struct {
	proj *gop.Project

	// mainSpxFile is the main.spx file path.
	mainSpxFile string

	// spxResourceSet is the set of spx resources.
	spxResourceSet SpxResourceSet

	// spxResourceRefs stores spx resource references.
	spxResourceRefs []SpxResourceRef

	// seenSpxResourceRefs stores already seen spx resource references to avoid
	// duplicates.
	seenSpxResourceRefs map[SpxResourceRef]struct{}

	// spxSoundResourceAutoBindings stores spx sound resource auto-bindings.
	spxSoundResourceAutoBindings map[types.Object]struct{}

	// spxSpriteResourceAutoBindings stores spx sprite resource auto-bindings.
	spxSpriteResourceAutoBindings map[types.Object]struct{}

	// diagnostics stores diagnostic messages for each document.
	diagnostics map[DocumentURI][]Diagnostic

	// seenDiagnostics stores already reported diagnostics to avoid duplicates.
	seenDiagnostics map[DocumentURI]map[string]struct{}

	// hasErrorSeverityDiagnostic is true if the compile result has any
	// diagnostics with error severity.
	hasErrorSeverityDiagnostic bool

	// computedCache is the cache for computed results.
	computedCache compileResultComputedCache

	// documentURIs maps each spx file path to its document URI.
	documentURIs map[string]DocumentURI
}

// compileResultComputedCache represents the computed cache for [compileResult].
type compileResultComputedCache struct {
	// identsAtASTFileLines stores the identifiers at the given AST file line.
	identsAtASTFileLines sync.Map // map[astFileLine][]*gopast.Ident

	// spxDefinitionsForNamedStructs stores spx definitions for named structs.
	spxDefinitionsForNamedStructs sync.Map // map[*types.Named][]SpxDefinition

	// documentLinks stores document links for each document URI.
	documentLinks sync.Map // map[DocumentURI][]DocumentLink

	// semanticTokens stores semantic tokens for each document URI.
	semanticTokens sync.Map // map[DocumentURI][]uint32
}

// astFileLine represents an AST file line.
type astFileLine struct {
	astFile *gopast.File
	line    int
}

// newCompileResult creates a new [compileResult].
func newCompileResult(proj *gop.Project) *compileResult {
	return &compileResult{
		proj:                          proj,
		spxSoundResourceAutoBindings:  make(map[types.Object]struct{}),
		spxSpriteResourceAutoBindings: make(map[types.Object]struct{}),
		diagnostics:                   make(map[DocumentURI][]Diagnostic),
		documentURIs:                  make(map[string]DocumentURI),
	}
}

// isInFset reports whether the given position exists in the file set.
func (r *compileResult) isInFset(pos goptoken.Pos) bool {
	return r.proj.Fset.File(pos) != nil
}

// innermostScopeAt returns the innermost scope that contains the given
// position. It returns nil if not found.
func (r *compileResult) innermostScopeAt(pos goptoken.Pos) *types.Scope {
	typeInfo := getTypeInfo(r.proj)
	fileScope := typeInfo.Scopes[r.posASTFile(pos)]
	if fileScope == nil {
		return nil
	}
	innermostScope := fileScope
	for _, scope := range typeInfo.Scopes {
		if scope.Contains(pos) && fileScope.Contains(scope.Pos()) && innermostScope.Contains(scope.Pos()) {
			innermostScope = scope
		}
	}
	return innermostScope
}

// identsAtASTFileLine returns the identifiers at the given line in the given
// AST file.
func (r *compileResult) identsAtASTFileLine(astFile *gopast.File, line int) (idents []*gopast.Ident) {
	astFileLine := astFileLine{astFile: astFile, line: line}
	if identsAtLineIface, ok := r.computedCache.identsAtASTFileLines.Load(astFileLine); ok {
		return identsAtLineIface.([]*gopast.Ident)
	}
	defer func() {
		r.computedCache.identsAtASTFileLines.Store(astFileLine, slices.Clip(idents))
	}()

	fset := r.proj.Fset
	astFilePos := fset.Position(astFile.Pos())
	collectIdentAtLine := func(ident *gopast.Ident) {
		identPos := fset.Position(ident.Pos())
		if identPos.Filename == astFilePos.Filename && identPos.Line == line {
			idents = append(idents, ident)
		}
	}
	typeInfo := getTypeInfo(r.proj)
	for ident := range typeInfo.Defs {
		if goputil.IsShadow(r.proj, ident) {
			continue
		}
		collectIdentAtLine(ident)
	}
	for ident, obj := range typeInfo.Uses {
		if goputil.IsShadow(r.proj, r.defIdentFor(obj)) {
			continue
		}
		collectIdentAtLine(ident)
	}
	return
}

// identAtASTFilePosition returns the identifier at the given position in the
// given AST file.
func (r *compileResult) identAtASTFilePosition(astFile *gopast.File, position goptoken.Position) *gopast.Ident {
	var (
		bestIdent    *gopast.Ident
		bestNodeSpan int
	)
	fset := r.proj.Fset
	for _, ident := range r.identsAtASTFileLine(astFile, position.Line) {
		identPos := fset.Position(ident.Pos())
		identEnd := fset.Position(ident.End())
		if position.Column < identPos.Column || position.Column > identEnd.Column {
			continue
		}

		nodeSpan := identEnd.Column - identPos.Column
		if bestIdent == nil || nodeSpan < bestNodeSpan {
			bestIdent = ident
			bestNodeSpan = nodeSpan
		}
	}
	return bestIdent
}

// defIdentFor returns the identifier where the given object is defined.
func (r *compileResult) defIdentFor(obj types.Object) *gopast.Ident {
	if obj == nil {
		return nil
	}
	for ident, o := range getTypeInfo(r.proj).Defs {
		if o == obj {
			return ident
		}
	}
	return nil
}

// refIdentsFor returns all identifiers where the given object is referenced.
func (r *compileResult) refIdentsFor(obj types.Object) []*gopast.Ident {
	if obj == nil {
		return nil
	}
	var idents []*gopast.Ident
	for ident, o := range getTypeInfo(r.proj).Uses {
		if o == obj {
			idents = append(idents, ident)
		}
	}
	return idents
}

// selectorTypeNameForIdent returns the selector type name for the given
// identifier. It returns empty string if no selector can be inferred.
func (r *compileResult) selectorTypeNameForIdent(ident *gopast.Ident) string {
	astFile := r.nodeASTFile(ident)
	if astFile == nil {
		return ""
	}

	if path, _ := util.PathEnclosingInterval(astFile, ident.Pos(), ident.End()); len(path) > 0 {
		for _, node := range slices.Backward(path) {
			sel, ok := node.(*gopast.SelectorExpr)
			if !ok {
				continue
			}
			tv, ok := getTypeInfo(r.proj).Types[sel.X]
			if !ok {
				continue
			}

			switch typ := unwrapPointerType(tv.Type).(type) {
			case *types.Named:
				obj := typ.Obj()
				typeName := obj.Name()
				if isSpxPkgObject(obj) && typeName == "SpriteImpl" {
					typeName = "Sprite"
				}
				return typeName
			case *types.Interface:
				if typ.String() == "interface{}" {
					return ""
				}
				return typ.String()
			}
		}
	}

	typeInfo := getTypeInfo(r.proj)
	obj := typeInfo.ObjectOf(ident)
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	if isSpxPkgObject(obj) {
		astFileScope := typeInfo.Scopes[astFile]
		innermostScope := r.innermostScopeAt(ident.Pos())
		if innermostScope == astFileScope {
			spxFile := r.nodeFilename(ident)
			if spxFile == r.mainSpxFile {
				return "Game"
			}
			return "Sprite"
		}
	}
	switch obj := obj.(type) {
	case *types.Var:
		if !obj.IsField() {
			return ""
		}

		for _, def := range typeInfo.Defs {
			if def == nil {
				continue
			}
			named, ok := unwrapPointerType(def.Type()).(*types.Named)
			if !ok || named.Obj().Pkg() != obj.Pkg() || !isNamedStructType(named) {
				continue
			}

			var typeName string
			walkStruct(named, func(member types.Object, selector *types.Named) bool {
				if field, ok := member.(*types.Var); ok && field == obj {
					typeName = selector.Obj().Name()
					return false
				}
				return true
			})
			if isSpxPkgObject(obj) && typeName == "SpriteImpl" {
				typeName = "Sprite"
			}
			if typeName != "" {
				return typeName
			}
		}
	case *types.Func:
		recv := obj.Type().(*types.Signature).Recv()
		if recv == nil {
			return ""
		}

		switch typ := unwrapPointerType(recv.Type()).(type) {
		case *types.Named:
			obj := typ.Obj()
			typeName := obj.Name()
			if isSpxPkgObject(obj) && typeName == "SpriteImpl" {
				typeName = "Sprite"
			}
			return typeName
		case *types.Interface:
			if typ.String() == "interface{}" {
				return ""
			}
			return typ.String()
		}
	}
	return ""
}

// isDefinedInFirstVarBlock reports whether the given object is defined in the
// first var block of an AST file.
func (r *compileResult) isDefinedInFirstVarBlock(obj types.Object) bool {
	defIdent := r.defIdentFor(obj)
	if defIdent == nil {
		return false
	}
	astFile := r.nodeASTFile(defIdent)
	if astFile == nil {
		return false
	}
	firstVarBlock := goputil.ClassFieldsDecl(astFile)
	if firstVarBlock == nil {
		return false
	}
	return defIdent.Pos() >= firstVarBlock.Pos() && defIdent.End() <= firstVarBlock.End()
}

// spxDefinitionsFor returns all spx definitions for the given object. It
// returns multiple definitions only if the object is a Go+ overloadable
// function.
func (r *compileResult) spxDefinitionsFor(obj types.Object, selectorTypeName string) []SpxDefinition {
	if obj == nil {
		return nil
	}
	if isBuiltinObject(obj) {
		return []SpxDefinition{GetSpxDefinitionForBuiltinObj(obj)}
	}

	var pkgDoc *pkgdoc.PkgDoc
	if pkgPath := obj.Pkg().Path(); pkgPath == "main" {
		pkgDoc = getPkgDoc(r.proj)
	} else {
		pkgDoc, _ = pkgdata.GetPkgDoc(pkgPath)
	}

	switch obj := obj.(type) {
	case *types.Var:
		return []SpxDefinition{GetSpxDefinitionForVar(obj, selectorTypeName, r.isDefinedInFirstVarBlock(obj), pkgDoc)}
	case *types.Const:
		return []SpxDefinition{GetSpxDefinitionForConst(obj, pkgDoc)}
	case *types.TypeName:
		return []SpxDefinition{GetSpxDefinitionForType(obj, pkgDoc)}
	case *types.Func:
		if goputil.IsShadow(r.proj, r.defIdentFor(obj)) {
			return nil
		}
		if isUnexpandableGopOverloadableFunc(obj) {
			return nil
		}
		if funcOverloads := expandGopOverloadableFunc(obj); funcOverloads != nil {
			defs := make([]SpxDefinition, 0, len(funcOverloads))
			for _, funcOverload := range funcOverloads {
				defs = append(defs, GetSpxDefinitionForFunc(funcOverload, selectorTypeName, pkgDoc))
			}
			return defs
		}
		return []SpxDefinition{GetSpxDefinitionForFunc(obj, selectorTypeName, pkgDoc)}
	case *types.PkgName:
		return []SpxDefinition{GetSpxDefinitionForPkg(obj, pkgDoc)}
	}
	return nil
}

// spxDefinitionsForIdent returns all spx definitions for the given identifier.
// It returns multiple definitions only if the identifier is a Go+ overloadable
// function.
func (r *compileResult) spxDefinitionsForIdent(ident *gopast.Ident) []SpxDefinition {
	typeInfo := getTypeInfo(r.proj)
	return r.spxDefinitionsFor(typeInfo.ObjectOf(ident), r.selectorTypeNameForIdent(ident))
}

// spxDefinitionsForNamedStruct returns all spx definitions for the given named
// struct type.
func (r *compileResult) spxDefinitionsForNamedStruct(named *types.Named) (defs []SpxDefinition) {
	if defsIface, ok := r.computedCache.spxDefinitionsForNamedStructs.Load(named); ok {
		return defsIface.([]SpxDefinition)
	}
	defer func() {
		r.computedCache.spxDefinitionsForNamedStructs.Store(named, slices.Clip(defs))
	}()

	walkStruct(named, func(member types.Object, selector *types.Named) bool {
		defs = append(defs, r.spxDefinitionsFor(member, selector.Obj().Name())...)
		return true
	})
	return
}

// spxDefinitionForField returns the spx definition for the given field and
// optional selector type name.
func (r *compileResult) spxDefinitionForField(field *types.Var, selectorTypeName string) SpxDefinition {
	var (
		forceVar bool
		pkgDoc   *pkgdoc.PkgDoc
	)
	if defIdent := r.defIdentFor(field); defIdent != nil {
		if selectorTypeName == "" {
			selectorTypeName = r.selectorTypeNameForIdent(defIdent)
		}
		forceVar = r.isDefinedInFirstVarBlock(field)
		pkgDoc = getPkgDoc(r.proj)
	} else {
		pkg := field.Pkg()
		pkgDoc, _ = pkgdata.GetPkgDoc(pkg.Path())
	}
	return GetSpxDefinitionForVar(field, selectorTypeName, forceVar, pkgDoc)
}

// spxDefinitionForMethod returns the spx definition for the given method and
// optional selector type name.
func (r *compileResult) spxDefinitionForMethod(method *types.Func, selectorTypeName string) SpxDefinition {
	var pkgDoc *pkgdoc.PkgDoc
	if defIdent := r.defIdentFor(method); defIdent != nil {
		if selectorTypeName == "" {
			selectorTypeName = r.selectorTypeNameForIdent(defIdent)
		}
		pkgDoc = getPkgDoc(r.proj)
	} else {
		if idx := strings.LastIndex(selectorTypeName, "."); idx >= 0 {
			selectorTypeName = selectorTypeName[idx+1:]
		}
		pkg := method.Pkg()
		pkgDoc, _ = pkgdata.GetPkgDoc(pkg.Path())
	}
	return GetSpxDefinitionForFunc(method, selectorTypeName, pkgDoc)
}

// isInSpxEventHandler checks if the given position is inside an spx event
// handler callback.
func (r *compileResult) isInSpxEventHandler(pos goptoken.Pos) bool {
	astFile := r.posASTFile(pos)
	if astFile == nil {
		return false
	}

	typeInfo := getTypeInfo(r.proj)
	path, _ := util.PathEnclosingInterval(astFile, pos-1, pos)
	for _, node := range path {
		callExpr, ok := node.(*gopast.CallExpr)
		if !ok || len(callExpr.Args) == 0 {
			continue
		}
		funcIdent, ok := callExpr.Fun.(*gopast.Ident)
		if !ok {
			continue
		}
		funcObj := typeInfo.ObjectOf(funcIdent)
		if !isSpxPkgObject(funcObj) {
			continue
		}

		if isSpxEventHandlerFuncName(funcIdent.Name) {
			return true
		}
	}
	return false
}

// spxResourceRefAtASTFilePosition returns the spx resource reference at the
// given position in the given AST file.
func (r *compileResult) spxResourceRefAtASTFilePosition(astFile *gopast.File, position goptoken.Position) *SpxResourceRef {
	var (
		bestRef      *SpxResourceRef
		bestNodeSpan int
	)
	fset := r.proj.Fset
	for _, ref := range r.spxResourceRefs {
		nodePos := fset.Position(ref.Node.Pos())
		nodeEnd := fset.Position(ref.Node.End())
		if nodePos.Filename != position.Filename ||
			position.Line != nodePos.Line ||
			position.Column < nodePos.Column ||
			position.Column > nodeEnd.Column {
			continue
		}

		nodeSpan := nodeEnd.Column - nodePos.Column
		if bestRef == nil || nodeSpan < bestNodeSpan {
			bestRef = &ref
			bestNodeSpan = nodeSpan
		}
	}
	return bestRef
}

// spxImportsAtASTFilePosition returns the import at the given position in the given AST file.
func (r *compileResult) spxImportsAtASTFilePosition(astFile *gopast.File, position goptoken.Position) *SpxReferencePkg {
	fset := r.proj.Fset
	for _, imp := range astFile.Imports {
		nodePos := fset.Position(imp.Pos())
		nodeEnd := fset.Position(imp.End())
		if nodePos.Filename != position.Filename ||
			position.Line != nodePos.Line ||
			position.Column < nodePos.Column ||
			position.Column > nodeEnd.Column {
			continue
		}

		pkg, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		pkgDoc, err := pkgdata.GetPkgDoc(pkg)
		if err != nil {
			continue
		}
		return &SpxReferencePkg{
			Pkg:     pkgDoc,
			PkgPath: pkg,
			Node:    imp,
		}
	}
	return nil
}

// addSpxResourceRef adds an spx resource reference to the compile result.
func (r *compileResult) addSpxResourceRef(ref SpxResourceRef) {
	if r.seenSpxResourceRefs == nil {
		r.seenSpxResourceRefs = make(map[SpxResourceRef]struct{})
	}

	if _, ok := r.seenSpxResourceRefs[ref]; ok {
		return
	}
	r.seenSpxResourceRefs[ref] = struct{}{}

	r.spxResourceRefs = append(r.spxResourceRefs, ref)
}

// addDiagnostics adds diagnostics to the compile result.
func (r *compileResult) addDiagnostics(documentURI DocumentURI, diags ...Diagnostic) {
	if r.seenDiagnostics == nil {
		r.seenDiagnostics = make(map[DocumentURI]map[string]struct{})
	}
	seenDiagnostics := r.seenDiagnostics[documentURI]
	if seenDiagnostics == nil {
		seenDiagnostics = make(map[string]struct{})
		r.seenDiagnostics[documentURI] = seenDiagnostics
	}

	r.diagnostics[documentURI] = slices.Grow(r.diagnostics[documentURI], len(diags))
	for _, diag := range diags {
		fingerprint := fmt.Sprintf("%d\n%v\n%s", diag.Severity, diag.Range, diag.Message)
		if _, ok := seenDiagnostics[fingerprint]; ok {
			continue
		}
		seenDiagnostics[fingerprint] = struct{}{}

		r.diagnostics[documentURI] = append(r.diagnostics[documentURI], diag)
		if diag.Severity == SeverityError {
			r.hasErrorSeverityDiagnostic = true
		}
	}
}

// addDiagnosticsForSpxFile adds diagnostics to the compile result for the given
// spx file.
func (r *compileResult) addDiagnosticsForSpxFile(spxFile string, diags ...Diagnostic) {
	r.addDiagnostics(r.documentURIs[spxFile], diags...)
}

// posFilename returns the filename for the given position.
func (r *compileResult) posFilename(pos goptoken.Pos) string {
	return r.proj.Fset.Position(pos).Filename
}

// nodeFilename returns the filename for the given node.
func (r *compileResult) nodeFilename(node gopast.Node) string {
	return r.posFilename(node.Pos())
}

// posASTFile returns the AST file for the given position.
func (r *compileResult) posASTFile(pos goptoken.Pos) *gopast.File {
	return getASTPkg(r.proj).Files[r.posFilename(pos)]
}

// nodeASTFile returns the AST file for the given node.
func (r *compileResult) nodeASTFile(node gopast.Node) *gopast.File {
	return r.posASTFile(node.Pos())
}

// posDocumentURI returns the [DocumentURI] for the given position.
func (r *compileResult) posDocumentURI(pos goptoken.Pos) DocumentURI {
	return r.documentURIs[r.posFilename(pos)]
}

// nodeDocumentURI returns the [DocumentURI] for the given node.
func (r *compileResult) nodeDocumentURI(node gopast.Node) DocumentURI {
	return r.posDocumentURI(node.Pos())
}

// fromPosition converts a [goptoken.Position] to a protocol [Position].
func (r *compileResult) fromPosition(astFile *gopast.File, position goptoken.Position) Position {
	tokenFile := r.proj.Fset.File(astFile.Pos())

	line := position.Line
	lineStart := int(tokenFile.LineStart(line))
	relLineStart := lineStart - tokenFile.Base()
	lineContent := astFile.Code[relLineStart : relLineStart+position.Column-1]
	utf16Offset := utf8OffsetToUTF16(string(lineContent), position.Column-1)

	return Position{
		Line:      uint32(position.Line - 1),
		Character: uint32(utf16Offset),
	}
}

// toPosition converts a protocol [Position] to a [goptoken.Position].
func (r *compileResult) toPosition(astFile *gopast.File, position Position) goptoken.Position {
	tokenFile := r.proj.Fset.File(astFile.Pos())

	line := min(int(position.Line)+1, tokenFile.LineCount())
	lineStart := int(tokenFile.LineStart(line))
	relLineStart := lineStart - tokenFile.Base()
	lineContent := astFile.Code[relLineStart:]
	if i := bytes.IndexByte(lineContent, '\n'); i >= 0 {
		lineContent = lineContent[:i]
	}
	utf8Offset := utf16OffsetToUTF8(string(lineContent), int(position.Character))
	column := utf8Offset + 1

	return goptoken.Position{
		Filename: tokenFile.Name(),
		Offset:   relLineStart + utf8Offset,
		Line:     line,
		Column:   column,
	}
}

// posAt returns the [goptoken.Pos] of the given position in the given AST file.
func (r *compileResult) posAt(astFile *gopast.File, position Position) goptoken.Pos {
	tokenFile := r.proj.Fset.File(astFile.Pos())
	if int(position.Line) > tokenFile.LineCount()-1 {
		return goptoken.Pos(tokenFile.Base() + tokenFile.Size()) // EOF
	}
	return tokenFile.Pos(r.toPosition(astFile, position).Offset)
}

// rangeForASTFilePosition returns a [Range] for the given [goptoken.Position]
// in the given AST file.
func (r *compileResult) rangeForASTFilePosition(astFile *gopast.File, position goptoken.Position) Range {
	p := r.fromPosition(astFile, position)
	return Range{Start: p, End: p}
}

// rangeForPos returns the [Range] for the given position.
func (r *compileResult) rangeForPos(pos goptoken.Pos) Range {
	return r.rangeForASTFilePosition(r.posASTFile(pos), r.proj.Fset.Position(pos))
}

// rangeForASTFileNode returns the [Range] for the given node in the given AST file.
func (r *compileResult) rangeForASTFileNode(astFile *gopast.File, node gopast.Node) Range {
	fset := r.proj.Fset
	return Range{
		Start: r.fromPosition(astFile, fset.Position(node.Pos())),
		End:   r.fromPosition(astFile, fset.Position(node.End())),
	}
}

// rangeForStartEnd returns the [Range] for the given start and end positions.
func (r *compileResult) rangeForStartEnd(astFile *gopast.File, start, end goptoken.Pos) Range {
	fset := r.proj.Fset
	return Range{
		Start: r.fromPosition(astFile, fset.Position(start)),
		End:   r.fromPosition(astFile, fset.Position(end)),
	}
}

// rangeForNode returns the [Range] for the given node.
func (r *compileResult) rangeForNode(node gopast.Node) Range {
	return r.rangeForASTFileNode(r.nodeASTFile(node), node)
}

// locationForPos returns the [Location] for the given position.
func (r *compileResult) locationForPos(pos goptoken.Pos) Location {
	return Location{
		URI:   r.documentURIs[r.posFilename(pos)],
		Range: r.rangeForPos(pos),
	}
}

// locationForNode returns the [Location] for the given node.
func (r *compileResult) locationForNode(node gopast.Node) Location {
	return Location{
		URI:   r.documentURIs[r.nodeFilename(node)],
		Range: r.rangeForNode(node),
	}
}

// compile compiles spx source files and returns compile result. It uses cached
// result if available.
func (s *Server) compile() (*compileResult, error) {
	// NOTE(xsw): don't create a snapshot
	snapshot := s.workspaceRootFS // .Snapshot()

	// TODO(wyvern): remove this once we have a better way to update files.
	snapshot.UpdateFiles(s.fileMapGetter())
	return s.compileAt(snapshot)
}

// compileAt compiles spx source files at the given snapshot and returns the
// compile result.
func (s *Server) compileAt(snapshot *vfs.MapFS) (*compileResult, error) {
	spxFiles, err := vfs.ListSpxFiles(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to get spx files: %w", err)
	}
	if len(spxFiles) == 0 {
		return nil, errNoMainSpxFile
	}

	result := newCompileResult(snapshot)
	for _, spxFile := range spxFiles {
		documentURI := s.toDocumentURI(spxFile)
		result.diagnostics[documentURI] = []Diagnostic{}
		result.documentURIs[spxFile] = documentURI

		astFile, err := snapshot.AST(spxFile)
		if err != nil {
			var (
				errorList gopscanner.ErrorList
				codeError *gogen.CodeError
			)
			if errors.As(err, &errorList) {
				// Handle parse errors.
				for _, e := range errorList {
					result.addDiagnostics(documentURI, Diagnostic{
						Severity: SeverityError,
						Range:    result.rangeForASTFilePosition(astFile, e.Pos),
						Message:  e.Msg,
					})
				}
			} else if errors.As(err, &codeError) {
				// Handle code generation errors.
				result.addDiagnostics(documentURI, Diagnostic{
					Severity: SeverityError,
					Range:    result.rangeForPos(codeError.Pos),
					Message:  codeError.Error(),
				})
			} else {
				// Handle unknown errors (including recovered panics).
				result.addDiagnostics(documentURI, Diagnostic{
					Severity: SeverityError,
					Message:  fmt.Sprintf("failed to parse spx file: %v", err),
				})
			}
		}
		if astFile == nil {
			continue
		}
		if astFile.Name.Name != "main" {
			result.addDiagnostics(documentURI, Diagnostic{
				Severity: SeverityError,
				Range:    result.rangeForASTFileNode(astFile, astFile.Name),
				Message:  "package name must be main",
			})
			continue
		}

		if spxFileBaseName := path.Base(spxFile); spxFileBaseName == "main.spx" {
			result.mainSpxFile = spxFile
		}
	}
	if result.mainSpxFile == "" {
		if len(result.diagnostics) == 0 {
			return nil, errNoMainSpxFile
		}
		return result, nil
	}

	mod := gopmod.New(modload.Default)
	if err := mod.ImportClasses(); err != nil {
		return nil, fmt.Errorf("failed to import classes: %w", err)
	}
	handleErr := func(err error) {
		if typeErr, ok := err.(types.Error); ok {
			position := typeErr.Fset.Position(typeErr.Pos)
			result.addDiagnosticsForSpxFile(position.Filename, Diagnostic{
				Severity: SeverityError,
				Range:    result.rangeForPos(typeErr.Pos),
				Message:  typeErr.Msg,
			})
		}
	}

	snapshot.Path = "main"
	snapshot.Mod = mod
	snapshot.Importer = internal.Importer
	_, _, err, _ = snapshot.TypeInfo()
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

	s.inspectForSpxResourceSet(snapshot, result)
	s.inspectForSpxResourceRefs(result)
	s.inspectDiagnosticsAnalyzers(result)

	return result, nil
}

// compileAndGetASTFileForDocumentURI handles common compilation and file
// retrieval logic for a given document URI. The returned astFile is probably
// nil even if the compilation succeeded.
func (s *Server) compileAndGetASTFileForDocumentURI(uri DocumentURI) (result *compileResult, spxFile string, astFile *gopast.File, err error) {
	spxFile, err = s.fromDocumentURI(uri)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to get file path from document URI %q: %w", uri, err)
	}
	if path.Ext(spxFile) != ".spx" {
		return nil, "", nil, fmt.Errorf("file %q does not have .spx extension", spxFile)
	}
	result, err = s.compile()
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to compile: %w", err)
	}
	astFile = getASTPkg(result.proj).Files[spxFile]
	return
}

// inspectForSpxResourceSet inspects for spx resource set in main.spx.
func (s *Server) inspectForSpxResourceSet(snapshot *vfs.MapFS, result *compileResult) {
	var typeInfo = getTypeInfo(snapshot)
	var spxResourceRootDir string
	gopast.Inspect(getASTPkg(result.proj).Files[result.mainSpxFile], func(node gopast.Node) bool {
		callExpr, ok := node.(*gopast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := callExpr.Fun.(*gopast.Ident)
		if !ok || ident.Name != "run" {
			return true
		}

		if len(callExpr.Args) == 0 {
			return true
		}
		firstArg := callExpr.Args[0]
		firstArgTV, ok := typeInfo.Types[firstArg]
		if !ok {
			return true
		}

		if types.AssignableTo(firstArgTV.Type, types.Typ[types.String]) {
			spxResourceRootDir, _ = getStringLitOrConstValue(firstArg, firstArgTV)
		} else {
			result.addDiagnosticsForSpxFile(result.mainSpxFile, Diagnostic{
				Severity: SeverityError,
				Range:    result.rangeForNode(firstArg),
				Message:  "first argument of run must be a string literal or constant",
			})
		}
		return false
	})
	if spxResourceRootDir == "" {
		spxResourceRootDir = "assets"
	}
	spxResourceRootFS := vfs.Sub(snapshot, spxResourceRootDir)

	spxResourceSet, err := NewSpxResourceSet(spxResourceRootFS)
	if err != nil {
		result.addDiagnosticsForSpxFile(result.mainSpxFile, Diagnostic{
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to create spx resource set: %v", err),
		})
		return
	}
	result.spxResourceSet = *spxResourceSet
}

// inspectDiagnosticsAnalyzers runs registered analyzers on each spx source file
// and collects diagnostics.
//
// For each spx file in the main package, it:
// 1. Creates an analysis pass with file-specific information
// 2. Runs all registered analyzers on the file
// 3. Collects diagnostics from analyzers
// 4. Reports any analyzer errors as diagnostics
//
// Parameters:
//   - result: The compilation result containing AST and type information
//
// The function updates result.diagnostics with any issues found by analyzers.
// Diagnostic severity levels include:
//   - Error: For analyzer failures or serious code issues
//   - Warning: For potential problems that don't prevent compilation
func (s *Server) inspectDiagnosticsAnalyzers(result *compileResult) {
	proj := result.proj
	fset := proj.Fset
	typeInfo := getTypeInfo(proj)
	for spxFile, astFile := range getASTPkg(proj).Files {

		var diagnostics []Diagnostic
		pass := &protocol.Pass{
			Fset:      fset,
			Files:     []*gopast.File{astFile},
			TypesInfo: typeInfo,
			Report: func(d protocol.Diagnostic) {
				diagnostics = append(diagnostics, Diagnostic{
					Range:    result.rangeForStartEnd(astFile, d.Pos, d.End),
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
				diagnostics = append(diagnostics, Diagnostic{
					Severity: SeverityError,
					Message:  fmt.Sprintf("analyzer %q failed: %v", an.Name, err),
				})
			}
		}

		result.addDiagnosticsForSpxFile(spxFile, diagnostics...)
	}
}

// inspectForSpxResourceRefs inspects for spx resource references in the code.
func (s *Server) inspectForSpxResourceRefs(result *compileResult) {
	proj := result.proj
	typeInfo := getTypeInfo(proj)
	mainSpxFileScope := typeInfo.Scopes[getASTPkg(proj).Files[result.mainSpxFile]]

	// Check all identifier definitions.
	for ident, obj := range typeInfo.Defs {
		if ident == nil || !ident.Pos().IsValid() || obj == nil {
			continue
		}

		switch obj.(type) {
		case *types.Const, *types.Var:
			if ident.Obj == nil {
				break
			}
			valueSpec, ok := ident.Obj.Decl.(*gopast.ValueSpec)
			if !ok {
				break
			}
			idx := slices.Index(valueSpec.Names, ident)
			if idx < 0 || idx >= len(valueSpec.Values) {
				break
			}
			expr := valueSpec.Values[idx]

			s.inspectSpxResourceRefForTypeAtExpr(result, expr, unwrapPointerType(obj.Type()), nil)
		}

		v, ok := obj.(*types.Var)
		if !ok {
			continue
		}
		varType, ok := v.Type().(*types.Named)
		if !ok {
			continue
		}

		spxFile := result.nodeFilename(ident)
		if spxFile != result.mainSpxFile || result.innermostScopeAt(ident.Pos()) != mainSpxFileScope {
			continue
		}

		var (
			isSpxSoundResourceAutoBinding  bool
			isSpxSpriteResourceAutoBinding bool
		)
		switch varType {
		case GetSpxSoundType():
			isSpxSoundResourceAutoBinding = result.spxResourceSet.Sound(v.Name()) != nil
		case GetSpxSpriteType():
			isSpxSpriteResourceAutoBinding = result.spxResourceSet.Sprite(v.Name()) != nil
		default:
			isSpxSpriteResourceAutoBinding = v.Name() == varType.Obj().Name() && vfs.HasSpriteType(result.proj, varType)
		}
		if !isSpxSoundResourceAutoBinding && !isSpxSpriteResourceAutoBinding {
			continue
		}

		if !result.isDefinedInFirstVarBlock(obj) {
			result.addDiagnosticsForSpxFile(spxFile, Diagnostic{
				Severity: SeverityWarning,
				Range:    result.rangeForNode(ident),
				Message:  "resources must be defined in the first var block for auto-binding",
			})
			continue
		}

		switch {
		case isSpxSoundResourceAutoBinding:
			result.spxSoundResourceAutoBindings[obj] = struct{}{}
		case isSpxSpriteResourceAutoBinding:
			result.spxSpriteResourceAutoBindings[obj] = struct{}{}
		}
		s.inspectSpxResourceRefForTypeAtExpr(result, ident, unwrapPointerType(obj.Type()), nil)
	}

	// Check all identifier uses.
	for ident, obj := range typeInfo.Uses {
		if ident == nil || !ident.Pos().IsValid() || obj == nil {
			continue
		}
		s.inspectSpxResourceRefForTypeAtExpr(result, ident, unwrapPointerType(obj.Type()), nil)
	}

	// Check all type-checked expressions.
	for expr, tv := range typeInfo.Types {
		if expr == nil || !expr.Pos().IsValid() || tv.IsType() || tv.Type == nil {
			continue // Skip type identifiers.
		}

		switch expr := expr.(type) {
		case *gopast.CallExpr:
			funcTV, ok := typeInfo.Types[expr.Fun]
			if !ok {
				continue
			}
			funcSig, ok := funcTV.Type.(*types.Signature)
			if !ok {
				continue
			}

			var spxSpriteResource *SpxSpriteResource
			if recv := funcSig.Recv(); recv != nil {
				recvType := unwrapPointerType(recv.Type())
				switch recvType {
				case GetSpxSpriteType(), GetSpxSpriteImplType():
					spxSpriteResource = s.inspectSpxSpriteResourceRefAtExpr(result, expr, recvType)
				}
			}

			var lastParamType types.Type
			for i, arg := range expr.Args {
				var paramType types.Type
				if i < funcSig.Params().Len() {
					paramType = unwrapPointerType(funcSig.Params().At(i).Type())
					lastParamType = paramType
				} else {
					// Use the last parameter type for variadic functions.
					paramType = lastParamType
				}

				// Handle slice/array parameter types.
				if sliceType, ok := paramType.(*types.Slice); ok {
					paramType = unwrapPointerType(sliceType.Elem())
				} else if arrayType, ok := paramType.(*types.Array); ok {
					paramType = unwrapPointerType(arrayType.Elem())
				}

				if sliceLit, ok := arg.(*gopast.SliceLit); ok {
					for _, elt := range sliceLit.Elts {
						s.inspectSpxResourceRefForTypeAtExpr(result, elt, paramType, spxSpriteResource)
					}
				} else {
					s.inspectSpxResourceRefForTypeAtExpr(result, arg, paramType, spxSpriteResource)
				}
			}
		default:
			s.inspectSpxResourceRefForTypeAtExpr(result, expr, unwrapPointerType(tv.Type), nil)
		}
	}
}

// inspectSpxResourceRefForTypeAtExpr inspects an spx resource reference for a
// given type at an expression.
func (s *Server) inspectSpxResourceRefForTypeAtExpr(result *compileResult, expr gopast.Expr, typ types.Type, spxSpriteResource *SpxSpriteResource) {
	if ident, ok := expr.(*gopast.Ident); ok {
		switch typ {
		case GetSpxBackdropNameType(),
			GetSpxSpriteNameType(),
			GetSpxSoundNameType(),
			GetSpxWidgetNameType():
			astFile := result.nodeASTFile(ident)
			if astFile == nil {
				return
			}

			path, _ := util.PathEnclosingInterval(astFile, ident.Pos(), ident.End())
			for _, node := range path {
				assignStmt, ok := node.(*gopast.AssignStmt)
				if !ok {
					continue
				}

				idx := slices.IndexFunc(assignStmt.Lhs, func(lhs gopast.Expr) bool {
					return lhs == ident
				})
				if idx < 0 || idx >= len(assignStmt.Rhs) {
					continue
				}
				expr = assignStmt.Rhs[idx]
				break
			}
		}
	}

	switch typ {
	case GetSpxBackdropNameType():
		s.inspectSpxBackdropResourceRefAtExpr(result, expr, typ)
	case GetSpxSpriteNameType(), GetSpxSpriteType():
		s.inspectSpxSpriteResourceRefAtExpr(result, expr, typ)
	case GetSpxSpriteCostumeNameType():
		if spxSpriteResource != nil {
			s.inspectSpxSpriteCostumeResourceRefAtExpr(result, spxSpriteResource, expr, typ)
		}
	case GetSpxSpriteAnimationNameType():
		if spxSpriteResource != nil {
			s.inspectSpxSpriteAnimationResourceRefAtExpr(result, spxSpriteResource, expr, typ)
		}
	case GetSpxSoundNameType(), GetSpxSoundType():
		s.inspectSpxSoundResourceRefAtExpr(result, expr, typ)
	case GetSpxWidgetNameType():
		s.inspectSpxWidgetResourceRefAtExpr(result, expr, typ)
	default:
		if vfs.HasSpriteType(result.proj, typ) {
			s.inspectSpxSpriteResourceRefAtExpr(result, expr, typ)
		}
	}
}

// inspectSpxBackdropResourceRefAtExpr inspects an spx backdrop resource
// reference at an expression. It returns the spx backdrop resource if it was
// successfully retrieved.
func (s *Server) inspectSpxBackdropResourceRefAtExpr(result *compileResult, expr gopast.Expr, declaredType types.Type) *SpxBackdropResource {
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := getTypeInfo(result.proj).Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxBackdropNameType() {
		return nil
	}

	spxBackdropName, ok := getStringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	if spxBackdropName == "" {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  "backdrop resource name cannot be empty",
		})
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*gopast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	result.addSpxResourceRef(SpxResourceRef{
		ID:   SpxBackdropResourceID{BackdropName: spxBackdropName},
		Kind: spxResourceRefKind,
		Node: expr,
	})

	spxBackdropResource := result.spxResourceSet.Backdrop(spxBackdropName)
	if spxBackdropResource == nil {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  fmt.Sprintf("backdrop resource %q not found", spxBackdropName),
		})
		return nil
	}
	return spxBackdropResource
}

// inspectSpxSpriteResourceRefAtExpr inspects an spx sprite resource reference
// at an expression. It returns the spx sprite resource if it was successfully
// retrieved.
func (s *Server) inspectSpxSpriteResourceRefAtExpr(result *compileResult, expr gopast.Expr, declaredType types.Type) *SpxSpriteResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var spxSpriteName string
	if callExpr, ok := expr.(*gopast.CallExpr); ok {
		switch fun := callExpr.Fun.(type) {
		case *gopast.Ident:
			spxSpriteName = strings.TrimSuffix(path.Base(result.nodeFilename(callExpr)), ".spx")
		case *gopast.SelectorExpr:
			ident, ok := fun.X.(*gopast.Ident)
			if !ok {
				return nil
			}
			return s.inspectSpxSpriteResourceRefAtExpr(result, ident, declaredType)
		default:
			return nil
		}
	}
	if spxSpriteName == "" {
		var spxResourceRefKind SpxResourceRefKind
		if typ == GetSpxSpriteNameType() {
			var ok bool
			spxSpriteName, ok = getStringLitOrConstValue(expr, exprTV)
			if !ok {
				return nil
			}
			spxResourceRefKind = SpxResourceRefKindStringLiteral
			if _, ok := expr.(*gopast.Ident); ok {
				spxResourceRefKind = SpxResourceRefKindConstantReference
			}
		} else {
			ident, ok := expr.(*gopast.Ident)
			if !ok {
				return nil
			}
			obj := typeInfo.ObjectOf(ident)
			if obj == nil {
				return nil
			}
			if _, ok := result.spxSpriteResourceAutoBindings[obj]; !ok {
				return nil
			}
			spxSpriteName = obj.Name()
			defIdent := result.defIdentFor(obj)
			if defIdent == ident {
				spxResourceRefKind = SpxResourceRefKindAutoBinding
			} else {
				spxResourceRefKind = SpxResourceRefKindAutoBindingReference
			}
		}
		if spxSpriteName == "" {
			result.addDiagnostics(exprDocumentURI, Diagnostic{
				Severity: SeverityError,
				Range:    exprRange,
				Message:  "sprite resource name cannot be empty",
			})
			return nil
		}
		result.addSpxResourceRef(SpxResourceRef{
			ID:   SpxSpriteResourceID{SpriteName: spxSpriteName},
			Kind: spxResourceRefKind,
			Node: expr,
		})
	}

	spxSpriteResource := result.spxResourceSet.Sprite(spxSpriteName)
	if spxSpriteResource == nil {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  fmt.Sprintf("sprite resource %q not found", spxSpriteName),
		})
		return nil
	}
	return spxSpriteResource
}

// inspectSpxSpriteCostumeResourceRefAtExpr inspects an spx sprite costume
// resource reference at an expression. It returns the spx sprite costume
// resource if it was successfully retrieved.
func (s *Server) inspectSpxSpriteCostumeResourceRefAtExpr(result *compileResult, spxSpriteResource *SpxSpriteResource, expr gopast.Expr, declaredType types.Type) *SpxSpriteCostumeResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxSpriteCostumeNameType() {
		return nil
	}

	spxSpriteCostumeName, ok := getStringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	if spxSpriteCostumeName == "" {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  "sprite costume resource name cannot be empty",
		})
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*gopast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	result.addSpxResourceRef(SpxResourceRef{
		ID:   SpxSpriteCostumeResourceID{SpriteName: spxSpriteResource.Name, CostumeName: spxSpriteCostumeName},
		Kind: spxResourceRefKind,
		Node: expr,
	})

	spxSpriteCostumeResource := spxSpriteResource.Costume(spxSpriteCostumeName)
	if spxSpriteCostumeResource == nil {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  fmt.Sprintf("costume resource %q not found in sprite %q", spxSpriteCostumeName, spxSpriteResource.Name),
		})
		return nil
	}
	return spxSpriteCostumeResource
}

// inspectSpxSpriteAnimationResourceRefAtExpr inspects an spx sprite animation
// resource reference at an expression. It returns the spx sprite animation
// resource if it was successfully retrieved.
func (s *Server) inspectSpxSpriteAnimationResourceRefAtExpr(result *compileResult, spxSpriteResource *SpxSpriteResource, expr gopast.Expr, declaredType types.Type) *SpxSpriteAnimationResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxSpriteAnimationNameType() {
		return nil
	}

	spxSpriteAnimationName, ok := getStringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*gopast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	if spxSpriteAnimationName == "" {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  "sprite animation resource name cannot be empty",
		})
		return nil
	}
	result.addSpxResourceRef(SpxResourceRef{
		ID:   SpxSpriteAnimationResourceID{SpriteName: spxSpriteResource.Name, AnimationName: spxSpriteAnimationName},
		Kind: spxResourceRefKind,
		Node: expr,
	})

	spxSpriteAnimationResource := spxSpriteResource.Animation(spxSpriteAnimationName)
	if spxSpriteAnimationResource == nil {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  fmt.Sprintf("animation resource %q not found in sprite %q", spxSpriteAnimationName, spxSpriteResource.Name),
		})
		return nil
	}
	return spxSpriteAnimationResource
}

// inspectSpxSoundResourceRefAtExpr inspects an spx sound resource reference at
// an expression. It returns the spx sound resource if it was successfully
// retrieved.
func (s *Server) inspectSpxSoundResourceRefAtExpr(result *compileResult, expr gopast.Expr, declaredType types.Type) *SpxSoundResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var (
		spxSoundName       string
		spxResourceRefKind SpxResourceRefKind
	)
	switch typ {
	case GetSpxSoundNameType():
		var ok bool
		spxSoundName, ok = getStringLitOrConstValue(expr, exprTV)
		if !ok {
			return nil
		}
		spxResourceRefKind = SpxResourceRefKindStringLiteral
		if _, ok := expr.(*gopast.Ident); ok {
			spxResourceRefKind = SpxResourceRefKindConstantReference
		}
	case GetSpxSoundType():
		ident, ok := expr.(*gopast.Ident)
		if !ok {
			return nil
		}
		obj := typeInfo.ObjectOf(ident)
		if obj == nil {
			return nil
		}
		if _, ok := result.spxSoundResourceAutoBindings[obj]; !ok {
			return nil
		}
		spxSoundName = obj.Name()
		defIdent := result.defIdentFor(obj)
		if defIdent == ident {
			spxResourceRefKind = SpxResourceRefKindAutoBinding
		} else {
			spxResourceRefKind = SpxResourceRefKindAutoBindingReference
		}
	default:
		return nil
	}
	if spxSoundName == "" {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  "sound resource name cannot be empty",
		})
		return nil
	}
	result.addSpxResourceRef(SpxResourceRef{
		ID:   SpxSoundResourceID{SoundName: spxSoundName},
		Kind: spxResourceRefKind,
		Node: expr,
	})

	spxSoundResource := result.spxResourceSet.Sound(spxSoundName)
	if spxSoundResource == nil {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  fmt.Sprintf("sound resource %q not found", spxSoundName),
		})
		return nil
	}
	return spxSoundResource
}

// inspectSpxWidgetResourceRefAtExpr inspects an spx widget resource reference
// at an expression. It returns the spx widget resource if it was successfully
// retrieved.
func (s *Server) inspectSpxWidgetResourceRefAtExpr(result *compileResult, expr gopast.Expr, declaredType types.Type) *SpxWidgetResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxWidgetNameType() {
		return nil
	}

	spxWidgetName, ok := getStringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*gopast.Ident); ok {
		spxResourceRefKind = SpxResourceRefKindConstantReference
	}
	if spxWidgetName == "" {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  "widget resource name cannot be empty",
		})
		return nil
	}
	result.addSpxResourceRef(SpxResourceRef{
		ID:   SpxWidgetResourceID{WidgetName: spxWidgetName},
		Kind: spxResourceRefKind,
		Node: expr,
	})

	spxWidgetResource := result.spxResourceSet.Widget(spxWidgetName)
	if spxWidgetResource == nil {
		result.addDiagnostics(exprDocumentURI, Diagnostic{
			Severity: SeverityError,
			Range:    exprRange,
			Message:  fmt.Sprintf("widget resource %q not found", spxWidgetName),
		})
		return nil
	}
	return spxWidgetResource
}
