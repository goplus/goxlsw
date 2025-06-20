package server

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentFormatting(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
// An spx game.

var (
  MyAircraft MyAircraft
  Bullet Bullet
)
type Score int
run "assets",    { Title:    "Bullet (by XGo)" }
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 9, Character: 0},
			},
			NewText: `// An spx game.

type Score int

var (
	MyAircraft MyAircraft
	Bullet     Bullet
)

run "assets", {Title: "Bullet (by XGo)"}
`,
		})
	})

	t.Run("NonSpxFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.xgo": []byte(`echo "Hello, XGo!"`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.xgo"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Nil(t, edits)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{}), nil, fileMapGetter(map[string][]byte{}), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///notexist.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.Nil(t, edits)
	})

	t.Run("NoChangesNeeded", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`run "assets", {Title: "Bullet (by XGo)"}` + "\n"),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Nil(t, edits)
	})

	t.Run("AcceptableFormatError", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

var MyAircraft MyAircraft
!InvalidSyntax
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 4, Character: 0},
			},
			NewText: `// An spx game.

var (
	MyAircraft MyAircraft
)

!InvalidSyntax
`,
		})
	})

	t.Run("WithFormatSpx", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

// The first var block.
var (
	// The aircraft.
	MyAircraft MyAircraft // The only aircraft.
) // Trailing comment for the first var block.

var Bullet Bullet // The first bullet.

// The second bullet.
var Bullet2 Bullet

var ( // Weirdly placed comment for the fourth var block.
	// The third bullet.
	Bullet3 Bullet
)

// The fifth var block.
var (
	Bullet4 Bullet // The fourth bullet.
) // Trailing comment for the fifth var block.

// The last var block.
var (
	// The fifth bullet.
	Bullet5 Bullet

	Bullet6 Bullet // The sixth bullet.
) // Trailing comment for the last var block.
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 30, Character: 0},
			},
			NewText: `// An spx game.

var (
	// The first var block.

	// The aircraft.
	MyAircraft MyAircraft // The only aircraft.

	// Trailing comment for the first var block.

	Bullet Bullet // The first bullet.

	// The second bullet.
	Bullet2 Bullet

	// Weirdly placed comment for the fourth var block.

	// The third bullet.
	Bullet3 Bullet

	// The fifth var block.

	Bullet4 Bullet // The fourth bullet.

	// Trailing comment for the fifth var block.

	// The last var block.

	// The fifth bullet.
	Bullet5 Bullet

	Bullet6 Bullet // The sixth bullet.

	// Trailing comment for the last var block.
)
`,
		})
	})

	t.Run("VarBlockWithoutDoc", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

var (
	// The aircraft.
	MyAircraft MyAircraft // The only aircraft.
)

var Bullet Bullet
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 8, Character: 0},
			},
			NewText: `// An spx game.

var (
	// The aircraft.
	MyAircraft MyAircraft // The only aircraft.

	Bullet Bullet
)
`,
		})
	})

	t.Run("NoTypeSpriteVarDeclaration", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

var (
	MySprite
)

run "assets", {Title: "My Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Nil(t, edits)
	})

	t.Run("WithImportStmt", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.
import "math"

onClick => {
	println math.floor(2.5)
}

run "assets", {Title: "My Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Nil(t, edits)
	})

	t.Run("WithUnusedLambdaParams", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.
onKey [KeyLeft, KeyRight], (key) => {
	println "key"
}

onKey [KeyLeft, KeyRight], (key) => {
	println key
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 8, Character: 0},
			},
			NewText: `// An spx game.
onKey [KeyLeft, KeyRight], () => {
	println "key"
}

onKey [KeyLeft, KeyRight], (key) => {
	println key
}
`,
		})
	})

	t.Run("WithUnusedLambdaParamsForSprite", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": {},
			"MySprite.spx": []byte(`// An spx game.
onKey [KeyLeft, KeyRight], (key) => {
	println "key"
}
onTouchStart s => {}
onTouchStart s => {
	println "touched", s
}
onTouchStart => {}
onTouchStart (s, t) => { // type mismatch
}
onTouchStart 123, (s) => { // type mismatch
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///MySprite.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 13, Character: 0},
			},
			NewText: `// An spx game.
onKey [KeyLeft, KeyRight], () => {
	println "key"
}
onTouchStart => {
}
onTouchStart s => {
	println "touched", s
}
onTouchStart => {
}
onTouchStart (s, t) => { // type mismatch
}
onTouchStart 123, (s) => { // type mismatch
}
`,
		})
	})

	t.Run("EmptyFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(``),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Nil(t, edits)
	})

	t.Run("WhitespaceOnlyFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(` `),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
			},
			NewText: ``,
		})
	})

	t.Run("WithFloatingComments", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`import "fmt"

// floating comment1

// comment for var a
var a int

// floating comment2

// comment for func test
func test() {
	// comment inside func test
}

// floating comment3

// comment for const b
const b = "123"

// floating comment4

run "assets", {Title: "My Game"}

// floating comment5
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 24, Character: 0},
			},
			NewText: `import "fmt"

// floating comment1

// floating comment2

// floating comment3

// comment for const b
const b = "123"

var (
	// comment for var a
	a int
)

// comment for func test
func test() {
	// comment inside func test
}

// floating comment4

run "assets", {Title: "My Game"}

// floating comment5
`,
		})
	})

	t.Run("WithTrailingComments", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`import "fmt" // trailing comment for import "fmt"

const foo = "bar" // trailing comment for const foo

var a int // trailing comment for var a

func test() {} // trailing comment for func test
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 7, Character: 0},
			},
			NewText: `import "fmt" // trailing comment for import "fmt"

const foo = "bar" // trailing comment for const foo

var (
	a int // trailing comment for var a
)

func test() {} // trailing comment for func test
`,
		})
	})

	t.Run("WithMethods", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

type Foo struct{}

var (
	flag bool
)

func (Foo) Bar() {}

func Bar() {}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 11, Character: 0},
			},
			NewText: `// An spx game.

type Foo struct{}

func (Foo) Bar() {}

var (
	flag bool
)

func Bar() {}
`,
		})
	})

	t.Run("VarBlocksWithAndWithoutInit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

var (
	dir int
	snakeBodyParts []Sprite
)

var (
	moveStep int = 20
)

run "assets", {Title: "Snake Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 12, Character: 0},
			},
			NewText: `// An spx game.

var (
	dir            int
	snakeBodyParts []Sprite
)

var (
	moveStep int = 20
)

run "assets", {Title: "Snake Game"}
`,
		})
	})

	t.Run("MultipleVarBlocksWithMultipleTypes", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

var (
    score int
    highScore int
)

var (
    playerName string = "Player1"
    gameStarted bool = false
)

var (
    lives int
)

var (
    gameSpeed int = 5
)

run "assets", {Title: "Game"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 21, Character: 0},
			},
			NewText: `// An spx game.

var (
	score     int
	highScore int

	lives int
)

var (
	playerName  string = "Player1"
	gameStarted bool   = false

	gameSpeed int = 5
)

run "assets", {Title: "Game"}
`,
		})
	})

	t.Run("MixedVarDeclarationsWithComments", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

// Variables without initialization
var (
    // Score for the player
    playerScore int
    // Lives remaining
    livesRemaining int
)

// Variables with initialization
var (
    // Game speed setting
    speed int = 10
    // Player name
    name string = "DefaultPlayer"
)

run "assets", {Title: "Game With Comments"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 19, Character: 0},
			},
			NewText: `// An spx game.

// Variables without initialization
var (
	// Score for the player
	playerScore int
	// Lives remaining
	livesRemaining int
)

// Variables with initialization
var (
	// Game speed setting
	speed int = 10
	// Player name
	name string = "DefaultPlayer"
)

run "assets", {Title: "Game With Comments"}
`,
		})
	})

	t.Run("SingleVarDeclarationsWithMixedInit", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.

var x int
var y int = 10
var z string
var name string = "Player"

run "assets", {Title: "Single Vars"}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 1)
		assert.Contains(t, edits, TextEdit{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 8, Character: 0},
			},
			NewText: `// An spx game.

var (
	x int

	z string
)

var (
	y int = 10

	name string = "Player"
)

run "assets", {Title: "Single Vars"}
`,
		})
	})

	t.Run("WithShadowEntryComments", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`// An spx game.
var (
	count int
)

// onStart comment
onStart => {
	count++
	echo count
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})
		params := &DocumentFormattingParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		edits, err := s.textDocumentFormatting(params)
		require.NoError(t, err)
		require.Len(t, edits, 0)
	})
}
