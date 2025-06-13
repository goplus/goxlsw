package xgoutil

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsXgotMethodName(t *testing.T) {
	t.Run("ValidXGotMethodName", func(t *testing.T) {
		assert.True(t, IsXGotMethodName("Gopt_Type_Method"))
	})

	t.Run("ValidXGotMethodNameWithUnderscore", func(t *testing.T) {
		assert.True(t, IsXGotMethodName("Gopt_MyType_MyMethod"))
	})

	t.Run("InvalidPrefix", func(t *testing.T) {
		assert.False(t, IsXGotMethodName("Type_Method"))
	})

	t.Run("InvalidXGoxPrefix", func(t *testing.T) {
		assert.False(t, IsXGotMethodName("Gopx_Type_Method"))
	})

	t.Run("NoUnderscore", func(t *testing.T) {
		assert.False(t, IsXGotMethodName("GoptTypeMethod"))
	})

	t.Run("OnlyPrefix", func(t *testing.T) {
		assert.False(t, IsXGotMethodName("Gopt_"))
	})

	t.Run("OnlyPrefixAndType", func(t *testing.T) {
		assert.False(t, IsXGotMethodName("Gopt_Type"))
	})

	t.Run("EmptyString", func(t *testing.T) {
		assert.False(t, IsXGotMethodName(""))
	})
}

func TestSplitXGotMethodName(t *testing.T) {
	t.Run("ValidXGotMethodName", func(t *testing.T) {
		recvTypeName, methodName, ok := SplitXGotMethodName("Gopt_Type_Method", false)
		assert.True(t, ok)
		assert.Equal(t, "Type", recvTypeName)
		assert.Equal(t, "Method", methodName)
	})

	t.Run("ValidXGotMethodNameWithXGoxPrefix", func(t *testing.T) {
		recvTypeName, methodName, ok := SplitXGotMethodName("Gopt_Type_Gopx_Method", false)
		assert.True(t, ok)
		assert.Equal(t, "Type", recvTypeName)
		assert.Equal(t, "Gopx_Method", methodName)
	})

	t.Run("ValidXGotMethodNameTrimXGox", func(t *testing.T) {
		recvTypeName, methodName, ok := SplitXGotMethodName("Gopt_Type_Gopx_Method", true)
		assert.True(t, ok)
		assert.Equal(t, "Type", recvTypeName)
		assert.Equal(t, "Method", methodName)
	})

	t.Run("InvalidPrefix", func(t *testing.T) {
		_, _, ok := SplitXGotMethodName("Type_Method", false)
		assert.False(t, ok)
	})

	t.Run("NoUnderscore", func(t *testing.T) {
		_, _, ok := SplitXGotMethodName("GoptTypeMethod", false)
		assert.False(t, ok)
	})

	t.Run("OnlyPrefix", func(t *testing.T) {
		_, _, ok := SplitXGotMethodName("Gopt_", false)
		assert.False(t, ok)
	})

	t.Run("OnlyPrefixAndType", func(t *testing.T) {
		_, _, ok := SplitXGotMethodName("Gopt_Type", false)
		assert.False(t, ok)
	})

	t.Run("EmptyString", func(t *testing.T) {
		_, _, ok := SplitXGotMethodName("", false)
		assert.False(t, ok)
	})

	t.Run("MultipleUnderscores", func(t *testing.T) {
		recvTypeName, methodName, ok := SplitXGotMethodName("Gopt_MyType_My_Method", false)
		assert.True(t, ok)
		assert.Equal(t, "MyType", recvTypeName)
		assert.Equal(t, "My_Method", methodName)
	})
}

func TestSplitXGoxFuncName(t *testing.T) {
	t.Run("ValidXGoxFuncName", func(t *testing.T) {
		funcName, ok := SplitXGoxFuncName("Gopx_Method")
		assert.True(t, ok)
		assert.Equal(t, "Method", funcName)
	})

	t.Run("ValidXGoxFuncNameWithUnderscores", func(t *testing.T) {
		funcName, ok := SplitXGoxFuncName("Gopx_My_Method")
		assert.True(t, ok)
		assert.Equal(t, "My_Method", funcName)
	})

	t.Run("InvalidPrefix", func(t *testing.T) {
		_, ok := SplitXGoxFuncName("Method")
		assert.False(t, ok)
	})

	t.Run("InvalidXGotPrefix", func(t *testing.T) {
		_, ok := SplitXGoxFuncName("Gopt_Method")
		assert.False(t, ok)
	})

	t.Run("OnlyPrefix", func(t *testing.T) {
		funcName, ok := SplitXGoxFuncName("Gopx_")
		assert.True(t, ok)
		assert.Equal(t, "", funcName)
	})

	t.Run("EmptyString", func(t *testing.T) {
		_, ok := SplitXGoxFuncName("")
		assert.False(t, ok)
	})
}

func TestParseXGoFuncName(t *testing.T) {
	t.Run("RegularFunctionName", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("MyFunction")
		assert.Equal(t, "myFunction", parsedName)
		assert.Nil(t, overloadID)
	})

	t.Run("OverloadedFunctionName", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("MyFunction__a")
		assert.Equal(t, "myFunction", parsedName)
		assert.NotNil(t, overloadID)
		assert.Equal(t, "a", *overloadID)
	})

	t.Run("OverloadedFunctionNameWithNumber", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("MyFunction__1")
		assert.Equal(t, "myFunction", parsedName)
		assert.NotNil(t, overloadID)
		assert.Equal(t, "1", *overloadID)
	})

	t.Run("OverloadedFunctionNameWithZero", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("MyFunction__0")
		assert.Equal(t, "myFunction", parsedName)
		assert.NotNil(t, overloadID)
		assert.Equal(t, "0", *overloadID)
	})

	t.Run("FunctionNameWithSingleUnderscore", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("My_Function")
		assert.Equal(t, "my_Function", parsedName)
		assert.Nil(t, overloadID)
	})

	t.Run("FunctionNameWithDoubleUnderscoreButInvalidSuffix", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("MyFunction__AA")
		assert.Equal(t, "myFunction__AA", parsedName)
		assert.Nil(t, overloadID)
	})

	t.Run("EmptyString", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("")
		assert.Equal(t, "", parsedName)
		assert.Nil(t, overloadID)
	})

	t.Run("PascalCaseFunction", func(t *testing.T) {
		parsedName, overloadID := ParseXGoFuncName("PascalCaseFunction")
		assert.Equal(t, "pascalCaseFunction", parsedName)
		assert.Nil(t, overloadID)
	})
}

func TestIsXGoOverloadedFuncName(t *testing.T) {
	t.Run("ValidOverloadedName", func(t *testing.T) {
		assert.True(t, IsXGoOverloadedFuncName("MyFunction__a"))
	})

	t.Run("ValidOverloadedNameWithNumber", func(t *testing.T) {
		assert.True(t, IsXGoOverloadedFuncName("MyFunction__1"))
	})

	t.Run("ValidOverloadedNameWithZero", func(t *testing.T) {
		assert.True(t, IsXGoOverloadedFuncName("MyFunction__0"))
	})

	t.Run("RegularFunctionName", func(t *testing.T) {
		assert.False(t, IsXGoOverloadedFuncName("MyFunction"))
	})

	t.Run("FunctionNameWithSingleUnderscore", func(t *testing.T) {
		assert.False(t, IsXGoOverloadedFuncName("My_Function"))
	})

	t.Run("FunctionNameWithDoubleUnderscoreButInvalidSuffix", func(t *testing.T) {
		assert.False(t, IsXGoOverloadedFuncName("MyFunction__AA"))
	})

	t.Run("FunctionNameWithDoubleUnderscoreButEmptySuffix", func(t *testing.T) {
		assert.False(t, IsXGoOverloadedFuncName("MyFunction__"))
	})

	t.Run("EmptyString", func(t *testing.T) {
		assert.False(t, IsXGoOverloadedFuncName(""))
	})

	t.Run("OnlyDoubleUnderscore", func(t *testing.T) {
		assert.False(t, IsXGoOverloadedFuncName("__"))
	})
}

func TestIsXGoOverloadableFunc(t *testing.T) {
	t.Run("RegularFunction", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.False(t, IsXGoOverloadableFunc(fun))
	})

	t.Run("FunctionWithIntParameter", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		params := types.NewTuple(types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int]))
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.False(t, IsXGoOverloadableFunc(fun))
	})

	t.Run("FunctionWithMultipleParameters", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		params := types.NewTuple(
			types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int]),
			types.NewParam(token.NoPos, pkg, "y", types.Typ[types.String]),
		)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.False(t, IsXGoOverloadableFunc(fun))
	})
}

func TestIsUnexpandableXGoOverloadableFunc(t *testing.T) {
	t.Run("RegularFunction", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.False(t, IsUnexpandableXGoOverloadableFunc(fun))
	})

	t.Run("FunctionWithIntParameter", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		params := types.NewTuple(types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int]))
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.False(t, IsUnexpandableXGoOverloadableFunc(fun))
	})

	t.Run("FunctionWithMultipleParameters", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		params := types.NewTuple(
			types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int]),
			types.NewParam(token.NoPos, pkg, "y", types.Typ[types.String]),
		)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.False(t, IsUnexpandableXGoOverloadableFunc(fun))
	})
}

func TestExpandXGoOverloadableFunc(t *testing.T) {
	t.Run("RegularFunction", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.Nil(t, ExpandXGoOverloadableFunc(fun))
	})

	t.Run("FunctionWithIntParameter", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		params := types.NewTuple(types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int]))
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.Nil(t, ExpandXGoOverloadableFunc(fun))
	})

	t.Run("FunctionWithMultipleParameters", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		params := types.NewTuple(
			types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int]),
			types.NewParam(token.NoPos, pkg, "y", types.Typ[types.String]),
		)
		sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
		fun := types.NewFunc(token.NoPos, pkg, "TestFunc", sig)
		assert.Nil(t, ExpandXGoOverloadableFunc(fun))
	})
}
