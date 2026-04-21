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
	"gopkg.in/yaml.v3"

	"github.com/crd2go/crd2go/internal/plugins"
	"github.com/crd2go/crd2go/pkg/config"
)

func TestGenClientAnnotate(t *testing.T) {
	tests := []struct {
		title      string
		optionsSrc string
		wants      []string
		notwant    []string
	}{
		{
			title:      "no options produces +genclient only",
			optionsSrc: "",
			wants:      []string{"// +genclient"},
			notwant:    []string{"// +genclient:nonNamespaced"},
		},
		{
			title:      "nonNamespaced false produces +genclient only",
			optionsSrc: "nonNamespaced: false",
			wants:      []string{"// +genclient"},
			notwant:    []string{"// +genclient:nonNamespaced"},
		},
		{
			title:      "nonNamespaced true produces both markers",
			optionsSrc: "nonNamespaced: true",
			wants:      []string{"// +genclient", "// +genclient:nonNamespaced"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			pluginList, err := plugins.CodegenPlugins([]config.Plugin{
				{Name: "gen-client", Options: pluginOptionsFromYAML(t, tc.optionsSrc)},
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

func TestGenClientOptionsErrors(t *testing.T) {
	tests := []struct {
		title      string
		optionsSrc string
		wantErr    string
	}{
		{
			title:      "unknown field is rejected",
			optionsSrc: "nonNamspaced: true", // typo
			wantErr:    "field nonNamspaced not found",
		},
		{
			title:      "wrong type is rejected",
			optionsSrc: `nonNamespaced: "true"`, // quoted string, not bool
			wantErr:    "cannot unmarshal",
		},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			_, err := plugins.CodegenPlugins([]config.Plugin{
				{Name: "gen-client", Options: pluginOptionsFromYAML(t, tc.optionsSrc)},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
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

// pluginOptionsFromYAML parses a YAML fragment into the yaml.Node shape a
// plugin would receive from config load. An empty src returns a zero node,
// matching the "options omitted" case.
func pluginOptionsFromYAML(t *testing.T, src string) yaml.Node {
	t.Helper()
	var node yaml.Node
	if src == "" {
		return node
	}
	require.NoError(t, yaml.Unmarshal([]byte(src), &node))
	require.Equal(t, yaml.DocumentNode, node.Kind)
	require.Len(t, node.Content, 1)
	return *node.Content[0]
}
