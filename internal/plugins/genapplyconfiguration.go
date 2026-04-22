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
	"fmt"

	"github.com/dave/jennifer/jen"

	"github.com/crd2go/crd2go/pkg/config"
)

const (
	GenApplyConfigurationPlugin = "gen-applyconfiguration"
)

// GenApplyConfiguration emits the `+kubebuilder:ac` markers that
// controller-gen's applyconfiguration generator consumes, plus the
// SchemeGroupVersion alias that the generated apply-configuration code
// expects (k8s.io/code-generator convention).
type GenApplyConfiguration struct {
	BasePlugin
	outputPackage string
}

type genApplyConfigurationOpts struct {
	OutputPackage string `yaml:"outputPackage"`
}

func newGenApplyConfigurationPlugin(cfg config.Plugin) (Plugin, error) {
	var opts genApplyConfigurationOpts
	if err := decodePluginOptions(cfg, &opts); err != nil {
		return nil, err
	}
	return &GenApplyConfiguration{outputPackage: opts.OutputPackage}, nil
}

func (*GenApplyConfiguration) Name() string {
	return GenApplyConfigurationPlugin
}

// DocAnnotate emits the applyconfiguration header markers in doc.go.
func (gac *GenApplyConfiguration) DocAnnotate(f *jen.File, _, _ string) error {
	f.HeaderComment("+kubebuilder:ac:generate=true")
	if gac.outputPackage != "" {
		f.HeaderComment(fmt.Sprintf("+kubebuilder:ac:output:package=%s", gac.outputPackage))
	}
	return nil
}

// SchemeVars contributes the SchemeGroupVersion alias required by the
// generated apply-configuration code.
func (*GenApplyConfiguration) SchemeVars(_, _ string) ([]jen.Code, error) {
	return []jen.Code{
		jen.Line(),
		jen.Comment("SchemeGroupVersion is an alias for GroupVersion, required by the"),
		jen.Comment("generated apply configuration code (k8s.io/code-generator convention)."),
		jen.Id("SchemeGroupVersion").Op("=").Id("GroupVersion"),
	}, nil
}
