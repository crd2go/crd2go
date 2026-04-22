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

package plugins

import (
	"bytes"
	"fmt"

	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"

	"github.com/crd2go/crd2go/internal/gotype"
	"github.com/crd2go/crd2go/pkg/config"
)

type CodegenRequest struct {
	Type *gotype.GoType
	File *jen.File
}

// Plugin is the full surface a codegen plugin exposes. Plugins contribute to
// up to four distinct locations in the generated output, one per hook:
//
//   - Annotate: marker comments immediately before the root CRD type.
//   - Process: additional Go code after the root CRD type definition.
//   - DocAnnotate: header comments and body of doc.go.
//   - SchemeVars: extra entries in the var block of groupversion_info.go.
//
// Most plugins only care about one or two of these. Embed BasePlugin to get
// no-op defaults for the rest.
type Plugin interface {
	Name() string
	Annotate(f *jen.File, kind string) error
	Process(cgr *CodegenRequest) error
	DocAnnotate(f *jen.File, group, version string) error
	SchemeVars(group, version string) ([]jen.Code, error)
}

// BasePlugin is an embeddable no-op implementation of every Plugin hook
// except Name(), which must be provided by the concrete plugin since it is
// always unique per plugin.
type BasePlugin struct{}

func (BasePlugin) Annotate(_ *jen.File, _ string) error       { return nil }
func (BasePlugin) Process(_ *CodegenRequest) error            { return nil }
func (BasePlugin) DocAnnotate(_ *jen.File, _, _ string) error { return nil }
func (BasePlugin) SchemeVars(_, _ string) ([]jen.Code, error) { return nil, nil }

type PluginBuilderFunc func(config.Plugin) (Plugin, error)

var codegenPlugins = map[string]PluginBuilderFunc{
	GetConditionsPlugin:         newGetConditionsPlugin,
	GenClientPlugin:             newGenClientPlugin,
	GenDeepCopyPlugin:           newGenDeepCopyPlugin,
	GenApplyConfigurationPlugin: newGenApplyConfigurationPlugin,
}

func CodegenPlugins(configs []config.Plugin) ([]Plugin, error) {
	plugins := []Plugin{}
	for _, cfg := range configs {
		builder, ok := codegenPlugins[cfg.Name]
		if !ok {
			return nil, fmt.Errorf("%q is not a registered plugin", cfg.Name)
		}
		plugin, err := builder(cfg)
		if err != nil {
			return nil, fmt.Errorf("plugin %q: %w", cfg.Name, err)
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// decodePluginOptions decodes cfg.Options into out using strict decoding,
// so unknown fields produce an error. An empty options node is treated as
// a no-op, leaving out at its zero value.
func decodePluginOptions(cfg config.Plugin, out any) error {
	if cfg.Options.IsZero() {
		return nil
	}
	buf, err := yaml.Marshal(&cfg.Options)
	if err != nil {
		return fmt.Errorf("re-encode options: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(buf))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode options: %w", err)
	}
	return nil
}
