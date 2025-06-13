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

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"

	"github.com/goplus/xgolsw/pkgdoc"
	"golang.org/x/mod/module"
	"golang.org/x/tools/go/gcexportdata"

	_ "github.com/goplus/spx/v2"
	_ "github.com/qiniu/x"
)

// stdPkgPaths is the list of standard package paths to generate the exported symbols for.
var stdPkgPaths = []string{
	"builtin",

	"archive/tar",
	"archive/zip",
	"bufio",
	"bytes",
	"cmp",
	"compress/bzip2",
	"compress/flate",
	"compress/gzip",
	"compress/lzw",
	"compress/zlib",
	"context",
	"crypto",
	"crypto/aes",
	"crypto/cipher",
	"crypto/des",
	"crypto/dsa",
	"crypto/ecdh",
	"crypto/ecdsa",
	"crypto/ed25519",
	"crypto/elliptic",
	"crypto/hmac",
	"crypto/md5",
	"crypto/rand",
	"crypto/rc4",
	"crypto/rsa",
	"crypto/sha1",
	"crypto/sha256",
	"crypto/sha512",
	"crypto/subtle",
	"encoding",
	"encoding/asn1",
	"encoding/base32",
	"encoding/base64",
	"encoding/binary",
	"encoding/csv",
	"encoding/gob",
	"encoding/hex",
	"encoding/json",
	"encoding/pem",
	"encoding/xml",
	"errors",
	"fmt",
	"hash",
	"hash/adler32",
	"hash/crc32",
	"hash/crc64",
	"hash/fnv",
	"hash/maphash",
	"html",
	"html/template",
	"image",
	"image/color",
	"image/color/palette",
	"image/draw",
	"image/gif",
	"image/jpeg",
	"image/png",
	"io",
	"io/fs",
	"io/ioutil",
	"log",
	"log/slog",
	"maps",
	"math",
	"math/big",
	"math/bits",
	"math/cmplx",
	"math/rand",
	"mime",
	"net/http",
	"net/netip",
	"net/url",
	"os",
	"path",
	"path/filepath",
	"reflect",
	"regexp",
	"regexp/syntax",
	"runtime",
	"slices",
	"sort",
	"strconv",
	"strings",
	"sync",
	"sync/atomic",
	"syscall/js",
	"text/scanner",
	"text/tabwriter",
	"text/template",
	"text/template/parse",
	"time",
	"time/tzdata",
	"unicode",
	"unicode/utf16",
	"unicode/utf8",

	// See github.com/goplus/xgo/cl.newBuiltinDefault for the list of packages required by XGo builtins.
	"github.com/qiniu/x/osx",
	"github.com/qiniu/x/xgo",
	"github.com/qiniu/x/xgo/ng",
	"github.com/qiniu/x/stringutil",
	"github.com/qiniu/x/stringslice",

	"github.com/goplus/spx/v2",
	"github.com/goplus/spx/v2/pkg/gdspx/pkg/engine",
}

// generate generates the package data file containing the exported symbols of
// the given packages.
func generate(pkgPaths []string, outputFile string) error {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for _, pkgPath := range pkgPaths {
		buildPkg, err := build.Import(pkgPath, "", build.ImportComment)
		if err != nil {
			continue
		}

		var pkgName string
		if prefix, _, ok := module.SplitPathVersion(pkgPath); ok {
			pkgName = path.Base(prefix)
		} else {
			pkgName = path.Base(buildPkg.ImportPath)
		}

		var pkgDoc *pkgdoc.PkgDoc
		if pkgPath == "builtin" {
			astFile, err := parser.ParseFile(token.NewFileSet(), filepath.Join(buildPkg.Dir, "builtin.go"), nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("failed to parse builtin.go: %w", err)
			}

			pkgDoc = &pkgdoc.PkgDoc{
				Path:   pkgPath,
				Name:   pkgName,
				Vars:   make(map[string]string),
				Consts: make(map[string]string),
				Types:  make(map[string]*pkgdoc.TypeDoc),
				Funcs:  make(map[string]string),
			}
			for _, decl := range astFile.Decls {
				switch decl := decl.(type) {
				case *ast.GenDecl:
					switch decl.Tok {
					case token.VAR:
						for _, spec := range decl.Specs {
							switch spec := spec.(type) {
							case *ast.ValueSpec:
								for _, name := range spec.Names {
									doc := spec.Doc.Text()
									if doc == "" {
										doc = decl.Doc.Text()
									}
									pkgDoc.Vars[name.Name] = doc
								}
							default:
								return fmt.Errorf("unknown builtin gen decl spec: %T", spec)
							}
						}
					case token.CONST:
						for _, spec := range decl.Specs {
							switch spec := spec.(type) {
							case *ast.ValueSpec:
								for _, name := range spec.Names {
									doc := spec.Doc.Text()
									if doc == "" {
										doc = decl.Doc.Text()
									}
									pkgDoc.Consts[name.Name] = doc
								}
							default:
								return fmt.Errorf("unknown builtin gen decl spec: %T", spec)
							}
						}
					case token.IMPORT:
					case token.TYPE:
						for _, spec := range decl.Specs {
							switch spec := spec.(type) {
							case *ast.TypeSpec:
								doc := spec.Doc.Text()
								if doc == "" {
									doc = decl.Doc.Text()
								}
								pkgDoc.Types[spec.Name.Name] = &pkgdoc.TypeDoc{
									Doc: doc,
								}
							default:
								return fmt.Errorf("unknown builtin gen decl spec: %T", spec)
							}
						}
					default:
						return fmt.Errorf("unknown builtin gen decl token: %s", decl.Tok)
					}
				case *ast.FuncDecl:
					pkgDoc.Funcs[decl.Name.Name] = decl.Doc.Text()
				default:
					return fmt.Errorf("unknown builtin decl: %T", decl)
				}
			}
		} else {
			exportFile, err := execGo("list", "-trimpath", "-export", "-f", "{{.Export}}", pkgPath)
			if err != nil {
				return err
			}
			exportFile = bytes.TrimSpace(exportFile)
			if len(exportFile) == 0 {
				continue
			}

			f, err := os.Open(string(exportFile))
			if err != nil {
				return err
			}
			defer f.Close()

			r, err := gcexportdata.NewReader(f)
			if err != nil {
				return fmt.Errorf("failed to create package export reader: %w", err)
			}

			fset := token.NewFileSet()
			typesPkg, err := gcexportdata.Read(r, fset, make(map[string]*types.Package), pkgPath)
			if err != nil {
				return fmt.Errorf("failed to read package export data: %w", err)
			}
			if zf, err := zw.Create(pkgPath + ".pkgexport"); err != nil {
				return err
			} else if err := gcexportdata.Write(zf, fset, typesPkg); err != nil {
				return fmt.Errorf("failed to write optimized package export data: %w", err)
			}

			astPkgs, err := parser.ParseDir(token.NewFileSet(), buildPkg.Dir, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("failed to parse package: %w", err)
			}
			astPkg, ok := astPkgs[pkgName]
			if !ok {
				continue
			}

			pkgDoc = pkgdoc.NewGo(pkgPath, astPkg)
		}
		if zf, err := zw.Create(pkgPath + ".pkgdoc"); err != nil {
			return err
		} else if err := json.NewEncoder(zf).Encode(pkgDoc); err != nil {
			return fmt.Errorf("failed to encode package doc: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(outputFile, zipBuf.Bytes(), 0o644)
}

// execGo executes the given go command.
func execGo(args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("%w: %s", err, ee.Stderr)
		}
		return nil, fmt.Errorf("failed to execute go command: %w", err)
	}
	return output, nil
}

func main() {
	outputFile := flag.String("o", "pkgdata.zip", "output file")
	noStd := flag.Bool("no-std", false, "do not generate standard packages")
	flag.Parse()

	var pkgPaths []string
	if !*noStd {
		pkgPaths = stdPkgPaths
	}
	for _, pkgPath := range flag.Args() {
		if !slices.Contains(pkgPaths, pkgPath) {
			pkgPaths = append(pkgPaths, pkgPath)
		}
	}

	if err := generate(pkgPaths, *outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate package data: %v\n", err)
		os.Exit(1)
	}
}
