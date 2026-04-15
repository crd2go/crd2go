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
	"strings"
)

// SuggestPinnings scans each conflict's existing on-disk struct and compares its
// fields against the candidate GoTypes to find the best match. It returns one
// pinning path string per conflict: the single matching path if narrowing succeeds,
// or all candidate paths joined with " | " when no single match is found.
//
// File I/O is performed here, not in the naming engine, so it only happens when
// conflicts actually exist.
func SuggestPinnings(conflicts []ExistingNameConflict) ([]string, error) {
	pinnings := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		suggestion, err := suggestPinning(c)
		if err != nil {
			return nil, err
		}
		pinnings = append(pinnings, suggestion)
	}
	return pinnings, nil
}

func suggestPinning(c ExistingNameConflict) (string, error) {
	existingFields, err := ScanStructFields(c.ExistingFile, c.Name)
	if err != nil {
		return "", fmt.Errorf("scanning fields of existing type %q in %s: %w", c.Name, c.ExistingFile, err)
	}
	var matched []string
	for _, info := range c.candidates {
		if goTypeFieldsMatch(info.gt, existingFields) {
			matched = append(matched, strings.Join(info.path, "."))
		}
	}
	if len(matched) == 1 {
		return matched[0], nil
	}
	all := make([]string, len(c.candidates))
	for i, info := range c.candidates {
		all[i] = strings.Join(info.path, ".")
	}
	return strings.Join(all, " | "), nil
}

func goTypeFieldsMatch(gt *GoType, existingFields []string) bool {
	var names []string
	for _, f := range gt.Fields {
		if !f.IsEmbedded() {
			names = append(names, f.Name)
		}
	}
	if len(names) != len(existingFields) {
		return false
	}
	for i, name := range names {
		if name != existingFields[i] {
			return false
		}
	}
	return true
}

// FormatPinningsSuggestion formats a list of pinning suggestions as a YAML snippet
// that can be pasted directly into the crd2go config file.
// Entries containing " | " are annotated with "# pick one".
func FormatPinningsSuggestion(pinnings []string) string {
	var b strings.Builder
	b.WriteString("pinnings:\n")
	for _, p := range pinnings {
		if strings.Contains(p, " | ") {
			fmt.Fprintf(&b, "  - %s # pick one\n", p)
		} else {
			fmt.Fprintf(&b, "  - %s\n", p)
		}
	}
	return b.String()
}
