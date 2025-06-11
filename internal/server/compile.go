package server

import (
	"fmt"
	"go/types"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/goplus/gogen"
	xgoast "github.com/goplus/xgo/ast"
	xgoscanner "github.com/goplus/xgo/scanner"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgo/x/typesutil"
	"github.com/goplus/xgolsw/internal/analysis/ast/inspector"
	"github.com/goplus/xgolsw/internal/analysis/passes/inspect"
	"github.com/goplus/xgolsw/internal/analysis/protocol"
	"github.com/goplus/xgolsw/internal/pkgdata"
	"github.com/goplus/xgolsw/internal/vfs"
	"github.com/goplus/xgolsw/pkgdoc"
	"github.com/goplus/xgolsw/xgo"
	"github.com/goplus/xgolsw/xgo/xgoutil"
	"github.com/qiniu/x/errors"
)

// errNoMainSpxFile is the error returned when no valid main.spx file is found
// in the main package while compiling.
var errNoMainSpxFile = errors.New("no valid main.spx file found in main package")

func getPkgDoc(proj *xgo.Project) *pkgdoc.PkgDoc {
	ret, _ := proj.PkgDoc()
	return ret
}

func getTypeInfo(proj *xgo.Project) *typesutil.Info {
	_, ret, _, _ := proj.TypeInfo()
	return ret
}

func getPkg(proj *xgo.Project) *types.Package {
	ret, _, _, _ := proj.TypeInfo()
	return ret
}

func getASTPkg(proj *xgo.Project) *xgoast.Package {
	ret, _ := proj.ASTPackage()
	return ret
}

// compileResult contains the compile results and additional information from
// the compile process.
type compileResult struct {
	proj *xgo.Project

	// mainSpxFile is the main.spx file path.
	mainSpxFile string

	// spxSpriteTypes stores the spx sprite types.
	spxSpriteTypes map[types.Type]struct{}

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
}

// newCompileResult creates a new [compileResult].
func newCompileResult(proj *xgo.Project) *compileResult {
	return &compileResult{
		proj:                          proj,
		spxSpriteTypes:                make(map[types.Type]struct{}),
		spxSoundResourceAutoBindings:  make(map[types.Object]struct{}),
		spxSpriteResourceAutoBindings: make(map[types.Object]struct{}),
		diagnostics:                   make(map[DocumentURI][]Diagnostic),
	}
}

// spxDefinitionsFor returns all spx definitions for the given object. It
// returns multiple definitions only if the object is an XGo overloadable
// function.
func (r *compileResult) spxDefinitionsFor(obj types.Object, selectorTypeName string) []SpxDefinition {
	if obj == nil {
		return nil
	}
	if xgoutil.IsInBuiltinPkg(obj) {
		return []SpxDefinition{GetSpxDefinitionForBuiltinObj(obj)}
	}

	var pkgDoc *pkgdoc.PkgDoc
	if xgoutil.IsInMainPkg(obj) {
		pkgDoc = getPkgDoc(r.proj)
	} else {
		pkgPath := xgoutil.PkgPath(obj.Pkg())
		pkgDoc, _ = pkgdata.GetPkgDoc(pkgPath)
	}

	switch obj := obj.(type) {
	case *types.Var:
		return []SpxDefinition{GetSpxDefinitionForVar(obj, selectorTypeName, xgoutil.IsDefinedInClassFieldsDecl(r.proj, obj), pkgDoc)}
	case *types.Const:
		return []SpxDefinition{GetSpxDefinitionForConst(obj, pkgDoc)}
	case *types.TypeName:
		return []SpxDefinition{GetSpxDefinitionForType(obj, pkgDoc)}
	case *types.Func:
		if defIdent := xgoutil.DefIdentFor(getTypeInfo(r.proj), obj); defIdent != nil && defIdent.Implicit() {
			return nil
		}
		if xgoutil.IsUnexpandableXGoOverloadableFunc(obj) {
			return nil
		}
		if funcOverloads := xgoutil.ExpandXGoOverloadableFunc(obj); funcOverloads != nil {
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
// It returns multiple definitions only if the identifier is an XGo
// overloadable function.
func (r *compileResult) spxDefinitionsForIdent(ident *xgoast.Ident) []SpxDefinition {
	if ident.Name == "_" {
		return nil
	}
	typeInfo := getTypeInfo(r.proj)
	return r.spxDefinitionsFor(typeInfo.ObjectOf(ident), SelectorTypeNameForIdent(r.proj, ident))
}

// spxDefinitionsForNamedStruct returns all spx definitions for the given named
// struct type.
func (r *compileResult) spxDefinitionsForNamedStruct(named *types.Named) []SpxDefinition {
	var defs []SpxDefinition
	xgoutil.WalkStruct(named, func(member types.Object, selector *types.Named) bool {
		defs = append(defs, r.spxDefinitionsFor(member, selector.Obj().Name())...)
		return true
	})
	return defs
}

// spxDefinitionForField returns the spx definition for the given field and
// optional selector type name.
func (r *compileResult) spxDefinitionForField(field *types.Var, selectorTypeName string) SpxDefinition {
	var (
		forceVar bool
		pkgDoc   *pkgdoc.PkgDoc
	)
	if defIdent := xgoutil.DefIdentFor(getTypeInfo(r.proj), field); defIdent != nil {
		if selectorTypeName == "" {
			selectorTypeName = SelectorTypeNameForIdent(r.proj, defIdent)
		}
		forceVar = xgoutil.IsDefinedInClassFieldsDecl(r.proj, field)
		pkgDoc = getPkgDoc(r.proj)
	} else {
		pkg := field.Pkg()
		pkgPath := xgoutil.PkgPath(pkg)
		pkgDoc, _ = pkgdata.GetPkgDoc(pkgPath)
	}
	return GetSpxDefinitionForVar(field, selectorTypeName, forceVar, pkgDoc)
}

// spxDefinitionForMethod returns the spx definition for the given method and
// optional selector type name.
func (r *compileResult) spxDefinitionForMethod(method *types.Func, selectorTypeName string) SpxDefinition {
	var pkgDoc *pkgdoc.PkgDoc
	if defIdent := xgoutil.DefIdentFor(getTypeInfo(r.proj), method); defIdent != nil {
		if selectorTypeName == "" {
			selectorTypeName = SelectorTypeNameForIdent(r.proj, defIdent)
		}
		pkgDoc = getPkgDoc(r.proj)
	} else {
		if idx := strings.LastIndex(selectorTypeName, "."); idx >= 0 {
			selectorTypeName = selectorTypeName[idx+1:]
		}
		pkg := method.Pkg()
		pkgPath := xgoutil.PkgPath(pkg)
		pkgDoc, _ = pkgdata.GetPkgDoc(pkgPath)
	}
	return GetSpxDefinitionForFunc(method, selectorTypeName, pkgDoc)
}

// isInSpxEventHandler checks if the given position is inside an spx event
// handler callback.
func (r *compileResult) isInSpxEventHandler(pos xgotoken.Pos) bool {
	astFile := xgoutil.PosASTFile(r.proj, pos)
	if astFile == nil {
		return false
	}

	typeInfo := getTypeInfo(r.proj)

	var isIn bool
	xgoutil.WalkPathEnclosingInterval(astFile, pos-1, pos, false, func(node xgoast.Node) bool {
		callExpr, ok := node.(*xgoast.CallExpr)
		if !ok || len(callExpr.Args) == 0 {
			return true
		}
		funcIdent, ok := callExpr.Fun.(*xgoast.Ident)
		if !ok {
			return true
		}
		funcObj := typeInfo.ObjectOf(funcIdent)
		if !IsInSpxPkg(funcObj) {
			return true
		}
		isIn = IsSpxEventHandlerFuncName(funcIdent.Name)
		return !isIn // Stop walking if we found a match.
	})
	return isIn
}

// spxResourceRefAtASTFilePosition returns the spx resource reference at the
// given position in the given AST file.
func (r *compileResult) spxResourceRefAtASTFilePosition(astFile *xgoast.File, position xgotoken.Position) *SpxResourceRef {
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
func (r *compileResult) spxImportsAtASTFilePosition(astFile *xgoast.File, position xgotoken.Position) *SpxReferencePkg {
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

// hasSpxSpriteType reports whether the given type is an spx sprite type.
func (r *compileResult) hasSpxSpriteType(typ types.Type) bool {
	_, ok := r.spxSpriteTypes[typ]
	return ok
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

		astFile, err := snapshot.AST(spxFile)
		if err != nil {
			var (
				errorList xgoscanner.ErrorList
				codeError *gogen.CodeError
			)
			if errors.As(err, &errorList) && astFile.Pos().IsValid() {
				// Handle parse errors.
				for _, e := range errorList {
					result.addDiagnostics(documentURI, Diagnostic{
						Severity: SeverityError,
						Range:    RangeForASTFilePosition(result.proj, astFile, e.Pos),
						Message:  e.Msg,
					})
				}
			} else if errors.As(err, &codeError) {
				// Handle code generation errors.
				result.addDiagnostics(documentURI, Diagnostic{
					Severity: SeverityError,
					Range:    RangeForPos(result.proj, codeError.Pos),
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
		if astFile.Name.Name != "main" && astFile.Pos().IsValid() {
			result.addDiagnostics(documentURI, Diagnostic{
				Severity: SeverityError,
				Range:    RangeForASTFileNode(result.proj, astFile, astFile.Name),
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

	handleErr := func(err error) {
		if typeErr, ok := err.(types.Error); ok {
			if !typeErr.Pos.IsValid() {
				panic(fmt.Sprintf("unexpected nopos error: %s", typeErr.Msg))
			}
			position := typeErr.Fset.Position(typeErr.Pos)
			documentURI := s.toDocumentURI(position.Filename)
			result.addDiagnostics(documentURI, Diagnostic{
				Severity: SeverityError,
				Range:    RangeForPos(result.proj, typeErr.Pos),
				Message:  typeErr.Msg,
			})
		}
	}

	pkg, _, err, _ := snapshot.TypeInfo()
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

	vfs.RangeSpriteNames(snapshot, func(name string) bool {
		obj := pkg.Scope().Lookup(name)
		if obj != nil {
			named, ok := obj.Type().(*types.Named)
			if ok {
				result.spxSpriteTypes[named] = struct{}{}
			}
		}
		return true
	})

	s.inspectForSpxResourceSet(snapshot, result)
	s.inspectForSpxResourceRefs(result)
	s.inspectDiagnosticsAnalyzers(result)

	return result, nil
}

// compileAndGetASTFileForDocumentURI handles common compilation and file
// retrieval logic for a given document URI. The returned astFile is probably
// nil even if the compilation succeeded.
func (s *Server) compileAndGetASTFileForDocumentURI(uri DocumentURI) (result *compileResult, spxFile string, astFile *xgoast.File, err error) {
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
	mainASTFile, _ := result.proj.AST(result.mainSpxFile)
	typeInfo := getTypeInfo(snapshot)

	var spxResourceRootDir string
	for expr, tv := range typeInfo.Types {
		if expr == nil || !expr.Pos().IsValid() || expr.Pos() < mainASTFile.Pos() || expr.End() > mainASTFile.End() {
			continue
		}

		callExpr, ok := expr.(*xgoast.CallExpr)
		if !ok || len(callExpr.Args) == 0 || tv.Type != GetSpxGoptGameRunFunc().Type() {
			continue
		}
		firstArg := callExpr.Args[0]
		firstArgTV, ok := typeInfo.Types[firstArg]
		if !ok {
			continue
		}

		if types.AssignableTo(firstArgTV.Type, types.Typ[types.String]) {
			spxResourceRootDir, _ = xgoutil.StringLitOrConstValue(firstArg, firstArgTV)
		} else {
			documentURI := s.toDocumentURI(result.mainSpxFile)
			result.addDiagnostics(documentURI, Diagnostic{
				Severity: SeverityError,
				Range:    RangeForNode(result.proj, firstArg),
				Message:  "first argument of run must be a string literal or constant",
			})
		}
		break
	}
	if spxResourceRootDir == "" {
		spxResourceRootDir = "assets"
	}
	spxResourceRootFS := vfs.Sub(snapshot, spxResourceRootDir)

	spxResourceSet, err := NewSpxResourceSet(spxResourceRootFS)
	if err != nil {
		documentURI := s.toDocumentURI(result.mainSpxFile)
		result.addDiagnostics(documentURI, Diagnostic{
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
//  1. Creates an analysis pass with file-specific information
//  2. Runs all registered analyzers on the file
//  3. Collects diagnostics from analyzers
//  4. Reports any analyzer errors as diagnostics
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
			Files:     []*xgoast.File{astFile},
			TypesInfo: typeInfo,
			Report: func(d protocol.Diagnostic) {
				diagnostics = append(diagnostics, Diagnostic{
					Range:    RangeForPosEnd(proj, d.Pos, d.End),
					Severity: SeverityError,
					Message:  d.Message,
				})
			},
			ResultOf: map[*protocol.Analyzer]any{
				inspect.Analyzer: inspector.New([]*xgoast.File{astFile}),
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

		documentURI := s.toDocumentURI(spxFile)
		result.addDiagnostics(documentURI, diagnostics...)
	}
}

// isInspectableSpxResourceType reports whether the given type is an
// inspectable spx resource type.
func isInspectableSpxResourceType(typ types.Type) bool {
	switch typ {
	case GetSpxBackdropNameType(),
		GetSpxSpriteNameType(),
		GetSpxSpriteType(),
		GetSpxSpriteCostumeNameType(),
		GetSpxSpriteAnimationNameType(),
		GetSpxSoundNameType(),
		GetSpxSoundType(),
		GetSpxWidgetNameType():
		return true
	}
	return false
}

// inspectForSpxResourceRefs inspects for spx resource references in the code.
func (s *Server) inspectForSpxResourceRefs(result *compileResult) {
	mainASTFile, _ := result.proj.AST(result.mainSpxFile)
	typeInfo := getTypeInfo(result.proj)

	// Check all identifier definitions.
	for ident, obj := range typeInfo.Defs {
		if ident == nil || !ident.Pos().IsValid() || ident.Implicit() || obj == nil {
			continue
		}

		switch obj.(type) {
		case *types.Const, *types.Var:
			if ident.Obj == nil {
				break
			}
			valueSpec, ok := ident.Obj.Decl.(*xgoast.ValueSpec)
			if !ok {
				break
			}
			idx := slices.Index(valueSpec.Names, ident)
			if idx < 0 || idx >= len(valueSpec.Values) {
				break
			}
			expr := valueSpec.Values[idx]

			s.inspectSpxResourceRefForTypeAtExpr(result, expr, xgoutil.DerefType(obj.Type()), nil)
		}

		// Check for auto-binding spx resources.
		if ident.Pos() >= mainASTFile.Pos() && ident.End() <= mainASTFile.End() {
			v, ok := obj.(*types.Var)
			if !ok {
				continue
			}
			varType, ok := v.Type().(*types.Named)
			if !ok {
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
				isSpxSpriteResourceAutoBinding = v.Name() == varType.Obj().Name() && result.hasSpxSpriteType(varType)
			}
			if !isSpxSoundResourceAutoBinding && !isSpxSpriteResourceAutoBinding {
				continue
			}

			if !xgoutil.IsDefinedInClassFieldsDecl(result.proj, obj) {
				spxFile := xgoutil.NodeFilename(result.proj, ident)
				documentURI := s.toDocumentURI(spxFile)
				result.addDiagnostics(documentURI, Diagnostic{
					Severity: SeverityWarning,
					Range:    RangeForNode(result.proj, ident),
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
			s.inspectSpxResourceRefForTypeAtExpr(result, ident, xgoutil.DerefType(obj.Type()), nil)
		}
	}

	// Check all type-checked expressions.
	for expr, tv := range typeInfo.Types {
		if expr == nil || !expr.Pos().IsValid() || tv.IsType() || tv.Type == nil {
			continue
		}

		switch expr := expr.(type) {
		case *xgoast.CallExpr:
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
				recvType := xgoutil.DerefType(recv.Type())
				switch recvType {
				case GetSpxSpriteType(), GetSpxSpriteImplType():
					spxSpriteResource = s.inspectSpxSpriteResourceRefAtExpr(result, expr, recvType)
				}
			}

			var lastParamType types.Type
			for i, arg := range expr.Args {
				var paramType types.Type
				if i < funcSig.Params().Len() {
					paramType = xgoutil.DerefType(funcSig.Params().At(i).Type())
					lastParamType = paramType
				} else {
					// Use the last parameter type for variadic functions.
					paramType = lastParamType
				}

				// Handle slice/array parameter types.
				if sliceType, ok := paramType.(*types.Slice); ok {
					paramType = xgoutil.DerefType(sliceType.Elem())
				} else if arrayType, ok := paramType.(*types.Array); ok {
					paramType = xgoutil.DerefType(arrayType.Elem())
				}

				if sliceLit, ok := arg.(*xgoast.SliceLit); ok {
					for _, elt := range sliceLit.Elts {
						s.inspectSpxResourceRefForTypeAtExpr(result, elt, paramType, spxSpriteResource)
					}
				} else {
					s.inspectSpxResourceRefForTypeAtExpr(result, arg, paramType, spxSpriteResource)
				}
			}
		default:
			typ := xgoutil.DerefType(tv.Type)
			if isInspectableSpxResourceType(typ) || result.hasSpxSpriteType(typ) {
				s.inspectSpxResourceRefForTypeAtExpr(result, expr, typ, nil)
			}
		}
	}
}

// inspectSpxResourceRefForTypeAtExpr inspects an spx resource reference for a
// given type at an expression.
func (s *Server) inspectSpxResourceRefForTypeAtExpr(result *compileResult, expr xgoast.Expr, typ types.Type, spxSpriteResource *SpxSpriteResource) {
	if ident, ok := expr.(*xgoast.Ident); ok {
		switch typ {
		case GetSpxBackdropNameType(),
			GetSpxSpriteNameType(),
			GetSpxSoundNameType(),
			GetSpxWidgetNameType():
			astFile := xgoutil.NodeASTFile(result.proj, ident)
			if astFile == nil {
				return
			}

			xgoutil.WalkPathEnclosingInterval(astFile, ident.Pos(), ident.End(), false, func(node xgoast.Node) bool {
				assignStmt, ok := node.(*xgoast.AssignStmt)
				if !ok {
					return true
				}

				idx := slices.IndexFunc(assignStmt.Lhs, func(lhs xgoast.Expr) bool {
					return lhs == ident
				})
				if idx < 0 || idx >= len(assignStmt.Rhs) {
					return true
				}
				expr = assignStmt.Rhs[idx]
				return false
			})
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
		if result.hasSpxSpriteType(typ) {
			s.inspectSpxSpriteResourceRefAtExpr(result, expr, typ)
		}
	}
}

// inspectSpxBackdropResourceRefAtExpr inspects an spx backdrop resource
// reference at an expression. It returns the spx backdrop resource if it was
// successfully retrieved.
func (s *Server) inspectSpxBackdropResourceRefAtExpr(result *compileResult, expr xgoast.Expr, declaredType types.Type) *SpxBackdropResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := s.nodeDocumentURI(result.proj, expr)
	exprRange := RangeForNode(result.proj, expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxBackdropNameType() {
		return nil
	}

	spxBackdropName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
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
	if _, ok := expr.(*xgoast.Ident); ok {
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
func (s *Server) inspectSpxSpriteResourceRefAtExpr(result *compileResult, expr xgoast.Expr, declaredType types.Type) *SpxSpriteResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := s.nodeDocumentURI(result.proj, expr)
	exprRange := RangeForNode(result.proj, expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var spxSpriteName string
	if callExpr, ok := expr.(*xgoast.CallExpr); ok {
		switch fun := callExpr.Fun.(type) {
		case *xgoast.Ident:
			spxSpriteName = strings.TrimSuffix(path.Base(xgoutil.NodeFilename(result.proj, callExpr)), ".spx")
		case *xgoast.SelectorExpr:
			ident, ok := fun.X.(*xgoast.Ident)
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
			spxSpriteName, ok = xgoutil.StringLitOrConstValue(expr, exprTV)
			if !ok {
				return nil
			}
			spxResourceRefKind = SpxResourceRefKindStringLiteral
			if _, ok := expr.(*xgoast.Ident); ok {
				spxResourceRefKind = SpxResourceRefKindConstantReference
			}
		} else {
			ident, ok := expr.(*xgoast.Ident)
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
			defIdent := xgoutil.DefIdentFor(typeInfo, obj)
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
func (s *Server) inspectSpxSpriteCostumeResourceRefAtExpr(result *compileResult, spxSpriteResource *SpxSpriteResource, expr xgoast.Expr, declaredType types.Type) *SpxSpriteCostumeResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := s.nodeDocumentURI(result.proj, expr)
	exprRange := RangeForNode(result.proj, expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxSpriteCostumeNameType() {
		return nil
	}

	spxSpriteCostumeName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
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
	if _, ok := expr.(*xgoast.Ident); ok {
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
func (s *Server) inspectSpxSpriteAnimationResourceRefAtExpr(result *compileResult, spxSpriteResource *SpxSpriteResource, expr xgoast.Expr, declaredType types.Type) *SpxSpriteAnimationResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := s.nodeDocumentURI(result.proj, expr)
	exprRange := RangeForNode(result.proj, expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxSpriteAnimationNameType() {
		return nil
	}

	spxSpriteAnimationName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*xgoast.Ident); ok {
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
func (s *Server) inspectSpxSoundResourceRefAtExpr(result *compileResult, expr xgoast.Expr, declaredType types.Type) *SpxSoundResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := s.nodeDocumentURI(result.proj, expr)
	exprRange := RangeForNode(result.proj, expr)
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
		spxSoundName, ok = xgoutil.StringLitOrConstValue(expr, exprTV)
		if !ok {
			return nil
		}
		spxResourceRefKind = SpxResourceRefKindStringLiteral
		if _, ok := expr.(*xgoast.Ident); ok {
			spxResourceRefKind = SpxResourceRefKindConstantReference
		}
	case GetSpxSoundType():
		ident, ok := expr.(*xgoast.Ident)
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
		defIdent := xgoutil.DefIdentFor(typeInfo, obj)
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
func (s *Server) inspectSpxWidgetResourceRefAtExpr(result *compileResult, expr xgoast.Expr, declaredType types.Type) *SpxWidgetResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := s.nodeDocumentURI(result.proj, expr)
	exprRange := RangeForNode(result.proj, expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}
	if typ != GetSpxWidgetNameType() {
		return nil
	}

	spxWidgetName, ok := xgoutil.StringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*xgoast.Ident); ok {
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
