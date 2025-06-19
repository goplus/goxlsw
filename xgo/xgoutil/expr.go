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
	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
)

// FilterExprAtPosition returns the Filter function at the given position in the given AST file.
func FilterExprAtPosition(proj *xgo.Project, astFile *ast.File, position token.Position) func(expr ast.Expr) (isBestPossibleMatch bool) {
	fset := proj.Fset
	astFilePosition := fset.Position(astFile.Pos())
	if astFilePosition.Filename != position.Filename {
		return nil
	}

	tokenFile := PosTokenFile(proj, astFile.Pos())
	if tokenFile == nil {
		return nil
	}
	if position.Line < 1 || position.Line > tokenFile.LineCount() {
		return nil
	}

	var (
		linePos = tokenFile.LineStart(position.Line)
		lineEnd token.Pos
	)
	if position.Line < tokenFile.LineCount() {
		lineEnd = tokenFile.LineStart(position.Line + 1)
	} else {
		lineEnd = token.Pos(tokenFile.Base() + tokenFile.Size())
	}

	checkIdent := func(expr ast.Expr) (isBestPossibleMatch bool) {
		exprPos := expr.Pos()
		if exprPos < linePos || exprPos >= lineEnd {
			return
		}
		exprPosPosition := fset.Position(exprPos)
		exprEndPosition := fset.Position(expr.End())
		if exprPosPosition.Column > position.Column || exprEndPosition.Column < position.Column {
			return
		}

		return true
	}

	return checkIdent
}
