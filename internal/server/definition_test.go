package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentDefinition(t *testing.T) {
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
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		mainSpxMySpriteDef, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 0},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, mainSpxMySpriteDef)
		require.IsType(t, Location{}, mainSpxMySpriteDef)
		assert.Equal(t, Location{
			URI: "file:///main.spx",
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 1},
			},
		}, mainSpxMySpriteDef.(Location))

		mainSpxMySpriteTurnDef, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 9},
			},
		})
		require.NoError(t, err)
		require.Nil(t, mainSpxMySpriteTurnDef)

		mySpriteSpxMySpriteDef, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
				Position:     Position{Line: 2, Character: 1},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, mySpriteSpxMySpriteDef)
		require.IsType(t, Location{}, mainSpxMySpriteDef)
		assert.Equal(t, Location{
			URI: "file:///main.spx",
			Range: Range{
				Start: Position{Line: 2, Character: 1},
				End:   Position{Line: 2, Character: 1},
			},
		}, mainSpxMySpriteDef.(Location))
	})

	t.Run("BuiltinType", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var x int
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 6},
			},
		})
		require.NoError(t, err)
		require.Nil(t, def)
	})

	t.Run("ThisPtr", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
this.run "assets", {Title: "My Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 0},
			},
		})
		require.NoError(t, err)
		require.Nil(t, def)
	})

	t.Run("InvalidPosition", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var x int
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 99, Character: 99},
			},
		})
		require.NoError(t, err)
		require.Nil(t, def)
	})

	t.Run("ImportedPackage", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import "fmt"
fmt.println "Hello, spx!"
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 0},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, def)
		require.IsType(t, Location{}, def)
		assert.Equal(t, Location{
			URI: "file:///main.spx",
			Range: Range{
				Start: Position{Line: 1, Character: 7},
				End:   Position{Line: 1, Character: 7},
			},
		}, def.(Location))
	})

	t.Run("ImportedPackageWithAlias", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
import fmt2 "fmt"
fmt2.println "Hello, spx!"
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 0},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, def)
		require.IsType(t, Location{}, def)
		assert.Equal(t, Location{
			URI: "file:///main.spx",
			Range: Range{
				Start: Position{Line: 1, Character: 7},
				End:   Position{Line: 1, Character: 7},
			},
		}, def.(Location))
	})

	t.Run("InvalidTextDocument", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var x int
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentDefinition(&DefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "bucket:///main.spx"},
				Position:     Position{Line: 99, Character: 99},
			},
		})
		require.Contains(t, err.Error(), "failed to get file path from document URI")
		require.Nil(t, def)
	})
}

func TestServerTextDocumentTypeDefinition(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
type MyType struct {
	field int
}
var x MyType
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentTypeDefinition(&TypeDefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, def)
		require.IsType(t, Location{}, def)
		assert.Equal(t, Location{
			URI: "file:///main.spx",
			Range: Range{
				Start: Position{Line: 1, Character: 5},
				End:   Position{Line: 1, Character: 5},
			},
		}, def)
	})

	t.Run("SpriteType", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentTypeDefinition(&TypeDefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 10},
			},
		})
		require.NoError(t, err)
		require.Nil(t, def)
	})

	t.Run("BuiltinType", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var x int
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentTypeDefinition(&TypeDefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 6},
			},
		})
		require.NoError(t, err)
		require.Nil(t, def)
	})

	t.Run("InvalidPosition", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var x int
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		def, err := s.textDocumentTypeDefinition(&TypeDefinitionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 99, Character: 99},
			},
		})
		require.NoError(t, err)
		require.Nil(t, def)
	})
}
