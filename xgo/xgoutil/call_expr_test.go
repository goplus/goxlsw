package xgoutil

import (
	"go/constant"
	"go/types"
	"testing"

	"github.com/goplus/xgo/ast"
	"github.com/goplus/xgo/token"
	"github.com/goplus/xgo/x/typesutil"
	"github.com/stretchr/testify/assert"
)

func TestCreateCallExprFromBranchStmt(t *testing.T) {
	t.Run("NilTypeInfo", func(t *testing.T) {
		stmt := &ast.BranchStmt{Tok: token.GOTO}
		assert.Nil(t, CreateCallExprFromBranchStmt(nil, stmt))
	})

	t.Run("NilStatement", func(t *testing.T) {
		typeInfo := &typesutil.Info{}
		assert.Nil(t, CreateCallExprFromBranchStmt(typeInfo, nil))
	})

	t.Run("NonGotoStatement", func(t *testing.T) {
		stmt := &ast.BranchStmt{
			Tok: token.BREAK,
		}
		typeInfo := &typesutil.Info{}
		assert.Nil(t, CreateCallExprFromBranchStmt(typeInfo, stmt))
	})

	t.Run("GotoStatementWithLabelObjectNil", func(t *testing.T) {
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label: &ast.Ident{
				NamePos: token.Pos(15),
				Name:    "label",
			},
		}
		typeInfo := &typesutil.Info{
			Defs: make(map[*ast.Ident]types.Object),
			Uses: make(map[*ast.Ident]types.Object),
		}
		assert.Nil(t, CreateCallExprFromBranchStmt(typeInfo, stmt))
	})

	t.Run("GotoStatementWithRealLabel", func(t *testing.T) {
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label: &ast.Ident{
				NamePos: token.Pos(15),
				Name:    "label",
			},
		}

		pkg := types.NewPackage("test", "test")
		label := types.NewLabel(token.NoPos, pkg, "label")
		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				stmt.Label: label,
			},
			Uses: make(map[*ast.Ident]types.Object),
		}
		assert.Nil(t, CreateCallExprFromBranchStmt(typeInfo, stmt))
	})

	t.Run("GotoStatementWithoutMatchingIdent", func(t *testing.T) {
		labelIdent := &ast.Ident{
			NamePos: token.Pos(15),
			Name:    "label",
		}
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label:  labelIdent,
		}

		pkg := types.NewPackage("test", "test")
		variable := types.NewVar(token.NoPos, pkg, "label", types.Typ[types.Int])
		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				labelIdent: variable, // Not a label, so it won't be skipped.
			},
			Uses: make(map[*ast.Ident]types.Object),
		}
		assert.Nil(t, CreateCallExprFromBranchStmt(typeInfo, stmt))
	})

	t.Run("GotoStatementWithMatchingIdentButNotFunc", func(t *testing.T) {
		labelIdent := &ast.Ident{
			NamePos: token.Pos(15),
			Name:    "label",
		}
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label:  labelIdent,
		}

		// Create ident that matches position (TokPos=10, "goto" has length 4, so End=14).
		ident := &ast.Ident{
			NamePos: token.Pos(10),
			Name:    "goto",
		}

		pkg := types.NewPackage("test", "test")
		labelVar := types.NewVar(token.NoPos, pkg, "label", types.Typ[types.Int])
		gotoVar := types.NewVar(token.NoPos, pkg, "goto", types.Typ[types.Int])
		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				labelIdent: labelVar, // Not a label.
			},
			Uses: map[*ast.Ident]types.Object{
				ident: gotoVar, // Not a function.
			},
		}

		assert.Nil(t, CreateCallExprFromBranchStmt(typeInfo, stmt))
	})

	t.Run("GotoStatementWithMatchingFuncIdent", func(t *testing.T) {
		labelIdent := &ast.Ident{
			NamePos: token.Pos(15),
			Name:    "label",
		}
		stmt := &ast.BranchStmt{
			Tok:    token.GOTO,
			TokPos: token.Pos(10),
			Label:  labelIdent,
		}

		// Create ident that matches position (TokPos=10, "goto" has length 4, so End=14).
		ident := &ast.Ident{
			NamePos: token.Pos(10),
			Name:    "goto",
		}

		pkg := types.NewPackage("test", "test")
		labelVar := types.NewVar(token.NoPos, pkg, "label", types.Typ[types.Int])
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "goto", sig)
		typeInfo := &typesutil.Info{
			Defs: map[*ast.Ident]types.Object{
				labelIdent: labelVar, // Not a label.
			},
			Uses: map[*ast.Ident]types.Object{
				ident: fun, // Is a function.
			},
		}

		got := CreateCallExprFromBranchStmt(typeInfo, stmt)
		assert.NotNil(t, got)
		assert.Equal(t, ident, got.Fun)
		assert.Len(t, got.Args, 1)
		assert.Equal(t, stmt.Label, got.Args[0])
	})
}

func TestFuncFromCallExpr(t *testing.T) {
	t.Run("NilCallExpr", func(t *testing.T) {
		assert.Nil(t, FuncFromCallExpr(nil, nil))
	})

	t.Run("NilTypeInfo", func(t *testing.T) {
		expr := &ast.CallExpr{
			Fun: &ast.Ident{Name: "test"},
		}
		assert.Nil(t, FuncFromCallExpr(nil, expr))
	})

	t.Run("CallExprWithUnknownFunType", func(t *testing.T) {
		expr := &ast.CallExpr{
			Fun: &ast.BasicLit{
				Kind:  token.STRING,
				Value: "\"test\"",
			},
		}
		typeInfo := &typesutil.Info{}
		assert.Nil(t, FuncFromCallExpr(typeInfo, expr))
	})

	t.Run("CallExprWithIdentFun", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		expr := &ast.CallExpr{
			Fun: ident,
		}

		pkg := types.NewPackage("test", "test")
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "testFunc", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		got := FuncFromCallExpr(typeInfo, expr)
		assert.NotNil(t, got)
		assert.Equal(t, fun, got)
	})

	t.Run("CallExprWithSelectorExprFun", func(t *testing.T) {
		sel := &ast.Ident{Name: "Method"}
		expr := &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "obj"},
				Sel: sel,
			},
		}

		pkg := types.NewPackage("test", "test")
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "Method", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				sel: fun,
			},
		}

		got := FuncFromCallExpr(typeInfo, expr)
		assert.NotNil(t, got)
		assert.Equal(t, fun, got)
	})

	t.Run("CallExprWithIdentFunButNotFunc", func(t *testing.T) {
		ident := &ast.Ident{Name: "testVar"}
		expr := &ast.CallExpr{
			Fun: ident,
		}

		pkg := types.NewPackage("test", "test")
		variable := types.NewVar(token.NoPos, pkg, "testVar", types.Typ[types.Int])

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: variable,
			},
		}

		assert.Nil(t, FuncFromCallExpr(typeInfo, expr))
	})

	t.Run("CallExprWithNilObject", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		expr := &ast.CallExpr{
			Fun: ident,
		}

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{},
		}

		assert.Nil(t, FuncFromCallExpr(typeInfo, expr))
	})
}

func TestWalkCallExprArgs(t *testing.T) {
	t.Run("NilCallExpr", func(t *testing.T) {
		walkCalled := false
		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalled = true
			return true
		}

		WalkCallExprArgs(nil, nil, walkFn)
		assert.False(t, walkCalled)
	})

	t.Run("CallExprWithNilFunction", func(t *testing.T) {
		expr := &ast.CallExpr{
			Fun: &ast.Ident{Name: "unknown"},
		}

		walkCalled := false
		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalled = true
			return true
		}

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{},
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)
		assert.False(t, walkCalled)
	})

	t.Run("CallExprWithSimpleFunction", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		arg1 := &ast.Ident{Name: "arg1"}
		arg2 := &ast.Ident{Name: "arg2"}
		expr := &ast.CallExpr{
			Fun:  ident,
			Args: []ast.Expr{arg1, arg2},
		}

		pkg := types.NewPackage("test", "test")
		param1 := types.NewParam(token.NoPos, pkg, "p1", types.Typ[types.Int])
		param2 := types.NewParam(token.NoPos, pkg, "p2", types.Typ[types.String])
		params := types.NewTuple(param1, param2)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "testFunc", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		var walkCalls []struct {
			paramIndex int
			argIndex   int
			arg        ast.Expr
		}

		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalls = append(walkCalls, struct {
				paramIndex int
				argIndex   int
				arg        ast.Expr
			}{paramIndex, argIndex, arg})
			return true
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)

		assert.Len(t, walkCalls, 2)
		assert.Equal(t, 0, walkCalls[0].paramIndex)
		assert.Equal(t, 0, walkCalls[0].argIndex)
		assert.Equal(t, arg1, walkCalls[0].arg)
		assert.Equal(t, 1, walkCalls[1].paramIndex)
		assert.Equal(t, 1, walkCalls[1].argIndex)
		assert.Equal(t, arg2, walkCalls[1].arg)
	})

	t.Run("CallExprWithVariadicFunction", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		arg1 := &ast.Ident{Name: "arg1"}
		arg2 := &ast.Ident{Name: "arg2"}
		arg3 := &ast.Ident{Name: "arg3"}
		expr := &ast.CallExpr{
			Fun:  ident,
			Args: []ast.Expr{arg1, arg2, arg3},
		}

		pkg := types.NewPackage("test", "test")
		param1 := types.NewParam(token.NoPos, pkg, "p1", types.Typ[types.Int])
		variadicParam := types.NewParam(token.NoPos, pkg, "args", types.NewSlice(types.Typ[types.String]))
		params := types.NewTuple(param1, variadicParam)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, true)
		fun := types.NewFunc(token.NoPos, pkg, "testFunc", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		var walkCalls []struct {
			paramIndex int
			argIndex   int
		}

		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalls = append(walkCalls, struct {
				paramIndex int
				argIndex   int
			}{paramIndex, argIndex})
			return true
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)

		assert.Len(t, walkCalls, 3)
		assert.Equal(t, 0, walkCalls[0].paramIndex) // First param
		assert.Equal(t, 0, walkCalls[0].argIndex)
		assert.Equal(t, 1, walkCalls[1].paramIndex) // Variadic param
		assert.Equal(t, 1, walkCalls[1].argIndex)
		assert.Equal(t, 1, walkCalls[2].paramIndex) // Still variadic param
		assert.Equal(t, 2, walkCalls[2].argIndex)
	})

	t.Run("CallExprStopsWhenWalkFnReturnsFalse", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		arg1 := &ast.Ident{Name: "arg1"}
		arg2 := &ast.Ident{Name: "arg2"}
		expr := &ast.CallExpr{
			Fun:  ident,
			Args: []ast.Expr{arg1, arg2},
		}

		pkg := types.NewPackage("test", "test")
		param1 := types.NewParam(token.NoPos, pkg, "p1", types.Typ[types.Int])
		param2 := types.NewParam(token.NoPos, pkg, "p2", types.Typ[types.String])
		params := types.NewTuple(param1, param2)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "testFunc", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		walkCallCount := 0
		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCallCount++
			return false // Stop after first call.
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)
		assert.Equal(t, 1, walkCallCount)
	})

	t.Run("CallExprWithXGoPackageXGotMethod", func(t *testing.T) {
		ident := &ast.Ident{Name: "Gopt_Sprite_Move"}
		arg1 := &ast.Ident{Name: "arg1"}
		expr := &ast.CallExpr{
			Fun:  ident,
			Args: []ast.Expr{arg1},
		}

		pkg := types.NewPackage("test", "test")
		scope := pkg.Scope()
		scope.Insert(types.NewConst(0, pkg, XGoPackage, types.Typ[types.UntypedBool], constant.MakeBool(true)))

		recv := types.NewParam(token.NoPos, pkg, "recv", types.Typ[types.Int])
		param1 := types.NewParam(token.NoPos, pkg, "p1", types.Typ[types.Int])
		params := types.NewTuple(recv, param1)
		sig := types.NewSignatureType(recv, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "Gopt_Sprite_Move", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		var walkCalls []struct {
			paramIndex int
			argIndex   int
		}

		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalls = append(walkCalls, struct {
				paramIndex int
				argIndex   int
			}{paramIndex, argIndex})
			return true
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)

		// Should skip the receiver parameter.
		assert.Len(t, walkCalls, 1)
		assert.Equal(t, 0, walkCalls[0].paramIndex) // First non-receiver param index in new tuple.
		assert.Equal(t, 0, walkCalls[0].argIndex)   // First arg.
	})

	t.Run("CallExprWithMoreArgsThanParams", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		arg1 := &ast.Ident{Name: "arg1"}
		arg2 := &ast.Ident{Name: "arg2"}
		arg3 := &ast.Ident{Name: "arg3"}
		expr := &ast.CallExpr{
			Fun:  ident,
			Args: []ast.Expr{arg1, arg2, arg3},
		}

		pkg := types.NewPackage("test", "test")
		param1 := types.NewParam(token.NoPos, pkg, "p1", types.Typ[types.Int])
		params := types.NewTuple(param1)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "testFunc", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		var walkCalls []struct {
			paramIndex int
			argIndex   int
		}

		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalls = append(walkCalls, struct {
				paramIndex int
				argIndex   int
			}{paramIndex, argIndex})
			return true
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)

		// Should only process one argument since function is not variadic.
		assert.Len(t, walkCalls, 1)
		assert.Equal(t, 0, walkCalls[0].paramIndex)
		assert.Equal(t, 0, walkCalls[0].argIndex)
	})

	t.Run("CallExprWithNoParams", func(t *testing.T) {
		ident := &ast.Ident{Name: "testFunc"}
		arg1 := &ast.Ident{Name: "arg1"}
		expr := &ast.CallExpr{
			Fun:  ident,
			Args: []ast.Expr{arg1},
		}

		pkg := types.NewPackage("test", "test")
		params := types.NewTuple()
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "testFunc", sig)

		typeInfo := &typesutil.Info{
			Uses: map[*ast.Ident]types.Object{
				ident: fun,
			},
		}

		walkCalled := false
		walkFn := func(fun *types.Func, params *types.Tuple, paramIndex int, arg ast.Expr, argIndex int) bool {
			walkCalled = true
			return true
		}

		WalkCallExprArgs(typeInfo, expr, walkFn)

		// Should not call walkFn since function has no parameters and more args than params.
		assert.False(t, walkCalled)
	})
}
