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
	"strings"
	"unicode"

	"github.com/dave/jennifer/jen"

	"github.com/crd2go/crd2go/pkg/config"
)

const (
	GetConditionsPlugin = "get-conditions"
)

type GetConditions struct {
}

func newGetConditionsPlugin(cfg config.Plugin) (Plugin, error) {
	if err := decodePluginOptions(cfg, &struct{}{}); err != nil {
		return nil, err
	}
	return &GetConditions{}, nil
}

func (*GetConditions) Name() string {
	return GetConditionsPlugin
}

func (*GetConditions) Annotate(_ *jen.File, _ string) error {
	return nil
}

func (*GetConditions) Process(cr *CodegenRequest) error {
	shortName := shorten(cr.Type.Name)
	f := cr.File
	f.Line()
	f.Comment(fmt.Sprintf("GetConditions for %s", cr.Type.Name))
	f.Func().Params(
		jen.Id(shortName).Op("*").Id(cr.Type.Name),
	).Id("GetConditions").Params().Index().Qual("k8s.io/apimachinery/pkg/apis/meta/v1", "Condition").Block(

		jen.If(jen.Id(shortName).Dot("Status").Dot("Conditions").Op("==").Nil()).Block(
			jen.Return(jen.Nil()),
		),

		jen.Return(jen.Op("*").Id(shortName).Dot("Status").Dot("Conditions")),
	)
	return nil
}

func shorten(s string) string {
	if len(s) == 0 {
		return ""
	}
	var sb strings.Builder

	runes := []rune(s)
	lastLower := false
	for i, r := range runes {
		if i == 0 {
			sb.WriteRune(unicode.ToLower(r))
			continue
		}
		if unicode.IsUpper(r) && lastLower {
			sb.WriteRune(unicode.ToLower(r))
			lastLower = false
		} else {
			lastLower = true
		}
	}
	return sb.String()
}
