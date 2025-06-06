/*
 * Copyright (c) 2025 The GoPlus Authors (goplus.org). All rights reserved.
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

package goputil

import (
	"go/types"
	"slices"
	"testing"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"github.com/goplus/goxlsw/gop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveIdentFromNode(t *testing.T) {
	t.Run("NilTypeInfo", func(t *testing.T) {
		ident := &ast.Ident{Name: "test"}
		assert.Nil(t, ResolveIdentFromNode(nil, ident))
	})

	t.Run("NilNode", func(t *testing.T) {
		proj := gop.NewProject(nil, map[string]gop.File{
			"main.gop": file("var x = 1"),
		}, gop.FeatAll)
		_, typeInfo, _, _ := proj.TypeInfo()

		assert.Nil(t, ResolveIdentFromNode(typeInfo, nil))
	})

	t.Run("IdentifierNode", func(t *testing.T) {
		ident := &ast.Ident{Name: "testIdent"}
		proj := gop.NewProject(nil, map[string]gop.File{
			"main.gop": file("var x = 1"),
		}, gop.FeatAll)
		_, typeInfo, _, _ := proj.TypeInfo()

		assert.Equal(t, ident, ResolveIdentFromNode(typeInfo, ident))
	})

	t.Run("BranchStmtWithFunction", func(t *testing.T) {
		labelIdent := &ast.Ident{
			NamePos: token.Pos(15),
			Name:    "label",
		}
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label:  labelIdent,
		}

		// Create ident that matches position (TokPos=10, "goto" has length 4, so End=14).
		gotoIdent := &ast.Ident{
			NamePos: token.Pos(10),
			Name:    "goto",
		}

		pkg := types.NewPackage("test", "test")
		labelVar := types.NewVar(token.NoPos, pkg, "label", types.Typ[types.Int])
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "goto", sig)

		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				labelIdent: labelVar, // Not a label.
			},
			Uses: map[*ast.Ident]types.Object{
				gotoIdent: fun, // Is a function.
			},
		}

		got := ResolveIdentFromNode(typeInfo, stmt)
		require.NotNil(t, got)
		assert.Equal(t, gotoIdent, got)
		assert.Equal(t, "goto", got.Name)
	})

	t.Run("BranchStmtWithoutFunction", func(t *testing.T) {
		labelIdent := &ast.Ident{
			NamePos: token.Pos(15),
			Name:    "label",
		}
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label:  labelIdent,
		}

		// Create ident that matches position but is not a function.
		gotoIdent := &ast.Ident{
			NamePos: token.Pos(10),
			Name:    "goto",
		}

		pkg := types.NewPackage("test", "test")
		labelVar := types.NewVar(token.NoPos, pkg, "label", types.Typ[types.Int])
		gotoVar := types.NewVar(token.NoPos, pkg, "goto", types.Typ[types.Int])

		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				labelIdent: labelVar, // Not a label.
			},
			Uses: map[*ast.Ident]types.Object{
				gotoIdent: gotoVar, // Not a function.
			},
		}

		assert.Nil(t, ResolveIdentFromNode(typeInfo, stmt))
	})

	t.Run("BranchStmtWithNoMatchingIdent", func(t *testing.T) {
		labelIdent := &ast.Ident{
			NamePos: token.Pos(15),
			Name:    "label",
		}
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label:  labelIdent,
		}

		pkg := types.NewPackage("test", "test")
		labelVar := types.NewVar(token.NoPos, pkg, "label", types.Typ[types.Int])

		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				labelIdent: labelVar, // Not a label.
			},
			Uses: map[*ast.Ident]types.Object{},
		}

		assert.Nil(t, ResolveIdentFromNode(typeInfo, stmt))
	})

	t.Run("UnsupportedNodeType", func(t *testing.T) {
		lit := &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"test"`,
		}

		proj := gop.NewProject(nil, map[string]gop.File{
			"main.gop": file("var x = 1"),
		}, gop.FeatAll)
		_, typeInfo, _, _ := proj.TypeInfo()

		assert.Nil(t, ResolveIdentFromNode(typeInfo, lit))
	})
}

func TestIdentsAtLine(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file(`
var x = 1
var y = 2

func test() {
	z := x + y
	println(z)
}
`),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	t.Run("VariableDeclarations", func(t *testing.T) {
		// Line 2: var x = 1
		idents := IdentsAtLine(proj, astFile, 2)
		require.Len(t, idents, 1)
		assert.Equal(t, "x", idents[0].Name)

		// Line 3: var y = 2
		idents = IdentsAtLine(proj, astFile, 3)
		require.Len(t, idents, 1)
		assert.Equal(t, "y", idents[0].Name)
	})

	t.Run("FunctionBody", func(t *testing.T) {
		// Line 6: z := x + y
		idents := IdentsAtLine(proj, astFile, 6)
		slices.SortFunc(idents, func(a, b *ast.Ident) int { return int(a.Pos()) - int(b.Pos()) })
		require.Len(t, idents, 3)
		assert.Equal(t, "z", idents[0].Name)
		assert.Equal(t, "x", idents[1].Name)
		assert.Equal(t, "y", idents[2].Name)
	})

	t.Run("EmptyLine", func(t *testing.T) {
		idents := IdentsAtLine(proj, astFile, 1) // Empty line
		assert.Empty(t, idents)
	})
}

func TestIdentAtPosition(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file(`
var longVarName = 1
var short = 2

func test() {
	result := longVarName + short
	println(result)
}
`),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
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
}

func TestDefIdentFor(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file(`
var x = 1
var y = x + 2

func test() {
	z := x + y
	println(z)
}
`),
	}, gop.FeatAll)

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
		ident := DefIdentFor(proj, xObj)
		require.NotNil(t, ident)
		assert.Equal(t, "x", ident.Name)

		ident = DefIdentFor(proj, yObj)
		require.NotNil(t, ident)
		assert.Equal(t, "y", ident.Name)
	})

	t.Run("NilObject", func(t *testing.T) {
		assert.Nil(t, DefIdentFor(proj, nil))
	})

	t.Run("UnknownObject", func(t *testing.T) {
		unknownObj := types.NewVar(token.NoPos, nil, "unknown", types.Typ[types.Int])
		assert.Nil(t, DefIdentFor(proj, unknownObj))
	})
}

func TestRefIdentsFor(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file(`
var x = 1
var y = x + 2

func test() {
	z := x + y
	println(z, x)
}
`),
	}, gop.FeatAll)

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
		refs := RefIdentsFor(proj, xObj)
		require.Len(t, refs, 3) // y = x + 2, z := x + y, println(z, x)
		for _, ref := range refs {
			assert.Equal(t, "x", ref.Name)
		}
	})

	t.Run("NilObject", func(t *testing.T) {
		assert.Nil(t, RefIdentsFor(proj, nil))
	})

	t.Run("UnknownObject", func(t *testing.T) {
		unknownObj := types.NewVar(token.NoPos, nil, "unknown", types.Typ[types.Int])
		assert.Nil(t, RefIdentsFor(proj, unknownObj))
	})
}
