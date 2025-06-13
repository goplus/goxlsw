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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDerefType(t *testing.T) {
	t.Run("PointerTypes", func(t *testing.T) {
		intPtr := types.NewPointer(types.Typ[types.Int])
		assert.Equal(t, types.Typ[types.Int], DerefType(intPtr))

		stringPtr := types.NewPointer(types.Typ[types.String])
		assert.Equal(t, types.Typ[types.String], DerefType(stringPtr))

		intPtrPtr := types.NewPointer(intPtr)
		assert.Equal(t, intPtr, DerefType(intPtrPtr))
	})

	t.Run("NonPointerTypes", func(t *testing.T) {
		assert.Equal(t, types.Typ[types.Int], DerefType(types.Typ[types.Int]))
		assert.Equal(t, types.Typ[types.String], DerefType(types.Typ[types.String]))
		assert.Equal(t, types.Typ[types.Bool], DerefType(types.Typ[types.Bool]))

		intSlice := types.NewSlice(types.Typ[types.Int])
		assert.Equal(t, intSlice, DerefType(intSlice))

		structType := types.NewStruct([]*types.Var{}, []string{})
		assert.Equal(t, structType, DerefType(structType))

		interfaceType := types.NewInterfaceType([]*types.Func{}, []types.Type{})
		assert.Equal(t, interfaceType, DerefType(interfaceType))
	})

	t.Run("ComplexTypes", func(t *testing.T) {
		structType := types.NewStruct([]*types.Var{types.NewField(0, nil, "field", types.Typ[types.String], false)}, []string{})
		structPtr := types.NewPointer(structType)
		assert.Equal(t, structType, DerefType(structPtr))

		sliceType := types.NewSlice(types.Typ[types.Int])
		slicePtr := types.NewPointer(sliceType)
		assert.Equal(t, sliceType, DerefType(slicePtr))

		chanType := types.NewChan(types.SendRecv, types.Typ[types.Int])
		chanPtr := types.NewPointer(chanType)
		assert.Equal(t, chanType, DerefType(chanPtr))
	})

	t.Run("NilType", func(t *testing.T) {
		got := DerefType(nil)
		assert.Nil(t, got)
	})
}

func TestIsTypesCompatible(t *testing.T) {
	t.Run("NilTypes", func(t *testing.T) {
		assert.False(t, IsTypesCompatible(nil, nil))
		assert.False(t, IsTypesCompatible(types.Typ[types.Int], nil))
		assert.False(t, IsTypesCompatible(nil, types.Typ[types.Int]))
	})

	t.Run("AssignableTypes", func(t *testing.T) {
		assert.True(t, IsTypesCompatible(types.Typ[types.Int], types.Typ[types.Int]))
		assert.True(t, IsTypesCompatible(types.Typ[types.String], types.Typ[types.String]))

		// Untyped constants are assignable to typed values.
		assert.True(t, IsTypesCompatible(types.Typ[types.UntypedInt], types.Typ[types.Int]))
		assert.True(t, IsTypesCompatible(types.Typ[types.UntypedString], types.Typ[types.String]))
	})

	t.Run("InterfaceTypes", func(t *testing.T) {
		stringMethod := types.NewFunc(0, nil, "String", types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.String])), false))
		stringerIface := types.NewInterfaceType([]*types.Func{stringMethod}, nil)

		structType := types.NewStruct([]*types.Var{}, []string{})
		namedStruct := types.NewNamed(types.NewTypeName(0, nil, "TestStruct", nil), structType, nil)
		namedStruct.AddMethod(stringMethod)

		assert.True(t, IsTypesCompatible(namedStruct, stringerIface))
		assert.False(t, IsTypesCompatible(types.Typ[types.Int], stringerIface))
	})

	t.Run("PointerTypes", func(t *testing.T) {
		intPtr := types.NewPointer(types.Typ[types.Int])
		stringPtr := types.NewPointer(types.Typ[types.String])

		// Both pointers with same element type.
		assert.True(t, IsTypesCompatible(intPtr, intPtr))

		// Different pointer element types.
		assert.False(t, IsTypesCompatible(intPtr, stringPtr))

		// Non-pointer to pointer with same element type.
		assert.True(t, IsTypesCompatible(types.Typ[types.Int], intPtr))

		// Non-pointer to pointer with different element type.
		assert.False(t, IsTypesCompatible(types.Typ[types.String], intPtr))
	})

	t.Run("SliceTypes", func(t *testing.T) {
		intSlice := types.NewSlice(types.Typ[types.Int])
		stringSlice := types.NewSlice(types.Typ[types.String])

		// Same slice element types.
		assert.True(t, IsTypesCompatible(intSlice, intSlice))

		// Different slice element types.
		assert.False(t, IsTypesCompatible(intSlice, stringSlice))

		// Non-slice to slice.
		assert.False(t, IsTypesCompatible(types.Typ[types.Int], intSlice))
	})
	t.Run("ChannelTypes", func(t *testing.T) {
		intChanSendRecv := types.NewChan(types.SendRecv, types.Typ[types.Int])
		intChanSend := types.NewChan(types.SendOnly, types.Typ[types.Int])
		intChanRecv := types.NewChan(types.RecvOnly, types.Typ[types.Int])
		stringChanSendRecv := types.NewChan(types.SendRecv, types.Typ[types.String])

		// Compatible channel directions.
		assert.True(t, IsTypesCompatible(intChanSendRecv, intChanSendRecv))
		assert.True(t, IsTypesCompatible(intChanSend, intChanSend))
		assert.True(t, IsTypesCompatible(intChanRecv, intChanRecv))

		// SendRecv channels are assignable to unidirectional channels.
		assert.True(t, IsTypesCompatible(intChanSendRecv, intChanSend))
		assert.True(t, IsTypesCompatible(intChanSendRecv, intChanRecv))

		// SendRecv want accepts any direction.
		assert.True(t, IsTypesCompatible(intChanSend, intChanSendRecv))
		assert.True(t, IsTypesCompatible(intChanRecv, intChanSendRecv))

		// Incompatible channel directions (unidirectional to different unidirectional).
		assert.False(t, IsTypesCompatible(intChanSend, intChanRecv))
		assert.False(t, IsTypesCompatible(intChanRecv, intChanSend))

		// Different channel element types.
		assert.False(t, IsTypesCompatible(intChanSendRecv, stringChanSendRecv))

		// Non-channel to channel.
		assert.False(t, IsTypesCompatible(types.Typ[types.Int], intChanSendRecv))
	})

	t.Run("NamedTypes", func(t *testing.T) {
		namedInt1 := types.NewNamed(types.NewTypeName(0, nil, "MyInt1", nil), types.Typ[types.Int], nil)
		namedInt2 := types.NewNamed(types.NewTypeName(0, nil, "MyInt2", nil), types.Typ[types.Int], nil)

		// Same named type.
		assert.True(t, IsTypesCompatible(namedInt1, namedInt1))

		// Different named types with same underlying type.
		assert.False(t, IsTypesCompatible(namedInt1, namedInt2))

		// Named type to underlying type.
		assert.False(t, IsTypesCompatible(namedInt1, types.Typ[types.Int]))
	})

	t.Run("IncompatibleTypes", func(t *testing.T) {
		// Basic incompatible types.
		assert.False(t, IsTypesCompatible(types.Typ[types.Int], types.Typ[types.String]))
		assert.False(t, IsTypesCompatible(types.Typ[types.Bool], types.Typ[types.Float64]))

		// Complex incompatible types.
		intSlice := types.NewSlice(types.Typ[types.Int])
		stringPtr := types.NewPointer(types.Typ[types.String])
		assert.False(t, IsTypesCompatible(intSlice, stringPtr))
	})
}
