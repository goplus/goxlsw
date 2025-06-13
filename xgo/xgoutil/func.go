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
	"regexp"
	"strings"

	"github.com/goplus/gogen"
)

const (
	XGotPrefix = "Gopt_" // XGo template method.
	XGooPrefix = "Gopo_" // XGo overload function/method.
	XGoxPrefix = "Gopx_" // XGo type as parameters function/method.
)

// IsXGotMethodName reports whether the given name is an XGo template method name.
func IsXGotMethodName(name string) bool {
	_, _, ok := SplitXGotMethodName(name, false)
	return ok
}

// SplitXGotMethodName splits an XGo template method name into receiver type
// name and method name.
func SplitXGotMethodName(name string, trimXGox bool) (recvTypeName string, methodName string, ok bool) {
	if !strings.HasPrefix(name, XGotPrefix) {
		return "", "", false
	}
	recvTypeName, methodName, ok = strings.Cut(name[len(XGotPrefix):], "_")
	if !ok {
		return "", "", false
	}
	if trimXGox {
		if funcName, ok := SplitXGoxFuncName(methodName); ok {
			methodName = funcName
		}
	}
	return
}

// SplitXGoxFuncName splits an XGo type as parameters function name into the
// function name.
func SplitXGoxFuncName(name string) (funcName string, ok bool) {
	if !strings.HasPrefix(name, XGoxPrefix) {
		return "", false
	}
	funcName = strings.TrimPrefix(name, XGoxPrefix)
	ok = true
	return
}

// ParseXGoFuncName parses the XGo overloaded function name.
func ParseXGoFuncName(name string) (parsedName string, overloadID *string) {
	parsedName = name
	if matches := xgoOverloadFuncNameRE.FindStringSubmatch(parsedName); len(matches) == 3 {
		parsedName = matches[1]
		overloadID = &matches[2]
	}
	parsedName = ToLowerCamelCase(parsedName)
	return
}

// xgoOverloadFuncNameRE is the regular expression of the XGo overloaded
// function name.
var xgoOverloadFuncNameRE = regexp.MustCompile(`^(.+)__([0-9a-z])$`)

// IsXGoOverloadedFuncName reports whether the given function name is an XGo
// overloaded function name.
func IsXGoOverloadedFuncName(name string) bool {
	return xgoOverloadFuncNameRE.MatchString(name)
}

// IsXGoOverloadableFunc reports whether the given function is an XGo overloadable
// function with a signature like `func(__gop_overload_args__ interface{_()})`.
func IsXGoOverloadableFunc(fun *types.Func) bool {
	typ, _ := gogen.CheckSigFuncExObjects(fun.Type().(*types.Signature))
	return typ != nil
}

// IsUnexpandableXGoOverloadableFunc reports whether the given function is a
// Unexpandable-XGo-Overloadable-Func, which is a function that:
//  1. is overloadable: has a signature like `func(__gop_overload_args__ interface{_()})`
//  2. but not expandable: can not be expanded into overloads
//
// A typical example is method `GetWidget` on spx `Game`.
func IsUnexpandableXGoOverloadableFunc(fun *types.Func) bool {
	sig := fun.Type().(*types.Signature)
	if _, ok := gogen.CheckSigFuncEx(sig); ok { // is `func(__gop_overload_args__ interface{_()})`
		if t, _ := gogen.CheckSigFuncExObjects(sig); t == nil { // not expandable
			return true
		}
	}
	return false
}

// ExpandXGoOverloadableFunc expands the given XGo function with a signature
// like `func(__gop_overload_args__ interface{_()})` to all its overloads. It
// returns nil if the function is not qualified for overload expansion.
func ExpandXGoOverloadableFunc(fun *types.Func) []*types.Func {
	typ, objs := gogen.CheckSigFuncExObjects(fun.Type().(*types.Signature))
	if typ == nil {
		return nil
	}
	overloads := make([]*types.Func, 0, len(objs))
	for _, obj := range objs {
		overloads = append(overloads, obj.(*types.Func))
	}
	return overloads
}
