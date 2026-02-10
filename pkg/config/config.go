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

package config

import (
	"io"
)

// CodeWriterFunc is a function type that takes a CRD and returns a writer for the generated code
type CodeWriterFunc func(filename string, overwrite bool) (io.WriteCloser, error)

// GenMode controls how optional code generation steps are handled
type GenMode string

const (
	// GenModeAuto runs the generator when present in $PATH
	GenModeAuto GenMode = "auto"

	// GenModeOff will skip the generator
	GenModeOff GenMode = "off"

	// GenModeForced always runs the generator after CRD code generation
	GenModeForced GenMode = "forced"
)

// GenDeepCopy controls how deep copy generation is handled
type GenDeepCopy = GenMode

const (
	// GenDeepCopyAuto runs controller-gen when present in $PATH
	GenDeepCopyAuto = GenModeAuto

	// GenDeepCopyOff will skip controller-gen
	GenDeepCopyOff = GenModeOff

	// GenDeepCopyForced always runs controller-gen after CRD code generation
	GenDeepCopyForced = GenModeForced
)

// Config holds all CLI configurable parameters
type Config struct {
	CoreConfig `yaml:",inline"`

	Input  string `yaml:"input"`
	Output string `yaml:"output"`
}

// ImportedTypeConfig holds one imported type information
type ImportedTypeConfig struct {
	ImportInfo `yaml:",inline"`
	Name       string `yaml:"name"`
}

// ImportInfo holds the import path and alias for existing types
type ImportInfo struct {
	Alias string
	Path  string
}

// CoreConfig holds the subset of the config without the input and output fields
type CoreConfig struct {
	Version            string               `yaml:"version"`
	Reserved           []string             `yaml:"reserved"`
	SkipList           []string             `yaml:"skipList"`
	Renames            map[string]string    `yaml:"renames"`
	Imports            []ImportedTypeConfig `yaml:"imports"`
	Plugins            []Plugin             `yaml:"plugins"`
	DeepCopy           DeepCopy             `yaml:"deepCopy"`
	ApplyConfiguration ApplyConfiguration   `yaml:"applyConfiguration"`
	GroupVersion       string               `yaml:"-"`
}

// Plugin represents a named plugin code that can be optionally invoked
type Plugin struct {
	Name string `yaml:"name"`
}

type DeepCopy struct {
	Generate          GenDeepCopy `yaml:"generate"`
	ControllerGenPath string      `yaml:"controllerGenPath"`
}

// ApplyConfiguration controls how apply configuration generation is handled
type ApplyConfiguration struct {
	Generate          GenMode `yaml:"generate"`
	OutputPackage     string  `yaml:"outputPackage"`
	ControllerGenPath string  `yaml:"controllerGenPath"`
}
