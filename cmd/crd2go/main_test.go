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

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/crd2go/crd2go/internal/gotype"
)

func TestPrintConflictHint(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantOutput   string
		wantNoOutput bool
	}{
		{
			name:         "non-conflict error prints nothing",
			err:          fmt.Errorf("some other error"),
			wantNoOutput: true,
		},
		{
			name: "single candidate prints a direct pinning",
			err: fmt.Errorf("wrapped: %w", &gotype.ExistingNameConflictError{
				Conflicts: []gotype.ExistingNameConflict{
					{Name: "Config", Candidates: [][]string{{"A", "Config"}}},
				},
			}),
			wantOutput: "conflicting names found with existing type names, please use these pinnings in the config to fix:\n\npinnings:\n  - A.Config\n\nOr use flag --force-renames to allow crd2go to rename existing types as needed.\n",
		},
		{
			name: "multiple candidates prints a pick-one comment",
			err: &gotype.ExistingNameConflictError{
				Conflicts: []gotype.ExistingNameConflict{
					{Name: "Config", Candidates: [][]string{{"A", "Config"}, {"B", "Config"}}},
				},
			},
			wantOutput: "conflicting names found with existing type names, please use these pinnings in the config to fix:\n\npinnings:\n  - A.Config | B.Config # pick one\n\nOr use flag --force-renames to allow crd2go to rename existing types as needed.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			require.NoError(t, err)
			origStderr := os.Stderr
			os.Stderr = w
			t.Cleanup(func() { os.Stderr = origStderr })

			printConflictHint(tt.err)
			require.NoError(t, w.Close())

			var buf bytes.Buffer
			_, err = io.Copy(&buf, r)
			require.NoError(t, err)

			if tt.wantNoOutput {
				assert.Empty(t, buf.String())
				return
			}
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}
