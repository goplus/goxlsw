package server

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentCompletion(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

MySprite.
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
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		emptyLineItems, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 0},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, emptyLineItems)
		assert.NotEmpty(t, emptyLineItems)
		assert.True(t, containsCompletionItemLabel(emptyLineItems, "println"))
		assert.True(t, containsCompletionSpxDefinitionID(emptyLineItems, SpxDefinitionIdentifier{
			Package: ToPtr("main"),
			Name:    ToPtr("MySprite"),
		}))

		assert.Contains(t, emptyLineItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package: ToPtr(SpxPkgPath),
				Name:    ToPtr("Game.getWidget"),
			},
			Overview: "func getWidget(T Type, name WidgetName) *T",
			Detail:   "GetWidget returns the widget instance (in given type) with given name. It panics if not found.\n",

			CompletionItemLabel:            "getWidget",
			CompletionItemKind:             FunctionCompletion,
			CompletionItemInsertText:       "getWidget",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())

		mySpriteDotItems, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 5, Character: 9},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, mySpriteDotItems)
		assert.NotEmpty(t, mySpriteDotItems)
		assert.False(t, containsCompletionItemLabel(mySpriteDotItems, "println"))
		assert.True(t, containsCompletionSpxDefinitionID(mySpriteDotItems, SpxDefinitionIdentifier{
			Package:    ToPtr(SpxPkgPath),
			Name:       ToPtr("Sprite.turn"),
			OverloadID: ToPtr("0"),
		}))
		assert.True(t, containsCompletionSpxDefinitionID(mySpriteDotItems, SpxDefinitionIdentifier{
			Package:    ToPtr(SpxPkgPath),
			Name:       ToPtr("Sprite.turn"),
			OverloadID: ToPtr("0"),
		}))
		assert.True(t, containsCompletionSpxDefinitionID(mySpriteDotItems, SpxDefinitionIdentifier{
			Package:    ToPtr(SpxPkgPath),
			Name:       ToPtr("Sprite.turn"),
			OverloadID: ToPtr("1"),
		}))
		assert.True(t, containsCompletionSpxDefinitionID(mySpriteDotItems, SpxDefinitionIdentifier{
			Package:    ToPtr(SpxPkgPath),
			Name:       ToPtr("Sprite.clone"),
			OverloadID: ToPtr("0"),
		}))
		assert.True(t, containsCompletionSpxDefinitionID(mySpriteDotItems, SpxDefinitionIdentifier{
			Package:    ToPtr(SpxPkgPath),
			Name:       ToPtr("Sprite.clone"),
			OverloadID: ToPtr("1"),
		}))
	})

	t.Run("InSpxEventHandler", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {

}
run "assets", {Title: "My Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 1},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.False(t, containsCompletionSpxDefinitionID(items, SpxDefinitionIdentifier{
			Package: ToPtr(SpxPkgPath),
			Name:    ToPtr("Sprite.onStart"),
		}))
		assert.False(t, containsCompletionSpxDefinitionID(items, SpxDefinitionIdentifier{
			Package: ToPtr(SpxPkgPath),
			Name:    ToPtr("Sprite.onClick"),
		}))
	})

	t.Run("InStringLit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
run "a
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("InComment", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
// Run My G
run "assets", {Title: "My Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 11},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("InStringLit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
run "a
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("InImportStringLit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import "f
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 9},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "fmt"))
	})

	t.Run("InImportGroupStringLit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import (
	"f
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 3},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "fmt"))
	})

	t.Run("PackageMember", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import "fmt"
fmt.
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 4},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "println"))
	})

	t.Run("GeneralOrUnknown", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`

onStart => {

}
run "assets", {Title: "My Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items1, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 1},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items1)
		assert.NotEmpty(t, items1)
		assert.True(t, containsCompletionItemLabel(items1, "len"))

		items2, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 12},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items2)
		assert.Empty(t, items2)

		items3, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 3, Character: 1},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items3)
		assert.NotEmpty(t, items3)
		assert.True(t, containsCompletionItemLabel(items3, "len"))
	})

	t.Run("VarDecl", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
func test() {}
onStart => {
	var x i
}
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 3, Character: 8},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "int"))
		assert.True(t, containsCompletionItemLabel(items, "MySprite"))
		assert.True(t, containsCompletionItemLabel(items, "Sprite"))
		assert.False(t, containsCompletionItemLabel(items, "len"))
		assert.False(t, containsCompletionItemLabel(items, "test"))
		assert.False(t, containsCompletionItemLabel(items, "play"))
	})

	t.Run("VarDeclAndAssign", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {
	var x SpriteName = "m"
}
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 22},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "MySprite"))
	})

	t.Run("SpxSoundResourceStringLit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
play "r"
run "assets", {Title: "My Game"}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sounds/recording/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 7},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "recording"))
	})

	t.Run("FuncOverloads", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	recording Sound
)

play r
run "assets", {Title: "My Game"}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sounds/recording/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 5, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "recording"))
		assert.True(t, containsCompletionItemLabel(items, `"recording"`))
	})

	t.Run("WithImplicitSpxSpriteResource", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
onClick => {
	setCostume "c"
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{"costumes":[{"name":"costume"}]}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
				Position:     Position{Line: 2, Character: 14},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "costume"))
	})

	t.Run("WithExplicitSpxSpriteResource", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

MySprite.setCostume "c"
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{"costumes":[{"name":"costume"}]}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 5, Character: 22},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "costume"))
	})

	t.Run("WithCrossSpxSpriteResource", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	Sprite1 Sprite
	Sprite2 Sprite2
)
run "assets", {Title: "My Game"}
`),
			"Sprite1.spx": []byte(`
onClick => {
	Sprite2.setCostume "c"
}
`),
			"Sprite2.spx": []byte(`
`),
			"assets/index.json":                 []byte(`{}`),
			"assets/sprites/Sprite1/index.json": []byte(`{"costumes":[{"name":"Sprite1Costume"}]}`),
			"assets/sprites/Sprite2/index.json": []byte(`{"costumes":[{"name":"Sprite2Costume"}]}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///Sprite1.spx"},
				Position:     Position{Line: 2, Character: 22},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "Sprite2Costume"))
	})

	t.Run("AtLineStartWithAnIdentifier", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onClick => {
	pr
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 3},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "println"))
	})

	t.Run("AtLineStartWithAMemberAccessExpression", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)
MySprite.setCo`), // Cursor at EOF.
			"MySprite.spx": []byte(`
onClick => {
	MySprite.setCo
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items1, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 14},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items1)
		assert.NotEmpty(t, items1)
		assert.True(t, containsCompletionItemLabel(items1, "setCostume"))

		items2, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
				Position:     Position{Line: 2, Character: 15},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items2)
		assert.NotEmpty(t, items2)
		assert.True(t, containsCompletionItemLabel(items2, "setCostume"))
	})

	t.Run("WithXGoBuiltins", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onClick => {
	var n in
}
`),
			"MySprite.spx": []byte(`
onClick => {
	ec
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items1, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 9},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items1)
		assert.NotEmpty(t, items1)
		assert.True(t, containsCompletionItemLabel(items1, "int128"))

		items2, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
				Position:     Position{Line: 2, Character: 3},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items2)
		assert.NotEmpty(t, items2)
		assert.True(t, containsCompletionItemLabel(items2, "echo"))
	})

	t.Run("MainPackageInterfaceMethod", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type Runner interface {
	Run()
}

type MyRunner struct {}
func (r *MyRunner) Run() {}

onStart => {}
	var r Runner = new(MyRunner)
	r.
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 10, Character: 3},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionSpxDefinitionID(items, SpxDefinitionIdentifier{
			Package: ToPtr("main"),
			Name:    ToPtr("Runner.Run"),
		}))
		assert.False(t, containsCompletionSpxDefinitionID(items, SpxDefinitionIdentifier{
			Package: ToPtr("main"),
			Name:    ToPtr("MyRunner.Run"),
		}))
	})

	t.Run("NonMainPackageInterfaceMethod", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import "fmt"

type MyStringer struct {}
func (s *MyStringer) String() string {}

onStart => {}
	var s fmt.Stringer = new(MyStringer)
	s.
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 8, Character: 3},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionSpxDefinitionID(items, SpxDefinitionIdentifier{
			Package: ToPtr("fmt"),
			Name:    ToPtr("Stringer.string"),
		}))
		assert.False(t, containsCompletionSpxDefinitionID(items, SpxDefinitionIdentifier{
			Package: ToPtr("main"),
			Name:    ToPtr("MyStringer.String"),
		}))
	})

	t.Run("MainPackageStructLiteralField", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type Point struct {
	X int
	Y int
}

onStart => {
	p := Point{}
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 7, Character: 12},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, slices.ContainsFunc(items, func(item CompletionItem) bool {
			itemData, ok := item.Data.(*CompletionItemData)
			if ok && itemData.Definition.String() == "xgo:main?Point.X" {
				assert.Equal(t, "X: ${1:}", item.InsertText)
				assert.Equal(t, ToPtr(SnippetTextFormat), item.InsertTextFormat)
				return true
			}
			return false
		}))
		assert.True(t, slices.ContainsFunc(items, func(item CompletionItem) bool {
			itemData, ok := item.Data.(*CompletionItemData)
			if ok && itemData.Definition.String() == "xgo:main?Point.Y" {
				assert.Equal(t, "Y: ${1:}", item.InsertText)
				assert.Equal(t, ToPtr(SnippetTextFormat), item.InsertTextFormat)
				return true
			}
			return false
		}))
	})

	t.Run("NonMainPackageStructLiteralField", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import "image/color"

onStart => {
	c := color.RGBA{}
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 17},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, slices.ContainsFunc(items, func(item CompletionItem) bool {
			itemData, ok := item.Data.(*CompletionItemData)
			if ok && itemData.Definition.String() == "xgo:image/color?RGBA.R" {
				assert.Equal(t, "R: ${1:}", item.InsertText)
				assert.Equal(t, ToPtr(SnippetTextFormat), item.InsertTextFormat)
				return true
			}
			return false
		}))
	})

	t.Run("UnresolvedFuncCall", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStar => {
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "onStart"))
	})

	t.Run("WithinIdentifier", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {
	var abc bool
	if ab {
	}
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 3, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "abc"))
	})

	t.Run("ErrorInterfaceMethodCall", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type myError struct{}

func (myError) Error() string {
	return "myError"
}

onStart => {
	var err error = myError{}
	echo err.
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 9, Character: 10},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "error"))
	})

	t.Run("StartWithInvalidChar", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
“”var (
	maps []int
)
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 9, Character: 10},
			},
		})
		require.NoError(t, err)
		require.Nil(t, items)
		assert.Empty(t, items)
	})

	t.Run("MathPackage", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {
	n := ab
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 8},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "abs"))
	})

	t.Run("StructLit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type MyStruct struct {
	Foobar int
}

onStart => {
	ms := My
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 6, Character: 9},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "MyStruct"))
	})

	t.Run("StructLitFieldName", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type MyStruct struct {
	Foobar int
}

onStart => {
	ms := MyStruct{
		Fo
	}
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 7, Character: 4},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "Foobar"))
	})

	t.Run("TypeAssertion", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type MyStruct struct {
	Foobar int
}

onStart => {
	var i any = MyStruct{}
	_, ok := i.(My)
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 7, Character: 15},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
		assert.True(t, containsCompletionItemLabel(items, "MyStruct"))
	})
}

func containsCompletionItemLabel(items []CompletionItem, label string) bool {
	return slices.ContainsFunc(items, func(item CompletionItem) bool {
		return item.Label == label
	})
}

func containsCompletionSpxDefinitionID(items []CompletionItem, id SpxDefinitionIdentifier) bool {
	return slices.ContainsFunc(items, func(item CompletionItem) bool {
		itemData, ok := item.Data.(*CompletionItemData)
		if !ok {
			return false
		}
		return itemData.Definition.String() == id.String()
	})
}
