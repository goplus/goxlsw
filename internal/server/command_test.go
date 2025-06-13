package server

import (
	"go/types"
	"reflect"
	"slices"
	"testing"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo/xgoutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerSpxGetDefinitions(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)
MySprite.turn Left
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
onStart => {
	MySprite.turn Right
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		mainSpxFileScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
					Position:     Position{Line: 0, Character: 0},
				},
			},
		}
		mainSpxFileScopeDefs, err := s.spxGetDefinitions(mainSpxFileScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mainSpxFileScopeDefs)
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr("builtin"),
			Name:    ToPtr("println"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr("main"),
			Name:    ToPtr("MySprite"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Game.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onClick"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.turn"),
			OverloadID: ToPtr("1"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onStart"),
		}))

		mySpriteSpxFileScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
					Position:     Position{Line: 0, Character: 0},
				},
			},
		}
		mySpriteSpxFileScopeDefs, err := s.spxGetDefinitions(mySpriteSpxFileScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mySpriteSpxFileScopeDefs)
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr("builtin"),
			Name:    ToPtr("println"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Game.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.turn"),
			OverloadID: ToPtr("1"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onStart"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onClick"),
		}))

		mySpriteSpxOnStartScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
					Position:     Position{Line: 2, Character: 0},
				},
			},
		}
		mySpriteSpxOnStartScopeDefs, err := s.spxGetDefinitions(mySpriteSpxOnStartScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mySpriteSpxOnStartScopeDefs)
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr("builtin"),
			Name:    ToPtr("println"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Game.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onClick"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.turn"),
			OverloadID: ToPtr("1"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onStart"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onClick"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.clone"),
			OverloadID: ToPtr("1"),
		}))
	})

	t.Run("ParseError", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
// Invalid syntax
var (
	MySprite Sprite
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		mainSpxFileScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
					Position:     Position{Line: 0, Character: 0},
				},
			},
		}
		mainSpxFileScopeDefs, err := s.spxGetDefinitions(mainSpxFileScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mainSpxFileScopeDefs)
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr("builtin"),
			Name:    ToPtr("println"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr("main"),
			Name:    ToPtr("Game.MySprite"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onStart"),
		}))
	})

	t.Run("TrailingEmptyLinesOfSpriteCode", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)
MySprite.turn Left
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
onStart => {
	MySprite.turn Right
}


`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		mySpriteSpxFileScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
					Position:     Position{Line: 5, Character: 0},
				},
			},
		}
		mySpriteSpxFileScopeDefs, err := s.spxGetDefinitions(mySpriteSpxFileScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mySpriteSpxFileScopeDefs)
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Game.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.False(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.play"),
			OverloadID: ToPtr("1"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onStart"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(mySpriteSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onClick"),
		}))
	})

	t.Run("EOF", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		mainSpxOnStartScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
					Position:     Position{Line: 1, Character: 13},
				},
			},
		}
		mainSpxOnStartScopeDefs, err := s.spxGetDefinitions(mainSpxOnStartScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mainSpxOnStartScopeDefs)
		assert.False(t, spxDefinitionIdentifierSliceContains(mainSpxOnStartScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))

		mainSpxFileScopeParams := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
					Position:     Position{Line: 2, Character: 0},
				},
			},
		}
		mainSpxFileScopeDefs, err := s.spxGetDefinitions(mainSpxFileScopeParams)
		require.NoError(t, err)
		require.NotNil(t, mainSpxFileScopeDefs)
		assert.True(t, spxDefinitionIdentifierSliceContains(mainSpxFileScopeDefs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Game.onStart"),
		}))
	})

	// See https://github.com/goplus/builder/issues/1398.
	t.Run("Issue#1398", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
onTouchStart "" => {
	say "touched by someone"
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
					Position:     Position{Line: 1, Character: 15},
				},
			},
		}
		defs, err := s.spxGetDefinitions(params)
		require.NoError(t, err)
		require.NotNil(t, defs)
		assert.False(t, spxDefinitionIdentifierSliceContains(defs, SpxDefinitionIdentifier{
			Package: ToPtr(GetSpxPkg().Path()),
			Name:    ToPtr("Sprite.onTouchStart"),
		}))
		assert.True(t, spxDefinitionIdentifierSliceContains(defs, SpxDefinitionIdentifier{
			Package:    ToPtr(GetSpxPkg().Path()),
			Name:       ToPtr("Sprite.say"),
			OverloadID: ToPtr("0"),
		}))
	})

	t.Run("MathPackage", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {

}
`),
			"assets/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetDefinitionsParams{
			{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
					Position:     Position{Line: 2, Character: 1},
				},
			},
		}
		defs, err := s.spxGetDefinitions(params)
		require.NoError(t, err)
		require.NotNil(t, defs)
		assert.True(t, spxDefinitionIdentifierSliceContains(defs, SpxDefinitionIdentifier{
			Package: ToPtr(GetMathPkg().Path()),
			Name:    ToPtr("abs"),
		}))
	})
}

// spxDefinitionIdentifierSliceContains reports whether a slice of [SpxDefinitionIdentifier]
// contains a specific [SpxDefinitionIdentifier].
func spxDefinitionIdentifierSliceContains(defs []SpxDefinitionIdentifier, def SpxDefinitionIdentifier) bool {
	return slices.ContainsFunc(defs, func(d SpxDefinitionIdentifier) bool {
		return FromPtr(d.Package) == FromPtr(def.Package) &&
			FromPtr(d.Name) == FromPtr(def.Name) &&
			FromPtr(d.OverloadID) == FromPtr(def.OverloadID)
	})
}

func TestServerSpxGetInputSlots(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Literals of different types.
	count := 5
	message := "Hello"
	isVisible := true
	direction := Left

	// Function calls with different types.
	println 42, 3.14, "text"
	myColor := HSB(255, 0, 0)
	otherColor := HSBA(0, 255, 0, 128)

	// Conditions and calculations.
	if count > 3 && isVisible {
		println "Count is greater than 3 and is visible"
	}

	// Spx resource name.
	MySprite.goto "OtherSprite"
}
`),
			"MySprite.spx":                          []byte(``),
			"OtherSprite.spx":                       []byte(``),
			"assets/index.json":                     []byte(`{}`),
			"assets/sprites/MySprite/index.json":    []byte(`{}`),
			"assets/sprites/OtherSprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetInputSlotsParams{{TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"}}}
		inputSlots, err := s.spxGetInputSlots(params)
		require.NoError(t, err)
		require.NotNil(t, inputSlots)
		assert.Greater(t, len(inputSlots), 10)

		t.Run("InPlaceValues", func(t *testing.T) {
			for _, tt := range []struct {
				name        string
				value       any
				acceptType  SpxInputType
				inputType   SpxInputType
				inputKind   SpxInputKind
				shouldExist bool
			}{
				{
					name:        "Integer",
					value:       int64(5),
					acceptType:  SpxInputTypeInteger,
					inputType:   SpxInputTypeInteger,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name:        "String",
					value:       "Hello",
					acceptType:  SpxInputTypeString,
					inputType:   SpxInputTypeString,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name:        "Direction",
					value:       float64(-90),
					acceptType:  SpxInputTypeDecimal,
					inputType:   SpxInputTypeDirection,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name:        "Boolean",
					value:       true,
					acceptType:  SpxInputTypeBoolean,
					inputType:   SpxInputTypeBoolean,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name: "HSB",
					value: SpxColorInputValue{
						Constructor: SpxInputTypeSpxColorConstructorHSB,
						Args:        []float64{255, 0, 0},
					},
					acceptType:  SpxInputTypeColor,
					inputType:   SpxInputTypeColor,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name: "HSBA",
					value: SpxColorInputValue{
						Constructor: SpxInputTypeSpxColorConstructorHSBA,
						Args:        []float64{0, 255, 0, 128},
					},
					acceptType:  SpxInputTypeColor,
					inputType:   SpxInputTypeColor,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name:        "SpxResourceName",
					value:       SpxResourceURI("spx://resources/sprites/OtherSprite"),
					acceptType:  SpxInputTypeResourceName,
					inputType:   SpxInputTypeResourceName,
					inputKind:   SpxInputKindInPlace,
					shouldExist: true,
				},
				{
					name:        "NonExistentValue",
					value:       int64(999),
					acceptType:  SpxInputTypeInteger,
					inputType:   SpxInputTypeInteger,
					inputKind:   SpxInputKindInPlace,
					shouldExist: false,
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					slot := findInputSlot(inputSlots, tt.value, "", tt.inputType, tt.inputKind)
					if tt.shouldExist {
						require.NotNil(t, slot)
						assert.Equal(t, SpxInputSlotKindValue, slot.Kind)
						assert.Equal(t, tt.acceptType, slot.Accept.Type)
						assert.Equal(t, tt.inputKind, slot.Input.Kind)
						assert.Equal(t, tt.inputType, slot.Input.Type)
						assert.Equal(t, tt.value, slot.Input.Value)
						assert.NotEmpty(t, slot.PredefinedNames)
						assert.NotEmpty(t, slot.Range)
					} else {
						assert.Nil(t, slot)
					}
				})
			}
		})

		t.Run("PredefinedValues", func(t *testing.T) {
			for _, tt := range []struct {
				name        string
				inputName   string
				inputType   SpxInputType
				inputKind   SpxInputKind
				shouldExist bool
			}{
				{"Variable", "count", SpxInputTypeUnknown, SpxInputKindPredefined, true},
				{"NonExistentName", "nonExistent", SpxInputTypeUnknown, SpxInputKindPredefined, false},
			} {
				t.Run(tt.name, func(t *testing.T) {
					slot := findInputSlot(inputSlots, nil, tt.inputName, tt.inputType, tt.inputKind)
					if tt.shouldExist {
						require.NotNil(t, slot)
						assert.Equal(t, tt.inputType, slot.Accept.Type)
						assert.Equal(t, tt.inputKind, slot.Input.Kind)
						assert.Equal(t, tt.inputType, slot.Input.Type)
						assert.Equal(t, tt.inputName, slot.Input.Name)
						assert.NotEmpty(t, slot.PredefinedNames)
						assert.NotEmpty(t, slot.Range)
					} else {
						assert.Nil(t, slot)
					}
				})
			}
		})

		t.Run("AddressSlots", func(t *testing.T) {
			for _, tt := range []struct {
				name        string
				inputName   string
				shouldExist bool
			}{
				{"CountVariable", "count", true},
				{"NonExistentVariable", "nonExistent", false},
			} {
				t.Run(tt.name, func(t *testing.T) {
					slot := findAddressInputSlot(inputSlots, tt.inputName)
					if tt.shouldExist {
						require.NotNil(t, slot)
						assert.Equal(t, SpxInputSlotKindAddress, slot.Kind)
						assert.Equal(t, SpxInputTypeUnknown, slot.Accept.Type)
						assert.Equal(t, SpxInputKindPredefined, slot.Input.Kind)
						assert.Equal(t, SpxInputTypeUnknown, slot.Input.Type)
						assert.Equal(t, tt.inputName, slot.Input.Name)
						assert.NotEmpty(t, slot.PredefinedNames)
						assert.NotEmpty(t, slot.Range)
					} else {
						assert.Nil(t, slot)
					}
				})
			}
		})
	})

	t.Run("InvalidSyntax", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
// Missing closing parenthesis.
var (
	count     int
	message   string
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetInputSlotsParams{{TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"}}}
		inputSlots, err := s.spxGetInputSlots(params)
		require.NoError(t, err)
		assert.Nil(t, inputSlots)
	})

	t.Run("EmptyFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(``),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetInputSlotsParams{{TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"}}}
		inputSlots, err := s.spxGetInputSlots(params)
		require.NoError(t, err)
		assert.Empty(t, inputSlots)
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		m := map[string][]byte{}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetInputSlotsParams{{TextDocument: TextDocumentIdentifier{URI: "file:///nonexistent.spx"}}}
		inputSlots, err := s.spxGetInputSlots(params)
		require.Error(t, err)
		assert.Nil(t, inputSlots)
	})

	t.Run("MultipleParams", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`var a = 1`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetInputSlotsParams{
			{TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"}},
			{TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"}},
		}
		inputSlots, err := s.spxGetInputSlots(params)
		require.Error(t, err)
		assert.Nil(t, inputSlots)
		assert.ErrorContains(t, err, "only supports one document")
	})

	t.Run("EmptyParams", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`var a = 1`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

		params := []SpxGetInputSlotsParams{}
		inputSlots, err := s.spxGetInputSlots(params)
		require.NoError(t, err)
		assert.Nil(t, inputSlots)
	})
}

func TestFindInputSlots(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Initialize variables
	count := 5
	message := "Hello"
	isVisible := true
	direction := Left

	// CallExpr with various arg types
	println 42, 3.14, "text", true, Left, LeftRight

	// BinaryExpr
	sum := 10 + 20
	isEqual := count == 5

	// UnaryExpr
	notTrue := !isVisible

	// AssignStmt
	count = 10
	myColor := HSB(255, 0, 0)

	// IfStmt
	if count > 3 {
		println "Greater than 3"
	}

	// ForStmt
	for i := 0; i < 5; i++ {
		println i
	}

	// ReturnStmt in a function
	calculateValue := func() int {
		return 100
	}

	// SwitchStmt and CaseClause
	switch direction {
	case Left:
		println "Going left"
	case Right:
		println "Going right"
	default:
		println "Other direction"
	}

	// RangeStmt
	numbers := []int{1, 2, 3}
	for index, value := range numbers {
		println index, value
	}

	// IncDecStmt
	count++

	// Spx resource name
	MySprite.goto "OtherSprite"

	// Other commands
	MySprite.turn MySprite.heading
	getWidget Monitor, "myWidget"
}
`),
		"MySprite.spx": []byte(`
onStart => {
	name := "OtherSprite"
	goto name
	data := "data"
	clone data
}
`),
		"OtherSprite.spx":                       []byte(``),
		"assets/index.json":                     []byte(`{"zorder":[{"name":"myWidget"}]}`),
		"assets/sprites/MySprite/index.json":    []byte(`{}`),
		"assets/sprites/OtherSprite/index.json": []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	inputSlots := findInputSlots(result, astFile)
	require.NotNil(t, inputSlots)
	assert.NotEmpty(t, inputSlots)

	t.Run("ValueSlots", func(t *testing.T) {
		for _, tt := range []struct {
			name           string
			value          any
			wantAcceptType SpxInputType
			wantInputType  SpxInputType
		}{
			{
				name:           "Integer",
				value:          int64(42),
				wantAcceptType: SpxInputTypeUnknown,
				wantInputType:  SpxInputTypeInteger,
			},
			{
				name:           "Decimal",
				value:          3.14,
				wantAcceptType: SpxInputTypeUnknown,
				wantInputType:  SpxInputTypeDecimal,
			},
			{
				name:           "String",
				value:          "text",
				wantAcceptType: SpxInputTypeUnknown,
				wantInputType:  SpxInputTypeString,
			},
			{
				name:           "RotationStyle",
				value:          "LeftRight",
				wantAcceptType: SpxInputTypeUnknown,
				wantInputType:  SpxInputTypeRotationStyle,
			},
			{
				name:           "Direction",
				value:          float64(-90),
				wantAcceptType: SpxInputTypeDecimal,
				wantInputType:  SpxInputTypeDirection,
			},
			{
				name:           "SpxResourceName",
				value:          SpxResourceURI("spx://resources/sprites/OtherSprite"),
				wantAcceptType: SpxInputTypeResourceName,
				wantInputType:  SpxInputTypeResourceName,
			},
			{
				name:           "Boolean",
				value:          true,
				wantAcceptType: SpxInputTypeBoolean,
				wantInputType:  SpxInputTypeBoolean,
			},
			{
				name:           "BinaryExprResult",
				value:          int64(10),
				wantAcceptType: SpxInputTypeInteger,
				wantInputType:  SpxInputTypeInteger,
			},
			{
				name: "ColorHSB",
				value: SpxColorInputValue{
					Constructor: SpxInputTypeSpxColorConstructorHSB,
					Args:        []float64{255, 0, 0},
				},
				wantAcceptType: SpxInputTypeColor,
				wantInputType:  SpxInputTypeColor,
			},
			{
				name:           "WidgetName",
				value:          SpxResourceURI("spx://resources/widgets/myWidget"),
				wantAcceptType: SpxInputTypeResourceName,
				wantInputType:  SpxInputTypeResourceName,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				slot := findInputSlot(inputSlots, tt.value, "", tt.wantInputType, SpxInputKindInPlace)
				require.NotNil(t, slot)
				assert.Equal(t, SpxInputSlotKindValue, slot.Kind)
				assert.Equal(t, tt.wantAcceptType, slot.Accept.Type)
				assert.Equal(t, SpxInputKindInPlace, slot.Input.Kind)
				assert.Equal(t, tt.wantInputType, slot.Input.Type)
				assert.Equal(t, tt.value, slot.Input.Value)
				assert.NotEmpty(t, slot.PredefinedNames)
				assert.NotEmpty(t, slot.Range)
			})
		}
	})

	t.Run("AddressSlots", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			wantInputName string
		}{
			{name: "AssignmentTarget", wantInputName: "count"},
			{name: "RangeIndex", wantInputName: "index"},
			{name: "RangeValue", wantInputName: "value"},
			{name: "IncDecTarget", wantInputName: "count"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				slot := findAddressInputSlot(inputSlots, tt.wantInputName)
				require.NotNil(t, slot)
				assert.Equal(t, SpxInputSlotKindAddress, slot.Kind)
				assert.Equal(t, SpxInputTypeUnknown, slot.Accept.Type)
				assert.Equal(t, SpxInputKindPredefined, slot.Input.Kind)
				assert.Equal(t, SpxInputTypeUnknown, slot.Input.Type)
				assert.Equal(t, tt.wantInputName, slot.Input.Name)
				assert.NotEmpty(t, slot.PredefinedNames)
				assert.NotEmpty(t, slot.Range)
			})
		}
	})

	t.Run("PredefinedNameSlots", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			wantInputName string
		}{
			{name: "Variable", wantInputName: "count"},
			{name: "MessageVar", wantInputName: "message"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				slot := findInputSlot(inputSlots, nil, tt.wantInputName, SpxInputTypeUnknown, SpxInputKindPredefined)
				require.NotNil(t, slot)
				assert.Equal(t, SpxInputTypeUnknown, slot.Accept.Type)
				assert.Equal(t, SpxInputKindPredefined, slot.Input.Kind)
				assert.Equal(t, SpxInputTypeUnknown, slot.Input.Type)
				assert.Equal(t, tt.wantInputName, slot.Input.Name)
				assert.Contains(t, slot.PredefinedNames, "backdropName")
				assert.NotEmpty(t, slot.Range)
			})
		}
	})

	t.Run("spx.Sprite.Goto", func(t *testing.T) {
		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///MySprite.spx")
		require.NoError(t, err)
		require.False(t, result.hasErrorSeverityDiagnostic)
		require.NotNil(t, astFile)

		inputSlots := findInputSlots(result, astFile)
		require.NotNil(t, inputSlots)
		assert.NotEmpty(t, inputSlots)

		slot := findInputSlot(inputSlots, nil, "name", SpxInputTypeString, SpxInputKindPredefined)
		require.NotNil(t, slot)
		assert.Equal(t, SpxInputTypeResourceName, slot.Accept.Type)
		assert.Equal(t, SpxInputKindPredefined, slot.Input.Kind)
		assert.Equal(t, SpxInputTypeString, slot.Input.Type)
		assert.Equal(t, "name", slot.Input.Name)
		assert.Contains(t, slot.PredefinedNames, "backdropName")
		assert.Equal(t, slot.Range, Range{
			Start: Position{Line: 3, Character: 6},
			End:   Position{Line: 3, Character: 10},
		})
	})

	t.Run("spx.Sprite.Clone", func(t *testing.T) {
		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///MySprite.spx")
		require.NoError(t, err)
		require.False(t, result.hasErrorSeverityDiagnostic)
		require.NotNil(t, astFile)

		inputSlots := findInputSlots(result, astFile)
		require.NotNil(t, inputSlots)
		assert.NotEmpty(t, inputSlots)

		slot := findInputSlot(inputSlots, nil, "data", SpxInputTypeString, SpxInputKindPredefined)
		require.NotNil(t, slot)
		assert.Equal(t, SpxInputTypeUnknown, slot.Accept.Type)
		assert.Equal(t, SpxInputKindPredefined, slot.Input.Kind)
		assert.Equal(t, SpxInputTypeString, slot.Input.Type)
		assert.Equal(t, "data", slot.Input.Name)
		assert.Contains(t, slot.PredefinedNames, "backdropName")
		assert.Equal(t, slot.Range, Range{
			Start: Position{Line: 5, Character: 7},
			End:   Position{Line: 5, Character: 11},
		})
	})
}

func TestCheckValueInputSlot(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
onStart => {
	// Basic literals.
	numValue := 42
	floatValue := 3.14
	strValue := "hello"

	// Identifiers.
	dirValue := Left
	boolValue := true

	// Color function calls.
	colorValue := HSB(255, 0, 0)

	// Other expressions.
	arrayValue := []int{1, 2, 3}
}
`),
		"assets/index.json": []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	for _, tt := range []struct {
		name           string
		exprPosition   Position
		exprFilter     func(xgoast.Node) bool
		wantNil        bool
		wantKind       SpxInputSlotKind
		wantAcceptType SpxInputType
		wantInputKind  SpxInputKind
		wantInputType  SpxInputType
		wantInputValue any
		wantInputName  string
	}{
		{
			name:           "IntegerLiteral",
			exprPosition:   Position{Line: 3, Character: 14},
			exprFilter:     func(node xgoast.Node) bool { _, ok := node.(*xgoast.BasicLit); return ok },
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeInteger,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeInteger,
			wantInputValue: int64(42),
		},
		{
			name:           "FloatLiteral",
			exprPosition:   Position{Line: 4, Character: 16},
			exprFilter:     func(node xgoast.Node) bool { _, ok := node.(*xgoast.BasicLit); return ok },
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeDecimal,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeDecimal,
			wantInputValue: 3.14,
		},
		{
			name:           "StringLiteral",
			exprPosition:   Position{Line: 5, Character: 14},
			exprFilter:     func(node xgoast.Node) bool { _, ok := node.(*xgoast.BasicLit); return ok },
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeString,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeString,
			wantInputValue: "hello",
		},
		{
			name:           "DirectionIdentifier",
			exprPosition:   Position{Line: 8, Character: 14},
			exprFilter:     func(node xgoast.Node) bool { _, ok := node.(*xgoast.Ident); return ok },
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeDirection,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeDirection,
			wantInputValue: float64(-90),
		},
		{
			name:           "BooleanIdentifier",
			exprPosition:   Position{Line: 9, Character: 15},
			exprFilter:     func(node xgoast.Node) bool { _, ok := node.(*xgoast.Ident); return ok },
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeBoolean,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeBoolean,
			wantInputValue: true,
		},
		{
			name:           "ColorFunctionCall",
			exprPosition:   Position{Line: 12, Character: 16},
			exprFilter:     func(node xgoast.Node) bool { _, ok := node.(*xgoast.CallExpr); return ok },
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeColor,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeColor,
			wantInputValue: SpxColorInputValue{
				Constructor: SpxInputTypeSpxColorConstructorHSB,
				Args:        []float64{255, 0, 0},
			},
		},
		{
			name:         "NonValueNode",
			exprPosition: Position{Line: 15, Character: 16},
			exprFilter:   func(node xgoast.Node) bool { _, ok := node.(*xgoast.CompositeLit); return ok },
			wantNil:      true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos := PosAt(result.proj, astFile, tt.exprPosition)
			require.True(t, pos.IsValid())

			var expr xgoast.Expr
			xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
				if node, ok := node.(xgoast.Expr); ok && tt.exprFilter(node) {
					expr = node
					return false
				}
				return true
			})
			require.NotNil(t, expr)

			got := checkValueInputSlot(result, expr, nil)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantKind, got.Kind)
				assert.Equal(t, tt.wantAcceptType, got.Accept.Type)
				assert.Equal(t, tt.wantInputKind, got.Input.Kind)
				assert.Equal(t, tt.wantInputType, got.Input.Type)
				assert.Equal(t, tt.wantInputValue, got.Input.Value)
				assert.Equal(t, tt.wantInputName, got.Input.Name)
				assert.NotEmpty(t, got.Range)
			}
		})
	}
}

func TestCheckAddressInputSlot(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
var (
	varA int
)

onStart => {
	varA = 10
	println varA
	otherVar := 20
}
`),
		"assets/index.json": []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	for _, tt := range []struct {
		name         string
		exprPosition Position
		exprFilter   func(xgoast.Node) bool
		wantNil      bool
		wantName     string
	}{
		{
			name:         "ExistingIdentifier",
			exprPosition: Position{Line: 6, Character: 2},
			exprFilter:   func(node xgoast.Node) bool { _, ok := node.(*xgoast.Ident); return ok },
			wantName:     "varA",
		},
		{
			name:         "CallExpr",
			exprPosition: Position{Line: 7, Character: 2},
			exprFilter:   func(node xgoast.Node) bool { _, ok := node.(*xgoast.CallExpr); return ok },
			wantNil:      true,
		},
		{
			name:         "BasicLit",
			exprPosition: Position{Line: 8, Character: 14},
			exprFilter:   func(node xgoast.Node) bool { _, ok := node.(*xgoast.BasicLit); return ok },
			wantNil:      true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos := PosAt(result.proj, astFile, tt.exprPosition)
			require.True(t, pos.IsValid())

			var expr xgoast.Expr
			xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
				if node, ok := node.(xgoast.Expr); ok && tt.exprFilter(node) {
					expr = node
					return false
				}
				return true
			})
			require.NotNil(t, expr)

			got := checkAddressInputSlot(result, expr, nil)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, SpxInputSlotKindAddress, got.Kind)
				assert.Equal(t, SpxInputTypeUnknown, got.Accept.Type)
				assert.Equal(t, SpxInputKindPredefined, got.Input.Kind)
				assert.Equal(t, SpxInputTypeUnknown, got.Input.Type)
				assert.Equal(t, tt.wantName, got.Input.Name)
				assert.NotEmpty(t, got.Range)
			}
		})
	}
}

func TestCreateValueInputSlotFromBasicLit(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Integer literals.
	x := 42
	hexValue := 0xFF

	// Float literals.
	y := 3.14
	scientific := 1.5e2

	// String literals.
	message := "Hello, world!"
	MySprite.goto "OtherSprite"
}
`),
		"MySprite.spx":                          []byte(``),
		"OtherSprite.spx":                       []byte(``),
		"assets/index.json":                     []byte(`{}`),
		"assets/sprites/MySprite/index.json":    []byte(`{}`),
		"assets/sprites/OtherSprite/index.json": []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	for _, tt := range []struct {
		name           string
		litPosition    Position
		declaredType   types.Type
		wantAcceptType SpxInputType
		wantInputType  SpxInputType
		wantInputKind  SpxInputKind
		wantValue      any
	}{
		{
			name:           "Integer",
			litPosition:    Position{Line: 7, Character: 7},
			wantAcceptType: SpxInputTypeInteger,
			wantInputType:  SpxInputTypeInteger,
			wantInputKind:  SpxInputKindInPlace,
			wantValue:      int64(42),
		},
		{
			name:           "HexInteger",
			litPosition:    Position{Line: 8, Character: 14},
			wantAcceptType: SpxInputTypeInteger,
			wantInputType:  SpxInputTypeInteger,
			wantInputKind:  SpxInputKindInPlace,
			wantValue:      int64(255), // 0xFF = 255
		},
		{
			name:           "Float",
			litPosition:    Position{Line: 11, Character: 7},
			wantAcceptType: SpxInputTypeDecimal,
			wantInputType:  SpxInputTypeDecimal,
			wantInputKind:  SpxInputKindInPlace,
			wantValue:      3.14,
		},
		{
			name:           "ScientificFloat",
			litPosition:    Position{Line: 12, Character: 16},
			wantAcceptType: SpxInputTypeDecimal,
			wantInputType:  SpxInputTypeDecimal,
			wantInputKind:  SpxInputKindInPlace,
			wantValue:      150.0, // 1.5e2 = 150
		},
		{
			name:           "String",
			litPosition:    Position{Line: 15, Character: 13},
			wantAcceptType: SpxInputTypeString,
			wantInputType:  SpxInputTypeString,
			wantInputKind:  SpxInputKindInPlace,
			wantValue:      "Hello, world!",
		},
		{
			name:           "SpxResourceString",
			litPosition:    Position{Line: 16, Character: 16},
			declaredType:   GetSpxSpriteNameType(),
			wantAcceptType: SpxInputTypeResourceName,
			wantInputType:  SpxInputTypeResourceName,
			wantInputKind:  SpxInputKindInPlace,
			wantValue:      SpxResourceURI("spx://resources/sprites/OtherSprite"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos := PosAt(result.proj, astFile, tt.litPosition)
			require.True(t, pos.IsValid())

			var lit *xgoast.BasicLit
			xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
				if node, ok := node.(*xgoast.BasicLit); ok {
					lit = node
					return false
				}
				return true
			})
			require.NotNil(t, lit)

			got := createValueInputSlotFromBasicLit(result, lit, tt.declaredType)
			require.NotNil(t, got)
			assert.Equal(t, SpxInputSlotKindValue, got.Kind)
			assert.Equal(t, tt.wantAcceptType, got.Accept.Type)
			assert.Equal(t, tt.wantInputKind, got.Input.Kind)
			assert.Equal(t, tt.wantInputType, got.Input.Type)
			assert.Equal(t, tt.wantValue, got.Input.Value)
			assert.NotEmpty(t, got.Range)
		})
	}

	t.Run("InvalidIntLiteral", func(t *testing.T) {
		invalidIntLit := &xgoast.BasicLit{
			Kind:  xgotoken.INT,
			Value: "not.a.int",
		}
		got := createValueInputSlotFromBasicLit(result, invalidIntLit, nil)
		assert.Nil(t, got)
	})

	t.Run("InvalidFloatLiteral", func(t *testing.T) {
		invalidFloatLit := &xgoast.BasicLit{
			Kind:  xgotoken.FLOAT,
			Value: "not.a.float",
		}
		got := createValueInputSlotFromBasicLit(result, invalidFloatLit, nil)
		assert.Nil(t, got)
	})

	t.Run("UnsupportedLiteralKind", func(t *testing.T) {
		unsupportedLit := &xgoast.BasicLit{
			Kind:  xgotoken.CHAR,
			Value: "'c'",
		}
		got := createValueInputSlotFromBasicLit(result, unsupportedLit, nil)
		assert.Nil(t, got)
	})

	t.Run("InvalidStringLiteral", func(t *testing.T) {
		invalidStringLit := &xgoast.BasicLit{
			Kind:  xgotoken.STRING,
			Value: "\"unclosed string literal", // Missing ending quote.
		}
		got := createValueInputSlotFromBasicLit(result, invalidStringLit, nil)
		assert.Nil(t, got)
	})
}

func TestCreateValueInputSlotFromIdent(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
var (
	regularVar int
	MySprite   Sprite
)

onStart => {
	// Boolean
	boolVar := true

	// Direction
	MySprite.turn Left

	// Special object
	if MySprite.touching(Mouse) {}

	// Special object (variable)
	myMouse := Mouse
	if MySprite.touching(myMouse) {}

	// Effect kind
	setEffect ColorEffect, 0

	// Play action
	play "MySound", &PlayOptions{Action: PlayStop}

	// Key
	if keyPressed(KeySpace) {}

	// Regular
	myVar := regularVar
}
`),
		"MySprite.spx":                       []byte(``),
		"assets/index.json":                  []byte(`{}`),
		"assets/sprites/MySprite/index.json": []byte(`{}`),
		"assets/sounds/MySound/index.json":   []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	for _, tt := range []struct {
		name           string
		identPosition  Position
		wantInputKind  SpxInputKind
		wantInputType  SpxInputType
		wantInputValue any
		wantInputName  string
		wantBoolValue  *bool
	}{
		{
			name:           "Boolean",
			identPosition:  Position{Line: 8, Character: 13},
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeBoolean,
			wantInputValue: true,
		},
		{
			name:           "Direction",
			identPosition:  Position{Line: 11, Character: 16},
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeDirection,
			wantInputValue: float64(-90),
		},
		{
			name:           "SpecialObject",
			identPosition:  Position{Line: 14, Character: 23},
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeSpecialObj,
			wantInputValue: "Mouse",
		},
		{
			name:          "SpecialObjectVariable",
			identPosition: Position{Line: 18, Character: 23},
			wantInputKind: SpxInputKindPredefined,
			wantInputType: SpxInputTypeSpecialObj,
			wantInputName: "myMouse",
		},
		{
			name:           "EffectKind",
			identPosition:  Position{Line: 21, Character: 12},
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeEffectKind,
			wantInputValue: "ColorEffect",
		},
		{
			name:           "PlayAction",
			identPosition:  Position{Line: 24, Character: 39},
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypePlayAction,
			wantInputValue: "PlayStop",
		},
		{
			name:           "Key",
			identPosition:  Position{Line: 27, Character: 16},
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeKey,
			wantInputValue: "KeySpace",
		},
		{
			name:          "Regular",
			identPosition: Position{Line: 30, Character: 11},
			wantInputKind: SpxInputKindPredefined,
			wantInputType: SpxInputTypeInteger,
			wantInputName: "regularVar",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos := PosAt(result.proj, astFile, tt.identPosition)
			require.True(t, pos.IsValid())

			var ident *xgoast.Ident
			xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
				if node, ok := node.(*xgoast.Ident); ok {
					ident = node
					return false
				}
				return true
			})
			require.NotNil(t, ident)

			got := createValueInputSlotFromIdent(result, ident, nil)
			require.NotNil(t, got)
			assert.Equal(t, SpxInputSlotKindValue, got.Kind)
			assert.Equal(t, tt.wantInputType, got.Accept.Type)
			assert.Equal(t, tt.wantInputKind, got.Input.Kind)
			assert.Equal(t, tt.wantInputType, got.Input.Type)
			assert.Equal(t, tt.wantInputValue, got.Input.Value)
			assert.Equal(t, tt.wantInputName, got.Input.Name)
			assert.NotEmpty(t, got.Range)
		})
	}
}

func TestCreateValueInputSlotFromUnaryExpr(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
onStart => {
	// Unary minus with integer.
	negInt := -42

	// Unary minus with float.
	negFloat := -3.14

	// Unary plus with integer.
	posInt := +10

	// Bitwise complement with integer.
	complementInt := ^0xFF

	// Logical not with boolean.
	notBool := !true
}
`),
		"assets/index.json": []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	for _, tt := range []struct {
		name           string
		exprPosition   Position
		wantKind       SpxInputSlotKind
		wantAcceptType SpxInputType
		wantInputKind  SpxInputKind
		wantInputType  SpxInputType
		wantInputValue any
	}{
		{
			name:           "UnaryMinusInteger",
			exprPosition:   Position{Line: 3, Character: 12},
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeInteger,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeInteger,
			wantInputValue: int64(-42),
		},
		{
			name:           "UnaryMinusFloat",
			exprPosition:   Position{Line: 6, Character: 14},
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeDecimal,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeDecimal,
			wantInputValue: -3.14,
		},
		{
			name:           "UnaryPlusInteger",
			exprPosition:   Position{Line: 9, Character: 12},
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeInteger,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeInteger,
			wantInputValue: int64(10),
		},
		{
			name:           "BitwiseComplement",
			exprPosition:   Position{Line: 12, Character: 19},
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeInteger,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeInteger,
			wantInputValue: int64(^0xFF), // ~255 = -256
		},
		{
			name:           "LogicalNot",
			exprPosition:   Position{Line: 15, Character: 13},
			wantKind:       SpxInputSlotKindValue,
			wantAcceptType: SpxInputTypeBoolean,
			wantInputKind:  SpxInputKindInPlace,
			wantInputType:  SpxInputTypeBoolean,
			wantInputValue: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos := PosAt(result.proj, astFile, tt.exprPosition)
			require.True(t, pos.IsValid())

			var unaryExpr *xgoast.UnaryExpr
			xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
				if expr, ok := node.(*xgoast.UnaryExpr); ok {
					unaryExpr = expr
					return false
				}
				return true
			})
			require.NotNil(t, unaryExpr)

			got := createValueInputSlotFromUnaryExpr(result, unaryExpr, nil)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantKind, got.Kind)
			assert.Equal(t, tt.wantAcceptType, got.Accept.Type)
			assert.Equal(t, tt.wantInputKind, got.Input.Kind)
			assert.Equal(t, tt.wantInputType, got.Input.Type)
			assert.Equal(t, tt.wantInputValue, got.Input.Value)
			assert.NotEmpty(t, got.Range)
		})
	}
}

func TestCreateValueInputSlotFromColorFuncCall(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
onStart => {
	// Color functions.
	myColor1 := HSB(255, 0, 0)
	myColor2 := HSBA(255, 0, 0, 128)

	// Non-color function calls.
	println 1, 2, 3
}
`),
		"assets/index.json": []byte(`{}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
	require.NoError(t, err)
	require.False(t, result.hasErrorSeverityDiagnostic)
	require.NotNil(t, astFile)

	for _, tt := range []struct {
		name             string
		callExprPosition Position
		wantNil          bool
		wantValue        SpxColorInputValue
	}{
		{
			name:             "HSB",
			callExprPosition: Position{Line: 3, Character: 14},
			wantValue: SpxColorInputValue{
				Constructor: SpxInputTypeSpxColorConstructorHSB,
				Args:        []float64{255, 0, 0},
			},
		},
		{
			name:             "HSBA",
			callExprPosition: Position{Line: 4, Character: 14},
			wantValue: SpxColorInputValue{
				Constructor: SpxInputTypeSpxColorConstructorHSBA,
				Args:        []float64{255, 0, 0, 128},
			},
		},
		{
			name:             "RegularFunction",
			callExprPosition: Position{Line: 7, Character: 2},
			wantNil:          true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos := PosAt(result.proj, astFile, tt.callExprPosition)
			require.True(t, pos.IsValid())

			var callExpr *xgoast.CallExpr
			xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
				if node, ok := node.(*xgoast.CallExpr); ok {
					callExpr = node
					return false
				}
				return true
			})
			require.NotNil(t, callExpr)

			got := createValueInputSlotFromColorFuncCall(result, callExpr, nil)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, SpxInputSlotKindValue, got.Kind)
				assert.Equal(t, SpxInputTypeColor, got.Accept.Type)
				assert.Equal(t, SpxInputKindInPlace, got.Input.Kind)
				assert.Equal(t, SpxInputTypeColor, got.Input.Type)

				colorValue, ok := got.Input.Value.(SpxColorInputValue)
				require.True(t, ok)
				assert.Equal(t, tt.wantValue.Constructor, colorValue.Constructor)
				assert.ElementsMatch(t, tt.wantValue.Args, colorValue.Args)

				assert.NotEmpty(t, got.Range)
			}
		})
	}

	t.Run("NonIdentifierFunction", func(t *testing.T) {
		callExpr := &xgoast.CallExpr{
			Fun: &xgoast.SelectorExpr{
				X:   &xgoast.Ident{Name: "math"},
				Sel: &xgoast.Ident{Name: "Max"},
			},
			Args: []xgoast.Expr{
				&xgoast.BasicLit{Kind: xgotoken.INT, Value: "1"},
				&xgoast.BasicLit{Kind: xgotoken.INT, Value: "2"},
			},
		}
		got := createValueInputSlotFromColorFuncCall(result, callExpr, nil)
		assert.Nil(t, got)
	})

	t.Run("NilFunctionType", func(t *testing.T) {
		callExpr := &xgoast.CallExpr{
			Fun:  &xgoast.Ident{Name: "unknownFunction"},
			Args: []xgoast.Expr{&xgoast.BasicLit{Kind: xgotoken.INT, Value: "1"}},
		}
		got := createValueInputSlotFromColorFuncCall(result, callExpr, nil)
		assert.Nil(t, got)
	})
}

func TestIsSpxColorFunc(t *testing.T) {
	for _, tt := range []struct {
		name string
		fun  *types.Func
		want bool
	}{
		{"HSB", GetSpxHSBFunc(), true},
		{"HSBA", GetSpxHSBAFunc(), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := isSpxColorFunc(tt.fun)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInferSpxInputTypeFromType(t *testing.T) {
	t.Run("BasicTypes", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			typ  types.Type
			want SpxInputType
		}{
			{"Bool", types.Typ[types.Bool], SpxInputTypeBoolean},
			{"UntypedBool", types.Typ[types.UntypedBool], SpxInputTypeBoolean},

			{"Int", types.Typ[types.Int], SpxInputTypeInteger},
			{"Int8", types.Typ[types.Int8], SpxInputTypeInteger},
			{"Int16", types.Typ[types.Int16], SpxInputTypeInteger},
			{"Int32", types.Typ[types.Int32], SpxInputTypeInteger},
			{"Int64", types.Typ[types.Int64], SpxInputTypeInteger},
			{"Uint", types.Typ[types.Uint], SpxInputTypeInteger},
			{"Uint8", types.Typ[types.Uint8], SpxInputTypeInteger},
			{"Uint16", types.Typ[types.Uint16], SpxInputTypeInteger},
			{"Uint32", types.Typ[types.Uint32], SpxInputTypeInteger},
			{"Uint64", types.Typ[types.Uint64], SpxInputTypeInteger},
			{"UntypedInt", types.Typ[types.UntypedInt], SpxInputTypeInteger},

			{"Float32", types.Typ[types.Float32], SpxInputTypeDecimal},
			{"Float64", types.Typ[types.Float64], SpxInputTypeDecimal},
			{"UntypedFloat", types.Typ[types.UntypedFloat], SpxInputTypeDecimal},

			{"String", types.Typ[types.String], SpxInputTypeString},
			{"UntypedString", types.Typ[types.UntypedString], SpxInputTypeString},

			{"Complex64", types.Typ[types.Complex64], SpxInputTypeUnknown},
			{"Complex128", types.Typ[types.Complex128], SpxInputTypeUnknown},
		} {
			t.Run(tt.name, func(t *testing.T) {
				got := inferSpxInputTypeFromType(tt.typ)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("NonBasicType", func(t *testing.T) {
		pkg := types.NewPackage("example.com/pkg", "pkg")
		structType := types.NewStruct([]*types.Var{}, []string{})
		namedType := types.NewNamed(types.NewTypeName(0, pkg, "MyStruct", nil), structType, nil)

		got := inferSpxInputTypeFromType(namedType)
		assert.Equal(t, SpxInputTypeUnknown, got)
	})

	t.Run("PointerType", func(t *testing.T) {
		pointerType := types.NewPointer(types.Typ[types.Int])

		got := inferSpxInputTypeFromType(pointerType)
		assert.Equal(t, SpxInputTypeUnknown, got)
	})

	t.Run("SpxAliasTypes", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			typeGetter func() *types.Alias
			want       SpxInputType
		}{
			{"BackdropName", GetSpxBackdropNameType, SpxInputTypeResourceName},
			{"SoundName", GetSpxSoundNameType, SpxInputTypeResourceName},
			{"SpriteName", GetSpxSpriteNameType, SpxInputTypeResourceName},
			{"SpriteCostumeName", GetSpxSpriteCostumeNameType, SpxInputTypeResourceName},
			{"SpriteAnimationName", GetSpxSpriteAnimationNameType, SpxInputTypeResourceName},
			{"WidgetName", GetSpxWidgetNameType, SpxInputTypeResourceName},
			{"SpecialDir", GetSpxDirectionType, SpxInputTypeDirection},
			{"Key", GetSpxKeyType, SpxInputTypeKey},
		} {
			t.Run(tt.name, func(t *testing.T) {
				got := inferSpxInputTypeFromType(tt.typeGetter())
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("SpxNamedTypes", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			typeGetter func() *types.Named
			want       SpxInputType
		}{
			{"EffectKind", GetSpxEffectKindType, SpxInputTypeEffectKind},
			{"PlayAction", GetSpxPlayActionType, SpxInputTypePlayAction},
			{"SpecialObj", GetSpxSpecialObjType, SpxInputTypeSpecialObj},
		} {
			t.Run(tt.name, func(t *testing.T) {
				got := inferSpxInputTypeFromType(tt.typeGetter())
				assert.Equal(t, tt.want, got)
			})
		}
	})
}

func TestInferSpxSpriteResourceEnclosingNode(t *testing.T) {
	m := map[string][]byte{
		"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	MySprite.setXYpos 10, 20
}
`),
		"MySprite.spx": []byte(`
onStart => {
	setCostume "costume1"
}
`),
		"assets/index.json":                  []byte(`{}`),
		"assets/sprites/MySprite/index.json": []byte(`{"costumes":[{"name":"costume1"}]}`),
	}
	s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m))

	t.Run("MainFile", func(t *testing.T) {
		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.False(t, result.hasErrorSeverityDiagnostic)
		require.NotNil(t, astFile)

		// MySprite.setXYpos
		pos := PosAt(result.proj, astFile, Position{Line: 6, Character: 11})
		require.True(t, pos.IsValid())

		var callExpr *xgoast.CallExpr
		xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
			if node, ok := node.(*xgoast.CallExpr); ok {
				callExpr = node
				return false
			}
			return true
		})
		require.NotNil(t, callExpr)

		spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, callExpr)
		require.NotNil(t, spxSpriteResource)
		assert.Equal(t, "MySprite", spxSpriteResource.Name)
	})

	t.Run("SpriteFile", func(t *testing.T) {
		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///MySprite.spx")
		require.NoError(t, err)
		require.False(t, result.hasErrorSeverityDiagnostic)
		require.NotNil(t, astFile)

		// setCostume
		pos := PosAt(result.proj, astFile, Position{Line: 2, Character: 2})
		require.True(t, pos.IsValid())

		var callExpr *xgoast.CallExpr
		xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
			if node, ok := node.(*xgoast.CallExpr); ok {
				callExpr = node
				return false
			}
			return true
		})
		require.NotNil(t, callExpr)

		spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, callExpr)
		require.NotNil(t, spxSpriteResource)
		assert.Equal(t, "MySprite", spxSpriteResource.Name)
	})

	t.Run("NonSpriteNode", func(t *testing.T) {
		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.False(t, result.hasErrorSeverityDiagnostic)
		require.NotNil(t, astFile)

		// onStart
		pos := PosAt(result.proj, astFile, Position{Line: 5, Character: 2})
		require.True(t, pos.IsValid())

		var callExpr *xgoast.CallExpr
		xgoutil.WalkPathEnclosingInterval(astFile, pos, pos, false, func(node xgoast.Node) bool {
			if node, ok := node.(*xgoast.CallExpr); ok {
				callExpr = node
				return false
			}
			return true
		})
		require.NotNil(t, callExpr)

		spxSpriteResource := inferSpxSpriteResourceEnclosingNode(result, callExpr)
		require.Nil(t, spxSpriteResource)
	})
}

func TestIsBlank(t *testing.T) {
	for _, tt := range []struct {
		name string
		expr xgoast.Expr
		want bool
	}{
		{"BlankIdent", &xgoast.Ident{Name: "_"}, true},
		{"NonBlankIdent", &xgoast.Ident{Name: "variable"}, false},
		{"BasicLit", &xgoast.BasicLit{Value: "test"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlank(tt.expr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSortSpxInputSlots(t *testing.T) {
	t.Run("SortingOrder", func(t *testing.T) {
		slots := []SpxInputSlot{
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 42, Character: 0},
					End:   Position{Line: 42, Character: 10},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 5, Character: 0},
					End:   Position{Line: 5, Character: 10},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 100, Character: 0},
					End:   Position{Line: 100, Character: 10},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 1, Character: 0},
					End:   Position{Line: 1, Character: 10},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 20, Character: 0},
					End:   Position{Line: 20, Character: 10},
				},
			},
		}

		sortSpxInputSlots(slots)

		assert.Equal(t, uint32(1), slots[0].Range.Start.Line)
		assert.Equal(t, uint32(100), slots[len(slots)-1].Range.Start.Line)
		for i := range len(slots) - 1 {
			assert.LessOrEqual(t, slots[i].Range.Start.Line, slots[i+1].Range.Start.Line)
		}
	})

	t.Run("CharacterPositionSorting", func(t *testing.T) {
		slots := []SpxInputSlot{
			// Line 5 with different character positions.
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 5, Character: 20},
					End:   Position{Line: 5, Character: 25},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 5, Character: 5},
					End:   Position{Line: 5, Character: 10},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 5, Character: 15},
					End:   Position{Line: 5, Character: 20},
				},
			},

			// Line 7 with different character positions.
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 7, Character: 30},
					End:   Position{Line: 7, Character: 35},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 7, Character: 10},
					End:   Position{Line: 7, Character: 15},
				},
			},
		}

		sortSpxInputSlots(slots)

		l5Slots := slices.DeleteFunc(slices.Clone(slots), func(s SpxInputSlot) bool {
			return s.Range.Start.Line != 5
		})
		require.Equal(t, 3, len(l5Slots))
		assert.Equal(t, uint32(5), l5Slots[0].Range.Start.Character)
		assert.Equal(t, uint32(15), l5Slots[1].Range.Start.Character)
		assert.Equal(t, uint32(20), l5Slots[2].Range.Start.Character)

		l7Slots := slices.DeleteFunc(slices.Clone(slots), func(s SpxInputSlot) bool {
			return s.Range.Start.Line != 7
		})
		require.Equal(t, 2, len(l7Slots))
		assert.Equal(t, uint32(10), l7Slots[0].Range.Start.Character)
		assert.Equal(t, uint32(30), l7Slots[1].Range.Start.Character)
	})

	t.Run("KindSorting", func(t *testing.T) {
		slots := []SpxInputSlot{
			// Same position (5, 10) with different kinds.
			{
				Kind: SpxInputSlotKindAddress,
				Range: Range{
					Start: Position{Line: 5, Character: 10},
					End:   Position{Line: 5, Character: 15},
				},
			},
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 5, Character: 10},
					End:   Position{Line: 5, Character: 15},
				},
			},

			// Different position.
			{
				Kind: SpxInputSlotKindValue,
				Range: Range{
					Start: Position{Line: 5, Character: 5},
					End:   Position{Line: 5, Character: 10},
				},
			},
		}

		sortSpxInputSlots(slots)

		// First, slots are sorted by position.
		assert.Equal(t, uint32(5), slots[0].Range.Start.Character)

		// Then, slots with the same position are sorted by kind.
		l5c10Slots := slices.DeleteFunc(slices.Clone(slots), func(s SpxInputSlot) bool {
			return s.Range.Start.Line != 5 || s.Range.Start.Character != 10
		})
		require.Equal(t, 2, len(l5c10Slots))
		assert.Equal(t, SpxInputSlotKindAddress, l5c10Slots[0].Kind)
		assert.Equal(t, SpxInputSlotKindValue, l5c10Slots[1].Kind)
	})
}

func findInputSlot(inputSlots []SpxInputSlot, value any, name string, inputType SpxInputType, kind SpxInputKind) *SpxInputSlot {
	for _, slot := range inputSlots {
		if slot.Input.Kind == kind {
			if kind == SpxInputKindInPlace && reflect.DeepEqual(slot.Input.Value, value) && slot.Input.Type == inputType {
				return &slot
			} else if kind == SpxInputKindPredefined && slot.Input.Name == name && slot.Input.Type == inputType {
				return &slot
			}
		}
	}
	return nil
}
func findAddressInputSlot(inputSlots []SpxInputSlot, name string) *SpxInputSlot {
	for _, slot := range inputSlots {
		if slot.Kind == SpxInputSlotKindAddress && slot.Input.Name == name {
			return &slot
		}
	}
	return nil
}
