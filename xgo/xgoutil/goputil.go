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
	"iter"
	"slices"
	"strconv"

	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
)

// RangeASTSpecs iterates all XGo AST specs.
func RangeASTSpecs(proj *xgo.Project, tok token.Token, f func(spec ast.Spec)) {
	proj.RangeASTFiles(func(_ string, file *ast.File) bool {
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok && decl.Tok == tok {
				for _, spec := range decl.Specs {
					f(spec)
				}
			}
		}
		return true
	})
}

// IsDefinedInClassFieldsDecl reports whether the given object is defined in
// the class fields declaration of an AST file.
func IsDefinedInClassFieldsDecl(proj *xgo.Project, obj types.Object) bool {
	_, typeInfo, _, _ := proj.TypeInfo()
	defIdent := DefIdentFor(typeInfo, obj)
	if defIdent == nil {
		return false
	}
	astFile := NodeASTFile(proj, defIdent)
	if astFile == nil {
		return false
	}
	decl := astFile.ClassFieldsDecl()
	if decl == nil {
		return false
	}
	return defIdent.Pos() >= decl.Pos() && defIdent.End() <= decl.End()
}

// WalkPathEnclosingInterval calls walkFn for each node in the AST path
// enclosing the given [start, end) interval, starting from the innermost node
// and walking outward. The walk stops if walkFn returns false.
func WalkPathEnclosingInterval(root *ast.File, start, end token.Pos, backward bool, walkFn func(node ast.Node) bool) {
	path, _ := PathEnclosingInterval(root, start, end)
	var seq iter.Seq2[int, ast.Node]
	if backward {
		seq = slices.Backward(path)
	} else {
		seq = slices.All(path)
	}
	for _, node := range seq {
		if !walkFn(node) {
			break
		}
	}
}

// ToLowerCamelCase converts the first character of a Go identifier to lowercase.
func ToLowerCamelCase(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]|32) + s[1:]
}

// StringLitOrConstValue attempts to get the value from a string literal or
// constant. It returns the string value and true if successful, or empty string
// and false if the expression is not a string literal or constant, or if the
// value cannot be determined.
func StringLitOrConstValue(expr ast.Expr, tv types.TypeAndValue) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return v, true
	case *ast.Ident:
		if tv.Value != nil && tv.Value.Kind() == constant.String {
			// If it's a constant, we can get its value.
			return constant.StringVal(tv.Value), true
		}
		// There is nothing we can do for string variables.
		return "", false
	default:
		return "", false
	}
}
