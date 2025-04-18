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

package gop

import (
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/parser"
	"github.com/goplus/gop/scanner"
	"github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"github.com/goplus/goxlsw/pkgdoc"
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
	{FeatSpxResourceSet, "spxResourceSet", buildSpxResourceSet, false},
	{FeatSpxResourceRefs, "spxResourceRefs", buildSpxResourceRefs, false},
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
	if !strings.HasSuffix(path, ".gop") { // TODO(xsw): use gopmod
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

// AST returns the AST of a Go+ source file.
func (p *Project) AST(path string) (file *ast.File, err error) {
	c, err := p.FileCache("ast", path)
	if err != nil {
		return
	}
	ret := c.(*astRet)
	return ret.file, ret.err
}

// ASTFiles returns the AST of all Go+ source files.
func (p *Project) ASTFiles() (name string, ret []*ast.File, err error) {
	name, err = p.RangeASTFiles(func(_ string, f *ast.File) {
		ret = append(ret, f)
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

// TypeInfo returns the type information of a Go+ project.
func (p *Project) TypeInfo() (pkg *types.Package, info *typesutil.Info, err, astErr error) {
	c, err := p.Cache("typeinfo")
	if err != nil {
		return
	}
	ret := c.(*typeInfoRet)
	return ret.pkg, ret.info, ret.typErr.ToError(), ret.astErr
}

// -----------------------------------------------------------------------------

// RangeParsableFiles iterates all files can be parsed by Go+ AST parser.
func (p *Project) RangeParsableFiles(fn func(path string)) (name string, err error) {
	p.RangeFiles(func(path string) bool {
		switch filepath.Ext(path) { // TODO(xsw): use gopmod
		case ".spx", ".gop", ".gox":
			fn(path)
		}
		return true
	})
	return
}

// RangeASTFiles iterates all Go+ AST files.
func (p *Project) RangeASTFiles(fn func(path string, f *ast.File)) (name string, err error) {
	var errs scanner.ErrorList
	p.RangeParsableFiles(func(path string) {
		f, e := p.AST(path)
		if f != nil {
			if name == "" {
				name = f.Name.Name
			}
			fn(path, f)
		}
		if e != nil {
			if el, ok := e.(scanner.ErrorList); ok {
				errs = append(errs, el...)
			} else {
				errs.Add(token.Position{}, e.Error())
			}
		}
	})
	err = errs.Err()
	return
}

// ASTPackage returns the AST package of a Go+ project.
func (p *Project) ASTPackage() (pkg *ast.Package, err error) {
	pkg = &ast.Package{
		Files: make(map[string]*ast.File),
	}
	pkg.Name, err = p.RangeASTFiles(func(path string, f *ast.File) {
		pkg.Files[path] = f
	})
	return
}

// -----------------------------------------------------------------------------

func buildPkgDoc(proj *Project) (ret any, err error) {
	pkg, err := proj.ASTPackage()
	if err != nil {
		return
	}
	return pkgdoc.NewGop(proj.Path, pkg), nil
}

// PkgDoc returns the package documentation of a Go+ project.
func (p *Project) PkgDoc() (pkg *pkgdoc.PkgDoc, err error) {
	c, err := p.Cache("pkgdoc")
	if err != nil {
		return
	}
	return c.(*pkgdoc.PkgDoc), nil
}

// -----------------------------------------------------------------------------
