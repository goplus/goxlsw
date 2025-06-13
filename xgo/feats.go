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

package xgo

import (
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/parser"
	"github.com/goplus/xgo/scanner"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgo/x/typesutil"
	"github.com/goplus/xgolsw/pkgdoc"
	"github.com/qiniu/x/errors"
)

type supportedFeat struct {
	feat     uint
	kind     string
	builder  any
	fileFeat bool
}

var supportedFeats = []supportedFeat{
	{FeatAST, "ast", buildAST, true},
	{FeatTypeInfo, "typeinfo", buildTypeInfo, false},
	{FeatPkgDoc, "pkgdoc", buildPkgDoc, false},
}

// -----------------------------------------------------------------------------

const parserMode = parser.ParseComments | parser.AllErrors

func buildAST(proj *Project, path string, file File) (ret any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parser panic: %v", r)
		}
	}()
	mode := parserMode
	if !strings.HasSuffix(path, ".xgo") && !strings.HasSuffix(path, ".gop") { // TODO(xsw): use xgomod
		mode |= parser.ParseGoPlusClass
	}
	f, e := parser.ParseEntry(proj.Fset, path, file.Content, parser.Config{
		Mode: mode,
	})
	return &astRet{f, e}, nil
}

type astRet struct {
	file *ast.File
	err  error
}

// AST returns the AST of an XGo source file.
func (p *Project) AST(path string) (file *ast.File, err error) {
	c, err := p.FileCache("ast", path)
	if err != nil {
		return
	}
	ret := c.(*astRet)
	return ret.file, ret.err
}

// ASTFiles returns the AST of all XGo source files.
func (p *Project) ASTFiles() (name string, ret []*ast.File, err error) {
	name, err = p.RangeASTFiles(func(_ string, f *ast.File) bool {
		ret = append(ret, f)
		return true
	})
	return
}

// -----------------------------------------------------------------------------

func defaultNewTypeInfo() *typesutil.Info {
	return &typesutil.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
}

func buildTypeInfo(proj *Project) (any, error) {
	var errs errors.List
	name, files, astErr := proj.ASTFiles()
	pkg := types.NewPackage(proj.Path, name)
	info := proj.NewTypeInfo()
	chk := typesutil.NewChecker(
		&types.Config{
			Error:    func(err error) { errs.Add(err) },
			Importer: proj.Importer,
		},
		&typesutil.Config{
			Types: pkg,
			Fset:  proj.Fset,
			Mod:   proj.Mod,
		},
		nil,
		info,
	)
	if e := chk.Files(nil, files); e != nil && len(errs) == 0 {
		errs.Add(e)
	}
	return &typeInfoRet{pkg, info, errs, astErr}, nil
}

type typeInfoRet struct {
	pkg    *types.Package
	info   *typesutil.Info
	typErr errors.List
	astErr error
}

// TypeInfo returns the type information of an XGo project.
func (p *Project) TypeInfo() (pkg *types.Package, info *typesutil.Info, err, astErr error) {
	c, err := p.Cache("typeinfo")
	if err != nil {
		return
	}
	ret := c.(*typeInfoRet)
	return ret.pkg, ret.info, ret.typErr.ToError(), ret.astErr
}

// -----------------------------------------------------------------------------

// RangeASTFiles iterates all XGo AST files.
func (p *Project) RangeASTFiles(fn func(path string, f *ast.File) bool) (name string, err error) {
	var errs scanner.ErrorList
	p.RangeFiles(func(path string) bool {
		switch filepath.Ext(path) { // TODO(xsw): use xgomod
		case ".spx", ".xgo", ".gop", ".gox":
			f, e := p.AST(path)
			if f != nil {
				if name == "" {
					name = f.Name.Name
				}
				if !fn(path, f) {
					return false
				}
			}
			if e != nil {
				if el, ok := e.(scanner.ErrorList); ok {
					errs = append(errs, el...)
				} else {
					errs.Add(token.Position{}, e.Error())
				}
			}
		}
		return true
	})
	err = errs.Err()
	return
}

// ASTPackage returns the AST package of an XGo project.
func (p *Project) ASTPackage() (pkg *ast.Package, err error) {
	pkg = &ast.Package{
		Files: make(map[string]*ast.File),
	}
	pkg.Name, err = p.RangeASTFiles(func(path string, f *ast.File) bool {
		pkg.Files[path] = f
		return true
	})
	return
}

// -----------------------------------------------------------------------------

func buildPkgDoc(proj *Project) (ret any, err error) {
	pkg, err := proj.ASTPackage()
	if err != nil {
		return
	}
	return pkgdoc.NewXGo(proj.Path, pkg), nil
}

// PkgDoc returns the package documentation of an XGo project.
func (p *Project) PkgDoc() (pkg *pkgdoc.PkgDoc, err error) {
	c, err := p.Cache("pkgdoc")
	if err != nil {
		return
	}
	return c.(*pkgdoc.PkgDoc), nil
}

// -----------------------------------------------------------------------------
