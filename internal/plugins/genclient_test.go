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

	"github.com/crd2go/crd2go/internal/plugins"
	"github.com/crd2go/crd2go/pkg/config"
)

func TestGenClientAnnotate(t *testing.T) {
	tests := []struct {
		title   string
		options map[string]string
		wants   []string
		notwant []string
	}{
		{
			title:   "no options produces +genclient only",
			options: nil,
			wants:   []string{"// +genclient"},
			notwant: []string{"// +genclient:nonNamespaced"},
		},
		{
			title:   "nonNamespaced false produces +genclient only",
			options: map[string]string{"nonNamespaced": "false"},
			wants:   []string{"// +genclient"},
			notwant: []string{"// +genclient:nonNamespaced"},
		},
		{
			title:   "nonNamespaced true produces both markers",
			options: map[string]string{"nonNamespaced": "true"},
			wants:   []string{"// +genclient", "// +genclient:nonNamespaced"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			pluginList, err := plugins.CodegenPlugins([]config.Plugin{
				{Name: "gen-client", Options: tc.options},
			})
			require.NoError(t, err)
			require.Len(t, pluginList, 1)

			f := jen.NewFile("v1")
			require.NoError(t, pluginList[0].Annotate(f, "MyKind"))

			buf := &bytes.Buffer{}
			require.NoError(t, f.Render(buf))
			output := buf.String()

			for _, want := range tc.wants {
				assert.Contains(t, output, want)
			}
			for _, notwant := range tc.notwant {
				assert.NotContains(t, output, notwant)
			}
		})
	}
}

func TestGenClientProcess(t *testing.T) {
	pluginList, err := plugins.CodegenPlugins([]config.Plugin{{Name: "gen-client"}})
	require.NoError(t, err)
	require.Len(t, pluginList, 1)

	f := jen.NewFile("v1")
	err = pluginList[0].Process(&plugins.CodegenRequest{File: f})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	require.NoError(t, f.Render(buf))
	assert.NotContains(t, buf.String(), "+genclient", "Process should not add any genclient markers")
}

func TestGenClientAnnotateIsBeforeType(t *testing.T) {
	// Verify that when used in a file, the +genclient marker appears before the type definition.
	f := jen.NewFile("v1")

	pluginList, err := plugins.CodegenPlugins([]config.Plugin{{Name: "gen-client"}})
	require.NoError(t, err)

	require.NoError(t, pluginList[0].Annotate(f, "MyKind"))

	f.Type().Id("MyKind").Struct()

	buf := &bytes.Buffer{}
	require.NoError(t, f.Render(buf))
	output := buf.String()

	genClientPos := bytes.Index(buf.Bytes(), []byte("+genclient"))
	typePos := bytes.Index(buf.Bytes(), []byte("type MyKind"))
	assert.Greater(t, typePos, genClientPos, "+genclient marker must appear before the type definition\n%s", output)
}
