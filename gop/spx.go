package gop

import (
	"fmt"
	"go/types"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/goplus/gop/ast"
	"github.com/goplus/goxlsw/internal"
	"github.com/goplus/goxlsw/internal/util"
)

var (
	// GetSpxPkg returns the spx package.
	GetSpxPkg = sync.OnceValue(func() *types.Package {
		spxPkg, err := internal.Importer.Import("github.com/goplus/spx")
		if err != nil {
			panic(fmt.Errorf("failed to import spx package: %w", err))
		}
		return spxPkg
	})

	// GetSpxGameType returns the [spx.Game] type.
	GetSpxGameType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Game").Type().(*types.Named)
	})

	// GetSpxBackdropNameType returns the [spx.BackdropName] type.
	GetSpxBackdropNameType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("BackdropName").Type().(*types.Alias)
	})

	// GetSpxSpriteType returns the [spx.Sprite] type.
	GetSpxSpriteType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Sprite").Type().(*types.Named)
	})

	// GetSpxSpriteImplType returns the [spx.SpriteImpl] type.
	GetSpxSpriteImplType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("SpriteImpl").Type().(*types.Named)
	})

	// GetSpxSpriteNameType returns the [spx.SpriteName] type.
	GetSpxSpriteNameType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("SpriteName").Type().(*types.Alias)
	})

	// GetSpxSpriteCostumeNameType returns the [spx.SpriteCostumeName] type.
	GetSpxSpriteCostumeNameType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("SpriteCostumeName").Type().(*types.Alias)
	})

	// GetSpxSpriteAnimationNameType returns the [spx.SpriteAnimationName] type.
	GetSpxSpriteAnimationNameType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("SpriteAnimationName").Type().(*types.Alias)
	})

	// GetSpxSoundType returns the [spx.Sound] type.
	GetSpxSoundType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Sound").Type().(*types.Named)
	})

	// GetSpxSoundNameType returns the [spx.SoundName] type.
	GetSpxSoundNameType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("SoundName").Type().(*types.Alias)
	})

	// GetSpxWidgetNameType returns the [spx.WidgetName] type.
	GetSpxWidgetNameType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("WidgetName").Type().(*types.Alias)
	})
)

// -----------------------------------------------------------------------------

// RangeSpxSpriteNames iterates spx sprite names.
func RangeSpxSpriteNames(proj *Project, f func(name string) bool) {
	proj.RangeFiles(func(filename string) bool {
		name := path.Base(filename)
		if strings.HasSuffix(name, ".spx") {
			return f(name[:len(name)-4])
		}
		return true
	})
}

// HasSpxSpriteType checks if there is specified spx sprite type.
func HasSpxSpriteType(proj *Project, typ types.Type) (has bool) {
	pkg, _, _, _ := proj.TypeInfo()
	RangeSpxSpriteNames(proj, func(name string) bool {
		if obj := pkg.Scope().Lookup(name); obj != nil && obj.Type() == typ {
			has = true
			return false
		}
		return true
	})
	return
}

// -----------------------------------------------------------------------------

func buildSpxResourceSet(proj *Project) (any, error) {
	set, err := newSpxResourceSet(proj)
	return &spxResourceSetRet{set: set, err: err}, nil
}

type spxResourceSetRet struct {
	set *SpxResourceSet
	err error
}

// SpxResourceSet returns the spx resource set for a Go+ project.
func (p *Project) SpxResourceSet() (set *SpxResourceSet, err error) {
	c, err := p.Cache("spxResourceSet")
	if err != nil {
		return
	}
	ret := c.(*spxResourceSetRet)
	return ret.set, ret.err
}

// -----------------------------------------------------------------------------

func buildSpxResourceRefs(proj *Project) (ret any, err error) {
}

type spxResourceRefsRet struct {
	set  SpxResourceSet
	refs []SpxResourceRef
	err  error
}

// SpxResourceRefs returns the spx resource references for a Go+ project.
func (p *Project) SpxResourceRefs() (set SpxResourceSet, refs []SpxResourceRef, err error) {
	c, err := p.Cache("spxResourceRefs")
	if err != nil {
		return
	}
	ret := c.(*spxResourceRefsRet)
	return ret.set, ret.refs, ret.err
}

// inspectForSpxResourceRefs inspects for spx resource references in the code.
func inspectForSpxResourceRefs(proj *Project) error {
	mainASTFile, err := proj.AST("main.spx")
	if err != nil {
		return fmt.Errorf("failed to get main.spx AST file: %w", err)
	}
	_, typeInfo, _, _ := proj.TypeInfo()
	mainASTFileScope := typeInfo.Scopes[mainASTFile]

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
			valueSpec, ok := ident.Obj.Decl.(*ast.ValueSpec)
			if !ok {
				break
			}
			idx := slices.Index(valueSpec.Names, ident)
			if idx < 0 || idx >= len(valueSpec.Values) {
				break
			}
			expr := valueSpec.Values[idx]

			inspectSpxResourceRefForTypeAtExpr(result, expr, UnwrapPointerType(obj.Type()), nil)
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
			isSpxSpriteResourceAutoBinding = v.Name() == varType.Obj().Name() && HasSpxSpriteType(proj, varType)
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
		s.inspectSpxResourceRefForTypeAtExpr(result, ident, UnwrapPointerType(obj.Type()), nil)
	}

	// Check all identifier uses.
	for ident, obj := range typeInfo.Uses {
		if ident == nil || !ident.Pos().IsValid() || obj == nil {
			continue
		}
		s.inspectSpxResourceRefForTypeAtExpr(result, ident, UnwrapPointerType(obj.Type()), nil)
	}

	// Check all type-checked expressions.
	for expr, tv := range typeInfo.Types {
		if expr == nil || !expr.Pos().IsValid() || tv.IsType() || tv.Type == nil {
			continue // Skip type identifiers.
		}

		switch expr := expr.(type) {
		case *ast.CallExpr:
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
				recvType := UnwrapPointerType(recv.Type())
				switch recvType {
				case GetSpxSpriteType(), GetSpxSpriteImplType():
					spxSpriteResource = s.inspectSpxSpriteResourceRefAtExpr(result, expr, recvType)
				}
			}

			var lastParamType types.Type
			for i, arg := range expr.Args {
				var paramType types.Type
				if i < funcSig.Params().Len() {
					paramType = UnwrapPointerType(funcSig.Params().At(i).Type())
					lastParamType = paramType
				} else {
					// Use the last parameter type for variadic functions.
					paramType = lastParamType
				}

				// Handle slice/array parameter types.
				if sliceType, ok := paramType.(*types.Slice); ok {
					paramType = UnwrapPointerType(sliceType.Elem())
				} else if arrayType, ok := paramType.(*types.Array); ok {
					paramType = UnwrapPointerType(arrayType.Elem())
				}

				if sliceLit, ok := arg.(*ast.SliceLit); ok {
					for _, elt := range sliceLit.Elts {
						s.inspectSpxResourceRefForTypeAtExpr(result, elt, paramType, spxSpriteResource)
					}
				} else {
					s.inspectSpxResourceRefForTypeAtExpr(result, arg, paramType, spxSpriteResource)
				}
			}
		default:
			s.inspectSpxResourceRefForTypeAtExpr(result, expr, UnwrapPointerType(tv.Type), nil)
		}
	}
}

// inspectSpxResourceRefForTypeAtExpr inspects an spx resource reference for a
// given type at an expression.
func inspectSpxResourceRefForTypeAtExpr(result *compileResult, expr ast.Expr, typ types.Type, spxSpriteResource *SpxSpriteResource) {
	if ident, ok := expr.(*ast.Ident); ok {
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
				assignStmt, ok := node.(*ast.AssignStmt)
				if !ok {
					continue
				}

				idx := slices.IndexFunc(assignStmt.Lhs, func(lhs ast.Expr) bool {
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
		inspectSpxBackdropResourceRefAtExpr(result, expr, typ)
	case GetSpxSpriteNameType(), GetSpxSpriteType():
		inspectSpxSpriteResourceRefAtExpr(result, expr, typ)
	case GetSpxSpriteCostumeNameType():
		if spxSpriteResource != nil {
			inspectSpxSpriteCostumeResourceRefAtExpr(result, spxSpriteResource, expr, typ)
		}
	case GetSpxSpriteAnimationNameType():
		if spxSpriteResource != nil {
			inspectSpxSpriteAnimationResourceRefAtExpr(result, spxSpriteResource, expr, typ)
		}
	case GetSpxSoundNameType(), GetSpxSoundType():
		inspectSpxSoundResourceRefAtExpr(result, expr, typ)
	case GetSpxWidgetNameType():
		inspectSpxWidgetResourceRefAtExpr(result, expr, typ)
	default:
		if HasSpxSpriteType(proj, typ) {
			inspectSpxSpriteResourceRefAtExpr(result, expr, typ)
		}
	}
}

// inspectSpxBackdropResourceRefAtExpr inspects an spx backdrop resource
// reference at an expression. It returns the spx backdrop resource if it was
// successfully retrieved.
func inspectSpxBackdropResourceRefAtExpr(result *compileResult, expr ast.Expr, declaredType types.Type) *SpxBackdropResource {
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

	spxBackdropName, ok := GetStringLitOrConstValue(expr, exprTV)
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
	if _, ok := expr.(*ast.Ident); ok {
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
func inspectSpxSpriteResourceRefAtExpr(result *compileResult, expr ast.Expr, declaredType types.Type) *SpxSpriteResource {
	typeInfo := getTypeInfo(result.proj)
	exprDocumentURI := result.nodeDocumentURI(expr)
	exprRange := result.rangeForNode(expr)
	exprTV := typeInfo.Types[expr]

	typ := exprTV.Type
	if declaredType != nil {
		typ = declaredType
	}

	var spxSpriteName string
	if callExpr, ok := expr.(*ast.CallExpr); ok {
		switch fun := callExpr.Fun.(type) {
		case *ast.Ident:
			spxSpriteName = strings.TrimSuffix(path.Base(result.nodeFilename(callExpr)), ".spx")
		case *ast.SelectorExpr:
			ident, ok := fun.X.(*ast.Ident)
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
			spxSpriteName, ok = GetStringLitOrConstValue(expr, exprTV)
			if !ok {
				return nil
			}
			spxResourceRefKind = SpxResourceRefKindStringLiteral
			if _, ok := expr.(*ast.Ident); ok {
				spxResourceRefKind = SpxResourceRefKindConstantReference
			}
		} else {
			ident, ok := expr.(*ast.Ident)
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
func inspectSpxSpriteCostumeResourceRefAtExpr(result *compileResult, spxSpriteResource *SpxSpriteResource, expr ast.Expr, declaredType types.Type) *SpxSpriteCostumeResource {
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

	spxSpriteCostumeName, ok := GetStringLitOrConstValue(expr, exprTV)
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
	if _, ok := expr.(*ast.Ident); ok {
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
func inspectSpxSpriteAnimationResourceRefAtExpr(result *compileResult, spxSpriteResource *SpxSpriteResource, expr ast.Expr, declaredType types.Type) *SpxSpriteAnimationResource {
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

	spxSpriteAnimationName, ok := GetStringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*ast.Ident); ok {
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
func inspectSpxSoundResourceRefAtExpr(result *compileResult, expr ast.Expr, declaredType types.Type) *SpxSoundResource {
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
		spxSoundName, ok = GetStringLitOrConstValue(expr, exprTV)
		if !ok {
			return nil
		}
		spxResourceRefKind = SpxResourceRefKindStringLiteral
		if _, ok := expr.(*ast.Ident); ok {
			spxResourceRefKind = SpxResourceRefKindConstantReference
		}
	case GetSpxSoundType():
		ident, ok := expr.(*ast.Ident)
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
func inspectSpxWidgetResourceRefAtExpr(result *compileResult, expr ast.Expr, declaredType types.Type) *SpxWidgetResource {
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

	spxWidgetName, ok := GetStringLitOrConstValue(expr, exprTV)
	if !ok {
		return nil
	}
	spxResourceRefKind := SpxResourceRefKindStringLiteral
	if _, ok := expr.(*ast.Ident); ok {
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
