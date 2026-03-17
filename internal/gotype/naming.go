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
)

func HashType(gt *GoType) string {
	if gt == nil {
		return ""
	}
	if gt.IsPrimitive() {
		return gt.Kind
	}
	if gt.Element != nil {
		return fmt.Sprintf("[]%s", HashType(gt.Element))
	}
	hash := sha256.New()
	for _, field := range gt.Fields {
		hash.Write([]byte(fmt.Sprintf("%s:%s", field.Name, HashType(field.GoType))))
	}
	return fmt.Sprintf("sha256:%s", hex.EncodeToString(hash.Sum(nil)))
}

var ErrDuplicateRoot = errors.New("root already registered with same name/path")

type NameEngine interface {
	// Register a new type in the name engine.
	// Returns ErrDuplicateRoot if a root is already registered with the same path.
	Register(path []string, gt *GoType) error

	// Get the named roots of the name engine
	// This is a alphabetically sorted list of the types that are roots
	// in the name engine.
	// All types are uniquely and deterministically named with the shortest
	// possible name.
	// Collisions are resolved by appending the field parent
	// name to all coliding types.
	NamedRoots() []*GoType
}

type typeInfo struct {
	path []string
	hash string
	gt   *GoType
}

type nameEngine struct {
	byHash     map[string][]typeInfo
	hashCache  map[*GoType]string
	roots      []*GoType
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

func (n *nameEngine) NamedRoots() []*GoType {
	byName := n.deduplicateNIndex()
	for _, candidateTypes := range byName {
		if len(candidateTypes) <= 1 {
			continue
		}
		n.fixConflicts(candidateTypes)
	}
	return n.roots
}

// deduplicateNIndex picks a winner name for each type (by hash), applies it to all aliases,
// and builds a byName index in a single pass. Returns the index for conflict resolution.
// Winner: shortest name, then first alphabetically.
func (n *nameEngine) deduplicateNIndex() map[string][]typeInfo {
	byName := make(map[string][]typeInfo)
	for hash, infos := range n.byHash {
		if len(infos) == 1 {
			byName[infos[0].gt.Name] = append(byName[infos[0].gt.Name], infos[0])
			continue
		}
		winner := bestName(infos)
		for _, info := range infos {
			info.gt.Name = winner
		}
		n.byHash[hash] = []typeInfo{infos[0]}
		byName[winner] = append(byName[winner], infos[0])
	}
	return byName
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

func (n *nameEngine) fixConflicts(candidateTypes []typeInfo) {
	maxPathLen := len(candidateTypes[0].path)
	for _, c := range candidateTypes[1:] {
		if len(c.path) > maxPathLen {
			maxPathLen = len(c.path)
		}
	}
	for round := 1; round <= maxPathLen; round++ {
		for i := range candidateTypes {
			c := &candidateTypes[i]
			if round <= len(c.path) {
				c.gt.Name = c.path[len(c.path)-round] + c.gt.Name
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
			return
		}
	}
}

func hashTypeFast(hashCache map[*GoType]string, gt *GoType) string {
	if hash, ok := hashCache[gt]; ok {
		return hash
	}
	hash := HashType(gt)
	hashCache[gt] = hash
	return hash
}
