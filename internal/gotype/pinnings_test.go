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

func TestSuggestPinnings(t *testing.T) {
	configA := NewStruct("Config", []*GoField{NewGoField("X", NewPrimitive("string", StringKind))})
	configB := NewStruct("Config", []*GoField{NewGoField("Y", NewPrimitive("string", StringKind))})

	tests := []struct {
		name         string
		existingDecl string
		want         []string
	}{
		{
			name:         "no field match returns all candidates joined with pipe",
			existingDecl: "type Config struct{ Z string }",
			want:         []string{"A.Config | B.Config"},
		},
		{
			name:         "single field match returns the matching path",
			existingDecl: "type Config struct{ X string }",
			want:         []string{"A.Config"},
		},
		{
			name:         "all fields match returns all candidates joined with pipe",
			existingDecl: "type Config struct{ X string\n Y string }",
			want:         []string{"A.Config | B.Config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflicts := []ExistingNameConflict{
				{
					Name:         "Config",
					ExistingFile: writeTempGoFile(t, tt.existingDecl),
					candidates: []typeInfo{
						{path: []string{"A", "Config"}, gt: configA},
						{path: []string{"B", "Config"}, gt: configB},
					},
				},
			}
			got, err := SuggestPinnings(conflicts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatPinningsSuggestion(t *testing.T) {
	tests := []struct {
		name     string
		pinnings []string
		want     string
	}{
		{
			name:     "single pinning",
			pinnings: []string{"A.Config"},
			want:     "pinnings:\n  - A.Config\n",
		},
		{
			name:     "pick-one pinning gets comment",
			pinnings: []string{"A.Config | B.Config"},
			want:     "pinnings:\n  - A.Config | B.Config # pick one\n",
		},
		{
			name:     "mixed pinnings",
			pinnings: []string{"A.Config", "A.Spec | B.Spec"},
			want:     "pinnings:\n  - A.Config\n  - A.Spec | B.Spec # pick one\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatPinningsSuggestion(tt.pinnings))
		})
	}
}

// sortedCandidatePaths joins each candidate's path into a dot-separated string
// and sorts them, so candidate order from non-deterministic map iteration doesn't
// affect assertions.
func sortedCandidatePaths(infos []typeInfo) []string {
	joined := make([]string, len(infos))
	for i, info := range infos {
		joined[i] = strings.Join(info.path, ".")
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
