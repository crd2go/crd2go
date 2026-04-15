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

package crd2go

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/crd2go/crd2go/internal/crd"
	"github.com/crd2go/crd2go/internal/crd/hooks"
	"github.com/crd2go/crd2go/internal/fileinput"
	"github.com/crd2go/crd2go/internal/gotype"
	"github.com/crd2go/crd2go/internal/render"
	"github.com/crd2go/crd2go/pkg/config"
)

func ParseGroupVersion(gv string) (string, string, error) {
	if gv == "" {
		return "", "", nil
	}
	parts := strings.Split(gv, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected properly formated group version, "+
			"such as \"gen.example.com/v1\" but got %q", gv)
	}
	return parts[0], parts[1], nil
}

func LoadConfig(r io.Reader) (*config.Config, error) {
	yml, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read the file: %w", err)
	}
	cfg := config.Config{}
	if err = yaml.Unmarshal(yml, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return &cfg, nil
}

// CodeWriterAtPath creates a file writer for the given CRD at the specified directory
func CodeWriterAtPath(dir string) config.CodeWriterFunc {
	return func(filename string, overwrite bool) (io.WriteCloser, error) {
		srcFile := filepath.Join(dir, filename)
		flags := os.O_CREATE | os.O_EXCL | os.O_WRONLY
		if overwrite {
			flags = os.O_CREATE | os.O_TRUNC | os.O_RDWR
		}
		safeSrcFile, err := fileinput.SafeAt(dir, srcFile)
		if err != nil {
			return nil, fmt.Errorf("unsafe file path %s: %w", srcFile, err)
		}
		// #nosec G304 gosec is confused here as SafeAt above already sanitized the input
		w, err := os.OpenFile(safeSrcFile, flags, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", srcFile, err)
		}
		return w, nil
	}
}

// GenerateToDir generates Go code from a CRD YAML file into a directory
func GenerateToDir(cfg *config.Config) error {
	if cfg.Output == "" {
		return fmt.Errorf("output directory is required")
	}
	td := gotype.NewTypeDict(cfg.Renames, gotype.KnownTypes()...).WithPinnings(cfg.Pinnings)
	if !cfg.ForceRenames {
		existing, err := gotype.ScanExistingStructNames(cfg.Output)
		if err != nil {
			return fmt.Errorf("scan package for existing type names: %w", err)
		}
		td.WithExistingNames(existing)
	}
	in, err := os.Open(cfg.Input)
	if err != nil {
		return fmt.Errorf("failed to open input file %s: %w", cfg.Input, err)
	}
	req := gotype.Request{
		CoreConfig:   cfg.CoreConfig,
		CodeWriterFn: CodeWriterAtPath(cfg.Output),
		TypeDict:     td,
	}
	if err := Generate(&req, in); err != nil {
		return fmt.Errorf("failed to generate CRD code: %w", err)
	}
	return nil
}

// Generate will write files using the CodeWriterFunc
func Generate(req *gotype.Request, r io.Reader) error {
	generatedGVRs, err := GenerateStream(req, r)
	if err != nil {
		return fmt.Errorf("failed to generate CRDs: %w", err)
	}
	if len(generatedGVRs) <= 0 {
		return nil
	}
	parts := strings.Split(generatedGVRs[0], "/")
	if len(parts) <= 2 {
		return nil
	}
	group := parts[0]
	version := parts[1]
	if err := render.Default.RenderDoc(req, group, version); err != nil {
		return fmt.Errorf("failed to generate the doc.go file for group version '%s/%s': %w", group, version, err)
	}
	if err := render.Default.RenderSchema(req, group, version); err != nil {
		return fmt.Errorf("failed to generate the schema.go file for group version '%s/%s': %w", group, version, err)
	}
	return nil
}

// GenerateStream generates Go code from a stream of CRDs within a YAML reader.
// It uses the provided CodeWriterFunc to write the generated code to the specified output.
// The version parameter specifies the version of the CRD to generate code for.
// The preloadedTypes parameter allows for preloading specific types to avoid name collisions.
func GenerateStream(req *gotype.Request, r io.Reader) ([]string, error) {
	preloaded := []*gotype.GoType{}
	for _, name := range req.Reserved {
		preloaded = append(preloaded, gotype.NewOpaqueType(name))
	}
	for _, importType := range req.Imports {
		preloaded = append(preloaded, gotype.NewAutoImportType(&importType))
	}
	req.TypeDict.AddAll(preloaded...)

	renderRequests, err := computeRenderRequests(req, bufio.NewScanner(r))
	if err != nil {
		return nil, fmt.Errorf("failed to generate CRDs code generation information: %w", err)
	}

	roots := make([]*gotype.GoType, 0, len(renderRequests))
	for _, renderReq := range renderRequests {
		roots = append(roots, renderReq.Type)
	}
	if err := req.TypeDict.RegisterAndResolve(roots); err != nil {
		return nil, fmt.Errorf("failed to resolve type names: %w", err)
	}

	generatedGVRs := []string{}
	for _, renderReq := range renderRequests {
		if err := render.Default.RenderCRD(&renderReq); err != nil {
			return nil, fmt.Errorf("failed to generate CRD code: %w", err)
		}
		gvr := fmt.Sprintf("%s/%s/%s", renderReq.Group, renderReq.Version, renderReq.Resource)
		gvr = strings.TrimPrefix(gvr, "/")
		generatedGVRs = append(generatedGVRs, gvr)
	}
	return generatedGVRs, nil
}

func computeRenderRequests(req *gotype.Request, scanner *bufio.Scanner) ([]render.CRDRenderRequest, error) {
	renderRequests := []render.CRDRenderRequest{}
	group, version, err := selectGroupVersion(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gv: %w", err)
	}
	for {
		crdSchema, err := crd.ParseCRD(scanner)
		if errors.Is(err, io.EOF) {
			if len(renderRequests) == 0 {
				return nil, fmt.Errorf("failed to parse CRD: %w", err)
			}
			return renderRequests, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
		if in(req.SkipList, crdSchema.Spec.Names.Kind) {
			continue
		}
		versionedCRD := crd.SelectVersion(&crdSchema.Spec, req.Version)
		if versionedCRD == nil {
			if req.Version == "" {
				return nil, fmt.Errorf("no versions to generate code from")
			}
			return nil, fmt.Errorf("no version %q to generate code from", req.Version)
		}
		goCRD, err := buildCRDType(req, versionedCRD)
		if err != nil {
			return nil, fmt.Errorf("could not build CRD type: %w", err)
		}

		group, version = autodetectGroupVersion(group, version,
			crdSchema.Spec.Group, versionedCRD.Version.Name)
		if version != versionedCRD.Version.Name || group != crdSchema.Spec.Group {
			if req.GroupVersion != "" {
				log.Printf("skipping %s of \"%s/%s\", not part of selected Group Version %q",
					versionedCRD.Kind,
					versionedCRD.Version.Name, crdSchema.Spec.Group, req.GroupVersion)
				continue
			}
			return nil, fmt.Errorf("YAML input should only contain kinds for %s/%s but got %s/%s",
				group, version, versionedCRD.Version.Name, crdSchema.Spec.Group)
		}
		renderReq := render.CRDRenderRequest{
			Request:  *req,
			Filename: crd.Kind2Filename(versionedCRD.Kind),
			Group:    crdSchema.Spec.Group,
			Version:  versionedCRD.Version.Name,
			Kind:     versionedCRD.Kind,
			Resource: crdSchema.Spec.Names.Plural,
			Type:     goCRD,
		}
		renderRequests = append(renderRequests, renderReq)
	}
}

func selectGroupVersion(req *gotype.Request) (string, string, error) {
	if req.GroupVersion != "" {
		return ParseGroupVersion(req.GroupVersion)
	}
	return "", "", nil
}

func autodetectGroupVersion(selectedGroup, selectedVersion, crdGroup, crdVersion string) (string, string) {
	group := selectedGroup
	version := selectedVersion
	if group == "" {
		group = crdGroup
	}
	if selectedVersion == "" {
		version = crdVersion
	}
	return group, version
}

func buildCRDType(req *gotype.Request, versionedCRD *crd.VersionedCRD) (*gotype.GoType, error) {
	specSchema := versionedCRD.Version.Schema.OpenAPIV3Schema.Properties["spec"]
	spec, err := crd.FromOpenAPIType(req.TypeDict, hooks.Hooks, &crd.CRDType{
		Name:    versionedCRD.SpecTypename(),
		Parents: []string{versionedCRD.Kind},
		Schema:  &specSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate spec type: %w", err)
	}

	statusSchema := versionedCRD.Version.Schema.OpenAPIV3Schema.Properties["status"]
	status, err := crd.FromOpenAPIType(req.TypeDict, hooks.Hooks, &crd.CRDType{
		Name:    versionedCRD.StatusTypename(),
		Parents: []string{versionedCRD.Kind},
		Schema:  &statusSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate status type: %w", err)
	}
	return combineSpecAndStatus(versionedCRD.Kind, spec, status)
}

func combineSpecAndStatus(kind string, spec, status *gotype.GoType) (*gotype.GoType, error) {
	metav1Package := config.ImportInfo{
		Alias: "metav1", Path: "k8s.io/apimachinery/pkg/apis/meta/v1",
	}
	typeMeta := gotype.NewAutoImportType(&config.ImportedTypeConfig{
		ImportInfo: metav1Package, Name: "TypeMeta",
	})
	objectMeta := gotype.NewAutoImportType(&config.ImportedTypeConfig{
		ImportInfo: metav1Package, Name: "ObjectMeta",
	})
	goCRD := gotype.NewStruct(kind, []*gotype.GoField{
		gotype.NewEmbeddedField(typeMeta).WithOptions(gotype.Required(true), gotype.JSONTag(",inline")),
		gotype.NewEmbeddedField(objectMeta).WithOptions(gotype.Required(true), gotype.JSONTag("metadata,omitempty")),
		gotype.NewGoField("Spec", spec).WithOptions(gotype.Required(true), gotype.JSONTag("spec,omitempty")),
		gotype.NewGoField("Status", status).WithOptions(gotype.Required(true), gotype.JSONTag("status,omitempty")),
	})
	return goCRD, nil
}

func in[T comparable](list []T, target T) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
