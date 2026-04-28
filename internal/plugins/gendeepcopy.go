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
	GenDeepCopyPlugin = "gen-deepcopy"
)

// GenDeepCopy emits the `+k8s:deepcopy-gen` markers that controller-gen
// consumes to produce DeepCopy methods. Presence of the plugin in the config
// turns generation on; absence turns it off.
type GenDeepCopy struct {
	BasePlugin
}

func newGenDeepCopyPlugin(cfg config.Plugin) (Plugin, error) {
	if err := decodePluginOptions(cfg, &struct{}{}); err != nil {
		return nil, err
	}
	return &GenDeepCopy{}, nil
}

func (*GenDeepCopy) Name() string {
	return GenDeepCopyPlugin
}

// Annotate emits the per-CRD deepcopy-gen interface marker before the root
// kind type, so controller-gen generates a DeepCopyObject method.
func (*GenDeepCopy) Annotate(f *jen.File, _ string) error {
	f.Comment("+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object")
	return nil
}

// DocAnnotate emits the package-level deepcopy-gen header marker in doc.go
// and a trailing reminder comment pointing at controller-gen.
func (*GenDeepCopy) DocAnnotate(f *jen.File, _, _ string) error {
	f.HeaderComment("+k8s:deepcopy-gen=package")
	f.Add(jen.Commentf("controller-gen object paths=..."))
	return nil
}
