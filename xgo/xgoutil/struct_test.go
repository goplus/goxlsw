package xgoutil

import (
	"go/constant"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const spxPkgPath = "github.com/goplus/spx/v2"

func TestIsNamedStructType(t *testing.T) {
	t.Run("NilNamedType", func(t *testing.T) {
		var named *types.Named
		assert.False(t, IsNamedStructType(named))
	})

	t.Run("StructType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		structType := types.NewStruct([]*types.Var{
			types.NewField(token.NoPos, pkg, "field1", types.Typ[types.String], false),
			types.NewField(token.NoPos, pkg, "field2", types.Typ[types.Int], false),
		}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "TestStruct", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.True(t, IsNamedStructType(named))
	})

	t.Run("EmptyStructType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "EmptyStruct", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.True(t, IsNamedStructType(named))
	})

	t.Run("InterfaceType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		interfaceType := types.NewInterfaceType([]*types.Func{}, []types.Type{})
		typeName := types.NewTypeName(token.NoPos, pkg, "TestInterface", interfaceType)
		named := types.NewNamed(typeName, interfaceType, nil)
		assert.False(t, IsNamedStructType(named))
	})

	t.Run("BasicType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		typeName := types.NewTypeName(token.NoPos, pkg, "TestInt", types.Typ[types.Int])
		named := types.NewNamed(typeName, types.Typ[types.Int], nil)
		assert.False(t, IsNamedStructType(named))
	})

	t.Run("SliceType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		sliceType := types.NewSlice(types.Typ[types.String])
		typeName := types.NewTypeName(token.NoPos, pkg, "TestSlice", sliceType)
		named := types.NewNamed(typeName, sliceType, nil)
		assert.False(t, IsNamedStructType(named))
	})

	t.Run("PointerType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		pointerType := types.NewPointer(types.Typ[types.Int])
		typeName := types.NewTypeName(token.NoPos, pkg, "TestPointer", pointerType)
		named := types.NewNamed(typeName, pointerType, nil)
		assert.False(t, IsNamedStructType(named))
	})

	t.Run("FunctionType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		signature := types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewParam(token.NoPos, pkg, "x", types.Typ[types.Int])),
			types.NewTuple(types.NewParam(token.NoPos, pkg, "", types.Typ[types.String])),
			false)
		typeName := types.NewTypeName(token.NoPos, pkg, "TestFunc", signature)
		named := types.NewNamed(typeName, signature, nil)
		assert.False(t, IsNamedStructType(named))
	})
}

func TestIsXGoClassStructType(t *testing.T) {
	t.Run("NilNamedType", func(t *testing.T) {
		var named *types.Named
		assert.False(t, IsXGoClassStructType(named))
	})

	t.Run("NamedTypeWithNilObject", func(t *testing.T) {
		named := &types.Named{}
		assert.False(t, IsXGoClassStructType(named))
	})

	t.Run("NonXGoPackage", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "Game", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.False(t, IsXGoClassStructType(named))
	})

	t.Run("XGoPackageWithNonClassType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")

		// Mark package as XGo package.
		scope := pkg.Scope()
		scope.Insert(types.NewConst(token.NoPos, pkg, XGoPackage, types.Typ[types.UntypedBool], constant.MakeBool(true)))

		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "SomeOtherType", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.False(t, IsXGoClassStructType(named))
	})

	t.Run("SpxGameType", func(t *testing.T) {
		pkg := types.NewPackage(spxPkgPath, "spx")

		// Mark package as XGo package.
		scope := pkg.Scope()
		scope.Insert(types.NewConst(token.NoPos, pkg, XGoPackage, types.Typ[types.UntypedBool], constant.MakeBool(true)))

		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "Game", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.True(t, IsXGoClassStructType(named))
	})

	t.Run("SpxSpriteImplType", func(t *testing.T) {
		pkg := types.NewPackage(spxPkgPath, "spx")

		// Mark package as XGo package.
		scope := pkg.Scope()
		scope.Insert(types.NewConst(token.NoPos, pkg, XGoPackage, types.Typ[types.UntypedBool], constant.MakeBool(true)))

		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "SpriteImpl", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.True(t, IsXGoClassStructType(named))
	})

	t.Run("SpxGameTypeInNonXGoPackage", func(t *testing.T) {
		pkg := types.NewPackage(spxPkgPath, "spx")
		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "Game", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.False(t, IsXGoClassStructType(named))
	})

	t.Run("XGoPackageWithDifferentSpxType", func(t *testing.T) {
		pkg := types.NewPackage(spxPkgPath, "spx")

		// Mark package as XGo package.
		scope := pkg.Scope()
		scope.Insert(types.NewConst(token.NoPos, pkg, XGoPackage, types.Typ[types.UntypedBool], constant.MakeBool(true)))

		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "Sprite", structType)
		named := types.NewNamed(typeName, structType, nil)
		assert.False(t, IsXGoClassStructType(named))
	})
}

func TestWalkStruct(t *testing.T) {
	t.Run("NilNamedType", func(t *testing.T) {
		var (
			named  *types.Named
			called bool
		)
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			called = true
			return true
		})
		assert.False(t, called)
	})

	t.Run("NonStructType", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		typeName := types.NewTypeName(token.NoPos, pkg, "TestInt", types.Typ[types.Int])
		named := types.NewNamed(typeName, types.Typ[types.Int], nil)

		var called bool
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			called = true
			return true
		})
		assert.False(t, called)
	})

	t.Run("EmptyStruct", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "EmptyStruct", structType)
		named := types.NewNamed(typeName, structType, nil)

		var called bool
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			called = true
			return true
		})
		assert.False(t, called)
	})

	t.Run("SimpleStructWithFields", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		field1 := types.NewField(token.NoPos, pkg, "Field1", types.Typ[types.String], false)
		field2 := types.NewField(token.NoPos, pkg, "Field2", types.Typ[types.Int], false)
		structType := types.NewStruct([]*types.Var{field1, field2}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "TestStruct", structType)
		named := types.NewNamed(typeName, structType, nil)

		var (
			members   []types.Object
			selectors []*types.Named
		)
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			selectors = append(selectors, selector)
			return true
		})
		require.Len(t, members, 2)
		assert.Equal(t, "Field1", members[0].Name())
		assert.Equal(t, "Field2", members[1].Name())
		assert.Equal(t, named, selectors[0])
		assert.Equal(t, named, selectors[1])
	})

	t.Run("StructWithMethods", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		structType := types.NewStruct([]*types.Var{}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "TestStruct", structType)
		named := types.NewNamed(typeName, structType, nil)

		// Add a method to the named type.
		signature := types.NewSignatureType(nil, nil, nil,
			types.NewTuple(),
			types.NewTuple(types.NewParam(token.NoPos, pkg, "", types.Typ[types.String])),
			false)
		method := types.NewFunc(token.NoPos, pkg, "Method1", signature)
		named.AddMethod(method)

		var members []types.Object
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			return true
		})
		require.Len(t, members, 1)
		assert.Equal(t, "Method1", members[0].Name())
	})
	t.Run("StructWithEmbeddedStruct", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")

		// Create embedded struct.
		embeddedField := types.NewField(token.NoPos, pkg, "EmbeddedField", types.Typ[types.String], false)
		embeddedStructType := types.NewStruct([]*types.Var{embeddedField}, []string{})
		embeddedTypeName := types.NewTypeName(token.NoPos, pkg, "EmbeddedStruct", embeddedStructType)
		embeddedNamed := types.NewNamed(embeddedTypeName, embeddedStructType, nil)

		// Create main struct with embedded field.
		mainField := types.NewField(token.NoPos, pkg, "MainField", types.Typ[types.Int], false)
		embeddedFieldVar := types.NewField(token.NoPos, pkg, "EmbeddedStruct", embeddedNamed, true) // embedded
		mainStructType := types.NewStruct([]*types.Var{mainField, embeddedFieldVar}, []string{})
		mainTypeName := types.NewTypeName(token.NoPos, pkg, "MainStruct", mainStructType)
		mainNamed := types.NewNamed(mainTypeName, mainStructType, nil)

		var members []types.Object
		WalkStruct(mainNamed, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			return true
		})
		require.Len(t, members, 3)
		memberNames := make([]string, len(members))
		for i, member := range members {
			memberNames[i] = member.Name()
		}
		assert.Contains(t, memberNames, "MainField")
		assert.Contains(t, memberNames, "EmbeddedStruct")
		assert.Contains(t, memberNames, "EmbeddedField")
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		field1 := types.NewField(token.NoPos, pkg, "Field1", types.Typ[types.String], false)
		field2 := types.NewField(token.NoPos, pkg, "Field2", types.Typ[types.Int], false)
		structType := types.NewStruct([]*types.Var{field1, field2}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "TestStruct", structType)
		named := types.NewNamed(typeName, structType, nil)

		var members []types.Object
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			return len(members) < 1 // Stop after first member.
		})
		require.Len(t, members, 1)
		assert.Equal(t, "Field1", members[0].Name())
	})

	t.Run("CircularReferenceAvoidance", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")

		// Create type names first.
		typeNameA := types.NewTypeName(token.NoPos, pkg, "StructA", nil)
		typeNameB := types.NewTypeName(token.NoPos, pkg, "StructB", nil)

		// Create placeholder named types.
		namedA := types.NewNamed(typeNameA, nil, nil)
		namedB := types.NewNamed(typeNameB, nil, nil)

		// Create fields that reference each other.
		fieldAToB := types.NewField(token.NoPos, pkg, "StructB", namedB, true)
		fieldBToA := types.NewField(token.NoPos, pkg, "StructA", namedA, true)

		// Create struct types with circular references.
		structATypeWithB := types.NewStruct([]*types.Var{fieldAToB}, []string{})
		structBTypeWithA := types.NewStruct([]*types.Var{fieldBToA}, []string{})

		// Set the underlying types.
		namedA.SetUnderlying(structATypeWithB)
		namedB.SetUnderlying(structBTypeWithA)

		var callCount int
		WalkStruct(namedA, func(member types.Object, selector *types.Named) bool {
			callCount++
			return true
		})
		assert.GreaterOrEqual(t, callCount, 0) // Should not cause infinite recursion.
	})

	t.Run("UnexportedFieldsAndMethods", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")
		exportedField := types.NewField(token.NoPos, pkg, "ExportedField", types.Typ[types.String], false)
		unexportedField := types.NewField(token.NoPos, pkg, "unexportedField", types.Typ[types.Int], false)
		structType := types.NewStruct([]*types.Var{exportedField, unexportedField}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "TestStruct", structType)
		named := types.NewNamed(typeName, structType, nil)

		var members []types.Object
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			return true
		})

		// Only exported field should be included.
		require.Len(t, members, 1)
		assert.Equal(t, "ExportedField", members[0].Name())
	})
	t.Run("DuplicateMemberNames", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")

		// Create embedded struct with a field.
		embeddedField := types.NewField(token.NoPos, pkg, "SameName", types.Typ[types.String], false)
		embeddedStructType := types.NewStruct([]*types.Var{embeddedField}, []string{})
		embeddedTypeName := types.NewTypeName(token.NoPos, pkg, "EmbeddedStruct", embeddedStructType)
		embeddedNamed := types.NewNamed(embeddedTypeName, embeddedStructType, nil)

		// Create main struct with same field name.
		mainField := types.NewField(token.NoPos, pkg, "SameName", types.Typ[types.Int], false)
		embeddedFieldVar := types.NewField(token.NoPos, pkg, "EmbeddedStruct", embeddedNamed, true)
		mainStructType := types.NewStruct([]*types.Var{mainField, embeddedFieldVar}, []string{})
		mainTypeName := types.NewTypeName(token.NoPos, pkg, "MainStruct", mainStructType)
		mainNamed := types.NewNamed(mainTypeName, mainStructType, nil)

		var members []types.Object
		WalkStruct(mainNamed, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			return true
		})

		// Should see SameName (first occurrence) and EmbeddedStruct, but not the duplicate SameName.
		require.Len(t, members, 2)
		memberNames := make([]string, len(members))
		memberTypes := make([]types.Type, len(members))
		for i, member := range members {
			memberNames[i] = member.Name()
			memberTypes[i] = member.Type()
		}
		assert.Contains(t, memberNames, "SameName")
		assert.Contains(t, memberNames, "EmbeddedStruct")
		// First occurrence should be from main struct (Int type).
		sameNameIndex := -1
		for i, name := range memberNames {
			if name == "SameName" {
				sameNameIndex = i
				break
			}
		}
		assert.NotEqual(t, -1, sameNameIndex)
		assert.Equal(t, types.Typ[types.Int], memberTypes[sameNameIndex])
	})

	t.Run("XGoClassStructSelector", func(t *testing.T) {
		pkg := types.NewPackage(spxPkgPath, "spx")

		// Mark package as XGo package.
		scope := pkg.Scope()
		scope.Insert(types.NewConst(token.NoPos, pkg, XGoPackage, types.Typ[types.UntypedBool], constant.MakeBool(true)))

		// Create XGo class struct.
		field := types.NewField(token.NoPos, pkg, "TestField", types.Typ[types.String], false)
		structType := types.NewStruct([]*types.Var{field}, []string{})
		typeName := types.NewTypeName(token.NoPos, pkg, "Game", structType)
		named := types.NewNamed(typeName, structType, nil)

		var selectors []*types.Named
		WalkStruct(named, func(member types.Object, selector *types.Named) bool {
			selectors = append(selectors, selector)
			return true
		})
		require.Len(t, selectors, 1)
		assert.Equal(t, named, selectors[0])
	})
	t.Run("EmbeddedPointerStruct", func(t *testing.T) {
		pkg := types.NewPackage("test", "test")

		// Create embedded struct.
		embeddedField := types.NewField(token.NoPos, pkg, "EmbeddedField", types.Typ[types.String], false)
		embeddedStructType := types.NewStruct([]*types.Var{embeddedField}, []string{})
		embeddedTypeName := types.NewTypeName(token.NoPos, pkg, "EmbeddedStruct", embeddedStructType)
		embeddedNamed := types.NewNamed(embeddedTypeName, embeddedStructType, nil)

		// Create main struct with embedded pointer field.
		mainField := types.NewField(token.NoPos, pkg, "MainField", types.Typ[types.Int], false)
		embeddedPointerType := types.NewPointer(embeddedNamed)
		embeddedFieldVar := types.NewField(token.NoPos, pkg, "EmbeddedStruct", embeddedPointerType, true)
		mainStructType := types.NewStruct([]*types.Var{mainField, embeddedFieldVar}, []string{})
		mainTypeName := types.NewTypeName(token.NoPos, pkg, "MainStruct", mainStructType)
		mainNamed := types.NewNamed(mainTypeName, mainStructType, nil)

		var members []types.Object
		WalkStruct(mainNamed, func(member types.Object, selector *types.Named) bool {
			members = append(members, member)
			return true
		})
		require.Len(t, members, 3)
		memberNames := make([]string, len(members))
		for i, member := range members {
			memberNames[i] = member.Name()
		}
		assert.Contains(t, memberNames, "MainField")
		assert.Contains(t, memberNames, "EmbeddedStruct")
		assert.Contains(t, memberNames, "EmbeddedField")
	})
}
