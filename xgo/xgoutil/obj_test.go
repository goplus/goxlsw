package xgoutil

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInBuiltinPkg(t *testing.T) {
	t.Run("NilObject", func(t *testing.T) {
		assert.False(t, IsInBuiltinPkg(nil))
	})

	t.Run("ObjectInBuiltinPackage", func(t *testing.T) {
		pkg := types.NewPackage("", "builtin")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.True(t, IsInBuiltinPkg(obj))
	})

	t.Run("ObjectInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInBuiltinPkg(obj))
	})

	t.Run("ObjectInStandardLibraryPackage", func(t *testing.T) {
		pkg := types.NewPackage("fmt", "fmt")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInBuiltinPkg(obj))
	})

	t.Run("ObjectInThirdPartyPackage", func(t *testing.T) {
		pkg := types.NewPackage("example.com/pkg", "pkg")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInBuiltinPkg(obj))
	})

	t.Run("ObjectWithNilPackage", func(t *testing.T) {
		obj := types.NewVar(token.NoPos, nil, "test", types.Typ[types.Int])
		assert.True(t, IsInBuiltinPkg(obj))
	})
}

func TestIsInMainPkg(t *testing.T) {
	t.Run("NilObject", func(t *testing.T) {
		assert.False(t, IsInMainPkg(nil))
	})

	t.Run("ObjectInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.True(t, IsInMainPkg(obj))
	})

	t.Run("ObjectInBuiltinPackage", func(t *testing.T) {
		pkg := types.NewPackage("", "builtin")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInMainPkg(obj))
	})

	t.Run("ObjectInStandardLibraryPackage", func(t *testing.T) {
		pkg := types.NewPackage("fmt", "fmt")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInMainPkg(obj))
	})

	t.Run("ObjectInThirdPartyPackage", func(t *testing.T) {
		pkg := types.NewPackage("example.com/pkg", "pkg")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInMainPkg(obj))
	})

	t.Run("ObjectWithNilPackage", func(t *testing.T) {
		obj := types.NewVar(token.NoPos, nil, "test", types.Typ[types.Int])
		assert.False(t, IsInMainPkg(obj))
	})

	t.Run("ObjectInPackageNamedMainButDifferentPath", func(t *testing.T) {
		pkg := types.NewPackage("example.com/main", "main")
		obj := types.NewVar(token.NoPos, pkg, "test", types.Typ[types.Int])
		assert.False(t, IsInMainPkg(obj))
	})
}

func TestIsExportedOrInMainPkg(t *testing.T) {
	t.Run("NilObject", func(t *testing.T) {
		assert.False(t, IsExportedOrInMainPkg(nil))
	})

	t.Run("ExportedObjectInNonMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("fmt", "fmt")
		obj := types.NewVar(token.NoPos, pkg, "ExportedVar", types.Typ[types.Int])
		assert.True(t, IsExportedOrInMainPkg(obj))
	})

	t.Run("UnexportedObjectInNonMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("fmt", "fmt")
		obj := types.NewVar(token.NoPos, pkg, "unexportedVar", types.Typ[types.Int])
		assert.False(t, IsExportedOrInMainPkg(obj))
	})

	t.Run("ExportedObjectInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewVar(token.NoPos, pkg, "ExportedVar", types.Typ[types.Int])
		assert.True(t, IsExportedOrInMainPkg(obj))
	})

	t.Run("UnexportedObjectInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewVar(token.NoPos, pkg, "unexportedVar", types.Typ[types.Int])
		assert.True(t, IsExportedOrInMainPkg(obj))
	})

	t.Run("ExportedObjectInBuiltinPackage", func(t *testing.T) {
		pkg := types.NewPackage("", "builtin")
		obj := types.NewVar(token.NoPos, pkg, "ExportedVar", types.Typ[types.Int])
		assert.True(t, IsExportedOrInMainPkg(obj))
	})

	t.Run("UnexportedObjectInBuiltinPackage", func(t *testing.T) {
		pkg := types.NewPackage("", "builtin")
		obj := types.NewVar(token.NoPos, pkg, "unexportedVar", types.Typ[types.Int])
		assert.False(t, IsExportedOrInMainPkg(obj))
	})

	t.Run("ObjectWithNilPackage", func(t *testing.T) {
		obj := types.NewVar(token.NoPos, nil, "ExportedVar", types.Typ[types.Int])
		assert.True(t, IsExportedOrInMainPkg(obj))
	})
}

func TestIsRenameable(t *testing.T) {
	t.Run("NilObject", func(t *testing.T) {
		assert.False(t, IsRenameable(nil))
	})

	t.Run("VariableInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewVar(token.Pos(1), pkg, "testVar", types.Typ[types.Int])
		assert.True(t, IsRenameable(obj))
	})

	t.Run("ConstantInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewConst(token.Pos(1), pkg, "testConst", types.Typ[types.Int], nil)
		assert.True(t, IsRenameable(obj))
	})

	t.Run("TypeNameInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewTypeName(token.Pos(1), pkg, "TestType", types.Typ[types.Int])
		assert.True(t, IsRenameable(obj))
	})

	t.Run("FunctionInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		obj := types.NewFunc(token.Pos(1), pkg, "testFunc", sig)
		assert.True(t, IsRenameable(obj))
	})

	t.Run("LabelInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewLabel(token.Pos(1), pkg, "testLabel")
		assert.True(t, IsRenameable(obj))
	})

	t.Run("PackageNameInMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewPkgName(token.Pos(1), pkg, "testPkg", nil)
		assert.False(t, IsRenameable(obj))
	})

	t.Run("VariableInNonMainPackage", func(t *testing.T) {
		pkg := types.NewPackage("fmt", "fmt")
		obj := types.NewVar(token.Pos(1), pkg, "testVar", types.Typ[types.Int])
		assert.False(t, IsRenameable(obj))
	})

	t.Run("VariableWithInvalidPosition", func(t *testing.T) {
		pkg := types.NewPackage("main", "main")
		obj := types.NewVar(token.NoPos, pkg, "testVar", types.Typ[types.Int])
		assert.False(t, IsRenameable(obj))
	})

	t.Run("VariableInUniverseScope", func(t *testing.T) {
		obj := types.Universe.Lookup("int")
		assert.False(t, IsRenameable(obj))
	})

	t.Run("BuiltinVariable", func(t *testing.T) {
		pkg := types.NewPackage("", "builtin")
		obj := types.NewVar(token.Pos(1), pkg, "testVar", types.Typ[types.Int])
		assert.False(t, IsRenameable(obj))
	})

	t.Run("ObjectWithNilPackage", func(t *testing.T) {
		obj := types.NewVar(token.Pos(1), nil, "testVar", types.Typ[types.Int])
		assert.False(t, IsRenameable(obj))
	})
}
