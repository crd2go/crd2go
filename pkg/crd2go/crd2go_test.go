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

package crd2go

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/crd2go/crd2go/internal/checkerr"
	"github.com/crd2go/crd2go/internal/crd"
	"github.com/crd2go/crd2go/internal/gotype"
	"github.com/crd2go/crd2go/internal/plugins"
	"github.com/crd2go/crd2go/internal/testdata"
	"github.com/crd2go/crd2go/k8s"
	"github.com/crd2go/crd2go/pkg/config"
)

// pluginsFromYAML parses a YAML fragment like "- name: foo\n  options: {...}"
// into a []config.Plugin, matching what LoadConfig would produce.
func pluginsFromYAML(t *testing.T, src string) []config.Plugin {
	t.Helper()
	var plugins []config.Plugin
	require.NoError(t, yaml.Unmarshal([]byte(src), &plugins))
	return plugins
}

const (
	expectedSources = 19
)

var disabledKinds = []string{} // use ito skip problematic CRD kinds temporarily

var extraReserved = []string{} // use to fix problematic name picks, usually due to skips

func TestGenerateFromCRDs(t *testing.T) {
	// Goldens under internal/testdata/v1 must match repo-root crd2go.yaml (same as
	// `go run ./cmd/crd2go -config crd2go.yaml`), not a hand-built CoreConfig.
	cfg := loadGoldenCRD2GoConfig(t)
	if len(disabledKinds) > 0 {
		cfg.SkipList = append(append([]string{}, cfg.SkipList...), disabledKinds...)
	}

	buffers := make(map[string]*bytes.Buffer)
	in := bytes.NewBuffer(testdata.CRDsYAML)
	req := gotype.Request{
		CodeWriterFn: BufferForCRD(buffers),
		TypeDict:     gotype.NewTypeDict(cfg.Renames, gotype.KnownTypes()),
		CoreConfig:   cfg.CoreConfig,
	}
	require.NoError(t, Generate(&req, in))

	assert.NotEmpty(t, buffers)
	assert.Len(t, buffers, expectedSources)
	for key, buf := range buffers {
		want := readTestFile(t, testdata.V1, filepath.Join("v1", key))
		require.Equal(t, want, buf.String())
	}
}

func TestGenerateFromMixedGroupVersion(t *testing.T) {
	buffers := make(map[string]*bytes.Buffer)

	in := bytes.NewBuffer(testdata.DifferentGVYAML)
	req := gotype.Request{
		CodeWriterFn: BufferForCRD(buffers),
		TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
		CoreConfig: config.CoreConfig{
			Version:  crd.FirstVersion,
			SkipList: disabledKinds,
		},
	}
	want := "YAML input should only contain kinds for atlas.generated.mongodb.com/v2"
	require.ErrorContains(t, Generate(&req, in), want)
}

func TestGenerateSelectedGroupVersion(t *testing.T) {
	buffers := make(map[string]*bytes.Buffer)

	in := bytes.NewBuffer(testdata.DifferentGVYAML)
	req := gotype.Request{
		CodeWriterFn: BufferForCRD(buffers),
		TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
		CoreConfig: config.CoreConfig{
			Version:      crd.FirstVersion,
			SkipList:     disabledKinds,
			GroupVersion: "ea.generated.mongodb.com/v1",
			Plugins:      []config.Plugin{{Name: "gen-deepcopy"}},
		},
	}
	require.NoError(t, Generate(&req, in))
	assert.Len(t, buffers, 3)
	want := testdata.ResourceGoGenerated
	assert.Equal(t, want, buffers["resource.go"].String())
}

func TestRefs(t *testing.T) {
	buffers := make(map[string]*bytes.Buffer)

	in := bytes.NewBuffer(testdata.SampleRefsYAML)
	req := gotype.Request{
		CodeWriterFn: BufferForCRD(buffers),
		TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
		CoreConfig: config.CoreConfig{
			Version:  crd.FirstVersion,
			SkipList: disabledKinds,
			Plugins:  []config.Plugin{{Name: "gen-deepcopy"}},
		},
	}
	builtPlugins, err := Prepare(&req)
	require.NoError(t, err)
	_, err = GenerateStream(&req, builtPlugins, in)
	require.NoError(t, err)

	assert.NotEmpty(t, buffers)
	assert.Len(t, buffers, 1)
	for key, buf := range buffers {
		want := readTestFile(t, testdata.Refs, filepath.Join("refs", "v1", key))
		require.Equal(t, want, buf.String())
	}
}

func boolPtr(b bool) *bool { return &b }

func TestGenerateWithDeepCopy(t *testing.T) {
	for _, tc := range []struct {
		name             string
		dcConfig         config.DeepCopy
		plugins          []config.Plugin
		wantDocDCMarker  bool
		wantCRDDCMarker  bool
		wantControllerGe bool
	}{
		{
			// New semantics: no explicit opt-in means no deepcopy markers.
			// The legacy default-on behavior is gone — plugin presence is the switch.
			name:             "no config produces no markers",
			dcConfig:         config.DeepCopy{},
			wantDocDCMarker:  false,
			wantCRDDCMarker:  false,
			wantControllerGe: false,
		},
		{
			// Legacy field still honored (with deprecation warning to stderr).
			name:             "legacy DeepCopy true produces markers",
			dcConfig:         config.DeepCopy{Generate: boolPtr(true)},
			wantDocDCMarker:  true,
			wantCRDDCMarker:  true,
			wantControllerGe: true,
		},
		{
			name:             "legacy DeepCopy false produces no markers",
			dcConfig:         config.DeepCopy{Generate: boolPtr(false)},
			wantDocDCMarker:  false,
			wantCRDDCMarker:  false,
			wantControllerGe: false,
		},
		{
			name:             "gen-deepcopy plugin produces markers",
			plugins:          []config.Plugin{{Name: "gen-deepcopy"}},
			wantDocDCMarker:  true,
			wantCRDDCMarker:  true,
			wantControllerGe: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buffers := make(map[string]*bytes.Buffer)

			in := bytes.NewBuffer(testdata.CRDsYAML)
			req := gotype.Request{
				CodeWriterFn: BufferForCRD(buffers),
				TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
				CoreConfig: config.CoreConfig{
					Version:  crd.FirstVersion,
					SkipList: disabledKinds,
					DeepCopy: tc.dcConfig,
					Plugins:  tc.plugins,
				},
			}
			require.NoError(t, Generate(&req, in))

			docBuf, ok := buffers["doc.go"]
			require.True(t, ok, "doc.go should be generated")
			docContent := docBuf.String()

			if tc.wantDocDCMarker {
				assert.Contains(t, docContent, "+k8s:deepcopy-gen=package")
			} else {
				assert.NotContains(t, docContent, "+k8s:deepcopy-gen=package")
			}

			if tc.wantControllerGe {
				assert.Contains(t, docContent, "controller-gen object paths=")
			} else {
				assert.NotContains(t, docContent, "controller-gen object paths=")
			}

			// Check a CRD file for the per-type deepcopy marker
			for name, buf := range buffers {
				if name == "doc.go" || name == "groupversion_info.go" {
					continue
				}
				content := buf.String()
				if tc.wantCRDDCMarker {
					assert.Contains(t, content, "+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object", "expected deepcopy marker in %s", name)
				} else {
					assert.NotContains(t, content, "+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object", "unexpected deepcopy marker in %s", name)
				}
				break
			}
		})
	}
}

func TestGenerateWithGenClient(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		plugins               []config.Plugin
		wantGenClient         bool
		wantNonNamespaced     bool
		wantBeforeKubebuilder bool
	}{
		{
			name:          "no gen-client plugin produces no marker",
			plugins:       []config.Plugin{},
			wantGenClient: false,
		},
		{
			name:          "gen-client plugin produces +genclient marker",
			plugins:       []config.Plugin{{Name: "gen-client"}},
			wantGenClient: true,
		},
		{
			name: "gen-client nonNamespaced produces both markers",
			plugins: pluginsFromYAML(t, `
- name: gen-client
  options:
    nonNamespaced: true`),
			wantGenClient:     true,
			wantNonNamespaced: true,
		},
		{
			name:                  "gen-client marker appears before +kubebuilder:object:root",
			plugins:               []config.Plugin{{Name: "gen-client"}},
			wantGenClient:         true,
			wantBeforeKubebuilder: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buffers := make(map[string]*bytes.Buffer)

			in := bytes.NewBuffer(testdata.CRDsYAML)
			req := gotype.Request{
				CodeWriterFn: BufferForCRD(buffers),
				TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
				CoreConfig: config.CoreConfig{
					Version:  crd.FirstVersion,
					SkipList: disabledKinds,
					Plugins:  tc.plugins,
				},
			}
			require.NoError(t, Generate(&req, in))

			// Check a CRD file (skip doc.go and groupversion_info.go)
			for name, buf := range buffers {
				if name == "doc.go" || name == "groupversion_info.go" {
					continue
				}
				content := buf.String()

				if tc.wantGenClient {
					assert.Contains(t, content, "// +genclient", "expected +genclient in %s", name)
				} else {
					assert.NotContains(t, content, "// +genclient", "unexpected +genclient in %s", name)
				}

				if tc.wantNonNamespaced {
					assert.Contains(t, content, "// +genclient:nonNamespaced", "expected +genclient:nonNamespaced in %s", name)
				} else {
					assert.NotContains(t, content, "// +genclient:nonNamespaced", "unexpected +genclient:nonNamespaced in %s", name)
				}

				if tc.wantBeforeKubebuilder {
					genClientPos := bytes.Index(buf.Bytes(), []byte("+genclient"))
					kubebuilderPos := bytes.Index(buf.Bytes(), []byte("+kubebuilder:object:root"))
					assert.Greater(t, kubebuilderPos, genClientPos, "+genclient must appear before +kubebuilder:object:root in %s", name)
				}
				break
			}
		})
	}
}

func TestGenerateWithApplyConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		acConfig               config.ApplyConfiguration
		plugins                []config.Plugin
		wantDocACMarker        bool
		wantDocOutputPkg       string
		wantSchemeGroupVersion bool
	}{
		{
			name:                   "no config produces no markers",
			acConfig:               config.ApplyConfiguration{},
			wantDocACMarker:        false,
			wantSchemeGroupVersion: false,
		},
		{
			name:                   "legacy AC true produces markers",
			acConfig:               config.ApplyConfiguration{Generate: true},
			wantDocACMarker:        true,
			wantSchemeGroupVersion: true,
		},
		{
			name: "legacy AC with output package",
			acConfig: config.ApplyConfiguration{
				Generate:      true,
				OutputPackage: "../../applyconfiguration",
			},
			wantDocACMarker:        true,
			wantDocOutputPkg:       "../../applyconfiguration",
			wantSchemeGroupVersion: true,
		},
		{
			name:                   "gen-applyconfiguration plugin produces markers",
			plugins:                []config.Plugin{{Name: "gen-applyconfiguration"}},
			wantDocACMarker:        true,
			wantSchemeGroupVersion: true,
		},
		{
			name: "gen-applyconfiguration plugin with outputPackage option",
			plugins: pluginsFromYAML(t, `
- name: gen-applyconfiguration
  options:
    outputPackage: ../../applyconfiguration`),
			wantDocACMarker:        true,
			wantDocOutputPkg:       "../../applyconfiguration",
			wantSchemeGroupVersion: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buffers := make(map[string]*bytes.Buffer)

			in := bytes.NewBuffer(testdata.CRDsYAML)
			req := gotype.Request{
				CodeWriterFn: BufferForCRD(buffers),
				TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
				CoreConfig: config.CoreConfig{
					Version:            crd.FirstVersion,
					SkipList:           disabledKinds,
					ApplyConfiguration: tc.acConfig,
					Plugins:            tc.plugins,
				},
			}
			require.NoError(t, Generate(&req, in))

			// Check doc.go
			docBuf, ok := buffers["doc.go"]
			require.True(t, ok, "doc.go should be generated")
			docContent := docBuf.String()

			if tc.wantDocACMarker {
				assert.Contains(t, docContent, "+kubebuilder:ac:generate=true")
			} else {
				assert.NotContains(t, docContent, "+kubebuilder:ac:generate=true")
			}

			if tc.wantDocOutputPkg != "" {
				assert.Contains(t, docContent, "+kubebuilder:ac:output:package="+tc.wantDocOutputPkg)
			} else {
				assert.NotContains(t, docContent, "+kubebuilder:ac:output:package=")
			}

			// Check groupversion_info.go
			schemeBuf, ok := buffers["groupversion_info.go"]
			require.True(t, ok, "groupversion_info.go should be generated")
			schemeContent := schemeBuf.String()

			if tc.wantSchemeGroupVersion {
				assert.Contains(t, schemeContent, "SchemeGroupVersion = GroupVersion")
			} else {
				assert.NotContains(t, schemeContent, "SchemeGroupVersion")
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		want    *config.Config
		wantErr string
	}{
		{
			name:  "empty config",
			input: "{}",
			want:  &config.Config{},
		},
		{
			name: "defaults is empty lists and maps",
			input: `skipList: []
reserved: []
renames: {}
imports: []`,
			want: &config.Config{
				CoreConfig: config.CoreConfig{
					Reserved: []string{},
					SkipList: []string{},
					Renames:  map[string]string{},
					Imports:  []config.ImportedTypeConfig{},
				},
			},
		},
		{
			name: "just input and output",
			input: `input: ./pkg/crd2go/samples/crds.yaml
output: ./pkg/crd2go/samples/v1
skipList: []
reserved: []
renames: {}
imports: []`,
			want: &config.Config{
				Input:  "./pkg/crd2go/samples/crds.yaml",
				Output: "./pkg/crd2go/samples/v1",
				CoreConfig: config.CoreConfig{
					Reserved: []string{},
					SkipList: []string{},
					Renames:  map[string]string{},
					Imports:  []config.ImportedTypeConfig{},
				},
			},
		},
		{
			name: "applyConfiguration enabled with output package",
			input: `applyConfiguration:
  generate: true
  outputPackage: ../../applyconfiguration`,
			want: &config.Config{
				CoreConfig: config.CoreConfig{
					ApplyConfiguration: config.ApplyConfiguration{
						Generate:      true,
						OutputPackage: "../../applyconfiguration",
					},
				},
			},
		},
		{
			name:  "deepCopy false",
			input: `deepCopy: {generate: false}`,
			want: &config.Config{
				CoreConfig: config.CoreConfig{
					DeepCopy: config.DeepCopy{
						Generate: boolPtr(false),
					},
				},
			},
		},
		{
			name:  "deepCopy true",
			input: `deepCopy: {generate: true}`,
			want: &config.Config{
				CoreConfig: config.CoreConfig{
					DeepCopy: config.DeepCopy{
						Generate: boolPtr(true),
					},
				},
			},
		},
		{
			name:  "applyConfiguration disabled",
			input: `applyConfiguration: {generate: false}`,
			want: &config.Config{
				CoreConfig: config.CoreConfig{
					ApplyConfiguration: config.ApplyConfiguration{
						Generate: false,
					},
				},
			},
		},
		{
			name:    "bad yaml",
			input:   "this is not a good YAML config",
			wantErr: "cannot unmarshal",
		},
		{
			name:    "no input fails",
			input:   "",
			wantErr: "fake error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputReader := newFakeFailureReader()
			if tc.input != "" {
				inputReader = bytes.NewBufferString(tc.input)
			}
			cfg, err := LoadConfig(inputReader)
			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.want, cfg)
			} else {
				require.Nil(t, cfg)
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestDeprecatedAndPluginConflict(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cfg    config.CoreConfig
		errSub string
	}{
		{
			name: "deepCopy and gen-deepcopy plugin both set",
			cfg: config.CoreConfig{
				Version:  crd.FirstVersion,
				DeepCopy: config.DeepCopy{Generate: boolPtr(true)},
				Plugins:  []config.Plugin{{Name: "gen-deepcopy"}},
			},
			errSub: `sets both deprecated deepCopy.generate and the "gen-deepcopy" plugin`,
		},
		{
			name: "applyConfiguration and gen-applyconfiguration plugin both set",
			cfg: config.CoreConfig{
				Version:            crd.FirstVersion,
				ApplyConfiguration: config.ApplyConfiguration{Generate: true},
				Plugins:            []config.Plugin{{Name: "gen-applyconfiguration"}},
			},
			errSub: `sets both deprecated applyConfiguration and the "gen-applyconfiguration" plugin`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buffers := make(map[string]*bytes.Buffer)
			req := gotype.Request{
				CodeWriterFn: BufferForCRD(buffers),
				TypeDict:     gotype.NewTypeDict(nil, preloadedTypes()),
				CoreConfig:   tc.cfg,
			}
			err := Generate(&req, bytes.NewBuffer(testdata.CRDsYAML))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errSub)
		})
	}
}

// TestLoadConfigPluginOptions checks that plugin options survive the
// YAML → CoreConfig → CodegenPlugins round trip and decode into the
// plugin's typed options. Direct struct equality on yaml.Node is fragile
// (line/column metadata), so this test validates the observable behaviour
// of the loaded plugin rather than the raw config shape.
func TestLoadConfigPluginOptions(t *testing.T) {
	input := `plugins:
  - name: gen-client
    options:
      nonNamespaced: true`

	cfg, err := LoadConfig(bytes.NewBufferString(input))
	require.NoError(t, err)
	require.Len(t, cfg.Plugins, 1)
	assert.Equal(t, "gen-client", cfg.Plugins[0].Name)

	// The observable check: the plugin actually carries the nonNamespaced
	// flag across the load boundary and emits the matching marker.
	pluginList, err := plugins.CodegenPlugins(cfg.Plugins)
	require.NoError(t, err)
	require.Len(t, pluginList, 1)

	f := jen.NewFile("v1")
	require.NoError(t, pluginList[0].Annotate(f, "MyKind"))
	var out bytes.Buffer
	require.NoError(t, f.Render(&out))
	assert.Contains(t, out.String(), "+genclient:nonNamespaced")
}

func TestCodeFileForCRDAtPath(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir, err := os.MkdirTemp(cwd, "test-code-file-for-crd-path")
	require.NoError(t, err)
	defer checkerr.CheckErr("removing test temp dir", func() error { return os.RemoveAll(tmpDir) })

	cwFn := CodeWriterAtPath(tmpDir)
	require.NotNil(t, cwFn)

	w1, err := cwFn("testfile.go", false)
	assert.NoError(t, err)
	defer checkerr.CheckErr("closing 1st testfile.go", w1.Close)

	w2, err := cwFn("testfile.go", true)
	assert.NoError(t, err)
	defer checkerr.CheckErr("closing 2nd testfile.go", w2.Close)

	_, err = cwFn("..", true)
	assert.ErrorContains(t, err, "unsafe file path")

	_, err = cwFn("Bad/name", true)
	assert.ErrorContains(t, err, "failed to create file")
}

func readTestFile(t *testing.T, filesys fs.FS, path string) string {
	t.Helper()
	f, err := filesys.Open(path)
	require.NoError(t, err)
	defer checkerr.CheckErr("closing test file", f.Close)

	b, err := io.ReadAll(f)
	require.NoError(t, err)

	return string(b)
}

// loadGoldenCRD2GoConfig loads ../../crd2go.yaml from this package's test working directory.
func loadGoldenCRD2GoConfig(t *testing.T) *config.Config {
	t.Helper()
	p := filepath.Join("..", "..", "crd2go.yaml")
	f, err := os.Open(p)
	require.NoError(t, err, "open %s (golden test expects repo crd2go.yaml)", p)
	defer checkerr.CheckErr("closing crd2go.yaml", f.Close)
	cfg, err := LoadConfig(f)
	require.NoError(t, err)
	return cfg
}

func BufferForCRD(buffers map[string]*bytes.Buffer) config.CodeWriterFunc {
	return func(filename string, overwrite bool) (io.WriteCloser, error) {
		buffers[filename] = bytes.NewBufferString("")
		return newWriteNopCloser(buffers[filename]), nil
	}
}

// WriteNopCloser wraps an io.Writer and adds a no-op Close method.
type writeNopCloser struct {
	io.Writer
}

// Close is a no-op to satisfy the io.WriteCloser interface.
func (w writeNopCloser) Close() error {
	return nil
}

// Helper function to create a WriteNopCloser
func newWriteNopCloser(w io.Writer) io.WriteCloser {
	return writeNopCloser{Writer: w}
}

func preloadedTypes() []*gotype.GoType {
	return append(testKnownTypes(), reservedTypeNames(extraReserved)...)
}

func reservedTypeNames(reservedNames []string) []*gotype.GoType {
	reserved := make([]*gotype.GoType, 0, len(reservedNames))
	for _, reservedName := range reservedNames {
		reserved = append(reserved, ReserveTypeName(reservedName))
	}
	return reserved
}

func ReserveTypeName(name string) *gotype.GoType {
	return gotype.NewOpaqueType(name)
}

type fakeFailureReader struct{}

func (ffr *fakeFailureReader) Read(_ []byte) (int, error) {
	return 0, errors.New("fake error")
}

func newFakeFailureReader() io.Reader {
	return &fakeFailureReader{}
}

func testKnownTypes() []*gotype.GoType {
	return append(gotype.KnownTypes(),
		gotype.MustTypeFrom(reflect.TypeOf(k8s.LocalReference{})),
		gotype.MustTypeFrom(reflect.TypeOf(k8s.Reference{})),
	)
}
