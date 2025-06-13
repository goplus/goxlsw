/*
 * Copyright (c) 2025 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xgoutil

import (
	"go/types"
	"testing"

	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentAtPosition(t *testing.T) {
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
		ident := IdentAtPosition(proj, astFile, pos(2, 5)) // 'longVarName' start
		require.NotNil(t, ident)
		assert.Equal(t, "longVarName", ident.Name)

		// Line 3: var short = 2
		ident = IdentAtPosition(proj, astFile, pos(3, 5)) // 'short' start
		require.NotNil(t, ident)
		assert.Equal(t, "short", ident.Name)
	})

	t.Run("MultipleIdentifiersOnSameLine", func(t *testing.T) {
		// Find the function declaration.
		var funcDecl *ast.FuncDecl
		for _, decl := range astFile.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok {
				funcDecl = fd
				break
			}
		}
		require.NotNil(t, funcDecl)

		// Find the assignment statement.
		var assignStmt *ast.AssignStmt
		for _, stmt := range funcDecl.Body.List {
			if as, ok := stmt.(*ast.AssignStmt); ok {
				assignStmt = as
				break
			}
		}
		require.NotNil(t, assignStmt)

		// Get positions from AST nodes.
		resultPos := fset.Position(assignStmt.Lhs[0].(*ast.Ident).Pos())
		longVarNamePos := fset.Position(assignStmt.Rhs[0].(*ast.BinaryExpr).X.(*ast.Ident).Pos())
		shortPos := fset.Position(assignStmt.Rhs[0].(*ast.BinaryExpr).Y.(*ast.Ident).Pos())

		// Test each identifier.
		ident := IdentAtPosition(proj, astFile, token.Position{
			Filename: resultPos.Filename,
			Line:     resultPos.Line,
			Column:   resultPos.Column,
		})
		require.NotNil(t, ident)
		assert.Equal(t, "result", ident.Name)

		ident = IdentAtPosition(proj, astFile, token.Position{
			Filename: longVarNamePos.Filename,
			Line:     longVarNamePos.Line,
			Column:   longVarNamePos.Column,
		})
		require.NotNil(t, ident)
		assert.Equal(t, "longVarName", ident.Name)

		ident = IdentAtPosition(proj, astFile, token.Position{
			Filename: shortPos.Filename,
			Line:     shortPos.Line,
			Column:   shortPos.Column,
		})
		require.NotNil(t, ident)
		assert.Equal(t, "short", ident.Name)
	})

	t.Run("NoMatch", func(t *testing.T) {
		// Line 1: empty
		ident := IdentAtPosition(proj, astFile, pos(1, 1))
		assert.Nil(t, ident)

		// Line 2: var longVarName = 1 (after identifier)
		ident = IdentAtPosition(proj, astFile, pos(2, 20))
		assert.Nil(t, ident)
	})

	t.Run("BoundaryConditions", func(t *testing.T) {
		// Line 2: var longVarName = 1
		ident := IdentAtPosition(proj, astFile, pos(2, 4)) // just before 'longVarName'
		assert.Nil(t, ident)

		ident = IdentAtPosition(proj, astFile, pos(2, 16)) // last character of 'longVarName'
		require.NotNil(t, ident)
		assert.Equal(t, "longVarName", ident.Name)

		ident = IdentAtPosition(proj, astFile, pos(2, 17)) // just after 'longVarName'
		assert.Nil(t, ident)
	})

	t.Run("OverlappingIdentifiers", func(t *testing.T) {
		projOverlap := xgo.NewProject(nil, map[string]xgo.File{
			"main.xgo": file(`
var i = 1
var ii = 2

func test() {
	result := i + ii
}
`),
		}, xgo.FeatAll)

		astFileOverlap, err := projOverlap.AST("main.xgo")
		require.NoError(t, err)

		fsetOverlap := projOverlap.Fset
		posOverlap := func(line, column int) token.Position {
			return token.Position{Filename: fsetOverlap.Position(astFileOverlap.Pos()).Filename, Line: line, Column: column}
		}

		// Should find 'i' not 'ii' when at the start of 'i'.
		ident := IdentAtPosition(projOverlap, astFileOverlap, posOverlap(2, 5)) // 'i' position
		require.NotNil(t, ident)
		assert.Equal(t, "i", ident.Name)

		// Should find 'ii' when at the second character.
		ident = IdentAtPosition(projOverlap, astFileOverlap, posOverlap(3, 5)) // 'ii' position
		require.NotNil(t, ident)
		assert.Equal(t, "ii", ident.Name)
	})

	t.Run("EdgeCases", func(t *testing.T) {
		// Invalid line numbers.
		ident := IdentAtPosition(proj, astFile, pos(0, 1)) // line 0
		assert.Nil(t, ident)

		ident = IdentAtPosition(proj, astFile, pos(100, 1)) // line beyond file
		assert.Nil(t, ident)

		// Wrong filename.
		wrongFilenamePos := token.Position{
			Filename: "wrong.xgo",
			Line:     2,
			Column:   5,
		}
		ident = IdentAtPosition(proj, astFile, wrongFilenamePos)
		assert.Nil(t, ident)

		// Very high column number.
		ident = IdentAtPosition(proj, astFile, pos(2, 1000))
		assert.Nil(t, ident)
	})
}

func TestDefIdentFor(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
		"main.xgo": file(`
var x = 1
var y = x + 2

func test() {
	z := x + y
	println(z)
}
`),
	}, xgo.FeatAll)

	_, typeInfo, _, _ := proj.TypeInfo()
	require.NotNil(t, typeInfo)

	// Get all definitions from typeInfo.
	var xObj, yObj types.Object
	for ident, obj := range typeInfo.Defs {
		switch ident.Name {
		case "x":
			xObj = obj
		case "y":
			yObj = obj
		}
	}
	require.NotNil(t, xObj)
	require.NotNil(t, yObj)

	t.Run("FindDefinition", func(t *testing.T) {
		ident := DefIdentFor(typeInfo, xObj)
		require.NotNil(t, ident)
		assert.Equal(t, "x", ident.Name)

		ident = DefIdentFor(typeInfo, yObj)
		require.NotNil(t, ident)
		assert.Equal(t, "y", ident.Name)
	})

	t.Run("NilTypeInfo", func(t *testing.T) {
		assert.Nil(t, DefIdentFor(nil, xObj))
	})

	t.Run("NilObject", func(t *testing.T) {
		assert.Nil(t, DefIdentFor(typeInfo, nil))
	})

	t.Run("UnknownObject", func(t *testing.T) {
		unknownObj := types.NewVar(token.NoPos, nil, "unknown", types.Typ[types.Int])
		assert.Nil(t, DefIdentFor(typeInfo, unknownObj))
	})
}

func TestRefIdentsFor(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
		"main.xgo": file(`
var x = 1
var y = x + 2

func test() {
	z := x + y
	println(z, x)
}
`),
	}, xgo.FeatAll)

	_, typeInfo, _, _ := proj.TypeInfo()
	require.NotNil(t, typeInfo)

	// Find x definition from Defs.
	var xObj types.Object
	for ident, obj := range typeInfo.Defs {
		if ident.Name == "x" {
			xObj = obj
			break
		}
	}
	require.NotNil(t, xObj)

	t.Run("FindReferences", func(t *testing.T) {
		refs := RefIdentsFor(typeInfo, xObj)
		require.Len(t, refs, 3) // y = x + 2, z := x + y, println(z, x)
		for _, ref := range refs {
			assert.Equal(t, "x", ref.Name)
		}
	})

	t.Run("NilTypeInfo", func(t *testing.T) {
		assert.Nil(t, RefIdentsFor(nil, xObj))
	})

	t.Run("NilObject", func(t *testing.T) {
		assert.Nil(t, RefIdentsFor(typeInfo, nil))
	})

	t.Run("UnknownObject", func(t *testing.T) {
		unknownObj := types.NewVar(token.NoPos, nil, "unknown", types.Typ[types.Int])
		assert.Nil(t, RefIdentsFor(typeInfo, unknownObj))
	})
}
