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
	"bytes"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/crd2go/crd2go/internal/gotype"
	"github.com/crd2go/crd2go/internal/plugins"
)

func TestGetConditionsProcess(t *testing.T) {
	tests := []struct {
		title         string
		inputTypeName string
		wants         []string // Using a slice allows checking multiple parts of the output
		wantErr       string
	}{
		{
			title:         "Valid Input SampleResource",
			inputTypeName: "SampleResource",
			wants: []string{
				// Check for the comment
				"// GetConditions for SampleResource",
				// Check for the function body
				`func (sr *SampleResource) GetConditions() []metav1.Condition {
	if sr.Status.Conditions == nil {
		return nil
	}
	return *sr.Status.Conditions
}`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			f := jen.NewFile("main")
			f.ImportAlias("k8s.io/apimachinery/pkg/apis/meta/v1", "metav1")

			req := &plugins.CodegenRequest{
				Type: &gotype.GoType{Name: tc.inputTypeName},
				File: f,
			}

			processor := &plugins.GetConditions{}
			err := processor.Process(req)

			if tc.wantErr != "" {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				buf := &bytes.Buffer{}
				err = f.Render(buf)
				require.NoError(t, err)
				generatedCode := buf.String()
				for _, want := range tc.wants {
					assert.Contains(t, generatedCode, want)
				}
			}
		})
	}
}
