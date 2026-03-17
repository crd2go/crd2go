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
)

func TestHashType(t *testing.T) {
	tests := map[string]struct {
		goType *GoType
		want   string
	}{
		"nil type": {
			goType: nil,
			want:   "",
		},
		"primitive string": {
			goType: NewPrimitive("string", StringKind),
			want:   StringKind,
		},
		"primitive int": {
			goType: NewPrimitive("int", IntKind),
			want:   IntKind,
		},
		"primitive float64": {
			goType: NewPrimitive("float64", FloatKind),
			want:   FloatKind,
		},
		"primitive bool": {
			goType: NewPrimitive("bool", BoolKind),
			want:   BoolKind,
		},
		"array of primitive": {
			goType: NewArray(NewPrimitive("string", StringKind)),
			want:   "[]string",
		},
		"array of int": {
			goType: NewArray(NewPrimitive("int", IntKind)),
			want:   "[]int",
		},
		"non-primitive empty struct": {
			goType: NewStruct("User", []*GoField{}),
			want:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		"non-primitive struct with primitive fields": {
			goType: NewStruct("User", []*GoField{
				NewGoField("Name", NewPrimitive("string", StringKind)),
				NewGoField("Age", NewPrimitive("int", IntKind)),
			}),
			want: "sha256:e8ce0fd007cd27d863ae44532c75f4e3417a61eedad006064aeb8ffb7ba8a75b",
		},
		"struct with array field": {
			goType: NewStruct("User", []*GoField{
				NewGoField("Tags", NewArray(NewPrimitive("string", StringKind))),
			}),
			want: "sha256:f4c743d99bf1d71498d000fbff906024ad7adb1d343c9b3ff6f80ad6c79ab974",
		},
		"one level of subtypes": {
			goType: NewStruct("Company", []*GoField{
				NewGoField("Name", NewPrimitive("string", StringKind)),
				NewGoField("Address", NewStruct("Address", []*GoField{
					NewGoField("Street", NewPrimitive("string", StringKind)),
					NewGoField("City", NewPrimitive("string", StringKind)),
				})),
			}),
			want: "sha256:62f1cca5d43d6f0b1d6976c22c7609633d88c41560048a29db0ba8a381b5e474",
		},
		"two levels of subtypes": {
			goType: NewStruct("Company", []*GoField{
				NewGoField("Name", NewPrimitive("string", StringKind)),
				NewGoField("Headquarters", NewStruct("Location", []*GoField{
					NewGoField("Address", NewStruct("Address", []*GoField{
						NewGoField("Street", NewPrimitive("string", StringKind)),
						NewGoField("ZipCode", NewPrimitive("string", StringKind)),
					})),
					NewGoField("Country", NewPrimitive("string", StringKind)),
				})),
			}),
			want: "sha256:c1e478cbc4edd989f2483fb5875965bd29eeb144e38b5395811f97d306c75c8f",
		},
		"struct with array of structs": {
			goType: NewStruct("Company", []*GoField{
				NewGoField("Employees", NewArray(NewStruct("Employee", []*GoField{
					NewGoField("Name", NewPrimitive("string", StringKind)),
					NewGoField("ID", NewPrimitive("int", IntKind)),
				}))),
			}),
			want: "sha256:e4aca4c894cb8ef5bb29cf4ccfac74d16a50c33fdc52c64ed12842fc494083f3",
		},
		"different field order produces same hash": {
			goType: NewStruct("User", []*GoField{
				NewGoField("Age", NewPrimitive("int", IntKind)),
				NewGoField("Name", NewPrimitive("string", StringKind)),
			}),
			want: "sha256:e8ce0fd007cd27d863ae44532c75f4e3417a61eedad006064aeb8ffb7ba8a75b", // Fields are sorted by name in NewStruct, so order doesn't matter
		},
		"different field names produce different hashes": {
			goType: NewStruct("User", []*GoField{
				NewGoField("Name", NewPrimitive("string", StringKind)),
			}),
			want: "sha256:b7dfc0eb7440cd454d729a467bc71d3925b093e6d5581a64e8aa60770fe82b2f",
		},
		"different field types produce different hashes": {
			goType: NewStruct("User", []*GoField{
				NewGoField("Age", NewPrimitive("int", IntKind)),
			}),
			want: "sha256:d7152c0127f4f13903ae33791ef5f3624cc3e4e6d1051bec5433d20923a9968d",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			hash := HashType(tt.goType)
			assert.Equal(t, tt.want, hash)
		})
	}
}

func TestHashType_Comparison(t *testing.T) {
	hash1 := HashType(NewStruct("User", []*GoField{
		NewGoField("Name", NewPrimitive("string", StringKind)),
	}))
	hash2 := HashType(NewStruct("User", []*GoField{
		NewGoField("FullName", NewPrimitive("string", StringKind)),
	}))
	assert.NotEqual(t, hash1, hash2)
	assert.Equal(t, "sha256:b7dfc0eb7440cd454d729a467bc71d3925b093e6d5581a64e8aa60770fe82b2f", hash1)
	assert.Equal(t, "sha256:2d95edf3ac22e5a66767b3e7e2c5f3406d873cfc73fdf84170b6418033a0a0c6", hash2)
}

func TestHashType_Comparison_DifferentFieldTypes(t *testing.T) {
	hash1 := HashType(NewStruct("User", []*GoField{
		NewGoField("Age", NewPrimitive("int", IntKind)),
	}))
	hash2 := HashType(NewStruct("User", []*GoField{
		NewGoField("Age", NewPrimitive("string", StringKind)),
	}))
	assert.NotEqual(t, hash1, hash2)
	assert.Equal(t, "sha256:d7152c0127f4f13903ae33791ef5f3624cc3e4e6d1051bec5433d20923a9968d", hash1)
	assert.Equal(t, "sha256:6289d5681e93ea53dd9a075241495a10474fb579fda376f637fd26c35a9d447a", hash2)
}

func TestNameEngine(t *testing.T) {
	spec := NewStruct("Spec", []*GoField{})     // shared for alias test
	config := NewStruct("Config", []*GoField{}) // shared for roots-with-nested test
	team := NewStruct("Team", []*GoField{})     // shared for duplicate-root test

	tests := []struct {
		name    string
		regs    []nameEngineReg
		want    []string
		wantErr bool
	}{
		{
			name: "single root",
			regs: []nameEngineReg{
				{path: []string{"Team"}, gt: NewStruct("Team", []*GoField{})},
			},
			want: []string{"Team"},
		},
		{
			name: "two roots sorted",
			regs: []nameEngineReg{
				{path: []string{"Cluster"}, gt: NewStruct("Cluster", []*GoField{NewGoField("C", NewPrimitive("string", StringKind))})},
				{path: []string{"Team"}, gt: NewStruct("Team", []*GoField{NewGoField("T", NewPrimitive("string", StringKind))})},
			},
			want: []string{"Cluster", "Team"},
		},
		{
			name: "nested non-root",
			regs: []nameEngineReg{
				{path: []string{"Cluster", "Spec"}, gt: NewStruct("Spec", []*GoField{})},
			},
			want: []string{},
		},
		{
			name: "root and nested",
			regs: []nameEngineReg{
				{path: []string{"Team"}, gt: NewStruct("Team", []*GoField{NewGoField("T", NewPrimitive("string", StringKind))})},
				{path: []string{"Team", "Spec"}, gt: NewStruct("Spec", []*GoField{})},
			},
			want: []string{"Team"},
		},
		{
			name: "conflict resolved by prepend",
			regs: []nameEngineReg{
				{path: []string{"A", "Config"}, gt: NewStruct("Config", []*GoField{NewGoField("X", NewPrimitive("string", StringKind))})},
				{path: []string{"B", "Config"}, gt: NewStruct("Config", []*GoField{NewGoField("Y", NewPrimitive("string", StringKind))})},
			},
			want: []string{},
		},
		{
			name: "alias deduplicated",
			regs: []nameEngineReg{
				{path: []string{"Team", "Spec"}, gt: spec},
				{path: []string{"Cluster", "Spec"}, gt: spec},
			},
			want: []string{},
		},
		{
			name: "roots with same-named nested",
			regs: []nameEngineReg{
				{path: []string{"Team"}, gt: NewStruct("Team", []*GoField{NewGoField("T", NewPrimitive("string", StringKind))})},
				{path: []string{"Cluster"}, gt: NewStruct("Cluster", []*GoField{NewGoField("C", NewPrimitive("string", StringKind))})},
				{path: []string{"Team", "Config"}, gt: config},
				{path: []string{"Cluster", "Config"}, gt: config},
			},
			want: []string{"Cluster", "Team"},
		},
		{
			name: "duplicate root errors",
			regs: []nameEngineReg{
				{path: []string{"Team"}, gt: team},
				{path: []string{"Team"}, gt: team},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ne := NewNameEngine().(*nameEngine)
			var regErr error
			for _, r := range tt.regs {
				regErr = ne.Register(r.path, r.gt)
				if regErr != nil {
					break
				}
			}
			if tt.wantErr {
				require.Error(t, regErr)
				assert.ErrorIs(t, regErr, ErrDuplicateRoot)
				return
			}
			require.NoError(t, regErr)
			roots := ne.NamedRoots()
			names := make([]string, len(roots))
			for i, r := range roots {
				names[i] = r.Name
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

type nameEngineReg struct {
	path []string
	gt   *GoType
}
