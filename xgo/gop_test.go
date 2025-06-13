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

package xgo

import (
	"testing"

	"github.com/goplus/mod/xgomod"
	"github.com/stretchr/testify/assert"
)

func TestSetClassfileAutoImportedPackages(t *testing.T) {
	t.Run("Spx", func(t *testing.T) {
		originalImports := xgomod.SpxProject.Import
		t.Cleanup(func() {
			xgomod.SpxProject.Import = originalImports
		})

		pkgs := map[string]string{
			"fmt":    "fmt",
			"foobar": "example.com/foobar",
			"math":   "math",
		}
		SetClassfileAutoImportedPackages("spx", pkgs)

		assert.Len(t, xgomod.SpxProject.Import, 3)

		got := make(map[string]string)
		for _, imp := range xgomod.SpxProject.Import {
			got[imp.Name] = imp.Path
		}
		assert.Equal(t, pkgs, got)
	})

	t.Run("UnknownClassfileID", func(t *testing.T) {
		assert.Panics(t, func() {
			SetClassfileAutoImportedPackages("unknown", nil)
		})
	})
}
