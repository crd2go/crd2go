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

package gotype

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ScanExistingStructNames walks dir for *.go files (excluding generated / meta files),
// parses each file, and returns a map from lowercased file stem (as in Kind2Filename)
// to the set of top-level struct type names declared in that file.
//
// The stem key matches strings.ToLower(kind) for CRD kind "Kind" emitted as kind.go,
// so it aligns with name-engine path[0] after lowercasing.
//
// If dir is empty or does not exist, returns (nil, nil).
func ScanExistingStructNames(dir string) (map[string]map[string]struct{}, error) {
	if dir == "" {
		return nil, nil
	}
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("naming stability scan path is not a directory: %s", dir)
	}
	out := make(map[string]map[string]struct{})
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".go") {
			return nil
		}
		if skipNamingScanFile(base) {
			return nil
		}
		stemKey := strings.TrimSuffix(strings.ToLower(base), ".go")
		names, perr := topLevelStructTypeNames(path)
		if perr != nil {
			return perr
		}
		if len(names) == 0 {
			return nil
		}
		if out[stemKey] == nil {
			out[stemKey] = make(map[string]struct{}, len(names))
		}
		for _, n := range names {
			out[stemKey][n] = struct{}{}
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func skipNamingScanFile(base string) bool {
	switch strings.ToLower(base) {
	case "doc.go", "groupversion_info.go", "schema.go":
		return true
	default:
		if strings.HasPrefix(strings.ToLower(base), "zz_generated") {
			return true
		}
		return false
	}
}

func topLevelStructTypeNames(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	var names []string
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil {
				continue
			}
			if _, isStruct := ts.Type.(*ast.StructType); isStruct {
				names = append(names, ts.Name.Name)
			}
		}
	}
	return names, nil
}
