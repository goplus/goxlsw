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
	"testing"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"github.com/goplus/goxlsw/gop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func file(text string) gop.File {
	return &gop.FileImpl{Content: []byte(text)}
}

func TestRangeASTSpecs(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("type A = int"),
	}, gop.FeatAll)
	RangeASTSpecs(proj, token.TYPE, func(spec ast.Spec) {
		ts := spec.(*ast.TypeSpec)
		if ts.Name.Name != "A" || ts.Assign == 0 {
			t.Fatal("RangeASTSpecs:", *ts)
		}
	})
}

func TestIsShadow(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("echo 100"),
	}, gop.FeatAll)
	f, err := proj.AST("main.gop")
	if err != nil {
		t.Fatal("AST:", err)
	}
	if !IsShadow(proj, f.ShadowEntry.Name) {
		t.Fatal("IsShadow: failed")
	}
}

func TestClassFieldsDecl_Basic(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gox": file(`import "a"; type T int; const pi=3.14; var x int`),
	}, gop.FeatAll)
	f, err := proj.AST("main.gox")
	if err != nil {
		t.Fatal("AST:", err)
	}
	if g := ClassFieldsDecl(f); g == nil || g.Tok != token.VAR {
		t.Fatal("ClassFieldsDecl: failed:", g)
	}
}

func TestClassFieldsDecl_NotFound(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gox": file(`import "a"; func f(); type T int; const pi=3.14; var x int`),
	}, gop.FeatAll)
	f, err := proj.AST("main.gox")
	if err != nil {
		t.Fatal("AST:", err)
	}
	if g := ClassFieldsDecl(f); g != nil {
		t.Fatal("ClassFieldsDecl: failed:", g)
	}
}

func TestInnermostScopeAt(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file(`
var x = 1

func test() {
	y := 2
	if true {
		z := 3
		println(x, y, z)
	}
}
`),
	}, gop.FeatAll)

	_, typeInfo, _, _ := proj.TypeInfo()
	require.NotNil(t, typeInfo)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)
	require.Len(t, astFile.Decls, 2)

	for _, tt := range []struct {
		name    string
		pos     token.Pos
		wantNil bool
		wantVar string
	}{
		{
			name:    "can see x",
			pos:     astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Pos(),
			wantVar: "x",
		},
		{
			name:    "can see y",
			pos:     astFile.Decls[1].(*ast.FuncDecl).Body.Pos(),
			wantVar: "y",
		},
		{
			name: "can see z",
			pos: func() token.Pos {
				body := astFile.Decls[1].(*ast.FuncDecl).Body
				for _, stmt := range body.List {
					if ifStmt, ok := stmt.(*ast.IfStmt); ok {
						return ifStmt.Body.Pos()
					}
				}
				return token.NoPos
			}(),
			wantVar: "z",
		},
		{
			name:    "not found",
			pos:     token.NoPos,
			wantNil: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scope := InnermostScopeAt(proj, tt.pos)
			if tt.wantNil {
				require.Nil(t, scope)
				return
			}
			require.NotNil(t, scope)

			if scope == typeInfo.Scopes[astFile] {
				scope = scope.Parent()
			}

			if tt.wantVar != "" {
				assert.NotNil(t, scope.Lookup(tt.wantVar))
			}
		})
	}
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

	t.Run("variable declarations", func(t *testing.T) {
		// Line 2: var x = 1
		idents := IdentsAtLine(proj, astFile, 2)
		require.Len(t, idents, 1)
		assert.Equal(t, "x", idents[0].Name)

		// Line 3: var y = 2
		idents = IdentsAtLine(proj, astFile, 3)
		require.Len(t, idents, 1)
		assert.Equal(t, "y", idents[0].Name)
	})

	t.Run("function body", func(t *testing.T) {
		// Line 6: z := x + y
		idents := IdentsAtLine(proj, astFile, 6)
		require.Len(t, idents, 3)
		assert.Equal(t, "z", idents[0].Name)
		assert.Equal(t, "x", idents[1].Name)
		assert.Equal(t, "y", idents[2].Name)
	})

	t.Run("empty line", func(t *testing.T) {
		idents := IdentsAtLine(proj, astFile, 1) // Empty line
		assert.Empty(t, idents)
	})

	t.Run("caching", func(t *testing.T) {
		// First call should populate cache
		idents1 := IdentsAtLine(proj, astFile, 6)
		require.NotEmpty(t, idents1)

		// Second call should use cached result
		idents2 := IdentsAtLine(proj, astFile, 6)
		require.Equal(t, idents1, idents2)
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

	// Get positions for all identifiers
	fset := proj.Fset
	pos := func(line, column int) token.Position {
		return token.Position{Filename: fset.Position(astFile.Pos()).Filename, Line: line, Column: column}
	}

	t.Run("exact match", func(t *testing.T) {
		// Line 2: var longVarName = 1
		ident := IdentAtPosition(proj, astFile, pos(2, 5)) // 'longVarName' start
		require.NotNil(t, ident)
		assert.Equal(t, "longVarName", ident.Name)

		// Line 3: var short = 2
		ident = IdentAtPosition(proj, astFile, pos(3, 5)) // 'short' start
		require.NotNil(t, ident)
		assert.Equal(t, "short", ident.Name)
	})

	t.Run("multiple identifiers on same line", func(t *testing.T) {
		// Find the function declaration
		var funcDecl *ast.FuncDecl
		for _, decl := range astFile.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok {
				funcDecl = fd
				break
			}
		}
		require.NotNil(t, funcDecl)

		// Find the assignment statement
		var assignStmt *ast.AssignStmt
		for _, stmt := range funcDecl.Body.List {
			if as, ok := stmt.(*ast.AssignStmt); ok {
				assignStmt = as
				break
			}
		}
		require.NotNil(t, assignStmt)

		// Get positions from AST nodes
		resultPos := fset.Position(assignStmt.Lhs[0].(*ast.Ident).Pos())
		longVarNamePos := fset.Position(assignStmt.Rhs[0].(*ast.BinaryExpr).X.(*ast.Ident).Pos())
		shortPos := fset.Position(assignStmt.Rhs[0].(*ast.BinaryExpr).Y.(*ast.Ident).Pos())

		// Test each identifier
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

	t.Run("no match", func(t *testing.T) {
		// Line 1: empty
		ident := IdentAtPosition(proj, astFile, pos(1, 1))
		assert.Nil(t, ident)

		// Line 2: var longVarName = 1 (after identifier)
		ident = IdentAtPosition(proj, astFile, pos(2, 20))
		assert.Nil(t, ident)
	})

	t.Run("boundary conditions", func(t *testing.T) {
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

	// Get all definitions from typeInfo
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

	t.Run("find definition", func(t *testing.T) {
		ident := DefIdentFor(proj, xObj)
		require.NotNil(t, ident)
		assert.Equal(t, "x", ident.Name)

		ident = DefIdentFor(proj, yObj)
		require.NotNil(t, ident)
		assert.Equal(t, "y", ident.Name)
	})

	t.Run("nil object", func(t *testing.T) {
		assert.Nil(t, DefIdentFor(proj, nil))
	})

	t.Run("unknown object", func(t *testing.T) {
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

	// Find x definition from Defs
	var xObj types.Object
	for ident, obj := range typeInfo.Defs {
		if ident.Name == "x" {
			xObj = obj
			break
		}
	}
	require.NotNil(t, xObj)

	t.Run("find references", func(t *testing.T) {
		refs := RefIdentsFor(proj, xObj)
		require.Len(t, refs, 3) // y = x + 2, z := x + y, println(z, x)
		for _, ref := range refs {
			assert.Equal(t, "x", ref.Name)
		}
	})

	t.Run("nil object", func(t *testing.T) {
		assert.Nil(t, RefIdentsFor(proj, nil))
	})

	t.Run("unknown object", func(t *testing.T) {
		unknownObj := types.NewVar(token.NoPos, nil, "unknown", types.Typ[types.Int])
		assert.Nil(t, RefIdentsFor(proj, unknownObj))
	})
}

func TestIsDefinedInClassFieldsDecl(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gox": file(`
var (
	x int
	y string
)

func test() {
	z := 1
	println(z)
}
`),
	}, gop.FeatAll)

	_, typeInfo, _, _ := proj.TypeInfo()
	require.NotNil(t, typeInfo)

	// Get objects from definitions
	var xObj, zObj types.Object
	for ident, obj := range typeInfo.Defs {
		switch ident.Name {
		case "x":
			xObj = obj
		case "z":
			zObj = obj
		}
	}
	require.NotNil(t, xObj)
	require.NotNil(t, zObj)

	t.Run("defined in class fields", func(t *testing.T) {
		assert.True(t, IsDefinedInClassFieldsDecl(proj, xObj))
	})

	t.Run("not defined in class fields", func(t *testing.T) {
		assert.False(t, IsDefinedInClassFieldsDecl(proj, zObj))
	})

	t.Run("nil object", func(t *testing.T) {
		assert.False(t, IsDefinedInClassFieldsDecl(proj, nil))
	})
}

func TestPosFilename(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xPos := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Pos()
	filename := PosFilename(proj, xPos)
	assert.NotEmpty(t, filename)
	assert.Contains(t, filename, "main.gop")
}

func TestNodeFilename(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xDecl := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0]
	filename := NodeFilename(proj, xDecl)
	assert.NotEmpty(t, filename)
	assert.Contains(t, filename, "main.gop")
}

func TestPosTokenFile(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xPos := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Pos()
	file := PosTokenFile(proj, xPos)
	assert.NotNil(t, file)
}

func TestNodeTokenFile(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xDecl := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0]
	file := NodeTokenFile(proj, xDecl)
	assert.NotNil(t, file)
}

func TestPosASTFile(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xPos := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Pos()
	file := PosASTFile(proj, xPos)
	assert.Equal(t, astFile, file)
}

func TestNodeASTFile(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xDecl := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0]
	file := NodeASTFile(proj, xDecl)
	assert.Equal(t, astFile, file)
}
