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
)

// XGoPackage indicates an XGo package.
const XGoPackage = "GopPackage"

// IsMarkedAsXGoPackage reports whether the given package is marked as an XGo package.
func IsMarkedAsXGoPackage(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}
	scope := pkg.Scope()
	if scope == nil {
		return false
	}
	obj := scope.Lookup(XGoPackage)
	if obj == nil {
		return false
	}
	cnst, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	if cnst.Type() != types.Typ[types.UntypedBool] {
		return false
	}
	cnstVal := cnst.Val()
	return cnstVal != nil && constant.BoolVal(cnstVal)
}

// PkgPath returns the package path of the given pkg. It returns "builtin" if
// the pkg is nil.
func PkgPath(pkg *types.Package) string {
	if pkg == nil {
		return "builtin"
	}
	pkgPath := pkg.Path()
	if pkgPath == "" {
		// Builtin objects do not belong to any package. But in the type system of XGo,
		// they may have non-nil package with an empty path, e.g., append.
		return "builtin"
	}
	return pkgPath
}

// IsBuiltinPkg reports whether the given package is the "builtin" package.
func IsBuiltinPkg(pkg *types.Package) bool {
	return PkgPath(pkg) == "builtin"
}

// IsMainPkg reports whether the given package is the "main" package.
func IsMainPkg(pkg *types.Package) bool {
	return PkgPath(pkg) == "main"
}
