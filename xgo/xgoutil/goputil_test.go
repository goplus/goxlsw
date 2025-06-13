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
	"go/constant"
	"go/types"
	"slices"
	"testing"

	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func file(text string) xgo.File {
	return &xgo.FileImpl{Content: []byte(text)}
}

func TestRangeASTSpecs(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
		"main.xgo": file("type A = int"),
	}, xgo.FeatAll)
	RangeASTSpecs(proj, token.TYPE, func(spec ast.Spec) {
		ts := spec.(*ast.TypeSpec)
		if ts.Name.Name != "A" || ts.Assign == 0 {
			t.Fatal("RangeASTSpecs:", *ts)
		}
	})
}

func TestIsDefinedInClassFieldsDecl(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
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
	}, xgo.FeatAll)

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

	t.Run("DefinedInClassFields", func(t *testing.T) {
		assert.True(t, IsDefinedInClassFieldsDecl(proj, xObj))
	})

	t.Run("NotDefinedInClassFields", func(t *testing.T) {
		assert.False(t, IsDefinedInClassFieldsDecl(proj, zObj))
	})

	t.Run("NilObject", func(t *testing.T) {
		assert.False(t, IsDefinedInClassFieldsDecl(proj, nil))
	})
}

func TestWalkPathEnclosingInterval(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
		"main.xgo": file(`
var x = 1
func test() {
	y := x + 2
	println(y)
}
`),
	}, xgo.FeatAll)

	astFile, err := proj.AST("main.xgo")
	require.NoError(t, err)

	t.Run("WalkFunction", func(t *testing.T) {
		var funcDecl *ast.FuncDecl
		ast.Inspect(astFile, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "test" {
				funcDecl = fn
				return false
			}
			return true
		})
		require.NotNil(t, funcDecl)

		var nodes []ast.Node
		WalkPathEnclosingInterval(astFile, funcDecl.Body.Pos(), funcDecl.Body.End(), false, func(node ast.Node) bool {
			nodes = append(nodes, node)
			return true
		})
		require.NotEmpty(t, nodes)
		assert.IsType(t, &ast.BlockStmt{}, nodes[0])
		assert.IsType(t, &ast.File{}, nodes[len(nodes)-1])
	})

	t.Run("WalkSinglePosition", func(t *testing.T) {
		var identPos token.Pos
		ast.Inspect(astFile, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == "x" {
				identPos = ident.Pos()
				return false
			}
			return true
		})
		require.NotEqual(t, token.NoPos, identPos)

		var nodes []ast.Node
		WalkPathEnclosingInterval(astFile, identPos, identPos+1, false, func(node ast.Node) bool {
			nodes = append(nodes, node)
			return true
		})
		require.NotEmpty(t, nodes)
		assert.True(t, slices.ContainsFunc(nodes, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && ident.Name == "x" {
				return true
			}
			return false
		}))
	})

	t.Run("StopWalk", func(t *testing.T) {
		var funcDecl *ast.FuncDecl
		ast.Inspect(astFile, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "test" {
				funcDecl = fn
				return false
			}
			return true
		})
		require.NotNil(t, funcDecl)

		var nodes []ast.Node
		WalkPathEnclosingInterval(astFile, funcDecl.Body.Pos(), funcDecl.Body.End(), false, func(node ast.Node) bool {
			nodes = append(nodes, node)
			return false // Stop after first node.
		})
		assert.Len(t, nodes, 1)
	})

	t.Run("EmptyInterval", func(t *testing.T) {
		var nodes []ast.Node
		WalkPathEnclosingInterval(astFile, token.NoPos, token.NoPos, false, func(node ast.Node) bool {
			nodes = append(nodes, node)
			return true
		})
		assert.Len(t, nodes, 1) // Should still return at least the file node.
	})

	t.Run("WalkBackward", func(t *testing.T) {
		var funcDecl *ast.FuncDecl
		ast.Inspect(astFile, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "test" {
				funcDecl = fn
				return false
			}
			return true
		})
		require.NotNil(t, funcDecl)

		var forwardNodes []ast.Node
		WalkPathEnclosingInterval(astFile, funcDecl.Body.Pos(), funcDecl.Body.End(), false, func(node ast.Node) bool {
			forwardNodes = append(forwardNodes, node)
			return true
		})

		var backwardNodes []ast.Node
		WalkPathEnclosingInterval(astFile, funcDecl.Body.Pos(), funcDecl.Body.End(), true, func(node ast.Node) bool {
			backwardNodes = append(backwardNodes, node)
			return true
		})

		require.NotEmpty(t, forwardNodes)
		require.NotEmpty(t, backwardNodes)
		require.Equal(t, len(forwardNodes), len(backwardNodes))

		// Backward walk should return nodes in reverse order.
		for i := range forwardNodes {
			assert.Equal(t, forwardNodes[i], backwardNodes[len(backwardNodes)-1-i])
		}
	})
}

func TestToLowerCamelCase(t *testing.T) {
	t.Run("EmptyString", func(t *testing.T) {
		assert.Equal(t, "", ToLowerCamelCase(""))
	})

	t.Run("SingleCharacterUpper", func(t *testing.T) {
		assert.Equal(t, "a", ToLowerCamelCase("A"))
	})

	t.Run("SingleCharacterLower", func(t *testing.T) {
		assert.Equal(t, "a", ToLowerCamelCase("a"))
	})

	t.Run("PascalCase", func(t *testing.T) {
		assert.Equal(t, "pascalCase", ToLowerCamelCase("PascalCase"))
	})

	t.Run("AlreadyCamelCase", func(t *testing.T) {
		assert.Equal(t, "camelCase", ToLowerCamelCase("camelCase"))
	})

	t.Run("AllUpperCase", func(t *testing.T) {
		assert.Equal(t, "aLLUPPERCASE", ToLowerCamelCase("ALLUPPERCASE"))
	})

	t.Run("MixedCaseWithNumbers", func(t *testing.T) {
		assert.Equal(t, "test123Variable", ToLowerCamelCase("Test123Variable"))
	})

	t.Run("WithSpecialCharacters", func(t *testing.T) {
		assert.Equal(t, "test_Variable", ToLowerCamelCase("Test_Variable"))
	})
}

func TestStringLitOrConstValue(t *testing.T) {
	proj := xgo.NewProject(nil, map[string]xgo.File{
		"main.xgo": file(`
const strConst = "constant value"
const intConst = 42
var strVar = "variable value"

func test() {
	local := "local string"
	println("literal", strConst, strVar, local)
}
`),
	}, xgo.FeatAll)

	_, typeInfo, _, _ := proj.TypeInfo()
	require.NotNil(t, typeInfo)

	astFile, err := proj.AST("main.xgo")
	require.NoError(t, err)

	t.Run("StringLiteral", func(t *testing.T) {
		var strLit *ast.BasicLit
		ast.Inspect(astFile, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"literal"` {
				strLit = lit
				return false
			}
			return true
		})
		require.NotNil(t, strLit)

		tv := typeInfo.Types[strLit]
		value, ok := StringLitOrConstValue(strLit, tv)
		assert.True(t, ok)
		assert.Equal(t, "literal", value)
	})

	t.Run("StringConstant", func(t *testing.T) {
		var constIdent *ast.Ident
		ast.Inspect(astFile, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == "strConst" {
				if obj := typeInfo.Uses[ident]; obj != nil {
					constIdent = ident
					return false
				}
			}
			return true
		})
		require.NotNil(t, constIdent)

		tv := typeInfo.Types[constIdent]
		value, ok := StringLitOrConstValue(constIdent, tv)
		assert.True(t, ok)
		assert.Equal(t, "constant value", value)
	})

	t.Run("StringVariable", func(t *testing.T) {
		var varIdent *ast.Ident
		ast.Inspect(astFile, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == "strVar" {
				if obj := typeInfo.Uses[ident]; obj != nil {
					varIdent = ident
					return false
				}
			}
			return true
		})
		require.NotNil(t, varIdent)

		tv := typeInfo.Types[varIdent]
		value, ok := StringLitOrConstValue(varIdent, tv)
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	t.Run("NonStringLiteral", func(t *testing.T) {
		var intLit *ast.BasicLit
		ast.Inspect(astFile, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "42" {
				intLit = lit
				return false
			}
			return true
		})
		require.NotNil(t, intLit)

		tv := typeInfo.Types[intLit]
		value, ok := StringLitOrConstValue(intLit, tv)
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	t.Run("NonStringConstant", func(t *testing.T) {
		var intIdent *ast.Ident
		ast.Inspect(astFile, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == "intConst" {
				if obj := typeInfo.Uses[ident]; obj != nil {
					intIdent = ident
					return false
				}
			}
			return true
		})

		if intIdent != nil {
			tv := typeInfo.Types[intIdent]
			value, ok := StringLitOrConstValue(intIdent, tv)
			assert.False(t, ok)
			assert.Equal(t, "", value)
		}
	})

	t.Run("InvalidStringLiteral", func(t *testing.T) {
		invalidLit := &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"invalid\x"`, // Invalid escape sequence.
		}

		tv := types.TypeAndValue{}
		value, ok := StringLitOrConstValue(invalidLit, tv)
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	t.Run("UnsupportedExpression", func(t *testing.T) {
		binExpr := &ast.BinaryExpr{
			X:  &ast.BasicLit{Kind: token.STRING, Value: `"hello"`},
			Op: token.ADD,
			Y:  &ast.BasicLit{Kind: token.STRING, Value: `"world"`},
		}

		tv := types.TypeAndValue{}
		value, ok := StringLitOrConstValue(binExpr, tv)
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	t.Run("IdentWithNilValue", func(t *testing.T) {
		ident := &ast.Ident{Name: "test"}
		tv := types.TypeAndValue{Value: nil}
		value, ok := StringLitOrConstValue(ident, tv)
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	t.Run("IdentWithNonStringConstant", func(t *testing.T) {
		ident := &ast.Ident{Name: "test"}
		tv := types.TypeAndValue{Value: constant.MakeInt64(42)}
		value, ok := StringLitOrConstValue(ident, tv)
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})
}
