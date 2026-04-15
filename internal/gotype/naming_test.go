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
	"os"
	"path/filepath"
	"sort"
	"strings"
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
			want:   "sha256:8406ae6ab928c84e7196cd61a0bac2ae1c41fc4c7bb5c72a6c23643704dd988f",
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

func TestHashType_EmptyStructsDistinctByName(t *testing.T) {
	a := HashType(NewStruct("User", []*GoField{}))
	b := HashType(NewStruct("Team", []*GoField{}))
	assert.NotEqual(t, a, b, "empty structs must not collapse to the same key")
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

func TestUniqueTypes_sameHashDistinctPointers(t *testing.T) {
	c1 := NewStruct("Config", []*GoField{NewGoField("X", NewPrimitive("string", StringKind))})
	c2 := NewStruct("Config", []*GoField{NewGoField("X", NewPrimitive("string", StringKind))})
	require.Equal(t, HashType(c1), HashType(c2))
	ne := NewNameEngine().(*nameEngine)
	h1 := hashTypeFast(ne.hashCache, c1)
	h2 := hashTypeFast(ne.hashCache, c2)
	require.Equal(t, h1, h2)
	infos := []typeInfo{
		{path: []string{"A", "Config"}, hash: h1, gt: c1},
		{path: []string{"B", "Config"}, hash: h2, gt: c2},
	}
	out := uniqueTypes(infos)
	require.Len(t, out, 1, "same structural hash must collapse to one candidate")
	assert.Equal(t, []string{"A", "Config"}, out[0].path)
}

func TestNameEngine(t *testing.T) {
	tests := []struct {
		name          string
		regsFn        func() []nameEngineReg
		setupFn       func(t *testing.T) (existingNames map[string]string, pinnedPaths map[string]bool)
		want          []string
		wantErr       bool
		wantConflicts []ExistingNameConflict
	}{
		{
			name: "single root",
			regsFn: func() []nameEngineReg {
				return []nameEngineReg{
					{
						path: []string{"Team"},
						gt:   NewStruct("Team", []*GoField{}),
					},
				}
			},
			want: []string{"Team"},
		},
		{
			name: "two roots sorted",
			regsFn: func() []nameEngineReg {
				return []nameEngineReg{
					{
						path: []string{"Cluster"},
						gt:   NewStruct("Cluster", []*GoField{NewGoField("C", NewPrimitive("string", StringKind))}),
					},
					{
						path: []string{"Team"},
						gt:   NewStruct("Team", []*GoField{NewGoField("T", NewPrimitive("string", StringKind))}),
					},
				}
			},
			want: []string{"Cluster", "Team"},
		},
		{
			name: "nested non-root",
			regsFn: func() []nameEngineReg {
				specForCluster, clusterWithSpecOnly := newNestedClusterStructs()
				return []nameEngineReg{
					{
						path: []string{"Cluster"},
						gt:   clusterWithSpecOnly,
					},
					{
						path: []string{"Cluster", "Spec"},
						gt:   specForCluster,
					},
				}
			},
			want: []string{"Cluster", "Spec"},
		},
		{
			name: "root and nested",
			regsFn: func() []nameEngineReg {
				spec := newTestSpec()
				return []nameEngineReg{
					{
						path: []string{"Team"},
						gt:   NewStruct("Team", []*GoField{NewGoField("T", NewPrimitive("string", StringKind)), NewGoField("Spec", spec)}),
					},
					{
						path: []string{"Team", "Spec"},
						gt:   spec,
					},
				}
			},
			want: []string{"Team", "Spec"},
		},
		{
			name: "conflict resolved by prepend",
			regsFn: func() []nameEngineReg {
				structA, configA := newConflictStructsA()
				structB, configB := newConflictStructsB()
				return []nameEngineReg{
					{
						path: []string{"A"},
						gt:   structA,
					},
					{
						path: []string{"A", "Config"},
						gt:   configA,
					},
					{
						path: []string{"B"},
						gt:   structB,
					},
					{
						path: []string{"B", "Config"},
						gt:   configB,
					},
				}
			},
			want: []string{"A", "AConfig", "B", "BConfig"},
		},
		{
			name: "alias deduplicated",
			regsFn: func() []nameEngineReg {
				spec := newTestSpec()
				teamWithSpec, clusterWithSpec := newAliasStructs(spec)
				return []nameEngineReg{
					{
						path: []string{"Team"},
						gt:   teamWithSpec,
					},
					{
						path: []string{"Team", "Spec"},
						gt:   spec,
					},
					{
						path: []string{"Cluster"},
						gt:   clusterWithSpec,
					},
					{
						path: []string{"Cluster", "Spec"},
						gt:   spec,
					},
				}
			},
			want: []string{"Cluster", "Spec", "Team"},
		},
		{
			name: "roots with same-named nested",
			regsFn: func() []nameEngineReg {
				config := newTestConfig()
				return []nameEngineReg{
					{
						path: []string{"Team"},
						gt:   NewStruct("Team", []*GoField{NewGoField("T", NewPrimitive("string", StringKind)), NewGoField("Config", config)}),
					},
					{
						path: []string{"Cluster"},
						gt:   NewStruct("Cluster", []*GoField{NewGoField("C", NewPrimitive("string", StringKind)), NewGoField("Config", config)}),
					},
					{
						path: []string{"Team", "Config"},
						gt:   config,
					},
					{
						path: []string{"Cluster", "Config"},
						gt:   config,
					},
				}
			},
			want: []string{"Cluster", "Config", "Team"},
		},
		{
			name: "duplicate root errors",
			regsFn: func() []nameEngineReg {
				team := newTestTeam()
				return []nameEngineReg{
					{
						path: []string{"Team"},
						gt:   team,
					},
					{
						path: []string{"Team"},
						gt:   team,
					},
				}
			},
			wantErr: true,
		},
		{
			name: "existing name conflict records all candidates when field narrowing finds no match",
			regsFn: func() []nameEngineReg {
				structA, configA := newConflictStructsA()
				structB, configB := newConflictStructsB()
				return []nameEngineReg{
					{path: []string{"A"}, gt: structA},
					{path: []string{"A", "Config"}, gt: configA},
					{path: []string{"B"}, gt: structB},
					{path: []string{"B", "Config"}, gt: configB},
				}
			},
			setupFn: func(t *testing.T) (map[string]string, map[string]bool) {
				// existing Config has field Z — matches neither configA (X) nor configB (Y)
				f := writeTempGoFile(t, "type Config struct{ Z string }")
				return map[string]string{"Config": f}, nil
			},
			wantConflicts: []ExistingNameConflict{
				{Name: "Config", Candidates: [][]string{{"A", "Config"}, {"B", "Config"}}},
			},
		},
		{
			name: "existing name conflict narrowed to one candidate by field match",
			regsFn: func() []nameEngineReg {
				structA, configA := newConflictStructsA()
				structB, configB := newConflictStructsB()
				return []nameEngineReg{
					{path: []string{"A"}, gt: structA},
					{path: []string{"A", "Config"}, gt: configA},
					{path: []string{"B"}, gt: structB},
					{path: []string{"B", "Config"}, gt: configB},
				}
			},
			setupFn: func(t *testing.T) (map[string]string, map[string]bool) {
				// existing Config has field X — matches only configA (A.Config)
				f := writeTempGoFile(t, "type Config struct{ X string }")
				return map[string]string{"Config": f}, nil
			},
			wantConflicts: []ExistingNameConflict{
				{Name: "Config", Candidates: [][]string{{"A", "Config"}}},
			},
		},
		{
			name: "pinned candidate bypasses existing name conflict and keeps its name",
			regsFn: func() []nameEngineReg {
				structA, configA := newConflictStructsA()
				structB, configB := newConflictStructsB()
				return []nameEngineReg{
					{path: []string{"A"}, gt: structA},
					{path: []string{"A", "Config"}, gt: configA},
					{path: []string{"B"}, gt: structB},
					{path: []string{"B", "Config"}, gt: configB},
				}
			},
			setupFn: func(t *testing.T) (map[string]string, map[string]bool) {
				f := writeTempGoFile(t, "type Config struct{ Z string }")
				return map[string]string{"Config": f}, map[string]bool{"A.Config": true}
			},
			want: []string{"A", "Config", "B", "BConfig"},
		},
		{
			name: "pinned candidate keeps name while other candidate is prepended",
			regsFn: func() []nameEngineReg {
				structA, configA := newConflictStructsA()
				structB, configB := newConflictStructsB()
				return []nameEngineReg{
					{path: []string{"A"}, gt: structA},
					{path: []string{"A", "Config"}, gt: configA},
					{path: []string{"B"}, gt: structB},
					{path: []string{"B", "Config"}, gt: configB},
				}
			},
			setupFn: func(t *testing.T) (map[string]string, map[string]bool) {
				return nil, map[string]bool{"A.Config": true}
			},
			want: []string{"A", "Config", "B", "BConfig"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regs := tt.regsFn()
			ne := NewNameEngine().(*nameEngine)
			if tt.setupFn != nil {
				existingNames, pinnedPaths := tt.setupFn(t)
				ne.existingNames = existingNames
				ne.pinnedPaths = pinnedPaths
			}
			var regErr error
			for _, r := range regs {
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
			roots, err := ne.NamedRoots()
			if tt.wantConflicts != nil {
				var conflictErr *ExistingNameConflictError
				require.ErrorAs(t, err, &conflictErr)
				require.Len(t, conflictErr.Conflicts, len(tt.wantConflicts))
				for i, want := range tt.wantConflicts {
					got := conflictErr.Conflicts[i]
					assert.Equal(t, want.Name, got.Name)
					assert.Equal(t, sortedPaths(want.Candidates), sortedPaths(got.Candidates))
				}
				return
			}
			require.NoError(t, err)
			got := collectNonPrimitiveNames(roots)
			assert.Equal(t, tt.want, got)
		})
	}
}

func newTestSpec() *GoType {
	return NewStruct("Spec", []*GoField{})
}

func newTestConfig() *GoType {
	return NewStruct("Config", []*GoField{})
}

func newTestTeam() *GoType {
	return NewStruct("Team", []*GoField{})
}

func newConflictStructsA() (structA, configA *GoType) {
	configA = NewStruct("Config", []*GoField{NewGoField("X", NewPrimitive("string", StringKind))})
	structA = NewStruct("A", []*GoField{NewGoField("Config", configA)})
	return structA, configA
}

func newConflictStructsB() (structB, configB *GoType) {
	configB = NewStruct("Config", []*GoField{NewGoField("Y", NewPrimitive("string", StringKind))})
	structB = NewStruct("B", []*GoField{NewGoField("Config", configB)})
	return structB, configB
}

func newAliasStructs(spec *GoType) (teamWithSpec, clusterWithSpec *GoType) {
	teamWithSpec = NewStruct("Team", []*GoField{NewGoField("Spec", spec)})
	clusterWithSpec = NewStruct("Cluster", []*GoField{NewGoField("Spec", spec)})
	return teamWithSpec, clusterWithSpec
}

func newNestedClusterStructs() (specForCluster, clusterWithSpecOnly *GoType) {
	specForCluster = NewStruct("Spec", []*GoField{})
	clusterWithSpecOnly = NewStruct("Cluster", []*GoField{NewGoField("Spec", specForCluster)})
	return specForCluster, clusterWithSpecOnly
}

// collectNonPrimitiveNames traverses roots and returns all non-primitive type names in order of first appearance.
func collectNonPrimitiveNames(roots []*GoType) []string {
	var names []string
	seen := make(map[*GoType]bool)
	var visit func(gt *GoType)
	visit = func(gt *GoType) {
		if gt == nil || seen[gt] {
			return
		}
		seen[gt] = true
		if gt.IsPrimitive() {
			return
		}
		names = append(names, gt.Name)
		if gt.Element != nil {
			visit(gt.Element)
		}
		for _, f := range gt.Fields {
			visit(f.GoType)
		}
	}
	for _, r := range roots {
		visit(r)
	}
	return names
}

type nameEngineReg struct {
	path []string
	gt   *GoType
}

// sortedPaths joins each path slice into a dot-separated string and sorts them,
// so candidate order from non-deterministic map iteration doesn't affect assertions.
func sortedPaths(paths [][]string) []string {
	joined := make([]string, len(paths))
	for i, p := range paths {
		joined[i] = strings.Join(p, ".")
	}
	sort.Strings(joined)
	return joined
}

// writeTempGoFile writes a temporary Go source file containing the given type
// declarations under package x, and returns its path.
func writeTempGoFile(t *testing.T, typeDecls string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "types.go")
	require.NoError(t, os.WriteFile(path, []byte("package x\n\n"+typeDecls+"\n"), 0o644))
	return path
}

func TestExistingNameConflictError_Error(t *testing.T) {
	tests := []struct {
		name      string
		conflicts []ExistingNameConflict
		want      string
	}{
		{
			name: "single conflict",
			conflicts: []ExistingNameConflict{
				{Name: "Config", Candidates: [][]string{{"A", "Config"}}},
			},
			want: "existing type name conflicts: Config (1 candidate(s))",
		},
		{
			name: "multiple conflicts",
			conflicts: []ExistingNameConflict{
				{Name: "Config", Candidates: [][]string{{"A", "Config"}, {"B", "Config"}}},
				{Name: "Spec", Candidates: [][]string{{"A", "Spec"}}},
			},
			want: "existing type name conflicts: Config (2 candidate(s)), Spec (1 candidate(s))",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ExistingNameConflictError{Conflicts: tt.conflicts}
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestFormatPinningsSuggestion(t *testing.T) {
	tests := []struct {
		name      string
		conflicts []ExistingNameConflict
		want      string
	}{
		{
			name: "single candidate produces a single pinning line",
			conflicts: []ExistingNameConflict{
				{Name: "Config", Candidates: [][]string{{"A", "Config"}}},
			},
			want: "pinnings:\n  - A.Config\n",
		},
		{
			name: "multiple candidates produces a pick-one comment",
			conflicts: []ExistingNameConflict{
				{Name: "Config", Candidates: [][]string{{"A", "Config"}, {"B", "Config"}}},
			},
			want: "pinnings:\n  - A.Config | B.Config # pick one\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatPinningsSuggestion(tt.conflicts))
		})
	}
}
