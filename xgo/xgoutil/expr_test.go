package xgoutil

import (
	"testing"

	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterExprAtPosition(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
		"main.xgo": file(`
var longVarName = 1
var short = 2

func test() {
	result := longVarName + short
	println(result)
}
`),
	}, xgo.FeatAll)

	astFile, err := proj.AST("main.xgo")
	require.NoError(t, err)

	// Get positions for all identifiers.
	fset := proj.Fset
	pos := func(line, column int) token.Position {
		return token.Position{Filename: fset.Position(astFile.Pos()).Filename, Line: line, Column: column}
	}

	t.Run("ExactMatch", func(t *testing.T) {
		// Line 2: var longVarName = 1
		filter := FilterExprAtPosition(proj, astFile, pos(2, 5)) // 'longVarName' start
		ident := IdentAtPosition(proj, astFile, pos(2, 5))
		assert.Equal(t, true, filter(ident))

		// Line 3: var short = 2
		filter = FilterExprAtPosition(proj, astFile, pos(3, 5)) // 'longVarName' start
		ident = IdentAtPosition(proj, astFile, pos(3, 5))       // 'short' start
		assert.Equal(t, true, filter(ident))
	})

	t.Run("NoMatch", func(t *testing.T) {
		// Line 1: empty
		filter := FilterExprAtPosition(proj, astFile, pos(1, 1)) // 'longVarName' start
		ident := IdentAtPosition(proj, astFile, pos(2, 5))
		assert.Equal(t, false, filter(ident))

		// Line 2: var longVarName = 1 (after identifier)
		filter = FilterExprAtPosition(proj, astFile, pos(2, 20)) // 'longVarName' start
		ident = IdentAtPosition(proj, astFile, pos(3, 5))
		assert.Equal(t, false, filter(ident))
	})
}
