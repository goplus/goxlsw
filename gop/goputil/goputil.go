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
	"errors"
	"go/types"
	"slices"
	"sync"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"github.com/goplus/goxlsw/gop"
)

// InnermostScopeAt returns the innermost scope that contains the given
// position. It returns nil if not found.
func InnermostScopeAt(proj *gop.Project, pos token.Pos) *types.Scope {
	_, typeInfo, _, _ := proj.TypeInfo()

	astFile := PosASTFile(proj, pos)
	if astFile == nil {
		return nil
	}

	fileScope := typeInfo.Scopes[astFile]
	if fileScope == nil {
		return nil
	}

	innermostScope := fileScope
	for _, scope := range typeInfo.Scopes {
		if scope.Contains(pos) && fileScope.Contains(scope.Pos()) && innermostScope.Contains(scope.Pos()) {
			innermostScope = scope
		}
	}
	return innermostScope
}

// astFileLine represents an AST file line.
type astFileLine struct {
	astFile *ast.File
	line    int
}

// IdentsAtLine returns the identifiers at the given line in the given AST file.
func IdentsAtLine(proj *gop.Project, astFile *ast.File, line int) (idents []*ast.Ident) {
	const cacheKind = "goputil.IdentsAtLine"

	file := NodeFilename(proj, astFile)
	cache, err := proj.FileCache(cacheKind, file)
	if err != nil {
		if errors.Is(err, gop.ErrUnknownKind) {
			proj.InitFileCache(cacheKind, func(proj *gop.Project, path string, file gop.File) (any, error) {
				cache := &sync.Map{} // map[astFileLine][]*gopast.Ident
				return cache, nil
			})
			return IdentsAtLine(proj, astFile, line)
		}
		return nil
	}
	identsAtLines := cache.(*sync.Map)

	astFileLine := astFileLine{astFile: astFile, line: line}
	if identsAtLineIface, ok := identsAtLines.Load(astFileLine); ok {
		return identsAtLineIface.([]*ast.Ident)
	}
	defer func() {
		identsAtLines.Store(astFileLine, slices.Clip(idents))
	}()

	fset := proj.Fset
	astFilePos := fset.Position(astFile.Pos())
	collectIdentAtLine := func(ident *ast.Ident) {
		identPos := fset.Position(ident.Pos())
		if identPos.Filename == astFilePos.Filename && identPos.Line == line {
			idents = append(idents, ident)
		}
	}
	_, typeInfo, _, _ := proj.TypeInfo()
	for ident := range typeInfo.Defs {
		if IsShadow(proj, ident) {
			continue
		}
		collectIdentAtLine(ident)
	}
	for ident, obj := range typeInfo.Uses {
		if IsShadow(proj, DefIdentFor(proj, obj)) {
			continue
		}
		collectIdentAtLine(ident)
	}
	return
}

// IdentAtPosition returns the identifier at the given position in the given AST file.
func IdentAtPosition(proj *gop.Project, astFile *ast.File, position token.Position) *ast.Ident {
	var (
		bestIdent    *ast.Ident
		bestNodeSpan int
	)
	for _, ident := range IdentsAtLine(proj, astFile, position.Line) {
		identPos := proj.Fset.Position(ident.Pos())
		identEnd := proj.Fset.Position(ident.End())
		if position.Column < identPos.Column || position.Column > identEnd.Column {
			continue
		}

		nodeSpan := identEnd.Column - identPos.Column
		if bestIdent == nil || nodeSpan < bestNodeSpan {
			bestIdent = ident
			bestNodeSpan = nodeSpan
		}
	}
	return bestIdent
}

// DefIdentFor returns the identifier where the given object is defined.
func DefIdentFor(proj *gop.Project, obj types.Object) *ast.Ident {
	if obj == nil {
		return nil
	}
	_, typeInfo, _, _ := proj.TypeInfo()
	for ident, o := range typeInfo.Defs {
		if o == obj {
			return ident
		}
	}
	return nil
}

// RefIdentsFor returns all identifiers where the given object is referenced.
func RefIdentsFor(proj *gop.Project, obj types.Object) []*ast.Ident {
	if obj == nil {
		return nil
	}
	_, typeInfo, _, _ := proj.TypeInfo()
	var idents []*ast.Ident
	for ident, o := range typeInfo.Uses {
		if o == obj {
			idents = append(idents, ident)
		}
	}
	return idents
}

// ClassFieldsDecl returns the class fields declaration.
func ClassFieldsDecl(f *ast.File) *ast.GenDecl {
	if f.IsClass {
		for _, decl := range f.Decls {
			if g, ok := decl.(*ast.GenDecl); ok {
				if g.Tok == token.VAR {
					return g
				}
				continue
			}
			break
		}
	}
	return nil
}

// IsDefinedInClassFieldsDecl reports whether the given object is defined in
// the class fields declaration of an AST file.
func IsDefinedInClassFieldsDecl(proj *gop.Project, obj types.Object) bool {
	defIdent := DefIdentFor(proj, obj)
	if defIdent == nil {
		return false
	}
	astFile := NodeASTFile(proj, defIdent)
	if astFile == nil {
		return false
	}
	firstVarBlock := ClassFieldsDecl(astFile)
	if firstVarBlock == nil {
		return false
	}
	return defIdent.Pos() >= firstVarBlock.Pos() && defIdent.End() <= firstVarBlock.End()
}

// RangeASTSpecs iterates all Go+ AST specs.
func RangeASTSpecs(proj *gop.Project, tok token.Token, f func(spec ast.Spec)) {
	proj.RangeASTFiles(func(_ string, file *ast.File) {
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok && decl.Tok == tok {
				for _, spec := range decl.Specs {
					f(spec)
				}
			}
		}
	})
}

// IsShadow checks if the ident is shadowed.
func IsShadow(proj *gop.Project, ident *ast.Ident) (shadow bool) {
	proj.RangeASTFiles(func(_ string, file *ast.File) {
		if e := file.ShadowEntry; e != nil {
			if e.Name == ident {
				shadow = true
			}
		}
	})
	return
}

// PosFilename returns the filename for the given position.
func PosFilename(proj *gop.Project, pos token.Pos) string {
	return proj.Fset.Position(pos).Filename
}

// NodeFilename returns the filename for the given node.
func NodeFilename(proj *gop.Project, node ast.Node) string {
	return PosFilename(proj, node.Pos())
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

// PosTokenFile returns the token file for the given position.
func PosTokenFile(proj *gop.Project, pos token.Pos) *token.File {
	return proj.Fset.File(pos)
}

// NodeTokenPos returns the token position for the given node.
func NodeTokenFile(proj *gop.Project, node ast.Node) *token.File {
	return PosTokenFile(proj, node.Pos())
}
