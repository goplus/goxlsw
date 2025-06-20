package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentDocumentLink(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
const Backdrop1 BackdropName = "backdrop1"
const Backdrop1a = Backdrop1
var (
	MySound  Sound
	MySprite Sprite
)
run "assets", {Title: "Bullet (by XGo)"}
`),
			"MySprite.spx": []byte(`
onStart => {
	play "MySound"
	onBackdrop "backdrop1", func() {}
	MySprite.setCostume "costume1"
	MySprite.animate "anim1"
	getWidget Monitor, "widget1"
	var spriteName SpriteName = "MySprite"
	spriteName = "MySprite"
}
`),
			"assets/index.json":                  []byte(`{"backdrops":[{"name":"backdrop1"}],"zorder":[{"name":"widget1","type":"monitor"}]}`),
			"assets/sprites/MySprite/index.json": []byte(`{"costumes":[{"name":"costume1"}],"fAnimations":{"anim1":{}}}`),
			"assets/sounds/MySound/index.json":   []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		linksForMainSpx, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		})
		require.NoError(t, err)
		require.Len(t, linksForMainSpx, 14)
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 1, Character: 6},
				End:   Position{Line: 1, Character: 15},
			},
			Target: toURI("xgo:main?Backdrop1"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 1, Character: 16},
				End:   Position{Line: 1, Character: 28},
			},
			Target: toURI("xgo:github.com/goplus/spx/v2?BackdropName"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 1, Character: 31},
				End:   Position{Line: 1, Character: 42},
			},
			Target: toURI("spx://resources/backdrops/backdrop1"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 6},
				End:   Position{Line: 2, Character: 16},
			},
			Target: toURI("xgo:main?Backdrop1a"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 19},
				End:   Position{Line: 2, Character: 28},
			},
			Target: toURI("xgo:main?Backdrop1"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 19},
				End:   Position{Line: 2, Character: 28},
			},
			Target: toURI("spx://resources/backdrops/backdrop1"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindConstantReference,
			},
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 4, Character: 1},
				End:   Position{Line: 4, Character: 8},
			},
			Target: toURI("spx://resources/sounds/MySound"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBinding,
			},
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 4, Character: 1},
				End:   Position{Line: 4, Character: 8},
			},
			Target: toURI("xgo:main?Game.MySound"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 4, Character: 10},
				End:   Position{Line: 4, Character: 15},
			},
			Target: toURI("xgo:github.com/goplus/spx/v2?Sound"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 5, Character: 1},
				End:   Position{Line: 5, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBinding,
			},
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 5, Character: 1},
				End:   Position{Line: 5, Character: 9},
			},
			Target: toURI("xgo:main?Game.MySprite"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 5, Character: 10},
				End:   Position{Line: 5, Character: 16},
			},
			Target: toURI("xgo:github.com/goplus/spx/v2?Sprite"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 7, Character: 0},
				End:   Position{Line: 7, Character: 3},
			},
			Target: toURI("xgo:github.com/goplus/spx/v2?Game.run"),
		})
		assert.Contains(t, linksForMainSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 7, Character: 15},
				End:   Position{Line: 7, Character: 20},
			},
			Target: toURI("xgo:github.com/goplus/spx/v2?Game.Title"),
		})

		linksForMySpriteSpx, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
		})
		require.NoError(t, err)
		require.Len(t, linksForMySpriteSpx, 21)
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 3, Character: 12},
				End:   Position{Line: 3, Character: 23},
			},
			Target: toURI("spx://resources/backdrops/backdrop1"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 6},
				End:   Position{Line: 2, Character: 15},
			},
			Target: toURI("spx://resources/sounds/MySound"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 4, Character: 1},
				End:   Position{Line: 4, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBindingReference,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 5, Character: 1},
				End:   Position{Line: 5, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBindingReference,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 5, Character: 18},
				End:   Position{Line: 5, Character: 25},
			},
			Target: toURI("spx://resources/sprites/MySprite/animations/anim1"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 4, Character: 21},
				End:   Position{Line: 4, Character: 31},
			},
			Target: toURI("spx://resources/sprites/MySprite/costumes/costume1"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 6, Character: 20},
				End:   Position{Line: 6, Character: 29},
			},
			Target: toURI("spx://resources/widgets/widget1"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 7, Character: 29},
				End:   Position{Line: 7, Character: 39},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
		assert.Contains(t, linksForMySpriteSpx, DocumentLink{
			Range: Range{
				Start: Position{Line: 8, Character: 14},
				End:   Position{Line: 8, Character: 24},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindStringLiteral,
			},
		})
	})

	t.Run("NonSpxFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.xgo": []byte(`echo "Hello, XGo!"`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.xgo"},
		})
		assert.EqualError(t, err, `file "main.xgo" does not have .spx extension`)
		assert.Nil(t, links)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{}), nil, fileMapGetter(map[string][]byte{}), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///notexist.spx"},
		})
		assert.ErrorIs(t, err, errNoMainSpxFile)
		assert.Nil(t, links)
	})

	t.Run("ParseError", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
// Invalid syntax
var (
	MySound Sound
`),
			"assets/index.json":                []byte(`{}`),
			"assets/sounds/MySound/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		})
		require.NoError(t, err)
		require.Len(t, links, 3)
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 3, Character: 1},
				End:   Position{Line: 3, Character: 8},
			},
			Target: toURI("spx://resources/sounds/MySound"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBinding,
			},
		})
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 3, Character: 1},
				End:   Position{Line: 3, Character: 8},
			},
			Target: toURI("xgo:main?Game.MySound"),
		})
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 3, Character: 9},
				End:   Position{Line: 3, Character: 14},
			},
			Target: toURI("xgo:github.com/goplus/spx/v2?Sound"),
		})
	})

	t.Run("AutoBindingSprite", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)
`),
			"MySprite.spx":                       []byte(``),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		})
		require.NoError(t, err)
		require.Len(t, links, 3)
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBinding,
			},
		})
		assert.NotContains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBindingReference,
			},
		})
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 9},
			},
			Target: toURI("xgo:main?Game.MySprite"),
		})
	})

	t.Run("AutoBindingSpriteAsEmbeddedField", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite
)
`),
			"MySprite.spx":                       []byte(``),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		})
		require.NoError(t, err)
		require.Len(t, links, 3)
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBinding,
			},
		})
		assert.NotContains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 9},
			},
			Target: toURI("spx://resources/sprites/MySprite"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBindingReference,
			},
		})
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 9},
			},
			Target: toURI("xgo:main?Game.MySprite"),
		})
	})

	t.Run("AutoBindingSound", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySound Sound
)
`),
			"assets/index.json":                []byte(`{}`),
			"assets/sounds/MySound/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		})
		require.NoError(t, err)
		require.Len(t, links, 3)
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 8},
			},
			Target: toURI("spx://resources/sounds/MySound"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBinding,
			},
		})
		assert.NotContains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 8},
			},
			Target: toURI("spx://resources/sounds/MySound"),
			Data: SpxResourceRefDocumentLinkData{
				Kind: SpxResourceRefKindAutoBindingReference,
			},
		})
		assert.Contains(t, links, DocumentLink{
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 8},
			},
			Target: toURI("xgo:main?Game.MySound"),
		})
	})

	t.Run("BlankIdentifier", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx":          []byte(`type`),
			"assets/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		links, err := s.textDocumentDocumentLink(&DocumentLinkParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		})
		require.NoError(t, err)
		require.Empty(t, links)
	})
}

func TestSortDocumentLinks(t *testing.T) {
	t.Run("NilTargetSorting", func(t *testing.T) {
		links := []DocumentLink{
			{Range: Range{Start: Position{Line: 10, Character: 5}}, Target: nil},
			{Range: Range{Start: Position{Line: 5, Character: 10}}, Target: toURI("spx://resources/target1")},
			{Range: Range{Start: Position{Line: 20, Character: 15}}, Target: nil},
			{Range: Range{Start: Position{Line: 15, Character: 20}}, Target: toURI("spx://resources/target2")},
		}

		sortDocumentLinks(links)

		// Links with nil targets should come first.
		require.Equal(t, 4, len(links))
		assert.Nil(t, links[0].Target)
		assert.Nil(t, links[1].Target)
		assert.NotNil(t, links[2].Target)
		assert.NotNil(t, links[3].Target)

		// Nil targets should be sorted by line number.
		assert.Equal(t, uint32(10), links[0].Range.Start.Line)
		assert.Equal(t, uint32(20), links[1].Range.Start.Line)
	})

	t.Run("TargetURISorting", func(t *testing.T) {
		targetA := toURI("spx://resources/A")
		targetB := toURI("spx://resources/B")
		targetC := toURI("spx://resources/C")
		links := []DocumentLink{
			{Range: Range{Start: Position{Line: 10, Character: 5}}, Target: targetC},
			{Range: Range{Start: Position{Line: 5, Character: 10}}, Target: targetA},
			{Range: Range{Start: Position{Line: 15, Character: 15}}, Target: targetB},
		}

		sortDocumentLinks(links)

		// Links with targets should be sorted by target URI string.
		require.Equal(t, 3, len(links))
		assert.Equal(t, targetA, links[0].Target)
		assert.Equal(t, targetB, links[1].Target)
		assert.Equal(t, targetC, links[2].Target)
	})

	t.Run("LineNumberSorting", func(t *testing.T) {
		target := toURI("spx://resources/same-target")
		links := []DocumentLink{
			{Range: Range{Start: Position{Line: 30, Character: 5}}, Target: target},
			{Range: Range{Start: Position{Line: 10, Character: 10}}, Target: target},
			{Range: Range{Start: Position{Line: 20, Character: 15}}, Target: target},
		}

		sortDocumentLinks(links)

		// Same target URI should be sorted by line number.
		require.Equal(t, 3, len(links))
		assert.Equal(t, uint32(10), links[0].Range.Start.Line)
		assert.Equal(t, uint32(20), links[1].Range.Start.Line)
		assert.Equal(t, uint32(30), links[2].Range.Start.Line)
	})

	t.Run("CharacterPositionSorting", func(t *testing.T) {
		target := toURI("spx://resources/same-target")
		links := []DocumentLink{
			{Range: Range{Start: Position{Line: 10, Character: 25}}, Target: target},
			{Range: Range{Start: Position{Line: 10, Character: 5}}, Target: target},
			{Range: Range{Start: Position{Line: 10, Character: 15}}, Target: target},
		}

		sortDocumentLinks(links)

		// Same target URI and line should be sorted by character position.
		require.Equal(t, 3, len(links))
		assert.Equal(t, uint32(5), links[0].Range.Start.Character)
		assert.Equal(t, uint32(15), links[1].Range.Start.Character)
		assert.Equal(t, uint32(25), links[2].Range.Start.Character)
	})

	t.Run("ComplexSorting", func(t *testing.T) {
		targetA := toURI("spx://resources/A")
		targetB := toURI("spx://resources/B")
		links := []DocumentLink{
			{Range: Range{Start: Position{Line: 5, Character: 10}}, Target: nil},
			{Range: Range{Start: Position{Line: 5, Character: 20}}, Target: targetB},
			{Range: Range{Start: Position{Line: 10, Character: 5}}, Target: targetA},
			{Range: Range{Start: Position{Line: 5, Character: 5}}, Target: targetA},
			{Range: Range{Start: Position{Line: 5, Character: 15}}, Target: targetA},
			{Range: Range{Start: Position{Line: 1, Character: 10}}, Target: nil},
		}

		sortDocumentLinks(links)

		// Nil targets should come first, sorted by line number.
		require.Equal(t, 6, len(links))
		assert.Nil(t, links[0].Target)
		assert.Nil(t, links[1].Target)
		assert.Equal(t, uint32(1), links[0].Range.Start.Line)
		assert.Equal(t, uint32(5), links[1].Range.Start.Line)

		// Then links sorted by target URI.
		assert.Equal(t, targetA, links[2].Target)
		assert.Equal(t, targetA, links[3].Target)
		assert.Equal(t, targetA, links[4].Target)
		assert.Equal(t, targetB, links[5].Target)

		// Same target URIs should be sorted by line number.
		assert.Equal(t, uint32(5), links[2].Range.Start.Line)
		assert.Equal(t, uint32(5), links[3].Range.Start.Line)
		assert.Equal(t, uint32(10), links[4].Range.Start.Line)

		// Same target URI and line should be sorted by character position.
		assert.Equal(t, uint32(5), links[2].Range.Start.Character)
		assert.Equal(t, uint32(15), links[3].Range.Start.Character)
	})
}
