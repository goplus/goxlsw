package server

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentInlayHint(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Function calls with named parameters.
	println "Hello, World!"
	MySprite.turn Left

	// Call with multiple parameters.
	setEffect ColorEffect, 50

	// Function with HSB color value.
	color := HSB(255, 0, 0)
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		params := &InlayHintParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 100, Character: 0},
			},
		}

		inlayHints, err := s.textDocumentInlayHint(params)
		require.NoError(t, err)
		require.NotNil(t, inlayHints)
		assert.NotEmpty(t, inlayHints)

		assert.True(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line == 7 && hint.Label != "" && hint.Kind == Parameter
		}))

		assert.True(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line == 8 && hint.Label != "" && hint.Kind == Parameter
		}))

		setEffectHintCount := 0
		for _, hint := range inlayHints {
			if hint.Position.Line == 11 && hint.Kind == Parameter {
				setEffectHintCount++
			}
		}
		assert.Equal(t, 2, setEffectHintCount)

		hsbHintCount := 0
		for _, hint := range inlayHints {
			if hint.Position.Line == 14 && hint.Kind == Parameter {
				hsbHintCount++
			}
		}
		assert.Equal(t, 3, hsbHintCount)
	})

	t.Run("EmptyFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(``),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		params := &InlayHintParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 1, Character: 0},
			},
		}

		inlayHints, err := s.textDocumentInlayHint(params)
		require.NoError(t, err)
		assert.Empty(t, inlayHints)
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{}), nil, fileMapGetter(map[string][]byte{}), &MockScheduler{})

		params := &InlayHintParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///nonexistent.spx"},
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 10, Character: 0},
			},
		}

		inlayHints, err := s.textDocumentInlayHint(params)
		require.Error(t, err)
		assert.Nil(t, inlayHints)
	})

	t.Run("SpecificRange", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Line 6
	println "Hello"
	// Line 8
	MySprite.turn Left
	// Line 10
	setEffect ColorEffect, 50
	// Line 12
	color := HSB(255, 0, 0)
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		params := &InlayHintParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
			Range: Range{
				Start: Position{Line: 7, Character: 0},
				End:   Position{Line: 11, Character: 0},
			},
		}

		inlayHints, err := s.textDocumentInlayHint(params)
		require.NoError(t, err)
		require.NotNil(t, inlayHints)
		assert.Equal(t, 2, len(inlayHints))

		for _, hint := range inlayHints {
			assert.True(t, hint.Position.Line >= 7 && hint.Position.Line <= 11)
		}
		assert.True(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line == 7 && hint.Kind == Parameter
		}))
		assert.True(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line == 9 && hint.Kind == Parameter
		}))
		assert.False(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line < 7 || hint.Position.Line > 11
		}))
	})
}

func TestCollectInlayHints(t *testing.T) {
	t.Run("FunctionCallsWithNamedParams", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Regular function calls with parameters.
	println "Hello, World!"
	MySprite.turn Left

	// Function call with multiple parameters.
	setEffect ColorEffect, 50
	getWidget Monitor, "myWidget"

	// Function call with lambda expression.
	onKey [KeySpace], () => {
		println "Space pressed"
	}

	// Variables with function calls.
	color := HSB(255, 0, 0)
}
`),
			"MySprite.spx": []byte(`
onStart => {
	// Branch statement that converts to call expression.
	goto "OtherSprite"
}
`),
			"OtherSprite.spx":                       []byte(``),
			"assets/index.json":                     []byte(`{"zorder":[{"name":"myWidget"}]}`),
			"assets/sprites/MySprite/index.json":    []byte(`{}`),
			"assets/sprites/OtherSprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		inlayHints := collectInlayHints(result, astFile, 0, 0)
		require.NotNil(t, inlayHints)
		assert.NotEmpty(t, inlayHints)

		assert.True(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line == 7 && hint.Label != ""
		}))

		assert.True(t, slices.ContainsFunc(inlayHints, func(hint InlayHint) bool {
			return hint.Position.Line == 8 && hint.Label != ""
		}))

		setEffectHintCount := 0
		for _, hint := range inlayHints {
			if hint.Position.Line == 11 {
				setEffectHintCount++
			}
		}
		assert.Equal(t, 2, setEffectHintCount)

		var (
			getWidgetHintCount  int
			getWidgetHintLabels []string
		)
		for _, hint := range inlayHints {
			if hint.Position.Line == 12 {
				getWidgetHintCount++
				getWidgetHintLabels = append(getWidgetHintLabels, hint.Label)
			}
		}
		assert.Equal(t, 2, getWidgetHintCount)
		assert.ElementsMatch(t, []string{"T", "name"}, getWidgetHintLabels)

		hsbHintCount := 0
		for _, hint := range inlayHints {
			if hint.Position.Line == 20 {
				hsbHintCount++
			}
		}
		assert.Equal(t, 3, hsbHintCount)

		spriteResult, _, spriteAstFile, err := s.compileAndGetASTFileForDocumentURI("file:///MySprite.spx")
		require.NoError(t, err)
		require.NotNil(t, spriteAstFile)

		spriteInlayHints := collectInlayHints(spriteResult, spriteAstFile, 0, 0)
		require.NotNil(t, spriteInlayHints)
		assert.NotEmpty(t, spriteInlayHints)

		hasGotoHint := false
		for _, hint := range spriteInlayHints {
			if hint.Position.Line == 3 {
				hasGotoHint = true
				break
			}
		}
		assert.True(t, hasGotoHint)
	})

	t.Run("NoInlayHints", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// No function calls with parameters.
	a := 5
	b := 10
	c := a + b
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		inlayHints := collectInlayHints(result, astFile, 0, 0)
		assert.Empty(t, inlayHints)
	})

	t.Run("LambdaExpressionSkipped", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {
	// Function with lambda argument (should be skipped).
	onKey [KeySpace], () => {
		println "Space pressed"
	}
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		inlayHints := collectInlayHints(result, astFile, 0, 0)
		require.NotNil(t, inlayHints)
		assert.NotEmpty(t, inlayHints)

		onKeyHintCount := 0
		for _, hint := range inlayHints {
			if hint.Position.Line == 4 {
				onKeyHintCount++
			}
		}
		assert.Equal(t, 1, onKeyHintCount)
	})

	t.Run("EmptyFile", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(``),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		inlayHints := collectInlayHints(result, astFile, 0, 0)
		assert.Empty(t, inlayHints)
	})

	t.Run("RangeFiltering", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

onStart => {
	// Line 6
	println "Hello"
	// Line 8
	MySprite.turn Left
	// Line 10
	setEffect ColorEffect, 50
	// Line 12
	color := HSB(255, 0, 0)
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		rangeStart := PosAt(result.proj, astFile, Position{Line: 7, Character: 0})
		rangeEnd := PosAt(result.proj, astFile, Position{Line: 10, Character: 0})
		filteredHints := collectInlayHints(result, astFile, rangeStart, rangeEnd)
		require.NotNil(t, filteredHints)
		assert.NotEmpty(t, filteredHints)

		allHints := collectInlayHints(result, astFile, 0, 0)
		require.NotNil(t, allHints)
		assert.NotEmpty(t, allHints)

		assert.Less(t, len(filteredHints), len(allHints))

		for _, hint := range filteredHints {
			assert.True(t, hint.Position.Line >= 7 && hint.Position.Line <= 10)
		}
		assert.True(t, slices.ContainsFunc(filteredHints, func(hint InlayHint) bool {
			return hint.Position.Line == 7 && hint.Kind == Parameter
		}))
		assert.True(t, slices.ContainsFunc(filteredHints, func(hint InlayHint) bool {
			return hint.Position.Line == 9 && hint.Kind == Parameter
		}))

		hsbHintCount := 0
		for _, hint := range filteredHints {
			if hint.Position.Line > 10 {
				hsbHintCount++
			}
		}
		assert.Zero(t, hsbHintCount)
	})

	t.Run("UnresolvedOverloadFuncCall", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {
	onKey nonExistent
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		inlayHints := collectInlayHints(result, astFile, 0, 0)
		require.Nil(t, inlayHints)
	})

	t.Run("VariadicFunctionArguments", func(t *testing.T) {
		m := map[string][]byte{
			"main.spx": []byte(`
onStart => {
	echo 1
	echo 1, 2, 3
}
`),
		}
		s := New(newMapFSWithoutModTime(m), nil, fileMapGetter(m), &MockScheduler{})

		result, _, astFile, err := s.compileAndGetASTFileForDocumentURI("file:///main.spx")
		require.NoError(t, err)
		require.NotNil(t, astFile)

		inlayHints := collectInlayHints(result, astFile, 0, 0)
		require.NotNil(t, inlayHints)
		require.Len(t, inlayHints, 2)
		assert.Equal(t, "a...", inlayHints[0].Label)
		assert.Equal(t, "a...", inlayHints[1].Label)
	})
}

func TestSortInlayHints(t *testing.T) {
	t.Run("SortingOrder", func(t *testing.T) {
		hints := []InlayHint{
			{Position: Position{Line: 42, Character: 0}, Label: "Z", Kind: Parameter},
			{Position: Position{Line: 5, Character: 0}, Label: "Y", Kind: Parameter},
			{Position: Position{Line: 100, Character: 0}, Label: "X", Kind: Parameter},
			{Position: Position{Line: 1, Character: 0}, Label: "W", Kind: Parameter},
			{Position: Position{Line: 20, Character: 0}, Label: "V", Kind: Parameter},
		}

		sortInlayHints(hints)

		assert.Equal(t, uint32(1), hints[0].Position.Line)
		assert.Equal(t, "W", hints[0].Label)
		assert.Equal(t, uint32(100), hints[len(hints)-1].Position.Line)
		assert.Equal(t, "X", hints[len(hints)-1].Label)
		for i := range len(hints) - 1 {
			assert.LessOrEqual(t, hints[i].Position.Line, hints[i+1].Position.Line)
		}
	})

	t.Run("CharacterPositionSorting", func(t *testing.T) {
		hints := []InlayHint{
			// Line 5 with different character positions.
			{Position: Position{Line: 5, Character: 20}, Label: "A1", Kind: Parameter},
			{Position: Position{Line: 5, Character: 5}, Label: "A2", Kind: Parameter},
			{Position: Position{Line: 5, Character: 15}, Label: "A3", Kind: Parameter},

			// Line 7 with different character positions.
			{Position: Position{Line: 7, Character: 30}, Label: "B1", Kind: Parameter},
			{Position: Position{Line: 7, Character: 10}, Label: "B2", Kind: Parameter},
		}

		sortInlayHints(hints)

		l5Hints := slices.DeleteFunc(slices.Clone(hints), func(h InlayHint) bool {
			return h.Position.Line != 5
		})
		require.Equal(t, 3, len(l5Hints))
		assert.Equal(t, uint32(5), l5Hints[0].Position.Character)
		assert.Equal(t, uint32(15), l5Hints[1].Position.Character)
		assert.Equal(t, uint32(20), l5Hints[2].Position.Character)

		l7Hints := slices.DeleteFunc(slices.Clone(hints), func(h InlayHint) bool {
			return h.Position.Line != 7
		})
		require.Equal(t, 2, len(l7Hints))
		assert.Equal(t, uint32(10), l7Hints[0].Position.Character)
		assert.Equal(t, "B2", l7Hints[0].Label)
		assert.Equal(t, uint32(30), l7Hints[1].Position.Character)
		assert.Equal(t, "B1", l7Hints[1].Label)
	})

	t.Run("LabelSorting", func(t *testing.T) {
		hints := []InlayHint{
			// Same position (5, 10) with different labels in non-alphabetical order.
			{Position: Position{Line: 5, Character: 10}, Label: "Z", Kind: Parameter},
			{Position: Position{Line: 5, Character: 10}, Label: "A", Kind: Parameter},
			{Position: Position{Line: 5, Character: 10}, Label: "M", Kind: Parameter},

			// Different position.
			{Position: Position{Line: 5, Character: 5}, Label: "X", Kind: Parameter},
		}

		sortInlayHints(hints)

		l5c10Hints := slices.DeleteFunc(slices.Clone(hints), func(hint InlayHint) bool {
			return hint.Position.Line != 5 || hint.Position.Character != 10
		})
		require.Equal(t, 3, len(l5c10Hints))
		assert.Equal(t, "A", l5c10Hints[0].Label)
		assert.Equal(t, "M", l5c10Hints[1].Label)
		assert.Equal(t, "Z", l5c10Hints[2].Label)
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

		params := &InlayHintParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 4, Character: 0},
			},
		}

		inlayHints, err := s.textDocumentInlayHint(params)
		require.NoError(t, err)
		require.Nil(t, inlayHints)
		assert.Empty(t, inlayHints)
	})
}
