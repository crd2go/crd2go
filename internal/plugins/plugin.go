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

type Plugin interface {
	Name() string
	Process(cgr *CodegenRequest) error
	Annotate(f *jen.File, kind string) error
}

type PluginBuilderFunc func(config.Plugin) (Plugin, error)

var codegenPlugins = map[string]PluginBuilderFunc{
	GetConditionsPlugin: newGetConditionsPlugin,
	GenClientPlugin:     newGenClientPlugin,
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
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// decodePluginOptions decodes cfg.Options into out using strict decoding,
// so unknown fields produce an error. An empty options node is treated as
// a no-op, leaving out at its zero value.
func decodePluginOptions(cfg config.Plugin, out any) error {
	if cfg.Options.Kind == 0 {
		return nil
	}
	buf, err := yaml.Marshal(&cfg.Options)
	if err != nil {
		return fmt.Errorf("plugin %q: re-encode options: %w", cfg.Name, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(buf))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("plugin %q: decode options: %w", cfg.Name, err)
	}
	return nil
}
