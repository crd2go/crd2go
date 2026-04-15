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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/crd2go/crd2go/internal/checkerr"
	"github.com/crd2go/crd2go/internal/fileinput"
	"github.com/crd2go/crd2go/internal/gotype"
	"github.com/crd2go/crd2go/pkg/config"
	"github.com/crd2go/crd2go/pkg/crd2go"
)

func main() {
	var input, output, gv, config string
	var forceRenames bool
	flag.StringVar(&input, "input", "", "input YAML to process")
	flag.StringVar(&output, "output", "", "output directory to produce source code to")
	flag.StringVar(&config, "config", "crd2go.yaml", "YAML file with the CRD2Go config")
	flag.StringVar(&gv, "gv", "", "Group Version (e.g 'gen.example.com/v1') to generate from.")
	flag.BoolVar(&forceRenames, "force-renames", false, "allow crd2go to rename existing types when conflicts arise")
	flag.Parse()

	cfg, err := generate(input, output, gv, config, forceRenames)
	if err != nil {
		printConflictHint(err)
		log.Fatalf("Failed to generate go structs: %v", err)
	}
	log.Printf("Code generated at %s", cfg.Output)
}

// printConflictHint checks if err contains an ExistingNameConflictError and, if so,
// prints a pinning suggestion to stderr to help the user resolve the conflict.
func printConflictHint(err error) {
	var conflictErr *gotype.ExistingNameConflictError
	if !errors.As(err, &conflictErr) {
		return
	}
	fmt.Fprintln(os.Stderr, "conflicting names found with existing type names, please use these pinnings in the config to fix:")
	fmt.Fprintln(os.Stderr)
	pinnings, pinErr := gotype.SuggestPinnings(conflictErr.Conflicts)
	if pinErr == nil {
		fmt.Fprint(os.Stderr, gotype.FormatPinningsSuggestion(pinnings))
	}
	fmt.Fprintln(os.Stderr, "\nOr use flag --force-renames to allow crd2go to rename existing types as needed.")
}

func generate(input, output, gv, config string, forceRenames bool) (*config.Config, error) {
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
	cfg.ForceRenames = forceRenames
	if _, _, err := crd2go.ParseGroupVersion(gv); err != nil {
		return nil, fmt.Errorf("failed to parse gv: %w", err)
	}
	cfg.GroupVersion = gv
	if err := crd2go.GenerateToDir(cfg); err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}
	return cfg, nil
}
