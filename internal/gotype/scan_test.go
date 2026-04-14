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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanStructFields(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "widget.go"), []byte(`package x

type Widget struct {
	Name  string
	Count int
	Tags  []string
}

type Other struct{ X int }
`), 0o644))

	fields, err := ScanStructFields(dir, "widget.go", "Widget")
	require.NoError(t, err)
	require.Equal(t, []string{"Name", "Count", "Tags"}, fields)

	fields, err = ScanStructFields(dir, "widget.go", "Other")
	require.NoError(t, err)
	require.Equal(t, []string{"X"}, fields)

	fields, err = ScanStructFields(dir, "widget.go", "Missing")
	require.NoError(t, err)
	require.Nil(t, fields)
}

func TestScanExistingStructNames(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "widget.go"), []byte(`package x

type Widget struct { A int }
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gadget.go"), []byte(`package x
type Gadget struct{}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "doc.go"), []byte(`package x
type DocIgnored struct{}
`), 0o644))

	got, err := ScanExistingStructNames(dir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "widget.go"), got["Widget"])
	require.Equal(t, filepath.Join(dir, "gadget.go"), got["Gadget"])
	require.Empty(t, got["DocIgnored"])
}
