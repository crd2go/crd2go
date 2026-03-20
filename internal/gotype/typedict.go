// Copyright 2025 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gotype

import (
	"github.com/crd2go/crd2go/pkg/config"
)

// TypeDict is a dictionary of Go types, used to track and ensure unique type names.
// It embeds a NameEngine for naming and deduplication, and separately tracks which
// types have already been emitted during generation (render).
type TypeDict struct {
	nameEngine    NameEngine
	knownTypes    map[string]*GoType
	knownByHash   map[string]*GoType
	renames       map[string]string
	// generated records type names that have been rendered (keyed by final Go type name).
	generated map[string]bool
}

// Request holds the runtime information to handle a CRD generation request
type Request struct {
	config.CoreConfig
	CodeWriterFn config.CodeWriterFunc
	TypeDict     *TypeDict
}

// NewTypeDict creates a new TypeDict with the given renames and known types (preloaded).
func NewTypeDict(renames map[string]string, goTypes ...*GoType) *TypeDict {
	knownTypes := make(map[string]*GoType)
	knownByHash := make(map[string]*GoType)
	for _, gt := range goTypes {
		knownTypes[gt.Name] = gt
		knownByHash[HashType(gt)] = gt
	}
	return &TypeDict{
		nameEngine:  NewNameEngine(),
		knownTypes:  knownTypes,
		knownByHash: knownByHash,
		renames:     renames,
		generated:   make(map[string]bool),
	}
}

// Has checks if the TypeDict contains a GoType with the same structure (via NameEngine).
func (td *TypeDict) Has(gt *GoType) bool {
	return td.nameEngine.Has(gt)
}

// Get retrieves a GoType by its name. Checks known types first, then NameEngine (after ResolveNames).
func (td *TypeDict) Get(name string) (*GoType, bool) {
	if gt, ok := td.knownTypes[name]; ok {
		return gt, true
	}
	return td.nameEngine.Get(name)
}

// AddAll adds types to known types (preloaded). Used for reserved names and imports.
func (td *TypeDict) AddAll(goTypes ...*GoType) {
	for _, gt := range goTypes {
		td.knownTypes[gt.Name] = gt
		td.knownByHash[HashType(gt)] = gt
	}
}

// MarkGenerated records that a GoType definition has been emitted during render.
func (td *TypeDict) MarkGenerated(gt *GoType) {
	if gt != nil {
		td.generated[gt.Name] = true
	}
}

// WasGenerated reports whether that type's definition was already emitted.
func (td *TypeDict) WasGenerated(gt *GoType) bool {
	if gt == nil {
		return false
	}
	return td.generated[gt.Name]
}

// RegisterAndResolve registers all types from the given roots with the NameEngine,
// applies user renames and import matching, then resolves all names.
// Must be called after all CRD types are built and before rendering.
func (td *TypeDict) RegisterAndResolve(roots []*GoType) error {
	for _, gt := range td.knownTypes {
		if gt.Kind != AutoImportKind {
			_ = td.nameEngine.Register([]string{gt.Name}, gt)
		}
	}
	for _, root := range roots {
		if err := td.registerTree(root, nil); err != nil {
			return err
		}
	}
	_ = td.nameEngine.NamedRoots()
	return nil
}

func (td *TypeDict) registerTree(gt *GoType, path []string) error {
	if gt == nil {
		return nil
	}
	base := gt.BaseType()
	if base.IsPrimitive() {
		return nil
	}
	if base.Kind == AutoImportKind || base.Kind == OpaqueKind {
		return nil
	}

	base.Name = td.rename(base.Name)
	if importInfo := td.matchImport(base); importInfo != nil {
		base.Import = importInfo
		return nil
	}
	if td.matchKnownByHash(base) {
		return nil
	}

	typePath := append(path, base.Name)
	if err := td.nameEngine.Register(typePath, base); err != nil {
		return err
	}

	if base.Element != nil {
		if err := td.registerTree(base.Element, typePath); err != nil {
			return err
		}
	}
	for _, f := range base.Fields {
		if err := td.registerTree(f.GoType, typePath); err != nil {
			return err
		}
	}
	return nil
}

func (td *TypeDict) rename(name string) string {
	if len(td.renames) > 0 {
		if newName, ok := td.renames[name]; ok {
			return newName
		}
	}
	return name
}

// matchImport checks if the given type matches a known auto-import type by name.
func (td *TypeDict) matchImport(gt *GoType) *config.ImportInfo {
	entry, ok := td.knownTypes[gt.Name]
	if !ok || entry.Kind != AutoImportKind {
		return nil
	}
	entry.CloneStructure(gt)
	return entry.Import
}

// matchKnownByHash checks if the type matches a known preloaded type by HashType.
// If so, copies the known type's name and import and returns true.
func (td *TypeDict) matchKnownByHash(gt *GoType) bool {
	entry, ok := td.knownByHash[HashType(gt)]
	if !ok {
		return false
	}
	gt.Name = entry.Name
	gt.Import = entry.Import
	return true
}
