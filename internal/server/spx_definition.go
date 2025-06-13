package server

import (
	"fmt"
	"go/types"
	"html/template"
	"slices"
	"strings"
	"sync"

	"github.com/goplus/xgolsw/internal"
	"github.com/goplus/xgolsw/internal/pkgdata"
	"github.com/goplus/xgolsw/pkgdoc"
	"github.com/goplus/xgolsw/xgo/xgoutil"
)

// SpxDefinition represents an spx definition.
type SpxDefinition struct {
	// TypeHint represents a type hint for this definition. It may be nil if
	// the definition has no associated type.
	TypeHint types.Type

	ID       SpxDefinitionIdentifier
	Overview string
	Detail   string

	CompletionItemLabel            string
	CompletionItemKind             CompletionItemKind
	CompletionItemInsertText       string
	CompletionItemInsertTextFormat InsertTextFormat
}

// HTML returns the HTML representation of the definition.
func (def SpxDefinition) HTML() string {
	return fmt.Sprintf("<definition-item def-id=%q overview=%q>\n%s</definition-item>\n", template.HTMLEscapeString(def.ID.String()), template.HTMLEscapeString(def.Overview), def.Detail)
}

// CompletionItem constructs a [CompletionItem] from the definition.
func (def SpxDefinition) CompletionItem() CompletionItem {
	return CompletionItem{
		Label:            def.CompletionItemLabel,
		Kind:             def.CompletionItemKind,
		Documentation:    &Or_CompletionItem_documentation{Value: MarkupContent{Kind: Markdown, Value: def.HTML()}},
		InsertText:       def.CompletionItemInsertText,
		InsertTextFormat: &def.CompletionItemInsertTextFormat,
		Data: &CompletionItemData{
			Definition: &def.ID,
		},
	}
}

var (
	// GeneralSpxDefinitions are general spx definitions.
	GeneralSpxDefinitions = []SpxDefinition{
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("for_iterate")},
			Overview: "for i, v <- set { ... }",
			Detail:   "Iterate within given set",

			CompletionItemLabel:            "for",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "for ${1:i}, ${2:v} <- ${3:set} {\n\t$0\n}",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("for_loop_with_condition")},
			Overview: "for condition { ... }",
			Detail:   "Loop with condition",

			CompletionItemLabel:            "for",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "for ${1:condition} {\n\t$0\n}",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("for_loop_with_range")},
			Overview: "for i <- start:end { ... }",
			Detail:   "Loop with range",

			CompletionItemLabel:            "for",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "for ${1:i} <- ${2:start}:${3:end} {\n\t$0\n}",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("if_statement")},
			Overview: "if condition { ... }",
			Detail:   "If statement",

			CompletionItemLabel:            "if",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "if ${1:condition} {\n\t$0\n}",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("if_else_statement")},
			Overview: "if condition { ... } else { ... }",
			Detail:   "If else statement",

			CompletionItemLabel:            "if",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "if ${1:condition} {\n\t${2:}\n} else {\n\t$0\n}",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("var_declaration")},
			Overview: "var name type",
			Detail:   "Variable declaration, e.g., `var count int`",

			CompletionItemLabel:            "var",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "var ${1:name} $0",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
	}

	// FileScopeSpxDefinitions are spx definitions that are only available
	// in file scope.
	FileScopeSpxDefinitions = []SpxDefinition{
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("import_declaration")},
			Overview: "import \"package\"",
			Detail:   "Import package declaration, e.g., `import \"fmt\"`",

			CompletionItemLabel:            "import",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "import \"${1:package}\"$0",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
		{
			ID:       SpxDefinitionIdentifier{Name: ToPtr("func_declaration")},
			Overview: "func name(params) { ... }",
			Detail:   "Function declaration, e.g., `func add(a int, b int) int {}`",

			CompletionItemLabel:            "func",
			CompletionItemKind:             KeywordCompletion,
			CompletionItemInsertText:       "func ${1:name}(${2:params}) ${3:returnType} {\n\t$0\n}",
			CompletionItemInsertTextFormat: SnippetTextFormat,
		},
	}

	// builtinSpxDefinitionOverviews contains overview descriptions for
	// builtin spx definitions.
	builtinSpxDefinitionOverviews = map[string]string{
		// Variables.
		"nil": "var nil Type",

		// Constants.
		"false": "const false = 0 != 0",
		"iota":  "const iota = 0",
		"true":  "const true = 0 == 0",

		// Types.
		"any":        "type any",
		"bool":       "type bool",
		"byte":       "type byte",
		"complex64":  "type complex64",
		"complex128": "type complex128",
		"error":      "type error",
		"float32":    "type float32",
		"float64":    "type float64",
		"int":        "type int",
		"int8":       "type int8",
		"int16":      "type int16",
		"int32":      "type int32",
		"int64":      "type int64",
		"rune":       "type rune",
		"string":     "type string",
		"uint":       "type uint",
		"uint8":      "type uint8",
		"uint16":     "type uint16",
		"uint32":     "type uint32",
		"uint64":     "type uint64",
		"uintptr":    "type uintptr",

		// Functions.
		"append":  "func append(slice []T, elems ...T) []T",
		"cap":     "func cap(v Type) int",
		"clear":   "func clear(m Type)",
		"close":   "func close(c chan<- Type)",
		"complex": "func complex(r, i FloatType) ComplexType",
		"copy":    "func copy(dst, src []Type) int",
		"delete":  "func delete(m map[Type]Type1, key Type)",
		"imag":    "func imag(c ComplexType) FloatType",
		"len":     "func len(v Type) int",
		"make":    "func make(t Type, size ...IntegerType) Type",
		"max":     "func max(x Type, y ...Type) Type",
		"min":     "func min(x Type, y ...Type) Type",
		"new":     "func new(Type) *Type",
		"panic":   "func panic(v interface{})",
		"print":   "func print(args ...Type)",
		"println": "func println(args ...Type)",
		"real":    "func real(c ComplexType) FloatType",
		"recover": "func recover() interface{}",
	}

	// xgoBuiltinAliases contains aliases for XGo builtins.
	//
	// See github.com/goplus/xgo/cl.initBuiltin for the list of XGo builtin aliases.
	xgoBuiltinAliases = map[string]string{
		// Types.
		"bigfloat": "github.com/qiniu/x/xgo/ng#Bigfloat",
		"bigint":   "github.com/qiniu/x/xgo/ng#Bigint",
		"bigrat":   "github.com/qiniu/x/xgo/ng#Bigrat",
		"int128":   "github.com/qiniu/x/xgo/ng#Int128",
		"uint128":  "github.com/qiniu/x/xgo/ng#Uint128",

		// Functions.
		"blines":   "github.com/qiniu/x/osx#BLines",
		"create":   "os#Create",
		"echo":     "fmt#Println",
		"errorf":   "fmt#Errorf",
		"fprint":   "fmt#Fprint",
		"fprintf":  "fmt#Fprintf",
		"fprintln": "fmt#Fprintln",
		"lines":    "github.com/qiniu/x/osx#Lines",
		"newRange": "github.com/qiniu/x/xgo#NewRange__0",
		"open":     "os#Open",
		"print":    "fmt#Print",
		"printf":   "fmt#Printf",
		"println":  "fmt#Println",
		"sprint":   "fmt#Sprint",
		"sprintf":  "fmt#Sprintf",
		"sprintln": "fmt#Sprintln",
		// "type":     "reflect#TypeOf",
	}
)

// GetSpxDefinitionForBuiltinObj returns the spx definition for the given object.
func GetSpxDefinitionForBuiltinObj(obj types.Object) SpxDefinition {
	const pkgPath = "builtin"

	idName := obj.Name()
	if def, err := getSpxDefinitionForXGoBuiltinAlias(idName); err == nil {
		return def
	}

	overview, ok := builtinSpxDefinitionOverviews[idName]
	if !ok {
		overview = "builtin " + idName
	}

	var detail string
	if pkgDoc, err := pkgdata.GetPkgDoc(pkgPath); err == nil {
		if doc, ok := pkgDoc.Vars[idName]; ok {
			detail = doc
		} else if doc, ok := pkgDoc.Consts[idName]; ok {
			detail = doc
		} else if typeDoc, ok := pkgDoc.Types[idName]; ok {
			if doc, ok := typeDoc.Fields[idName]; ok {
				detail = doc
			} else if doc, ok := typeDoc.Methods[idName]; ok {
				detail = doc
			} else {
				detail = typeDoc.Doc
			}
		} else if doc, ok := pkgDoc.Funcs[idName]; ok {
			detail = doc
		}
	}

	completionItemKind := TextCompletion
	if keyword, _, ok := strings.Cut(overview, " "); ok {
		switch keyword {
		case "var":
			completionItemKind = VariableCompletion
		case "const":
			completionItemKind = ConstantCompletion
		case "type":
			switch idName {
			case "any", "error":
				completionItemKind = InterfaceCompletion
			default:
				completionItemKind = ClassCompletion
			}
		case "func":
			completionItemKind = FunctionCompletion
		}
	}

	return SpxDefinition{
		TypeHint: obj.Type(),

		ID: SpxDefinitionIdentifier{
			Package: ToPtr(pkgPath),
			Name:    &idName,
		},
		Overview: overview,
		Detail:   detail,

		CompletionItemLabel:            obj.Name(),
		CompletionItemKind:             completionItemKind,
		CompletionItemInsertText:       obj.Name(),
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}
}

// GetBuiltinSpxDefinitions returns the builtin spx definitions.
var GetBuiltinSpxDefinitions = sync.OnceValue(func() []SpxDefinition {
	names := types.Universe.Names()
	defs := make([]SpxDefinition, 0, len(names)+len(xgoBuiltinAliases))
	for _, name := range names {
		if _, ok := xgoBuiltinAliases[name]; ok {
			continue
		}
		if obj := types.Universe.Lookup(name); obj != nil && obj.Pkg() == nil {
			defs = append(defs, GetSpxDefinitionForBuiltinObj(obj))
		}
	}
	for alias := range xgoBuiltinAliases {
		def, err := getSpxDefinitionForXGoBuiltinAlias(alias)
		if err != nil {
			panic(fmt.Errorf("failed to get spx definition for xgo builtin alias %q: %w", alias, err))
		}
		defs = append(defs, def)
	}
	return slices.Clip(defs)
})

// getSpxDefinitionForXGoBuiltinAlias returns the spx definition for the
// given XGo builtin alias.
func getSpxDefinitionForXGoBuiltinAlias(alias string) (SpxDefinition, error) {
	ref, ok := xgoBuiltinAliases[alias]
	if !ok {
		return SpxDefinition{}, fmt.Errorf("unknown xgo builtin alias: %s", alias)
	}

	pkgPath, name, ok := strings.Cut(ref, "#")
	if !ok {
		return SpxDefinition{}, fmt.Errorf("invalid xgo builtin alias: %s", alias)
	}
	pkg, err := internal.Importer.Import(pkgPath)
	if err != nil {
		return SpxDefinition{}, fmt.Errorf("failed to import package for xgo builtin alias %q: %w", alias, err)
	}
	pkgDoc, err := pkgdata.GetPkgDoc(pkgPath)
	if err != nil {
		return SpxDefinition{}, fmt.Errorf("failed to get package doc for xgo builtin alias %q: %w", alias, err)
	}

	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return SpxDefinition{}, fmt.Errorf("symbol %s not found in package %s", name, pkgPath)
	}
	var def SpxDefinition
	switch obj := obj.(type) {
	case *types.TypeName:
		def = GetSpxDefinitionForType(obj, pkgDoc)
	case *types.Func:
		def = GetSpxDefinitionForFunc(obj, "", pkgDoc)
	default:
		return SpxDefinition{}, fmt.Errorf("unexpected object type for xgo builtin alias %q: %T", alias, obj)
	}

	return SpxDefinition{
		TypeHint: obj.Type(),
		ID: SpxDefinitionIdentifier{
			Package: ToPtr("builtin"),
			Name:    &alias,
		},
		Overview: def.Overview,
		Detail:   def.Detail,

		CompletionItemLabel:            alias,
		CompletionItemKind:             def.CompletionItemKind,
		CompletionItemInsertText:       alias,
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}, nil
}

var (
	// GetMathPkg returns the math package.
	GetMathPkg = sync.OnceValue(func() *types.Package {
		mathPkg, err := internal.Importer.Import("math")
		if err != nil {
			panic(fmt.Errorf("failed to import math package: %w", err))
		}
		return mathPkg
	})

	// GetMathPkgSpxDefinitions returns the spx definitions for the math package.
	GetMathPkgSpxDefinitions = sync.OnceValue(func() []SpxDefinition {
		mathPkg := GetMathPkg()
		mathPkgPath := xgoutil.PkgPath(mathPkg)
		mathPkgDoc, err := pkgdata.GetPkgDoc(mathPkgPath)
		if err != nil {
			panic(fmt.Errorf("failed to get math package doc: %w", err))
		}
		return GetSpxDefinitionsForPkg(mathPkg, mathPkgDoc)
	})
)

// SpxPkgPath is the path to the spx package.
const SpxPkgPath = "github.com/goplus/spx/v2"

var (

	// GetSpxPkg returns the spx package.
	GetSpxPkg = sync.OnceValue(func() *types.Package {
		spxPkg, err := internal.Importer.Import(SpxPkgPath)
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

	// GetSpxDirectionType returns the [spx.Direction] type.
	GetSpxDirectionType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Direction").Type().(*types.Alias)
	})

	// GetSpxEffectKindType returns the [spx.EffectKind] type.
	GetSpxEffectKindType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("EffectKind").Type().(*types.Named)
	})

	// GetSpxKeyType returns the [spx.Key] type.
	GetSpxKeyType = sync.OnceValue(func() *types.Alias {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Key").Type().(*types.Alias)
	})

	// GetSpxPlayActionType returns the [spx.PlayAction] type.
	GetSpxPlayActionType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("PlayAction").Type().(*types.Named)
	})

	// GetSpxSpecialObjType returns the [spx.SpecialObj] type.
	GetSpxSpecialObjType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Edge").Type().(*types.Named)
	})

	// GetSpxRotationStyleType returns the [spx.RotationStyle] type.
	GetSpxRotationStyleType = sync.OnceValue(func() *types.Named {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("RotationStyle").Type().(*types.Named)
	})

	// GetSpxPkgDefinitions returns the spx definitions for the spx package.
	GetSpxPkgDefinitions = sync.OnceValue(func() []SpxDefinition {
		spxPkg := GetSpxPkg()
		spxPkgPath := xgoutil.PkgPath(spxPkg)
		spxPkgDoc, err := pkgdata.GetPkgDoc(spxPkgPath)
		if err != nil {
			panic(fmt.Errorf("failed to get spx package doc: %w", err))
		}
		return GetSpxDefinitionsForPkg(spxPkg, spxPkgDoc)
	})

	// GetSpxHSBFunc returns the [spx.HSB] type.
	GetSpxHSBFunc = sync.OnceValue(func() *types.Func {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("HSB").(*types.Func)
	})

	// GetSpxHSBAFunc returns the [spx.HSBA] type.
	GetSpxHSBAFunc = sync.OnceValue(func() *types.Func {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("HSBA").(*types.Func)
	})

	// GetSpxGoptGameRunFunc returns the [spx.Gopt_Game_Run] type.
	GetSpxGoptGameRunFunc = sync.OnceValue(func() *types.Func {
		spxPkg := GetSpxPkg()
		return spxPkg.Scope().Lookup("Gopt_Game_Run").(*types.Func)
	})
)

// nonMainPkgSpxDefsCache is a cache of non-main package spx definitions.
var nonMainPkgSpxDefsCache sync.Map // map[*types.Package][]SpxDefinition

// GetSpxDefinitionsForPkg returns the spx definitions for the given package.
func GetSpxDefinitionsForPkg(pkg *types.Package, pkgDoc *pkgdoc.PkgDoc) (defs []SpxDefinition) {
	if !xgoutil.IsMainPkg(pkg) {
		if defsIface, ok := nonMainPkgSpxDefsCache.Load(pkg); ok {
			return defsIface.([]SpxDefinition)
		}
		defer func() {
			nonMainPkgSpxDefsCache.Store(pkg, defs)
		}()
	}

	names := pkg.Scope().Names()
	defs = make([]SpxDefinition, 0, len(names))
	for _, name := range names {
		if obj := pkg.Scope().Lookup(name); obj != nil && obj.Exported() {
			switch obj := obj.(type) {
			case *types.Var:
				defs = append(defs, GetSpxDefinitionForVar(obj, "", false, pkgDoc))
			case *types.Const:
				defs = append(defs, GetSpxDefinitionForConst(obj, pkgDoc))
			case *types.TypeName:
				defs = append(defs, GetSpxDefinitionForType(obj, pkgDoc))
			case *types.Func:
				if funcOverloads := xgoutil.ExpandXGoOverloadableFunc(obj); funcOverloads != nil {
					for _, funcOverload := range funcOverloads {
						defs = append(defs, GetSpxDefinitionForFunc(funcOverload, "", pkgDoc))
					}
				} else {
					defs = append(defs, GetSpxDefinitionForFunc(obj, "", pkgDoc))
				}
			case *types.PkgName:
				defs = append(defs, GetSpxDefinitionForPkg(obj, pkgDoc))
			}
		}
	}
	return slices.Clip(defs)
}

// nonMainPkgSpxDefCacheForVars is a cache of non-main package spx definitions
// for variables.
var nonMainPkgSpxDefCacheForVars sync.Map // map[nonMainPkgSpxDefCacheForVarsKey]SpxDefinition

// nonMainPkgSpxDefCacheForVarsKey is the key for the non-main package spx
// definition cache for variables.
type nonMainPkgSpxDefCacheForVarsKey struct {
	v                *types.Var
	selectorTypeName string
}

// GetSpxDefinitionForVar returns the spx definition for the provided variable.
func GetSpxDefinitionForVar(v *types.Var, selectorTypeName string, forceVar bool, pkgDoc *pkgdoc.PkgDoc) (def SpxDefinition) {
	if !xgoutil.IsInMainPkg(v) {
		cacheKey := nonMainPkgSpxDefCacheForVarsKey{
			v:                v,
			selectorTypeName: selectorTypeName,
		}
		if defIface, ok := nonMainPkgSpxDefCacheForVars.Load(cacheKey); ok {
			return defIface.(SpxDefinition)
		}
		defer func() {
			nonMainPkgSpxDefCacheForVars.Store(cacheKey, def)
		}()
	}

	if IsInSpxPkg(v) && selectorTypeName == "Sprite" {
		selectorTypeName = "SpriteImpl"
	}

	var overview strings.Builder
	if !v.IsField() || forceVar {
		overview.WriteString("var ")
	} else {
		overview.WriteString("field ")
	}
	overview.WriteString(v.Name())
	overview.WriteString(" ")
	overview.WriteString(GetSimplifiedTypeString(v.Type()))

	var detail string
	if pkgDoc != nil {
		if selectorTypeName == "" {
			detail = pkgDoc.Vars[v.Name()]
		} else if typeDoc, ok := pkgDoc.Types[selectorTypeName]; ok {
			detail = typeDoc.Fields[v.Name()]
		}
	}

	idName := v.Name()
	if selectorTypeName != "" {
		selectorTypeDisplayName := selectorTypeName
		if IsInSpxPkg(v) && selectorTypeDisplayName == "SpriteImpl" {
			selectorTypeDisplayName = "Sprite"
		}
		idName = selectorTypeDisplayName + "." + idName
	}
	completionItemKind := VariableCompletion
	if strings.HasPrefix(overview.String(), "field ") {
		completionItemKind = FieldCompletion
	}
	def = SpxDefinition{
		TypeHint: v.Type(),

		ID: SpxDefinitionIdentifier{
			Package: ToPtr(xgoutil.PkgPath(v.Pkg())),
			Name:    &idName,
		},
		Overview: overview.String(),
		Detail:   detail,

		CompletionItemLabel:            v.Name(),
		CompletionItemKind:             completionItemKind,
		CompletionItemInsertText:       v.Name(),
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}
	return
}

// nonMainPkgSpxDefCacheForConsts is a cache of non-main package spx definitions
// for constants.
var nonMainPkgSpxDefCacheForConsts sync.Map // map[*types.Const]SpxDefinition

// GetSpxDefinitionForConst returns the spx definition for the provided constant.
func GetSpxDefinitionForConst(c *types.Const, pkgDoc *pkgdoc.PkgDoc) (def SpxDefinition) {
	if !xgoutil.IsInMainPkg(c) {
		if defIface, ok := nonMainPkgSpxDefCacheForConsts.Load(c); ok {
			return defIface.(SpxDefinition)
		}
		defer func() {
			nonMainPkgSpxDefCacheForConsts.Store(c, def)
		}()
	}

	var overview strings.Builder
	overview.WriteString("const ")
	overview.WriteString(c.Name())
	overview.WriteString(" = ")
	overview.WriteString(c.Val().String())

	var detail string
	if pkgDoc != nil {
		detail = pkgDoc.Consts[c.Name()]
	}

	def = SpxDefinition{
		TypeHint: c.Type(),

		ID: SpxDefinitionIdentifier{
			Package: ToPtr(xgoutil.PkgPath(c.Pkg())),
			Name:    ToPtr(c.Name()),
		},
		Overview: overview.String(),
		Detail:   detail,

		CompletionItemLabel:            c.Name(),
		CompletionItemKind:             ConstantCompletion,
		CompletionItemInsertText:       c.Name(),
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}
	return
}

// nonMainPkgSpxDefCacheForTypes is a cache of non-main package spx definitions
// for types.
var nonMainPkgSpxDefCacheForTypes sync.Map // map[*types.TypeName]SpxDefinition

// GetSpxDefinitionForType returns the spx definition for the provided type.
func GetSpxDefinitionForType(typeName *types.TypeName, pkgDoc *pkgdoc.PkgDoc) (def SpxDefinition) {
	if !xgoutil.IsInMainPkg(typeName) {
		if defIface, ok := nonMainPkgSpxDefCacheForTypes.Load(typeName); ok {
			return defIface.(SpxDefinition)
		}
		defer func() {
			nonMainPkgSpxDefCacheForTypes.Store(typeName, def)
		}()
	}

	var overview strings.Builder
	overview.WriteString("type ")
	overview.WriteString(typeName.Name())

	var detail string
	if pkgDoc != nil {
		typeDoc, ok := pkgDoc.Types[typeName.Name()]
		if ok {
			detail = typeDoc.Doc
		}
	}

	completionKind := ClassCompletion
	if named, ok := typeName.Type().(*types.Named); ok {
		switch named.Underlying().(type) {
		case *types.Interface:
			completionKind = InterfaceCompletion
		case *types.Struct:
			completionKind = StructCompletion
		}
	}

	def = SpxDefinition{
		TypeHint: typeName.Type(),

		ID: SpxDefinitionIdentifier{
			Package: ToPtr(xgoutil.PkgPath(typeName.Pkg())),
			Name:    ToPtr(typeName.Name()),
		},
		Overview: overview.String(),
		Detail:   detail,

		CompletionItemLabel:            typeName.Name(),
		CompletionItemKind:             completionKind,
		CompletionItemInsertText:       typeName.Name(),
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}
	return
}

// nonMainPkgSpxDefCacheForFuncs is a cache of non-main package spx definitions
// for functions.
var nonMainPkgSpxDefCacheForFuncs sync.Map // map[nonMainPkgSpxDefCacheForFuncsKey]SpxDefinition

// nonMainPkgSpxDefCacheForFuncsKey is the key for the non-main package spx
// definition cache for functions.
type nonMainPkgSpxDefCacheForFuncsKey struct {
	fun          *types.Func
	recvTypeName string
}

// GetSpxDefinitionForFunc returns the spx definition for the provided function.
func GetSpxDefinitionForFunc(fun *types.Func, recvTypeName string, pkgDoc *pkgdoc.PkgDoc) (def SpxDefinition) {
	if !xgoutil.IsInMainPkg(fun) {
		cacheKey := nonMainPkgSpxDefCacheForFuncsKey{
			fun:          fun,
			recvTypeName: recvTypeName,
		}
		if defIface, ok := nonMainPkgSpxDefCacheForFuncs.Load(cacheKey); ok {
			return defIface.(SpxDefinition)
		}
		defer func() {
			nonMainPkgSpxDefCacheForFuncs.Store(cacheKey, def)
		}()
	}

	if IsInSpxPkg(fun) && recvTypeName == "Sprite" {
		recvTypeName = "SpriteImpl"
	}

	overview, parsedRecvTypeName, parsedName, overloadID := makeSpxDefinitionOverviewForFunc(fun)
	if recvTypeName == "" {
		recvTypeName = parsedRecvTypeName
	}

	var detail string
	if pkgDoc != nil {
		funcName := fun.Name()
		if recvTypeName == "" || xgoutil.IsXGotMethodName(funcName) {
			detail = pkgDoc.Funcs[funcName]
		} else if typeDoc, ok := pkgDoc.Types[recvTypeName]; ok {
			detail = typeDoc.Methods[funcName]
		}
	}

	idName := parsedName
	if recvTypeName != "" {
		recvTypeDisplayName := recvTypeName
		if IsInSpxPkg(fun) && recvTypeDisplayName == "SpriteImpl" {
			recvTypeDisplayName = "Sprite"
		}
		idName = recvTypeDisplayName + "." + idName
	}
	def = SpxDefinition{
		TypeHint: fun.Type(),

		ID: SpxDefinitionIdentifier{
			Package:    ToPtr(xgoutil.PkgPath(fun.Pkg())),
			Name:       &idName,
			OverloadID: overloadID,
		},
		Overview: overview,
		Detail:   detail,

		CompletionItemLabel:            parsedName,
		CompletionItemKind:             FunctionCompletion,
		CompletionItemInsertText:       parsedName,
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}
	return
}

// makeSpxDefinitionOverviewForFunc makes an overview string for a function that
// is used in [SpxDefinition].
func makeSpxDefinitionOverviewForFunc(fun *types.Func) (overview, parsedRecvTypeName, parsedName string, overloadID *string) {
	isXGoPkg := xgoutil.IsMarkedAsXGoPackage(fun.Pkg())
	name := fun.Name()
	sig := fun.Type().(*types.Signature)

	var sb strings.Builder
	sb.WriteString("func ")

	var isXGotMethod bool
	if recv := sig.Recv(); recv != nil {
		recvType := xgoutil.DerefType(recv.Type())
		if named, ok := recvType.(*types.Named); ok {
			parsedRecvTypeName = named.Obj().Name()
		}
	} else if isXGoPkg {
		switch {
		case strings.HasPrefix(name, xgoutil.XGotPrefix):
			recvTypeName, methodName, ok := xgoutil.SplitXGotMethodName(name, true)
			if !ok {
				break
			}
			parsedRecvTypeName = recvTypeName
			name = methodName
			isXGotMethod = true
		}
	}

	parsedName = name
	if isXGoPkg {
		parsedName, overloadID = xgoutil.ParseXGoFuncName(parsedName)
	} else if !xgoutil.IsInMainPkg(fun) {
		parsedName = xgoutil.ToLowerCamelCase(parsedName)
	}
	sb.WriteString(parsedName)
	sb.WriteString("(")
	params := make([]string, 0, sig.TypeParams().Len()+sig.Params().Len())
	for typeParam := range sig.TypeParams().TypeParams() {
		params = append(params, typeParam.Obj().Name()+" Type")
	}
	for i := range sig.Params().Len() {
		if isXGotMethod && i == 0 {
			continue
		}
		param := sig.Params().At(i)
		paramType := param.Type()
		paramTypeName := GetSimplifiedTypeString(paramType)

		// Check if this is a variadic parameter.
		if sig.Variadic() && i == sig.Params().Len()-1 {
			if slice, ok := paramType.(*types.Slice); ok {
				elemTypeName := GetSimplifiedTypeString(slice.Elem())
				paramTypeName = "..." + elemTypeName
			}
		}

		params = append(params, param.Name()+" "+paramTypeName)
	}
	sb.WriteString(strings.Join(params, ", "))
	sb.WriteString(")")

	if results := sig.Results(); results.Len() > 0 {
		if results.Len() == 1 {
			sb.WriteString(" ")
			sb.WriteString(GetSimplifiedTypeString(results.At(0).Type()))
		} else {
			sb.WriteString(" (")
			for i := range results.Len() {
				if i > 0 {
					sb.WriteString(", ")
				}
				result := results.At(i)
				if name := result.Name(); name != "" {
					sb.WriteString(name)
					sb.WriteString(" ")
				}
				sb.WriteString(GetSimplifiedTypeString(result.Type()))
			}
			sb.WriteString(")")
		}
	}

	overview = sb.String()
	return
}

// nonMainPkgSpxDefCacheForPkgs is a cache of non-main package spx definitions
// for packages.
var nonMainPkgSpxDefCacheForPkgs sync.Map // map[*types.PkgName]SpxDefinition

// GetSpxDefinitionForPkg returns the spx definition for the provided package.
func GetSpxDefinitionForPkg(pkgName *types.PkgName, pkgDoc *pkgdoc.PkgDoc) (def SpxDefinition) {
	if !xgoutil.IsInMainPkg(pkgName) {
		if defIface, ok := nonMainPkgSpxDefCacheForPkgs.Load(pkgName); ok {
			return defIface.(SpxDefinition)
		}
		defer func() {
			nonMainPkgSpxDefCacheForPkgs.Store(pkgName, def)
		}()
	}

	var detail string
	if pkgDoc != nil {
		detail = pkgDoc.Doc
	}

	def = SpxDefinition{
		TypeHint: pkgName.Type(),

		ID: SpxDefinitionIdentifier{
			Package: ToPtr(xgoutil.PkgPath(pkgName.Pkg())),
		},
		Overview: "package " + pkgName.Name(),
		Detail:   detail,

		CompletionItemLabel:            pkgName.Name(),
		CompletionItemKind:             ModuleCompletion,
		CompletionItemInsertText:       pkgName.Name(),
		CompletionItemInsertTextFormat: PlainTextTextFormat,
	}
	return
}
