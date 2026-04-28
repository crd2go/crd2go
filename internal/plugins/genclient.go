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
	"github.com/dave/jennifer/jen"

	"github.com/crd2go/crd2go/pkg/config"
)

const (
	GenClientPlugin = "gen-client"
)

type GenClient struct {
	BasePlugin
	nonNamespaced bool
}

type genClientOpts struct {
	NonNamespaced bool `yaml:"nonNamespaced"`
}

func newGenClientPlugin(cfg config.Plugin) (Plugin, error) {
	var opts genClientOpts
	if err := decodePluginOptions(cfg, &opts); err != nil {
		return nil, err
	}
	return &GenClient{nonNamespaced: opts.NonNamespaced}, nil
}

func (*GenClient) Name() string {
	return GenClientPlugin
}

// Annotate adds +genclient markers before the top-level kind type definition.
func (gc *GenClient) Annotate(f *jen.File, _ string) error {
	f.Comment("+genclient")
	if gc.nonNamespaced {
		f.Comment("+genclient:nonNamespaced")
	}
	return nil
}
