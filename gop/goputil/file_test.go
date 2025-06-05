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
	"testing"

	"github.com/goplus/gop/ast"
	"github.com/goplus/goxlsw/gop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPosFilename(t *testing.T) {
	proj := gop.NewProject(nil, map[string]gop.File{
		"main.gop": file("var x = 1"),
	}, gop.FeatAll)

	astFile, err := proj.AST("main.gop")
	require.NoError(t, err)

	xPos := astFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Pos()
	filename := PosFilename(proj, xPos)
	require.NotEmpty(t, filename)
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
	require.NotEmpty(t, filename)
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
