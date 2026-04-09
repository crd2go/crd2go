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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrUnresolvedNameCollision = errors.New("could not assign distinct Go names to colliding types")

// HashType returns a stable structural key for a GoType: the single representation
// used for deduplication (NameEngine), known-type lookup (TypeDict), and builtins.
func HashType(gt *GoType) string {
	if gt == nil {
		return ""
	}
	switch gt.Kind {
	case OpaqueKind, AutoImportKind:
		if gt.Import != nil {
			return fmt.Sprintf("%s.%s", gt.Import.Path, gt.Name)
		}
		return gt.Name
	case StructKind:
		if len(gt.Fields) == 0 {
			sum := sha256.Sum256([]byte("struct:empty:" + gt.Name))
			return fmt.Sprintf("sha256:%s", hex.EncodeToString(sum[:]))
		}
		hash := sha256.New()
		for _, field := range gt.Fields {
			_, _ = fmt.Fprintf(hash, "%s:%s", field.Name, HashType(field.GoType))
		}
		return fmt.Sprintf("sha256:%s", hex.EncodeToString(hash.Sum(nil)))
	case ArrayKind:
		if gt.Element == nil {
			return "[]"
		}
		return fmt.Sprintf("[]%s", HashType(gt.Element))
	case MapKind:
		if gt.Element == nil {
			return "map:"
		}
		return fmt.Sprintf("map:%s", HashType(gt.Element))
	default:
		return gt.Kind
	}
}

var ErrDuplicateRoot = errors.New("root already registered with same name/path")

type NameEngine interface {
	// Register a new type in the name engine.
	// Returns ErrDuplicateRoot if a root is already registered with the same path.
	Register(path []string, gt *GoType) error

	// NamedRoots returns the named roots and applies final names to all registered types.
	// Names are unique and deterministic. Among types with the same structure (aliases), the
	// shortest name wins. When different types would share the same Go identifier, ancestor
	// path segments are prepended (immediate parent first, then toward the root) until names
	// differ; those disambiguated names are not guaranteed to be the shortest possible.
	NamedRoots() ([]*GoType, error)

	// Has returns true if a type with the same structure (hash) was registered.
	Has(gt *GoType) bool

	// Get returns a type by its final name. Must be called after NamedRoots.
	Get(name string) (*GoType, bool)
}

type typeInfo struct {
	path []string
	hash string
	gt   *GoType
}

type nameEngine struct {
	byHash        map[string][]typeInfo
	hashCache     map[*GoType]string
	roots         []*GoType
	byName        map[string]*GoType
	existingNames map[string]map[string]struct{}
}

func NewNameEngine() NameEngine {
	return &nameEngine{
		byHash:    make(map[string][]typeInfo),
		hashCache: make(map[*GoType]string),
	}
}

func (n *nameEngine) Register(path []string, gt *GoType) error {
	hash := hashTypeFast(n.hashCache, gt)
	info := typeInfo{path: path, hash: hash, gt: gt}
	n.byHash[hash] = append(n.byHash[hash], info)
	if len(path) == 1 && path[0] == gt.Name {
		if !n.insertSorted(gt) {
			return fmt.Errorf("%w: %s", ErrDuplicateRoot, gt.Name)
		}
	}
	return nil
}

func (n *nameEngine) insertSorted(gt *GoType) bool {
	name := gt.Name
	i := sort.Search(len(n.roots), func(j int) bool {
		return n.roots[j].Name >= name
	})
	if i < len(n.roots) && n.roots[i].Name == name {
		return false
	}
	n.roots = append(n.roots, nil)
	copy(n.roots[i+1:], n.roots[i:])
	n.roots[i] = gt
	return true
}

func (n *nameEngine) NamedRoots() ([]*GoType, error) {
	byName := n.solveTypeAliases()
	for _, sameName := range byName {
		conflictingTypes := uniqueTypes(sameName)
		if len(conflictingTypes) <= 1 {
			continue
		}
		if err := n.solveConflictingNames(conflictingTypes); err != nil {
			return nil, err
		}
	}
	n.buildByNameIndex()
	return n.roots, nil
}

func (n *nameEngine) Has(gt *GoType) bool {
	if gt == nil {
		return false
	}
	hash := hashTypeFast(n.hashCache, gt.BaseType())
	_, ok := n.byHash[hash]
	return ok
}

func (n *nameEngine) Get(name string) (*GoType, bool) {
	if n.byName == nil {
		return nil, false
	}
	gt, ok := n.byName[name]
	return gt, ok
}

func (n *nameEngine) buildByNameIndex() {
	n.byName = make(map[string]*GoType)
	for _, infos := range n.byHash {
		if len(infos) == 1 {
			n.byName[infos[0].gt.Name] = infos[0].gt
		}
		if len(infos) > 1 {
			panic(fmt.Sprintf("multiple %v types with hash: %s", infos, infos[0].hash))
		}
	}
}

// solveTypeAliases picks a winner name for each type (by hash), applies it to all aliases,
// and builds a byName index in a single pass. Returns the index for conflict resolution.
// Winner: shortest name, then first alphabetically.
func (n *nameEngine) solveTypeAliases() map[string][]typeInfo {
	byName := make(map[string][]typeInfo)
	for hash, infos := range n.byHash {
		if len(infos) == 1 {
			byName[infos[0].gt.Name] = append(byName[infos[0].gt.Name], infos[0])
			continue
		}
		winner := bestName(infos)
		for _, info := range infos {
			if !isRoot(info) {
				info.gt.Name = winner
			}
			byName[info.gt.Name] = append(byName[info.gt.Name], info)
		}
		// safe to update while in range. See https://go.dev/ref/spec#For_range
		n.byHash[hash] = []typeInfo{infos[0]}
	}
	return byName
}

func isRoot(info typeInfo) bool {
	return len(info.path) == 1 && info.path[0] == info.gt.Name
}

// uniqueTypes returns one typeInfo per unique GoType (by hash).
// For types with the same hash, it keeps the one with shortest path, closer to
// the root.
func uniqueTypes(infos []typeInfo) []typeInfo {
	byHash := make(map[string]typeInfo)
	for _, info := range infos {
		h := info.hash
		if existing, ok := byHash[h]; ok {
			if len(info.path) < len(existing.path) {
				byHash[h] = info
			}
			continue
		}
		byHash[h] = info
	}
	out := make([]typeInfo, 0, len(byHash))
	for _, info := range byHash {
		out = append(out, info)
	}
	return out
}

func bestName(infos []typeInfo) string {
	names := make(map[string]bool)
	for _, info := range infos {
		names[info.gt.Name] = true
	}
	candidates := make([]string, 0, len(names))
	for name := range names {
		candidates = append(candidates, name)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i]) != len(candidates[j]) {
			return len(candidates[i]) < len(candidates[j])
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0]
}

func (n *nameEngine) solveConflictingNames(candidateTypes []typeInfo) error {
	maxPathLen := len(candidateTypes[0].path)
	for _, c := range candidateTypes[1:] {
		if len(c.path) > maxPathLen {
			maxPathLen = len(c.path)
		}
	}
	// Prepend ancestor path segments (root first). Skip the last segment since it often equals the type name.
	maxRounds := maxPathLen
	if maxRounds > 0 {
		maxRounds--
	}
	// Prepend from immediate parent toward root (leaf-to-root) to match legacy behavior.
	for round := 1; round <= maxRounds; round++ {
		for i := range candidateTypes {
			c := &candidateTypes[i]
			if n.shouldFreezeConflictCandidate(*c) {
				continue
			}
			if round <= len(c.path)-1 {
				idx := len(c.path) - 1 - round
				c.gt.Name = c.path[idx] + c.gt.Name
			}
		}
		seen := make(map[string]bool)
		allUnique := true
		for _, c := range candidateTypes {
			if seen[c.gt.Name] {
				allUnique = false
				break
			}
			seen[c.gt.Name] = true
		}
		if allUnique {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedNameCollision, candidateTypes[0].gt.Name)
}

func (n *nameEngine) shouldFreezeConflictCandidate(c typeInfo) bool {
	if len(n.existingNames) == 0 || len(c.path) == 0 {
		return false
	}
	stem := strings.ToLower(c.path[0])
	fileTypes, ok := n.existingNames[stem]
	if !ok {
		return false
	}
	_, found := fileTypes[c.gt.Name]
	return found
}

func hashTypeFast(hashCache map[*GoType]string, gt *GoType) string {
	if hash, ok := hashCache[gt]; ok {
		return hash
	}
	hash := HashType(gt)
	hashCache[gt] = hash
	return hash
}
