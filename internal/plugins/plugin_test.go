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

package plugins_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/crd2go/crd2go/internal/plugins"
	"github.com/crd2go/crd2go/pkg/config"
)

func TestCodegenPlugins(t *testing.T) {
	tests := []struct {
		title   string
		configs []config.Plugin
		want    []string
		wantErr string
	}{
		{
			title:   "Empty Config",
			configs: []config.Plugin{},
			want:    []string{},
		},
		{
			title:   "Single Valid Plugin",
			configs: []config.Plugin{{Name: "get-conditions"}},
			want:    []string{"get-conditions"},
		},
		{
			title:   "Unknown Plugin Error",
			configs: []config.Plugin{{Name: "non-existent-plugin"}},
			wantErr: "\"non-existent-plugin\" is not a registered plugin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			got, err := plugins.CodegenPlugins(tc.configs)
			if tc.wantErr != "" {
				require.Nil(t, got)
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				names := []string{}
				for _, plugin := range got {
					names = append(names, plugin.Name())
				}
				require.Equal(t, tc.want, names)
			}
		})
	}
}
