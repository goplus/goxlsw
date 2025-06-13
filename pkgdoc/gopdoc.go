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

package pkgdoc

import (
	"path"
	"strings"

	xgoast "github.com/goplus/xgo/ast"
	xgotoken "github.com/goplus/xgo/token"
)

// NewXGo creates a new [PkgDoc] for an XGo package.
func NewXGo(pkgPath string, pkg *xgoast.Package) *PkgDoc {
	pkgDoc := &PkgDoc{
		Path:   pkgPath,
		Name:   pkg.Name,
		Vars:   make(map[string]string),
		Consts: make(map[string]string),
		Types:  make(map[string]*TypeDoc),
		Funcs:  make(map[string]string),
	}

	for _, astFile := range pkg.Files {
		if astFile.Doc != nil {
			pkgDoc.Doc = astFile.Doc.Text()
			break
		}
	}

	for spxFile, astFile := range pkg.Files {
		var spxBaseSelectorTypeName string
		if spxFileBaseName := path.Base(spxFile); spxFileBaseName == "main.spx" {
			spxBaseSelectorTypeName = "Game"
		} else {
			spxBaseSelectorTypeName = strings.TrimSuffix(spxFileBaseName, ".spx")
		}
		spxBaseSelectorTypeDoc := pkgDoc.typeDoc(spxBaseSelectorTypeName)

		var firstVarBlock *xgoast.GenDecl
		for _, decl := range astFile.Decls {
			switch decl := decl.(type) {
			case *xgoast.GenDecl:
				if firstVarBlock == nil && decl.Tok == xgotoken.VAR {
					firstVarBlock = decl
				}

				for _, spec := range decl.Specs {
					var doc string
					switch spec := spec.(type) {
					case *xgoast.ValueSpec:
						if spec.Doc != nil {
							doc = spec.Doc.Text()
						}
					case *xgoast.TypeSpec:
						if spec.Doc != nil {
							doc = spec.Doc.Text()
						}
					case *xgoast.ImportSpec:
						if spec.Doc != nil {
							doc = spec.Doc.Text()
						}
					}
					if doc == "" && decl.Doc != nil && len(decl.Specs) == 1 {
						doc = decl.Doc.Text()
					}

					switch spec := spec.(type) {
					case *xgoast.ValueSpec:
						for _, name := range spec.Names {
							switch decl.Tok {
							case xgotoken.VAR:
								if decl == firstVarBlock {
									spxBaseSelectorTypeDoc.Fields[name.Name] = doc
								} else {
									pkgDoc.Vars[name.Name] = doc
								}
							case xgotoken.CONST:
								pkgDoc.Consts[name.Name] = doc
							}
						}
					case *xgoast.TypeSpec:
						if structType, ok := spec.Type.(*xgoast.StructType); ok {
							typeDoc := pkgDoc.typeDoc(spec.Name.Name)
							typeDoc.Doc = doc
							for _, field := range structType.Fields.List {
								fieldDoc := ""
								if field.Doc != nil {
									fieldDoc = field.Doc.Text()
								}

								if len(field.Names) == 0 {
									ident, ok := field.Type.(*xgoast.Ident)
									if !ok {
										continue
									}
									typeDoc.Fields[ident.Name] = fieldDoc
								} else {
									for _, name := range field.Names {
										typeDoc.Fields[name.Name] = fieldDoc
									}
								}
							}
						}
					}
				}
			case *xgoast.FuncDecl:
				if decl.Shadow {
					continue
				}

				var doc string
				if decl.Doc != nil {
					doc = decl.Doc.Text()
				}

				var recvTypeDoc *TypeDoc
				if decl.Recv == nil {
					recvTypeDoc = spxBaseSelectorTypeDoc
				} else if len(decl.Recv.List) == 1 {
					recvType := decl.Recv.List[0].Type
					if star, ok := recvType.(*xgoast.StarExpr); ok {
						recvType = star.X
					}
					recvTypeName := recvType.(*xgoast.Ident).Name
					recvTypeDoc = pkgDoc.typeDoc(recvTypeName)
				}

				recvTypeDoc.Methods[decl.Name.Name] = doc
			}
		}
	}

	return pkgDoc
}
