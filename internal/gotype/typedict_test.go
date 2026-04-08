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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/crd2go/crd2go/pkg/config"
)

func TestTypeDictHas(t *testing.T) {
	tests := map[string]struct {
		goType   *GoType
		preload  []*GoType
		expected bool
	}{
		"empty dict": {
			goType:   NewStruct("User", []*GoField{}),
			preload:  []*GoType{},
			expected: false,
		},
		"existing item after resolve": {
			goType:   NewStruct("User", []*GoField{}),
			preload:  []*GoType{},
			expected: true,
		},
		"non-existing item": {
			goType:   NewStruct("Other", []*GoField{}),
			preload:  []*GoType{NewStruct("User", []*GoField{})},
			expected: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			td := NewTypeDict(nil, tt.preload...)
			if tt.expected {
				root := NewStruct("Root", []*GoField{NewGoField("User", tt.goType)})
				err := td.RegisterAndResolve([]*GoType{root})
				requireNoErr(t, err)
			}
			assert.Equal(t, tt.expected, td.Has(tt.goType))
		})
	}
}

func TestTypeDictGet(t *testing.T) {
	tests := map[string]struct {
		preload      []*GoType
		name         string
		expectedType *GoType
		expected     bool
	}{
		"empty dict": {
			preload:      []*GoType{},
			name:         "MyType",
			expectedType: nil,
			expected:     false,
		},
		"known type by name": {
			preload: []*GoType{
				NewStruct("MyType", []*GoField{}),
			},
			name:         "MyType",
			expectedType: NewStruct("MyType", []*GoField{}),
			expected:     true,
		},
		"non-existing item": {
			preload: []*GoType{
				NewStruct("MyType", []*GoField{}),
			},
			name:         "OtherType",
			expectedType: nil,
			expected:     false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			td := NewTypeDict(nil, tt.preload...)
			goType, ok := td.Get(tt.name)
			assert.Equal(t, tt.expected, ok)
			if tt.expected {
				assert.Equal(t, tt.name, goType.Name)
			} else {
				assert.Nil(t, goType)
			}
		})
	}
}

func TestTypeDictAddAll(t *testing.T) {
	typeString := NewPrimitive("String", StringKind)
	typeUser := NewStruct("User", []*GoField{})

	td := NewTypeDict(map[string]string{}, []*GoType{}...)
	td.AddAll(typeString, typeUser)

	gt, ok := td.Get("String")
	assert.True(t, ok)
	assert.Equal(t, "String", gt.Name)

	gt, ok = td.Get("User")
	assert.True(t, ok)
	assert.Equal(t, "User", gt.Name)
}

func TestTypeDict_MarkGenerated(t *testing.T) {
	gt := NewStruct("User", []*GoField{})
	td := NewTypeDict(map[string]string{}, []*GoType{}...)
	err := td.RegisterAndResolve([]*GoType{gt})
	requireNoErr(t, err)

	td.MarkGenerated(gt)
	assert.True(t, td.WasGenerated(gt))
}

func TestTypeDict_RegisterAndResolve(t *testing.T) {
	spec := NewStruct("Spec", []*GoField{NewGoField("Name", NewPrimitive("string", StringKind))})
	root := NewStruct("Resource", []*GoField{NewGoField("Spec", spec)})

	td := NewTypeDict(nil)
	err := td.RegisterAndResolve([]*GoType{root})
	requireNoErr(t, err)

	assert.Equal(t, "Resource", root.Name)
	assert.Equal(t, "Spec", spec.Name)
}

func TestTypeDict_RegisterAndResolve_WithRenames(t *testing.T) {
	config := NewStruct("Config", []*GoField{})
	root := NewStruct("Resource", []*GoField{NewGoField("Config", config)})

	td := NewTypeDict(map[string]string{"Config": "Settings"})
	err := td.RegisterAndResolve([]*GoType{root})
	requireNoErr(t, err)

	assert.Equal(t, "Settings", config.Name)
}

func TestTypeDict_RegisterAndResolve_MatchImportDoesNotCorruptPreload(t *testing.T) {
	auto := NewAutoImportType(&config.ImportedTypeConfig{
		Name: "LocalReference",
		ImportInfo: config.ImportInfo{
			Alias: "k8s",
			Path:  "github.com/crd2go/crd2go/k8s",
		},
	})
	// Two roots with the same OpenAPI shape renamed to LocalReference — both must
	// keep import info; the preload entry must stay AutoImportKind for every match.
	gr := func() *GoType {
		return NewStruct("GroupRef", []*GoField{
			NewGoField("Name", NewPrimitive("string", StringKind)),
		})
	}
	root1 := NewStruct("R1", []*GoField{NewGoField("Ref", gr())})
	root2 := NewStruct("R2", []*GoField{NewGoField("Ref", gr())})

	td := NewTypeDict(map[string]string{"GroupRef": "LocalReference"}, auto)
	err := td.RegisterAndResolve([]*GoType{root1, root2})
	require.NoError(t, err)

	assert.Equal(t, AutoImportKind, auto.Kind, "preloaded LocalReference must remain AutoImportKind")
	for _, root := range []*GoType{root1, root2} {
		ref := root.Fields[0].GoType.BaseType()
		require.NotNil(t, ref)
		assert.Equal(t, AutoImportKind, ref.Kind)
		require.NotNil(t, ref.Import)
		assert.Equal(t, "k8s", ref.Import.Alias)
	}
}

func TestTypeDict_RegisterAndResolve_WithKnownTypes(t *testing.T) {
	known := NewStruct("Reference", []*GoField{
		NewGoField("Name", NewPrimitive("string", StringKind)),
		NewGoField("Namespace", NewPrimitive("string", StringKind)),
	})
	known.Import = &config.ImportInfo{Alias: "k8s", Path: "github.com/crd2go/crd2go/k8s"}

	ref := NewStruct("CrossReference", []*GoField{
		NewGoField("Name", NewPrimitive("string", StringKind)),
		NewGoField("Namespace", NewPrimitive("string", StringKind)),
	})
	root := NewStruct("Resource", []*GoField{NewGoField("Ref", ref)})

	td := NewTypeDict(nil, known)
	err := td.RegisterAndResolve([]*GoType{root})
	requireNoErr(t, err)

	assert.Equal(t, "Reference", ref.Name)
	assert.NotNil(t, ref.Import)
}

func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
