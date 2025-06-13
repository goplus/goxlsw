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
	"go/types"

	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgolsw/xgo"
)

// InnermostScopeAt returns the innermost scope that contains the given
// position. It returns nil if not found.
func InnermostScopeAt(proj *xgo.Project, pos token.Pos) *types.Scope {
	if !pos.IsValid() {
		return nil
	}
	_, typeInfo, _, _ := proj.TypeInfo()

	astFile := PosASTFile(proj, pos)
	if astFile == nil {
		return nil
	}

	var scope *types.Scope
	WalkPathEnclosingInterval(astFile, pos, pos, false, func(node ast.Node) bool {
		scope = typeInfo.Scopes[node]
		if scope == nil {
			// NOTE: If we have a FuncDecl but no direct scope, try to get the
			// scope from its FuncType (function parameter/local variable scope).
			if funcDecl, ok := node.(*ast.FuncDecl); ok {
				scope = typeInfo.Scopes[funcDecl.Type]
			}
		}
		return scope == nil // Stop at the first non-nil scope.
	})
	return scope
}
