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
	"slices"

	"github.com/goplus/gogen"
	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgo/x/typesutil"
)

// CreateCallExprFromBranchStmt attempts to create a call expression from a
// branch statement. This handles cases in spx where the `Sprite.Goto` method
// is intended to precede the goto statement.
func CreateCallExprFromBranchStmt(typeInfo *typesutil.Info, stmt *ast.BranchStmt) *ast.CallExpr {
	if typeInfo == nil || stmt == nil {
		return nil
	}
	if stmt.Tok != token.GOTO {
		// Currently, we only need to handle goto statements.
		return nil
	}

	// Skip if this is a real branch statement with an actual label object.
	if obj := typeInfo.ObjectOf(stmt.Label); obj == nil {
		return nil
	} else if _, ok := obj.(*types.Label); ok {
		return nil
	}

	// Performance note: This requires traversing the typeInfo.Uses map to locate
	// the function object, which is unavoidable since the AST still treats this
	// node as a branch statement rather than a call expression.
	stmtTokEnd := stmt.TokPos + token.Pos(len(stmt.Tok.String()))
	for ident, obj := range typeInfo.Uses {
		if ident.Pos() == stmt.TokPos && ident.End() == stmtTokEnd {
			if _, ok := obj.(*types.Func); ok {
				return &ast.CallExpr{
					Fun:  ident,
					Args: []ast.Expr{stmt.Label},
				}
			}
			break
		}
	}
	return nil
}

// FuncFromCallExpr returns the function object from a call expression.
func FuncFromCallExpr(typeInfo *typesutil.Info, expr *ast.CallExpr) *types.Func {
	if typeInfo == nil || expr == nil {
		return nil
	}

	var ident *ast.Ident
	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return nil
	}

	obj := typeInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}
	fun, _ := obj.(*types.Func)
	return fun
}

// WalkCallExprArgs walks the arguments of a call expression and calls the
// provided walkFn for each argument. It does nothing if the function is not
// found or if the function is XGo FuncEx type. The walk stops if walkFn
// returns false.
func WalkCallExprArgs(typeInfo *typesutil.Info, expr *ast.CallExpr, walkFn func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool) {
	fun := FuncFromCallExpr(typeInfo, expr)
	if fun == nil {
		return
	}
	sig := fun.Signature()
	if _, ok := gogen.CheckFuncEx(sig); ok {
		return
	}

	params := sig.Params()
	if IsMarkedAsXGoPackage(fun.Pkg()) {
		_, methodName, ok := SplitXGotMethodName(fun.Name(), false)
		if ok {
			var vars []*types.Var
			if _, ok := SplitXGoxFuncName(methodName); ok {
				typeParams := fun.Signature().TypeParams()
				if typeParams != nil {
					vars = slices.Grow(vars, typeParams.Len())
					for typeParam := range typeParams.TypeParams() {
						param := types.NewParam(token.NoPos, typeParam.Obj().Pkg(), typeParam.Obj().Name(), typeParam.Constraint().Underlying())
						vars = append(vars, param)
					}
				}
			}

			vars = slices.Grow(vars, params.Len()-1)
			for i := 1; i < params.Len(); i++ {
				vars = append(vars, params.At(i))
			}

			params = types.NewTuple(vars...)
		}
	}

	totalParams := params.Len()
	for i, arg := range expr.Args {
		paramIndex := i
		if paramIndex >= totalParams {
			if !sig.Variadic() || totalParams == 0 {
				break
			}
			paramIndex = totalParams - 1
		}

		if !walkFn(fun, params, paramIndex, arg, i) {
			break
		}
	}
}
