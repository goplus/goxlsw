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
	"go/token"

	"github.com/goplus/gop/ast"
	"github.com/goplus/goxlsw/gop"
)

// PosFilename returns the filename for the given position.
func PosFilename(proj *gop.Project, pos token.Pos) string {
	return proj.Fset.Position(pos).Filename
}

// NodeFilename returns the filename for the given node.
func NodeFilename(proj *gop.Project, node ast.Node) string {
	return PosFilename(proj, node.Pos())
}

// PosTokenFile returns the token file for the given position.
func PosTokenFile(proj *gop.Project, pos token.Pos) *token.File {
	return proj.Fset.File(pos)
}

// NodeTokenPos returns the token position for the given node.
func NodeTokenFile(proj *gop.Project, node ast.Node) *token.File {
	return PosTokenFile(proj, node.Pos())
}

// PosASTFile returns the AST file for the given position.
func PosASTFile(proj *gop.Project, pos token.Pos) *ast.File {
	astPkg, _ := proj.ASTPackage()
	return astPkg.Files[PosFilename(proj, pos)]
}

// NodeASTFile returns the AST file for the given node.
func NodeASTFile(proj *gop.Project, node ast.Node) *ast.File {
	return PosASTFile(proj, node.Pos())
}
