/*
 * Copyright (c) 2025 The GoPlus Authors (goplus.org). All rights reserved.
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

package gop

import (
	"fmt"

	"github.com/goplus/mod/gopmod"
	"github.com/goplus/mod/modfile"
)

func init() {
	gopmod.SpxProject.Works = []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl"}}
}

// SetClassfileAutoImportedPackages sets the auto-imported packages for the
// classfile specified by id.
func SetClassfileAutoImportedPackages(id string, pkgs map[string]string) {
	var project *modfile.Project
	switch id {
	case "spx":
		project = gopmod.SpxProject
	default:
		panic(fmt.Sprintf("unknown classfile id: %s", id))
	}

	project.Import = nil // Clear previous imports.
	for k, v := range pkgs {
		imp := &modfile.Import{Name: k, Path: v}
		project.Import = append(gopmod.SpxProject.Import, imp)
	}
}
