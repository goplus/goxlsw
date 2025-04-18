package gop

import (
	"go/constant"
	"go/types"
	"strconv"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
)

// UnwrapPointerType returns the underlying type of t. For pointer types, it
// returns the element type that the pointer points to. For non-pointer types,
// it returns the type unchanged.
func UnwrapPointerType(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// GetStringLitOrConstValue attempts to get the value from a string literal or
// constant. It returns the string value and true if successful, or empty string
// and false if the expression is not a string literal or constant, or if the
// value cannot be determined.
func GetStringLitOrConstValue(expr ast.Expr, tv types.TypeAndValue) (string, bool) {
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
