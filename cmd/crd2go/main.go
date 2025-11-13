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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/crd2go/crd2go/internal/checkerr"
	"github.com/crd2go/crd2go/internal/fileinput"
	"github.com/crd2go/crd2go/pkg/config"
	"github.com/crd2go/crd2go/pkg/crd2go"
)

func main() {
	var input, output, kinds, config string
	flag.StringVar(&input, "input", "", "input YAML to process")
	flag.StringVar(&output, "output", "", "output directory to produce source code to")
	flag.StringVar(&kinds, "kinds", "", "comma separated list of Kinds to consider for generation. "+
		"If empty, it generates all Kinds.")
	flag.StringVar(&config, "config", "crd2go.yaml", "YAML file with the CRD2Go config")
	flag.Parse()

	cfg, err := generateTypes(input, output, config, asList(kinds))
	if err != nil {
		log.Fatalf("Failed to generate go structs: %v", err)
	}
	log.Printf("Code generated at %s", cfg.Output)
}

func generateTypes(input, output, config string, kinds []string) (*config.Config, error) {
	f, err := os.Open(fileinput.MustBeSafe(config))
	if err != nil {
		return nil, fmt.Errorf("failed to open configuration file: %w", err)
	}
	defer checkerr.CheckErr("closing config file", f.Close)
	cfg, err := crd2go.LoadConfig(f)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if input != "" {
		cfg.Input = input
	}
	if output != "" {
		cfg.Output = output
	}
	cfg.Kinds = kinds
	if err := crd2go.GenerateToDir(cfg); err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}
	return cfg, nil
}

func asList(s string) []string {
	if s == "" {
		return []string{}
	}
	results := strings.Split(s, ",")
	for i, item := range results {
		results[i] = strings.TrimSpace(item)
	}
	return results
}
